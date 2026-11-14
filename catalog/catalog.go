// Package catalog lists every icon of the bundled collections, for icon
// browsers, pickers and documentation.
//
// Importing a collection links only the icons a program names. Calling a
// function of this package keeps that whole collection in the executable,
// so call it only where every icon is wanted.
package catalog

import "github.com/egoist/mygo/ui"

// Icon is one accessor of a collection.
type Icon struct {
	// Name is the Go function, such as ArrowRight.
	Name string
	// ID is the Iconify name, such as arrow-right.
	ID string
	// AliasOf names the icon with the same artwork that this one repeats,
	// or is empty. A browser showing each drawing once skips aliases.
	AliasOf string
	// SVG returns the shared, parsed artwork. Treat it as immutable.
	SVG func() *ui.SVG
}

// Collection is one bundled icon set. Icons is shared: do not modify it.
type Collection struct {
	// Prefix is the Iconify prefix and the Go package name.
	Prefix string
	// Title is the collection's display name.
	Title string
	// Version is the pinned @iconify-json release.
	Version string
	// License is the artwork's SPDX identifier and attribution.
	License string
	// Colored artwork keeps its own colors: draw it with ui.Image. Other
	// collections follow the text color through ui.Icon.
	Colored bool
	Icons   []Icon
}
