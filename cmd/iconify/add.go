package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Cyenoch/mygoicons/generate"
	"github.com/Cyenoch/mygoicons/internal/maint"
)

var collectionName = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

type importedSource struct {
	maint.Collection
	Icons []string `json:"selection,omitempty"`
}

func add(args []string, stdout, stderr io.Writer, u *maint.Updater) error {
	flags := flag.NewFlagSet("iconify add", flag.ContinueOnError)
	flags.SetOutput(stderr)
	icons := flags.String("icons", "", "comma-separated original IDs; empty selects all")
	out := flags.String("out", "", "package directory; default internal/<collection>")
	check := flags.Bool("check", false, "check saved inputs and output offline; never download or write")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: iconify add <collection>[@version] [-icons id,id] [-out directory] [-check]")
		flags.PrintDefaults()
	}
	if len(args) == 0 {
		return errors.New("use iconify add mingcute or iconify add mingcute@1.2.8")
	}
	if strings.HasPrefix(args[0], "-") {
		if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return errors.New("put the collection before flags")
	}
	prefix, version, pinned := strings.Cut(args[0], "@")
	if !collectionName.MatchString(prefix) || len(prefix) > 128 || pinned && version == "" {
		return errors.New("invalid collection; use mingcute or mingcute@1.2.8")
	}
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	if *out == "" {
		*out = filepath.Join("internal", prefix)
	}
	dir, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	pkg := strings.ReplaceAll(prefix, "-", "")
	var previous []generate.Export
	var lock importedSource
	files := map[string][]byte{}
	stat, statErr := os.Lstat(dir)
	if statErr == nil {
		if !stat.IsDir() || stat.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s must be a regular directory", *out)
		}
		root, err := os.OpenRoot(dir)
		if err != nil {
			return err
		}
		defer root.Close()
		raw, err := root.ReadFile("source.json")
		if err != nil {
			return fmt.Errorf("refusing to overwrite %s without its source.json", *out)
		}
		if err = json.Unmarshal(raw, &lock); err != nil {
			return err
		}
		if lock.Prefix != prefix || lock.Package != "@iconify-json/"+prefix || len(lock.Files) != 3 || len(lock.License.Notices) == 0 {
			return errors.New("saved collection identity mismatch")
		}
		for _, name := range []string{"icons.json", "info.json", "package.json"} {
			d, ok := lock.Files[name]
			if !ok {
				return errors.New("incomplete saved source")
			}
			raw, err := root.ReadFile(filepath.Join("source", name))
			if err != nil {
				return err
			}
			if len(raw) != d.Bytes || fmt.Sprintf("%x", sha256.Sum256(raw)) != d.SHA256 {
				return fmt.Errorf("saved %s checksum mismatch", name)
			}
			files[filepath.Join("source", name)] = raw
		}
		for _, n := range lock.License.Notices {
			if n.File != "LICENSE" && n.File != "NOTICE" {
				return errors.New("unexpected saved notice path")
			}
			raw, err := root.ReadFile(n.File)
			if err != nil {
				return err
			}
			if len(raw) != n.Bytes || fmt.Sprintf("%x", sha256.Sum256(raw)) != n.SHA256 {
				return errors.New("saved notice checksum mismatch")
			}
			files[n.File] = raw
		}
		raw, err = root.ReadFile("icons_gen.go")
		if err != nil {
			return err
		}
		previous, err = generate.ReadExports(raw)
		if err != nil {
			return err
		}
		files["icons_gen.go"] = raw
		if err := root.Close(); err != nil {
			return err
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	if version == "" && statErr == nil {
		version = lock.Version
	}
	if *check && (statErr != nil || version != lock.Version) {
		return errors.New("saved collection is missing or version differs; run add without -check")
	}
	if statErr != nil || version != lock.Version {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		fetched, inputs, err := u.Fetch(ctx, prefix, version)
		if err != nil {
			return err
		}
		notices, err := u.Notices(ctx, &fetched)
		if err != nil {
			return err
		}
		if statErr == nil {
			if !bytes.Equal(compactJSON(lock.License.Metadata), compactJSON(fetched.License.Metadata)) {
				fmt.Fprintf(stderr, "iconify: license metadata changed: %s → %s\n", compactJSON(lock.License.Metadata), compactJSON(fetched.License.Metadata))
			}
			noticesChanged := len(notices) != len(lock.License.Notices)
			for _, old := range lock.License.Notices {
				if !bytes.Equal(files[old.File], notices[old.File]) {
					noticesChanged = true
				}
			}
			if noticesChanged {
				fmt.Fprintln(stderr, "iconify: license notices changed; see LICENSE, NOTICE and source.json")
			}
		}
		lock.Collection = fetched
		files = notices
		for name, raw := range inputs {
			files[filepath.Join("source", name)] = raw
		}
	}
	options := generate.Options{Package: pkg, Previous: previous, Icons: lock.Icons}
	if *icons != "" {
		options.Icons = nil
		for name := range strings.SplitSeq(*icons, ",") {
			options.Icons = append(options.Icons, strings.TrimSpace(name))
		}
	}
	lock.Icons = options.Icons
	result, err := generate.Build(files[filepath.Join("source", "icons.json")], options)
	if err != nil {
		return err
	}
	files["icons_gen.go"] = result.Source
	raw, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	files["source.json"] = append(raw, '\n')
	attributes, err := os.ReadFile(filepath.Join(dir, ".gitattributes"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(attributes) == 0 {
		attributes = []byte("/source/** -text\n/LICENSE -text\n/NOTICE -text\n")
	}
	files[".gitattributes"] = attributes
	unchanged := statErr == nil
	for name, raw := range files {
		old, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if !bytes.Equal(old, raw) {
			unchanged = false
		}
	}
	if unchanged {
		fmt.Fprintf(stdout, "%s@%s: %d icons, unchanged\n", prefix, lock.Version, result.Count)
		return nil
	}
	if *check {
		return errors.New("generated package is stale; rerun add without -check")
	}
	if err := replacePackage(dir, files, statErr == nil); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%s@%s: %d icons → %s\n", prefix, lock.Version, result.Count, *out)
	return nil
}

func compactJSON(raw []byte) []byte {
	var out bytes.Buffer
	if json.Compact(&out, raw) != nil {
		return raw
	}
	return out.Bytes()
}

func replacePackage(dir string, files map[string][]byte, exists bool) error {
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	// A fixed backup survives interruption between the two directory renames.
	backup := dir + ".iconify-backup"
	if _, err := os.Lstat(backup); err == nil {
		return fmt.Errorf("interrupted import: restore %s before retrying", backup)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".iconify-add-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, 0o755); err != nil {
		return err
	}
	if exists {
		if err := os.CopyFS(stage, os.DirFS(dir)); err != nil {
			return err
		}
	}
	for name, raw := range files {
		target := filepath.Join(stage, name)
		// CopyFS preserves links. Never write generated data through one.
		for _, path := range []string{filepath.Dir(target), target} {
			info, err := os.Lstat(path)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if err == nil && info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("generated path is a symlink: %s", path)
			}
		}
		if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err = os.WriteFile(target, raw, 0o644); err != nil {
			return err
		}
	}
	if exists {
		if err = os.Rename(dir, backup); err != nil {
			return err
		}
	}
	if err = os.Rename(stage, dir); err != nil {
		if exists {
			err = errors.Join(err, os.Rename(backup, dir))
		}
		return err
	}
	if exists {
		return os.RemoveAll(backup)
	}
	return nil
}
