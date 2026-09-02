---

description: "Task list template for feature implementation"
---

# Tasks: [FEATURE NAME]

**Input**: Design documents from `/specs/[###-feature-name]/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Test tasks are REQUIRED (constitution Principle III, Golden-Corpus Testing). Every user story phase lists its corpus, equivalence and tolerance tests before its implementation tasks, and every bug fix ships a regression test. Tests are written first and MUST fail before the implementation task starts.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

parsec is a single Go module with packages at the repository root (see plan.md "Source Code"):

- **Packages**: `model/`, `gatling/`, `stats/`, one `<tool>/` package per adapter, `internal/` for shared non-public helpers
- **Tests**: `<pkg>/<file>_test.go` beside the code; table-driven on stdlib `testing`
- **Golden corpus**: `testdata/corpus/<tool>/<version>/`, recorded from real runs; a hand-edited fixture says so in its name
- Paths below are examples for a Gatling decoder story - adjust based on plan.md structure

<!--
  ============================================================================
  IMPORTANT: The tasks below are SAMPLE TASKS for illustration purposes only.

  The /speckit-tasks command MUST replace these with actual tasks based on:
  - User stories from spec.md (with their priorities P1, P2, P3...)
  - Feature requirements from plan.md
  - Entities from data-model.md
  - Endpoints from contracts/

  Tasks MUST be organized by user story so each story can be:
  - Implemented independently
  - Tested independently
  - Delivered as an MVP increment

  DO NOT keep these sample tasks in the generated tasks.md file.
  ============================================================================
-->

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Package skeleton and golden corpus for this feature

- [ ] T001 Create the package directories named in plan.md (e.g. gatling/, testdata/corpus/gatling/3.12.1/)
- [ ] T002 Record the golden corpus for every version in scope into testdata/corpus/<tool>/<version>/ with the tool's own report beside it
- [ ] T003 [P] Confirm .golangci.yml needs no change for this feature; if it does, justify it in plan.md Complexity Tracking

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

Examples of foundational tasks (adjust based on your project):

- [ ] T004 Define the model types and Capabilities every story depends on in model/<file>.go
- [ ] T005 [P] Implement the version gate (refuse below range, warn above) in <tool>/version.go
- [ ] T006 [P] Define error types that carry byte offset or line number in <tool>/errors.go
- [ ] T007 Add the io.Reader-based decoder skeleton with bounded buffers and allocation caps in <tool>/decoder.go
- [ ] T008 Add the benchmark harness over the largest corpus file in <tool>/decoder_bench_test.go

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - [Title] (Priority: P1) 🎯 MVP

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Tests for User Story 1 (REQUIRED - write first, MUST fail before implementation) ⚠️

- [ ] T009 [P] [US1] Golden test: decode testdata/corpus/<tool>/<version>/ and compare byte for byte with the recorded record stream in <tool>/decoder_test.go
- [ ] T010 [P] [US1] Equivalence test: chunked reads == whole-file read for every corpus file in <tool>/decoder_chunk_test.go
- [ ] T011 [P] [US1] Version-gate test: below range refused with both versions in the error, unknown newer decodes with a warning in <tool>/version_test.go
- [ ] T012 [US1] Tolerance test: statistics against the tool's own report within the documented tolerance in stats/<file>_test.go

### Implementation for User Story 1

- [ ] T013 [P] [US1] Add [Entity1] to model/<file>.go with doc comments
- [ ] T014 [P] [US1] Declare what this source cannot provide through Capabilities in <tool>/capabilities.go
- [ ] T015 [US1] Implement record decoding for [record kinds] in <tool>/decoder.go (depends on T013, T014)
- [ ] T016 [US1] Wire malformed-input errors with offsets and the allocation caps in <tool>/decoder.go
- [ ] T017 [US1] Doc comments for every new exported identifier; CHANGELOG.md entry under Unreleased

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently

---

## Phase 4: User Story 2 - [Title] (Priority: P2)

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Tests for User Story 2 (REQUIRED - write first, MUST fail before implementation) ⚠️

- [ ] T018 [P] [US2] Golden test for [version or format] in <tool>/<file>_test.go
- [ ] T019 [P] [US2] Equivalence or tolerance test for [scenario] in <pkg>/<file>_test.go

### Implementation for User Story 2

- [ ] T020 [P] [US2] Add [Entity] to model/<file>.go
- [ ] T021 [US2] Implement [codec or aggregation] in <pkg>/<file>.go
- [ ] T022 [US2] Extend Capabilities or the version gate for [scope] in <tool>/<file>.go
- [ ] T023 [US2] Integrate with User Story 1 components (if needed)

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - [Title] (Priority: P3)

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Tests for User Story 3 (REQUIRED - write first, MUST fail before implementation) ⚠️

- [ ] T024 [P] [US3] Golden test for [version or format] in <tool>/<file>_test.go
- [ ] T025 [P] [US3] Regression or tolerance test for [scenario] in <pkg>/<file>_test.go

### Implementation for User Story 3

- [ ] T026 [P] [US3] Add [Entity] to model/<file>.go
- [ ] T027 [US3] Implement [codec or aggregation] in <pkg>/<file>.go
- [ ] T028 [US3] Extend Capabilities or the version gate for [scope] in <tool>/<file>.go

**Checkpoint**: All user stories should now be independently functional

---

[Add more user story phases as needed, following the same pattern]

---

## Phase N: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] TXXX [P] CHANGELOG.md entries complete under Unreleased (Keep a Changelog)
- [ ] TXXX [P] godoc review: every exported identifier documented; decoders state the versions they accept
- [ ] TXXX Coverage check: ≥ 90% decoder packages, ≥ 80% overall (go test -cover ./...); numbers go in the PR description
- [ ] TXXX Benchmark on the largest corpus file; record throughput and peak memory in plan.md
- [ ] TXXX Verify model/ and gatling/ are still stdlib-only (go list -deps) and go mod tidy is clean
- [ ] TXXX Run quickstart.md validation

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P3)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - May integrate with US1 but should be independently testable
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - May integrate with US1/US2 but should be independently testable

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Model types before decoders
- Decoders before statistics
- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- Once Foundational phase completes, all user stories can start in parallel (if team capacity allows)
- All tests for a user story marked [P] can run in parallel
- Model types within a story marked [P] can run in parallel
- Different user stories can be worked on in parallel by different team members

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together:
Task: "Golden test for testdata/corpus/gatling/3.12.1/ in gatling/decoder_test.go"
Task: "Chunked vs whole-file equivalence test in gatling/decoder_chunk_test.go"
Task: "Version-gate test in gatling/version_test.go"

# Launch all model work for User Story 1 together:
Task: "Add [Entity1] to model/sample.go"
Task: "Declare Capabilities in gatling/capabilities.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 3 → Test independently → Deploy/Demo
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1
   - Developer B: User Story 2
   - Developer C: User Story 3
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- One tracked issue = one semantic commit, green on its own (go build ./... && go test ./...)
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
