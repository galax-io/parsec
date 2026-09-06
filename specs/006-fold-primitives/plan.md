# Implementation Plan: The Primitives a Consumer Folds

**Branch**: `006-fold-primitives` | **Date**: 2026-09-06 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/006-fold-primitives/spec.md`

**Milestone**: [v0.0.6 The primitives a consumer folds](https://github.com/galax-io/parsec/milestone/7) ·
**Issues**: [parsec#8](https://github.com/galax-io/parsec/issues/8), [#56](https://github.com/galax-io/parsec/issues/56), [#57](https://github.com/galax-io/parsec/issues/57), [#55](https://github.com/galax-io/parsec/issues/55)

## Summary

The stream a consumer reads today hands over a path, a name and timestamps, and leaves two
definitions to whoever folds it: what a request position is, and where a run begins and ends. Two
consumers that derive those differently disagree about what is measured, not how. This feature adds
the two definitions to `model` — a `Position` that is one comparable value, and a `Bounds` the
consumer extends one item at a time — and nothing else: no count, no mean, no percentile. The
arithmetic stays in galaxio-cli, whose #51 is waiting on exactly these two types.

Both were checked against the recordings before being designed. Folding the 3.15.1 run through the
bounds rule reproduces its console summary's throughput to the digit, and the hand-rolled fold the
verification suite has kept since spec 002 becomes the independent second fold the spec requires.
The design decisions are in [research.md](./research.md), R1 and R2.

The milestone also carries three fixes to v0.0.5. Two of them turned out smaller than filed: the
"nil for the first record" divergence in #56 does not reproduce on `main`, and the model's own
absent-time representation was the real gap behind it. One of them touches observable behaviour
and is an ask-first item, marked as such below.

## Technical Context

**Language/Version**: Go 1.25 (`go.mod` is authoritative)

**Primary Dependencies**: standard library only. Everything here lands in `model/`, `gatling/`,
`gatling/text/`, `gatling/binary/` and `internal/wire/`, all of which the `deps` job holds to the
standard library plus this module. The exported-surface golden uses `go/parser`, which is standard
library.

**Storage**: N/A — artefacts are read through `io.Reader`; nothing is persisted.

**Testing**: stdlib `testing`, table-driven. Unit tests in `model` for both new types and for the
exported surface; an example in `model` that reads the 3.15.1 recording and prints the console
summary's figures; the integration suites of both codecs gain a third fold and, for the binary runs,
the console throughput check; fixtures built by `gatling/binary`'s test `builder` for the malformed
inputs and the large counts, named as fixtures. `go test -race -shuffle=on ./...`; integration
behind `-tags=integration`. No new recording — the five runs already in the corpus are the evidence.

**Engineering guidance**: this change triggers every required-reading row — it adds exported
identifiers, exported types, tests and doc comments, and changes an error path. `golang-naming` and
`golang-structs-interfaces` were read for this plan and shaped the API (constructors, the kind enum,
receivers, zero values); `golang-error-handling`, `golang-testing` and `golang-documentation` are
read before the code. Consult rows whose occasion has arisen: `golang-benchmark` (the goal below),
`golang-safety` (the ceilings, and copying out of the reader's reused slice). Forbidden as always:
testify and the `samber/*` family. No disagreement with the constitution was found; details in
[research.md R13](./research.md).

**Target Platform**: any Go 1.25 target; consumed as a library by galaxio-cli, the comet sidecar and
the Galaxio backend.

**Project Type**: library (Go module `github.com/galax-io/parsec`)

**Performance Goals**: the consumer's whole pass — decode to model, take a position for every sample
and group, extend the bounds for every item — stays under twice the decode-to-model time per item on
the synthetic 64 MiB log, which is above four million items a second on the reference machine; at
most one allocation per position and none per `Extend`; peak memory under the 32 MiB budget and
unmoved by a tenfold longer log.

**Measured on `main` at f454217** (Apple M-series, Go 1.25, `-benchtime=2s`):

| | ns/op | items/op | ns/item | allocs/op |
|---|---|---|---|---|
| `binary BenchmarkDecodeToModel/synthetic-64MiB` | 468,440,292 | 4,357,716 | **107** | 21 |
| `text BenchmarkRunReader/synthetic-64MiB` | 183,688,559 | 1,315,860 | **140** | 32 |

**Measured with the fold** (same machine, same flags, on the #8 commit):

| | ns/op | items/op | ns/item | allocs/op |
|---|---|---|---|---|
| `binary BenchmarkDecodeToModel/synthetic-64MiB` | 475,585,425 | 4,357,716 | **109** | 21 |
| `binary BenchmarkFold/synthetic-64MiB` | 671,470,764 | 4,357,716 | **154** | 1,743,108 |
| `binary BenchmarkDecodeToModel/corpus` | 21,313 | 132 | 161 | 48 |
| `binary BenchmarkFold/corpus` | 30,102 | 132 | 228 | 162 |

The fold costs 1.41× the decode-to-model time per item, under the 2× goal. The allocation count is
exactly one per position and none per `Extend`: the synthetic log takes a position on two of every
five items, 1,743,086 of them, against 1,743,108 allocations; the corpus run takes 114 (102 samples,
12 groups) against 162 − 48. Peak memory folded through the primitives stays under the 32 MiB budget:
3.6 MiB on a 256 MiB synthetic log and 3.7 MiB on one ten times longer, 174,308,736 items
(`TestFoldPeakMemory`). The interning fallback in [research.md R12](./research.md) was therefore not
taken.

**Constraints**: nothing here retains an item; a position must outlive the reader's reused `Groups`
slice without the consumer copying; the bounds must equal the span Gatling's report used, which is
bounded by virtual-user events and not only by requests; a time the source could not resolve must be
visible as absent in the model and must not move a bound; both codecs must give one answer to the
three malformed inputs; every golden record stream stays byte-identical; the public API may change
before v0.1.0 but every observable change is recorded and, where it changes behaviour, approved
first.

**Scale/Scope**: two new types and one exported constant; four documentation sentences on existing
fields; one shared conversion function changed; three parser branches in `gatling/text` that stop
refusing; two constants added and one renamed in `gatling/binary`; a README, two comments and one
recording note. Five corpus runs, no new recording, two fixtures.

## Constitution Check

*GATE: passed before Phase 0; re-checked after Phase 1 — see [Post-design re-check](#post-design-re-check).*

Source: `.specify/memory/constitution.md` **v2.2.0**. This feature does not amend it.

- [x] **I. Canonical Model First** — the two new values live in `model`, are derived from fields
      every source provides, and add no capability. No tool package exports a result type. Nothing
      computes a count, a mean, a percentile, a range or a series: `Bounds` takes the earliest and
      latest instants at which things happened, which the constitution names as this module's
      definition of where a run begins and ends; it never takes an extreme of a duration. The
      exported surface is pinned by a golden so an addition is deliberate (R9). What the source
      cannot resolve is reported absent — the zero time — never as a plausible instant (R3).
- [x] **II. Version-Gated, Streaming Decoders** — no artefact read changes shape. The gate, the
      `io.Reader` entry points and the chunked-equals-whole guarantee are untouched; positions and
      bounds are pure functions of the items, so they inherit it. Every count read from a file keeps
      a ceiling, now one per count (R7); a corrupt count is still refused at its offset before it
      sizes anything. No panic, no `recover`.
- [x] **III. Golden-Corpus Testing** — the five recordings already in the tree are the evidence:
      every count is held to each run's own report exactly, and the bounds to the span-derived
      figure each run kept — `stats.json` rates for the text runs, the console summary's throughput
      for the binary runs — with a documented tolerance of zero. The two folds required by the spec
      are the hand-rolled tally that already exists and the primitive one (R11). Each fix carries a
      regression test that fails on `main` (R6, and the ceiling tests); the one claim that does not
      reproduce gets a pinning test and no code (R5). Fixtures are built by the test builder and
      named as fixtures. Coverage floors inherited: 90% for `model`, `gatling/text`, `gatling/binary`.
- [x] **IV. Minimal, Explicit Dependencies** — stdlib only; `go.mod` unchanged; no `replace`.
- [x] **V. Compatibility-Sensitive Public API** — every addition is in
      [contracts/model-fold.md](./contracts/model-fold.md) with the doc comment it will carry. Two
      observable changes were ask-first and **were approved by the maintainer on 2026-09-06**: the
      zero-time convention on four existing fields (R3), and `gatling/text` reporting three malformed
      values absent instead of refusing (R5). Both are drafted under Changed. Pre-v0.1.0, so no deprecation
      window applies.
- [x] **VI. Idiomatic, Simple Go** — two small value types with useful zero values; no interface,
      no option, no `Span()` helper, no `Item.Position()` convenience until a need appears; errors
      stay values with line numbers and offsets; `.golangci.yml` unchanged. The required-reading
      skills for naming and types were read and applied, and no disagreement with the constitution
      was found (R13).
- [x] **Workflow** — milestone v0.0.6; four issues, four commits, each green on its own, in the
      order [contracts/gatling-fixes.md §5](./contracts/gatling-fixes.md) gives; spec artefacts
      committed first as `docs(speckit): add 006-fold-primitives spec/plan/tasks`.

**No gate fails, so Complexity Tracking is empty.**

## Project Structure

### Documentation (this feature)

```text
specs/006-fold-primitives/
├── plan.md              # This file
├── research.md          # Phase 0 — 13 decisions, 3 questions carried forward
├── data-model.md        # Phase 1 — the key encoding, the bounds rules, absence, ceilings, goldens
├── quickstart.md        # Phase 1 — 9 runnable validation scenarios
├── contracts/
│   ├── model-fold.md    #   Position, Bounds, AbsentTimestamp, the zero-time convention, changelog
│   └── gatling-fixes.md #   #56, #57, #55 as observable changes, tests, changelog, commit order
├── checklists/
│   └── requirements.md  # from /speckit-specify — 16/16
└── tasks.md             # Phase 2 — /speckit-tasks, not created here
```

### Source Code (repository root)

```text
model/
├── position.go                     # NEW  Position, PositionKind, NewSamplePosition, NewGroupPosition
├── bounds.go                       # NEW  Bounds: Extend, Start, End
├── sample.go                       #  +   Sample.Position, GroupSample.Position; zero-time sentence on Start/At
├── run.go                          #  +   zero-time sentence on RunError.At
├── doc.go                          #  +   "The primitives a consumer folds"
├── position_test.go  bounds_test.go  exports_test.go        # NEW
├── example_fold_test.go            # NEW  Example_fold over the 3.15.1 recording, via gatling/simlog
└── testdata/exports.golden         # NEW  the exported surface, one identifier per line

gatling/
├── record.go                       #  +   AbsentTimestamp; Record field docs name it
├── format.go  format_test.go       #  ~   provenance comments cite the 3.15.1 recording (#55)
├── text/parse.go                   #  ~   negative times → AbsentTimestamp; negative cumulated kept (#56)
├── text/*_test.go                  #  +   negative-field table; primitive tally beside modelTally
├── binary/record.go                #  ~   absentTime → gatling.AbsentTimestamp; maxScenarios, maxAssertions, initialPathCap (#57)
└── binary/*_test.go                #  +   TestCodecsAgreeOnMalformedInput; large-count fixtures; primitive tally;
                                    #      console throughput check; TestFoldPeakMemory; BenchmarkFold

internal/wire/wire.go               #  ~   Millis: negative → zero time; span: absent end → unset

README.md                           #  ~   the binary codec row and Status (#55)
testdata/corpus/gatling/3.15.1/RECORDING.md   #  +   one line: what the log opens with (#55)
CHANGELOG.md                        #  +   Unreleased: Added / Changed / Fixed, drafted in contracts/
```

**Structure Decision**: both primitives are `model` types because they are definitions every source
shares and every consumer buckets and divides by; putting either in a codec would make two of them.
The sentinel moves up from `gatling/binary` to `gatling` because both codecs write it and the wire
records are exported. No new package: the agreement test lives in `gatling/binary`'s tests, which
already import the text codec and own the log builder (R6), and the example lives in `model`'s
external test package, which may import `gatling/simlog` without making `model` depend on anything.

## Post-design re-check

Re-read Principles I–VI against the Phase 1 artefacts. All still pass; four points moved during
design and are worth recording.

- **I — the line between a definition and a statistic was drawn in one sentence, and it holds.**
  `Bounds` is a minimum and a maximum, and the objection that a minimum is a statistic was
  considered. The answer is where it is taken: over *when* things happened, which is where a run
  begins and ends, never over *how long* they took, which is a response-time extreme and stays in
  galaxio-cli. The doc comment says so, and the exported-surface golden makes any drift a diff.
- **I — absence in the model was the real gap behind #56.** The issue asked for one answer from
  two codecs; giving it exposed that the model could not show an absent start at all, and would have
  rendered the agreed answer as a date 292 million years ago. R3 closes that with the standard
  library's own convention and no shape change, and it is the one place this plan needs approval
  before code.
- **III — the corpus decided two things reading could not.** The nil-path divergence does not
  exist on `main`, so it gets a pinning test and no fix; and the binary runs' console summaries
  carry the throughput that makes FR-015 checkable for them, which the suite had not used until
  now. Both are exactly the kind of fact the constitution wants taken from a recording rather than
  from an issue's text.
- **VI — three things were left out on purpose.** A `Span()`, an `Item.Position()` and a position
  interner were each argued for and each rejected until a consumer or a benchmark asks; the
  fallback for the third is written down with the measurement that would justify it.

What the design could not settle on its own — whether the maintainer prefers the conventional
zero-time representation or the structural `Opt[time.Time]` (R3), and whether the text codec's three
refusals may become values (R5) — was put to the maintainer and **approved on 2026-09-06**, so
nothing in this plan waits on an answer.

## Complexity Tracking

No Constitution Check gate failed. This section is intentionally empty.
