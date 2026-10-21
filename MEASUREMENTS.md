# Cost measurements

These observations describe Go 1.27.2 on macOS/arm64 (Apple M5 Pro), with MyGo v0.5.5 and Lucide 1.2.147 / Twemoji 1.2.5. They are not performance promises or CI thresholds. Keep runtime, build, executable and module-download costs separate.

```sh
go run ./cmd/measure -out measurements.json
go test . -run '^$' -bench 'Benchmark(CachedIcon|FirstParse)$' -benchmem -count=3
go test ./generate -run TestGeneratedPackageLinksOnlyUsedIcon -v
go run ./cmd/verify-consumer -version v0.1.0
```

`measure` creates a temporary consumer, uses the same package path and `Search` accessor for both builds, and starts with a fresh build cache for each. Dependencies are already downloaded. Executables use `-trimpath -ldflags='-s -w'`. It generates the single-icon subset from the actual bundled JSON. It deletes its temporary build caches afterward. Cold-build wall time includes the standard library and MyGo; it is one observation per variant, not an isolated estimate of generator or collection compiler time.

| Measurement | Observed result |
| --- | ---: |
| Lucide generated Go source | 1,124,575 bytes |
| Twemoji generated Go source | 12,281,508 bytes |
| Source JSON, manifests and retained notices (`sources/`) | 11,964,440 bytes |
| Generated one-icon Lucide subset | 926 bytes |
| Full Lucide, cold build, one referenced icon | 3.331 s |
| One-icon subset, cold build | 2.927 s |
| Full Lucide, executable with one referenced icon | 3,450,562 bytes |
| One-icon subset, same executable | 3,450,562 bytes |
| Native MyGo probe, full / compatibility alias / subset | 9,283,170 bytes each |
| Cached `lucide.Search()` | 1.678–1.865 ns/op, 0 B/op, 0 allocs/op |
| Fresh representative SVG parse | 3.610–3.868 µs/op, 6,240 B/op, 89 allocs/op |

The parse benchmark runs `ui.MustParseSVG` on a small search-shaped SVG every iteration; it measures parsing, not process startup or a collection-wide cache. First accessor use adds its `sync.Once` operation. Cached calls return the shared pointer and allocate nothing. The full and single-icon packages retain exactly one SVG marker in both a console probe and a native MyGo application; compatibility forwarding functions are checked too. Actual binary size varies by target, MyGo version, build flags and application imports.

The module download is the **whole Go module**, including both collections, inputs, generator and development example. Linking removes unused data from the executable; it does not reduce module download size or avoid compiling the imported package. `verify-consumer` downloads the actual published module into an empty module cache and prints its compressed zip byte count. Record the exact release/commit with that output; dependencies and Go toolchain downloads are separate and not included. The v0.1.0 release notes include the final remote module zip measurement.

For an independent module-only measurement:

```sh
# Run outside this checkout, without go.work or local replace directives.
GOWORK=off GOENV=off go mod download -json github.com/Cyenoch/mygoicons@v0.1.0
```

The JSON result's `Zip` is the downloaded compressed module file and `Dir` is its expanded source directory. Measure those separately from executable sizes. A custom subset committed to an application requires only MyGo and the standard library at build time; using the fixed generator still downloads the generator module when it is first invoked. No multi-module split is introduced: the measured executable result supports preserving the small public interface, and users needing lower source/compiler cost can generate subsets.
