# Implementation Plan: Pre-freeze hardening

**Branch**: `010-pre-freeze-hardening` | **Date**: 2026-09-10 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/010-pre-freeze-hardening/spec.md`

## Summary

Nine review findings before the v0.1.0 freeze. Seven change code here — two in the binary codec's
endings (#82, #87), two in `simlog`'s classification of a failing source (#83, #102), two in the
binary codec's gate order and memory budget (#76, #75), one in `run.Find`'s ordering (#88) — and two
are already moving in their own pull requests: the compat gate (#106, landed in PR #109) and the
toolchain pin (#104, PR #110). Nothing exported is added, renamed or re-typed. What changes is
whether the code keeps promises its documentation already makes, and each promise is stated in a
contract so it is frozen as it will be, not as it is.

Reading the code changed three things the spec assumed, all recorded in research
[R12](research.md#r12--what-reading-the-code-corrected-in-the-spec). The memory-test branch the spec
listed as a dependency is stale: `main` already samples the live heap after a collection, which is
the instrument #75's tests use. Parity for #76 reaches the text codec: its header parser validates
the run start before it reads the version, so moving the binary gate first alone would open the
reverse divergence, and both codecs take the same two-step shape instead. And #83's first chain cut
made `errors.Is(err, cause)` false for a cause that wraps `io.EOF`; review reversed that, and one
shared helper now hides only `io.EOF`, in all three packages.

Six fix commits, one per issue — #87's landed on `main` first, as PR #111 — in three stacked pull
requests, one per contract ([R9](research.md#r9--landing-order-commits-and-pull-requests)). Every commit carries its regression
test, its doc-comment changes and its `CHANGELOG.md` entry, and is green on its own.

## Technical Context

**Language/Version**: Go 1.25 is the floor (`go.mod`); the toolchain is `go1.26.8`, pinned by PR #110
(merged 2026-09-10), onto which this branch is rebased. Nothing here needs a feature past 1.25 — `slices.MaxFunc`
and `time.Time.Compare` (Go 1.21 and 1.20) are the newest used.

**Primary Dependencies**: standard library only — `bufio`, `errors`, `fmt`, `io`, `slices`,
`strings`, `time`; in tests also `testing/iotest` and `compress/gzip`. No module is added, and
`gatling/` stays stdlib-only as the `deps` job requires (Principle IV).

**Storage**: N/A — artefacts are read through `io.Reader`; `run.Find` reads directory entries.

**Testing**: stdlib `testing`, table-driven; one regression test per issue, each failing on `main`
(research R1–R7 name them and what they fail with); the golden corpus proves nothing else moved
(`go test -tags=integration ./...`, byte-for-byte); the three new memory tests carry the
`integration` tag and end in `PeakMemory`, because CI runs that class alone and un-instrumented
([R10](research.md#r10--performance-and-how-memory-is-measured)); `go test -race -shuffle=on ./...`.

**Engineering guidance**: required-reading rows triggered — `golang-error-handling` (three error
paths change: the binary fill loop, `identify`'s failure branch, the head sentinel; read, and three of
its rules are overruled here) and `golang-testing` (every change; its testify sections excluded).
Consulted: `golang-safety` and `golang-security` for byte ceilings on untrusted counts (R6),
`golang-benchmark` for the live-heap measurement and the throughput check (R10),
`golang-documentation` for the doc comments on existing exported identifiers that change
(`binary.Reader`, `binary.NewReader`, `run.FoundByNewest`). Not triggered: `golang-naming` and
`golang-structs-interfaces` — no exported identifier is added or changed. Every disagreement is
recorded in [R11](research.md#r11--skills-read-and-where-they-disagreed-with-the-constitution): the
skill's "always `%w`" and "always `errors.Is`" rules lose to Principle II and `simlog`'s documented
promise; `samber/oops` and `slog` lose to Principle IV and to the fact that a library logs nothing.

**Target Platform**: any Go 1.25 target; consumed by galaxio-cli, the comet sidecar and the Galaxio
backend. The 16-byte string-header constant assumes a 64-bit target; a 32-bit one retains less, so
the bound loosens only in the safe direction (R6).

**Project Type**: library (Go module `github.com/galax-io/parsec`).

**Performance Goals**: no throughput regression on `BenchmarkDecode` over the largest corpus file —
the decoder benchmark spec 005 shipped — measured before and after each binary commit (R10); the
changes on the hot path are an interface indirection per 64 KiB fill, a reordered `switch`, and one
addition per *introduced* string, nothing per record. Peak memory: the documented **32 MiB live
heap** for `binary.Reader`, now provable — worst-case retention after R6 is about 26 MiB by
arithmetic, against a figure bounded by nothing but entry counts today — and asserted by three new
`PeakMemory` tests beside the existing family. No figure changes for the text codec or `run.Find`.

**Measured 2026-09-10**, darwin/arm64, Go 1.26.4, five runs each of
`go test -tags=integration -run '^$' -bench 'BenchmarkDecode$' -benchmem`, before the first fix and
after the last (means; `benchstat` is not installed on the planning machine):

| Benchmark | ns/op before | after | MB/s before | after | B/op before | after | allocs/op before | after |
|---|---|---|---|---|---|---|---|---|
| `BenchmarkDecode/corpus` | 14 338 | 14 078 | 276.5 | 281.6 | 67 920 | 67 936 | 45 | 46 |
| `BenchmarkDecode/synthetic-64MiB` | 280 796 838 | 276 779 773 | 239.0 | 242.5 | 66 352 | 66 368 | 19 | 20 |

Throughput moved within noise, in the right direction. The one extra allocation and sixteen bytes per
operation are the `struct{ io.Reader }` wrapper boxed into an interface once per reader (#87) — the
cost the text codec has always paid, per constructor and never per record.

**Peak memory**, live heap after a collection (`-run 'PeakMemory$'`, un-instrumented, as CI runs
it): every table just under its ceiling at once, followed by a hundred thousand referring records —
**23.8 MiB**, accepted; 48 scenario names of `MaxStringLen` — refused at 3.4 MiB; 48 introduced
strings of `MaxStringLen` — refused at 14.4 MiB; the assertion payloads at their ceiling — 8.4 MiB;
the existing family unchanged at 0.5 MiB. The arithmetic bound of about 26 MiB holds with room.

**Coverage** (integration suite, `-skip 'PeakMemory$'`, floors enforced by
`scripts/check-coverage.sh`): `gatling` 98.1%, `gatling/binary` 98.9%, `gatling/run` 96.4%,
`gatling/simlog` 91.3%, `gatling/text` 97.6%, `internal/corpus` 83.8%, `internal/wire` 92.3%,
`model` 96.8%, overall 94.9% — every package above its floor.

**Gates**: `gofmt -l .` clean; `gofumpt -l .` names three files this feature does not touch
(`gatling/text/model_review_test.go`, `internal/corpus/console.go`, `internal/corpus/report_html.go`),
pre-existing on `main` and out of scope here; `golangci-lint` 2.12.2 over the module with the
integration tags: 0 issues; `go mod tidy` clean; `model/` and `gatling/` still standard-library only;
`go test -race -shuffle=on ./...` green; each of the seven fix commits green on its own
(`go build ./... && go test ./gatling/...` in a worktree at each). PR #110 (#104) merged while the
gates ran; the branch was rebased onto it, `go.mod` carries `toolchain go1.26.8` from `main`, and
every gate above was re-run under that toolchain. PR #111 (#87) merged in the same hour with the
same wrapper this branch had written, so this branch's #87 commit was dropped at rebase.

**Constraints**: chunked == whole-file, and every new source shape joins the chunk matrix; no
exported identifier added (spec, Out of Scope); every observable move recorded under Changed or
Fixed in `CHANGELOG.md` in the same commit (FR-018); every corpus entry unchanged (FR-019);
`.golangci.yml` unchanged; identity comparisons on `io.EOF` keep their `//nolint:errorlint` with the
reason stated.

