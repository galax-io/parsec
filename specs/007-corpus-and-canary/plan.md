# Implementation Plan: The corpus and the canary

**Branch**: `007-corpus-and-canary` | **Date**: 2026-09-06 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/007-corpus-and-canary/spec.md`

**Milestone**: v0.0.7 — The corpus and the canary ([parsec#14](https://github.com/galax-io/parsec/issues/14),
[#60](https://github.com/galax-io/parsec/issues/60), [#61](https://github.com/galax-io/parsec/issues/61))

## Summary

Close the five gaps that stand between the two Gatling codecs and the v0.1.0 freeze
[parsec#13](https://github.com/galax-io/parsec/issues/13) asks for. Nothing in `model/` changes and
no record decodes differently; the work is evidence.

The technical spine is one idea: **every Gatling version states its run's numbers in a different
artefact, and all of them reduce to the same tree** — a node with a name, a parent and a
total/ok/ko triple. Research R1 confirmed this by walking 3.13.1's `js/stats.json` and 3.14.9's
`index.html` to identical trees. A new `internal/corpus` package holds that tree and three readers
for it; both codecs' test packages then compare per-request figures the same way, whether the
version wrote JSON, HTML or only a console summary.

Around that: the canary matrix widens from two versions to five so a live Gatling exercises the
binary codec; a `record-corpus` dispatch replaces the nine-step manual recording procedure; the
three existing fuzz targets get a bounded CI leg each; and the peak-memory assertion gains a field at
the string ceiling — which **fails today**, measured at 52.3 MiB against a documented 32 MiB budget,
and is why `MaxStringLen` comes down from 8 MiB to 1 MiB (approved 2026-09-06).

## Technical Context

**Language/Version**: Go 1.25 (`go.mod` is authoritative)

**Primary Dependencies**: standard library only — unchanged. Research R2 records the one place this
bit: extracting a per-request table from `index.html` would ordinarily reach for
`golang.org/x/net/html`, which Principle IV forbids here; `regexp` plus `html.UnescapeString` do the
job against markup that is machine-generated and class-tagged.

**Storage**: N/A — artefacts are read through `io.Reader`; nothing is persisted.

**Testing**: stdlib `testing`, table-driven; golden corpus under `testdata/corpus/gatling/<version>/`;
`go test -race -shuffle=on ./...`; integration suite behind `-tags=integration`; canary behind
`-tags=canary` with `PARSEC_CANARY_RUNS`; fuzzing under `-fuzz` in its own CI legs.

**Engineering guidance**: required-reading rows triggered — `golang-testing` (every task is a test
change; testify sections excluded by Principle IV), `golang-documentation` and `golang-naming`
(`MaxStringLen`'s value and doc comment), `golang-error-handling` (the extractor's failure paths),
`golang-structs-interfaces` (`internal/corpus` types). Consulted: `golang-benchmark` (this plan
states a peak-memory figure), `golang-safety`/`golang-security` (the fuzz legs are untrusted input),
`galaxio-gatling-pro`/`gatling-versions`/`gatling-build` (recording and version questions). No
disagreement with the constitution was found; the near-miss is recorded in research R2 and R9.

**Target Platform**: any Go 1.25 target; consumed as a library by galaxio-cli, the comet sidecar and
the Galaxio backend. Recordings produced by the new workflow are made on `ubuntu-latest`; the five
existing entries record macOS/arm64 and keep saying so.

**Project Type**: library (Go module `github.com/galax-io/parsec`)

**Performance Goals**: throughput unchanged — this feature adds no decode path. Peak memory is the
figure under revision: `Reader` documents and `TestPeakMemory` asserts **32 MiB**, and after this
change both hold for a field at `MaxStringLen` in every encoding. Measured worst case at a 1 MiB
ceiling is **6.8 MiB**; at today's 8 MiB it is **52.3 MiB** and the bound is false (research R8).

**Constraints**: no new module anywhere; no change to any decoded record or to either version gate;
the corpus stays under 5 MB (440 KB today); the peak-memory step runs without `-race`, which is why
CI already splits it out.

**Scale/Scope**: Gatling 3.11.5–3.12.0 (text) and 3.13.1–3.15.1 (binary) — five versions, five
recordings, three fuzz targets, two new workflows/legs, one new `internal/` package, one exported
constant.

## Constitution Check

*Constitution v2.2.0. Re-checked after Phase 1 design — verdicts unchanged.*

- [x] **I. Canonical Model First** — no `model/` type added or changed; no tool package gains an
      exported result type. **No statistic is computed by this module**: every figure compared is
      either read from Gatling's own report or folded inside a test, exactly as
      `gatling/text/helpers_test.go` and `gatling/binary/tolerance_test.go` already do. Absences
      stay declared: neither report nor console carries a virtual-user or error-record count, and
      those remain pinned against the recorded stream by name (FR-010). `internal/corpus` is
      internal by construction and exports nothing to consumers.
- [x] **II. Version-Gated, Streaming Decoders** — no gate is widened and no decode path changes. The
      canary's above-range case is preserved: it decodes, surfaces its warning, is reported as a
      candidate for widening, and is excluded from the equality comparison. `MaxStringLen` coming
      down **tightens** the allocation cap this principle requires. `TestCanaryCoversSupportedRange`
      in both codecs keeps "range equals corpus coverage" enforced rather than asserted.
- [x] **III. Golden-Corpus Testing (NON-NEGOTIABLE)** — this feature *is* Principle III. All five
      recordings keep their own tool report; the new comparison reaches the per-request rows those
      reports already carry (FR-006). Counts are compared **exactly** — they are discrete events, not
      measurements — and printed doubles at the precision the report printed them, the rule already
      in `sameAtPrecision`, documented at the assertion with its reason (FR-009). The recording
      workflow keeps artefacts as Gatling wrote them and cannot replace a recording note (FR-023).
      Race detector stays on everywhere except the peak-memory step, which CI already separates.
- [x] **IV. Minimal, Explicit Dependencies** — no module added; `go.mod` untouched. See Technical
      Context for the HTML-parsing decision and research R2 for why the standard library was used
      instead. `internal/corpus` is stdlib-only.
- [x] **V. Compatibility-Sensitive Public API** — one change: `gatling/binary.MaxStringLen`
      8 MiB → 1 MiB. It is an observable behaviour change on a compatibility-sensitive surface
      immediately before the v0.1.0 freeze, so AGENTS.md's *Ask first* applied and spec FR-028
      required approval — **given 2026-09-06** on the measurements in research R8, with the
      alternative (keep 8 MiB, restate the budget to ≥ 56 MiB) put alongside it and not taken.
      Contract: [contracts/public-api.md](contracts/public-api.md). A `CHANGELOG.md` **Changed**
      entry and restated doc comments land in the same PR. Every other interface this feature
      touches is operator-facing, not consumer-facing, and is recorded in
      [contracts/](contracts/README.md) anyway.
- [x] **VI. Idiomatic, Simple Go** — the report extractor is written once in `internal/corpus`
      rather than copied into a second test package, which would have been the sixth instance of the
      duplication [parsec#59](https://github.com/galax-io/parsec/issues/59) was filed about. **That
      issue's refactor is not done here** — it is not in this milestone, and AGENTS.md sends an
      out-of-scope refactor to its own PR; this feature only *adds* to the package #59 will grow.
      Errors are values; no panic control flow; fuzz targets exist precisely to prove no input
      reaches one.
- [x] **Workflow** — every issue belongs to milestone v0.0.7. Spec artifacts commit as
      `docs(speckit): add 007-corpus-and-canary spec/plan` before any `feat`/`fix`. Three issues →
      three green commits, each independently buildable (see Structure Decision).

**Gate verdict**: **PASS.** All six principles and the workflow rule hold. The one item that was
pending — Principle V's sign-off on `MaxStringLen` — was given on 2026-09-06, so no commit in this
feature is blocked. The justification stays recorded in Complexity Tracking, because an approved
deviation is still a deviation and the next reader needs the reasoning, not just the verdict.

## Project Structure

### Documentation (this feature)

```text
specs/007-corpus-and-canary/
├── plan.md              # This file
├── spec.md              # Phase −1 (/speckit-specify)
├── research.md          # Phase 0 — R1…R9, with the measurements
├── data-model.md        # Phase 1 — the report tree and what it is compared against
├── quickstart.md        # Phase 1 — how to run every check this adds
├── contracts/           # Phase 1 — the four interfaces that change
│   ├── README.md
│   ├── public-api.md            # MaxStringLen — the one consumer-facing change
│   ├── canary-env.md            # PARSEC_CANARY_RUNS, extended
│   ├── record-corpus-workflow.md
│   └── fuzz-ci.md
├── checklists/
│   └── requirements.md
└── tasks.md             # Phase 2 (/speckit-tasks — NOT created here)
```

### Source Code (repository root)

```text
internal/corpus/                       # NEW — the report tree and its three readers
  report.go                            #   Node, Triple, Report, Accounts(dir)
  stats_json.go                        #   FromStatsJSON  — 3.11.5, 3.12.0, 3.13.1
  report_html.go                       #   FromReportHTML — 3.14.0 and newer
  console.go                           #   FromConsole    — root only, absence declared
  *_test.go                            #   table-driven, over the committed recordings

