// Command measure records source, compilation and executable costs without thresholds.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Cyenoch/mygoicons/generate"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	output := flag.String("out", "measurements.json", "JSON measurement file")
	flag.Parse()
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	temp, err := os.MkdirTemp("", "mygoicons-measure-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	result := map[string]any{"go": runtime.Version(), "platform": runtime.GOOS + "/" + runtime.GOARCH, "note": "Cold means a new build cache; dependencies already downloaded. Timings are observations, never CI thresholds."}
	sizes := map[string]int64{}
	for _, name := range []string{"lucide", "twemoji", "sources"} {
		var size int64
		err = filepath.WalkDir(name, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				info, err := d.Info()
				if err != nil {
					return err
				}
				size += info.Size()
			}
			return nil
		})
		if err != nil {
			return err
		}
		sizes[name] = size
	}
	result["source_bytes"] = sizes
	for _, name := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		if name == "go.mod" {
			data = []byte(strings.Replace(string(data), "module github.com/Cyenoch/mygoicons", "module example.com/iconmeasure", 1))
		}
		if err = os.WriteFile(filepath.Join(temp, name), data, 0644); err != nil {
			return err
		}
	}
	if err = os.Mkdir(filepath.Join(temp, "glyphs"), 0755); err != nil {
		return err
	}
	input, err := os.ReadFile("sources/lucide/icons.json")
	if err != nil {
		return err
	}
	subset, err := generate.Build(input, generate.Options{Package: "glyphs", Icons: []string{"search"}})
	if err != nil {
		return err
	}
	result["subset_source_bytes"] = len(subset.Source)
	full, err := os.ReadFile("lucide/icons_gen.go")
	if err != nil {
		return err
	}
	main := []byte("package main\nimport(\"fmt\";\"example.com/iconmeasure/glyphs\")\nfunc main(){fmt.Println(glyphs.Search().Size())}\n")
	if err = os.WriteFile(filepath.Join(temp, "main.go"), main, 0644); err != nil {
		return err
	}
	observations := map[string]any{}
	for _, p := range []struct {
		name   string
		source []byte
	}{{"full", full}, {"subset", subset.Source}} {
		source := strings.Replace(string(p.source), "package lucide", "package glyphs", 1)
		if err = os.WriteFile(filepath.Join(temp, "glyphs/icons_gen.go"), []byte(source), 0644); err != nil {
			return err
		}
		env := append(os.Environ(), "GOENV=off", "GOWORK=off", "GOFLAGS=", "CGO_ENABLED=0", "GOCACHE="+filepath.Join(temp, "cache-"+p.name), "GOPROXY=off", "GOSUMDB=off")
		cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", filepath.Join(temp, p.name+".exe"), ".")
		cmd.Dir = temp
		cmd.Env = env
		start := time.Now()
		log, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("build %s: %w\n%s", p.name, err, log)
		}
		elapsed := time.Since(start)
		stat, err := os.Stat(filepath.Join(temp, p.name+".exe"))
		if err != nil {
			return err
		}
		observations[p.name] = map[string]any{"cold_build_seconds": elapsed.Seconds(), "executable_bytes": stat.Size()}
	}
	result["one_referenced_icon"] = observations
	// Benchmark the actual bundled accessor and a fresh parse separately.
	cmd := exec.Command("go", "test", ".", "-run=^$", "-bench=Benchmark(CachedIcon|FirstParse)$", "-benchmem", "-count=3")
	cmd.Dir = root
	log, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("benchmarks: %w\n%s", err, log)
	}
	result["runtime_benchmarks"] = string(log)
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(*output, append(data, '\n'), 0644); err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
