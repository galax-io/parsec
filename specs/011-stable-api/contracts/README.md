# Contracts: A stable API

**Feature**: 011-stable-api | **Date**: 2026-09-12

v0.1.0 turns documentation into contract, so the contracts here are of two kinds: one that *is* the
freeze, and four that make what is frozen true before the tag makes it permanent.

| # | Contract | What it settles | Issues |
|---|---|---|---|
| 1 | [public-api.md](./public-api.md) | the 275 identifiers that are promised, the compatibility rule, what is withdrawn first, and the three decisions #13 left open | #13, #107, #77 |
| 2 | [enum-rendering.md](./enum-rendering.md) | one rule and one idiom for eleven exported enums | #78 |
| 3 | [bounds.md](./bounds.md) | what an item with no recorded end does to the run's span | #103 |
| 4 | [wrong-format.md](./wrong-format.md) | what a codec returns when handed the other format's log | #84, #107 |
| 5 | [release-path.md](./release-path.md) | pinned actions, guarded dispatch inputs, a private reporting channel | #93, #101 |

## Not a contract, but on the frozen surface

Four issues change doc comments rather than behaviour, and Principle V freezes a doc comment with the
identifier it documents. They are specified in [spec.md](../spec.md) (User Story 5) and need no
separate contract because nothing a program can observe changes:

- **#79** — `SyntaxError.Error`'s comment says it names a line while rendering a byte offset for a
  binary log, and `TruncationError` states one fact twice. The third defect the issue names, the
  stray `//` in `binary.NewReader`'s comment, is already fixed on `main` (research
  [R9](../research.md#r9--79-is-two-defects-not-three)); a lint replaces it.
- **#85** — no exported reader says it may be used by one goroutine at a time, although the text
  interner is a map and a concurrent `Next` can end the process.
- **#86** — the two `Warning` types never mention each other, one `simlog` call apart.
- **#96** — the package overviews route a newcomer away from the entry point.

## Not a contract, and not on the surface

- **#95** — six tests that cannot fail, or stop testing what they name. Principle III.
- **#97** — the changelog's run-ordering rule contradicts the code and itself.
- **#99** — the README is a project narrative rather than usage documentation.

## What no contract here changes

The supported Gatling ranges, the version gate's decisions, `Capabilities`, format detection, the
binary reader's memory budget and the endings contract. All are v0.0.10 behaviour, and the contracts
of specs 007 through 010 continue to hold unchanged.
