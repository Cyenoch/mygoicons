package maint

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var revisionPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Notices retains license text at an immutable upstream commit. Packages whose
// license URL cannot identify a GitHub file require the local-input workflow.
func (u *Updater) Notices(ctx context.Context, c *Collection) (map[string][]byte, error) {
	var meta struct{ URL string }
	if err := json.Unmarshal(c.License.Metadata, &meta); err != nil {
		return nil, err
	}
	address, err := url.Parse(meta.URL)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.Trim(address.Path, "/"), "/")
	if address.Scheme != "https" || address.Host != "github.com" || len(parts) < 5 || parts[2] != "blob" {
		return nil, fmt.Errorf("cannot automatically retain license at %q; use local -input with the collection's complete notices", meta.URL)
	}
	repo := parts[0] + "/" + parts[1]
	ref := parts[3]
	file := strings.Join(parts[4:], "/")
	if !safeRelative(repo) || !safeRelative(file) {
		return nil, errors.New("unsafe upstream license path")
	}
	raw, err := u.get(ctx, u.GitHub+"/repos/"+repo+"/commits/"+url.PathEscape(ref), maxMetadata)
	if err != nil {
		return nil, err
	}
	var commit struct{ SHA string }
	if err = json.Unmarshal(raw, &commit); err != nil {
		return nil, err
	}
	if !revisionPattern.MatchString(commit.SHA) {
		return nil, errors.New("invalid notice commit")
	}
	c.License.NoticeCorrespondence = "Notice snapshots are pinned independently; exact artwork revision correspondence is not established."
	files := map[string][]byte{}
	// Preserve an applicable NOTICE file when the upstream repository provides one.
	for _, name := range []string{file, "NOTICE"} {
		if name == "NOTICE" && file == "NOTICE" && len(files) > 0 {
			continue
		}
		raw, err := u.get(ctx, u.GitHub+"/repos/"+repo+"/contents/"+url.PathEscape(name)+"?ref="+commit.SHA, maxMetadata)
		if name == "NOTICE" && name != file && errors.Is(err, errNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var blob struct{ SHA, Content, Encoding string }
		if err = json.Unmarshal(raw, &blob); err != nil {
			return nil, err
		}
		if blob.Encoding != "base64" {
			return nil, errors.New("unsupported notice encoding")
		}
		text, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(blob.Content, "\n", ""))
		if err != nil {
			return nil, err
		}
		if len(text) == 0 || len(text) > 1<<20 {
			return nil, errors.New("notice must contain 1 byte to 1 MiB")
		}
		// Git's object ID uses SHA-1; provenance additionally records SHA-256.
		hash := sha1.Sum(append(fmt.Appendf(nil, "blob %d\x00", len(text)), text...))
		if fmt.Sprintf("%x", hash) != blob.SHA {
			return nil, errors.New("notice Git blob hash mismatch")
		}
		local := "LICENSE"
		if name == "NOTICE" {
			local = "NOTICE"
		}
		files[local] = text
		c.License.Notices = append(c.License.Notices, Notice{File: local, UpstreamRepository: "https://github.com/" + repo, UpstreamRevision: commit.SHA, UpstreamPath: name, URL: "https://raw.githubusercontent.com/" + repo + "/" + commit.SHA + "/" + name, SHA256: digest(text), GitBlobSHA1: blob.SHA, Bytes: len(text)})
	}
	return files, nil
}
