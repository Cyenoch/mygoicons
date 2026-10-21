package mygoicons_test

import (
	"image/color"
	"image/png"
	"os"
	"sync"
	"testing"

	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoicons/twemoji"
	"github.com/egoist/mygo/ui"
)

var benchmarkIcon *ui.SVG

func BenchmarkCachedIcon(b *testing.B) {
	lucide.Search()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		benchmarkIcon = lucide.Search()
	}
}

func TestConcurrentFirstUse(t *testing.T) {
	const count = 32
	var results [count]*ui.SVG
	var workers sync.WaitGroup
	for i := range results {
		workers.Go(func() { results[i] = lucide.Activity() })
	}
	workers.Wait()
	for _, result := range results {
		if result == nil || result != results[0] {
			t.Fatal("concurrent calls did not share the cached SVG")
		}
	}
	if w, h := results[0].Size(); w != 24 || h != 24 {
		t.Fatalf("unexpected Lucide size %vx%v", w, h)
	}
}

func TestBundledSoftwareRendering(t *testing.T) {
	tint := ui.RGB(220, 30, 40)
	view := func(c *ui.Context) {
		ui.Column(c).Fill().Padding(8).Gap(8).Background(ui.RGB(255, 255, 255)).AlignItems(ui.Start).Children(func() {
			ui.Row(c).Gap(8).Children(func() {
				ui.Icon(c, lucide.Circle()).Size(72, 72).TextColor(tint).Label("tinted")
				ui.Image(c, twemoji.GrinningFace()).Size(72, 72).Label("emoji")
				ui.Image(c, twemoji.FlagTaiwan()).Size(72, 72).Label("flag")
				ui.Image(c, twemoji.ShakingFace()).Size(72, 72).Label("clip")
			})
			ui.Row(c).Gap(8).Children(func() {
				ui.Icon(c, lucide.Search()).Size(24, 24)
				ui.Icon(c, lucide.ArrowDown0_1()).Size(24, 24)
				ui.Icon(c, lucide.ArrowDown01()).Size(24, 24)
				ui.Image(c, twemoji.ArtistPalette()).Size(48, 48)
				ui.Image(c, twemoji.Maracas()).Size(48, 48)
			})
		})
	}
	tester := ui.NewTester(view, 340, 160)
	rect := func(label string) ui.Rect {
		t.Helper()
		r, ok := tester.Find(label)
		if !ok || r.W != 72 || r.H != 72 {
			t.Fatalf("%s: missing or wrong bounds %+v", label, r)
		}
		return r
	}
	circle := rect("tinted")
	pixel := func(r ui.Rect, x, y float32) color.RGBA {
		return tester.Image().RGBAAt(int(r.X+x*r.W), int(r.Y+y*r.H))
	}
	// Lucide circle is centered at (12,12), radius 10 and stroke width 2.
	if got := pixel(circle, .5, 1.0/12); got.R < 190 || got.G > 70 || got.B > 80 {
		t.Fatalf("Lucide stroke did not take red text color: %v", got)
	}
	tint = ui.RGB(20, 70, 220)
	tester.Frame()
	if got := pixel(circle, .5, 1.0/12); got.B < 190 || got.R > 70 {
		t.Fatalf("cached SVG did not adopt changed text color: %v", got)
	}
	emoji := rect("emoji")
	if got := pixel(emoji, .5, .25); got.R < 220 || got.G < 140 || got.B > 110 {
		t.Fatalf("Twemoji lost its yellow artwork fill: %v", got)
	}
	flag := rect("flag")
	if got := pixel(flag, .7, .7); got.R < 190 || got.G > 80 || got.B > 100 {
		t.Fatalf("Twemoji flag lost its red field: %v", got)
	}
	clip := rect("clip")
	// A clipped face has visible warm pixels and a white exterior.
	if got := pixel(clip, .5, .25); got.R < 200 || got.G < 120 || got.B > 120 {
		t.Fatalf("clipped Twemoji face is missing its artwork: %v", got)
	}
	if got := pixel(clip, 0, 0); got != (color.RGBA{255, 255, 255, 255}) {
		t.Fatalf("clip example painted outside its face: %v", got)
	}
	if path := os.Getenv("MYGO_ICONS_PREVIEW"); path != "" {
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(file, tester.Image())
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			t.Fatal(err, closeErr)
		}
	}
}
