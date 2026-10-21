// Command native shows typed icons in a standalone MyGo window.
package main

import (
	"log"

	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoicons/twemoji"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	mygo.App.WhenReady(func() {
		window := mygo.NewWindow(mygo.WindowOptions{
			Title: "MyGo Icons", Width: 420, Height: 180,
			Content: ui.View(func(c *ui.Context) {
				ui.Column(c).Fill().Padding(24).Gap(20).Children(func() {
					ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, lucide.Search()).Size(20, 20).Label("Search")
						ui.Icon(c, lucide.Settings()).Size(20, 20).Label("Settings")
						ui.Text(c, "Lucide follows the text color")
					})
					ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
						ui.Image(c, twemoji.GrinningFace()).Size(32, 32).Label("Grinning face")
						ui.Image(c, twemoji.ArtistPalette()).Size(32, 32).Label("Artist palette")
						ui.Text(c, "Twemoji keeps its colors")
					})
				})
			}),
		})
		window.Show()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
