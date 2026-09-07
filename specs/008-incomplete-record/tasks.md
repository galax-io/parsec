# Tasks: An incomplete record

**Input**: Design documents from `/specs/008-incomplete-record/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md),
[data-model.md](data-model.md), [contracts/](contracts/)

**Tests**: Test tasks are REQUIRED (constitution Principle III, Golden-Corpus Testing). Every story
phase lists its tests before its implementation tasks; each test is written first and MUST fail
before the implementation task it guards begins. The `/speckit-tasks` skill calls tests optional;
this repository's constitution overrides it, as `.specify/templates/tasks-template.md` already
states.

**Organization**: grouped by user story, so each is implementable and testable on its own.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel — different files, no dependency on an unfinished task
- **[Story]**: which user story the task serves (US1, US2, US3)
- Every task names the exact file it touches

## Path Conventions

parsec is a single Go module with packages at the repository root. Tests live beside the code they
cover, table-driven on stdlib `testing`. The five recordings under `testdata/corpus/gatling/<version>/`
are the only inputs; **no cut log is committed** — cut inputs are derived in-test (research R7).

---

## Phase 1: Setup

**Purpose**: establish the baseline the "no regression" claims are measured against. No package is
created and no corpus is recorded — this feature adds no version (plan.md, Constitution Check III).

- [ ] T001 [P] Record the pre-change baseline: run `go test -run '^$' -bench . -benchmem ./gatling/...` plus the peak-memory tests, and write the figures into the Performance Goals section of specs/008-incomplete-record/plan.md
- [ ] T002 [P] Confirm .golangci.yml needs no change for this feature; if it does, justify the change in the Complexity Tracking table of specs/008-incomplete-record/plan.md
- [ ] T003 Confirm the tree is green before any edit: `go build ./... && go test -race -shuffle=on ./...`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the error type both US1 and US3 are written against.

**⚠️ CRITICAL**: US1 and US3 cannot begin until this phase is complete. **US2 does not depend on it**
and may run at any time (research R6 — it needs no code).

- [ ] T004 [P] Write the error-type test in gatling/errors_test.go: the message for a text log names the line and for a binary log the offset, `errors.Is(err, io.EOF)` is false, `errors.As` reaches the type, and the rendered text says the log was cut short rather than malformed — MUST fail, the type does not exist yet
- [ ] T005 Add `TruncationError` (Format, Line, Offset, Dropped, Expected) with its doc comment and `Error()` to gatling/errors.go, exactly as [contracts/public-api.md](contracts/public-api.md) states, with no `Unwrap` (research R2)

**Checkpoint**: `go test ./gatling/` green; the type exists and is documented.

---

## Phase 3: User Story 1 — A killed run still reports what it recorded (Priority: P1) 🎯 MVP

**Goal**: every record that completed before a cut is delivered, and the read ends by saying the log
was cut short, where, and by how many bytes.

**Independent Test**: cut a recording inside its final record, read it, and confirm the records
before the cut arrive and the cut is reported — with US2 and US3 unbuilt.

### Tests for User Story 1 (write first, MUST fail) ⚠️

- [ ] T006 [P] [US1] Cut sweep for the binary codec in a new gatling/binary/truncation_test.go: for each of testdata/corpus/gatling/{3.13.1,3.14.9,3.15.1}/simulation.log, cut at every offset in the last 200 bytes, assert the records equal a whole-file read of the same prefix, the read ends with `*gatling.TruncationError`, and nothing panics
- [ ] T007 [P] [US1] The same sweep for the text codec in a new gatling/text/truncation_test.go over testdata/corpus/gatling/{3.11.5,3.12.0}/simulation.log
- [ ] T008 [P] [US1] Position and byte count in gatling/binary/truncation_test.go and gatling/text/truncation_test.go: `Offset + Dropped` equals the cut file's length for binary; `Line` names the unterminated final line and `Dropped` its length for text (data-model.md invariant 3)
- [ ] T009 [P] [US1] An intact recording still ends with `io.EOF`, reports no truncation, and decodes to its recorded golden stream unchanged — extend gatling/binary/golden_test.go and gatling/text/golden_test.go
- [ ] T010 [P] [US1] A cut inside the preamble or the run record fails the constructor with `*gatling.TruncationError` and returns no reader (FR-007, research R4) — in the two truncation_test.go files
- [ ] T011 [P] [US1] The same value reaches the model path unchanged through `NewRunReader` in both codecs (FR-012) — in the two truncation_test.go files

### Implementation for User Story 1

- [ ] T012 [US1] Track where the current record began: add the field to `reader` in gatling/binary/read.go and set it from `r.rd.off` in `Reader.Next` in gatling/binary/reader.go, once `atEnd` reports the stream is not exhausted
- [ ] T013 [US1] Build the new error in `reader.truncated` in gatling/binary/read.go — `Offset` the record's start, `Dropped` the stop offset minus that start, `Expected` the value being read (research R3)
- [ ] T014 [US1] Build the new error on the text path in gatling/text/reader.go: the unterminated-line case in `Next` and the same case in `NewReader`'s preamble loop, taking `Dropped` from the bytes `scanner.next` already returns and discards today
- [ ] T015 [P] [US1] Restate the doc comments that today declare a partial read "not a result": gatling/text/reader.go, gatling/binary/reader.go, gatling/text/model.go, gatling/binary/model.go and the `RecordReader`/`RunReader` interface docs in gatling/simlog/simlog.go — each now names the three endings from data-model.md
- [ ] T016 [US1] Add the CHANGELOG.md entry under Unreleased → Changed, covering the new type and the changed ending, per [contracts/public-api.md](contracts/public-api.md)

**Checkpoint**: a killed run reports what it recorded, on both formats and through both reader
shapes. #7's commit is green on its own.

---

## Phase 4: User Story 2 — A running test can be followed on a stated promise (Priority: P2)

**Goal**: the blocking-source guarantee is written where a consumer reads it, and held by a test that
fails if a later change removes it.

**Independent Test**: deliver a recording in pieces from a blocking source and compare with a
whole-file read — passes with Phases 2, 3 and 5 unbuilt.

**Note**: no read loop changes. Research R6 measured that this already works; these tasks turn an
accident into a contract.

### Tests for User Story 2 (write first) ⚠️

- [ ] T017 [P] [US2] Blocking-source test in a new gatling/simlog/follow_test.go: a source that blocks while it has no bytes, appends 300 at a time, and reports the end only when done; each of the five recordings yields the record stream a whole-file read yields (66 records for 3.11.5 and 3.12.0, 132 for 3.13.1, 3.14.9 and 3.15.1)
- [ ] T018 [P] [US2] In gatling/simlog/follow_test.go, place an append boundary inside a record and assert that record is delivered exactly once, neither lost nor duplicated (FR-008)
- [ ] T019 [P] [US2] In gatling/simlog/follow_test.go, assert a follower paused mid-log holds no more than its fixed buffers, measured the way gatling/binary/memory_test.go already measures (FR-011)

### Implementation for User Story 2

- [ ] T020 [US2] State the contract in gatling/simlog/doc.go: a blocking source is supported, a short read is a wait, `io.EOF` is the caller's statement that the log ended, a source failure is never an ending, and memory stays bounded while waiting — the five promises of [contracts/blocking-source.md](contracts/blocking-source.md)
- [ ] T021 [P] [US2] Point `text.NewReader` (gatling/text/reader.go), `binary.NewReader` (gatling/binary/reader.go) and `simlog.NewReader`/`NewRunReader` (gatling/simlog/simlog.go) at that contract, so a caller meets it where it starts a read

**Checkpoint**: #10's commit is green on its own, and comet#3 has a promise to build on.

---

## Phase 5: User Story 3 — A damaged log is still refused (Priority: P3)

**Goal**: salvage does not become guessing — corruption keeps failing, and a source failure stays a
source failure.

**Independent Test**: feed defects that are not truncation and confirm each still fails with its
position, and that none is reachable as a `*TruncationError`.

### Tests for User Story 3 (write first, MUST fail for T025) ⚠️

- [ ] T022 [P] [US3] Regression test in gatling/binary/read_test.go: a source failing with an error that *wraps* `io.EOF` is reported as that failure with its cause reachable through `errors.Is`, and never as a truncation — this fails on today's code (research R5)
- [ ] T023 [P] [US3] An undefined record kind, a length prefix past `MaxStringLen`, and an unparseable field each still end the read with `*gatling.SyntaxError` naming the position, and `errors.As` does not reach `*gatling.TruncationError` — extend gatling/binary/read_test.go and gatling/text/syntax_test.go
- [ ] T024 [P] [US3] A binary log cut exactly on a record boundary decodes cleanly and reports no truncation, because the format has no end marker — in gatling/binary/truncation_test.go, with the reason in the test's comment

### Implementation for User Story 3

- [ ] T025 [US3] Compare with identity rather than `errors.Is` in `sourceFailed` in gatling/binary/read.go, matching `atEnd`, `simlog.identify` and `simlog.readHead`, and carry the same `//nolint:errorlint` directive and stated reason those siblings carry

