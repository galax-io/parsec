---
description: "Task list for 006-fold-primitives"
---

# Tasks: The Primitives a Consumer Folds

**Input**: Design documents from `/specs/006-fold-primitives/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/model-fold.md](./contracts/model-fold.md),
[contracts/gatling-fixes.md](./contracts/gatling-fixes.md), [quickstart.md](./quickstart.md)

**Milestone**: [v0.0.6](https://github.com/galax-io/parsec/milestone/7) · **Issues**:
[parsec#8](https://github.com/galax-io/parsec/issues/8),
[#56](https://github.com/galax-io/parsec/issues/56),
[#57](https://github.com/galax-io/parsec/issues/57),
[#55](https://github.com/galax-io/parsec/issues/55)

**Tests**: REQUIRED (constitution Principle III). Every story phase lists its tests before its
implementation tasks. Tests are written first and MUST fail before the implementation task starts;
every bug fix ships a regression test that fails on `main`.

**Approvals**: both ask-first items — the zero-time convention (research R3) and the text codec
reporting three malformed values absent instead of refusing (R5) — **were approved by the
maintainer on 2026-09-06**. Nothing below waits on an answer.

**Organization**: grouped by user story in the spec's priority order. Four tracked issues map to
four commits, and a story does not always equal a commit: US1, US2 and US3 are one commit (#8),
US4 is one commit together with Phase 2 (#56), US5 (#57) and US6 (#55) are one each. The mapping
and the landing order are in [Commits](#commits) at the end.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependencies)
- **[Story]**: US1…US6, mapping to the user stories in spec.md

## Path Conventions

Single Go module, packages at the repository root. Tests live beside the code as `<file>_test.go`,
table-driven on stdlib `testing`; integration suites behind `//go:build integration`. Golden corpus
under `testdata/corpus/gatling/<version>/` — five runs already recorded, none added here. Hand-built
inputs come from `gatling/binary`'s test `builder` and are fixtures, not corpus.

---

## Phase 1: Setup

**Purpose**: nothing to scaffold and nothing to record; only the checks the constitution asks for
before code.

- [X] T001 Confirm `.golangci.yml` needs no change for this feature; if it does, justify it in `plan.md` Complexity Tracking
- [X] T002 [P] Read the required-reading skills not yet read at planning — `golang-error-handling`, `golang-testing`, `golang-documentation` — and record any disagreement with the constitution in `research.md` R13 (naming and structs were read for the plan)
- [X] T003 [P] Re-run the baseline on the branch base and keep the output for the #8 PR: `go test -tags=integration -run '^$' -bench 'BenchmarkDecodeToModel' -benchmem -benchtime=2s ./gatling/binary/` — research R12 recorded 107 ns/item on f454217

---

## Phase 2: Foundational — one sentinel, and absence that reaches the model (part of commit #56)

**Purpose**: the shared value both codecs write for a time they could not resolve, and the one
conversion that turns it into the zero `time.Time`. US2's bounds skip a zero start and US4's
agreement test needs both codecs to produce the same wire value, so this lands before either.

**⚠️ CRITICAL**: T006 and T007 must fail before T008 is written.

