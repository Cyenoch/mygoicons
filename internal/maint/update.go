// Package maint implements the network-enabled, maintainer-only collection update.
package maint

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Cyenoch/mygoicons/generate"
)

const maxDownload = 32 << 20
const maxExpanded = 96 << 20
const maxMetadata = 8 << 20

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

type Digest struct {
	SHA256      string `json:"sha256"`
	Bytes       int    `json:"bytes"`
	ArchivePath string `json:"archivePath,omitempty"`
}
type Notice struct {
	File               string `json:"file"`
	UpstreamRepository string `json:"upstreamRepository"`
	UpstreamRevision   string `json:"upstreamRevision"`
	UpstreamPath       string `json:"upstreamPath"`
	URL                string `json:"url"`
	SHA256             string `json:"sha256"`
	GitBlobSHA1        string `json:"gitBlobSHA1"`
	Bytes              int    `json:"bytes"`
}
type License struct {
	Metadata             json.RawMessage `json:"metadata"`
	NoticeCorrespondence string          `json:"noticeCorrespondence"`
	Notices              []Notice        `json:"notices"`
}
type Collection struct {
	Prefix              string            `json:"prefix"`
	Package             string            `json:"package"`
	Version             string            `json:"version"`
	RegistryMetadataURL string            `json:"registryMetadataURL"`
	TarballURL          string            `json:"tarballURL"`
	Integrity           string            `json:"integrity"`
	IntegrityVerified   bool              `json:"integrityVerified"`
	PackageGitHead      string            `json:"packageGitHead"`
	Files               map[string]Digest `json:"files"`
	Counts              struct {
		Icons   int `json:"icons"`
		Aliases int `json:"aliases"`
	} `json:"counts"`
	License License `json:"license"`
}
type Lock struct {
	SchemaVersion int          `json:"schemaVersion"`
	RetrievedAt   string       `json:"retrievedAt"`
	Collections   []Collection `json:"collections"`
}

// Updater's endpoints and client are injectable for tests. Production uses HTTPS.
type Updater struct {
	Root, Registry, GitHub string
	Client                 *http.Client
}

func New(root string) *Updater {
	return &Updater{Root: root, Registry: "https://registry.npmjs.org", GitHub: "https://api.github.com", Client: &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 || req.URL.Scheme != "https" {
			return errors.New("unsafe redirect")
		}
		return nil
	}}}
}

var errNotFound = errors.New("upstream file not found")

func (u *Updater) get(ctx context.Context, address string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mygoicons-maintainer")
	resp, err := u.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("GET %s: %w", address, errNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", address, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("GET %s exceeds %d bytes", address, limit)
	}
	return data, nil
}
func (u *Updater) load() (Lock, error) {
	data, err := os.ReadFile(filepath.Join(u.Root, "sources/lock.json"))
	if err != nil {
		return Lock{}, err
	}
	var lock Lock
	if err = json.Unmarshal(data, &lock); err != nil {
		return lock, err
	}
	if lock.SchemaVersion != 1 || len(lock.Collections) != 2 {
		return lock, errors.New("unsupported lock")
	}
	for _, c := range lock.Collections {
		if c.Prefix != "lucide" && c.Prefix != "twemoji" || c.Package != "@iconify-json/"+c.Prefix {
			return lock, errors.New("unexpected locked collection")
		}
		for name, d := range c.Files {
			if name != "icons.json" && name != "info.json" && name != "package.json" {
				return lock, errors.New("unexpected source file")
			}
			if err := verifyFile(filepath.Join(u.Root, "sources", c.Prefix, name), d.SHA256, d.Bytes); err != nil {
				return lock, err
			}
		}
		for _, n := range c.License.Notices {
			if !safeRelative(n.File) {
				return lock, errors.New("unsafe notice path")
			}
			if err := verifyFile(filepath.Join(u.Root, "sources", c.Prefix, n.File), n.SHA256, n.Bytes); err != nil {
				return lock, err
			}
		}
	}
	return lock, nil
}
func verifyFile(file, hash string, size int) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if len(data) != size || digest(data) != hash {
		return fmt.Errorf("%s differs from source lock", file)
	}
	return nil
}
func digest(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
func marshal(value any) []byte {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	return append(data, '\n')
}

// Versions queries npm's compact version manifest, avoiding the unbounded full packument.
func (u *Updater) Versions(ctx context.Context, prefix string) ([]string, error) {
	if prefix != "lucide" && prefix != "twemoji" {
		return nil, errors.New("unknown collection")
	}
	data, err := u.get(ctx, u.Registry+"/"+url.PathEscape("@iconify-json/"+prefix), maxMetadata)
	if err != nil {
		return nil, err
	}
	var metadata struct {
		Versions map[string]json.RawMessage `json:"versions"`
	}
	if err = json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}
	var versions []string
	for v := range metadata.Versions {
		if versionPattern.MatchString(v) {
			versions = append(versions, v)
		}
	}
	slices.SortFunc(versions, func(a, b string) int {
		var aa, bb [3]int
		fmt.Sscanf(a, "%d.%d.%d", &aa[0], &aa[1], &aa[2])
		fmt.Sscanf(b, "%d.%d.%d", &bb[0], &bb[1], &bb[2])
		for i := range 3 {
			if aa[i] < bb[i] {
				return -1
			}
			if aa[i] > bb[i] {
				return 1
			}
		}
		return 0
	})
	return versions, nil
}

