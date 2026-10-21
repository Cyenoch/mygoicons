# Pinned Iconify collection inputs

Retrieved and verified on 2026-10-16. These are publisher/generator inputs,
copied without reformatting from integrity-verified npm archives. No archive,
Node installation, or npm dependency tree is stored here.

| Directory | Package | Version | Icons | Aliases | Metadata license |
| --- | --- | --- | ---: | ---: | --- |
| `lucide/` | `@iconify-json/lucide` | `1.2.145` | 1966 | 220 | ISC |
| `twemoji/` | `@iconify-json/twemoji` | `1.2.5` | 4169 | 559 | CC-BY-4.0 |

Each directory contains the original `icons.json`, `info.json`, and
`package.json`. Counts above count map entries, including hidden entries;
they are not the human-facing `info.total` count.

`lock.json` records exact versioned registry and tarball URLs, npm SHA-512
integrity, SHA-256 and size of all three input files, raw counts, and notice
provenance. Each tarball was downloaded with a bounded response size and its
SHA-512 checked before extracting files. The inputs also match the earlier
research cache byte for byte. npm signatures and publisher attestations were
not verified.

## Notices and their correspondence

Iconify's individual packages split rendering data and license information:
`icons.json` does not contain `info.json`. Neither inspected icon archive
contains a standalone license file. License URLs in `info.json` are metadata;
they do not pin an original artwork revision. See
<https://iconify.design/docs/icons/json.html> and
<https://iconify.design/docs/types/iconify-info.html>.

The separately retrieved upstream notices are complete, unedited files from
the immutable revisions recorded in the lock. Their Git blob SHA-1 values were
checked against the respective GitHub tree, and their SHA-256 values are
recorded for subsequent local verification.

- Lucide: `lucide/LICENSE`, from
  <https://github.com/lucide-icons/lucide/tree/7b0196e249ca09ca27c584c497aca9a8cab8a7f5>.
  This includes ISC terms and the additional MIT terms and icon list for
  Feather-derived artwork. The ISC metadata label alone omits that detail.
- Twemoji: `twemoji/LICENSE`, `twemoji/LICENSE-GRAPHICS`, and the complete
  `twemoji/UPSTREAM-README.md`, from
  <https://github.com/jdecked/twemoji/tree/b6b55fef1e8636b540a6d016a4729ca8cdf2e60b>.
  The README distinguishes MIT code from CC-BY-4.0 graphics and includes the
  upstream attribution guidance. Keeping the code notice does not change the
  graphics license.

These revisions identify the notice snapshots, not a verified exact match to
the artwork revision used by Iconify for these npm releases. The original
artwork revision was not independently established. The npm `gitHead` values
are retained as registry metadata and must not be read as Lucide or Twemoji
artwork commits. No per-icon license correspondence beyond the retained
upstream statements has been independently established.

Retain the author/license information and full applicable notices when
redistributing generated collections or an application containing the artwork.
Twemoji attribution identifies Twitter and the Twemoji contributors, with
graphics under CC-BY-4.0; preserve its license link
<https://creativecommons.org/licenses/by/4.0/>. These source files preserve the
Iconify-distributed data; generation into Go is an additional packaging step.
This directory does not establish that a downstream application's notices or
user interface have already been updated.

## Lock shape

The top-level keys are `schemaVersion`, `retrievedAt`, and `collections`.
Each collection has `prefix`, `package`, `version`, `registryMetadataURL`,
`tarballURL`, `integrity`, `integrityVerified`, `packageGitHead`, `files`,
`counts`, and `license`.

`files` maps `icons.json`, `info.json`, and `package.json` to
`{sha256, bytes, archivePath}`. `license.metadata` is copied from the
package's `info.json`; `license.notices` records each retained file, original
URL and path, upstream repository and revision, SHA-256, Git blob SHA-1, and
size. SHA-256 values apply to raw file bytes, including their original line
endings.

The independent resolver/transform oracle is in
[`../testdata/upstream/`](../testdata/upstream/README.md). Its expected output
comes from executed, integrity-verified Iconify Utils 3.1.7, not from the Go
implementation.