- [X] T004 Add `AbsentTimestamp` to `gatling/record.go` with the doc comment from [contracts/model-fold.md §3](./contracts/model-fold.md), and reword the `Record` field docs for `Start`, `End`, `Timestamp` and `CumulatedResponseTime` to name it (data-model §3)
- [X] T005 Replace the private `absentTime` in `gatling/binary/record.go` with `gatling.AbsentTimestamp`, keeping its reasoning comment at the use site in `instant`
- [X] T006 [P] Tests first in `internal/wire/wire_test.go`: `Millis(gatling.AbsentTimestamp).IsZero()` and `Millis(-1).IsZero()` are true; `Millis(0)` is 1970-01-01T00:00:00Z; `span(gatling.AbsentTimestamp, x)` and `span(x, gatling.AbsentTimestamp)` are unset; `Item` over a request record with an absent start yields `Sample.Start.IsZero()` and an unset `Duration`
- [X] T007 [P] Test first in `gatling/binary/review_test.go`, extending `TestAnUnresolvableOffsetIsAbsentNotFatal`: the model item for the unresolvable offset has a zero `Start`, not a date in the distant past — landed as `TestAnUnresolvableOffsetIsTheZeroTimeInTheModel` in `gatling/binary/absence_test.go`, a sibling with its own imports
- [X] T008 Implement in `internal/wire/wire.go`: `Millis` maps every negative millisecond count to the zero time (doc: a recorded instant is never the zero time); `span` returns an unset duration when either end is `gatling.AbsentTimestamp`, checked before `end < start` so the subtraction cannot overflow
- [X] T009 [P] Add the zero-time sentence from [contracts/model-fold.md §4](./contracts/model-fold.md) to `Sample.Start`, `GroupSample.Start`, `UserEvent.At` and `RunError.At` in `model/sample.go`, and state the rule once in the "Absence" section of `model/doc.go`
- [X] T010 Run the goldens without `-update` — `go test -tags=integration -run TestGolden ./gatling/text/ ./gatling/binary/` — and `go test -race -shuffle=on ./internal/wire/ ./gatling/...`; every recorded stream must be byte-identical

**Checkpoint**: an absent instant is the zero time end to end, and the corpus is unchanged.

---

## Phase 3: User Story 1 — Two consumers bucket the same run by the same key (P1) 🎯 MVP

**Goal**: `model.Position` — one comparable value for a sample's or a group traversal's place in
the run, unambiguous for any names, valid after the reader advances, recoverable to its path and
name.

**Independent Test**: fold a corpus run twice, in two pieces of code written without reference to
each other, keyed by position; the key sets and the counts under them are identical.

### Tests for User Story 1 (write first, MUST fail) ⚠️

- [X] T011 [P] [US1] Unit tests in `model/position_test.go`: a table over equality and inequality — a request outside any group against the same name inside one; a group traversal `a` → `b` against a request `b` inside `a`; a single group named `a,b` against nested `a` then `b`; names holding a comma, a slash, a tab and a NUL; an empty request name and an empty group name; the zero value against every constructed value (FR-001 … FR-004)
- [X] T012 [P] [US1] Round-trip tests in `model/position_test.go`: `Groups()`, `Name()` and `Kind()` return exactly what the constructor was given, for every row of T011; `Groups()` is empty and non-nil for a position with no groups and nil only for the zero value; `String()` renders the segments and the name joined by ` / ` and renders the zero value empty (FR-005, FR-007)
- [X] T013 [P] [US1] Validity test in `model/position_test.go`: take a position from a `Sample` whose `Groups` is a slice, overwrite every element of that slice, and require the position unchanged and still equal to a fresh one built from the original names (FR-006)
- [X] T014 [P] [US1] Extend `TestEnumStringsCoverEveryConstant` in `model/item_test.go` with `PositionKind`: "sample", "group", "unknown", and an out-of-range value reading as unknown
- [X] T015 [P] [US1] Corpus test in `gatling/text/fold_corpus_test.go` (new, `//go:build integration`): a `primitiveTally` keyed by `Position` over every text run, mapped onto `statsKey{path: strings.Join(p.Groups(), ","), name: p.Name()}` and asserted equal, key for key and count for count, to `modelTally` from `gatling/text/model_golden_test.go` (SC-001, FR-033)
- [X] T016 [P] [US1] Corpus test in `gatling/binary/fold_corpus_test.go` (new, `//go:build integration`): the same primitive tally over every binary run, asserted equal to the per-request and per-group `counts` that `gatling/binary/report_test.go` extracts from `index.html` (SC-001, FR-032)

### Implementation for User Story 1

