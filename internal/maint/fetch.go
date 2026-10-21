package maint

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var prefixPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Fetch downloads an exact npm collection, verifying its identity, integrity
// and archive limits. An empty version resolves the current release first.
func (u *Updater) Fetch(ctx context.Context, prefix, version string) (Collection, map[string][]byte, error) {
	c := Collection{Prefix: prefix, Package: "@iconify-json/" + prefix}
	if len(prefix) > 128 || !prefixPattern.MatchString(prefix) {
		return c, nil, errors.New("invalid collection name")
	}
	if version == "" {
		raw, err := u.get(ctx, u.Registry+"/"+url.PathEscape(c.Package)+"/latest", maxMetadata)
		if err != nil {
			return c, nil, err
		}
		var meta struct{ Version string }
		if err = json.Unmarshal(raw, &meta); err != nil {
			return c, nil, err
		}
		version = meta.Version
	}
	if !versionPattern.MatchString(version) {
		return c, nil, fmt.Errorf("use an exact stable version, got %q", version)
	}
	c.Version = version
	c.RegistryMetadataURL = u.Registry + "/" + url.PathEscape(c.Package) + "/" + version
	raw, err := u.get(ctx, c.RegistryMetadataURL, maxMetadata)
	if err != nil {
		return c, nil, err
	}
	var m struct {
		Name, Version, GitHead string
		Dist                   struct{ Tarball, Integrity string }
	}
	if err = json.Unmarshal(raw, &m); err != nil {
		return c, nil, err
	}
	if m.Name != c.Package || m.Version != version {
		return c, nil, errors.New("npm metadata identity mismatch")
	}
	address, err := url.Parse(m.Dist.Tarball)
	if err != nil {
		return c, nil, err
	}
	registry, err := url.Parse(u.Registry)
	if err != nil {
		return c, nil, err
	}
	if address.Host != registry.Host || address.Scheme != registry.Scheme || address.User != nil {
		return c, nil, errors.New("tarball must use registry origin")
	}
	archive, err := u.get(ctx, m.Dist.Tarball, maxDownload)
	if err != nil {
		return c, nil, err
	}
	expected, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(m.Dist.Integrity, "sha512-"))
	actual := sha512.Sum512(archive)
	if err != nil || !strings.HasPrefix(m.Dist.Integrity, "sha512-") || !bytes.Equal(expected, actual[:]) {
		return c, nil, errors.New("npm SHA-512 integrity mismatch")
	}
	files, err := unpack(archive)
	if err != nil {
		return c, nil, err
	}
	var pkg struct{ Name, Version string }
	if err = json.Unmarshal(files["package.json"], &pkg); err != nil {
		return c, nil, err
	}
	if pkg.Name != m.Name || pkg.Version != m.Version {
		return c, nil, errors.New("archive package identity mismatch")
	}
	var info struct{ License json.RawMessage }
	if err = json.Unmarshal(files["info.json"], &info); err != nil {
		return c, nil, err
	}
	if len(info.License) == 0 || bytes.Equal(info.License, []byte("null")) {
		return c, nil, errors.New("missing artwork license metadata")
	}
	var icons struct {
		Prefix         string
		Icons, Aliases map[string]json.RawMessage
	}
	if err = json.Unmarshal(files["icons.json"], &icons); err != nil {
		return c, nil, err
	}
	if icons.Prefix != prefix {
		return c, nil, errors.New("collection prefix mismatch")
	}
	c.TarballURL = m.Dist.Tarball
	c.Integrity = m.Dist.Integrity
	c.IntegrityVerified = true
	c.PackageGitHead = m.GitHead
	c.License.Metadata = info.License
	c.Files = map[string]Digest{}
	for name, data := range files {
		c.Files[name] = Digest{SHA256: digest(data), Bytes: len(data), ArchivePath: "package/" + name}
	}
	c.Counts.Icons = len(icons.Icons)
	c.Counts.Aliases = len(icons.Aliases)
	return c, files, nil
}