**Scale/Scope**: Gatling text 3.11.5–3.12.0 and binary 3.13.1–3.15.1, unchanged. Seven issues, six
commits here and one that reached `main` first (PR #111), three pull requests; about twelve source and test files across `gatling/binary`,
`gatling/simlog`, `gatling/text` (one reorder) and `gatling/run`; two issues verified, not built.

## Constitution Check

*GATE: passed before Phase 0, re-checked after Phase 1 design. Source: `.specify/memory/constitution.md`
v2.2.0; v2.3.0 (PR #110) changes only the toolchain sentence and does not move any gate below.*

- [x] **I. Canonical Model First** — nothing enters or leaves `model/`; no tool package exports a new
      type; `Capabilities` is untouched. **Nothing is computed**: a byte ceiling is a bound on what
      the reader holds, not a statistic over records, and the run ordering is a comparison over
      filesystem metadata that v0.0.9 already made. Tool packages still import only `gatling`,
      `model` and `internal/`; the one cross-package test lives in `simlog`, which already imports both codecs.
- [x] **II. Version-Gated, Streaming Decoders** — the gate now runs *before* anything after the
      version is decoded, in both codecs (R5): this feature turns a current violation into
      compliance. Memory is bounded independently of artefact size in bytes, not entries (R6), and the
      read buffer is the codec's own (R2). Chunked and whole-file reads agree through one more source
      shape (R1). Every new refusal carries the offset of the entry that caused it; a filled read is
      never a truncation; no panic, no `recover`. The codec's range and its corpus coverage are
      unchanged.
- [x] **III. Golden-Corpus Testing** — no version is touched, so no recording is added; the existing
      corpus, byte-for-byte, and the canary are the proof that every accepted log is unchanged
      (FR-019). Every fixture is constructed and is a **fixture, not corpus** — a crafted run record,
      a generated 48 MiB stream, a gzip cut in a test, a temp-dir results root — and named as such.
      Each issue gets a regression test that fails on `main` (R1–R7 say what it fails with). Coverage
      floors (90% decoder packages, 80% module) are re-measured and stated in each pull request.
      No mocks: the sources are `iotest` readers and real `gzip`.
- [x] **IV. Minimal, Explicit Dependencies** — nothing added; `go.mod` untouched on this branch (the
      `toolchain` line arrives with PR #110 and is not a requirement); `gatling/` stays
      standard-library only; `unsafe` is kept out of the decoder by choosing a constant header size
      (R6).
- [x] **V. Compatibility-Sensitive Public API** — no exported identifier is added, renamed or
      re-typed. Observable behaviour moves only where it is itemised in
      [contracts/](contracts/README.md) and recorded under Changed (#76, #75, #88) or Fixed (#82,
      #83, #102, #87) in `CHANGELOG.md` in the same commit as each fix. Permitted before v0.1.0 with
      no deprecation window. **Ask first is met** by the maintainer's instruction commissioning exactly
      these issues (spec *Input*); the behaviour choices research made within them — the 12 MiB
      refusal (R6) and the unstamped-lowest ordering (R7) — are recorded under Changed in
      `CHANGELOG.md` and reach the maintainer for approval in the pull request's review. Doc comments on `binary.Reader`, `binary.NewReader` and `run.FoundByNewest`
      state the new behaviour.
- [x] **VI. Idiomatic, Simple Go** — no new abstraction beyond two helpers that each replace
      copies: one accounting helper shared by three call sites replaces one that existed for one, and
      `internal/source` replaces three copies of the `io.EOF` rule; a pairwise predicate becomes a key; a private
      sentinel mirrors the one the binary codec already has; `maxCacheEntries` goes because it
      becomes unreachable (dead code). Errors are values; identity comparisons on `io.EOF` keep
      their explained `//nolint`. `.golangci.yml` is unchanged. The required-reading skills were read
      and the disagreements recorded (R11).
- [x] **Workflow** — every issue is in milestone **v0.0.10 Pre-freeze hardening** (#19),
      moved from v0.1.0 A stable API (#11) to ship ahead of the freeze; these spec artifacts
      land as `docs(speckit): add 010-pre-freeze-hardening spec/plan/tasks` before any fix; each of
      the seven issues maps to one green commit (R9); #106 already has its commit (`410dc54`) and
      #104 gets its own from PR #110.

No row fails. Complexity Tracking is empty.

**Re-check after Phase 1**: unchanged. The design added no type, no dependency and no exported
identifier; the one thing Phase 1 widened — the text-side reorder for #76 — is inside the same
principle (II) and the same Changed entry, and the contracts say so.

## Project Structure

### Source Code (repository root)

```text
gatling/binary/
├── read.go                       # #82 readFull: filled means done (#87's wrapper arrived from main, PR #111)
├── record.go                     # #76 readRun → readVersion + readRunRest; #75 maxScenarioBytes, the shared
│                                 #   running total; maxScenarios' comment corrected
├── strings.go                    # #75 maxCacheBytes with maxCacheEntries' reasoning; maxCacheEntries removed
├── reader.go                     # #76 NewReader gates between the two halves; Reader doc: the budget's three tables
├── read_test.go                  # #82 readFull unit test; #76 bytes pulled before a refusal (internal: readBufferSize)
├── chunk_test.go                 # #82 iotest.DataErrReader joins the matrix
├── truncation_test.go            # #82 the 200,000-byte payload through three source shapes, and cut for real
├── agreement_test.go             # #76 TestCodecsGateBeforeTheRestOfTheRunRecord (#87's two-codec test is simlog/bufio_test.go, PR #111)
├── reader_test.go                # #76 1.0.0 + corrupt scenario count → *VersionError
├── limits_test.go                # #75 each ceiling at and past its boundary; 65,536 empty names still accepted
└── memory_test.go                # #75 three …PeakMemory tests over generated fixtures
gatling/text/
├── parse.go                      # #76 parseHeader → version first, then the gate's turn, then the start
└── reader.go                     # #76 finishPreamble applies the gate between the two halves
gatling/simlog/
├── simlog.go                     # #83 identify reports through internal/source; #102 readHead's errShortHead
└── errors_test.go                # #83 TestASourceFailureIsNeverTheEndOfTheLog (six constructors × four sources);
                                  #   the cause stays reachable; #102 the gzip, the spool and the Short cases
gatling/run/
├── find.go                       # #88 compareRuns + slices.MaxFunc replace later/newest's scan; FoundByNewest doc
├── find_test.go                  # #88 TestFindMixedStampedAndUnstampedNames
└── order_test.go                 # #88 permutation and shuffle properties (package run — the package's first internal test)
internal/source/
└── source.go                     # #83 Failed: a source failure hides io.EOF and keeps every other cause
CHANGELOG.md                      # [Unreleased]: Changed (#76, #75, #88), Fixed (#82, #83, #102, #87)
```

**Structure Decision**: no package is added and no file moves; each fix lands in the file that holds
the defect. Three placements are deliberate. The cross-package test FR-006 asks for lives in
`gatling/simlog`, the one package that already imports both codecs, so no import is added anywhere.
The two codec-parity tests (#87, #76) join `gatling/binary/agreement_test.go`, whose table is already
the record of what the codecs agree on and whose comment says a divergence cannot land without
changing it. And `gatling/run` gains its first internal test file, because the property #88 needs —
one answer under every permutation — is about `newest`, and `Find` cannot be made to list a directory
in six orders.

## Complexity Tracking

No constitution gate fails; nothing to justify.

## Order of work

Research [R9](research.md#r9--landing-order-commits-and-pull-requests) fixes it; in brief:

1. `docs(speckit): add 010-pre-freeze-hardening spec/plan/tasks` — its own pull request, merges on
   review (CI ignores `specs/`).
2. **PR A** contract 1, endings and source failures — #82, then #83, then #102. #82 first so the two
   fill loops are equal from the first commit that touches either.
3. **PR B** contract 2, the binary reader's gate, buffer and budget — #76 (with the text-side
   reorder), then #75; #87 landed on `main` as PR #111 while this branch was open. Stacked on A.
   #76 before #75 so the byte accounting is written into `readRunRest` once.
4. **PR C** contract 3, the run ordering — #88. Independent; may land at any point.
5. When PR #110 merges: rebase, confirm `go version` in the `verify` log, and close out FR-017.

Each pull request: milestone v0.0.10, `scripts/check-linkage.sh --pr N` green, coverage stated, the
`BenchmarkDecode` figure stated for B, issues closed on merge.

## What review should watch

The mistakes each fix is most likely to get wrong, so a reviewer looks there first:

- **#82** — the fullness check must come from the loop condition, not from reordering arms; a source
  that returns `(0, io.EOF)` inside a value must still be a cut, `(0, nil)` must still stall out, and a
  failure that arrives with the last bytes is reported, never cleared.
- **#83** — only `io.EOF` is hidden: every other cause stays reachable through `errors.Is` and
  `errors.As`, in a chain that holds `io.EOF` too (a caller may legitimately reach
  `fs.ErrPermission`, a `net.Error` or `context.Canceled`).
- **#102** — `readHead`'s own conversion and the source's `io.ErrUnexpectedEOF` must be different
  values; the stalled-reader guard must survive the rewrite.
- **#76** — the text side must gate *between* parsing the version and validating the start, not
  merely parse the version earlier; the field-count check stays first.
- **#75** — the fixtures must be generated readers, or the fixture's own bytes are measured as
  retention; the header constant is counted on every entry, including empty strings, or the count
  ceiling silently stops being reachable; `maxCacheEntries` must go, not stay unreachable.
- **#87** — the wrapper must hide the dynamic type; the test must use a caller buffer no larger than
  the codec's, or it measures the caller's `bufio` prefetching for itself.
- **#88** — the empty stamp must sort *below* every stamp, or a renamed directory outranks every run;
  the permutation test must cover the mixed root, because the all-stamped and all-unstamped roots
  passed before.
