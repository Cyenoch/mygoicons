package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Cyenoch/mygoicons/internal/maint"
)

func testImporter(t *testing.T, failure *string) (*maint.Updater, *int) {
	t.Helper()
	requests := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		writeJSON := func(value any) {
			t.Helper()
			if err := json.NewEncoder(w).Encode(value); err != nil {
				t.Error(err)
			}
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/latest"):
			writeJSON(map[string]string{"version": "1.2.8"})
		case strings.Contains(r.URL.Path, "/@iconify-json/"):
			version := filepath.Base(r.URL.Path)
			icons := `{"prefix":"mingcute","icons":{"search-line":{"body":"<path d=\"M0 0h1\"/>"},"home-1-line":{"body":"<circle r=\"2\"/>"}}}`
			if *failure == "compatibility" {
				icons = `{"prefix":"mingcute","icons":{"other":{"body":"<rect width=\"3\" height=\"2\"/>"}}}`
			}
			var b bytes.Buffer
			gz := gzip.NewWriter(&b)
			tw := tar.NewWriter(gz)
			files := map[string]string{
				"icons.json":   icons,
				"info.json":    `{"license":{"spdx":"Apache-2.0","url":"https://github.com/test/icons/blob/main/LICENSE"}}`,
				"package.json": fmt.Sprintf(`{"name":"@iconify-json/mingcute","version":%q}`, version),
			}
			if *failure == "license-metadata" {
				files["info.json"] = `{"license":{"spdx":"ISC","url":"https://github.com/test/icons/blob/main/LICENSE"}}`
			}
			if *failure == "unsupported-license" {
				files["info.json"] = `{"license":{"spdx":"Apache-2.0","url":"https://example.com/LICENSE"}}`
			}
			if *failure == "svg" {
				files["icons.json"] = `{"prefix":"mingcute","icons":{"search-line":{"body":"<script/>"}}}`
			}
			for _, name := range []string{"icons.json", "info.json", "package.json"} {
				data := files[name]
				if err := tw.WriteHeader(&tar.Header{Name: "package/" + name, Size: int64(len(data)), Mode: 0644}); err != nil {
					t.Error(err)
				}
				if _, err := tw.Write([]byte(data)); err != nil {
					t.Error(err)
				}
			}
			if err := tw.Close(); err != nil {
				t.Error(err)
			}
			if err := gz.Close(); err != nil {
				t.Error(err)
			}
			archive := b.Bytes()
			hash := sha512.Sum512(archive)
			integrity := "sha512-" + base64.StdEncoding.EncodeToString(hash[:])
			if *failure == "integrity" {
				integrity = "sha512-AAAA"
			}
			if r.URL.Query().Get("tarball") == "1" {
				w.Write(archive)
				return
			}
			writeJSON(map[string]any{"name": "@iconify-json/mingcute", "version": version, "dist": map[string]string{"tarball": server.URL + r.URL.Path + "?tarball=1", "integrity": integrity}})
		case strings.Contains(r.URL.Path, "/commits/"):
			writeJSON(map[string]string{"sha": strings.Repeat("1", 40)})
		case strings.Contains(r.URL.Path, "/contents/NOTICE"):
			http.NotFound(w, r)
		case strings.Contains(r.URL.Path, "/contents/LICENSE"):
			if *failure == "notice-download" {
				http.Error(w, "unavailable", 500)
				return
			}
			data := []byte("fixture license\n")
			if *failure == "license-body" {
				data = []byte("changed license\n")
			}
			hash := sha1.Sum(append(fmt.Appendf(nil, "blob %d\x00", len(data)), data...))
			sum := fmt.Sprintf("%x", hash)
			if *failure == "notice-hash" {
				sum = strings.Repeat("0", 40)
			}
			writeJSON(map[string]string{"sha": sum, "encoding": "base64", "content": base64.StdEncoding.EncodeToString(data)})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	u := maint.New(".")
	u.Registry = server.URL
	u.GitHub = server.URL
	u.Client = server.Client()
	return u, &requests
}

func packageSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[name] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestAddOfflineLifecycle(t *testing.T) {
	t.Chdir(t.TempDir())
	failure := ""
	u, requests := testImporter(t, &failure)
	var output bytes.Buffer
	call := func(args ...string) error { return add(args, &output, &output, u) }
	if err := call("mingcute", "-icons", "search-line"); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join("internal", "mingcute")
	before := packageSnapshot(t, dir)
	if len(before) != 7 || !strings.Contains(before["icons_gen.go"], "func SearchLine()") || strings.Contains(before["icons_gen.go"], "func Home1Line()") {
		t.Fatal("incorrect imported package", before)
	}
	count := *requests
	// Saved version and selection let both normal regeneration and -check stay offline.
	for _, args := range [][]string{{"mingcute"}, {"mingcute", "-check"}, {"mingcute@1.2.8", "-check"}} {
		if err := call(args...); err != nil {
			t.Fatal(err)
		}
	}
	if *requests != count || !reflect.DeepEqual(before, packageSnapshot(t, dir)) {
		t.Fatal("repeat import changed files or used network")
	}
	if err := call("mingcute@1.2.9", "-check"); err == nil || *requests != count {
		t.Fatal("check fetched a new version")
	}
	if err := call("mingcute", "-icons", "search-line,home-1-line"); err != nil {
		t.Fatal(err)
	}
	if *requests != count {
		t.Fatal("selection update used network")
	}
	before = packageSnapshot(t, dir)
	if err := call("mingcute", "-icons", "home-1-line"); err == nil {
		t.Fatal("silently removed existing accessor")
	}
	if !reflect.DeepEqual(before, packageSnapshot(t, dir)) {
		t.Fatal("compatibility failure changed package")
	}
	// Source corruption must fail rather than silently downloading a replacement.
	input := filepath.Join(dir, "source", "icons.json")
	if err := os.WriteFile(input, []byte("corrupt"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := call("mingcute", "-check"); err == nil || *requests != count {
		t.Fatal("corrupt source accepted or fetched")
	}
}

func TestAddFailureLeavesPackageUnchanged(t *testing.T) {
	for _, scenario := range []string{"integrity", "notice-download", "notice-hash", "unsupported-license", "svg", "compatibility"} {
		t.Run(scenario, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "mingcute")
			failure := ""
			u, _ := testImporter(t, &failure)
			var output bytes.Buffer
			args := []string{"mingcute@1.2.8", "-icons", "search-line", "-out", dir}
			if err := add(args, &output, &output, u); err != nil {
				t.Fatal(err)
			}
			failure = scenario
			if err := os.WriteFile(filepath.Join(dir, "custom.go"), []byte("package mingcute\n"), 0644); err != nil {
				t.Fatal(err)
			}
			before := packageSnapshot(t, dir)
			args[0] = "mingcute@1.2.9"
			err := add(args, &output, &output, u)
			expected := map[string]string{
				"integrity": "integrity mismatch", "notice-download": "500", "notice-hash": "blob hash mismatch",
				"unsupported-license": "cannot automatically retain", "svg": "unsupported", "compatibility": "unknown icon or alias",
			}[scenario]
			if err == nil || !strings.Contains(err.Error(), expected) {
				t.Fatalf("%s: want %q, got %v", scenario, expected, err)
			}
			if !reflect.DeepEqual(before, packageSnapshot(t, dir)) {
				t.Fatal("failed import changed package")
			}
			if _, err := os.Stat(dir + ".iconify-backup"); !os.IsNotExist(err) {
				t.Fatal("failure left a backup", err)
			}
		})
	}
}

func TestAddPreservesUserFilesAndReportsLicenseChanges(t *testing.T) {
	for _, scenario := range []string{"unchanged-license", "license-metadata", "license-body"} {
		t.Run(scenario, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "mingcute")
			failure := ""
			u, requests := testImporter(t, &failure)
			var output, warnings bytes.Buffer
			args := []string{"mingcute@1.2.8", "-icons", "search-line", "-out", dir}
			if err := add(args, &output, &warnings, u); err != nil {
				t.Fatal(err)
			}
			userFiles := map[string]string{
				"custom.go":        "package mingcute\n",
				"notes/readme.txt": "My notes\n",
				".gitattributes":   "/source/** -text\n/LICENSE -text\n/NOTICE -text\n*.txt text\n",
			}
			for name, content := range userFiles {
				path := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			count := *requests
			if err := add([]string{"mingcute", "-out", dir, "-check"}, &output, &warnings, u); err != nil {
				t.Fatal(err)
			}
			if *requests != count {
				t.Fatal("offline check used network")
			}
			failure = scenario
			args[0] = "mingcute@1.2.9"
			if err := add(args, &output, &warnings, u); err != nil {
				t.Fatal(err)
			}
			for name, content := range userFiles {
				raw, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil || string(raw) != content {
					t.Fatalf("user file %s changed: %v", name, err)
				}
			}
			if scenario != "unchanged-license" && !strings.Contains(warnings.String(), "changed") {
				t.Fatal("license change was not reported")
			}
			count = *requests
			if err := add([]string{"mingcute", "-out", dir, "-check"}, &output, &warnings, u); err != nil {
				t.Fatal(err)
			}
			if *requests != count {
				t.Fatal("updated package check used network")
			}
		})
	}
}

func TestReplacePackageDoesNotWriteThroughLinks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mingcute")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "user.txt")
	if err := os.WriteFile(outside, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "LICENSE")); err != nil {
		t.Skip("symlink unavailable:", err)
	}
	if err := replacePackage(dir, map[string][]byte{"LICENSE": []byte("generated")}, true); err == nil {
		t.Fatal("generated output followed symlink")
	}
	raw, err := os.ReadFile(outside)
	if err != nil || string(raw) != "keep" {
		t.Fatal("user data changed", err)
	}
}

func TestAddRejectsInvalidInputWithoutNetwork(t *testing.T) {
	failure := ""
	u, requests := testImporter(t, &failure)
	var output bytes.Buffer
	for _, args := range [][]string{{}, {"../mingcute"}, {"mingcute@"}, {"mingcute@latest"}, {"mingcute@1.2.8", "extra"}, {"mingcute@1.2.8", "-check", "-out", filepath.Join(t.TempDir(), "missing")}} {
		if err := add(args, &output, &output, u); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	if *requests != 0 {
		t.Fatal("invalid arguments used network")
	}
	if err := add([]string{"-help"}, &output, &output, u); err != nil {
		t.Fatal(err)
	}
}
