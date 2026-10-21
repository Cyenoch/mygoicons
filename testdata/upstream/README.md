# Iconify Utils resolver and SVG oracle

Generated on 2026-10-16 by executing the published JavaScript from the
integrity-verified `@iconify/utils` **3.1.7** archive in an isolated temporary
directory with Node.js **v24.22.0**. No npm install, global install, additional
runtime package, or Go module dependency was needed.

The exact published entrypoints were `lib/icon-set/get-icon.js` (`getIconData`)
and `lib/svg/build.js` (`iconToSVG`). The generation script imports them
directly from an extracted package; it does not import the package's broader
loader entrypoint or reimplement any merge or transform logic.

- Versioned package metadata:
  <https://registry.npmjs.org/@iconify%2futils/3.1.7>
- Exact executed archive:
  <https://registry.npmjs.org/@iconify/utils/-/utils-3.1.7.tgz>
- Package integrity, executed entrypoint digests, fixture digests, runtime,
  and case counts: [`provenance.json`](provenance.json).
- Full utility license, copied from that same verified archive:
  [`LICENSE-iconify-utils.txt`](LICENSE-iconify-utils.txt).
- Related upstream documentation:
  <https://iconify.design/docs/libraries/utils/get-icon-data.html> and
  <https://iconify.design/docs/libraries/utils/icon-to-svg.html>.

## Files and exact JSON shape

`alias-transform.input.json`:

```text
{
  schemaVersion: 1,
  collection: IconifyJSON,
  cases: [{ id: string, icon: string, customisations?: object }]
}
```

`alias-transform.expected.json`:

```text
{
  schemaVersion: 1,
  results: [{
    id: string,
    resolved: IconifyIcon | null,
    svg: {
      attributes: { width?: string, height?: string, viewBox: string },
      viewBox: [number, number, number, number],
      body: string
    } | null
  }]
}
```

`resolved` is the unmodified `getIconData` return value. It is sparse:
missing defaults and canceled transforms can be absent, and a negative
rotation can remain negative until rendering. Do not add fields to the golden
file to make it match a fully normalized Go representation.

`svg` is the unmodified `iconToSVG` return value. Its `viewBox` is the numeric
`[left, top, width, height]` array; `attributes.viewBox` is the serialized
attribute string. The web defaults can produce `em` sizes. These attributes
are upstream oracle output, not a native UI sizing policy. Object key order
is not a JSON semantic requirement; body strings preserve upstream transform
order and the placement of definitions exactly. This oracle does not run
ID replacement or full `<svg>` serialization.

## Coverage and verification

The handcrafted collection has three base icons and nine aliases. Twelve
cases produce nine resolved icons and three null results, covering:

- Collection defaults combined with the implicit height of 16.
- Nonzero and explicitly zero origins, nonsquare and fractional geometry.
- Alias chains, dimension overrides, flip XOR, rotation addition and
  cancellation, and negative rotations.
- Both flips becoming an additional half-turn.
- Definitions kept outside the added transform group.
- Caller customisations applied as a second transform stage.
- Missing icons, missing parents, and circular aliases returning null.

The official generation command was run repeatedly and produced identical
bytes. Separate checks verified known zero-origin and default viewBoxes,
transform cancellation, definitions placement, and the three null results.
No expected SVG body was computed by a local Go or handwritten transform
implementation.

## Reproduction

Download the exact archive outside the repository and verify its SHA-512
against `provenance.json` before extracting. The argument is the extracted
`package/` directory containing `package.json` and `lib/`.

From this directory, run:

```sh
node generate-oracle.mjs /absolute/path/to/verified-utils/package > /tmp/iconify-oracle.expected.json
cmp alias-transform.expected.json /tmp/iconify-oracle.expected.json
```

The script checks the package name and version; integrity verification is a
separate prerequisite. It only writes expected JSON to stdout. Normal Go
tests consume the checked-in JSON and do not invoke this script or Node.

The archive checksum was checked against npm metadata; publisher signatures
and attestations were not verified. This execution is tied to the published
archive, with no claim that a separately inspected Git commit is byte-identical
to that release. The fixture tests data and SVG construction semantics,
not MyGo rendering, visual fidelity, color resolution, or SVG feature support.