- [X] T017 [US1] Implement `model/position.go`: `PositionKind` with `String`, `Position` with the length-prefixed key of data-model §1 built with `encoding/binary.AppendUvarint`, `NewSamplePosition`, `NewGroupPosition`, `Kind`, `Groups`, `Name`, `String`, each with the doc comment from [contracts/model-fold.md §1](./contracts/model-fold.md)
- [X] T018 [US1] Add `Sample.Position()` and `GroupSample.Position()` to `model/sample.go` with the contract's doc comments (depends on T017) — declared in `model/position.go` beside the type, so that commit #56's edit to `sample.go` and commit #8's stay separate diffs
- [X] T019 [US1] Make T015 and T016 pass: the primitive tallies fold positions and counts only at this point; bounds arrive in US2

**Checkpoint**: positions bucket every corpus run exactly as the hand-rolled fold does. Two
consumers now agree on which rows a run has.

---

## Phase 4: User Story 2 — Two consumers agree where the run begins and ends (P1)

**Goal**: `model.Bounds` — the span Gatling's own report uses, extended one item at a time,
absent when nothing counted, skipping an absent start and never extended by an absent end, a
cumulated duration, an error or the run's recorded start.

**Independent Test**: fold every corpus run through `Bounds` and reproduce the mean request rate
each run's own account printed, exactly, with Gatling's rounding.

### Tests for User Story 2 (write first, MUST fail) ⚠️

- [X] T020 [P] [US2] Unit tests in `model/bounds_test.go`, one row per rule of data-model §2: a user START before the first request sets the start; a user END after the last request sets the end; a sample with an unset `Duration` sets the start and leaves the end; a sample with a zero `Start` moves nothing; a group's `Start + Duration` sets the end and `CumulatedDuration` never does; `ItemError`, `ItemAssertion`, `ItemUnknown` and a `UserEventUnknown` move nothing; the same items shuffled give the same bounds; an empty fold reports both absent; samples without ends and no user events give a start and no end (FR-008 … FR-013)
- [X] T021 [P] [US2] Extend `primitiveTally` in `gatling/text/fold_corpus_test.go` with a `Bounds`: its `Start` and `End` equal `modelTally`'s `injectStart` and `injectEnd` for every text run, and `checkRates` from `gatling/text/helpers_test.go` reproduces `stats.json`'s mean requests per second from the primitive span rounded up to whole seconds (SC-002, FR-015)
- [X] T022 [P] [US2] Extend `consoleSummary` in `gatling/binary/report_test.go` to read the `mean throughput (rps)` line (landed as `reportedThroughput` in `gatling/binary/fold_corpus_test.go`, which also reads `global_stats.json` for 3.13.1, whose console has no such line) — total, OK, KO — and assert in `gatling/binary/fold_corpus_test.go` that count ÷ ceil(span ms ÷ 1000) from the primitive bounds equals each figure at the precision the console printed, for 3.13.1, 3.14.9 and 3.15.1; 3.15.1 must give 25.5, 21 and 4.5 (SC-002, FR-015)

### Implementation for User Story 2

- [X] T023 [US2] Implement `model/bounds.go`: `Bounds` with `Extend(*Item)`, `Start`, `End`, pointer receivers throughout, the rules of data-model §2, and the doc comments from [contracts/model-fold.md §2](./contracts/model-fold.md)
- [X] T024 [US2] Make T021 and T022 pass by folding a `Bounds` in both primitive tallies (depends on T019, T023)

**Checkpoint**: the bounds reproduce every run's own rate. Two consumers now divide by the same
span.

---

## Phase 5: User Story 3 — A consumer folds a run in one pass and this library computes nothing (P2)

**Goal**: the consumer's loop shown as an executable example, the exported surface pinned so a
statistic cannot appear unnoticed, and the fold measured for cost and memory.

**Independent Test**: the example prints the 3.15.1 console summary's figures from the primitives
alone; adding any exported identifier to `model` fails a test.

### Tests for User Story 3 (write first, MUST fail) ⚠️