// Plan verifies complete inputs, generates output in memory, and returns only
// changed files. Nothing in the checkout is modified, including on failure.
func (u *Updater) Plan(ctx context.Context, versions map[string]string, acceptLicense bool) (map[string][]byte, string, error) {
	lock, err := u.load()
	if err != nil {
		return nil, "", err
	}
	changes := map[string][]byte{}
	var report strings.Builder
	for i := range lock.Collections {
		c := &lock.Collections[i]
		v := versions[c.Prefix]
		if v == "" {
			data, err := u.get(ctx, u.Registry+"/"+url.PathEscape(c.Package)+"/latest", maxMetadata)
			if err != nil {
				return nil, report.String(), err
			}
			var meta struct {
				Version string `json:"version"`
			}
			if err = json.Unmarshal(data, &meta); err != nil {
				return nil, report.String(), err
			}
			v = meta.Version
		}
		if !versionPattern.MatchString(v) {
			return nil, report.String(), fmt.Errorf("invalid exact version %q", v)
		}
		if v == c.Version {
			fmt.Fprintf(&report, "%s %s: unchanged\n", c.Prefix, v)
			continue
		}
		old := *c
		fetched, files, err := u.Fetch(ctx, c.Prefix, v)
		if err != nil {
			return nil, report.String(), err
		}
		fetched.License.Notices = slices.Clone(old.License.Notices)
		fetched.License.NoticeCorrespondence = old.License.NoticeCorrespondence
		*c = fetched
		var data struct{ Icons, Aliases map[string]json.RawMessage }
		if err := json.Unmarshal(files["icons.json"], &data); err != nil {
			return nil, report.String(), err
		}
		manifestPath := filepath.Join("sources", c.Prefix, "exports.json")
		manifest, err := os.ReadFile(filepath.Join(u.Root, manifestPath))
		if err != nil {
			return nil, report.String(), err
		}
		var previous []generate.Export
		if err = json.Unmarshal(manifest, &previous); err != nil {
			return nil, report.String(), err
		}
		result, err := generate.Build(files["icons.json"], generate.Options{Package: c.Prefix, Previous: previous})
		if err != nil {
			return nil, report.String(), err
		}
		for name, raw := range files {
			changes[filepath.Join("sources", c.Prefix, name)] = raw
		}
		changes[manifestPath] = marshal(result.Exports)
		changes[filepath.Join(c.Prefix, "icons_gen.go")] = result.Source
		fmt.Fprintf(&report, "\n%s %s → %s\n", c.Prefix, old.Version, v)
		oldIcons, err := os.ReadFile(filepath.Join(u.Root, "sources", c.Prefix, "icons.json"))
		if err != nil {
			return nil, report.String(), err
		}
		var prior struct{ Icons, Aliases map[string]json.RawMessage }
		if err = json.Unmarshal(oldIcons, &prior); err != nil {
			return nil, report.String(), err
		}
		summarize(&report, "icons", keys(prior.Icons), keys(data.Icons))
		summarize(&report, "aliases", keys(prior.Aliases), keys(data.Aliases))
		oldNames, newNames := []string{}, []string{}
		for _, e := range previous {
			oldNames = append(oldNames, e.Name)
		}
		for _, e := range result.Exports {
			newNames = append(newNames, e.Name)
		}
		summarize(&report, "exports", oldNames, newNames)
		licenseChanged := !jsonEqual(old.License.Metadata, c.License.Metadata)
		// The npm data does not identify an exact upstream artwork revision. Fetch
		// complete notices from an immutable commit without asserting correspondence.
		repo := "lucide-icons/lucide"
		if c.Prefix == "twemoji" {
			repo = "jdecked/twemoji"
		}
		commitData, err := u.get(ctx, u.GitHub+"/repos/"+repo+"/commits/HEAD", maxMetadata)
		if err != nil {
			return nil, report.String(), err
		}
		var commit struct {
			SHA string `json:"sha"`
		}
		if err = json.Unmarshal(commitData, &commit); err != nil {
			return nil, report.String(), err
		}
		if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(commit.SHA) {
			return nil, report.String(), errors.New("invalid notice commit")
		}
		for j := range c.License.Notices {
			n := &c.License.Notices[j]
			payload, err := u.get(ctx, u.GitHub+"/repos/"+repo+"/contents/"+url.PathEscape(n.UpstreamPath)+"?ref="+commit.SHA, maxMetadata)
			if err != nil {
				return nil, report.String(), err
			}
			var blob struct{ SHA, Content, Encoding string }
			if err = json.Unmarshal(payload, &blob); err != nil {
				return nil, report.String(), err
			}
			if blob.Encoding != "base64" {
				return nil, report.String(), errors.New("unsupported GitHub content encoding")
			}
			raw, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(blob.Content, "\n", ""))
			if err != nil {
				return nil, report.String(), err
			}
			if len(raw) > 1<<20 {
				return nil, report.String(), errors.New("notice exceeds 1 MiB")
			}
			gitHash := sha1.Sum(append(fmt.Appendf(nil, "blob %d\x00", len(raw)), raw...))
			if fmt.Sprintf("%x", gitHash) != blob.SHA {
				return nil, report.String(), errors.New("notice Git blob hash mismatch")
			}
			if digest(raw) != n.SHA256 {
				licenseChanged = true
				fmt.Fprintf(&report, "notice changed: %s\n", n.File)
			}
			n.UpstreamRevision = commit.SHA
			n.URL = "https://raw.githubusercontent.com/" + repo + "/" + commit.SHA + "/" + n.UpstreamPath
			n.SHA256 = digest(raw)
			n.GitBlobSHA1 = blob.SHA
			n.Bytes = len(raw)
			changes[filepath.Join("sources", c.Prefix, n.File)] = raw
		}
		fmt.Fprintf(&report, "license metadata: %s → %s; notice correspondence remains unverified\n", bytes.TrimSpace(old.License.Metadata), bytes.TrimSpace(c.License.Metadata))
		if licenseChanged && !acceptLicense {
			return nil, report.String(), errors.New("license/notice changes require review; rerun with -accept-license after reviewing upstream files")
		}
	}
	if len(changes) > 0 {
		lock.RetrievedAt = time.Now().UTC().Format("2006-01-02")
		changes["sources/lock.json"] = marshal(lock)
		catalog, err := Catalog(u.Root, changes)
		if err != nil {
			return nil, report.String(), err
		}
		changes[filepath.Join("examples", "native", "catalog_gen.go")] = catalog
	}
	for file, data := range changes {
		old, err := os.ReadFile(filepath.Join(u.Root, file))
		if err == nil && bytes.Equal(old, data) {
			delete(changes, file)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, report.String(), err
		}
	}
	return changes, report.String(), nil
}
func jsonEqual(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return bytes.Equal(marshal(x), marshal(y))
}
func keys[V any](m map[string]V) []string {
	r := make([]string, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	return r
}
func summarize(w io.Writer, label string, old, new []string) {
	a, b := map[string]bool{}, map[string]bool{}
	for _, s := range old {
		a[s] = true
	}
	for _, s := range new {
		b[s] = true
	}
	var added, removed []string
	for s := range b {
		if !a[s] {
			added = append(added, s)
		}
	}
	for s := range a {
		if !b[s] {
			removed = append(removed, s)
		}
	}
	slices.Sort(added)
	slices.Sort(removed)
	fmt.Fprintf(w, "%s: +%d -%d\n  added: %s\n  removed: %s\n", label, len(added), len(removed), strings.Join(added, ", "), strings.Join(removed, ", "))
}
func safeRelative(name string) bool {
	return name != "" && !strings.ContainsAny(name, "\\:\x00") && !strings.HasPrefix(name, "/") && path.Clean(name) == name && name != ".." && !strings.HasPrefix(name, "../")
}
func unpack(archive []byte) (map[string][]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	limited := &io.LimitedReader{R: gz, N: maxExpanded + 1}
	tr := tar.NewReader(limited)
	files := map[string][]byte{}
	seen := map[string]bool{}
	count := 0
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		count++
		name := strings.TrimSuffix(h.Name, "/")
		if count > 128 || !safeRelative(name) || name != "package" && !strings.HasPrefix(name, "package/") {
			return nil, errors.New("unsafe or excessive archive entries")
		}
		if seen[name] {
			return nil, errors.New("duplicate archive entry")
		}
		seen[name] = true
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeDir {
			return nil, errors.New("archive links and special entries are forbidden")
		}
		if h.Size < 0 || h.Size > generate.MaxInputBytes {
			return nil, errors.New("archive entry exceeds limit")
		}
		if h.Typeflag == tar.TypeDir && h.Size != 0 {
			return nil, errors.New("invalid archive directory")
		}
		if name == "package/icons.json" || name == "package/info.json" || name == "package/package.json" {
			if name != "package/icons.json" && h.Size > maxMetadata {
				return nil, errors.New("metadata entry exceeds limit")
			}
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			files[path.Base(name)] = data
		}
	}
	// Drain to validate the gzip checksum and expansion bound, including trailing data.
	if _, err = io.Copy(io.Discard, limited); err != nil {
		return nil, err
	}
	if limited.N <= 0 {
		return nil, errors.New("expanded archive exceeds limit")
	}
	if len(files) != 3 {
		return nil, errors.New("archive missing required JSON files")
	}
	return files, nil
}
