# Implementation Plan: A stable API

**Branch**: `011-stable-api` | **Date**: 2026-09-12 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/011-stable-api/spec.md`

## Summary

Freeze the public surface at v0.1.0 and make what is frozen honest. The surface is inventoried by a
generator rather than described — 278 exported identifiers today, 275 after two are withdrawn
(`gatling.Gate`, `gatling.MaxRunStart`) and two duplicates collapse into one (`text.Tool` and
`binary.Tool` become `gatling.Tool`) — and the inventory is held by a golden file so that any later
drift is a red test. Around that list, sixteen issues make the frozen thing true: one rendering rule
for eleven enums instead of two conventions in three idioms, one copy of the run-header construction
instead of two, a wrong-codec failure that says "wrong format" instead of "damaged log", a `Bounds`
span that never ends before something it counted began, doc comments that describe the code beneath
them, package overviews that route to the entry point, six tests that can go red, a README a stranger
can work from, a `SECURITY.md`, and a release path whose actions cannot move underneath it.

The approach is deliberately unambitious in mechanism: no new dependency, no new package, no new
exported identifier except the one `Tool` that replaces two. The only new machinery is a
stdlib-`go/parser` test that reproduces the inventory, and it exists because `gorelease` — the
compatibility gate #106 already built — reports what changed and never reports what the surface *is*.

## Technical Context

**Language/Version**: Go 1.25 (`go.mod` is authoritative; `toolchain go1.26.8`)

**Primary Dependencies**: standard library only. No module is added. The surface generator uses
`go/parser`, `go/ast` and `go/token`, all stdlib, in a test.

**Storage**: N/A. One new file under `testdata/api/` holds the golden surface inventory; nothing is
persisted at run time.

**Testing**: stdlib `testing`, table-driven; golden corpus under `testdata/corpus/gatling/<version>/`,
unchanged and unextended by this feature; `go test -race -shuffle=on ./...`; the shell gates test
themselves (`scripts/*_test.sh`, `.githooks/*_test.sh`). Two new kinds of assertion: the surface
golden file, and a doc-comment lint over every exported identifier.

**Engineering guidance**: every *required reading* row is triggered — `golang-naming` (exported
identifiers are withdrawn and one is added), `golang-error-handling` (`UnsupportedFormatError` gains
producers and a message), `golang-testing` (six tests are repaired structurally),
`golang-documentation` (doc comments change in all six packages), `golang-structs-interfaces` (the
two `simlog` interfaces are frozen two-sided). Of the *consult* rows, `golang-safety` and
`golang-security` apply: the module decodes untrusted input, and #93 changes the job that publishes.
`golang-benchmark` is not triggered — no throughput or memory figure changes. No skill disagrees
with the constitution: `golang-testing`'s `testify` recommendation stopped being a prohibition on
2026-09-12 (constitution v2.4.0) and is now a Principle IV question. Research
[R18](./research.md#r18--skills-read-and-the-testify-question) records the judgement for this
feature — no, because nothing in the six test repairs asks for it, not because a rule says so.

**Target Platform**: any Go 1.25 target; consumed as a library by galaxio-cli, the comet sidecar and
the Galaxio backend

**Project Type**: library (Go module `github.com/galax-io/parsec`)

**Performance Goals**: unchanged. No decode path changes except the `bufio.Reader.Peek` of at most
`gatling.DetectSize` (10) bytes that `binary.NewReader` takes before its first `u8` — a peek into a
buffer already filled, costing no additional read (research [R8](./research.md#r8--how-a-codec-detects-the-other-format-and-what-it-returns)).

**Constraints**: the v0.1.0 compatibility promise is the deliverable, so every identifier this
feature withdraws must go **before** the tag; after it, each costs a `// Deprecated:` window and a
MINOR release. Every such PR carries the `breaking` label and an `[Unreleased]` changelog bullet, or
`scripts/check-compat.sh` fails it (research [R2](./research.md#r2--the-gate-a-surface-change-already-has-to-pass)).
Streaming, bounded memory, version gating and chunked-equals-whole-file are v0.0.10 behaviour and are
not touched.

**Scale/Scope**: six packages, 278 → 275 exported identifiers; eleven exported enums; six reader
surfaces; six package overviews; six tests; six workflow files and one new shell gate; three repository-root documents
(`README.md`, `CHANGELOG.md`, new `SECURITY.md`) and two GitHub settings.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Source: `.specify/memory/constitution.md` **v2.4.0** (ratified 2026-09-02, last amended 2026-09-12).
This feature amends nothing in it.

- [x] **I. Canonical Model First** — no result data is added or changed. `model.Bounds`'s definition
      moves (an end-less item extends the end), which is a definition this module owns, not
      arithmetic: no count, mean, percentile, range or series is computed here. No tool package gains
      a consumer-facing result type; `gatling.Tool` is a constant naming the source, already present
      twice. `Capabilities` is untouched and stays per-codec.
- [x] **II. Version-Gated, Streaming Decoders** — the gate is unchanged in what it decides and when
      it runs. `Gate` becomes `gate`; `Policy.Apply` remains the entry point and gains the test that
      used to drive `Gate` directly. The wrong-format detection reads no byte the codec had not
      already read. No panic/recover is introduced.
- [x] **III. Golden-Corpus Testing** — no version is added, so no corpus entry is recorded; the
      existing `3.11.5` and `3.14.9` runs become the wrong-codec fixtures. Six tests that cannot fail
      are repaired, and each repair is demonstrated by breaking the production code and watching the
      test go red. Coverage floors (90% decoder packages, 80% overall) are unchanged and must hold.
- [x] **IV. Minimal, Explicit Dependencies** — no module is added. `go mod tidy` must leave the tree
      unchanged. The surface generator is `go/parser` from the standard library.
- [x] **V. Compatibility-Sensitive Public API** — this feature *is* Principle V's subject. Two
      identifiers are withdrawn and two collapse to one, all before v0.1.0, each recorded under
      **Removed**; `String()` output for five `model` enums changes, recorded under **Changed**;
      `Bounds.End()` moves, recorded under **Changed**; `UnsupportedFormatError`'s message changes and
      the type gains producers, recorded under **Changed**. Each carries the `breaking` label.
      Deprecation windows are not needed and not available: Principle V allows removal without one
      only while the module is below v0.1.0, which is exactly the window this feature works in.
- [x] **VI. Idiomatic, Simple Go** — the feature removes duplication rather than adding abstraction:
      one `Tool`, one warning builder, one rendering idiom. `.golangci.yml` is unchanged. The three
      `String()` idioms collapse to the one already used by four of the eleven enums, which is
      Principle VI's "follow the convention already in the codebase". Required-reading skills are
      named above and the one disagreement is recorded in research.md.
- [x] **Workflow** — milestone **#11 v0.1.0 A stable API**, confirmed by `scripts/check-linkage.sh`.
      Spec artifacts commit as `docs(speckit): add 011-stable-api spec/plan/tasks` before any
      `feat`/`fix`. Sixteen issues map to sixteen green commits.

**Re-check after Phase 1**: unchanged. The Phase 1 design adds no exported identifier beyond
`gatling.Tool`, no package, and no dependency; see [contracts/](./contracts/).

## Project Structure

### Source Code (repository root)

```text
doc.go                              # overview: add gatling/run, name the three-call path
model/                              # five enums re-render; Bounds.cover; Warning cross-link; doc.go
model/bounds.go                     #   cover extends the end; End()'s guard and paragraph go
model/capability.go                 #   Field's table is the idiom the other four adopt
model/run.go  model/sample.go  model/position.go
gatling/                            # Gate -> gate; MaxRunStart out; Tool in; errors reworded
gatling/version.go  gatling/policy.go  gatling/record.go  gatling/format.go  gatling/errors.go
gatling/text/                       # loses Tool and its NewRunReader body; gains the format check
gatling/binary/                     # loses Tool and its NewRunReader body; Peek before u8
gatling/simlog/                     # interfaces frozen; concurrency sentence; doc.go routes
gatling/run/                        # FoundBy unchanged in idiom; doc.go doc-links
internal/wire/                      # gains MaxRunStart, Warnings, Run
api_test.go                         # NEW: the surface inventory and the doc-comment lint
testdata/api/surface.txt            # NEW: the golden inventory, 275 identifiers at the tag
testdata/corpus/gatling/            # unchanged; 3.11.5 and 3.14.9 serve as wrong-codec fixtures
.github/workflows/*.yml             # SHA pins; env+regex guards on dispatch inputs
scripts/check-pins.sh               # NEW: the gate that holds the pins, with its own shell test
README.md  CHANGELOG.md  SECURITY.md
```

**Structure Decision**: no package is added and none is renamed. One shell gate is added,
`scripts/check-pins.sh` with `scripts/check-pins_test.sh` beside it, in the shape of the four
`check-*.sh` pairs already there: #93's SHA-pin rule is otherwise held by review, and a rule held by
review is what let four actions sit on movable tags in the first place. `internal/wire` grows by three
identifiers (`MaxRunStart`, `Warnings`, `Run`) because it is already the place the record-to-model
mapping lives and both codecs import it; it stays internal because none of the three is something a
consumer calls — the constant is a codec's refusal ceiling and the two functions are the codecs'
shared construction. The one new test file sits at the module root, in package `parsec_test`, because
it must see all six packages at once and belongs to none of them.

## Complexity Tracking

No Constitution Check gate failed. Nothing to justify.
