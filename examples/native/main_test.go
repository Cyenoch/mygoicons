package main

import (
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestBrowserInteractions(t *testing.T) {
	b := newBrowser()
	tester := ui.NewTester(b.view, 1060, 760)
	t.Cleanup(func() {
		if t.Failed() {
			f, err := os.Create(os.TempDir() + "/mygoicons-browser-failure.png")
			if err == nil {
				png.Encode(f, tester.Image())
				f.Close()
			}
			t.Log(tester.Texts())
		}
	})
	click := func(label string) {
		t.Helper()
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if b.built > 50 || len(b.results) < 2000 {
		t.Fatalf("virtualization: built %d of %d", b.built, len(b.results))
	}
	click("Search icons")
	tester.Type("Search")
	if len(b.results) == 0 || len(b.results) > 30 {
		t.Fatalf("search result count %d", len(b.results))
	}
	click("Copy accessor")
	if !strings.Contains(tester.Clipboard(), "lucide.") || !strings.Contains(b.status, "Copied") {
		t.Fatal("copy accessor failed", tester.Clipboard())
	}
	e, _ := b.current()
	click("Include " + e.ID)
	click("Copy -icons list")
	if tester.Clipboard() != e.ID {
		t.Fatalf("subset copy %q", tester.Clipboard())
	}
	click("Copy complete Go app")
	if !strings.Contains(tester.Clipboard(), "ui.Icon(c,") || !strings.Contains(tester.Clipboard(), "func main()") {
		t.Fatal("usage not a complete app")
	}
	click("Collection twemoji")
	b.query = "grinning-face"
	tester.Frame()
	if len(b.results) == 0 {
		t.Fatal("Iconify ID search failed")
	}
	click("Copy complete Go app")
	if !strings.Contains(tester.Clipboard(), "ui.Image(c,") {
		t.Fatal("emoji usage should retain color")
	}
	click("Dark preview background")
	if !b.dark {
		t.Fatal("dark toggle did not update")
	}
	b.query = "zzzz-no-such-icon"
	tester.Frame()
	if !tester.HasText("Select an icon to preview") {
		t.Fatal("empty state missing")
	}
	b.query = ""
	tester.Frame()
	if b.built > 50 {
		t.Fatalf("built %d rows", b.built)
	}
	click("Icon results")
	tester.Key(0, ui.KeyEnd)
	if b.selected != len(b.results)-1 {
		t.Fatalf("keyboard End: %d", b.selected)
	}
	tester.Key(0, ui.KeyHome)
	if b.selected != 0 {
		t.Fatal("keyboard Home failed")
	}
	if path := os.Getenv("MYGO_BROWSER_PREVIEW"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err = png.Encode(f, tester.Image()); err != nil {
			t.Fatal(err)
		}
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
