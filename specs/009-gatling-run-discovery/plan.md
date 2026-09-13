# Implementation Plan: Finding the run

**Branch**: `009-gatling-run-discovery` | **Date**: 2026-09-08 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/009-gatling-run-discovery/spec.md`

## Summary

Three consumers each work out where a Gatling run's `simulation.log` lives, and they are about to
work it out differently. This feature answers the question once: `run.Find` takes a path that
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

**Primary Dependencies**: standard library only — `errors`, `fmt`, `io`, `io/fs`, `os`, `path/filepath`,
`strconv`, `strings`, `time`. No module is added, and `gatling/` stays stdlib-only as the `deps` job
requires (Principle IV).

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
stated instead, and held by `BenchmarkFind` over a synthetic root of 1000 runs: **one directory
read, at most one `stat` per entry, one bounded read of `lastRun.txt` (64 KiB cap)**, with
allocations proportional to the entry count and independent of anything inside a run. No existing
benchmark is touched; no decoder code path changes, so none can regress.

**Measured 2026-09-08, re-measured 2026-09-09 after review**, darwin/arm64, `-benchtime=200ms`:

| Benchmark | allocs/op before | after | B/op before | after |
|---|---|---|---|---|
| `BenchmarkFind/runs=10` | 102 | 71 | 14 232 | 9 384 |
| `BenchmarkFind/runs=100` | 825 | 524 | 118 362 | 71 752 |
| `BenchmarkFind/runs=1000` | 8 028 | **5 027** | 1 166 752 | **686 554** |
| `BenchmarkFindLastRun` (1000 runs, pointer read) | 8 041 | 5 043 | 1 152 357 | 688 495 |
| `BenchmarkFindNamedPath` | 4 | 3 | 640 | 496 |

**−37% allocations and −41% bytes** at every size, because the ordering now uses the `FileInfo`
`holdsLog` was already fetching instead of spending a second `stat` on the run directory — the fix
for the report-regeneration defect and the cost reduction are the same change. The bound holds and
tightens: one directory read and **one** `stat` per entry, where the first implementation took two.

Wall-clock is not quoted as a delta. The re-measurement ran on a machine busy with the review itself
and its ns/op is not comparable with the original figure; allocations and bytes are the stable
numbers and are what the bound is stated in.

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

### Source Code (repository root)

```text
gatling/run/                      # new package — imports nothing else in this module
├── doc.go                        # what the package is for, and why it is not in gatling/
├── find.go                       # Find, the resolution pass, the bare-name rule, the
│                                 #   bounded pointer read, newest-by-(log mtime, run-id
│                                 #   stamp, name)
├── errors.go                     # NotFoundError
├── find_test.go                  # table-driven over t.TempDir(): the newest run, the
│                                 #   pointer, the pointers-to-nothing, a named run, a
│                                 #   shared mtime, two simulation ids, symlinks,
│                                 #   permissions, canonical paths
├── errors_test.go                # the error's message and errors.As
├── corpus_test.go                # the lastRun.txt recording, integration tag
├── example_test.go               # Find, and Find over a results root
├── fuzz_test.go                  # FuzzLastRun
└── bench_test.go                 # BenchmarkFind over a synthetic 1000-run root
gatling/
├── doc.go                        # points at gatling/run; the codec vocabulary is unchanged
└── errors.go                     # unchanged — the discovery error moved out with the code
testdata/corpus/gatling/simulation/pom.xml   # the same sources under Maven, the only
                                  #   build tool that writes a lastRun.txt at all
testdata/corpus/gatling/lastrun/  # recording — a Maven results root: three run
                                  #   directories, the lastRun.txt naming the middle one,
                                  #   all three runs' console output, RECORDING.md
