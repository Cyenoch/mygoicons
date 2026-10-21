package mygoicons_test

import (
	"fmt"

	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoicons/twemoji"
	"github.com/egoist/mygo/ui"
)

func Example() {
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Gap(8).Children(func() {
			ui.Icon(c, lucide.Search()).Size(16, 16).Label("Search")
			ui.Image(c, twemoji.GrinningFace()).Size(24, 24)
		})
	}, 80, 32)
	_, found := tester.Find("Search")
	fmt.Println(found)
	// Output: true
}
