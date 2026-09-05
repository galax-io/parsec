# Implementation Plan: A Canonical Model for Load-Test Results, and Requirements Stated Once

**Branch**: `003-canonical-model` | **Date**: 2026-09-04 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/003-canonical-model/spec.md`

## Summary

Milestone v0.0.3 turns the Gatling decoder's output into results anything can consume, and moves the
corpus probe's expectations out of Scala into a document that names no tool.

Two packages change. A new `model/` holds the canonical types — `Sample`, `GroupSample`,
`UserEvent`, `RunError`, `Run` and `Capabilities` — and is what the three downstream builds depend
on. `gatling/text/` gains a second entry point that reads the same log and yields those types
instead of wire records, streaming, so the 32 MiB ceiling the decoder already meets is unchanged.
The wire records stay exported and stay documented as the log's own events (clarification; FR-014a).
`Aggregate` is not built: it is deferred whole to v0.5.0 where Locust gives it something to be
designed against, which overrides issue #4 and means that issue must be corrected before the
implementation PR merges.

On the probe: its expectations become an OpenNFR `RequirementSet`, rendered into Gatling assertions
by `OpenNfrAssertions.fromYaml` from `gatling-picatinny`. This repository writes no renderer. Phase 0
settled the one thing that could have stopped it — the renderer runs under Gatling 3.11.5 and 3.12.0,
verified by running it, producing the same nine assertions and the same verdicts as today's
hand-written block ([research.md](research.md) R1). The story is therefore unconditional.

## Technical Context

**Language/Version**: Go 1.25 (`go.mod` is authoritative)

**Primary Dependencies**: standard library only. `model/` and `gatling/` are stdlib-only by Principle
IV and the `deps` CI job enforces it; nothing in this feature needs a module. The probe's Scala
project gains `org.galaxio %% gatling-picatinny % 1.27.0` in `Test` scope — that is a separate sbt
build under `testdata/`, so `go.mod` is untouched and `govulncheck` sees nothing new.

**Storage**: N/A — logs are read through `io.Reader`; nothing is persisted.

**Testing**: stdlib `testing`, table-driven; the two existing corpus entries under
`testdata/corpus/gatling/{3.11.5,3.12.0}/`; `go test -race -shuffle=on ./...`; integration suite
behind `-tags=integration`; canary behind `-tags=canary`.

**Target Platform**: any Go 1.25 target; consumed as a library by galaxio-cli, the comet sidecar and
the Galaxio backend.

**Project Type**: library (Go module `github.com/galax-io/parsec`)

**Performance Goals**: the conversion adds no allocation per record beyond the sample it yields, and
peak memory for a 1 GB log stays under the 32 MiB the decoder already meets (SC-004).

**Measured, and these are the regression baselines.** On a 64 MiB synthetic log of 1,315,860 records:
the decoder alone runs at ~504 MB/s with 1,062,728 B/op and 28 allocs/op; the conversion runs at
~324–353 MB/s with 1,062,928 B/op and 31 allocs/op. The allocation figures are the ones that matter
and they are flat — three allocations and ~200 bytes more across the whole run, so **nothing is
allocated per item**. Peak heap over a 256 MiB log is 5.4 MiB, unchanged at ten times the size.

The throughput cost — about a third — is the `model.Item` value copied out of `Next` on every call,
and it is accepted rather than optimised away: a 1 GB log still converts in about three seconds,
which no stated criterion is close to, and the alternatives (an interface per kind, or a
fill-in-place `Next(*Item)`) trade a clearer API for a number nothing needs. A regression against
these figures is justified in the PR or it is a bug.

**Constraints**: streaming with bounded memory; a run value holds nothing that grows with the run's
length (FR-011a); counts through the model equal counts through the wire records equal the run's own
report, exactly (FR-018); no third-party dependency reaches a consumer's build (FR-015).

**Scale/Scope**: Gatling 3.11.5 and 3.12.0 only, through the decoder shipped in v0.0.2 — this feature
reads no artefact itself and does not widen the supported range. Six wire record kinds in, four
model item kinds out, plus the run.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Source: `.specify/memory/constitution.md` v1.1.0.

**Initial evaluation (before Phase 0)** and **post-design re-evaluation (after Phase 1)** agree on
every gate. The one gate whose reasoning changed during Phase 0 is III, noted below.

- [x] **I. Canonical Model First** — PASS, and this feature is what the principle was waiting for.
      `model/` becomes the source of truth; `gatling/text/` converts into it, which is where the
      principle puts the conversion. The Complexity Tracking row spec 002 opened is closed here, not
      by hiding the wire records but by adding the conversion and documenting which types consumers
      build on — the clarified reading of the principle's "result type" (FR-014a). Everything the
      source cannot supply is declared through `Capabilities` and never defaulted; `Capabilities`
      stores what is **provided**, so a field added later is absent everywhere until an adapter
      claims it (research R4). One row remains in Complexity Tracking, for `Aggregate`.
- [x] **II. Version-Gated, Streaming Decoders** — PASS. The gate is not re-implemented: it belongs to
      the decoder, a refused log never reaches the conversion, and a warned log carries its warning
      into the run (FR-016a). The conversion is a cursor over the existing `io.Reader` path, so
      bounded memory and chunked/whole-file agreement are inherited rather than re-argued — and both
      are re-asserted through the model so a conversion that buffered would fail. Errors keep the
      line numbers the decoder attaches. No `recover`.
- [x] **III. Golden-Corpus Testing** — PASS. No new recording is required and none is possible: the
      two entries were captured with the reports their own Gatling generated, and that capture rule
      has already been met for them. Counts through the model are held to those same reports, exactly
      (FR-018, SC-002), beside the existing wire-record checks rather than instead of them, so a
      conversion bug and a decoder bug stay distinguishable (research R10). Coverage floors 90% for
      the conversion / 80% overall. Malformed inputs stay named as fixtures.
      **Changed during Phase 0:** the probe's assertions move into a document, so the *canary* — a
      fresh run on every change — is what enforces them from here on. The two committed recordings
      keep the assertions their runs evaluated; they are evidence, not something to re-cut.
- [x] **IV. Minimal, Explicit Dependencies** — PASS. `model/` and `gatling/` stay stdlib-only and the
      `deps` job keeps proving it. `go.mod` gains nothing. The one dependency added anywhere is
      `gatling-picatinny` in the probe's own sbt build under `testdata/`, which no consumer of this
      module ever resolves; it is named here and justified in research R1 and R8.
- [x] **V. Compatibility-Sensitive Public API** — PASS. Every new exported identifier is listed in
      [contracts/model.md](contracts/model.md) and [contracts/gatling-text.md](contracts/gatling-text.md)
      with its doc comment. Nothing existing changes signature or behaviour: the wire records, the
      gate and `text.NewReader` are untouched, so v0.0.2's surface keeps working. Pre-v0.1.0, so the
      new types may still move; the `CHANGELOG.md` entry lands under Added in the implementation PR.
- [x] **VI. Idiomatic, Simple Go** — PASS. One cursor, one flat item type with a `Kind` — the
      convention `gatling.Record` already set, chosen over an interface per kind precisely because
      the principle asks for the existing convention and because an interface allocates per item.
      `Opt[T]` is one small generic used by every optional field rather than a pointer per field.
      No abstraction is introduced for a second tool that does not exist yet. `.golangci.yml`
      unchanged.
- [x] **Workflow** — PASS. Milestone v0.0.3, issues #4 and #30. Spec artifacts commit as
      `docs(speckit): add 003-canonical-model spec/plan/tasks` before any `feat`. Two tracked issues,
      so two green commits, one per issue.

**Two workflow actions fall out of this plan rather than into it.** Both are corrections to tracked
issues that this feature's decisions override, and both must land before the implementation PR
merges or the tracked requirements and the shipped code disagree on the record:

1. **Issue #4** requires the model to accept pre-aggregated sources and names `Aggregate` among this
   milestone's types. Both are deferred to v0.5.0 (clarification; Complexity Tracking below).
2. **Issue #30** requires the translation to apply the recorded-name rule "so an author never has to
   know it". The renderer does not, and this repository is not writing one — so the document carries
   the recorded spelling and says why (FR-029). The substitution belongs upstream in
   `gatling-picatinny`, and an issue there is the right home for it.

## Project Structure

### Documentation (this feature)

```text
specs/003-canonical-model/
├── plan.md                  # This file
├── research.md              # Phase 0 output
├── data-model.md            # Phase 1 output
├── quickstart.md            # Phase 1 output
├── contracts/               # Phase 1 output — exported API surface
│   ├── model.md             # the canonical types
│   ├── gatling-text.md      # the conversion entry point
│   └── nfr.yaml             # the probe's requirements, as they will be committed
├── checklists/
│   └── requirements.md      # from /speckit-specify
└── tasks.md                 # /speckit-tasks output — NOT created here
```

### Source Code (repository root)

```text
model/                                    # NEW — the canonical result types
├── doc.go                                # what the package is, and what it is not
├── sample.go                             # Sample, GroupSample, UserEvent, RunError, Outcome, Failure
├── run.go                                # Run, Item, ItemKind, Warning
├── capability.go                         # Field, Capabilities, Provides
├── opt.go                                # Opt[T]
└── *_test.go

