# Contract 1 — the public API, frozen

**Applies to**: every exported identifier in `model`, `gatling`, `gatling/text`, `gatling/binary`,
`gatling/simlog` and `gatling/run`.
**Status**: this is the freeze. Issues [#13](https://github.com/galax-io/parsec/issues/13) (the
promise) and [#107](https://github.com/galax-io/parsec/issues/107) (its concrete scope),
[#77](https://github.com/galax-io/parsec/issues/77) (one `Tool`).

## The promise

From the `v0.1.0` tag:

- Changing the signature or the observable behaviour of any identifier listed below, or any format
  this module writes, is a **breaking change**. It is called out in a spec, approved before
  implementation, recorded in `CHANGELOG.md`, and released as a new **MINOR** version while the
  module is at v0.x.
- A superseded identifier keeps working for at least one MINOR release and carries a
  `// Deprecated:` comment naming its replacement. Removal without that window is available only
  below v0.1.0 — which is the window this feature works in, and it closes at the tag.
- Nothing outside the list is public. `internal/` is not importable and carries no promise.
- The supported Gatling range is part of the promise: **3.11.5 through 3.12.0** for the text format,
  **3.13.1 through 3.15.1** for the binary one. A 3.13.0 log is refused, although the codec could
  read it: that version writes the format and cannot generate a report, so no run of it can carry the
  second account of its own numbers a corpus entry needs (Principle III). Read the range from
  `simlog.Supported()` rather than from prose.
- `simlog.RecordReader` and `simlog.RunReader` are frozen **two-sided**: their method sets are final,
  because consumers' test doubles implement them and an added method breaks an implementer.

## What this feature withdraws, before the tag

| Identifier | Why it is not frozen | Changelog |
|---|---|---|
| `gatling.Gate` | its only production caller is `Policy.Apply`, documented as "the single place the outcomes are decided"; a second exported entry to one rule contradicts that | **Removed** |
| `gatling.MaxRunStart` | read only by the two codecs; no consumer computes with it. Moves to `internal/wire` | **Removed** |
| `gatling/text.Tool` | one value, two names, neither placement the convention | **Removed** |
| `gatling/binary.Tool` | as above | **Removed** |
| — | `gatling.Tool` replaces both, beside `Header` and `Version` | **Added** |

Each carries the `breaking` label and an `[Unreleased]` bullet, or `scripts/check-compat.sh` refuses
the merge.

## The three decisions #13 left open

| Question | Answer | Where it is stated |
|---|---|---|
| `UnsupportedFormatError` — keep or remove? | **Keep, and give it producers.** #84 needs an error meaning "a Gatling log this reader does not decode"; overloading `FormatError` would make a consumer's "not a Gatling file" branch wrong for every wrong-codec case | the type's doc comment; [wrong-format.md](./wrong-format.md) |
| `SyntaxError` — three position fields or one? | **Three, as they are.** Folding `Line` and `Offset` into one loses the type-level distinction, and `Format` is the discriminator two legitimately-zero positions need | `SyntaxError`'s doc comment |
| `Record.Line` = 0 for a binary log — contract or omission? | **The contract.** A binary record's offset serves no seek — the format cannot be resumed — and only a failure needs a position, which `SyntaxError.Offset` carries. An `Offset` field can be added compatibly later if a need appears | `Record.Line`'s doc comment |

## How the list is kept honest

`testdata/api/surface.txt` holds this list, and `exports_test.go`'s
`TestExportedSurfaceIsGolden` regenerates it with `go/parser` — `go test . -update` — and fails on
any difference. `gorelease` (through `scripts/check-compat.sh`) decides whether a change is
*compatible*; the golden file decides whether the surface is *what this contract says*. An addition
passes the first and fails the second, which is why both exist.

## The frozen surface — 275 identifiers

The list itself lives in [`testdata/api/surface.txt`](../../../testdata/api/surface.txt), one
identifier per line under a per-package header. It was embedded here too, byte for byte, which made
a second place for it to be wrong: the golden is generated from the tree and this copy was not.

Kinds, as the golden spells them: `func` package-level function · `method T.M` method on an exported
type · `imethod I.M` interface method · `type` · `const` · `var` · `field T.F` exported struct field.

Counted from the tree at `0a55f09` (278) with this feature's four withdrawals and one addition
applied. The figure that binds is the `TOTAL` line of the golden at the tag.
