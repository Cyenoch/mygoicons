// Command catalog regenerates the development-only icon directory.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Cyenoch/mygoicons/internal/maint"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "catalog:", err)
		os.Exit(1)
	}
}

func run() error {
	check := flag.Bool("check", false, "check without writing")
	flag.Parse()
	data, err := maint.Catalog(".", nil)
	if err != nil {
		return err
	}
	name := filepath.Join("examples", "native", "catalog_gen.go")
	old, err := os.ReadFile(name)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if bytes.Equal(data, old) {
		return nil
	}
	if *check {
		return fmt.Errorf("native catalog is stale")
	}
	return os.WriteFile(name, data, 0o644)
}