gatling/binary/
  read.go                              # MaxStringLen 8 MiB → 1 MiB (approved)
  reader.go                            # the memory paragraph restated to the same budget
  tolerance_test.go                    # per-request comparison, was global-triple only
  synth_test.go                        # + a run carrying a ceiling field in each encoding
  memory_test.go                       # + the ceiling assertion
  canary_test.go                       # NEW — mirrors gatling/text/canary_test.go
  limits_test.go                       # re-checked against the new ceiling

gatling/text/
  helpers_test.go                      # report walking now goes through internal/corpus
  canary_test.go                       # unchanged in shape; reads the shared tree

.github/workflows/
  gatling-canary.yml                   # matrix 2 versions → 5; per-format decode step
  verify.yml                           # + a `fuzz` workflow_call input (default false)
                                       #   and the fuzz leg behind it
  ci.yml                               # passes fuzz: <event is a pull request>
  release.yml                          # UNCHANGED — the default keeps fuzzing off a release
  fuzz-nightly.yml                     # NEW — the longer scheduled run
  record-corpus.yml                    # NEW — one dispatch records a version

testdata/corpus/gatling/simulation/
  README.md                            # the manual procedure now points at the workflow

CHANGELOG.md                           # Changed: MaxStringLen
```

**Structure Decision**: one new package, `internal/corpus`. It exists because two external test
packages need the same report tree and a `_test.go` file cannot be imported across packages;
`internal/` keeps it invisible to consumers, and `internal/wire` is the precedent. It is named for
the package #59 proposes so that issue later grows it rather than creating a second one. No existing
helper moves into it in this feature. No other package is added, and `model/`, `gatling/simlog/`,
`gatling/text/`'s non-test code and every adapter package are untouched.

## Delivery order

Three tracked issues, three green commits. Nothing is blocked on an open decision.

| Commit | Issue | Stories | Depends on |
|---|---|---|---|
| `feat(corpus): verify every per-request figure against the run's own report (#14)` | #14 | US1, US2, US4 | — |
| `ci(fuzz): run every fuzz target on every pull request (#60)` | #60 | US3 | — |
| `fix(binary): hold the peak-memory bound at the string ceiling (#61)` | #61 | US5 | — |

