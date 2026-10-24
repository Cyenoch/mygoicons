# Maintaining mygoicons

Use Go 1.27.1 or newer. The checked-in `go.mod` intentionally requires MyGo v0.6.3; the latest-stable compatibility job uses a disposable copy. On Linux install `libpangocairo-1.0-0` for software rendering. No Node.js or npm installation is needed.

## Validate a checkout

```sh
go mod download
go generate ./
go run ./cmd/catalog -check
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go test ./...
CGO_ENABLED=1 go test -race ./...
GOOS=js GOARCH=wasm go build ./...
git diff --exit-code -- sources lucide twemoji examples/native/catalog_gen.go
```

Tests cover full regeneration and raw-source hashes, the pinned official Iconify oracle, concurrent first-use caches, software rendering of color/size/clip/rotation/gradients, native browser interactions and clipboard contents, virtualized row construction, and linking full, compatibility-alias and one-icon packages. The linker tests run with the module proxy disabled after dependencies have been downloaded. Their context deadlines guard hung subprocesses; no timing threshold decides correctness.

Run the native browser with `go run ./examples/native`. Its event-based capture mode, `go run ./examples/native -capture preview.png`, exercises a real native window; CI runs that on macOS. Headless interaction tests exercise the same public MyGo view. The browser supports desktop windows of at least 960 × 720 logical pixels.

## Import a collection into an application

Install `go install github.com/Cyenoch/mygoicons/cmd/iconify@v0.2.1`, then run `iconify add mingcute` from the application module. Add `@1.2.8` to pin the first import, or `-icons search-line,home-1-line` for a subset. The command saves the exact version and selection in `internal/mingcute/source.json`, raw input under `source/`, full applicable license notices, and generated Go. Commit the complete directory. Later `iconify add mingcute` and `iconify add mingcute -check` reuse saved data offline; updating requires an explicit new version.

Imports reuse the updater's bounded npm download, SHA-512 verification and archive validation. Automatic notice retrieval supports GitHub file license URLs and records immutable commit identities and file digests. Other license locations require the existing local `-input` workflow. Application imports report license changes and refresh notices in place. Handwritten Go files, other files, subdirectories and custom `.gitattributes` are preserved during updates. Generated paths cannot redirect writes through symbolic links. The bundled-collection updater keeps its separate license-review gate. `.gitattributes` inside each package preserves the upstream bytes on Windows.

The package is generated in a temporary sibling directory before replacement. A failed final rename restores the previous directory. Process interruption between renames leaves `<directory>.iconify-backup`; restore that directory before retrying. Do not run concurrent writers against the same import directory. Existing local JSON generation and `-check` remain network-free.

## Update collections

```sh
# Query published stable versions.
go run ./cmd/update-icons -versions lucide
go run ./cmd/update-icons -versions twemoji

# Preview latest stable inputs; does not modify repository files.
go run ./cmd/update-icons -check

# An exact reproducible selection (the versions shipped in v0.1.0).
go run ./cmd/update-icons -lucide 1.2.147 -twemoji 1.2.5 -check

# Apply the selection or omit version flags to check current npm releases.
go run ./cmd/update-icons -lucide 1.2.147 -twemoji 1.2.5
```

The network-enabled tool resolves `latest` only as a discovery step, then fetches exact version metadata and its registry-origin archive. It checks package name/version and SHA-512 integrity before parsing. HTTP responses are bounded to 32 MiB for archives and 8 MiB for metadata, with a 60-second request timeout and at most five HTTPS redirects. Archives allow at most 128 regular-file/directory entries, 64 MiB per entry and 96 MiB total expanded data; JSON metadata entries allow 8 MiB. Links, special entries, duplicate entries and escaping paths fail. Generator limits apply afterward.

The updater verifies existing files against `sources/lock.json`, builds all outputs in memory, checks compatibility, and stages replacements in `.iconify-update/` before touching tracked files. Ordinary write failures restore previous files. An interrupted update leaves a journal and blocks further writes; restore with:

```sh
go run ./cmd/update-icons -recover
```

Do not delete that journal during an interrupted replacement. Filesystem failures during recovery are reported and the journal is retained. The transaction is recoverable across process interruption; it is not a filesystem-wide atomic snapshot for other processes reading midway through replacement. Run maintenance in a clean checkout without concurrent writers.

The summary lists added/removed original icons, upstream aliases, exports and license changes. A no-op writes nothing. `-summary report.txt` additionally saves the report to an explicit path, including failure summaries. A preview returns success when a compatible update is available; incompatibility or invalid input returns failure.

## Export compatibility

Each `sources/<collection>/exports.json` records the public Go name, original ID and normalized-artwork SHA-256. It is independent of the naming algorithm and is committed with generated code. `cmd/iconify` checks existing generated accessors automatically and can also take `-manifest`. `generate.Build` accepts `Options.Previous` and returns `Result.Exports`.

When an ID remains, naming-policy changes preserve the old accessor as a direct forwarding function. If an ID disappears, only a unique identical normalized SVG can establish an automatic replacement. Ambiguity, removal without a replacement, collisions or reassignment of an old public name to a different ID fail. Retaining aliases adds no runtime registry and preserves linker pruning. Compatibility aliases share their target's cache.