**Checkpoint**: all three stories independently green; the cut, the corruption and the source failure
are three distinguishable outcomes.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T026 [P] Seed `FuzzDecode` (gatling/binary/fuzz_test.go) and `FuzzReader` (gatling/text/mutation_test.go) with prefixes of the recordings, so the pull-request fuzz leg spends its budget on this feature's input class (research R8)
- [ ] T027 [P] godoc review: every new exported identifier documented, and both `Reader` doc comments name the three endings of a read (Principle V)
- [ ] T028 Coverage check: ≥ 90% for gatling/text, gatling/binary and gatling/simlog, ≥ 80% for the module (`go test -cover ./...`); the numbers go in the PR description
- [ ] T029 Re-run the T001 benchmarks and the peak-memory tests; record the figures next to the baseline in specs/008-incomplete-record/plan.md and justify any regression
- [ ] T030 [P] Verify model/ and gatling/ are still standard-library only (`go list -deps`) and that `go mod tidy` leaves the tree unchanged
- [ ] T031 Run [quickstart.md](quickstart.md) end to end, including the three "make it fail on purpose" steps
- [ ] T032 CHANGELOG.md complete under Unreleased in Keep a Changelog shape, covering both commits

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies
- **Foundational (Phase 2)**: after Setup — blocks US1 and US3 only
- **US1 (Phase 3)**: after Foundational
- **US2 (Phase 4)**: after Setup — **independent of Foundational, US1 and US3**
- **US3 (Phase 5)**: after Foundational; T024 also needs T006's file to exist
- **Polish (Phase 6)**: after the stories it covers

