// Command iconify imports Iconify collections or generates from local JSON.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cyenoch/mygoicons/generate"
	"github.com/Cyenoch/mygoicons/internal/maint"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "iconify:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 && args[0] == "add" {
		return add(args[1:], stdout, stderr, maint.New("."))
	}
	flags := flag.NewFlagSet("iconify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: iconify add <collection>[@version] [-icons id,id] [-out directory]\n       iconify -input icons.json -out icons_gen.go [-package name]")
		flags.PrintDefaults()
	}
	input := flags.String("input", "", "local Iconify icons.json (required)")
	output := flags.String("out", "", "generated Go file (required)")
	pkg := flags.String("package", "", "Go package name; default: the name of -out's directory")
	icons := flags.String("icons", "", "comma-separated original icon names; empty selects all")
	manifest := flags.String("manifest", "", "export compatibility manifest to preserve (optional)")
	check := flags.Bool("check", false, "fail if the output is missing or differs; never write")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q; see -help", flags.Arg(0))
	}
	var missing []string
	if *input == "" {
		missing = append(missing, "-input")
	}
	if *output == "" {
		missing = append(missing, "-out")
	}
	if len(missing) != 0 {
		return fmt.Errorf("missing %s; use -input icons.json -out icons_gen.go, see -help", strings.Join(missing, " and "))
	}
	if !strings.HasSuffix(*output, ".go") || strings.HasSuffix(*output, "_test.go") {
		return errors.New("-out must be a non-test Go source file")
	}
	if *pkg == "" {
		absolute, err := filepath.Abs(*output)
		if err != nil {
			return err
		}
		*pkg = filepath.Base(filepath.Dir(absolute))
	}
	source, err := readBounded(*input, generate.MaxInputBytes)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	options := generate.Options{Package: *pkg, Icons: iconList(*icons)}
	existing, err := readBounded(*output, generate.MaxOutputBytes)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read output: %w", err)
	}
	if err == nil {
		options.Previous, err = generate.ReadExports(existing)
		if err != nil {
			return fmt.Errorf("read existing exports: %w", err)
		}
	}
	if *manifest != "" {
		data, readErr := readBounded(*manifest, generate.MaxInputBytes)
		if readErr != nil {
			return fmt.Errorf("read manifest: %w", readErr)
		}
		var previous []generate.Export
		if err := json.Unmarshal(data, &previous); err != nil {
			return fmt.Errorf("manifest %s: %w", *manifest, err)
		}
		options.Previous = append(options.Previous, previous...)
	}
	result, buildErr := generate.Build(source, options)
	if buildErr != nil {
		return buildErr
	}
	if bytes.Equal(existing, result.Source) {
		fmt.Fprintf(stdout, "%s: %d icons, unchanged\n", result.Prefix, result.Count)
		return nil
	}
	if *check {
		return fmt.Errorf("%s is stale or missing; regenerate it", *output)
	}
	if err := writeSource(*output, result.Source); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%s: generated %d icons in %s\n", result.Prefix, result.Count, *output)
	return nil
}

// iconList splits a comma-separated -icons value, ignoring blanks.
func iconList(value string) []string {
	var names []string
	for name := range strings.SplitSeq(value, ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func readBounded(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds %d bytes", path, limit)
	}
	return data, nil
}

func writeSource(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".iconify-*.go")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Chmod(0o644); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("replace output: %w", err)
	}
	return nil
}
