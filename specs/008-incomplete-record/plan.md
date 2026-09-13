# Implementation Plan: An incomplete record

**Branch**: `008-incomplete-record` | **Date**: 2026-09-07 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/008-incomplete-record/spec.md`

## Summary

A run killed mid-flight leaves a log that ends inside a record, and both codecs answer that with the
same fatal error they give bytes that are not a Gatling log at all — so the whole run is lost
exactly when it matters most. This feature gives that ending its own name.

The approach is smaller than the milestone's original framing, because reading the code settled two
things. First, **the salvage needs no restructuring**: both readers already hand records out one at
a time as they decode, so the records before a cut are already delivered; what is missing is a
distinguishable ending and the two numbers that describe it. Second, **the growing-log half needs no
code at all** — measured against all five recordings, both codecs, a source that blocks between
appends already decodes to exactly what a whole-file read yields (research [R6](research.md)), so
#10 ships as a written contract plus the test that holds it.

The work is therefore: one new exported error type (`gatling.TruncationError`), the two numbers it
carries plumbed from data the codecs already compute, one identity-comparison defect fixed on the
way past (research [R5](research.md)), the blocking-source promise written where a consumer reads
it, and the tests that hold all of it.

## Technical Context

**Language/Version**: Go 1.25 (`go.mod` is authoritative)

**Primary Dependencies**: standard library only. No module is added, and `model/` and `gatling/`
stay stdlib-only as the `deps` job requires (Principle IV).

**Storage**: N/A — artefacts are read through `io.Reader`; nothing is persisted.

**Testing**: stdlib `testing`, table-driven; the five recordings under
`testdata/corpus/gatling/<version>/`; cut inputs derived in-test and never committed (research
[R7](research.md)); `go test -race -shuffle=on ./...`; the existing fuzz targets gain seeds.

**Engineering guidance**: required-reading rows triggered — `golang-error-handling` (a new error
type; the identity-vs-`errors.Is` decision), `golang-naming` (one new exported identifier),
`golang-documentation` (its doc comment and the contract text FR-009 requires),
`golang-structs-interfaces` (an exported struct), `golang-testing` (the cut sweep and the
blocking-source test). Disagreements recorded in research [R9](research.md): the skill set
recommends `stretchr/testify` and `samber/*`; both are forbidden here and neither is used.

**Target Platform**: any Go 1.25 target; consumed as a library by galaxio-cli, the comet sidecar and
the Galaxio backend.

**Project Type**: library (Go module `github.com/galax-io/parsec`)

**Performance Goals**: unchanged from v0.0.7 and re-measured, not assumed — peak heap under 32 MiB
for any log either codec accepts, and no throughput or allocation regression against
`gatling/binary/bench_test.go`, `gatling/text/bench_test.go`, `gatling/text/model_bench_test.go` and
`gatling/simlog/bench_test.go`. The design adds one `int64` assignment per record on the binary path
and nothing on the text path (research [R10](research.md)).

**Baseline recorded 2026-09-07 (T001)**, darwin/arm64, `-benchtime=200ms`, the codec benchmarks run
with `-tags=integration` because that is where they live:

| Benchmark | Throughput | Allocations |
|---|---|---|
| `binary.BenchmarkDecode/synthetic-64MiB` | 238.51 MB/s | 66320 B/op, 19 allocs/op |
| `binary.BenchmarkDecode/corpus` | 275.20 MB/s | 67888 B/op, 45 allocs/op |
| `binary.BenchmarkDecodeToModel/synthetic-64MiB` | 144.75 MB/s | 66496 B/op, 21 allocs/op |
| `text.BenchmarkReader/synthetic-64MiB` | 469.91 MB/s | 1062800 B/op, 30 allocs/op |
| `text.BenchmarkRunReader/synthetic-64MiB` | 340.75 MB/s | 1062992 B/op, 33 allocs/op |
| `simlog.BenchmarkOpen/dispatched` | 65.69 MB/s | 1063433 B/op, 46 allocs/op |

**After the change (T029)**, same machine and flags, with the corpus benchmarks repeated at
`-benchtime=2s -count=3` because a 200 ms run of the 64 MiB cases executes once and cannot separate
a regression from noise:

| Benchmark | Before | After | Allocations |
|---|---|---|---|
| `binary.BenchmarkDecode/corpus` | 275.20 MB/s | 268.6 / 270.8 / 272.0 MB/s | 45 allocs/op, 67888 → 67920 B/op |
| `binary.BenchmarkDecodeToModel/corpus` | 194.11 MB/s | 189.3 / 187.5 / 189.4 MB/s | 48 allocs/op, unchanged |
| `text.BenchmarkReader/corpus` | 222.38 MB/s | 220.4 / 219.6 / 218.9 MB/s | 41 allocs/op, 1063288 B/op — identical |
| `binary.BenchmarkDecode/synthetic-64MiB` | 238.51 MB/s | 229.48 MB/s | 19 allocs/op, unchanged |
| `text.BenchmarkReader/synthetic-64MiB` | 469.91 MB/s | 454.67 MB/s | 30 allocs/op, unchanged |

No regression to justify. Allocation counts are identical everywhere; the one byte-count difference
is **+32 B once per binary reader**, which is the `recordAt` field moving the `reader` struct into
the next size class — it is per read, not per record. The throughput figures move by 1–3% in both
directions, and the text codec, whose read loop this change does not touch at all, moves by the same
magnitude: that is the machine, not the change.

**Constraints**: streaming with bounded memory, including while a follower waits between appends;
chunked, delivered-over-time and whole-file reads all agree; the version gate is untouched; errors
carry the position of the failure; no panic on any input, at any cut offset.

**Scale/Scope**: Gatling 3.11.5 and 3.12.0 (text), 3.13.1, 3.14.9 and 3.15.1 (binary) — the
supported range is unchanged. Two issues, [#7](https://github.com/galax-io/parsec/issues/7) and
[#10](https://github.com/galax-io/parsec/issues/10), one commit each. 1000 cut positions exercised
(five recordings × the last 200 bytes).

## Constitution Check

*GATE: passed before Phase 0, re-checked after Phase 1 design. Source: `.specify/memory/constitution.md` v2.2.0.*

- [x] **I. Canonical Model First** — no `model/` type changes. `TruncationError` joins the four
      error types already in `gatling/` (`SyntaxError`, `VersionError`, `FormatError`,
      `UnsupportedFormatError`), which is where decoder errors live; `model/` carries none.
      `Capabilities` is untouched. **Nothing is computed**: the module states that a log was cut,
      where, and by how many bytes — all three are facts read off the artefact, not arithmetic over
      records. Whether a cut run may be used is `galaxio-cli`'s decision.
- [x] **II. Version-Gated, Streaming Decoders** — the gate is untouched and still runs before any
      record decodes, including in a cut log; the codecs' ranges still equal their corpus coverage;
      the `io.Reader` entry points and the allocation caps are unchanged; memory stays bounded while
      a follower waits (contract 2, promise 4); the new error carries a byte offset for binary and a
      line number for text; no panic and no `recover`. Chunked-equals-whole-file is extended, not
      weakened: delivered-over-time now equals whole-file too.
- [x] **III. Golden-Corpus Testing** — no version is added, so no recording is needed and none is
      taken; every input is derived from the five existing recordings, each of which already carries
      the report Gatling produced for it. Cut logs are **fixtures, not corpus** (research
      [R7](research.md)) and are produced in-test, so nothing hand-edited enters
      `testdata/corpus/`. Truncated output is compared byte-for-byte against a whole-file read of
      the same prefix; the intact recordings keep their existing golden and report comparisons
      unchanged. The R5 defect gets the regression test Principle III requires of a bug fix.
      Coverage floors (90% decoder packages / 80% module) are re-measured and reported in the PR.
- [x] **IV. Minimal, Explicit Dependencies** — nothing added; `go.mod` untouched.
- [x] **V. Compatibility-Sensitive Public API** — one addition and one observable change, both
      itemised in [contracts/public-api.md](contracts/public-api.md), with what each kind of
      consumer must do. Permitted before v0.1.0, and recorded under **Changed** in `CHANGELOG.md` in
      the same PR. Every new exported identifier carries a doc comment.
      **Approved 2026-09-07.** AGENTS.md requires asking before changing observable behaviour of
      a published API; the maintainer confirmed the change to how a cut-short read ends after the
      spec recorded the decision (Assumptions) and this plan its consequences. Implementation may
      proceed on that basis, and `CHANGELOG.md` records it under Changed.
- [x] **VI. Idiomatic, Simple Go** — one type, no interface, no abstraction beyond what the current
      spec needs; no `Unwrap` written on speculation (research [R2](research.md)); errors stay
      values; the identity comparison in R5 makes one file agree with the three that already state
      the rule. `.golangci.yml` unchanged.
- [x] **Workflow** — the feature belongs to milestone **v0.0.8 An incomplete record**; these spec
      artifacts are committed as `docs(speckit): …` before any implementation commit; #7 and #10 map
      to one green commit each.

No row fails. Complexity Tracking is empty.

## Project Structure

### Source Code (repository root)

```text
gatling/
├── errors.go                     # + TruncationError, its doc comment and Error()
├── errors_test.go                # + its message and its non-identity with io.EOF
├── binary/
│   ├── read.go                   # truncated() builds the new error; sourceFailed compares
│   │                             #   with == (R5); reader tracks the record's start offset
│   ├── reader.go                 # Next records the start offset; doc comments restated
│   ├── model.go                  # RunReader doc: the cut passes through unchanged
│   ├── truncation_test.go        # new — the cut sweep, the position and the byte count
│   ├── fuzz_test.go              # + seeds: prefixes of the recordings (R8)
│   └── bench_test.go             # unchanged; re-run to show the budget did not move
├── text/
│   ├── reader.go                 # unterminated() builds the new error with the dropped
│   │                             #   count the scanner already returns; docs restated
│   ├── model.go                  # RunReader doc: the partial run is no longer "refused"
│   ├── truncation_test.go        # new — the same sweep over the fixtures, and the
│   │                             #   source-failure and clean-end regressions
│   ├── truncation_corpus_test.go # new — the sweep over the recordings, integration tag
│   ├── scan.go doc.go            # the identity rule, and the package overview
│   └── mutation_test.go          # + seeds for FuzzReader (R8)
└── simlog/
    ├── simlog.go                 # package and interface docs state contract 2
    ├── example_test.go           # the rendered example keeps a cut-short run
    ├── truncation_test.go        # new — a cut log through this entry point
    └── follow_test.go            # new — the blocking-source test (the R6 probe)
CHANGELOG.md                      # Changed: the ending of a cut-short read
```

**Structure Decision**: no package is added and no file moves. The new type belongs in `gatling/`
because that is where every decoder error already lives and both codecs must raise it; the two
codecs keep their own detection because the position they report is in their own coordinates — a
line number for text, a byte offset for binary. `gatling/simlog` gains no code, only the
documentation that states contract 2 and the test that holds it, because it is the entry point a
follower actually calls.

## Complexity Tracking

No constitution gate fails; nothing to justify.