- [X] T025 [P] [US3] `TestExportedSurfaceIsGolden` in `model/exports_test.go`: parse `model/*.go` excluding `_test.go` with `go/parser`, list every exported type, function, method, constant, variable and struct field in the forms of data-model §6, sort, and compare to `model/testdata/exports.golden`; a `-update` flag rewrites it. Fails first because the golden does not exist (FR-018, SC-003)
- [X] T026 [P] [US3] `Example_fold` in `model/example_fold_test.go` (package `model_test`): open `../testdata/corpus/gatling/3.15.1/simulation.log` through `gatling/simlog.NewRunReader`, fold success and failure counts per `Position` and one `Bounds` in a single loop, print the request total with its OK/KO split, the number of distinct positions, the span in whole seconds as Gatling rounds it, and the three rates; `// Output:` carries 102, 84, 18, 4 s and 25.5 / 21 / 4.5, with the distinct-position count read off the run and checked against the per-request rows of `testdata/corpus/gatling/3.15.1/index.html` (FR-021, SC-010)
- [X] T027 [P] [US3] `TestFoldPeakMemory` in `gatling/binary/memory_test.go` (measured: 256 MiB → 3.6 MiB peak, 2,560 MiB and 174,308,736 items → 3.7 MiB peak): the existing `sampler` over `newSynthLog`, read through `NewRunReader` taking `Position()` for every sample and group and `Extend`ing one `Bounds` for every item; a 256 MiB log peaks under 32 MiB and a tenfold longer one within twice that figure; `-short` skips the larger run (SC-004)
- [X] T028 [P] [US3] `BenchmarkFold` in `gatling/binary/bench_test.go` over the largest corpus log and the synthetic 64 MiB log, shaped like `BenchmarkDecodeToModel`, reporting `items/op`, `ns/op`, `B/op` and `allocs/op` (research R12)

### Implementation for User Story 3

- [X] T029 [US3] Generate `model/testdata/exports.golden` with `go test ./model/ -run TestExportedSurfaceIsGolden -update`, review every line against contract §1–§2 and FR-017 — no count, mean, extreme of a duration, standard deviation, percentile, range or series — and commit it (depends on T017, T018, T023)
- [X] T030 [US3] Add the section "The primitives a consumer folds" to `model/doc.go` per [contracts/model-fold.md §6](./contracts/model-fold.md), naming `Position` and `Bounds` beside the outcome predicate and the stream
- [X] T031 [US3] Run `go test -tags=integration -run '^$' -bench 'BenchmarkDecodeToModel|BenchmarkFold' -benchmem -benchtime=2s ./gatling/binary/`; record both lines under "Measured" in `plan.md` Technical Context; the fold must add at most one allocation per position and stay under twice the decode-to-model time per item, else apply the R12 fallback in its own measured commit
- [X] T032 [US3] Add the #8 entries to `CHANGELOG.md` under Unreleased → Added, from [contracts/model-fold.md §7](./contracts/model-fold.md)

**Checkpoint**: the consumer's loop is documented and executed; the exported surface is a diff a
reviewer approves. Commit #8 is complete.

---

## Phase 6: User Story 4 — Two logs of one run read the same, even where the input is bad (P2)

**Goal**: `gatling/text` gives the binary codec's answer to a negative time and a negative
cumulated response time, and a test drives both codecs with the same inputs so a further
divergence cannot land unnoticed. The group-less path is pinned, not changed — research R5 found it
already agrees.

**Independent Test**: hand each of the three inputs to both codecs and compare; then confirm every
golden record stream is byte-identical.

### Tests for User Story 4 (write first, MUST fail) ⚠️

