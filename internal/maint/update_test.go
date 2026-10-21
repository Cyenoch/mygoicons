package maint

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cyenoch/mygoicons/generate"
)

func archive(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	for name, data := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func put(t *testing.T, root, name string, data []byte) {
	t.Helper()
	file := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0644); err != nil {
		t.Fatal(err)
	}
}
func fixture(t *testing.T) (string, Lock) {
	t.Helper()
	root := t.TempDir()
	lock := Lock{SchemaVersion: 1, RetrievedAt: "2026-10-16"}
	for _, prefix := range []string{"lucide", "twemoji"} {
		c := Collection{Prefix: prefix, Package: "@iconify-json/" + prefix, Version: "1.0.0", Files: map[string]Digest{}, License: License{Metadata: json.RawMessage(`{"spdx":"MIT"}`), Notices: []Notice{{File: "LICENSE", UpstreamPath: "LICENSE", SHA256: digest([]byte("notice")), Bytes: 6}}}}
		files := map[string][]byte{"icons.json": []byte(fmt.Sprintf(`{"prefix":%q,"icons":{"a":{"body":"<path d=\"M0 0h1\"/>"}}}`, prefix)), "info.json": []byte(`{"license":{"spdx":"MIT"}}`), "package.json": []byte(fmt.Sprintf(`{"name":%q,"version":"1.0.0"}`, c.Package))}
		for name, data := range files {
			put(t, root, "sources/"+prefix+"/"+name, data)
			c.Files[name] = Digest{SHA256: digest(data), Bytes: len(data)}
		}
		r, err := generate.Build(files["icons.json"], generate.Options{Package: prefix})
		if err != nil {
			t.Fatal(err)
		}
		put(t, root, prefix+"/icons_gen.go", r.Source)
		put(t, root, "sources/"+prefix+"/exports.json", marshal(r.Exports))
		put(t, root, "sources/"+prefix+"/LICENSE", []byte("notice"))
		lock.Collections = append(lock.Collections, c)
	}
	put(t, root, "sources/lock.json", marshal(lock))
	put(t, root, "examples/native/catalog_gen.go", []byte("old catalog"))
	return root, lock
}
func TestPlanOfflineAndConsistency(t *testing.T) {
	for _, scenario := range []string{"success", "integrity", "identity", "compatibility", "license", "late-failure"} {
		t.Run(scenario, func(t *testing.T) {
			root, _ := fixture(t)
			before := snapshot(t, root)
			files := map[string][]byte{"package/icons.json": []byte(`{"prefix":"lucide","icons":{"a":{"body":"<path d=\"M0 0h1\"/>"},"b":{"body":"<circle r=\"2\"/>"}}}`), "package/info.json": []byte(`{"license":{"spdx":"MIT"}}`), "package/package.json": []byte(`{"name":"@iconify-json/lucide","version":"1.1.0"}`)}
			if scenario == "compatibility" {
				files["package/icons.json"] = []byte(`{"prefix":"lucide","icons":{"b":{"body":"<circle/>"}}}`)
			}
			if scenario == "license" {
				files["package/info.json"] = []byte(`{"license":{"spdx":"ISC"}}`)
			}
			if scenario == "identity" {
				files["package/package.json"] = []byte(`{"name":"wrong","version":"1.1.0"}`)
			}
			tgz := archive(t, files)
			hash := sha512.Sum512(tgz)
			integrity := "sha512-" + base64.StdEncoding.EncodeToString(hash[:])
			if scenario == "integrity" {
				integrity = "sha512-AAAA"
			}
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/tarball":
					w.Write(tgz)
				case strings.HasSuffix(r.URL.Path, "/1.1.0"):
					w.Write(marshal(map[string]any{"name": "@iconify-json/lucide", "version": "1.1.0", "dist": map[string]string{"tarball": server.URL + "/tarball", "integrity": integrity}}))
				case strings.HasSuffix(r.URL.Path, "/commits/HEAD"):
					w.Write([]byte(`{"sha":"1111111111111111111111111111111111111111"}`))
				case strings.Contains(r.URL.Path, "/contents/"):
					if scenario == "late-failure" {
						http.Error(w, "failed", 500)
						return
					}
					raw := []byte("notice")
					h := sha1.Sum(append(fmt.Appendf(nil, "blob %d\x00", len(raw)), raw...))
					w.Write(marshal(map[string]string{"sha": fmt.Sprintf("%x", h), "encoding": "base64", "content": base64.StdEncoding.EncodeToString(raw)}))
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			u := New(root)
			u.Registry = server.URL
			u.GitHub = server.URL
			u.Client = server.Client()
			changes, report, err := u.Plan(t.Context(), map[string]string{"lucide": "1.1.0", "twemoji": "1.0.0"}, false)
			if !equalSnapshots(before, snapshot(t, root)) {
				t.Fatal("Plan modified files")
			}
			if scenario != "success" {
				if err == nil || len(changes) != 0 {
					t.Fatal("invalid update accepted", report, err)
				}
				return
			}
			if err != nil {
				t.Fatal(report, err)
			}
			if !strings.Contains(report, "exports: +1 -0") {
				t.Fatal(report)
			}
			if err = Apply(root, changes); err != nil {
				t.Fatal(err)
			}
			if _, err = u.load(); err != nil {
				t.Fatal("applied inconsistent source lock", err)
			}
			again, _, err := u.Plan(t.Context(), map[string]string{"lucide": "1.1.0", "twemoji": "1.0.0"}, false)
			if err != nil || len(again) != 0 {
				t.Fatal("same version not a no-op", err)
			}
		})
	}
}
func snapshot(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			result[rel] = data
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func equalSnapshots(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if !bytes.Equal(v, b[k]) {
			return false
		}
	}
	return true
}
func TestArchiveBoundaries(t *testing.T) {
	for _, name := range []string{"../icons.json", "/package/icons.json", "package/../../escape", "package\\icons.json", "package/a/../icons.json"} {
		if _, err := unpack(archive(t, map[string][]byte{name: []byte("x")})); err == nil {
			t.Fatal("unsafe archive accepted", name)
		}
	}
	for _, header := range []*tar.Header{{Name: "package/link", Typeflag: tar.TypeSymlink, Linkname: "../../x"}, {Name: "package/huge", Size: generate.MaxInputBytes + 1}} {
		var b bytes.Buffer
		gz := gzip.NewWriter(&b)
		tw := tar.NewWriter(gz)
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		tw.Close()
		gz.Close()
		if _, err := unpack(b.Bytes()); err == nil {
			t.Fatal("invalid entry accepted")
		}
	}
	files := map[string][]byte{}
	for i := range 130 {
		files[fmt.Sprintf("package/f%d", i)] = nil
	}
	if _, err := unpack(archive(t, files)); err == nil {
		t.Fatal("too many entries accepted")
	}
	// Expanded trailing bytes are bounded even after the tar end marker.
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	gz.Write(make([]byte, 1024))
	io.CopyN(gz, zeroReader{}, maxExpanded)
	gz.Close()
	if _, err := unpack(b.Bytes()); err == nil {
		t.Fatal("gzip expansion accepted")
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }
func TestDownloadBoundaries(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("123456789")) }))
	defer s.Close()
	u := New(t.TempDir())
	u.Client = s.Client()
	if _, err := u.get(context.Background(), s.URL, 8); err == nil {
		t.Fatal("oversized response accepted")
	}
}
func TestApplyFailureAndRecovery(t *testing.T) {
	root, _ := fixture(t)
	before := snapshot(t, root)
	if err := Apply(root, map[string][]byte{"sources/lucide/icons.json": []byte("bad"), "missing": nil}); err == nil {
		t.Fatal("missing output accepted")
	}
	if !equalSnapshots(before, snapshot(t, root)) {
		t.Fatal("staging failure changed files")
	}
	calls := 0
	err := apply(root, map[string][]byte{"sources/lucide/icons.json": []byte("new source"), "sources/lock.json": []byte("new lock")}, func(from, to string) error {
		calls++
		if calls == 2 {
			return fmt.Errorf("injected replacement failure")
		}
		return os.Rename(from, to)
	})
	if err == nil || calls != 2 {
		t.Fatal("replacement failure not exercised", err)
	}
	if !equalSnapshots(before, snapshot(t, root)) {
		t.Fatal("partial replacement was not rolled back")
	}
	name := "sources/lucide/icons.json"
	put(t, root, ".iconify-update/old/"+name, before[filepath.FromSlash(name)])
	put(t, root, ".iconify-update/files.json", marshal([]string{name}))
	put(t, root, name, []byte("interrupted"))
	if err := Apply(root, map[string][]byte{name: []byte("x")}); err == nil {
		t.Fatal("ignored interrupted journal")
	}
	if err := Recover(root); err != nil {
		t.Fatal(err)
	}
	if !equalSnapshots(before, snapshot(t, root)) {
		t.Fatal("recovery did not restore checkout")
	}
}
