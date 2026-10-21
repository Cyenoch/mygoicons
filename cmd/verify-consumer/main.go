// Command verify-consumer exercises remote installation without workspace overrides.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	version := flag.String("version", "v0.2.1", "published tag or full commit")
	addCollection := flag.Bool("add", false, "also verify one-command MingCute import and offline check")
	flag.Parse()
	if !regexp.MustCompile(`^(v[0-9]+\.[0-9]+\.[0-9]+|[0-9a-f]{40})$`).MatchString(*version) {
		return fmt.Errorf("use a fixed release or full commit")
	}
	dir, err := os.MkdirTemp("", "mygoicons-consumer-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	cache := filepath.Join(dir, "modcache")
	env := []string{}
	for _, e := range os.Environ() {
		key, _, _ := strings.Cut(e, "=")
		switch strings.ToUpper(key) {
		case "GOENV", "GOWORK", "GOFLAGS", "GOMODCACHE", "GOPROXY", "GOSUMDB", "GOPRIVATE", "GONOSUMDB", "GONOPROXY", "CGO_ENABLED":
			continue
		}
		env = append(env, e)
	}
	env = append(env, "GOENV=off", "GOWORK=off", "GOFLAGS=", "GOMODCACHE="+cache, "GOPROXY=https://proxy.golang.org,direct", "GOSUMDB=sum.golang.org", "GOPRIVATE=", "GONOSUMDB=", "GONOPROXY=", "CGO_ENABLED=0")
	command := func(args ...string) ([]byte, error) {
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			return out, fmt.Errorf("go %s: %w\n%s", strings.Join(args, " "), err, out)
		}
		return out, nil
	}
	write := func(name, content string) error {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return err
		}
		return os.WriteFile(p, []byte(content), 0644)
	}
	if _, err = command("mod", "init", "example.com/iconconsumer"); err != nil {
		return err
	}
	if _, err = command("get", "github.com/Cyenoch/mygoicons@"+*version); err != nil {
		return err
	}
	raw, err := command("mod", "download", "-json", "github.com/Cyenoch/mygoicons@"+*version)
	if err != nil {
		return err
	}
	var mod struct{ Dir, Version, Zip string }
	if err = json.Unmarshal(raw, &mod); err != nil {
		return err
	}
	if err = write("main.go", nativeMain("github.com/Cyenoch/mygoicons/lucide", "lucide")); err != nil {
		return err
	}
	if _, err = command("mod", "tidy"); err != nil {
		return err
	}
	if _, err = command("build", "-trimpath", "-o", filepath.Join(dir, "native-app"), "."); err != nil {
		return err
	}
	input := filepath.Join(mod.Dir, "sources", "lucide", "icons.json")
	if _, err = command("run", "github.com/Cyenoch/mygoicons/cmd/iconify@"+*version, "-input", input, "-out", "appicons/icons_gen.go", "-package", "appicons", "-icons", "search,arrow-right"); err != nil {
		return err
	}
	if _, err = command("run", "github.com/Cyenoch/mygoicons/cmd/iconify@"+*version, "-input", input, "-out", "appicons/icons_gen.go", "-package", "appicons", "-icons", "search,arrow-right", "-check"); err != nil {
		return err
	}
	if err = write("main.go", nativeMain("example.com/iconconsumer/appicons", "appicons")); err != nil {
		return err
	}
	if _, err = command("mod", "edit", "-droprequire=github.com/Cyenoch/mygoicons"); err != nil {
		return err
	}
	if _, err = command("mod", "tidy"); err != nil {
		return err
	}
	mods, err := command("list", "-m", "all")
	if err != nil {
		return err
	}
	if bytes.Contains(mods, []byte("github.com/Cyenoch/mygoicons")) {
		return fmt.Errorf("subset still depends on generator module")
	}
	if _, err = command("build", "-trimpath", "-o", filepath.Join(dir, "subset-app"), "."); err != nil {
		return err
	}
	if *addCollection {
		if _, err = command("run", "github.com/Cyenoch/mygoicons/cmd/iconify@"+*version, "add", "mingcute@1.2.8", "-icons", "search-line,home-1-line"); err != nil {
			return err
		}
		if _, err = command("run", "github.com/Cyenoch/mygoicons/cmd/iconify@"+*version, "add", "mingcute", "-check"); err != nil {
			return err
		}
		source := strings.Replace(nativeMain("example.com/iconconsumer/internal/mingcute", "mingcute"), "mingcute.Search()", "mingcute.SearchLine()", 1)
		if err = write("main.go", source); err != nil {
			return err
		}
		if _, err = command("mod", "tidy"); err != nil {
			return err
		}
		if _, err = command("build", "-trimpath", "-o", filepath.Join(dir, "imported-app"), "."); err != nil {
			return err
		}
		mods, err = command("list", "-m", "all")
		if err != nil {
			return err
		}
		if bytes.Contains(mods, []byte("github.com/Cyenoch/mygoicons")) {
			return fmt.Errorf("imported package retains generator dependency")
		}
		fmt.Println("One-command MingCute import, saved-selection offline check and standalone native build passed.")
	}
	zip, err := os.Stat(mod.Zip)
	if err != nil {
		return err
	}
	fmt.Printf("Remote %s: installed with an empty module cache; native app built; fixed generator and -check passed; standalone subset built without mygoicons dependency. Module zip: %d bytes.\n", mod.Version, zip.Size())
	return nil
}
func nativeMain(importPath, pkg string) string {
	return fmt.Sprintf(`package main
import("log";"github.com/egoist/mygo";"github.com/egoist/mygo/ui";%q)
func main(){mygo.App.WhenReady(func(){w:=mygo.NewWindow(mygo.WindowOptions{Title:"Icon consumer",Width:320,Height:240,Content:ui.View(func(c *ui.Context){ui.Icon(c,%s.Search()).Size(24,24).Label("Search")})});w.Show()});if err:=mygo.App.Run();err!=nil{log.Fatal(err)}}
`, importPath, pkg)
}
