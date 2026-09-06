# Implementation Plan: Reading the Binary simulation.log Gatling Writes From 3.13.0

**Branch**: `005-gatling-binary-decoder` | **Date**: 2026-09-06 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/005-gatling-binary-decoder/spec.md`

**Milestone**: [v0.0.5 Binary logs, 3.13 and newer](https://github.com/galax-io/parsec/milestone/5) ·
**Issue**: [parsec#6](https://github.com/galax-io/parsec/issues/6)

## Summary

Since 3.13.0 Gatling writes `simulation.log` as an undocumented binary stream that only the same
Gatling version reads. It is the format every current user produces, and until it decodes this
library reads only versions nobody runs. This feature adds `gatling/binary`, yielding the same wire
records and the same canonical model the text codec already produces, and fills the empty row in the
dispatch table milestone v0.0.4 left for it.

The format is not being reverse-engineered. It was read out of Gatling's own writer at both bounds of
the supported range and then decoded against the 64-byte sample this project holds — all 64 bytes
parse, every field where the source says it should be. Research [R1](./research.md) records the
grammar; [R2](./research.md) records the two ways a careful decoder still gets it wrong, both of them
silent.

The long pole is not the decoder. It is recording three real Gatling runs with their own accounts of
their numbers, which can only happen at the moment each run finishes.

## Technical Context

**Language/Version**: Go 1.25 (`go.mod` is authoritative)

**Primary Dependencies**: standard library only. `gatling/binary` sits inside `gatling/`, which the
`deps` job holds to stdlib plus this module's own packages.

**Storage**: N/A — artefacts are read through `io.Reader`; nothing is persisted.

**Testing**: stdlib `testing`, table-driven; a golden corpus recorded for 3.13.1, 3.14.9 and 3.15.1
under `testdata/corpus/gatling/<version>/`, each with the run's own HTML report and console summary —
and, for 3.13.1, the `global_stats.json` and `stats.json` Gatling still wrote on that line and stops
writing at 3.14.0;
`go test -race -shuffle=on ./...`; integration behind `-tags=integration`; `FuzzDecode` over the
record reader.

**Engineering guidance**: this change triggers every required-reading row of the constitution's
Engineering Guidance — it adds exported identifiers, errors, tests, doc comments and exported types.
`golang-naming`, `golang-error-handling`, `golang-testing`, `golang-documentation`,
`golang-structs-interfaces`. Consult rows whose occasion has arisen: `golang-benchmark` (the
throughput goal), `golang-safety` (the allocation caps and the reused group slice),
`galaxio-gatling-pro` and `gatling-versions` (the recordings). Forbidden as always: testify and the
`samber/*` family — Principle IV keeps this package stdlib-only. Details in
[R11](./research.md).

**Target Platform**: any Go 1.25 target; consumed as a library by galaxio-cli, the comet sidecar and
the Galaxio backend.

**Project Type**: library (Go module `github.com/galax-io/parsec`)

**Performance Goals**: at least the text codec's throughput on the same simulation, and faster is the
expectation — there is no line splitting, no field parsing and no integer-from-string, and a repeated
name costs an index read rather than an allocation. Peak memory must be independent of record count.

**Measured** (Apple M-series, Go 1.25, 64 MiB synthetic log of the same shape for both codecs;
`go test -tags=integration -run '^$' -bench BenchmarkDecode -benchmem`):

| | ns/record | records/s | allocs/op | B/op |
|---|---|---|---|---|
| `gatling/binary` | **64.9** | **15.4M** | 19 | 66 KB |
| `gatling/text` | 96.2 | 10.4M | 29 | 1.0 MB |

Faster per record by 1.48×, and 1.29× through the model-facing reader. Bytes per second understates
it — the binary format is several times denser, so the same megabyte is many more records — which is
why the benchmark reports `records/op` alongside `MB/s`.

**Peak memory**: a 2.5 GB synthetic log of 174,308,736 records decodes with a peak heap of **3.9 MiB**
— the same figure as a log a tenth its size, and the same as one ten times smaller again. Bounded by
the fixed 64 KiB read buffer, the reused group path and the distinct strings the simulation declares,
exactly as intended, and against a 32 MiB budget.

**Constraints**: the stream must start at byte 0, because the string table cannot be rebuilt from the
middle — this is a property of the format and it shapes milestone v0.0.9. Every length read from the
file is capped before it reaches an allocator. Timestamps are 32-bit millisecond offsets, so the
format cannot represent a run past 24.8 days and a value that resolves before the run's start is
reported absent rather than wrapped. The version gate is the shared one and is applied once, before
any record is decoded.

**Scale/Scope**: Gatling 3.13.1 through 3.15.1 — seventeen releases, one grammar. Five record kinds,
one new package, one new `internal/` package, one field added to a shared error type, one row added
to the dispatch table. Three corpus recordings.

## Constitution Check

*GATE: passed before Phase 0; re-checked after Phase 1 — see [Post-design re-check](#post-design-re-check).*

Source: `.specify/memory/constitution.md` **v2.2.0**. This feature does not amend it.

- [x] **I. Canonical Model First** — no new `model` field: the binary format carries what the text
      one does, and R10 asserts that rather than assuming it. `gatling/binary` exports no result type
      of its own; it yields `gatling.Record` and `model.Item`. What the source cannot record is
      declared through `Capabilities`. Nothing here computes a count, a mean, a percentile, a range
      or a series — the scenario *index* the format stores is resolved to a name before it reaches
      either level, which is decoding, not arithmetic. Tool packages still import only `model/`,
      `gatling/` and the new `internal/`.
- [x] **II. Version-Gated, Streaming Decoders** — the gate is the shared `Policy`, applied once
      before any record. The range is 3.13.1–3.15.1 and **equals the corpus**: the floor is recorded
      rather than argued from the source diff, which is exactly what this principle forbids
      substituting. It is 3.13.1 rather than the format's own boundary at 3.13.0 because 3.13.0
      cannot be recorded — it fails to read back its own assertion records, so no run of it produces
      a report ([research.md R8](./research.md)). `io.Reader` entry point; peak memory bounded by distinct strings, not records;
      every file-supplied length capped before allocation; chunked and whole-file reads asserted
      identical; every error carries a byte offset; no panic, no `recover`.
- [x] **III. Golden-Corpus Testing** — three recordings from real runs, committed as produced, each
      with the two accounts of its numbers that Gatling still writes. The constitution's exemption
      applies and is used honestly: the tool version genuinely produces no `global_stats.json` or
      `stats.json` from 3.13.5, this was confirmed on a real run, and each entry records that the
      absence is Gatling's. Byte-for-byte comparison against the recorded record stream; counts
      against the run's own report with a documented tolerance of zero. Coverage floors inherited —
      `*/gatling/*` maps to 90%. Tests land with the change.
- [x] **IV. Minimal, Explicit Dependencies** — stdlib only; `go.mod` unchanged; no `replace`.
- [x] **V. Compatibility-Sensitive Public API** — every addition and the two shared changes are in
      [contracts/gatling-binary.md](./contracts/gatling-binary.md) with the doc comment each will
      carry. `SyntaxError` gains a field, which is additive; the record-to-model conversion moves to
      `internal/`, where no exported identifier moves. Both are ask-first and are not implemented
      until approved. `CHANGELOG.md` entries drafted in §6 of the contract.
- [x] **VI. Idiomatic, Simple Go** — the two codecs share the record types, the policy, the errors
      and now one conversion; they share no parsing, because a line scanner and a record reader have
      nothing in common below those types, and inventing a shared layer would be the speculative
      abstraction this principle forbids. Errors as values with offsets; enums already exist behind
      their unknown sentinels; `.golangci.yml` unchanged.
- [x] **Workflow** — milestone v0.0.5, issue #6, one green `feat(gatling): …(#6)` commit; spec
      artifacts committed first as `docs(speckit): add 005-gatling-binary-decoder spec/plan/tasks`.

**No gate fails, so Complexity Tracking is empty.**

## Project Structure

### Documentation (this feature)

```text
specs/005-gatling-binary-decoder/
├── plan.md              # This file
├── research.md          # Phase 0 — 11 decisions, 3 questions carried forward
├── data-model.md        # Phase 1 — the grammar, the table, the reader's state
├── quickstart.md        # Phase 1 — 12 runnable validation scenarios
├── contracts/
│   └── gatling-binary.md
├── checklists/
│   └── requirements.md  # from /speckit-specify — 16/16
└── tasks.md             # Phase 2 — /speckit-tasks, not created here
```

### Source Code (repository root)

```text
gatling/
├── errors.go                       #  + SyntaxError.Offset
├── binary/                         # NEW PACKAGE
│   ├── doc.go
│   ├── read.go                     #   the sized-primitive reader and its caps
│   ├── strings.go                  #   the string table, Latin-1 and UTF-16
│   ├── record.go                   #   the five record grammars
│   ├── reader.go                   #   Reader, NewReader, the version gate
│   ├── model.go                    #   RunReader over internal/wire
│   ├── capability.go
│   └── *_test.go  golden_test.go  fuzz_test.go  bench_test.go
├── text/
│   └── model.go                    #   convert deleted; calls internal/wire
└── simlog/
    └── simlog.go                   #   one table row gains its constructors

internal/wire/                      # NEW — the record→model conversion, shared
model/                              #   untouched
testdata/corpus/gatling/
├── 3.13.1/  3.14.9/  3.15.1/       # NEW — simulation.log, index.html, console.txt, RECORDING.md
└── simulation/                     #   the probe: non-Latin-1 name, repeated names, plain assertions above 3.13.x
testdata/samples/gatling/binary/    #   superseded by the corpus; removed
```

**Structure Decision**: a new package beside `gatling/text` rather than a mode inside it — they
share the record types and nothing below them. `internal/wire` exists because the record-to-model
mapping is now needed by two codecs and Principle I names `internal/` as the home for a helper shared
across packages; leaving a copy in each would let them disagree about what a record means while both
looked correct. The 64-byte sample is deleted once the corpus lands: it existed to prove format
detection, and a real recording supersedes it.

## Post-design re-check

Re-read Principles I–VI against the Phase 1 artefacts. All still pass; four points are worth
recording because the design moved after the first check.

- **II — the source evidence changed the work, not the rule, and the corpus then overruled the
  plan.** Knowing the format is stable across the whole binary era is worth weeks, and it is still
  not permission to widen the gate: the floor is whatever is recorded. Planning said that would be
  3.13.0; recording said 3.13.1, because 3.13.0 cannot read back the assertion records it writes and
  so produces no report. Its *writer* is sound and a source diff would have shown nothing wrong, so
  this is the exact shortcut this principle exists to refuse, caught refusing it.
- **III — the corpus can prove more than the text one could, and less.** More, because the HTML
  report carries the per-request breakdown and the console summary is a second independent account.
  Less, because no artefact carries virtual-user or error counts, so those are pinned against the
  recorded record stream and the spec does not claim they match Gatling's own.
- **VI — one conversion, not two.** The record-to-model mapping was the only real duplication the
  second codec would have created. Moving it to `internal/` is reuse of one function, not a shared
  abstraction over two formats; the parsing stays entirely separate, which is what keeps this honest.
- **V — the error type grew rather than forked.** A binary failure and a text failure are the same
  event to a caller, so `SyntaxError` carries both positions instead of a second type forcing every
  consumer to branch on format to ask one question.

What the design cannot settle: whether a real log ever carries a UTF-16 string, and whether
little-endian decodes it. No corpus this project can record answers the byte-order half — every
available machine is little-endian — so the probe's non-Latin-1 name proves the coder path works and
the byte order stays a documented assumption. That is the honest boundary, and it is in the spec.

## Complexity Tracking

No Constitution Check gate failed. This section is intentionally empty.
