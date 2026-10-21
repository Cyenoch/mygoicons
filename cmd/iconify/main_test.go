package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestOutputLifecycle(t *testing.T) {
	dir := t.TempDir()
	input, output := filepath.Join(dir, "icons.json"), filepath.Join(dir, "set", "icons_gen.go")
	good := []byte(`{"prefix":"demo","icons":{"search":{"body":"<path d=\"M0 0h2\"/>"}}}`)
	if err := os.WriteFile(input, good, 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"-input", input, "-out", output, "-package", "demo"}
	var messages bytes.Buffer
	call := func(extra ...string) error {
		return run(append(append([]string{}, args...), extra...), &messages, &messages)
	}
	if err := call("-check"); err == nil {
		t.Fatal("missing output passed check")
	}
	if _, err := os.Stat(filepath.Dir(output)); !os.IsNotExist(err) {
		t.Fatal("check created output directory")
	}
	if err := call(); err != nil {
		t.Fatal(err)
	}
	generated, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := call("-check"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, []byte(`{"prefix":"demo","icons":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := call(); err == nil {
		t.Fatal("invalid source accepted")
	}
	after, err := os.ReadFile(output)
	if err != nil || !bytes.Equal(generated, after) {
		t.Fatal("invalid input changed output", err)
	}
	if err := os.WriteFile(input, good, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("package demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := call(); err == nil {
		t.Fatal("overwrote handwritten source")
	}
	if err := call("-check"); err == nil {
		t.Fatal("stale source passed check")
	}
}