CHANGELOG.md                      # Added: gatling/run and the four types in it
```

**Structure Decision**: discovery lives in **`gatling/run`**, symmetric with `gatling/simlog`, and
imports nothing else in this module. The first draft put it in the root `gatling` package; review
overturned that, and research [R5](research.md#r5--where-discovery-lives-and-what-it-is-called)
carries the argument and the reversal. In short: the root package is what every codec imports, and
discovery is imported by none of them — it runs strictly before them and is called only by end
consumers. Putting it there also made `gatling/` touch the filesystem for the first time and forced
its doc comment to contradict its own opening sentence.

Moving it was the one structural decision that got materially more expensive after v0.1.0, where a
package move cannot be deprecated cleanly. Done now it is a relocation plus a rename, and the rename
is a gain: with the package name carrying `run`, the identifiers are `Find`, `Location` and
`NotFoundError` rather than `FindRun`, `RunLocation` and `RunNotFoundError` — the last of which
existed only to dodge a collision with `model.Run` that does not arise here.

## Complexity Tracking

No constitution gate fails; nothing to justify.

## What review changed, after implementation

A max-effort review (ten finder angles, plus a Codex pass and an adversarial pass) ran against the
merged branch and found fifteen issues. The three that changed the design rather than the code:

**The empty-path default is withdrawn.** `Find("")` meant `target/gatling`, resolved against the
process working directory. `""` is also the zero value of every unset flag, absent configuration
field and omitted request member — so a server whose path silently went missing would have been
handed a confident report about whatever run happened to sit in its own working directory. The
layout is now published as `DefaultResultsRoot` for a caller to pass, an empty path returns
`ErrNoPath` before any filesystem access, and `RunNotFoundError.Default` is gone with it: nothing is
substituted, so there is nothing to disclose. FR-010 and FR-011 are rewritten to match.

**The ordering reads the log's modification time, not the run directory's.** A directory's time moves
whenever anything is written into it, so regenerating a report into an old run made that run the
newest — the exact failure US2 was written to prevent, on the path R2 shows is the ordinary one. The
log is written by the run and by nothing else, and `holdsLog` was already fetching its `FileInfo` and
discarding it, so the fix removes a syscall per candidate rather than adding one.

**The tie-break compares the run id's own UTC stamp before the directory name.** Whole-name order is
alphabetical by simulation id first, so a root holding two simulations — which is what Maven's
`runMultipleSimulations` produces — returned the run that started months earlier while reporting
`FoundByNewest`. R4 established the name format and that it is UTC; that is what the comparison now
uses, falling back to the whole name for a directory an archive renamed.

Beside those: a failed look inside a candidate is now an error rather than a skip (FR-012 at every
depth); an unreadable `lastRun.txt` is reported instead of degrading silently to a guess; the pointer
read is bounded by the open descriptor rather than by a prior stat of the path; paths are cleaned
once so one run yields one comparable `Location`; a path that exists but is not a directory reaches
`*NotFoundError` rather than a raw `ENOTDIR`; a directory named `simulation.log` resolves; the
pointer lookup is a set rather than a nested scan; the permission tests skip on Windows by platform
rather than by uid; and the corpus tests fail rather than skip when the committed recording is absent.

Mutation testing backs the new assertions: reverting each of the four load-bearing fixes in turn
fails a test that is specific to it. The earlier suite could not do this — swapping the ordering key
for the log's time had left all 214 tests green, because every fixture built its runs in ascending
name order so directory time, log time and name order all agreed.

## The package move

Review's adversarial pass called the root-package placement "defensible, but R5's reasoning is weak"
and recommended `gatling/run`. It was taken, in a separate commit from the defect fixes so that each
is reviewable on its own.

Nothing about the behaviour changed: the same resolution pass, the same tests, the same recording.
What changed is the import path, four exported names, and the fact that `gatling/` no longer imports
`os`. `gatling/run` deliberately imports nothing from the rest of the module — it carries its own
`unknownName` constant rather than borrowing the one in `gatling/record.go` — so a forwarding shim
from `gatling` remains possible without a cycle if a consumer ever needs one.

The coverage gate needed no change: its floor glob is `*/gatling/*`, so the new package inherits the
90% decoder floor and holds at 95.6%.