- [X] T033 [P] [US4] `TestCodecsAgreeOnMalformedInput` in `gatling/binary/agreement_test.go` (new): a table of three — a request with a negative time (a negative offset from `builder`; `-5` in the text literal), a group with a negative cumulated response time, and a group-less first record — each read through `text.NewRunReader` and `binary.NewRunReader`, requiring identical items (kind, sample and group fields, `Groups` length and nil-ness) or identical error kinds via `errors.As` to `*gatling.SyntaxError`. Fails on `main` for the first two rows and passes for the third (FR-022, FR-024, SC-006)
- [X] T034 [P] [US4] Table test in `gatling/text/parse_test.go` (rows added to `TestParseRecord` and `TestParseRecordErrors`): a negative request start, request end, group start, group end, user event time and error time each yield `gatling.AbsentTimestamp` with no error; a negative cumulated response time is carried verbatim; a non-digit value and a value that overflows `int64` still fail with a `*gatling.SyntaxError` naming the line (FR-022)
- [X] T035 [P] [US4] Model test in `gatling/text/model_time_test.go`: a text record with a negative start reaches the model with a zero `Start` and an unset `Duration`, and a negative cumulated response time reaches it as an unset `CumulatedDuration` (FR-023)

### Implementation for User Story 4

- [X] T036 [US4] In `gatling/text/parse.go`: `parseTimestamp` accepts a minus sign followed by digits and returns `gatling.AbsentTimestamp`; fold `parseEnd` into it and delete the `neverCompleted` string, since the rule subsumes the sentinel; parse the cumulated response time as a signed integer carried verbatim; reword the `Expected` strings that said "a non-negative integer" and the doc comments to match (data-model §4)
- [X] T037 [US4] Run the goldens without `-update` and the full text and binary suites: every recorded stream byte-identical, T033–T035 green (FR-025, SC-007)
- [X] T038 [US4] Add the #56 entries to `CHANGELOG.md` under Unreleased — Added `gatling.AbsentTimestamp` and Changed (the zero-time convention, the text codec's three values) — from [contracts/model-fold.md §7](./contracts/model-fold.md) and [contracts/gatling-fixes.md §4](./contracts/gatling-fixes.md)

**Checkpoint**: both codecs give one answer. Commit #56 (Phase 2 + this phase) is complete.

---

## Phase 7: User Story 5 — A valid run with a large assertion suite decodes (P3)

**Goal**: each count read from the binary run record has its own ceiling, named for what it
bounds; a corrupt count is still stopped before it sizes anything.

**Independent Test**: a run record declaring 2,000 assertions decodes and returns all 2,000; a
claimed group depth of 1 << 20 still fails naming the offset.

### Tests for User Story 5 (write first, MUST fail) ⚠️

- [X] T039 [P] [US5] `TestALargeAssertionSuiteDecodes` in `gatling/binary/limits_test.go`: a `builder` run record with 2,000 one-byte payloads decodes and `Assertions()` returns 2,000 (SC-008, FR-027)
- [X] T040 [P] [US5] `TestAScenarioListAboveTheGroupDepthDecodes` in `gatling/binary/limits_test.go`: a run record declaring 1,025 scenario names decodes (FR-026)
- [X] T041 [P] [US5] `TestACorruptCountIsRefusedBeforeAllocating` in `gatling/binary/limits_test.go`: a scenario count, an assertion count and a group depth of `math.MaxInt32` each fail with a `*gatling.SyntaxError` at the count's own offset, and `testing.AllocsPerRun` shows nothing sized by the count; `TestAnAbsurdGroupDepthIsRefused` stays as it is (FR-027, FR-028)

### Implementation for User Story 5

- [X] T042 [US5] In `gatling/binary/record.go`: add `maxScenarios` and `maxAssertions` at 1 << 16 with the justifications of data-model §5; give `readStrings` a ceiling parameter and use `maxAssertions` in `readBlobs`; replace `maxGroupDepth/128` with `initialPathCap = 8`; reword `maxGroupDepth`'s comment to say it bounds nesting only
- [X] T043 [US5] Add the #57 entry to `CHANGELOG.md` under Unreleased → Fixed, from [contracts/gatling-fixes.md §4](./contracts/gatling-fixes.md)

**Checkpoint**: a large but honest count decodes; a corrupt one is refused early. Commit #57 is
complete.

