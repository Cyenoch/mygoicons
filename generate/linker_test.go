package generate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGeneratedPackageLinksOnlyUsedIcon(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join("..", name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "go.mod" {
			data = bytes.Replace(data, []byte("module github.com/Cyenoch/mygoicons"), []byte("module iconprobe"), 1)
		}
		writeProbe(t, filepath.Join(dir, name), data)
	}
	const total = 2_000
	icons := make(map[string]map[string]string, total)
	for i := range total {
		icons[fmt.Sprintf("glyph-%04d", i)] = map[string]string{
			"body": fmt.Sprintf(`<path id="MYGO_ICON_LINK_MARKER_%04d_END" d="M0 0h1v1z"/>`, i),
		}
	}
	input, err := json.Marshal(map[string]any{"prefix": "probe", "icons": icons})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Build(input, Options{Package: "glyphs"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "glyphs"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeProbe(t, filepath.Join(dir, "glyphs", "icons_gen.go"), result.Source)
	main := []byte("package main\nimport (\"fmt\"; \"iconprobe/glyphs\")\nfunc main() { fmt.Println(glyphs.Glyph0000().Size()) }\n")
	writeProbe(t, filepath.Join(dir, "main.go"), main)
	binary := filepath.Join(dir, "probe.exe")
	command := func(args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off", "CGO_ENABLED=0", "GOPROXY=off", "GOSUMDB=off", "GOFLAGS=")
		return cmd.CombinedOutput()
	}
	if output, err := command("build", "-mod=mod", "-trimpath", "-ldflags=-s -w", "-o", binary, "."); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(data, []byte("MYGO_ICON_LINK_MARKER_")); got != 1 ||
		!bytes.Contains(data, []byte("MYGO_ICON_LINK_MARKER_0000_END")) {
		t.Fatalf("binary retains %d of %d markers; want only the referenced icon", got, total)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, binary).CombinedOutput(); err != nil || strings.TrimSpace(string(output)) != "16 16" {
		t.Fatalf("generated executable: %v, output %q", err, output)
	}
	writeProbe(t, filepath.Join(dir, "main.go"), bytes.Replace(main, []byte("Glyph0000"), []byte("GlyphTypo"), 1))
	if output, err := command("build", "-mod=mod", "-o", binary, "."); err == nil ||
		!bytes.Contains(output, []byte("undefined: glyphs.GlyphTypo")) {
		t.Fatalf("unknown icon did not produce a compiler diagnostic: %v\n%s", err, output)
	}
	t.Logf("linked %d-byte executable: 1/%d SVG markers; typo rejected by compiler", len(data), total)
	// The native MyGo application path can bring different reflection/runtime
	// roots into reachability. Build it too, without opening a window in tests.
	native := []byte(`package main
import ("github.com/egoist/mygo"; "github.com/egoist/mygo/ui"; "iconprobe/glyphs")
func main() {
	mygo.App.WhenReady(func() {
		w := mygo.NewWindow(mygo.WindowOptions{Title: "Icon probe", Width: 100, Height: 100,
			Content: ui.View(func(c *ui.Context) { ui.Icon(c, glyphs.Glyph0000()) })})
		w.Show()
	})
	if err := mygo.App.Run(); err != nil { panic(err) }
}
`)
	writeProbe(t, filepath.Join(dir, "main.go"), native)
	if output, err := command("build", "-mod=mod", "-trimpath", "-ldflags=-s -w", "-o", binary, "."); err != nil {
		t.Fatalf("native build: %v\n%s", err, output)
	}
	nativeData, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(nativeData, []byte("MYGO_ICON_LINK_MARKER_")); got != 1 ||
		!bytes.Contains(nativeData, []byte("MYGO_ICON_LINK_MARKER_0000_END")) {
		t.Fatalf("native binary retains %d of %d markers; want only the referenced icon", got, total)
	}
	t.Logf("native MyGo application: %d bytes, 1/%d SVG markers (build only)", len(nativeData), total)
	// A forwarding compatibility accessor must not retain the other caches.
	previous := []Export{{Name: "LegacyGlyph", ID: "glyph-0000", SHA256: result.Exports[0].SHA256}}
	compatible, err := Build(input, Options{Package: "glyphs", Previous: previous})
	if err != nil {
		t.Fatal(err)
	}
	subset, err := Build(input, Options{Package: "glyphs", Icons: []string{"glyph-0000"}, Previous: previous})
	if err != nil {
		t.Fatal(err)
	}
	for _, probe := range []struct {
		name   string
		source []byte
	}{{"compatibility", compatible.Source}, {"single subset", subset.Source}} {
		writeProbe(t, filepath.Join(dir, "glyphs", "icons_gen.go"), probe.source)
		writeProbe(t, filepath.Join(dir, "main.go"), bytes.Replace(native, []byte("Glyph0000"), []byte("LegacyGlyph"), 1))
		if output, err := command("build", "-mod=mod", "-trimpath", "-ldflags=-s -w", "-o", binary, "."); err != nil {
			t.Fatalf("%s: %v\n%s", probe.name, err, output)
		}
		data, err := os.ReadFile(binary)
		if err != nil {
			t.Fatal(err)
		}
		if got := bytes.Count(data, []byte("MYGO_ICON_LINK_MARKER_")); got != 1 {
			t.Fatalf("%s retains %d markers", probe.name, got)
		}
		t.Logf("%s: %d-byte native executable; exactly one SVG marker", probe.name, len(data))
	}
}

func writeProbe(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
