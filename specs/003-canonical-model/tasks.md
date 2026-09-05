---

description: "Task list for the canonical result model and the probe's requirements document"
---

# Tasks: A Canonical Model for Load-Test Results, and Requirements Stated Once

**Input**: Design documents from `/specs/003-canonical-model/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/model.md](contracts/model.md), [contracts/gatling-text.md](contracts/gatling-text.md), [contracts/nfr.yaml](contracts/nfr.yaml), [quickstart.md](quickstart.md)

**Tests**: REQUIRED (constitution Principle III). Every story lists its tests before its implementation, tests are written first and MUST fail before the implementation task starts. In Go a test file that does not compile is a failing test, which is how the first test of each phase fails.

**Organization**: grouped by user story. Two tracked issues mean **two** semantic commits — `feat(model): …(#4)` for US1–US3 and `feat(corpus): …(#30)` for US4 — each green on its own. The tasks are the working order, not the commit history; squash churn before review.

**Before T001**: commit and merge the spec PR — `docs(speckit): add 003-canonical-model spec/plan/tasks`, milestone v0.0.3 — on its own, before any `feat` commit. It merges on review alone because CI ignores `specs/`.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: US1–US4 from [spec.md](spec.md)
- Every task names the exact file it touches

## Path Conventions

Per [plan.md](plan.md) "Source Code":

- **The canonical types**: `model/` — standard library only, enforced by the `deps` job
- **The conversion**: `gatling/text/` — standard library only, same job
- **Tests**: `<pkg>/<file>_test.go` beside the code, table-driven on stdlib `testing`
- **Integration suite**: build tag `integration`; it MUST fail, not skip, when the corpus is absent
- **The probe**: `testdata/corpus/gatling/simulation/` — a separate sbt build; nothing it depends on reaches `go.mod`
- **Golden corpus**: `testdata/corpus/gatling/{3.11.5,3.12.0}/` — **read only in this feature**

**No corpus is recorded in this feature, and this is deliberate.** The two entries already exist and were captured with the reports their own Gatling generated, which is the half that cannot be recreated. Principle III's capture-at-recording-time rule has already been met for them. Re-recording them to carry a newly rendered assertion block would discard evidence to gain nothing — the payload is carried through unread. See [research.md](research.md) R10.

---

## Phase 1: Setup

**Purpose**: the new package's skeleton, and the two tracked-issue corrections this feature's decisions oblige

- [X] T001 Create `model/` with `model/doc.go`: what the package is (the canonical result types every source is decoded into, and what three downstream builds import) and what it is not (it computes nothing; statistics are v0.0.7 and v0.0.8; it holds no tool-specific type)
- [X] T002 [P] Confirm `.golangci.yml` needs no change for this feature; if it does, add the justification to [plan.md](plan.md) Complexity Tracking in the same PR
- [X] T003 [P] Correct issue [#4](https://github.com/galax-io/parsec/issues/4): remove `Aggregate` from the type list and the "MUST accept pre-aggregated sources" requirement, and point both at milestone v0.5.0 with the Principle VI reason. Required before the `feat(model)` PR merges, or the tracked requirements and the shipped model disagree on the record. **Then correct issue [#20](https://github.com/galax-io/parsec/issues/20) in the same pass**: it says "the reader MUST populate the aggregate form", which after this deferral does not exist — v0.5.0 must now define that form as well as read into it, or the deferred work has no home and falls through the gap between the two issues
- [X] T004 [P] Correct issue [#30](https://github.com/galax-io/parsec/issues/30): **drop** its second requirement — the translation applies the recorded-name rule "so an author never has to know it". OpenNFR defines a `loadtest.group.name` element as "a literal recorded name", so an author does have to know it, by the format's own design; the document carries the recorded spelling and explains it beside the requirement (FR-029). Nothing is filed upstream against this: there is no missing feature. Optionally suggest to `galax-io/gatling-picatinny` that its Gatling renderer *warn* when a group element contains a comma, since Gatling replaces one on write and such an element can therefore never match — a diagnostic, not a substitution

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the canonical types themselves. Every story depends on them.

**⚠️ CRITICAL**: no user story work can begin until this phase is complete

`gatling/text/capability.go` (T012) sits here rather than in User Story 2 because both P1 stories need it: User Story 1 puts the value on every `Run` it produces, and User Story 2 proves it is honest. The spec says as much — User Story 2 "ships with User Story 1 or the model is dishonest from its first release."

### Tests (write first — MUST fail)

- [X] T005 [P] Write `model/opt_test.go`: the zero `Opt[T]` is unset; `Some(v).Get()` returns `v, true`; an `Opt[int]` holding `0` is distinguishable from an unset one; `Or` returns the fallback only when unset. MUST fail — the package does not exist yet
- [X] T006 [P] Write `model/capability_test.go`: `NewCapabilities(a, b).Provides(a)` is true and `Provides(c)` is false; a `Field` no constructor was given reads as absent, which is the conservative direction ([research.md](research.md) R4); `Absent()` returns a stable order and never includes `FieldUnknown`

### Implementation

- [X] T007 [P] Implement `model/opt.go`: `Opt[T]`, `Some`, `Get`, `Or`, `IsSet`, per [contracts/model.md](contracts/model.md). A value type, not a pointer — an allocation per optional field per sample is what SC-004's ceiling cannot afford
- [X] T008 [P] Implement `model/capability.go`: the `Field` constants of [data-model.md](data-model.md) with `String()`, and `Capabilities` storing what the source **provides**, with `NewCapabilities`, `Provides` and `Absent`. The doc comment states why the set is what is provided rather than what is absent
- [X] T009 [P] Implement `model/sample.go`: `Outcome` with its three constants and `String()`, `Failure`, `Sample`, `GroupSample`, `UserEvent`, `UserEventKind`, `RunError` — every field with the doc comment [contracts/model.md](contracts/model.md) gives it, including the `Groups` ownership rule ("valid until the next call to Next; copy to keep"), stated identically to `gatling.Record.Groups` so there is one rule and not two
- [X] T010 Implement `model/run.go`: `Warning`, `Run`, `ItemKind` with `String()`, and `Item`. Each `Item` field's doc comment names the `Kind` that selects it and states that the others hold their zero value. Depends on T007–T009
- [X] T011 [P] Write `model/item_test.go`: reading a field whose `Kind` does not select it yields that field's zero value; every enum's `String()` covers every constant including the `Unknown` zero
- [X] T012 Implement `gatling/text/capability.go`: `Capabilities() model.Capabilities` declaring what a Gatling text `simulation.log` provides — request duration, group cumulated duration, group outcome — and, by omission, everything of [research.md](research.md) R7 that it does not. The doc comment names the absent set explicitly so `go doc` answers the question without running anything

**Checkpoint**: the types exist and are tested. User stories can begin.

---

## Phase 3: User Story 1 — A run can be read without knowing which tool produced it (Priority: P1) 🎯 MVP

**Goal**: the decoder that shipped in v0.0.2 becomes consumable by something other than itself. A consumer reads a complete run — requests, groups, virtual users, run identity, run-level errors — through `model` alone.

**Independent Test**: decode a recorded run into the canonical types and produce its request total and OK/KO split from those types alone, in code importing no tool package; the counts equal what that run's own Gatling report states.

### Tests for User Story 1 (write first — MUST fail)

- [X] T013 [P] [US1] Write `gatling/text/model_test.go`: one table case per wire record kind, over the fixtures in `gatling/text/testdata/fixtures/` — `REQUEST` → `ItemSample` with its group path, name, start, duration and outcome; `GROUP` → `ItemGroup` with its cumulated duration and its own outcome; `USER` → `ItemUser`; `ERROR` → `ItemError`; `RUN` → `Run`'s identity fields; `ASSERTION` → `Run.Assertions` verbatim. Assert one item per event record, in source order
- [X] T014 [P] [US1] Write `gatling/text/model_time_test.go`: epoch milliseconds convert to UTC without loss and without re-basing against the run start (FR-012); an end *before* the start and the sentinel Gatling's own reader branches on both yield `Duration` unset, as does a span too large to be a `time.Duration`; an end equal to the start is a recorded zero (FR-020)
- [X] T015 [P] [US1] Write `gatling/text/model_golden_test.go` behind `//go:build integration`: for each `testdata/corpus/gatling/*/`, fold the item stream into totals and compare against that run's `global_stats.json` and `stats.json` — the request total with its OK/KO split, and the same three numbers per request name and per group, exactly; and against the counts the existing wire-record path produces from the same log. FAILS, not skips, when no corpus directory is found
- [X] T016 [P] [US1] Write `gatling/text/model_chunk_test.go`: reading each corpus log in one pass and in chunks split at arbitrary byte boundaries yields identical item streams, and identical failures where the input fails
- [X] T017 [P] [US1] Write `gatling/text/model_memory_test.go` and a `BenchmarkRunReader` in `gatling/text/bench_test.go`: peak memory under 32 MiB on a generated 1 GB log, unchanged when the log is made ten times larger (SC-004); `B/op` recorded against the decoder's own figure
- [X] T018 [P] [US1] Write `model/example_test.go`: a runnable example that opens a run, folds its item stream into a total and an OK/KO split, and prints them. It imports `model` and nothing tool-shaped, which is what SC-001 asserts — the compiler is the assertion

### Implementation for User Story 1

- [X] T019 [US1] Implement `gatling/text/model.go`: `RunReader`, `NewRunReader`, `Run()` and `Next()` per [contracts/gatling-text.md](contracts/gatling-text.md), wrapping the existing `Reader` rather than re-parsing. `Run()` is complete when `NewRunReader` returns and does not change as items are read; the version gate is not re-implemented and a gate warning is carried onto `Run.Warnings` (FR-016a)
- [X] T020 [US1] Implement the record-to-item mapping in `gatling/text/model.go` per the table in [contracts/gatling-text.md](contracts/gatling-text.md), including time conversion and the sentinel rule of T014. An exception-backed failure yields **both** a failed sample and a separate `ItemError`, because Gatling writes both and neither is derived from the other (FR-021)
- [X] T021 [US1] Confirm nothing in the conversion buffers: no slice grows with the run, and `Groups` reuses the decoder's backing array under the ownership rule T009 documents. T016 and T017 are what prove it
- [X] T022 [US1] Doc comment every new exported identifier in `gatling/text/`, naming the Gatling versions accepted (Principle V); add the `CHANGELOG.md` entry under Unreleased → Added

**Checkpoint**: a run is readable through `model` alone, and its counts equal the tool's own report.

---

## Phase 4: User Story 2 — What the source could not measure is declared, never filled in (Priority: P1)

**Goal**: a consumer learns what a Gatling text log can never record *before* rendering anything, and never receives a zero standing in for a measurement.

**Independent Test**: ask a run's capabilities what the source provides and confirm the answer names every absent field; confirm no item of any corpus run carries a set value for one of them.

### Tests for User Story 2 (write first — MUST fail)

- [X] T023 [P] [US2] Write `gatling/text/capability_test.go`: `Capabilities().Absent()` names every field of [research.md](research.md) R7 — the sample's scenario, response code, bytes sent and received, failure type and user identity; the group's wall-clock duration; connect, DNS and TLS timings; the requirements the assertion payload encodes; per-interval series — and names nothing the format does record
- [X] T024 [P] [US2] Add to `gatling/text/model_golden_test.go` (integration): over both corpus runs, for every field the run declares absent, assert every item leaves it unset — a substituted zero, empty string or average anywhere fails (SC-005). Table-driven over `Absent()`, so a field added later is covered without editing the test
- [X] T025 [P] [US2] Write `model/capability_example_test.go`: a runnable example that prints what a source does not record, before any item is read — the shape a report uses to decide whether to render a column at all

### Implementation for User Story 2

- [X] T026 [US2] Populate `Run.Capabilities` from `text.Capabilities()` in `gatling/text/model.go`, and ensure the mapping in T020 leaves every absent field unset rather than assigning a zero — the invariant T024 checks
- [X] T027 [US2] Extend the package doc comment in `model/doc.go` with how absence is read: `Capabilities` for what the source never records, `Opt[T]` for what this record does not carry, and why neither implies the other

**Checkpoint**: absence is declared, reachable before rendering, and provably never filled in.

---

## Phase 5: User Story 3 — A failure can never be counted as a success (Priority: P2)

**Goal**: the success/failure distinction survives every step between the log and the report, and no entry point offers a statistic in which the two have been pooled.

**Independent Test**: select the successful samples of a run, then add every failed sample back into the input and select again; the two results are the same multiset.

### Tests for User Story 3 (write first — MUST fail)

- [X] T028 [P] [US3] Write `model/outcome_test.go`: `Failure` is set if and only if `Outcome == OutcomeFailure`, checked in both directions; a successful sample carries no failure at all (FR-009, and the presence is what OpenNFR's `{error.type: "*"}` numerator tests)
- [X] T029 [P] [US3] Write `model/selection_test.go`: selecting `OutcomeSuccess` samples returns an identical multiset whatever failures the input contains — over both corpus runs and over generated runs mixing the two in any proportion, including all-success and all-failure (SC-003)
- [X] T030 [P] [US3] Add to `gatling/text/model_test.go`: a group whose own status is `KO` while every request inside it succeeded yields `ItemGroup` with `OutcomeFailure` and samples with `OutcomeSuccess` — the group's outcome is its own, not the conjunction (FR-003)

### Implementation for User Story 3

- [X] T031 [US3] Ensure the mapping in `gatling/text/model.go` sets `Outcome` from the recorded status alone and never infers it from whether a message is present, and that a group's outcome is read from its own record (FR-002, FR-003)
- [X] T032 [US3] Add the invariant to the `Sample` and `GroupSample` doc comments in `model/sample.go`, and state in `model/doc.go` that this package exposes no pooled statistic — the reason no entry point can return one is that it returns no statistic at all

**Checkpoint**: the distinction is structural, not conventional, and a test proves it cannot be lost.

---

## Phase 6: User Story 4 — The probe's expectations are stated once (Priority: P2)

**Goal**: the corpus probe's expectations live in one OpenNFR document, rendered into Gatling assertions by the library that already does that translation. Changing an expectation is one edit and no Scala.

**Independent Test**: change a threshold in the document alone, run the probe under each supported version, confirm each run is held to the new number; then make a requirement false and confirm the run fails naming it.

**Separate commit**: `feat(corpus): …(#30)`. It touches no Go file and shares nothing with US1–US3.

**Already verified in Phase 0.** [research.md](research.md) R1 ran every check below under Gatling 3.11.5 and 3.12.0 against exactly the document in [contracts/nfr.yaml](contracts/nfr.yaml). These tasks land that result in the repository; they are not a fresh investigation.

### Tests for User Story 4 (the run itself is the test)

- [X] T033 [P] [US4] Extend `testdata/corpus/gatling/simulation/README.md`: how an expectation is changed now (edit `src/test/resources/nfr.yaml`, run, done — no Scala), what the two-space group spelling means, and the exact commands for both supported versions
- [X] T034 [P] [US4] Add a schema-validation step for `testdata/corpus/gatling/simulation/src/test/resources/nfr.yaml` to `.github/workflows/verify.yml`, against the JSON Schema `galax-io/opennfr` publishes, with the validator pinned. An unknown field must be rejected naming the field — the schema's `additionalProperties: false` does that work (FR-026). Decide vendored-vs-fetched here and record the choice in the workflow comment ([research.md](research.md) R9)

### Implementation for User Story 4

- [X] T035 [US4] Add `"org.galaxio" %% "gatling-picatinny" % "1.27.0" % Test` to `testdata/corpus/gatling/simulation/project/Dependencies.scala`, pinned exactly — the OpenNFR surface is documented upstream as experimental and outside that library's binary-compatibility guarantee, so a floating version is a broken build waiting to happen
- [X] T036 [US4] Add `testdata/corpus/gatling/simulation/src/test/resources/nfr.yaml`, copied from [contracts/nfr.yaml](contracts/nfr.yaml) with its comments intact — they carry why the group name has two spaces and why "18 successful" is written as "36 total, 18 failed"
- [X] T037 [US4] Replace the nine-line `.assertions(...)` block in `testdata/corpus/gatling/simulation/src/test/scala/io/galaxio/parsec/corpus/CorpusSimulation.scala` with `.assertions(OpenNfrAssertions.fromYaml("src/test/resources/nfr.yaml"))` and the matching import. The scenario, the injection profile and the stub are untouched: what the probe *does* is unchanged, only where its expectations are written
- [X] T038 [US4] Run `testdata/corpus/gatling/simulation` under 3.11.5 and under 3.12.0 against `testdata/corpus/gatling/simulation/stub` and confirm the nine assertions of [quickstart.md](quickstart.md) §6, with the same verdicts and the same 36/18/18 — the numbers the hand-written block asserted
- [X] T039 [US4] Confirm the two negative behaviours by editing `testdata/corpus/gatling/simulation/src/test/resources/nfr.yaml` and record them in the PR: a threshold changed from 18 to 17 fails the run naming the requirement (FR-027, SC-008); a document carrying a `good` numerator or `aggregation: sum` produces **no** assertions and lists both reasons (FR-024, SC-009). Restore the document afterwards
- [X] T040 [US4] Confirm `go.mod` is unchanged and `go list -deps ./model/... ./gatling/...` still reaches nothing outside the standard library — the new dependency lives in a separate sbt build under `testdata/` and must not have leaked

**Checkpoint**: the probe is held to a document that names no tool, under every supported version.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T041 [P] Update `doc.go` at the repository root: `model` is no longer "(planned)" — state what it holds and that it is what consumers build on
- [X] T042 [P] Update `README.md`: the packages block (`model/` ships), and the Status paragraph, which currently says "the canonical model that every source converts into is v0.0.3"
- [X] T043 [P] `CHANGELOG.md` under Unreleased → Added: the `model` package and the `gatling/text` conversion; note that the wire records stay exported and are documented as the log's own events
- [X] T044 [P] godoc review: every exported identifier in `model/` and every new one in `gatling/text/` has a doc comment; the conversion states the Gatling versions it accepts (Principle V)
- [X] T045 Coverage: `go test -cover ./...` — ≥ 90% for `gatling/text` and `model`, ≥ 80% overall; put the numbers in the PR description (Principle III, until CI enforces it)
- [X] T046 Run the full gate set of [quickstart.md](quickstart.md) §1 plus `go test -tags=integration ./...`, and the canary with `PARSEC_CANARY_RUNS` set for both versions
- [ ] T047 Assign both PRs to milestone v0.0.3 and close issues #4 and #30 when they land on `main` — no milestone, no merge (constitution, Development Workflow)

---

## Dependencies & Execution Order

### Phase dependencies

- **Setup (Phase 1)**: no dependencies. T003 and T004 are issue edits and can happen any time before the respective PR merges
- **Foundational (Phase 2)**: depends on T001. **Blocks every user story**
- **US1 (Phase 3)**: depends on Foundational
- **US2 (Phase 4)**: depends on Foundational and on T019–T020 of US1, because T026 edits the mapping US1 writes. Both are P1 and ship in the same commit; the spec says US2 "ships with User Story 1 or the model is dishonest from its first release"
- **US3 (Phase 5)**: depends on Foundational; T030 and T031 touch files US1 creates, so it follows US1
- **US4 (Phase 6)**: depends on **nothing** in this feature. It touches no Go file and can be done first, last, or by someone else entirely
- **Polish (Phase 7)**: depends on everything else

### Within each story

- Tests are written and MUST fail before the implementation task starts
- Model types before the conversion; the conversion before anything that folds its output
- One tracked issue = one green commit (`go build ./... && go test ./...`)

### Parallel opportunities

- T002, T003, T004 in Setup
- T005 and T006 (tests) together; then T007, T008, T009 and T011 together — four different files
- T013–T018: all six US1 tests, six different files
- T023 and T025 in US2; T028, T029 and T030 in US3
- T033 and T034 in US4
- T041–T044 in Polish
- **US4 in parallel with everything**: it is a Scala and YAML change in a separate build

---

## Parallel Example: User Story 1

```bash
# All six tests for User Story 1, six different files:
Task: "Per-record-kind mapping in gatling/text/model_test.go"
Task: "Times, sentinel and negative durations in gatling/text/model_time_test.go"
Task: "Counts vs each run's own report in gatling/text/model_golden_test.go"
Task: "Chunked vs whole-file item streams in gatling/text/model_chunk_test.go"
Task: "Peak memory and benchmark in gatling/text/model_memory_test.go"
Task: "A consumer importing no tool package in model/example_test.go"
```

```bash
# Foundational types, four different files:
Task: "Opt[T] in model/opt.go"
Task: "Field and Capabilities in model/capability.go"
Task: "Sample, GroupSample, UserEvent, RunError in model/sample.go"
Task: "Item zero-field rule in model/item_test.go"
```

---

## Implementation Strategy

### MVP (User Story 1 only)

1. Phase 1 Setup → Phase 2 Foundational → Phase 3 US1
2. **STOP and validate**: a recorded run reads through `model` alone and its counts equal the tool's own report
3. That is the milestone's reason for existing; everything after it makes the same result honest

### Incremental delivery

1. Foundational → types exist and are tested
2. + US1 → the model is usable (MVP)
3. + US2 → and honest about what it cannot measure
4. + US3 → and structurally unable to pool a failure into a success
5. + US4 → and the probe states its expectations once, in a form no tool owns

US1–US3 are one commit against issue #4; US4 is one commit against issue #30.

### Parallel team strategy

US4 shares no file with the rest and can start immediately, in parallel with Foundational. Within the Go work, US1 is the critical path: US2 edits its mapping and US3 asserts invariants over its output.

---

## Notes

- `[P]` = different files, no dependency on an incomplete task
- Verify each test fails before implementing what it covers; in Go a test file that does not compile is a failing test
- Two tracked issues, two commits, each green on its own
- Neither corpus entry is re-recorded, and no new one is added — see the Path Conventions note
- Issue corrections T003 and T004 are not optional tidying: without them the tracked requirements and the shipped code disagree on the record
