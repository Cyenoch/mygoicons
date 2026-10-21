// Command catalog regenerates the development-only icon directory.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/Cyenoch/mygoicons/internal/maint"
	"os"
	"path/filepath"
)

func main() {
	check := flag.Bool("check", false, "check without writing")
	flag.Parse()
	data, err := maint.Catalog(".", nil)
	if err != nil {
		panic(err)
	}
	name := filepath.Join("examples", "native", "catalog_gen.go")
	old, err := os.ReadFile(name)
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}
	if bytes.Equal(data, old) {
		return
	}
	if *check {
		fmt.Fprintln(os.Stderr, "native catalog is stale")
		os.Exit(1)
	}
	if err = os.WriteFile(name, data, 0o644); err != nil {
		panic(err)
	}
}