### Within Each Story

- Tests are written first and must fail before the implementation task they guard
- T012 before T013 (the offset must be tracked before the error can carry it)
- T005 before every test that names `TruncationError` compiles

### Parallel Opportunities

- T001 and T002 together
- T006, T007, T008, T009, T010, T011 together — different files, no shared state
- **US2 in parallel with everything else**: it touches gatling/simlog/ and doc comments only
- T022, T023, T024 together
- T026, T027, T030 together

## Parallel Example: User Story 1

```bash
# The sweep and its assertions, all at once:
Task: "Binary cut sweep in gatling/binary/truncation_test.go"
Task: "Text cut sweep in gatling/text/truncation_test.go"
Task: "Position and dropped-byte assertions in both truncation_test.go files"
Task: "Intact recordings unchanged in gatling/{binary,text}/golden_test.go"
Task: "Preamble cut fails the constructor in both truncation_test.go files"
Task: "The cut reaches the model path in both truncation_test.go files"
```

---

## Commit Mapping (AGENTS.md: 1 issue = 1 commit)

| Commit | Tasks | Green on its own |
|---|---|---|
| `docs(speckit): add 008-incomplete-record spec/plan/tasks` | the specs/ artifacts — lands **before** any code | n/a |
| `feat(gatling): hand back the records a cut-short log holds (#7)` | T004–T016, T022–T025, plus the polish tasks that cover them | `go build ./... && go test ./...` |
| `test(gatling): hold the blocking-source contract a follower reads through (#10)` | T017–T021 | `go build ./... && go test ./...` |

T001–T003 are measurements and checks, not changes. Nothing else lands in this PR: an improvement
found on the way that is not in scope goes to its own PR (AGENTS.md, *Never: opportunistic
refactors outside scope*).

Both issues belong to milestone **v0.0.8 An incomplete record**, and each is closed by the commit
that lands on `main`.

---

## Implementation Strategy

### MVP (User Story 1 only)

1. Phase 1 Setup → 2. Phase 2 Foundational → 3. Phase 3 US1 → **stop and validate**: a killed run
reports what it recorded, on both formats. That alone closes #7's headline failure and is worth
shipping.

### Incremental Delivery

1. Setup + Foundational → the type exists
2. US1 → the salvage works → #7's commit, minus its guard rails
3. US3 → corruption and source failures stay distinguishable → #7's commit complete
4. US2 → the follower's promise is written and held → #10's commit
5. Polish → coverage, benchmarks, fuzz seeds, CHANGELOG

US2 can be pulled forward at any point — it shares no file with the others.