---

## Phase 8: User Story 6 — The README says what the release does (P3)

**Goal**: the README agrees with `doc.go` about the binary codec, and the detection rule's
provenance points at a recording that exists.

**Independent Test**: no source file names the deleted sample directory; the README and `doc.go`
name the same packages and ranges.

### Tests for User Story 6 ⚠️

No code changes, so no Go test: the checks are the grep and the side-by-side read in
[quickstart.md Scenario 6](./quickstart.md), run in T047.

### Implementation for User Story 6

- [X] T044 [P] [US6] Edit `README.md` per the table in [contracts/gatling-fixes.md §3](./contracts/gatling-fixes.md): the `gatling/binary/` row in the package list; the Status paragraph saying a binary log is read over 3.13.1 through 3.15.1 through the same entry point; "except Gatling"; "both codecs share them"
- [X] T045 [P] [US6] Reword the provenance comments in `gatling/format.go` (the six-byte rule) and `gatling/format_test.go` (the golden head bytes) to cite `testdata/corpus/gatling/3.15.1/RECORDING.md`; `Detect` itself does not change (FR-030, FR-031)
- [X] T046 [P] [US6] Add one line to `testdata/corpus/gatling/3.15.1/RECORDING.md` stating that the log opens with `00 | 00 00 00 06 | "3.15.1"` — the kind byte, the release string's length, and the string
- [X] T047 [US6] Verify: `grep -rn 'testdata/samples' --include='*.go' .` prints nothing; the package lists in `README.md` and `doc.go` name the same five packages with the same ranges; `go test -race -run TestDetect ./gatling/` is green; add the #55 entry to `CHANGELOG.md` under Unreleased → Fixed (SC-009)

**Checkpoint**: the README no longer denies the headline feature. Commit #55 is complete.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: the gates every PR must pass, run once per commit and once over the whole milestone.

