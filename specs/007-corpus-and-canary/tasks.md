---

description: "Task list for 007-corpus-and-canary"
---

# Tasks: The corpus and the canary

**Input**: Design documents from `/specs/007-corpus-and-canary/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md),
[data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

**Tests**: REQUIRED (constitution Principle III). This feature is almost entirely test and CI work —
the *deliverable is the evidence*. Tests are written first and MUST fail before the task that makes
them pass. Where a task is itself a test, "fails first" means it fails against today's code.

**Organization**: grouped by user story. Phase 2 is genuinely blocking — US1, US2 and US4 all read
the report tree it builds.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an incomplete task)
- **[Story]**: US1…US5 from [spec.md](spec.md)

## Path Conventions

Single Go module, packages at the repository root. Tests sit beside the code, table-driven on stdlib
`testing`. Golden corpus at `testdata/corpus/gatling/<version>/`.

## Commit mapping (AGENTS.md: 1 issue = 1 commit)

| Commit | Issue | Phases |
|---|---|---|
| `feat(corpus): verify every per-request figure against the run's own report (#14)` | [#14](https://github.com/galax-io/parsec/issues/14) | 2, 3, 4, 6 |
| `ci(fuzz): run every fuzz target on every pull request (#60)` | [#60](https://github.com/galax-io/parsec/issues/60) | 5 |
| `fix(binary): hold the peak-memory bound at the string ceiling (#61)` | [#61](https://github.com/galax-io/parsec/issues/61) | 7 |

#14 is one large commit because it is one issue. Phases 5 and 7 are independent of it and of each
other — all three can be developed in parallel and merged in any order. Phase 8 splits across the
three commits by the file each task touches.

---

## Phase 1: Setup

**Purpose**: establish the baseline every later task is measured against, and read what the
constitution requires before code is written.

- [ ] T001 Record the current baseline: run `go build ./... && go test -race -shuffle=on ./...`, `go test -tags=integration -race -shuffle=on -count=1 -skip 'PeakMemory$' ./...`, `go test -tags=integration -count=1 -run 'PeakMemory$' ./...` and `bash scripts/check-coverage.sh`, and note the per-package coverage numbers — they are the floor this feature must not drop below and go in the PR description.
- [ ] T002 [P] Read the required-reading skills named in [plan.md](plan.md) Technical Context — `golang-testing` (testify sections excluded by Principle IV), `golang-documentation`, `golang-naming`, `golang-error-handling`, `golang-structs-interfaces` — and record in [research.md](research.md) R9 any point where a skill and the constitution disagree.
- [ ] T003 [P] Confirm `.golangci.yml` needs no change for this feature; if it does, add a row to [plan.md](plan.md) Complexity Tracking justifying it before writing the code that needs it.

---

## Phase 2: Foundational — the report tree

**Purpose**: one shape for every version's account of itself, with three readers. Research
[R1](research.md) and [R2](research.md); entities in [data-model.md](data-model.md) §1.

**⚠️ BLOCKING**: US1, US2 and US4 all read this. Nothing in those stories starts until Phase 2 is green.

**Build tags**: `internal/corpus` and its own tests carry **no build tag**. The package reads recorded
artefacts but starts no tool, and its first consumers — `gatling/binary/tolerance_test.go` and
`report_test.go` — are themselves untagged and run in the ordinary `go test -race -shuffle=on ./...`
job. An `integration` tag here would keep the readers out of that job while the tests depending on
them still ran. (`gatling/text`'s equivalents *are* `integration`-tagged; the two codecs already
differ, and this feature does not change either.)

- [ ] T004 Create `internal/corpus/report.go` with `Node` (ID, Parent, Name, Kind, Requests, RatePerSec), `NodeKind` (`KindRoot`, `KindGroup`, `KindRequest`), `Triple` (Total, OK, KO int64) and `Report` (Nodes, Source), each with a doc comment stating what it is and where it comes from — per [data-model.md](data-model.md) §1. Add a `repoPath(parts ...string) string` helper beside them: a test runs with its own package directory as the working directory, so the corpus is at `../../testdata/corpus/gatling/…` from here. `gatling/binary/helpers_test.go` already has a `repoPath`; copy that spelling rather than `gatling/text`'s inline `filepath.Join("..", "..", …)`, so the two agree when [parsec#59](https://github.com/galax-io/parsec/issues/59) merges them.
- [ ] T005 Add `Report.validate()` to `internal/corpus/report.go` enforcing the invariants in [data-model.md](data-model.md) §1: exactly one node with an empty Parent and Kind `KindRoot`; every non-root Parent naming a node in the same Report; acyclic and fully reachable from the root; at least one node beyond the root. It returns an error naming the invariant broken — never a panic (Principle VI).
- [ ] T006 [P] Write `internal/corpus/stats_json_test.go` asserting that `FromStatsJSON` over `testdata/corpus/gatling/3.11.5/stats.json`, `3.12.0/stats.json` and `3.13.1/js/stats.json` yields the ten-node tree recorded in [research.md](research.md) R1 — names, parents and all three triples. It MUST fail before T008.
- [ ] T007 [P] Write `internal/corpus/report_html_test.go` asserting that `FromReportHTML` over `testdata/corpus/gatling/3.14.9/index.html` and `3.15.1/index.html` yields the same ten-node tree, that `Проверка /ok` survives as UTF-8, and that the two `GET /ok` rows are told apart by their parent and not by their name. It MUST fail before T009.
- [ ] T008 Implement `FromStatsJSON` in `internal/corpus/stats_json.go` — `encoding/json` over the `contents` tree, `json.Number` for printed doubles so precision is preserved for T022's comparison.
- [ ] T009 Implement `FromReportHTML` in `internal/corpus/report_html.go` — `regexp` over `<tr id=… data-parent=…>`, the `ellipsed-name` span and the `col-2`…`col-6` cells, with `html.UnescapeString` on the name. **Standard library only**: `golang.org/x/net/html` is forbidden here by Principle IV, and [research R2](research.md) records why.
- [ ] T010 [P] Implement `FromConsole` in `internal/corpus/console.go` — the Global Information request count in both shapes Gatling wrote it (`102 (OK=84 KO=18)` up to 3.13.x, `| 102 | 84 | 18` from 3.14.0), yielding a **root node only**. Its doc comment states that the console carries no per-request rows, so the absence is declared rather than inferred (FR-010).
- [ ] T011 Implement `Accounts(dir)` in `internal/corpus/report.go`, returning every reader that found something keyed by source, and add `internal/corpus/accounts_test.go` asserting: all five recordings yield at least one account; a directory with no account yields none (so a caller can fail); and a report whose body the extractor cannot recognise yields **zero nodes rather than a root alone**, which is what makes FR-011 fail loudly.
- [ ] T012 Add `internal/corpus/report_agreement_test.go`: for `3.13.1`, which carries *both* `js/stats.json` and `index.html`, assert the two readers produce **identical trees**. This is the only recording that can prove the HTML reader against the JSON one, and it exists for exactly that reason.

**Checkpoint**: `go test ./internal/corpus/` green; the tree is proven against every committed recording.

---

## Phase 3: User Story 1 — a live Gatling exercises the binary codec (P1) 🎯 MVP

**Goal**: every version in both supported ranges is decoded from a run a real Gatling produced during
the check, and held to that run's own account.

**Independent Test**: point `PARSEC_CANARY_RUNS` at a freshly produced binary run and confirm it
decodes and matches its own account — see [quickstart.md](quickstart.md) §4.

### Tests for User Story 1 (write first) ⚠️

- [ ] T013 [P] [US1] Write `gatling/binary/canary_test.go` behind `//go:build canary`, mirroring `gatling/text/canary_test.go`: `canaryRuns(t)` parsing `PARSEC_CANARY_RUNS` per [contracts/canary-env.md](contracts/canary-env.md), `summarize` writing to `GITHUB_STEP_SUMMARY`, and `TestCanary` decoding each fresh run and comparing it to `internal/corpus.Accounts(run.dir)`. Skips with a reason when the variable is unset — it never fakes a run.
- [ ] T014 [P] [US1] Add `TestCanaryCoversSupportedRange` to `gatling/binary/canary_test.go`, failing when either bound of `binary.SupportedVersions()` (3.13.1, 3.15.1) was not among the runs. This is what stops the gate and the canary drifting apart (FR-002).
- [ ] T015 [P] [US1] Add `TestCanaryCrossVersion` to `gatling/binary/canary_test.go`: fresh runs of the same probe agree as multisets once timing, run identity, record order and the check-failure message text are set aside — the exclusions `gatling/binary/crossversion_test.go` already documents and the reasons for each.
- [ ] T016 [US1] Add `TestCanaryCrossFormat` to `gatling/binary/canary_test.go` (the package already imports `gatling/text` in `agreement_test.go`): a text run and a binary run of the same probe agree, with the group-name spelling normalised — text records `inner  with comma`, binary records `inner, with comma`, and [research R5](research.md) records why both are correct. The assertion states that reason inline.
- [ ] T017 [P] [US1] Add a case to `gatling/binary/canary_test.go` asserting an **above-range** run decodes, surfaces its warning through `Warnings()`, is summarised as a candidate for widening the range, and is **excluded** from the equality comparison — a warning is a statement about identity, not content (FR-004).
- [ ] T018 [P] [US1] Add a case to `gatling/binary/canary_test.go` for a **below-range** run: the decode is refused with an error naming both the version found and the range supported, and the canary reports that refusal as the **expected outcome** rather than as a failure of the codec (spec Edge Cases). Without it the canary would go red the first time someone lists an old version, and the gate's refusal — a correct behaviour — would read as a defect.

### Implementation for User Story 1

- [ ] T019 [US1] Widen the version list in `.github/workflows/gatling-canary.yml` from `["3.11.5","3.12.0"]` to all five, in both the `workflow_call` and `workflow_dispatch` input defaults and the `run` job's matrix fallback.
- [ ] T020 [US1] In `.github/workflows/gatling-canary.yml`, make the "decode the fresh run" step select `./gatling/text/` or `./gatling/binary/` by the matrix version's format, keeping the existing "no canary test ran" guard so a skip can never pass for a run (FR-005).
- [ ] T021 [US1] In the `compare` job of `.github/workflows/gatling-canary.yml`, run the cross-version and cross-format tests over every downloaded run, so the two formats are compared to each other and not only within themselves (FR-003).

**Checkpoint**: `gh workflow run gatling-canary.yml` green across five versions; three of five were never exercised live before (SC-001).

---

## Phase 4: User Story 2 — per-request figures against the run's own report (P2)

**Goal**: every per-request and per-group row the recorded reports state is compared against the same
figure folded from the decoded records — all five versions.

**Independent Test**: `go test -tags=integration -race -count=1 -run 'Report|Tolerance' ./gatling/...`
with no Gatling and no network; then alter one expected value and watch exactly one check fail.

### Tests for User Story 2 (write first) ⚠️

- [ ] T022 [P] [US2] Extend `gatling/binary/tolerance_test.go` with `TestDecodedPerRequestFiguresMatchTheRunReport`: fold the decoded records into a per-`statsKey` tally and compare every node of `internal/corpus.Accounts(dir)` against it. Counts compared **exactly** — discrete events, not measurements — and printed doubles at the precision the report printed them, with the reason documented at the assertion (FR-009). MUST fail before T025.
- [ ] T023 [P] [US2] Add to `gatling/binary/tolerance_test.go` a two-way completeness assertion: every row the report names is accounted for in the decoded stream, and every request and group in the decoded stream is accounted for in the report; a mismatch either way fails and names the row (FR-008).
- [ ] T024 [P] [US2] Add `gatling/binary/report_absence_test.go` asserting that for a recording whose account is console-only the excluded figures are **named** and pinned against the recorded record stream instead, and that a recording yielding no account at all fails rather than passing quietly (FR-010, FR-011).

### Implementation for User Story 2

- [ ] T025 [US2] Add the per-`statsKey` tally to `gatling/binary`: extend the fold in `gatling/binary/tolerance_test.go` (today a global triple only) to key requests by group path plus name and groups by path, matching `gatling/text/helpers_test.go`'s `statsKey` exactly so the two corpora stay comparable.
- [ ] T026 [US2] Route `gatling/text/helpers_test.go`'s report walking through `internal/corpus` — replace the local `reportStats`/`reportNode`/`loadJSON`/`walk` with the shared tree. **Behaviour must not change**: `gatling/text/report_test.go` and the text canary pass identically before and after.
- [ ] T027 [US2] Update `gatling/binary/report_test.go` to build its `counts` from `internal/corpus`, removing the local `statsJSON`/`consoleSummary`/`consoleOld`/`consoleNew` extraction now that `FromStatsJSON` and `FromConsole` own it.
- [ ] T028 [US2] Verify FR-012 and SC-003 by hand and record both in the PR. **Two expected files, two different checks**: altering `records.golden` must fail `TestGolden` (`golden_test.go`), and altering a figure in the run's own report — a `col-2` cell in `testdata/corpus/gatling/3.15.1/index.html` — must fail the report/tolerance test. The report and tolerance tests decode `simulation.log` and never open `records.golden`, so one substitution does not exercise the other. Both procedures are in [quickstart.md](quickstart.md) §1; restore both files afterwards.

**Checkpoint**: the number of report figures that go uncompared is zero or named as an exclusion (SC-002).

---

## Phase 5: User Story 3 — the fuzzers run on every pull request (P3)

**Goal**: every fuzz target runs beyond its seed corpus, bounded, on every pull request; a crasher
fails the job and is downloadable.

**Independent Test**: revert the v0.0.5 `math.MinInt32` fix on a branch and confirm the leg fails
within its budget.

**Independent of Phases 2–4** — it touches only `.github/workflows/`, so it can be built and merged
in parallel. Contract: [contracts/fuzz-ci.md](contracts/fuzz-ci.md).

### Tests for User Story 3 (write first) ⚠️

- [ ] T029 [P] [US3] Confirm the acceptance case before building the leg: on a scratch branch revert the `math.MinInt32` guard in `gatling/binary/read.go`, run `go test -run '^$' -fuzz '^FuzzDecode$' -fuzztime 90s ./gatling/binary/`, and record how long the crasher took to appear. That number, measured on a CI runner rather than a laptop, sets the budget (FR-014). Discard the scratch branch.
- [ ] T030 [P] [US3] Verify target discovery: `go test -list '^Fuzz' ./...` names `FuzzDetect` (`gatling`), `FuzzDecode` (`gatling/binary`) and `FuzzReader` (`gatling/text`), and its output shape is what the matrix generator parses. A hard-coded list is forbidden — FR-013 says *every* target, and a list stops covering one added later.

### Implementation for User Story 3

- [ ] T031 [US3] Add a boolean `fuzz` input to `.github/workflows/verify.yml`'s `workflow_call` block, default **`false`**, documented as "pull requests only: fuzzing is nondeterministic, and a release must not be gated on a finding that depends on the seed". Pass `fuzz: ${{ github.event_name == 'pull_request' }}` from the `gates` job in `.github/workflows/ci.yml`. Leave `.github/workflows/release.yml` **unchanged** — the default keeps a release from turning the leg on by omission. See [contracts/fuzz-ci.md](contracts/fuzz-ci.md) *Where the leg lives*: `verify.yml` is called by the release path as well as the pull-request path, and its own header forbids adding a gate anywhere else, so an input is the only fix that keeps both rules.
- [ ] T032 [US3] Add a `fuzz-targets` job to `.github/workflows/verify.yml`, guarded `if: inputs.fuzz`, that runs `go test -list '^Fuzz' ./...`, turns its output into a JSON matrix of `{package, target}` pairs, and exposes it as a job output. Fail the job when the matrix is empty — a discovery step that silently found nothing reads exactly like one that found everything.
- [ ] T033 [US3] Add the `fuzz` job to `.github/workflows/verify.yml`, guarded `if: inputs.fuzz`, matrix over `fuzz-targets`' output, running `go test -run '^$' -fuzz '^<target>$' -fuzztime <budget> ./<package>/`. **One job per target**: a single fuzzer saturates every core it is given (862% CPU measured, [research R6](research.md)), so three in one job would each get a third of the budget the flag appears to grant. `-run '^$'` is required, or the ordinary tests spend the budget first.
- [ ] T034 [US3] In the `fuzz` job, upload `**/testdata/fuzz/**` as an artefact `if: failure()` and let the non-zero exit fail the job (FR-015). Grant the workflow no write permission, so the generated corpus can never be committed (FR-016).
- [ ] T035 [P] [US3] Add `.github/workflows/fuzz-nightly.yml`: the same discovery and matrix on a schedule, with a budget several times the pull-request one (FR-017), uploading any crasher the same way.
- [ ] T036 [US3] Confirm the leg passes on the current decoder within its budget on the runner (FR-014's second half), and record the observed execution count in the PR description.

**Checkpoint**: reintroducing the v0.0.5 defect fails a pull request with no human reading the diff (SC-006).

---

## Phase 6: User Story 4 — one dispatch records a version (P4)

**Goal**: producing a corpus entry is one dispatch plus the note the recorder writes.

**Independent Test**: dispatch for a version already recorded and confirm it reproduces that entry's
mechanical contents. Contract: [contracts/record-corpus-workflow.md](contracts/record-corpus-workflow.md).

### Tests for User Story 4 (write first) ⚠️

- [ ] T037 [P] [US4] Add `gatling/binary/entry_test.go` (and the text counterpart if absent) asserting what a **complete entry** is per [data-model.md](data-model.md) §3: every corpus directory carries `simulation.log`, `records.golden`, `RECORDING.md`, and at least one artefact from which `internal/corpus.Accounts` yields a root. It MUST fail for a directory missing any of them.
- [ ] T038 [P] [US4] Add a case to `gatling/binary/entry_test.go` asserting the whole corpus stays under the 5 MB ceiling (FR-024, SC-005) — 440 KB today, so it fails only if the render-only assets creep back in.

### Implementation for User Story 4

- [ ] T039 [US4] Add `.github/workflows/record-corpus.yml` with a required `version` input and an optional `description`, reusing `gatling-canary.yml`'s stub-start, port-wait and sbt steps. It redirects standard output to `console.txt` — the console summary exists only if captured at run time and can never be recovered afterwards.
- [ ] T040 [US4] In `record-corpus.yml`, fail with no artefact when the run failed or produced no report directory (FR-022). A run that misses the probe's declared expectations, or that cannot read back what it wrote — Gatling 3.13.0 — is not an entry.
- [ ] T041 [US4] In `record-corpus.yml`, select the entry's files **by presence, not by a version table**: keep `simulation.log`, `index.html`, `js/{global_stats,stats}.json`, `js/{stats.js,all_sessions.js,assertions.xml}` and `console.txt` when they exist; drop `js/highstock.js`, `js/jquery-*.js`, `js/bootstrap.min.js`, `js/highcharts-more.js`, the rest of `js/` and all of `style/` (FR-020, FR-021).
- [ ] T042 [US4] In `record-corpus.yml`, generate `records.golden` by running the matching codec's golden test with `-update`, then upload the assembled directory as `corpus-entry-<version>`. The job **publishes and never commits** (FR-019) — it is granted no write permission.
- [ ] T043 [US4] In `record-corpus.yml`, write a `RECORDING.md` **scaffold** carrying the platform the run was made on, the version, the build command and the artefacts kept, plainly marked incomplete (FR-023). It never replaces an existing entry's note.
- [ ] T044 [US4] Rewrite the "Record a version" section of `testdata/corpus/gatling/simulation/README.md` to point at the dispatch, keeping the manual procedure below it as the fallback for iterating on a failure, and stating that new entries are recorded on `ubuntu-latest` while the five existing ones record macOS/arm64.

**Checkpoint**: recording a version costs one dispatch and the note; zero manual copy steps, zero local tools (SC-004).

---

## Phase 7: User Story 5 — the peak-memory bound covers a field at the ceiling (P5)

**Goal**: the budget `Reader` documents is the budget the check asserts, and it holds for a field at
the string ceiling in every encoding.

**Independent Test**: `go test -tags=integration -count=1 -run 'PeakMemory$' ./gatling/binary/` —
**without `-race`**, which moves the very `HeapAlloc` figure being asserted.

**Independent of Phases 2–6.** `MaxStringLen` 8 MiB → 1 MiB was approved 2026-09-06 —
[contracts/public-api.md](contracts/public-api.md).

### Tests for User Story 5 (write first) ⚠️

- [ ] T045 [P] [US5] Extend `gatling/binary/synth_test.go` with a run carrying one field at `MaxStringLen` in each of the three encodings the format can store one in — Latin-1 ASCII, Latin-1 above ASCII (bytes ≥ 0x80), and UTF-16. **Stream it, never materialise it**: a log held in a `[]byte` sits on the heap and is counted in every sample, which reports the input's own size back as the peak ([research R8](research.md)).
- [ ] T046 [US5] Add `TestPeakMemoryAtTheStringCeiling` to `gatling/binary/memory_test.go` asserting the 32 MiB budget over that log. It MUST fail against today's 8 MiB ceiling — measured 52.3 MiB — which is the whole point of the task order.
- [ ] T047 [P] [US5] Add a case to `gatling/binary/memory_test.go` covering the *other* ceiling: fill `maxAssertionBytes` with payloads at the string ceiling and assert the same budget. Measured 9.3 MiB at a 1 MiB ceiling ([research R8](research.md)); the synthetic log has never carried an assertion payload, so this path is as untested as the string one was.

### Implementation for User Story 5

- [ ] T048 [US5] Confirm the measurement **on a CI runner before the constant moves** (FR-027). Push T045–T047 alone on a scratch branch so `TestPeakMemoryAtTheStringCeiling` runs in the e2e job's `-run 'PeakMemory$'` step at the current 8 MiB ceiling, and record the peak it reports there. Every figure in [research R8](research.md) was measured locally on ten cores; FR-027 asks for the runner, and T029 already sets that precedent for the fuzz budget. If the runner's number differs materially from 52.3 MiB, revisit the choice of 1 MiB in [contracts/public-api.md](contracts/public-api.md) before T049 rather than after.
- [ ] T049 [US5] Change `MaxStringLen` from `8 << 20` to `1 << 20` in `gatling/binary/read.go`.
- [ ] T050 [US5] Restate the doc comment on `MaxStringLen` in `gatling/binary/read.go` and the memory paragraph on `Reader` in `gatling/binary/reader.go` so both name the **same** 32 MiB budget and the measured multiple, making FR-026 visibly true rather than merely intended.
- [ ] T051 [US5] Resize `TestAssertionPayloadsPastTheByteCeilingAreRefused` in `gatling/binary/limits_test.go` from two payloads of 4608 KiB to **nine of 1 MiB**. At the new ceiling each 4.5 MiB payload is refused by `sized()` first, with `"a length past the maximum this codec will allocate"`, so the test's `strings.Contains(se.Found, "past the ceiling")` fails and it stops exercising `maxAssertionBytes` at all ([research R8](research.md)).
- [ ] T052 [P] [US5] Re-check every other assertion in `gatling/binary/limits_test.go` and `gatling/binary/read_test.go` that names a size relative to `MaxStringLen` — including `read_test.go:201`'s `cap(r.scratch) > MaxStringLen` — and confirm each still tests what its name claims at 1 MiB.
- [ ] T053 [US5] Add the `CHANGELOG.md` **Changed** entry under Unreleased naming the constant, both values, and the reason (Principle V: a pre-v0.1.0 exported identifier may change, but every such change is recorded).

**Checkpoint**: documented budget and asserted budget are the same number, and it holds (SC-007).

---

## Phase 8: Polish & Cross-Cutting

Split across the three commits by the file each task touches.

- [ ] T054 [P] Add a `fuzz` line to the Commands block in `AGENTS.md`, beside `canary` and `integration`, so the local equivalent of the new CI leg is discoverable.
- [ ] T055 [P] godoc review: every new exported identifier in `internal/corpus` has a doc comment stating what it does; `MaxStringLen`'s and `Reader`'s restated comments read correctly in `go doc`.
- [ ] T056 Coverage: `bash scripts/check-coverage.sh --enforce` — ≥ 90% for `model/` and `gatling/...`, ≥ 80% overall. `internal/corpus` is report-only and enforced through the module floor; confirm it does not drag the overall number below 80%, and put the per-package figures in the PR description.
- [ ] T057 [P] Confirm `model/` and `gatling/` are still standard-library only (`go list -deps` per the `deps` job) and that `go mod tidy` leaves the tree unchanged — `internal/corpus` must not have pulled anything in.
- [ ] T058 [P] Run every shell gate: `for t in scripts/*_test.sh .claude/hooks/*_test.sh .githooks/*_test.sh; do bash "$t"; done`.
- [ ] T059 Run [quickstart.md](quickstart.md) end to end and correct anything it gets wrong — it is the document a maintainer will actually follow.
- [ ] T060 Assign every PR to milestone **v0.0.7** and close [#14](https://github.com/galax-io/parsec/issues/14), [#60](https://github.com/galax-io/parsec/issues/60) and [#61](https://github.com/galax-io/parsec/issues/61) as their commits land on `main` (AGENTS.md *Milestones*). `bash scripts/check-linkage.sh` names the active milestone.

---

## Dependencies & Execution Order

### Phase dependencies

- **Phase 1 (Setup)**: no dependencies.
- **Phase 2 (Foundational)**: after Setup. **Blocks Phases 3, 4 and 6.**
- **Phase 3 (US1)**, **Phase 4 (US2)**, **Phase 6 (US4)**: after Phase 2; independent of each other.
- **Phase 5 (US3)** and **Phase 7 (US5)**: after Setup only — independent of Phase 2 and of each other.
- **Phase 8 (Polish)**: after the phases whose files it touches.

```
Phase 1 ──┬─► Phase 2 ──┬─► Phase 3 (US1)  ─┐
          │             ├─► Phase 4 (US2)  ─┤
          │             └─► Phase 6 (US4)  ─┤
          ├─► Phase 5 (US3) ────────────────┤──► Phase 8
          └─► Phase 7 (US5) ────────────────┘
```

### Within Phase 2

T004 → T005 first (the types). T006 and T007 are written next, in parallel, and MUST fail. T008 and
T009 then make them pass. T010 is independent of both. T011 needs T008–T010; T012 needs T008 and T009.

### Within each user story

Tests are written and fail before the implementation that satisfies them.

**Phase 7's ordering is load-bearing rather than ceremonial.** T046 must be *seen to fail* at 8 MiB,
and T048 must confirm the figure on a runner, or the change to `MaxStringLen` in T049 rests on a
laptop measurement alone (FR-027).

**Phase 5 has one ordering constraint**: T031 adds the `fuzz` input to `verify.yml` and wires
`ci.yml` to it; T032 and T033 are guarded by that input, so they follow it. Get this wrong and the
fuzz leg gates releases as well as pull requests — see [contracts/fuzz-ci.md](contracts/fuzz-ci.md).

### Parallel opportunities

- **Three developers, no contention**: one on Phases 2→3→4→6 (#14), one on Phase 5 (#60), one on
  Phase 7 (#61). The three commits touch disjoint files apart from `CHANGELOG.md`.
- Within Phase 2: T006 ∥ T007; then T008 ∥ T009 ∥ T010.
- Within Phase 3: T013, T014, T015, T017 and T018 are all `[P]` — but they land in one new file, so
  either one developer writes them in sequence or they are split by function.
- Within Phase 4: T022, T023, T024 in parallel.
- Within Phase 5: T029 ∥ T030 (both measurements, neither writes a workflow); then T031 before
  T032–T034; T035 is independent of all of them.
- Within Phase 7: T045 ∥ T047; then T046, then T048 on a runner, and only then T049–T053.

---

## Parallel Example: Phase 2

```bash
# The two reader tests, written together — both must fail first:
Task: "internal/corpus/stats_json_test.go — the ten-node tree from 3.11.5, 3.12.0, 3.13.1"
Task: "internal/corpus/report_html_test.go — the same tree from 3.14.9 and 3.15.1"

# Then the three readers, in parallel:
Task: "FromStatsJSON in internal/corpus/stats_json.go"
Task: "FromReportHTML in internal/corpus/report_html.go"
Task: "FromConsole in internal/corpus/console.go"
```

---

## Implementation Strategy

### MVP (User Story 1)

1. Phase 1 → Phase 2 → Phase 3.
2. **STOP and validate**: `gh workflow run gatling-canary.yml` across five versions. Three of the five
   have never been exercised against a live Gatling; that alone is the milestone's headline gap closed.

### Incremental delivery

1. Phase 2 → the tree is proven against all five recordings.
2. **+ Phase 3 (US1)** → live Gatling covers the binary codec. *Demo.*
3. **+ Phase 4 (US2)** → per-request figures verified everywhere. *Demo.*
4. **+ Phase 6 (US4)** → recording is one dispatch. Commit `#14` is now complete.
5. **+ Phase 5 (US3)** → fuzzers guard every pull request. Commit `#60`.
6. **+ Phase 7 (US5)** → the memory bound is true again. Commit `#61`.
7. Phase 8 → polish, coverage, milestone hygiene.

Order 5 and 6 can come first; they are independent and each is a smaller, faster PR than #14.

---

## Notes

- `[P]` = different files, no dependency on an incomplete task.
- One tracked issue = one semantic commit, green on its own (`go build ./... && go test ./...`).
- **The peak-memory step never runs under `-race`** — CI already splits it out, and the detector moves
  the figure being asserted.
- **Never commit anything under `testdata/fuzz/`.** A crasher is an artefact to download, not a file
  to check in.
- A canary or corpus test that cannot reach its tool **skips with a reason**; the workflow fails when
  no test passed, so a skip can never be mistaken for a run.
- Statistics stay the consumer's arithmetic: every figure compared here is folded inside a test. This
  module still exports no count, mean, percentile, range or series.
