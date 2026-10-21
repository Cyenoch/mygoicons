// Package mygoicons contains generation directives for the bundled collections.
// Applications import a collection such as github.com/Cyenoch/mygoicons/lucide.
// Each icon accessor returns a lazily parsed, shared immutable MyGo SVG.
//
//go:generate go run ./cmd/iconify -input sources/lucide/icons.json -out lucide/icons_gen.go -package lucide -manifest sources/lucide/exports.json
//go:generate go run ./cmd/iconify -input sources/twemoji/icons.json -out twemoji/icons_gen.go -package twemoji -manifest sources/twemoji/exports.json
//go:generate go run ./cmd/catalog
package mygoicons
