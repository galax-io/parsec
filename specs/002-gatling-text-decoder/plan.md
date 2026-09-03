# Implementation Plan: Reading Gatling 3.11.5–3.12.x Text simulation.log Files

**Branch**: `002-gatling-text-decoder` | **Date**: 2026-09-02 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/002-gatling-text-decoder/spec.md`

## Summary

Land the module's first decoder: a streaming reader for the tab-separated `simulation.log` that Gatling 3.11.5 and 3.12.0 write, plus the record types the later binary codec will share, plus the module's first golden corpus.

The reader takes an `io.Reader`, walks the preamble to find the run header, gates on the version it names, then yields one record at a time in file order with memory that does not grow with the log. The first line it cannot decode ends the read with an error naming the line number — a partial read is refused rather than reported, because counts taken from a partial read cannot equal Gatling's own and would be indistinguishable from correct ones.

Correctness is proved two ways. Unit tests drive every field of every record kind, well-formed and malformed, from named fixtures. End-to-end tests read two complete recorded runs — the same sample simulation under each version — and compare the decoded stream field for field against a committed golden stream, and the request counts and mean request rate exactly against the report each run produced for itself.

Phase 0 established that the on-disk format is byte-identical between the two versions and that only Gatling's own reader changed, which is what makes a single codec across both honest rather than assumed. See [research.md](research.md).

## Technical Context

**Language/Version**: Go 1.25 (`go.mod` is authoritative)

**Primary Dependencies**: standard library only. No third-party module is introduced by this feature.

**Storage**: N/A — artefacts are read through `io.Reader`; nothing is persisted.

**Testing**: stdlib `testing`, table-driven; golden corpus under `testdata/corpus/gatling/<version>/`; named fixtures under `gatling/text/testdata/fixtures/`; `go test -race -shuffle=on ./...`; end-to-end suite behind `-tags=integration`

**Target Platform**: any Go 1.25 target; consumed as a library by galaxio-cli, the comet sidecar and the Galaxio backend

**Project Type**: library (Go module `github.com/galax-io/parsec`)

**Performance Goals**: ≥ 100 MB/s sustained on one core, and peak heap below 32 MiB for a log of any size, the per-line ceiling being 1 MiB (FR-016, SC-004).

**Recorded baseline (2026-09-03, Apple M-series, Go 1.26, `go test -tags=integration -bench BenchmarkReader`, benchstat n=6)** — a regression against these must be justified in the PR:

| Measure | Value |
|---|---|
| Steady-state throughput, 64 MiB synthetic log in memory | 496 MiB/s (≈ 520 MB/s), up from 406 MiB/s before research R12 |
| Allocations per read of that log | 27 — none per record; names are shared through a bounded table |
| Largest corpus file (4.9 KB) | dominated by the one-off 1 MiB line buffer `NewReader` allocates |
| Peak heap, 256 MiB log | 5.3 MiB |
| Peak heap, 2.5 GiB log (ten times larger) | 5.4 MiB |

The 1 MiB buffer is allocated once per `Reader`, up front, so that a line can never grow past the ceiling; for a library that reads one log per run this is the right trade, and it is stated in `NewReader`'s doc comment.

**Constraints**: streaming with bounded memory; chunked and whole-file reads must agree; version-gated reads; fail-fast on the first undecodable line with its line number; no panic on any input.

**Scale/Scope**: Gatling 3.11.5 and 3.12.0 — every released version in the 3.11.5–3.12.x range. Six record kinds. Logs up to multi-gigabyte soak runs. Text format only; the binary format from 3.13.0 belongs to milestone v0.0.5.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Source: `.specify/memory/constitution.md` v1.1.0.

**Initial evaluation (before Phase 0)** and **post-design re-evaluation (after Phase 1)** agree on every gate; the one gate whose reasoning changed during Phase 0 is II, noted below.

- [x] **I. Canonical Model First** — PASS, conditionally, with a Complexity Tracking row. This feature adds no `model/` types because `model/` is milestone v0.0.3. What it exports are the log's own wire records, not results: no count, timing or percentile is derived here, and no `Capabilities` claim is made. The condition is that these types are documented as wire records rather than a result model, and that v0.0.3 adds the conversion. Recorded as a row so the deferral is visible to a reviewer rather than argued away.
- [x] **II. Version-Gated, Streaming Decoders** — PASS. Gate refuses below range, decodes with a caller-reachable warning above it, and refuses a version string that is not a plain release (FR-009a). Range equals corpus coverage. `io.Reader` entry point, bounded memory, 1 MiB line ceiling. Chunked and whole-file agreement is a test, not an assumption. Errors carry line numbers. No `recover`. **Changed during Phase 0:** the surplus-field rule now differs inside and above the covered range precisely so that "an unknown newer version MUST decode and MUST surface a warning" keeps holding — see research.md R4.
- [x] **III. Golden-Corpus Testing** — PASS. The canary (issue #15, pulled into v0.0.2) adds a second layer above the corpus: every supported release is started for real on every change and held to its own fresh report. It does not replace the recording — a fresh run cannot be compared to the golden stream, only to its own report and to the other fresh runs — and the constitution's gate table needs a `canary` row, which is an amendment for its own PR. Two recordings, each from a real run, each committed with the two statistics files that run generated. Field-for-field comparison against a committed golden record stream; counts and mean rate against the run's own report with the tolerance and its reason documented at the assertion. Coverage floors 90% / 80%. Fixtures are named as fixtures, never as corpus.
- [x] **IV. Minimal, Explicit Dependencies** — PASS. `gatling/` and `gatling/text/` are standard library only. No module added.
- [x] **V. Compatibility-Sensitive Public API** — PASS. Every exported identifier is listed in [contracts/](contracts/) with a doc comment and the version range it accepts. Pre-v0.1.0, so these may still change; the `CHANGELOG.md` entry lands in the implementation PR under Added.
- [x] **VI. Idiomatic, Simple Go** — PASS. One reader, one flat record type, no interface introduced before a second codec needs it. `.golangci.yml` unchanged.
- [x] **Workflow** — PASS. Milestone v0.0.2, issue #3. Spec artifacts commit as `docs(speckit): add 002-gatling-text-decoder spec/plan/tasks` before any `feat`. One tracked issue, one green commit.

**One workflow action falls out of this plan rather than into it:** issue #3's acceptance list still says a malformed line "is reported with its line number and the read continues". The clarification session reversed that. The issue MUST be corrected before the implementation PR merges, or the tracked acceptance criteria and the shipped behaviour disagree on the record.

## Project Structure

### Documentation (this feature)

```text
specs/002-gatling-text-decoder/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output — exported API surface
│   └── gatling-text.md
├── checklists/
│   └── requirements.md  # from /speckit-specify
└── tasks.md             # /speckit-tasks output — NOT created here
```

### Source Code (repository root)

```text
gatling/                                  # shared across text and binary codecs
├── record.go                             # Record, Kind, Header, Status — the log's wire records
├── version.go                            # Version, parsing, the gate and its verdict
├── errors.go                             # SyntaxError (line number), VersionError, warning type
└── *_test.go
gatling/text/                             # this feature's codec
├── reader.go                             # Reader: preamble, gate, Next
├── scan.go                               # line splitting, 1 MiB ceiling, CR handling
├── parse.go                              # one parser per record kind
├── intern.go                             # bounded table of the names a log repeats
├── testdata/fixtures/                    # hand-written, named as fixtures
├── helpers_test.go                       # //go:build integration || canary: report and cross-run checks
├── canary_test.go                        # //go:build canary: fresh runs from PARSEC_CANARY_RUNS
└── *_test.go                             # unit + golden + chunked-agreement + benchmark
.github/workflows/gatling-canary.yml      # a real Gatling per version; called from ci.yml, by hand, weekly
testdata/corpus/gatling/3.11.5/           # recorded run
├── simulation.log
├── global_stats.json                     # the run's own report: totals and mean rate
├── stats.json                            # the run's own report: per request and group
└── records.golden                        # the decoded stream, canonical form
testdata/corpus/gatling/3.12.0/           # same simulation, same four files
```

**Structure Decision**: two packages. `gatling/` holds the record types, the version type and the gate — everything the binary codec in milestone v0.0.5 will share, which is why they do not live inside `gatling/text/`. `gatling/text/` holds the codec itself. Neither imports the other's internals and both are standard library only. No `internal/` package is added: nothing here is shared with a package outside `gatling/`, and adding one before there is a second consumer would be the speculative abstraction Principle VI forbids.

The corpus lives at `testdata/corpus/gatling/<version>/` — the constitution's `<tool>/<version>/` layout, not the flat path issue #3 sketches. Malformed inputs live under `gatling/text/testdata/fixtures/` with `fixture` in each name so no later reader can mistake an edited file for a recording.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Wire record types exported from a tool package with no `model/` conversion (Principle I) | `model/` is milestone v0.0.3. This milestone must ship a working decoder, and a decoder that converts to a model which does not exist yet cannot be written. The exported records are the log's events, not results — nothing derives a count, timing or percentile from them here, and no `Capabilities` claim is made. | Landing `model/` in this feature was rejected: it puts one PR across two milestones, breaks the one-issue-one-commit rule, and would fix the canonical model's shape before any second tool has been decoded to inform it. The exposure is bounded instead — pre-v0.1.0 so the types may still change, doc comments state they are wire records rather than a result model, and v0.0.3 owns the conversion. |
| Error record decoded by a different rule than the other five kinds (FR-008b) | Confirmed in Phase 0: Gatling assembles an error record's message from raw crash text and writes it without escaping, so a tab inside it is normal output. Under the exact-field-count rule such a record would fail an otherwise healthy read. Taking the message as everything between the kind and the trailing timestamp recovers it unambiguously, because the timestamp is the final field. | Treating it as damage was rejected — it would refuse logs Gatling itself wrote, for the sake of a rule the format does not actually keep. Sanitising on read was rejected because the spec forbids repairing content. |