For an intentional incompatible change, first retain a reviewed upstream alias or manually decide the replacement in the source data. If that is impossible, a breaking release requires an explicit reviewed edit of the manifest and generated API baseline; never regenerate the baseline blindly to silence a failure. For a deliberately smaller custom subset, write a new output path, review the removed accessors and replace the old file explicitly. This is why generating over an existing file refuses silent shrinkage.

## Licenses and provenance

The updater retains full notices and retrieves them from an immutable GitHub commit, checking the Git blob SHA-1 and recording SHA-256. Git SHA-1 is a protocol object ID, not the archive integrity mechanism. The exact npm version, metadata URL, tarball URL, npm integrity, file sizes/digests, package gitHead and notice revisions go in the lock. npm signatures/attestations are not verified. Iconify package gitHead is not an artwork commit.

A changed license metadata object or notice body blocks the update. Review the upstream full text and report, then rerun with `-accept-license` only after accepting those changes. Scheduled automation never passes that flag. A pinned notice revision identifies the retained notice; it does not establish an exact correspondence to every artwork revision in Iconify's npm release. Preserve that distinction in notices and release notes.

Project code stays MIT. Lucide retains ISC plus Feather MIT terms; Twemoji graphics retain CC-BY-4.0 and attribution to Twitter and the Twemoji contributors. Keep `.gitattributes`: source and oracle checksums are over original upstream bytes, including line endings.

## Automated update PRs

`Update icons` runs weekly and supports manual dispatch with optional exact versions. Only trusted default-branch code runs with write permissions. It generates, vets, tests and builds WASM before committing. One `automation/icon-collections` branch and PR is reused; an identical tree produces no commit. Existing branch history is preserved without force pushes. Release identities (collection version and npm integrity) deduplicate the same batch even when retrieval dates differ; failed PR creation is retried without an extra commit. No updates produce no empty PR. It neither merges nor publishes updates. License changes and incompatible exports stop the workflow for review.

The repository allows Actions to create PRs. If that setting is disabled, a maintainer can enable it with:

```sh
gh api -X PUT repos/Cyenoch/mygoicons/actions/permissions/workflow \
  -F default_workflow_permissions=read -F can_approve_pull_request_reviews=true
```

The update job needs `contents: write` and `pull-requests: write`. It also uses `actions: write` solely to dispatch read-only CI on the update branch, because pushes made with `GITHUB_TOKEN` do not trigger new CI runs. Other jobs use read-only repository permissions. Checkout credentials are not persisted; only the final PR step enables authenticated Git operations. The update job accepts only default-branch triggers and does not restore executable build caches. Actions are pinned to immutable commits; GitHub Dependabot groups weekly pin updates into reviewable PRs. No third-party service or extra secret is required. Review the PR and its explicitly dispatched CI before merging it.

No-update runs skip Pango setup, full validation and PR operations. Changed updates still pass every pre-commit gate. CI runs on main pushes, version tags, PRs and explicit dispatch, avoiding duplicate branch-push/PR runs. It keeps all supported platforms, latest-stable MyGo, race and WASM coverage. Pango installation is conditional on its absence, and module metadata is checked without raising the minimum MyGo version. Tests shuffle their order to reveal shared state. Linux also runs `python3 .github/scripts/test_update_pr.py`: a temporary local Git remote and a fake GitHub CLI exercise the actual workflow script's PR-failure recovery, cross-date deduplication and preserved branch history without external access.

## Remote consumer and cost checks

```sh
go run ./cmd/verify-consumer -version v0.2.1 -add
go run ./cmd/measure -out measurements.json
```

The consumer check uses an empty module cache, disables `go.work`, GOFLAGS and per-user Go configuration, uses the public proxy/checksum database, imports the published path and builds a native app. It runs the fixed generator twice (including `-check`), drops mygoicons from the consumer requirements, and builds a standalone subset that depends only on MyGo and the standard library. With `-add`, it also imports MingCute, checks the remembered selection offline and builds the imported package without a generator dependency. There is no local `replace`. A full pushed commit SHA can be checked before tagging. See [MEASUREMENTS.md](MEASUREMENTS.md) for separate download, compile, binary and runtime costs.

## Release

1. Review collection/API/license changes and update README collection counts, provenance and third-party notices as needed. Run the complete validation above and the native browser.
2. Commit and push without rewriting history. Wait for all jobs on that exact commit: all three operating systems, latest stable MyGo and public-path consumer check. Inspect failures and fix them before tagging.
3. Inspect `gh release list` and `git ls-remote --tags origin`. First release is `v0.1.0`; later versions follow the actual compatibility impact. Never move or reuse a published tag. Update reproducible documentation commands to that version before the final checked commit.
4. Create an annotated tag on the verified commit, push the tag, then create the release with an explicit notes file. For the first release:

   ```sh
   git tag -a v0.1.0 -m 'Release v0.1.0'
   git push origin v0.1.0
   gh release create v0.1.0 --verify-tag --title 'mygoicons v0.1.0' \
     --notes-file release-notes.md
   ```

5. Wait for tag CI too. Run `go run ./cmd/verify-consumer -version v0.2.1 -add` from the checked checkout; it downloads the remote tag into its own clean module/cache and verifies installation and pinned-generator use. Record release validation and any real limitations. Keep the workspace clean.

Release notes name the actual npm data versions, Go/MyGo support, API additions and artwork licenses. Do not claim a verified artwork revision when only the npm package and notice snapshots are pinned.
