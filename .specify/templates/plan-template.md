# Implementation Plan: [FEATURE]

**Branch**: `[###-feature-name]` | **Date**: [DATE] | **Spec**: [link]

**Input**: Feature specification from `/specs/[###-feature-name]/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

[Extract from feature spec: primary requirement + technical approach from research]

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for this feature. The defaults below are the parsec module's standing facts
  (constitution: Quality Gates & Tooling). Keep them unless the feature changes them,
  and fill every bracketed item or mark it NEEDS CLARIFICATION.
-->

**Language/Version**: Go 1.25 (`go.mod` is authoritative)

**Primary Dependencies**: standard library only in `model/` and `gatling/`; `github.com/caio/go-tdigest` in `stats/`; [any other module: name it here, justify it in research.md, ask first (Principle IV)]

**Storage**: N/A (artefacts are read through `io.Reader`; nothing is persisted) [or feature-specific]

**Testing**: stdlib `testing`, table-driven; golden corpus under `testdata/corpus/<tool>/<version>/`; `go test -race -shuffle=on ./...`; integration suite behind `-tags=integration`

**Target Platform**: any Go 1.25 target; consumed as a library by galaxio-cli, the comet sidecar and the Galaxio backend

**Project Type**: library (Go module `github.com/galax-io/parsec`)

**Performance Goals**: [decoder throughput and peak memory on the largest corpus file, e.g. "≥ 200 MB/s and < 64 MiB peak on a 2 GB simulation.log" or NEEDS CLARIFICATION]

**Constraints**: streaming with bounded memory; chunked == whole-file; version-gated reads; [feature-specific constraints or NEEDS CLARIFICATION]

**Scale/Scope**: [tool versions covered, artefact sizes, record kinds, e.g. "Gatling 3.11.5–3.12.x, logs up to 5 GB, 5 record kinds" or NEEDS CLARIFICATION]

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Source: `.specify/memory/constitution.md` v1.0.0. Mark each gate PASS, or FAIL with a row in
Complexity Tracking. N/A is allowed only with a one-line reason.

- [ ] **I. Canonical Model First** — new or changed result data lives in `model/` types; no
      tool package exports a consumer-facing result type; `stats/` and tool packages import
      only `model/` and `internal/`; everything the source cannot supply is declared through
      `Capabilities`, never defaulted.
- [ ] **II. Version-Gated, Streaming Decoders** — every artefact read passes the version gate
      (refuse below range, decode with a warning above); the codec's range equals its corpus
      coverage; `io.Reader` entry point with bounded memory and capped allocations; chunked
      and whole-file reads agree; errors carry offsets; no panic/recover control flow.
- [ ] **III. Golden-Corpus Testing** — corpus recordings from real runs listed for every
      version touched, each captured with that run's own tool report (or the entry recording
      that the tool version produced none); byte-for-byte and report-tolerance assertions planned with their
      tolerances documented; coverage stays ≥ 90% (decoder packages) / 80% (overall); each
      bug fixed gets a regression test; no mocks where a real artefact exists.
- [ ] **IV. Minimal, Explicit Dependencies** — `model/` and `gatling/` stay stdlib-only; any
      new module is named above, justified in research.md and approved before implementation.
- [ ] **V. Compatibility-Sensitive Public API** — exported API additions and changes listed in
      contracts/; breaking changes flagged in the spec and approved; deprecation path defined;
      `CHANGELOG.md` entry planned; doc comment for every new exported identifier.
- [ ] **VI. Idiomatic, Simple Go** — no abstraction without a current need; `.golangci.yml`
      unchanged or the change justified; errors as values; no control flow by panic.
- [ ] **Workflow** — feature assigned to a milestone; spec artifacts committed as
      `docs(speckit): …` before implementation; each tracked issue maps to one green commit.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)
<!--
  ACTION REQUIRED: parsec is a single Go module with packages at the repository root
  (constitution Principle I; AGENTS.md "Structure"). Keep only the packages this feature
  touches, and expand them with the real files the feature adds or changes.
-->

```text
model/                              # canonical result types and Capabilities
gatling/                            # text + binary simulation.log codecs, version gate, run discovery
jmeter/  k6/  locust/  phout/       # one adapter package per tool, added by milestone
stats/                              # counts, timings, percentiles (go-tdigest), per-interval series
internal/                           # helpers shared across packages; not public API
testdata/corpus/<tool>/<version>/   # golden artefacts recorded from real runs
<pkg>/*_test.go                     # table-driven tests beside the code they cover
```

**Structure Decision**: [Name the packages this feature adds or changes, the corpus
directories it records, and any new `internal/` package with the reason it is not public]

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., new third-party module in stats/] | [current need] | [why the standard library is insufficient] |
| [e.g., exported tool-specific result type] | [specific problem] | [why a model/ type plus Capabilities cannot express it] |
