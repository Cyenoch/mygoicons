package maint

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// Apply stages every output before replacing any file. On an ordinary I/O
// failure it restores replaced files. A durable journal survives interruption;
// the next invocation refuses updates until Recover restores the checkout.
func Apply(root string, changes map[string][]byte) error {
	return apply(root, changes, os.Rename)
}

func apply(root string, changes map[string][]byte, replace func(string, string) error) error {
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		return nil
	}
	journal := filepath.Join(root, ".iconify-update")
	if err := os.Mkdir(journal, 0o700); err != nil {
		return fmt.Errorf("update journal exists or cannot be created (run -recover): %w", err)
	}
	keep := false
	defer func() {
		if !keep {
			os.RemoveAll(journal)
		}
	}()
	files := make([]string, 0, len(changes))
	for name := range changes {
		if !safeRelative(filepath.ToSlash(name)) {
			return errors.New("unsafe output path")
		}
		files = append(files, name)
	}
	slices.Sort(files)
	for _, name := range files {
		target := filepath.Join(root, name)
		info, err := os.Lstat(target)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("output %s is not regular", name)
		}
		// All updated repository files already exist. Refuse symlinked parents too.
		for parent := filepath.Dir(target); parent != filepath.Clean(root) && parent != "."; parent = filepath.Dir(parent) {
			info, err := os.Lstat(parent)
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return errors.New("symlinked output parent")
			}
		}
		old, err := os.ReadFile(target)
		if err != nil {
			return err
		}
		backup := filepath.Join(journal, "old", name)
		next := filepath.Join(journal, "new", name)
		if err = os.MkdirAll(filepath.Dir(backup), 0o700); err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(next), 0o700); err != nil {
			return err
		}
		if err = os.WriteFile(backup, old, 0o644); err != nil {
			return err
		}
		if err = os.WriteFile(next, changes[name], 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(journal, "files.json"), marshal(files), 0o600); err != nil {
		return err
	}
	keep = true
	for _, name := range files {
		if err := replace(filepath.Join(journal, "new", name), filepath.Join(root, name)); err != nil {
			recovery := Recover(root)
			return errors.Join(err, recovery)
		}
	}
	if err := os.RemoveAll(journal); err != nil {
		return err
	}
	keep = false
	return nil
}

// Recover restores all files in an interrupted update before deleting its journal.
func Recover(root string) error {
	journal := filepath.Join(root, ".iconify-update")
	raw, err := os.ReadFile(filepath.Join(journal, "files.json"))
	if err != nil {
		return err
	}
	var files []string
	if err = json.Unmarshal(raw, &files); err != nil {
		return err
	}
	for _, name := range files {
		if !safeRelative(filepath.ToSlash(name)) {
			return errors.New("unsafe journal path")
		}
		old, err := os.ReadFile(filepath.Join(journal, "old", name))
		if err != nil {
			return err
		}
		temp, err := os.CreateTemp(filepath.Dir(filepath.Join(root, name)), ".iconify-recover-*")
		if err != nil {
			return err
		}
		_, writeErr := temp.Write(old)
		chmodErr := temp.Chmod(0o644)
		closeErr := temp.Close()
		if err = errors.Join(writeErr, chmodErr, closeErr); err != nil {
			os.Remove(temp.Name())
			return err
		}
		if err = os.Rename(temp.Name(), filepath.Join(root, name)); err != nil {
			os.Remove(temp.Name())
			return err
		}
	}
	return os.RemoveAll(journal)
}
