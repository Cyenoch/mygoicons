package generate

import (
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestGradientClipAndNonsquareRotation(t *testing.T) {
	data := []byte(`{
		"prefix":"art","width":20,"height":10,
		"icons":{"palette":{"body":"<defs><linearGradient id=\"paint\"><stop stop-color=\"#f00\"/><stop offset=\"1\" stop-color=\"#00f\"/></linearGradient><clipPath id=\"circle\"><circle cx=\"10\" cy=\"5\" r=\"5\"/></clipPath></defs><rect width=\"20\" height=\"10\" fill=\"url(#paint)\" clip-path=\"url(#circle)\"/>"}},
		"aliases":{"rotated":{"parent":"palette","rotate":1}}
	}`)
	c, err := readCollection(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"palette", "rotated"} {
		t.Run(name, func(t *testing.T) {
			r := resolver{c, make(map[string]properties), make(map[string]bool)}
			p, err := r.resolve(name, 0)
			if err != nil {
				t.Fatal(err)
			}
			icon, err := c.complete(p)
			if err != nil {
				t.Fatal(err)
			}
			doc, _, err := icon.svg()
			if err != nil {
				t.Fatal(err)
			}
			svg, err := ui.ParseSVG([]byte(doc))
			if err != nil {
				t.Fatal(err)
			}
			w, h := svg.Size()
			if name == "palette" && (w != 20 || h != 10) || name == "rotated" && (w != 10 || h != 20) {
				t.Fatalf("incorrect intrinsic size %vx%v", w, h)
			}
			tester := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Fill().Background(ui.RGB(255, 255, 255)).Children(func() {
					ui.Image(c, svg).Size(80, 80)
				})
			}, 80, 80)
			img := tester.Image()
			if got := img.RGBAAt(5, 5); got != (color.RGBA{255, 255, 255, 255}) {
				t.Fatalf("aspect ratio/clip painted outside circle: %v", got)
			}
			from, to := img.RGBAAt(26, 40), img.RGBAAt(54, 40)
			if name == "rotated" {
				from, to = img.RGBAAt(40, 26), img.RGBAAt(40, 54)
			}
			if from.R <= from.B+50 || to.B <= to.R+50 || from.A != 255 || to.A != 255 {
				t.Fatalf("gradient or alias transform lost: start %v, end %v", from, to)
			}
			if got := img.RGBAAt(21, 21); got != (color.RGBA{255, 255, 255, 255}) {
				t.Fatalf("clip path did not remove the corner: %v", got)
			}
		})
	}
}
