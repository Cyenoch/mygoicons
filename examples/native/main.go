// Command native is an offline, virtualized icon browser built with MyGo's public UI.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type browser struct {
	set              int
	query, lastQuery string
	lastSet          int
	results          []int
	selected         int
	chosen           map[string]bool
	size             float64
	dark             bool
	status           string
	list             ui.ListState
	built            int
}

func newBrowser() *browser {
	return &browser{lastSet: -1, selected: 0, size: 48, chosen: map[string]bool{}}
}
func (b *browser) filter() {
	if b.query == b.lastQuery && b.set == b.lastSet {
		return
	}
	b.lastQuery, b.lastSet = b.query, b.set
	b.results = nil
	b.selected = 0
	q := strings.ToLower(strings.TrimSpace(b.query))
	for i, e := range collections[b.set].Icons {
		if strings.Contains(strings.ToLower(e.Name), q) || strings.Contains(e.ID, q) {
			b.results = append(b.results, i)
		}
	}
	b.list.ScrollTo(0, ui.Start)
}
func (b *browser) current() (entry, bool) {
	if b.selected < 0 || b.selected >= len(b.results) {
		return entry{}, false
	}
	return collections[b.set].Icons[b.results[b.selected]], true
}
func (b *browser) copy(c *ui.Context, text, description string) {
	c.WriteClipboard(text)
	b.status = "Copied " + description
	c.Announce(b.status)
}
func (b *browser) selectedIDs() []string {
	var ids []string
	prefix := collections[b.set].Prefix + ":"
	for key := range b.chosen {
		if strings.HasPrefix(key, prefix) {
			ids = append(ids, strings.TrimPrefix(key, prefix))
		}
	}
	slices.Sort(ids)
	return ids
}
func usage(set string, e entry, size int) string {
	widget := "Icon"
	if set == "twemoji" {
		widget = "Image"
	}
	return fmt.Sprintf(`package main

import (
    "log"
    "github.com/egoist/mygo"
    "github.com/egoist/mygo/ui"
    "github.com/Cyenoch/mygoicons/%s"
)

func main() {
    mygo.App.WhenReady(func() {
        w := mygo.NewWindow(mygo.WindowOptions{Title: "Icon", Width: 320, Height: 240,
            Content: ui.View(func(c *ui.Context) {
                ui.%s(c, %s.%s()).Size(%d, %d).Label(%q)
            }),
        })
        w.Show()
    })
    if err := mygo.App.Run(); err != nil { log.Fatal(err) }
}
`, set, widget, set, e.Name, size, size, e.ID)
}
func showIcon(c *ui.Context, set string, e entry, size float32) {
	if set == "twemoji" {
		ui.Image(c, e.SVG()).Size(size, size).Label(e.ID)
	} else {
		ui.Icon(c, e.SVG()).Size(size, size).Label(e.ID)
	}
}
func (b *browser) view(c *ui.Context) {
	theme := ui.LightTheme()
	theme.Accent, theme.AccentHover, theme.AccentPressed = ui.RGB(32, 32, 32), ui.RGB(55, 55, 55), ui.RGB(10, 10, 10)
	theme.TextMuted = ui.RGB(95, 95, 95)
	c.SetTheme(theme)
	b.filter()
	b.built = 0
	set := collections[b.set]
	_, height := c.Size()
	listHeight := max(float32(120), height-232)
	ui.Column(c).Fill().Padding(24).Gap(12).Background(ui.RGB(248, 248, 248)).TextColor(ui.RGB(10, 10, 10)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Text(c, "mygoicons").FontSize(24)
			for i := range collections {
				ui.Button(c, collections[i].Prefix).Label("Collection " + collections[i].Prefix).OnClick(func() { b.set = i; b.status = "" })
			}
			ui.Spacer(c)
			ui.Button(c, "Copy -icons list").OnClick(func() {
				ids := b.selectedIDs()
				if len(ids) == 0 {
					b.status = "Select icons with the checkboxes first"
					return
				}
				b.copy(c, strings.Join(ids, ","), "selected Iconify IDs")
			})
		})
		ui.Text(c, fmt.Sprintf("%s %s · %s", set.Prefix, set.Version, set.License)).FontSize(14)
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c, &b.query).Label("Search icons").Placeholder("Go function or Iconify ID").Width(360).AutoFocus()
			ui.Text(c, fmt.Sprintf("%d results · %d selected", len(b.results), len(b.selectedIDs()))).FontSize(14)
			ui.Button(c, "Clear selection").OnClick(func() { b.chosen = map[string]bool{}; b.status = "Selection cleared" })
		})
		// 24px outer margins, a flexible list, a 12px gutter and 360px detail pane.
		ui.Row(c).Grow(1).Gap(12).Children(func() {
			ui.Column(c).Grow(1).Gap(8).Children(func() {
				if len(b.results) == 0 {
					ui.Text(c, "No icons match. Try a Go name such as Search or an ID such as arrow-right.").Label("No search results")
					return
				}
				b.list.Selected = &b.selected
				b.list.Key = func(row int) any { return set.Prefix + ":" + set.Icons[b.results[row]].Name }
				b.list.Label = func(row int) string { e := set.Icons[b.results[row]]; return e.Name + " · " + e.ID }
				ui.List(c, &b.list, len(b.results), func(row int) {
					b.built++
					e := set.Icons[b.results[row]]
					key := set.Prefix + ":" + e.ID
					chosen := b.chosen[key]
					ui.Row(c).Height(60).Gap(12).AlignItems(ui.Center).Padding(8).Children(func() {
						ui.Checkbox(c, &chosen, "").Label("Include " + e.ID).OnChange(func() {
							if chosen {
								b.chosen[key] = true
							} else {
								delete(b.chosen, key)
							}
						})
						showIcon(c, set.Prefix, e, 28)
						ui.Column(c).Gap(4).Children(func() { ui.Text(c, e.Name).FontSize(14); ui.Text(c, e.ID).FontSize(12) })
					})
				}).Height(listHeight).Shrink(0).Label("Icon results")
				ui.Text(c, "Arrow keys select · Space toggles a checkbox · Tab moves between controls").FontSize(12)
			})
			if b.selected < 0 && len(b.results) > 0 {
				b.selected = 0
			}
			ui.Column(c).Width(360).Gap(12).Padding(16).Background(ui.RGB(255, 255, 255)).Children(func() {
				e, ok := b.current()
				if !ok {
					ui.Text(c, "Select an icon to preview")
					return
				}
				ui.Text(c, e.Name).FontSize(18)
				ui.Text(c, set.Prefix+":"+e.ID).FontSize(14)
				bg, ink := ui.RGB(248, 248, 248), ui.RGB(29, 95, 191)
				if b.dark {
					bg, ink = ui.RGB(24, 24, 24), ui.RGB(235, 235, 235)
				}
				ui.Column(c).Height(180).AlignItems(ui.Center).Justify(ui.Center).Background(bg).TextColor(ink).Children(func() { showIcon(c, set.Prefix, e, float32(b.size)) })
				ui.Text(c, fmt.Sprintf("Preview size: %.0f px", b.size)).FontSize(14)
				ui.Slider(c, &b.size, 16, 128).Step(1).Label("Icon size")
				ui.Checkbox(c, &b.dark, "Dark preview background")
				ui.Text(c, set.Prefix+"."+e.Name+"()").FontSize(14)
				ui.Button(c, "Copy accessor").OnClick(func() { b.copy(c, set.Prefix+"."+e.Name+"()", "accessor") })
				ui.Button(c, "Copy complete Go app").OnClick(func() { b.copy(c, usage(set.Prefix, e, int(b.size)), "complete Go app") })
				ui.Text(c, "Lucide uses ui.Icon for text color. Twemoji uses ui.Image for original colors.").FontSize(14)
			})
		})
		ui.Text(c, b.status).FontSize(14).Label("Copy status")
	})
}
func main() {
	capture := flag.String("capture", "", "capture a native window frame to PNG, then exit")
	flag.Parse()
	b := newBrowser()
	mygo.App.WhenReady(func() {
		w := mygo.NewWindow(mygo.WindowOptions{Title: "mygoicons — Icon browser", Width: 1060, Height: 760, MinWidth: 960, MinHeight: 720, Content: ui.View(b.view)})
		if *capture != "" {
			w.OnReadyToShow(func() {
				w.Show()
				data, err := w.CapturePage()
				if err != nil {
					log.Fatal(err)
				}
				if err = os.WriteFile(*capture, data, 0o644); err != nil {
					log.Fatal(err)
				}
				log.Println("Native window rendered and captured")
				w.Close()
				mygo.App.Quit()
			})
		}
		w.Show()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