Issue #14 carries three stories, so its commit is large — that is what AGENTS.md's *1 issue = 1
commit* asks for, and splitting it would leave `main` carrying a report tree nothing reads. The other
two touch disjoint files and can be developed and merged in any order, before or after #14.
[tasks.md](tasks.md) sequences all three.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|---|---|---|
| **Principle V**: `MaxStringLen` 8 MiB → 1 MiB, an observable behaviour change on a compatibility-sensitive API immediately before the v0.1.0 freeze — **approved 2026-09-06** | The documented 32 MiB peak-memory bound is **false** at 8 MiB — measured at 52.3 MiB for a log carrying one ceiling-sized field per encoding (research R8). A bound with an unstated exclusion is not a bound, and this one is stated in `Reader`'s own documentation. | *Raise the documented budget to ≥ 56 MiB and keep 8 MiB.* Permitted by FR-028 and rejected: the budget is what a consumer sizes a process against, and inflating it by 75% to accommodate a field no real log contains — the longest in the corpus is 51 bytes — trades a real guarantee for an unused one. *Lower to 4 MiB instead of 1 MiB:* passes here with 18% margin, which is too little for a figure that moves with GC scheduling across runners and Go releases. |
| A test-support package (`internal/corpus`) that ships in the module rather than living in `_test.go` files | Two external test packages need the same report tree, and Go cannot import a `_test.go` file across packages. | *Copy the extractor into both test packages.* Rejected by Principle VI and by [#59](https://github.com/galax-io/parsec/issues/59), which was filed about exactly this duplication in five places already. |
