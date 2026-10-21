# mygoicons

[![CI](https://github.com/Cyenoch/mygoicons/actions/workflows/ci.yml/badge.svg)](https://github.com/Cyenoch/mygoicons/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Cyenoch/mygoicons.svg)](https://pkg.go.dev/github.com/Cyenoch/mygoicons)

Typed [Iconify](https://iconify.design) collections for [MyGo](https://github.com/egoist/mygo), with a generator for your own collections. Import a collection, pick a name with editor completion, and pass its `*ui.SVG` to MyGo.

```sh
go get github.com/Cyenoch/mygoicons@v0.1.0
```

Requires Go 1.27.1 or later. MyGo v0.5.5 is the minimum supported version; CI also checks the latest stable release (v0.6.0 at this release).

## Use icons

```go
import (
    "github.com/egoist/mygo/ui"
    "github.com/Cyenoch/mygoicons/lucide"
    "github.com/Cyenoch/mygoicons/twemoji"
)

func toolbar(c *ui.Context) {
    ui.Row(c).Gap(8).Children(func() {
        ui.Icon(c, lucide.Search()).Size(16, 16)
        ui.Icon(c, lucide.ArrowRight()).Size(16, 16)
        ui.Image(c, twemoji.GrinningFace()).Size(24, 24)
    })
}
```

`ui.Icon` follows the text color. `ui.Image` keeps the artwork's colors. Name an icon-only control with `Label` for accessibility.

Each function returns a shared SVG, parsed once on first use. Element size and color remain independent at each use. Treat returned SVGs as immutable.

- Ordinary Go functions provide completion, documentation with the original Iconify ID, and compiler errors for misspelled names.
- Each icon has its own lazy cache. There is no runtime collection registry, so Go's linker can remove unused icons.
- Generated source is committed. Application builds need no generator, JavaScript runtime, network requests, or asset server.
- The generator checks aliases, name collisions, SVG syntax, and compatibility with MyGo's supported SVG features.

## Collections

| Package | Iconify source | Version | Functions |
| --- | --- | --- | ---: |
| `lucide` | `@iconify-json/lucide` | 1.2.147 | 2,198 |
| `twemoji` | `@iconify-json/twemoji` | 1.2.5 | 4,728 |

Both collections include aliases and hidden names from their pinned source data. Counts describe exported names, not distinct artwork.

Names use MixedCaps with common Go initialisms: `scan-qr-code` becomes `ScanQRCode`. A leading digit gets `Icon`, as in `Icon1stPlaceMedal`. Adjacent numeric segments retain an underscore: `arrow-down-0-1` becomes `ArrowDown0_1`, while `arrow-down-01` becomes `ArrowDown01`. Other name collisions fail generation and identify both original IDs.

Unused artwork is removed from the executable, but importing a collection still downloads and compiles its generated source. A registry that references every accessor deliberately retains every icon. Generate a subset if download, compilation, or editor indexing costs matter.

## Find and preview icons

```sh
go run github.com/Cyenoch/mygoicons/examples/native@v0.1.0
```

The native browser switches between collections, searches Go names and original Iconify IDs, adjusts size and preview background, and copies an accessor or a complete Go application. Check multiple icons and use **Copy -icons list** for a generator selection. Each collection shows its pinned version and license. Arrow keys, Home/End and Tab support keyboard operation; checkboxes toggle with Space. Empty searches and copy actions provide feedback.

The example's generated directory references every accessor, and its virtual list builds only visible rows. The directory is confined to the example; collection packages contain no registry. A desktop window needs at least 960 × 720 logical pixels. From a checkout, run `go run ./examples/native`; `-capture preview.png` opens and captures a native frame, then exits on the ready event.

## Generate a collection

Run the generator against a local Iconify `icons.json`, from an individual `@iconify-json/{prefix}` package or a private collection:

```sh
go run github.com/Cyenoch/mygoicons/cmd/iconify@v0.1.0 \
  -input /path/to/icons.json \
  -out internal/appicons/icons_gen.go \
  -package appicons
```

Use `-icons search,arrow-right` to select a subset by original name. Omit it to generate the entire collection. Use `-check` with the same arguments to detect stale or missing output without writing.

Generated packages import only MyGo and the standard library. Commit the output into your application; it does not need to depend on this module. Preserve the source collection's license and attribution alongside the generated artwork.

The CLI validates input before writing, refuses to overwrite handwritten files, and replaces generated output through a temporary file. Existing generated accessors are checked automatically. Naming policy changes retain forwarding aliases; a removed ID is compatible automatically only when a unique replacement has identical normalized artwork. Missing replacements and identity conflicts fail without writing. `-manifest sources/lucide/exports.json` adds an independently committed compatibility baseline; programmatic callers supply `Options.Previous` and retain `Result.Exports`. To deliberately shrink a custom subset, generate to a new output file and review the API change. It reads local JSON only; it does not download collections or ingest raw SVG directories. Pin the generator to a version or commit for reproducible `go:generate` directives.

For programmatic use:

```go
result, err := generate.Build(data, generate.Options{
    Package: "appicons",
    Icons:   []string{"search", "arrow-right"},
})
```

Import `github.com/Cyenoch/mygoicons/generate`. `Build` returns generated source, an export manifest, the collection prefix, icon count, and input SHA-256 without accessing the filesystem or network.

## SVG compatibility

The generator targets MyGo's SVG renderer. It admits paths, basic shapes, groups, use/symbol, gradients, clip paths, and masks. Text, raster images, filters, patterns, markers, animations, external references, CSS style sheets, unsupported inline styles, and vector effects cause generation errors. Convert unsupported artwork to paths first.

Alias parents are resolved before collection defaults. Explicit zero origins, fractional and nonsquare geometry, flip XOR, quarter-turn rotations, definition placement, and local reference IDs are preserved.

Compatibility checks are conservative and do not establish browser SVG conformance. The bundled collections are generated and tested in full; compatibility with every Iconify collection is not promised. Selected-subset generation validates selected icons and their parents, not unused artwork.

Inputs are limited to 64 MiB, 50,000 names, 1 MiB per icon body, 128 alias parent levels, 128 nested SVG elements, 100,000 XML tokens per normalized SVG, and 128 MiB of generated source. ViewBox coordinates must be finite within ±1,048,576, with positive dimensions; path numbers must be finite within ±1e9. These bounds govern admitted input, not total process memory. MyGo's own canvas limits still apply.

## Development

```sh
go generate ./
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=1 go test -race ./...
go run ./cmd/catalog -check
git diff --exit-code -- sources lucide twemoji examples/native/catalog_gen.go
GOOS=js GOARCH=wasm go build ./...
```

On Linux, software rendering tests require Pango (`libpangocairo-1.0-0`). Generation and ordinary tests use checked-in data and need no Node.js or icon downloads. Go dependencies must be available before the linker tests, which build subprocesses with the module proxy disabled.

Tests compare alias resolution and SVG construction with pinned output from official Iconify Utils 3.1.7, verify source checksums and regeneration, exercise concurrent cache access, render representative icons in software, and confirm that a 2,000-icon package retains only one referenced SVG in an executable.

```sh
go run ./examples/native
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for safe collection updates, API checks, remote consumer verification and release steps, and [MEASUREMENTS.md](MEASUREMENTS.md) for reproducible cost measurements. Scheduled updates create review PRs only after generation and checks; license changes stop automation for manual review. Releases remain a separate maintainer action.

To export the software rendering test's frame, set `MYGO_ICONS_PREVIEW` to a PNG path when running `go test -run TestBundledSoftwareRendering .`.

## License

Project code is [MIT](LICENSE). Artwork retains its upstream licenses: Lucide carries ISC and Feather MIT notices; Twemoji graphics are CC-BY-4.0 and require attribution to Twitter and other Twemoji contributors.

See [third-party notices](THIRD_PARTY_NOTICES.md) and [source provenance](sources/README.md) for full licenses, pinned versions, integrity checks, and file digests.
