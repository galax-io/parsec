# Implementation Plan: Finding the run

**Branch**: `009-gatling-run-discovery` | **Date**: 2026-09-08 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/009-gatling-run-discovery/spec.md`

## Summary

Three consumers each work out where a Gatling run's `simulation.log` lives, and they are about to
work it out differently. This feature answers the question once: `gatling.FindRun` takes a path that
may be a run, a results root, or nothing, and returns the run directory, the log inside it, and
which rule chose it. It opens no log and runs no version gate — it stops exactly one line short of
`simlog.NewRunReader`.

Reading the primary sources changed the shape of the work in one important way. The spec treated
`lastRun.txt` as Gatling's own record, consulted first with modification time as a fallback. It is
not Gatling's: only `gatling-maven-plugin` writes it, the Gradle plugin never does, and
`gatling:verify` **deletes** it during an ordinary Maven build (research
[R1](research.md#r1--lastruntxt-is-written-by-the-maven-plugin-not-by-gatling),
[R2](research.md#r2--it-is-also-short-lived-so-modification-time-is-the-ordinary-path)). So the
fallback is the main path for almost every caller, and the weight of the feature moves onto the
ordering rule — which is why it is a total order, mtime then name descending, and why that is
correct rather than merely deterministic: a Gatling run directory ends in `yyyyMMddHHmmssSSS`, so
name order *is* run order (research
[R4](research.md#r4--a-run-directory-is-named-simulationid-yyyymmddhhmmsssss),
[R6](research.md#r6--the-tie-break-is-load-bearing-not-a-formality)).

The same reading settled the format of the file itself — bare names, one per run, an optional error
line, `System.lineSeparator()` — so nothing about it is guessed, and made three of the spec's
assumptions checkable facts. It also corrected one: the run directory does not end in epoch
milliseconds. The spec has been fixed.

The work is therefore: one exported function, one result struct, one enum, one error type, all in
`gatling/`; a resolution pass that reads directory entries and at most one small text file; and the
tests, the fuzz target and the benchmark that hold it. No package is added, no dependency, nothing
in `model/`, and no existing identifier changes.

## Technical Context

**Language/Version**: Go 1.25 (`go.mod` is authoritative; the toolchain on the planning machine is
1.26.4). `os.Root` is available and deliberately not used — research
[R8](research.md#r8--containment-is-textual-so-a-symlinked-run-still-resolves).

**Primary Dependencies**: standard library only. `os`, `io/fs`, `path/filepath`, `sort`, `strings`.
No module is added, and `gatling/` stays stdlib-only as the `deps` job requires (Principle IV).

**Storage**: N/A — the filesystem is read, never written. Discovery creates, moves and deletes
nothing, which also keeps it out of `VerifyMojo`'s way (contract 2, promise 5).

**Testing**: stdlib `testing`, table-driven, over real directory trees built in `t.TempDir()` —
real `stat` results, real modification times, real symlinks, real permission errors. One integration
test behind `-tags=integration` over the `lastRun.txt` recording, skipping with a reason when Maven
is absent. One fuzz target, `FuzzLastRun`, because that file is the only parsed input.
`go test -race -shuffle=on ./...`.

**Engineering guidance**: required-reading rows triggered — `golang-naming` (four new exported
identifiers; the `model.Run` collision), `golang-error-handling` (a new error type, and the
not-found / cannot-read distinction), `golang-documentation` (four doc comments, and the three facts
FR-015 pins), `golang-structs-interfaces` (an exported struct and enum, no interface),
`golang-testing` (every change). Consulted: `golang-safety` and `golang-security` for the
containment question, `golang-benchmark` for the bound. Disagreements recorded in research
[R11](research.md#r11--skills-read-and-where-they-disagreed): the skill set recommends
`stretchr/testify` and `samber/*`; both are forbidden here and neither is used.

**Target Platform**: any Go 1.25 target; consumed as a library by galaxio-cli, the comet sidecar and
the Galaxio backend. Path handling must hold on Windows, where the plugin writes CRLF line endings
(contract 2) and where `filepath.Separator` is not `/`.

**Performance Goals**: no throughput figure — this feature decodes nothing and opens no log, so the
constitution's decoder-benchmark rule does not bite (research
[R10](research.md#r10--no-decoder-benchmark-rule-applies-a-bound-is-stated-anyway)). The bound
stated instead, and held by `BenchmarkFindRun` over a synthetic root of 1000 runs: **one directory
read, at most one `stat` per entry, one bounded read of `lastRun.txt` (64 KiB cap)**, with
allocations proportional to the entry count and independent of anything inside a run. No existing
benchmark is touched; no decoder code path changes, so none can regress.

**Measured 2026-09-08 (T036)**, darwin/arm64, `-benchtime=200ms`:

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `BenchmarkFindRun/runs=10` | 64 214 | 14 232 | 102 |
| `BenchmarkFindRun/runs=100` | 524 134 | 118 362 | 825 |
| `BenchmarkFindRun/runs=1000` | 4 932 562 | 1 166 752 | 8 028 |
| `BenchmarkFindRunLastRun` (1000 runs, pointer read) | 4 601 819 | 1 152 357 | 8 041 |
| `BenchmarkFindRunNamedPath` | 2 068 | 640 | 4 |

The bound holds: cost is linear in the entry count — roughly 8 allocations and 1.2 KB per entry at
every size — and reading `lastRun.txt` adds 13 allocations to a 1000-run root, which is the file
itself and not per-entry work. A path that already names a run costs two stats and four allocations,
listing nothing, which is the floor the other rows are measured against.

**Constraints**: no `simulation.log` is opened (FR-013) and no version gate runs (FR-014); ordering
is total and platform-independent (FR-008); nothing outside the results root is followed (FR-007);
an unreadable directory is a failure and never an absence (FR-012). No panic on any input, including
a `lastRun.txt` of arbitrary bytes.

**Scale/Scope**: Gatling 3.11.5 through 3.15.1 — the same range the codecs cover, though discovery
reads no version. One issue,
[#11](https://github.com/galax-io/parsec/issues/11), one commit. Four new exported identifiers; one
new corpus recording, made (below); no existing file's behaviour changed.

## Constitution Check

*GATE: passed before Phase 0, re-checked after Phase 1 design. Source: `.specify/memory/constitution.md` v2.2.0.*

- [x] **I. Canonical Model First** — nothing enters `model/`. A located run is two paths and the
      reason they were chosen; it is discarded the moment the log is opened, and `model.Run` remains
      the only Run this module has. No tool package exports a consumer-facing *result* type here.
      `Capabilities` is untouched — discovery adds no field a source might or might not supply.
      **Nothing is computed**: "which directory is newest" is a comparison over filesystem metadata,
      not a statistic over records, and no count, mean, percentile, range or series is derived.
- [x] **II. Version-Gated, Streaming Decoders** — N/A by construction, and stated as a requirement
      rather than assumed: FR-013 forbids opening a log and FR-014 forbids applying or pre-empting
      the gate, so a run whose log is truncated, damaged or out of range still resolves and fails
      when it is read. No codec, no `io.Reader` entry point and no chunking is added or altered.
      Memory is bounded — the entry list and a 64 KiB cap on `lastRun.txt` — and no input panics.
- [x] **III. Golden-Corpus Testing** — no version is added and no codec changes, so no decoder
      corpus entry is needed. One new recording **was made** for the one artefact this feature parses
      and the existing corpus could not supply — `testdata/corpus/gatling/lastrun/`, three real Maven
      runs — and it corrected two things reading the plugin had got wrong (see below). Everything else is built in `t.TempDir()` and is a
      **fixture, not corpus** — real directories rather than mocks, which is what Principle III asks
      when a real path exists. Tests land with the change; coverage floors (90% decoder packages /
      80% module) re-measured and reported in the PR.
- [x] **IV. Minimal, Explicit Dependencies** — nothing added; `go.mod` untouched; `gatling/` stays
      standard-library only and the `deps` job stays green.
- [x] **V. Compatibility-Sensitive Public API** — four additions, no change to any existing
      identifier, itemised in [contracts/public-api.md](contracts/public-api.md) and recorded under
      **Added** in `CHANGELOG.md` in the same PR. Permitted before v0.1.0; **no approval needed**,
      because nothing observable changes for an existing consumer — the ask-first rule covers
      changed signatures and behaviour, not additions. Names are chosen as if permanent, since
      v0.1.0 is the next milestone. Every new identifier carries a doc comment.
- [x] **VI. Idiomatic, Simple Go** — one function, one struct, one enum, one error; no interface and
      no functional options, because there is one implementation and one knob and the knob is
      already the first argument. Errors are values, wrapped with `%w` and reached with `errors.As`;
      no panic, no `recover`. `.golangci.yml` unchanged. The required-reading skills were read and
      the one disagreement is recorded in research [R11](research.md#r11--skills-read-and-where-they-disagreed).
- [x] **Workflow** — the feature belongs to milestone **v0.0.9 Finding the run** (#10); these spec
      artifacts are committed as `docs(speckit): …` before any implementation commit; #11 maps to one
      green commit.

No row fails. Complexity Tracking is empty.

**The one open decision was taken: the recording was made.** Research
[R9](research.md#r9--what-the-corpus-already-proves-and-the-one-thing-it-cannot) shows that a real
`lastRun.txt` can only come from a Maven build, and the corpus simulation project was sbt-only; a
`pom.xml` beside `build.sbt` closed that, and `testdata/corpus/gatling/lastrun/` is the result.

It repaid the cost immediately, and not in the way it had been justified. The first Maven run
produced a run directory and **no** `lastRun.txt`, which sent the search back into
`GatlingMojo.execute()` and found the write gated on `failOnError` being false, against a default of
true. Reading the writer had settled the file's *format* and could not settle when it exists at all.
The recording also disproved this feature's own claim that a run directory is named in local time: it
is UTC, on both build tools. Both corrections are now in R1 and R4, in the spec, in the API doc
comments and in `RECORDING.md` — and neither would have been caught by any amount of further
reading.

## Project Structure

### Documentation (this feature)

```text
specs/009-gatling-run-discovery/
├── plan.md                       # this file
├── spec.md                       # what and why
├── research.md                   # R1–R11: what the plugin bytecode and the corpus settled
├── data-model.md                 # RunLocation, FoundBy, RunNotFoundError, and the resolution pass
├── quickstart.md                 # how to validate, and how to make each part fail
├── contracts/
│   ├── README.md                 # index
│   ├── public-api.md             # contract 1 — four additions, nothing changed
│   └── lastrun-file.md           # contract 2 — what gatling-maven-plugin writes, and what we assume
├── checklists/requirements.md    # spec quality gate, 16/16
└── tasks.md                      # /speckit-tasks output — not created here
```

### Source Code (repository root)

```text
gatling/
├── discover.go                   # new — FindRun, the resolution pass, the bare-name rule,
│                                 #   the 64 KiB cap, newest-by-(mtime, name) ordering
├── discover_test.go              # new — table-driven over t.TempDir(): the newest run, the
│                                 #   pointer, the four pointers-to-nothing, a path that is
│                                 #   already a run, shared mtimes, symlinks, permissions
├── discover_corpus_test.go       # new — four tests over the recording, integration tag,
│                                 #   skipping with a reason when it is absent
├── discover_fuzz_test.go         # new — FuzzLastRun: no panic, nothing outside the root
├── discover_bench_test.go        # new — BenchmarkFindRun over a synthetic 1000-run root
├── discover_example_test.go      # new — FindRun composed with os.Open and simlog
├── errors.go                     # + RunNotFoundError, its doc comment and Error()
├── errors_test.go                # + its message, and that it is not what an unreadable
│                                 #   directory returns
└── doc.go                        # package doc widens: finding the run sits before the codecs
testdata/corpus/gatling/simulation/pom.xml   # new — the same sources under Maven, the only
                                  #   build tool that writes a lastRun.txt at all
testdata/corpus/gatling/lastrun/  # new recording — a Maven results root: three run
                                  #   directories, the lastRun.txt naming the middle one,
                                  #   all three runs' console output, RECORDING.md
CHANGELOG.md                      # Added: gatling.FindRun and the three types beside it
```

**Structure Decision**: no package is added and no file moves. Discovery goes in the `gatling` root
package because #11 names it, and because `AGENTS.md` "Structure" and this template's own source
block already list run discovery there; it sits beside `Version`, `Format` and `Policy`, which are
the other things that are cross-cutting rather than any one codec's. A `gatling/run` subpackage was
considered and rejected in research
[R5](research.md#r5--where-discovery-lives-and-what-it-is-called): it is tidier in the abstract and
buys nothing this feature needs, while adding a second package to freeze at v0.1.0. The new files
are named `discover_*` so that the feature's whole surface is one `ls` away, matching how
`truncation_*` was grouped in v0.0.8.

## Complexity Tracking

No constitution gate fails; nothing to justify.
