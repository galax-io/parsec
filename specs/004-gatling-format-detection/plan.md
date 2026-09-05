# Implementation Plan: Telling Which Gatling Wrote a simulation.log

**Branch**: `004-gatling-format-detection` | **Date**: 2026-09-05 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/004-gatling-format-detection/spec.md`

**Milestone**: [v0.0.4 Which Gatling wrote this log](https://github.com/galax-io/parsec/milestone/4) ·
**Issue**: [parsec#5](https://github.com/galax-io/parsec/issues/5)

## Summary

A `simulation.log` carries no magic number and no format version, so today a caller must know in
advance which Gatling wrote a file. This feature identifies the format from the file's leading bytes
— never from its name — hands the stream to the codec that reads it, and applies one version policy
that every codec shares: refuse below the recorded range, decode inside it, decode with exactly one
warning above it, and refuse above it when the caller asks for strictness. The supported range
becomes readable programmatically, per format, including the honest answer that the binary format
has no codec yet.

Three packages are touched. `gatling/` gains `Detect`, the `Format` type, the shared read options,
the `Policy` type and three error types. `gatling/text/` moves onto the shared policy and its
constructors gain variadic options; its observable behaviour does not change. A new
`gatling/simlog/` does the stream plumbing and the dispatch, which cannot live in `gatling` because
`gatling/text` imports it.

Two findings from the spec phase shape the work. Issue #5's proposed detection rule — first byte
`'R'` — is falsified by this repository's own corpus, whose logs open with `ASSERTION\t`; the
corrected rule is `RUN\t` or `ASSERTION\t`. And `0x00` as the binary marker is still a claim, so the
rule and the real binary sample that proves it land in the same change.

## Technical Context

**Language/Version**: Go 1.25 (`go.mod` is authoritative)

**Primary Dependencies**: standard library only, in every package this feature touches. No module is
added anywhere. `gatling/simlog` imports `gatling`, `gatling/text` and `model`, all in-module; the
`deps` job in `.github/workflows/verify.yml` excludes this module's own packages from its
stdlib-only check, so the new package passes it unchanged (research R12).

**Storage**: N/A — artefacts are read through `io.Reader`; nothing is persisted.

**Testing**: stdlib `testing`, table-driven; the existing golden corpus under
`testdata/corpus/gatling/3.11.5/` and `3.12.0/` proves text detection; one new binary **sample**
under `testdata/samples/gatling/binary/` proves binary detection and is explicitly not a corpus
entry (FR-031a). `go test -race -shuffle=on ./...`; integration suite behind `-tags=integration`.

**Engineering guidance**: classified by the constitution, Quality Gates & Tooling → Engineering
Guidance (Skills), added in v2.1.0 as a result of this feature's review; the feature-specific
working is in research [R13](./research.md#r13--which-go-skills-apply-and-which-must-not). This
change triggers every required-reading row — it adds exported identifiers, errors, tests, doc
comments and exported types. Mandated:
`golang-naming`, `golang-error-handling`, `golang-structs-interfaces`, `golang-testing`,
`golang-documentation`. Situational: `golang-benchmark`, `golang-safety`, `golang-lint`, and
`galaxio-gatling-pro` for capturing the binary sample. Forbidden: `golang-stretchr-testify` and
`golang-samber-*`/`golang-popular-libraries` (Principle IV forbids the dependency; Principle III
fixes stdlib `testing`), and the DI-container skills. `golang-project-layout` is **not** forbidden —
its `pkg/`, `cmd/`, architecture and DI questions are settled by Principle I and are not open, but
its `internal/` and package-boundary guidance is usable and becomes more so as `jmeter/`, `k6/`,
`locust/` and `phout/` land. This feature adds no `internal/` package, so no occasion arises here.

**Target Platform**: any Go 1.25 target; consumed as a library by galaxio-cli, the comet sidecar and
the Galaxio backend.

**Project Type**: library (Go module `github.com/galax-io/parsec`)

**Performance Goals**: this feature decodes nothing, so the goal is *constant overhead* — a cost
paid once per opened log that the size of the log cannot reach — and the benchmarks measure exactly
that (research R11). **Measured** on the largest corpus log, arm64, Go 1.26:

| | direct | dispatched | difference |
|---|---|---|---|
| `NewReader` + full read | ~83 µs, 41 allocs | ~89 µs, 46 allocs | **+5 allocs, +138 B**, constant |
| `Detect`, success paths | — | 5.2–10.7 ns, **0 allocs** | — |
| `Detect`, 14 B vs 1 MiB input | — | 6.905 ns vs 7.030 ns | input size is unreachable |

The five allocations are `io.MultiReader`, its slice, the `bytes.Reader` over the replayed head, the
head escaping to the heap, and the clone a refusal hands back so a caller on a stream it cannot
rewind still holds the whole log. They are paid once per log rather than per record. The plan first
claimed *at most one*; the benchmark said four, and then five once refusals learned to return their
bytes — the claim was corrected each time rather than the measurement explained away.

Spec 002's end-to-end figures are unchanged — a 1 GB log read in under 32 MiB peak and under 60 s on
a single core — because nothing on the decoding path moved.

**Constraints**: detection must leave the stream readable from byte 0 (FR-004 — a correctness
requirement, because the binary codec reconstructs a string cache from the start of the file); a
fixed detection window that does not grow with the input (FR-005); the version policy applied once
per read and before any record is decoded (FR-016, FR-018); chunked and whole-file reads must agree
through the new entry point as they do through the codec; no panic on any input; the 1 MiB per-line
ceiling `text.newScanner` enforces must not be raised by the plumbing.

**Scale/Scope**: Gatling, both log formats. Detection covers text (through 3.12.0) and binary (3.13.0
and newer); decoding covers 3.11.5 and 3.12.0, which is what the corpus proves. Three packages, one
of them new; roughly 300 lines of implementation and the tests that hold it. No new `model/` field,
no new record kind, no new capability.

## Constitution Check

*GATE: passed before Phase 0 research; re-checked after Phase 1 design — see
[Post-design re-check](#post-design-re-check).*

Source: `.specify/memory/constitution.md` **v2.1.0** — the version this branch itself ships, whose Engineering Guidance section was added as a result of this feature's skills review.

- [x] **I. Canonical Model First** — no new result data, so nothing is added to `model/`.
      `gatling/simlog` returns `model.Run` and `model.Item` through `text.RunReader`; it exports no
      result type of its own, and `Support` describes coverage rather than a result. No tool package
      imports another. Nothing here computes a count, a mean, a percentile, a range or a series —
      detection classifies bytes and the policy decides a verdict. `Capabilities` is untouched
      because no source field changes hands.
- [x] **II. Version-Gated, Streaming Decoders** — the gate is not weakened but centralised:
      `Policy.Apply` is the single decision point, called before any record is decoded, and strict
      mode only ever tightens it. Each codec's range still equals its corpus coverage and is read
      through `text.SupportedVersions()`, which a caller cannot widen. The entry point is
      `io.Reader`; detection reads a fixed 10 bytes and replays them with `io.MultiReader`, so peak
      memory is unchanged and bounded. Chunked and whole-file reads through `simlog` are asserted
      identical (quickstart scenario 5). Errors carry what they can: `SyntaxError` keeps its line
      number, `FormatError` carries the bytes examined. No panic, no `recover`.
- [x] **III. Golden-Corpus Testing** — text detection is proved against the two existing recordings
      rather than against hand-written bytes, and their opening `ASSERTION\t` is the evidence that
      falsifies issue #5's rule. Binary detection is proved by a sample cut from a real Gatling
      3.15.1 run, recorded with its provenance and named so it cannot be mistaken for a corpus
      entry — no complete run, no report, and nothing compares a decoder against it. **No new corpus
      entry is created and none is needed**: this feature supports no new version. Coverage floors
      are inherited automatically — `scripts/check-coverage.sh` maps `*/gatling/*` to 90% — so
      `gatling/simlog` is held to 90% from its first commit. Tests land with the change.
- [x] **IV. Minimal, Explicit Dependencies** — stdlib only; `go.mod` unchanged; no `replace`. Named
      here because Principle IV asks even when the answer is none.
- [x] **V. Compatibility-Sensitive Public API** — every addition and the one signature change are
      listed in [contracts/gatling-detect.md](./contracts/gatling-detect.md) with the doc comment
      each will carry. The change to `text.NewReader` and `text.NewRunReader` is source-compatible
      and behaviour-identical; it is nonetheless an ask-first item under `AGENTS.md` and is not
      implemented until approved. `CHANGELOG.md` entries are drafted in §6 of the contract and land
      in the same PR. Pre-v0.1.0, so no deprecation window is owed — and none is needed, since no
      identifier is removed or renamed.
- [x] **VI. Idiomatic, Simple Go** — errors as values, one type per cause, inspected with
      `errors.As`; enums behind an unknown zero value, the convention `Kind`, `Status`, `Event` and
      `Verdict` already set; variadic functional options with the `With` prefix Go uses for them,
      over an unexported configuration; no registration-by-init magic (research R1 rejects the
      `image.Decode` pattern with reasons). `Gate` is kept rather than replaced so the change stays
      additive, and `Policy.Apply` sits above it rather than beside it, so there is one way to ask
      each question. The API was reviewed against the Go convention skills before it was written
      (research R13), which is what renamed `Strict` to `WithStrict` and removed two exported
      identifiers that no caller needed. `.golangci.yml` unchanged — `errname`, `errorlint`,
      `wsl_v5` and `godot` already enforce much of what those skills describe.
- [x] **Workflow** — milestone v0.0.4, issue #5, one green `feat(gatling): …(#5)` commit; spec
      artifacts committed first as `docs(speckit): add 004-gatling-format-detection spec/plan/tasks`.

**No gate fails, so Complexity Tracking is empty.**

## Project Structure

### Documentation (this feature)

```text
specs/004-gatling-format-detection/
├── plan.md                        # This file
├── research.md                    # Phase 0 — 13 decisions, 4 questions carried forward
├── data-model.md                  # Phase 1 — the types and their transitions
├── quickstart.md                  # Phase 1 — 10 runnable validation scenarios
├── contracts/
│   └── gatling-detect.md          # Phase 1 — the exported API and the CHANGELOG plan
├── checklists/
│   └── requirements.md            # from /speckit-specify — 16/16
└── tasks.md                       # Phase 2 — /speckit-tasks, not created here
```

### Source Code (repository root)

```text
gatling/
├── format.go                      # NEW  Format, DetectSize, Detect
├── options.go                     # NEW  Option, WithStrict (config unexported)
├── policy.go                      # NEW  Policy, Policy.Apply
├── errors.go                      #      + FormatError, UnsupportedFormatError, UnverifiedError
├── version.go                     #      unchanged — Gate stays as it is
├── record.go  doc.go              #      unchanged
├── format_test.go  options_test.go  policy_test.go   # NEW
│
├── text/
│   ├── reader.go                  #      NewReader gains opts; finishPreamble calls Policy.Apply
│   ├── model.go                   #      NewRunReader gains opts, forwards them
│   ├── parse.go                   #      minVersion/maxVersion now feed a gatling.Policy
│   └── *_test.go                  #      unchanged except new strict-mode cases
│
└── simlog/                        # NEW PACKAGE
    ├── doc.go
    ├── simlog.go                  #      NewReader, NewRunReader, the head-replay plumbing
    ├── support.go                 #      Support, Supported
    ├── simlog_test.go  support_test.go  chunk_test.go  bench_test.go
    ├── example_test.go            #      ExampleNewRunReader — pkg.go.dev's first impression
    └── golden_test.go             #      //go:build integration — over the whole corpus

model/                             #      untouched
testdata/
├── corpus/gatling/3.11.5/  3.12.0/   #   untouched; proves text detection
└── samples/gatling/binary/           # NEW
    ├── 3.15.1-head.bin               #      first 256 bytes of a real 3.15.1 simulation.log
    └── SAMPLE.md                     #      provenance; states this is NOT a corpus entry
```

**Structure Decision**: one new package, `gatling/simlog`, because the dispatcher must import a codec
and `gatling` cannot (research R1: `gatling/text` imports `gatling`). Detection and the version
policy stay in `gatling` where issue #5 puts them and where they carry no codec dependency; only the
step that *chooses* a codec moves up. No `internal/` package is added — nothing here is shared
machinery that must be hidden. The binary sample lands under `testdata/samples/`, deliberately not
under `testdata/corpus/`, because FR-031a forbids it being counted as corpus.

## Post-design re-check

Re-read Principles I–VI against the artefacts produced in Phase 1. All still pass; three points are
worth recording because the design moved after the first check.

- **II — one gate, and one warning.** The design makes FR-016 true by construction rather than by
  test: `simlog` never reads a version, so it cannot raise a warning, and `Policy.Apply` is called
  exactly once per read by the codec. The test still counts, but the invariant is structural.
- **V — the API change got smaller during Phase 0.** The first sketch replaced `Gate` with a
  `Policy` method, which would have been a breaking change. Research R7 keeps `Gate` and builds
  `Apply` on top, so the only signature change left is the variadic options — source-compatible.
  The approval request in the contract shrank accordingly.
- **III — the sample is not corpus, and the design says so in three places.** FR-031a, the contract's
  §4, and the `testdata/samples/` path. The risk this guards against is a later reader treating 256
  bytes as a recording and comparing a decoder against it.

- **VI — the skills review moved the contract, and it is cheaper now than in review.** Reading the
  Go convention skills against the drafted API renamed `Strict()` to `WithStrict()` (`With*` is the
  convention, and mixing prefixes is the anti-pattern it prevents), dropped `ReadOptions` and
  `Options` from the exported surface in favour of an unexported config plus
  `Policy.Apply(found, opts...)`, and forced an explicit justification for returning an interface
  from a constructor — Go's default is to return a concrete type and to wait for a second
  implementation, and this design does neither. It also surfaced a hazard worth naming: `NewReader`
  returns an interface, so an error path that returns a typed nil `*text.Reader` produces a non-nil
  interface. Every error path returns a literal `nil`, and a test asserts it.

One thing the design cannot settle and must not pretend to: whether a real binary `simulation.log`
begins with `0x00`. Research R4 records it as a claim, and the task that writes the rule is the task
that captures the sample. If the sample disagrees, the recording wins and both `Detect` and the spec
are corrected — the rule spec 002 already set for source-derived claims.

## Complexity Tracking

No Constitution Check gate failed. This section is intentionally empty.