gatling/text/                             # CHANGED — a second entry point over the same reader
├── model.go                              # RunReader: Run() + Next() (model.Item, error)
├── capability.go                          # what a Gatling text run provides, and what it never does
├── model_test.go                         # unit: every record kind, both outcomes, the sentinel
├── model_golden_test.go                  # integration: counts through the model vs each run's report
└── (reader.go, parse.go, scan.go, intern.go, record.go — untouched)

testdata/corpus/gatling/simulation/       # CHANGED — the probe states its requirements once
├── src/test/resources/nfr.yaml           # NEW — the OpenNFR RequirementSet
├── src/test/scala/.../CorpusSimulation.scala  # assertions block replaced by one call
├── project/Dependencies.scala            # gains gatling-picatinny, pinned
└── README.md                             # records how an expectation is changed now

.github/workflows/                        # CHANGED — the document is validated on every change
testdata/corpus/gatling/{3.11.5,3.12.0}/  # UNCHANGED — recordings are evidence, not re-cut
```

**Structure Decision**: two Go packages. `model/` is new and holds nothing tool-specific; it is the
package the three downstream builds import, and it is stdlib-only. The conversion lives in
`gatling/text/` because Principle I puts it in the tool package, and because it reads the wire
records that package already produces — a third package between them would earn nothing and would be
the speculative structure Principle VI forbids. No `internal/` package is added: nothing here is
shared outside these two.

`Run` and `Item` live in `model/` rather than in the tool package because they are what a consumer
depends on. The cursor that produces them lives in `gatling/text/` because it is the thing that knows
about Gatling.

The probe's document is committed at `src/test/resources/nfr.yaml` inside the simulation project,
which is where the renderer's path argument resolves from, and mirrored in `contracts/` so a reader
of this spec can see it without opening the corpus tree.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| `Aggregate` and the summary-only distinction are absent, though issue #4 requires them (Principle I's "a source that publishes summaries" reading, and the issue's own Requirements) | Principle VI forbids building a shape for which no source exists. The first summary-only source is Locust, milestone v0.5.0. Designing its type here means designing against nothing, and v0.5.0's own description records that the model "has never been exercised against" one — an argument for designing it when there is an artefact. FR-010 keeps the debt honest: nothing in the model may assume every source records individual operations, and no existing type may have to change meaning to admit one. | Shipping `Aggregate` now was rejected on Principle VI, having been recommended and then reversed in clarification. The cost is real and accepted rather than argued away: the model becomes a stability promise at v0.1.0, four milestones before Locust, so admitting a summary-only source afterwards is a breaking change for three downstream builds and takes a new MINOR version. Issue #4 is corrected instead. |
| Wire record types stay exported from a tool package (Principle I, "a tool package MUST NOT export a result type of its own for consumers to depend on") | They are the log's own events, not results: nothing derives a count, a timing or a percentile from them, and the binary codec in v0.0.5 shares the same types. A caller debugging an undocumented format has no other route to what a log actually contained. FR-014a fixes the reading: they are documented as wire records, the canonical types are documented as what consumers build on. | Making them unexported was rejected — it deletes the only honest view of an undocumented format for the sake of a rule aimed at result types, and v0.0.5 would have to re-export them. Deprecating them now for removal at v0.1.0 was rejected for the same reason: it promises to delete what the next codec needs. This row is a recorded reading of Principle I, not a deferral; spec 002's row is closed by it. |