- [X] T048 [P] godoc review: every new exported identifier — `Position`, `PositionKind` and its constants, `NewSamplePosition`, `NewGroupPosition`, `Bounds` and its methods, `Sample.Position`, `GroupSample.Position`, `AbsentTimestamp` — carries the contract's doc comment; `go doc -all ./model ./gatling` reads cleanly
- [X] T049 Coverage: `go test -race -shuffle=on -cover ./model/ ./gatling/... ./internal/...` — at or above 90% for `model`, `gatling/text`, `gatling/binary`; 80% overall; numbers go in each PR description
- [X] T050 [P] Verify `model/` and `gatling/` are still stdlib-only (`go list -deps`, the `deps` job in `.github/workflows/ci.yml`) and `go mod tidy && git diff --exit-code go.mod go.sum` is clean
- [X] T051 Run every scenario in [quickstart.md](./quickstart.md) and correct the `-run` patterns it guessed at before the tests existed to name the tests that do
- [ ] T052 Linkage: `scripts/check-linkage.sh --pr 64` — the PR carries milestone v0.0.6 and closes every issue in it. **Open until it merges**: the milestone ships as one PR, [#64](https://github.com/galax-io/parsec/pull/64), whose six commits are the spec, #56, #57, #55, #8 and the closeout; the gate passes on it, and #56, #57, #55 and #8 close when it lands on `main`
- [X] T053 Confirm every test that passed before this feature still passes unchanged, and that `CHANGELOG.md` Unreleased carries every observable change under Added, Changed or Fixed (SC-011, FR-035) — green at the tip: `go test -race -shuffle=on ./...`, the integration suite in CI's two halves (`-race -skip 'PeakMemory$'`, then `-run 'PeakMemory$'` without the race detector), `golangci-lint` with 0 issues; and each of the four issue commits green on its own in a throwaway worktree

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: no dependencies.
- **Phase 2 (Foundational)**: no dependency on Phase 1 beyond T002. **Blocks US2's end-to-end
  property and US4**; US1 does not need it but lands in the same commit as US2, so it is done
  first in practice.
- **Phases 3–8 (Stories)**: US1 → US2 → US3 are sequential (US2's tallies extend US1's; US3's
  golden and example need both types). US4, US5 and US6 are independent of the three and of each
  other.
- **Phase 9 (Polish)**: per commit before its PR opens; the whole of it before the milestone is
  tagged.

### User Story Dependencies

- **US1 (P1)** — starts immediately. The MVP.
- **US2 (P1)** — depends on US1 (T019 → T024) and on Phase 2 (T008) for the zero-start rule to be
  reachable end to end; its unit tests need neither.
- **US3 (P2)** — depends on US1 and US2: the golden pins both types, the example uses both.
- **US4 (P2)** — depends on Phase 2 (T004, T005, T008); independent of US1–US3.
- **US5 (P3)** — independent of everything; touches `gatling/binary/record.go`, which T005 also
  edits, so serialise with Phase 2.
- **US6 (P3)** — independent of everything.

### Within Each Story

Tests written and failing before implementation · model types before the tallies that use them ·
the golden and the example after both types exist · story complete before the next.

### Parallel Opportunities

- T002 and T003 in Phase 1; T006, T007 and T009 in Phase 2
- Every test task marked [P] within a story
- US4, US5 and US6 by different people at any time after Phase 2; US1 → US2 → US3 by one person
- T048 and T050 in Phase 9

---

## Parallel Example: User Story 1

```bash
# Launch every test for User Story 1 together — they fail until T017 and T018 exist:
Task: "Unit tests for equality and inequality in model/position_test.go"
Task: "Round-trip and String tests in model/position_test.go"
Task: "Validity-after-overwrite test in model/position_test.go"
Task: "PositionKind strings in model/item_test.go"
Task: "Primitive tally over the text corpus in gatling/text/fold_corpus_test.go"
Task: "Primitive tally over the binary corpus in gatling/binary/fold_corpus_test.go"

# Then, sequentially:
Task: "Implement model/position.go"
Task: "Add Sample.Position and GroupSample.Position in model/sample.go"
```

---

## Commits

One tracked issue per commit, each green on its own, in the landing order
[contracts/gatling-fixes.md §5](./contracts/gatling-fixes.md) gives:

| Order | Commit | Tasks |
|---|---|---|
| 1 | `fix(gatling): give both codecs one answer to malformed input (#56)` | T004–T010, T033–T038 |
| 2 | `fix(gatling): give each untrusted count its own ceiling (#57)` | T039–T043 |
| 3 | `feat(model): the position and bounds a consumer folds (#8)` | T003, T011–T032 |
| 4 | `docs(readme): describe the binary codec v0.0.5 shipped (#55)` | T044–T047 |

T001, T002 and T048–T053 run for every PR. #56 lands first because #8's zero-start rule is only
reachable once `Millis` produces the zero time; #8 stacks on it and is updated with
`--force-with-lease` as it lands. The spec artefacts go ahead of all four as
`docs(speckit): add 006-fold-primitives spec/plan/tasks`.

---

## Implementation Strategy

**MVP = Phase 2 + US1 + US2.** At that point two consumers bucket by the same key and divide by
the same span, which is what galaxio-cli#51 is waiting for. It ships as one commit with US3, because
the exported-surface golden and the example are what make the boundary reviewable.

**Then**: US4 makes the two codecs one on bad input · US5 removes a wrong error · US6 makes the
README true.

**Order of work for one person**: Phase 2 and US4 first — small, and they open the #56 PR that #8
stacks on; then US1 → US2 → US3; then US5 and US6, which touch nothing the others do.

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Verify tests fail before implementing; a fix without a failing test is not a fix (Principle III)
- One tracked issue = one semantic commit, green on its own (`go build ./... && go test ./...`)
- Nothing here computes a statistic; a test that needs a count computes it itself
- Avoid: vague tasks, same-file conflicts, cross-story dependencies that break independence
