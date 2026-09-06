---
description: "Task list for 005-gatling-binary-decoder"
---

# Tasks: Reading the Binary simulation.log Gatling Writes From 3.13.0

**Input**: Design documents from `/specs/005-gatling-binary-decoder/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/gatling-binary.md](./contracts/gatling-binary.md),
[quickstart.md](./quickstart.md)

**Milestone**: [v0.0.5](https://github.com/galax-io/parsec/milestone/5) · **Issue**:
[parsec#6](https://github.com/galax-io/parsec/issues/6)

**Tests**: REQUIRED (constitution Principle III). Every story phase lists its corpus, equivalence and
tolerance tests before its implementation tasks. Tests are written first and MUST fail before the
implementation task starts.

**Organization**: grouped by user story. The three P1 stories are not independent of each other by
accident — the spec makes all three P1 because US1 is not shippable without them. Phase 2 builds the
format's two mechanisms (primitives, strings and the table); US1 turns them into records; **US2 and
US3 are the guarantees over those mechanisms**, each adding its own refusals and each independently
testable against its own recording.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependencies)
- **[Story]**: US1…US6, mapping to the user stories in spec.md

## Path Conventions

Single Go module, packages at the repository root. Tests live beside the code as `<file>_test.go`,
table-driven on stdlib `testing`. Golden corpus under `testdata/corpus/gatling/<version>/`.

---

## Phase 1: Setup — the probe and the recordings (IRREVERSIBLE, do first)

**Purpose**: produce the corpus. Nothing else in this feature can be verified without it, and none of
it can be recovered from a run that has already finished (FR-029, research [R8](./research.md)).

**⚠️ Order matters**: every probe change (T001–T004) must land *before* any recording is made. A run
records what the simulation did; it cannot be amended afterwards.

- [X] T001 Split the probe's assertion flavours by log format in `testdata/corpus/gatling/simulation/project/Dependencies.scala` and `build.sbt`: picatinny renders `nfr.yaml` for the text versions (up to 3.12.x), and a `scala-plain` source directory states the same expectations in Gatling's own DSL for the binary ones. The split is the format, not the tool version — an OpenNFR `loadtest.group.name` is the literal recorded name, and only one format substitutes the comma (research R8)
- [X] T002 Write the plain-DSL assertions in `testdata/corpus/gatling/simulation/src/test/scala-plain/io/galaxio/parsec/corpus/CorpusAssertions.scala`, each naming the `nfr.yaml` requirement it mirrors, and move the picatinny call into `src/test/scala-opennfr/.../CorpusAssertions.scala` so `CorpusSimulation.scala` carries no number
- [X] T003 Add a request named in Cyrillic with at least one character outside Latin-1 to `CorpusSimulation.scala`, so coder 1 is exercised at all (FR-030)
- [X] T004 Add a name repeated far more often than it is introduced, and a group name repeated across users, to `CorpusSimulation.scala`, so the string table is exercised as a table (FR-030)
- [X] T004a Flatten `testdata/corpus/gatling/simulation/src/test/resources/logback.xml`: the logback Gatling ships from 3.13.0 raises `EmptyStackException` on the scaffolded template's nested `<if>/<then>/<else>` and the run dies at startup. Every branch was already false for this probe
- [X] T005 Record Gatling 3.13.1 into `testdata/corpus/gatling/3.13.1/`: `simulation.log`, the whole generated HTML report, the redirected console summary, and the `global_stats.json`/`stats.json` this line still writes — committed exactly as written (FR-028, FR-029, FR-029a)
- [X] T006 Record Gatling 3.14.9 into `testdata/corpus/gatling/3.14.9/`, same artefacts, same simulation
- [X] T007 Record Gatling 3.15.1 into `testdata/corpus/gatling/3.15.1/`, same artefacts, same simulation
- [X] T008 [P] Write `testdata/corpus/gatling/3.13.1/RECORDING.md` following the shape of `testdata/corpus/gatling/3.12.0/RECORDING.md`, stating what the run exercised, what was checked at capture time, and why 3.13.0 is excluded
- [X] T009 [P] Write `testdata/corpus/gatling/3.14.9/RECORDING.md`, recording that Gatling writes no `global_stats.json` or `stats.json` from 3.14.0 — the Principle III exemption, and that the absence is Gatling's
- [X] T010 [P] Write `testdata/corpus/gatling/3.15.1/RECORDING.md`
- [X] T010a [P] Rewrite `testdata/corpus/gatling/simulation/README.md`: what to keep differs by version, the console summary must be redirected at run time to exist at all, and the two assertion flavours are chosen by log format
- [ ] T011 [P] Confirm `.golangci.yml` needs no change for this feature; if it does, justify it in plan.md Complexity Tracking

**Checkpoint**: three complete recordings exist, each with every account of its own numbers Gatling produced. **Done** — `testdata/corpus/gatling/{3.13.1,3.14.9,3.15.1}/`, each parsing to its last byte with identical record counts (1 RUN, 12 USER, 102 REQUEST, 12 GROUP, 6 ERROR) and each report stating 102 requests, 84 OK, 18 KO.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the two mechanisms every record uses, and the two shared changes the contract flags.

**⚠️ CRITICAL**: no user story work can begin until this phase is complete.

- [X] T012 Obtain approval for the two ask-first items in [contracts/gatling-binary.md §5](./contracts/gatling-binary.md) before writing T013 or T014 — `gatling.SyntaxError` gaining a field, and the record-to-model conversion moving to `internal/`. `AGENTS.md` lists a public API change as ask-first — **both approved 2026-09-06**
- [ ] T013 Add `Offset int64` to `SyntaxError` in `gatling/errors.go` with the doc comments from contract §2, and render `gatling: byte N: expected …` when it is set; extend `gatling/errors_test.go` for both renderings
- [ ] T014 Create `internal/wire/wire.go` holding `Item(*model.Item, *gatling.Record) bool`, moved verbatim from the unexported `convert` in `gatling/text/model.go`; change the text call site; `gatling/text`'s tests MUST pass unedited (SC-013)
- [ ] T015 [P] Create `gatling/binary/doc.go` with the package doc from contract §1, stating the versions accepted and that the stream must begin at byte 0
- [ ] T016 Implement the sized-primitive reader in `gatling/binary/read.go`: big-endian `int32`/`int64`, `bool` refusing any byte but 0 and 1, bounded `blob`, a running byte offset for every error, `MaxStringLen`, and a cap checked before any allocation (FR-002, FR-025, data-model §1)
- [ ] T017 [P] Unit-test every primitive over well-formed and malformed input in `gatling/binary/read_test.go`: negative length, length past the cap, length past end of file, a bool that is neither 0 nor 1, truncation mid-value — each asserting a `*gatling.SyntaxError` at the right offset
- [ ] T018 Implement string decoding and the string table in `gatling/binary/strings.go`: `int32 n`, the empty-string special case that carries no coder byte, `n` bytes, the coder byte, and the cached form with positive-introduces / negative-refers semantics indexed from 1 (data-model §1–§2)
- [ ] T019 Add the benchmark harness over the largest corpus log in `gatling/binary/bench_test.go`, reporting ns/op, B/op and allocs/op, shaped like `gatling/text/bench_test.go`

**Checkpoint**: primitives, strings and the table exist; user story work can begin.

---

## Phase 3: User Story 1 — The format every current user produces becomes readable (P1) 🎯 MVP

**Goal**: a recorded 3.15.1 run decodes end to end into the same wire records and the same canonical
model the text codec produces, with the run header available before the first event record.

**Independent Test**: decode one complete recording and compare its counts against what that run's
own Gatling reported for itself; the test passes only on exact equality.

### Tests for User Story 1 (write first, MUST fail) ⚠️

- [ ] T020 [P] [US1] Golden test in `gatling/binary/golden_test.go`: decode each recording and compare byte for byte against `records.golden` beside it (SC-001)
- [ ] T021 [P] [US1] Extract the run total and the per-request rows from the recorded HTML report, and the Global Information block from the recorded console summary, in `gatling/binary/report_test.go` — extraction happens in the suite, not at recording time (FR-029a). **Both are two-shaped**: from 3.14.0 the report bakes the figures into the markup and the console block is a `|`-separated table; on 3.13.x the report fills the table in with JavaScript at page load and the console reads `102 (OK=84 KO=18)`. 3.13.1 also carries `js/global_stats.json` and `js/stats.json`, which are machine-readable and should be preferred where present (research R8)
- [ ] T022 [US1] Tolerance test in `gatling/binary/tolerance_test.go`: counts folded from the decoded records equal what that run's Gatling reported, tolerance zero; the test computes them, the library does not (SC-002, Principle I)
- [ ] T023 [P] [US1] Header test in `gatling/binary/reader_test.go`: simulation class, run identifier, start, description and version are all available before the first `Next` (FR-008)
- [ ] T024 [P] [US1] Group-path test in `gatling/binary/record_test.go`: every request and group carries its full ordered enclosing path; a record outside any group carries an empty one (FR-006)
- [ ] T025 [P] [US1] Assertion test in `gatling/binary/reader_test.go`: each payload is delivered exactly as written, nothing decoded or validated (FR-007)
- [ ] T026 [P] [US1] Cross-version test in `gatling/binary/crossversion_test.go`: the three recordings of the same simulation produce identical record multisets once timing, identity, order **and message text** are set aside (SC-004, FR-032). Message text must be excluded because Gatling reworded the check failure between 3.13.1 and 3.14.0 — `status.find.is(200), but actually found 500` became `status.find.is(200), found 500` — which is the entire 13-byte size difference between the recordings
- [ ] T027 [P] [US1] Model-equivalence test in `gatling/binary/model_test.go`: `RunReader` over a binary recording yields the same `model` values the text codec produces for an equivalent run (FR-020, SC-011)

### Implementation for User Story 1

- [ ] T028 [US1] Implement the five record grammars in `gatling/binary/record.go` — Run, Request, User, Group, Error, exactly as data-model §1 tabulates them — refusing a second Run record and an unknown kind byte
- [ ] T029 [US1] Resolve a user record's scenario index to its scenario name from the run record's plain-string list in `gatling/binary/record.go`, so no index reaches `gatling.Record` or `model` (data-model §6)
- [ ] T030 [US1] Resolve every `int32` millisecond offset against the run's `int64` start in `gatling/binary/record.go`, reporting an instant before the run's start as **absent** rather than wrapped (FR-009, data-model §4)
- [ ] T031 [US1] Implement `Reader`, `NewReader`, `Header`, `Assertions`, `Warnings` and `Next` in `gatling/binary/reader.go` per contract §1, with the reused group scratch slice and the terminal error that every later call returns
- [ ] T032 [P] [US1] Implement `Capabilities`, `Tool` and `SupportedVersions` in `gatling/binary/capability.go`, with the doc comments from contract §1
- [ ] T033 [US1] Implement `RunReader`, `NewRunReader`, `Run` and `Next` in `gatling/binary/model.go` over `internal/wire` (depends on T014, T031)
- [ ] T034 [US1] Record `records.golden` for each recording under `testdata/corpus/gatling/<version>/`, generated from the decoder and reviewed against the run's own report before it is committed

**Checkpoint**: a current Gatling run is readable. This is the milestone's MVP.

---

## Phase 4: User Story 2 — A name survives whatever alphabet it was written in (P1)

**Goal**: the coder byte is honoured, both encodings decode correctly, and an unknown coder is
refused rather than guessed.

**Independent Test**: decode the Cyrillic request name from the recording and compare it byte for
byte against the name the simulation declared.

### Tests for User Story 2 (write first, MUST fail) ⚠️

- [ ] T035 [P] [US2] Byte-identity test in `gatling/binary/strings_test.go`: the Cyrillic name decodes byte-identical to the declared name, for every such name in every recording (SC-003, FR-004)
- [ ] T036 [P] [US2] Mixed-encoding test in `gatling/binary/strings_test.go`: names that fit Latin-1 and names that do not both decode from the same log, because the coder is a property of each string (US2 scenario 2)
- [ ] T037 [P] [US2] Malformed-coder test in `gatling/binary/strings_test.go`: a coder that is neither 0 nor 1 fails with a `*gatling.SyntaxError` naming the offset, never a replacement character (FR-003, edge case)
- [ ] T038 [P] [US2] Supplementary-plane test in `gatling/binary/strings_test.go` over a named fixture: a character outside the BMP survives intact or fails naming the offset — never silently truncated (US2 scenario 3, FR-033)

### Implementation for User Story 2

- [ ] T039 [US2] Implement the Latin-1 path (coder 0) and the UTF-16 path (coder 1, little-endian per research R4) in `gatling/binary/strings.go`, refusing any other coder before allocating
- [ ] T040 [US2] Document the byte-order assumption in `gatling/binary/doc.go`: the file records nothing about the writing JVM's byte order, little-endian is assumed, and no corpus this project can record proves it (research R4, spec Assumptions)

**Checkpoint**: names are correct in any alphabet, or the read fails saying where.

---

## Phase 5: User Story 3 — Repeated names cost nothing, and a broken reference is caught (P1)

**Goal**: the table is rebuilt in the writer's order and a reference that was never introduced ends
the read at its offset instead of renaming every later record.

**Independent Test**: decode a recording that repeats names heavily and confirm every name matches
the golden stream; corrupt one back-reference in a copy and confirm the read fails naming the offset.

### Tests for User Story 3 (write first, MUST fail) ⚠️

- [ ] T041 [P] [US3] Repetition test in `gatling/binary/strings_test.go`: a name written once in full and many times by reference yields the same string every time (FR-010)
- [ ] T042 [P] [US3] Dangling-reference test in `gatling/binary/strings_test.go`: a reference to an entry never introduced fails naming the byte offset, and no record after it is delivered (FR-012, SC-007)
- [ ] T043 [P] [US3] Index-sequencing test in `gatling/binary/strings_test.go`: a positive index that is not the next expected one is malformed, and index 0 is malformed (data-model §2)
- [ ] T044 [P] [US3] One-corrupted-byte sweep in `gatling/binary/mutation_test.go`: a copy of a recording with exactly one byte changed either decodes or fails naming that byte's offset — never returns a wrong name (SC-007)
- [ ] T045 [P] [US3] Distinct-names memory test in `gatling/binary/memory_test.go`: table memory tracks the number of distinct names, not the number of records (FR-013, US3 scenario 1)

### Implementation for User Story 3

- [ ] T046 [US3] Enforce append-only, strictly-sequential table growth in `gatling/binary/strings.go`, failing with an offset on a gap, on index 0, and on a reference past the table's end
- [ ] T047 [US3] State in `gatling/binary/doc.go` that the reader requires the start of the stream and why the table makes reading from the middle impossible (FR-011, US3 scenario 3)

**Checkpoint**: the one failure mode that would be silent and everywhere is now loud and local.

---

## Phase 6: User Story 4 — Nothing is decoded that the project cannot vouch for (P2)

**Goal**: the shared version policy gates this codec, and the advertised range equals the corpus.

**Independent Test**: feed run records naming a version below the range, inside it, above it, and one
that is not a plain release; confirm refusal, clean acceptance, acceptance with a warning, refusal.

### Tests for User Story 4 (write first, MUST fail) ⚠️

- [ ] T048 [P] [US4] Gate test in `gatling/binary/gate_test.go`: below the range refused with both the version found and the range in the error; inside accepted clean; above accepted with exactly one warning naming the version; not a plain release refused (FR-014, FR-015, SC-009)
- [ ] T049 [P] [US4] Strictness test in `gatling/binary/strict_test.go`: the same above-range log under `gatling.WithStrict` fails with a `*gatling.UnverifiedError` (US4 scenario 3)
- [ ] T050 [P] [US4] Range-equals-corpus test in `gatling/binary/gate_test.go`: `SupportedVersions()` equals the versions `testdata/corpus/gatling/` holds a binary recording for, discovered from the filesystem so widening one without the other fails (FR-016, SC-004a)

### Implementation for User Story 4

- [ ] T051 [US4] Apply `gatling.Policy` once in `NewReader`, before any event record is decoded, in `gatling/binary/reader.go` — the shared policy from v0.0.4, not a second copy (FR-015)
- [ ] T052 [US4] Set `SupportedVersions()` to 3.13.1–3.15.1 in `gatling/binary/capability.go`, with the doc comment saying widening it means recording a corpus entry first

**Checkpoint**: an unrecorded version cannot pass for a recorded one.

---

## Phase 7: User Story 5 — The caller stops having to know which Gatling ran (P2)

**Goal**: the dispatch table's binary row gains constructors, so detection routes to this codec and
`Supported()` reports it readable — both from the same row.

**Independent Test**: open a text log and a binary log through the same entry point and confirm both
yield records; confirm `Supported()` reports the binary format readable over the corpus range
without a decode.

### Tests for User Story 5 (write first, MUST fail) ⚠️

- [ ] T053 [P] [US5] Dispatch test in `gatling/simlog/simlog_test.go`: a binary log yields records rather than a `*gatling.UnsupportedFormatError` (FR-017, SC-010)
- [ ] T054 [P] [US5] Identity test in `gatling/simlog/simlog_test.go`: records obtained through the detecting entry point are identical field for field to opening `gatling/binary` directly (FR-019)
- [ ] T055 [P] [US5] Coverage test in `gatling/simlog/support_test.go`: `Supported()` reports the binary format readable, and its reported range equals `binary.SupportedVersions()` (FR-018, US5 scenario 2)

### Implementation for User Story 5

- [ ] T056 [US5] Fill the binary row's `versions`, `records` and `run` constructors in `gatling/simlog/simlog.go` per contract §4 — one row, no special case
- [ ] T057 [US5] Update `NewReader`'s doc comment in `gatling/simlog/simlog.go` to stop saying every binary log is refused

**Checkpoint**: two codecs behave as one library.

---

## Phase 8: User Story 6 — A log larger than memory still reads (P3)

**Goal**: peak memory bounded by the simulation rather than the run, and chunked reads identical to
whole-file reads down to the failing offset.

**Independent Test**: read a large generated log while observing peak memory, then read the same log
split at arbitrary byte boundaries and compare the two record streams.

### Tests for User Story 6 (write first, MUST fail) ⚠️

- [ ] T058 [P] [US6] Chunk-equivalence test in `gatling/binary/chunk_test.go`: one pass and arbitrary chunk boundaries produce identical records, and a failing log fails at the same offset with the same error (FR-027, SC-006)
- [ ] T059 [P] [US6] Corpus chunk test in `gatling/binary/chunk_corpus_test.go` covering every recording, shaped like `gatling/text/chunk_corpus_test.go`
- [ ] T060 [P] [US6] Peak-memory test in `gatling/binary/memory_test.go`: a 1 GB generated log reads end to end under 32 MiB, unchanged when the log is ten times longer with the same set of names (SC-005, FR-026)
- [ ] T061 [P] [US6] Allocation-cap test in `gatling/binary/limits_test.go`: a length prefix claiming more bytes than the file holds fails without allocating what it asked for (US6 scenario 4, FR-025)
- [ ] T062 [P] [US6] `FuzzDecode` in `gatling/binary/fuzz_test.go` seeded from the recordings: no input crashes, including empty, truncated, text and randomly mutated content (FR-024, SC-008)
- [ ] T063 [P] [US6] Truncation test in `gatling/binary/limits_test.go`: a log cut mid-record, mid-string and mid-length-prefix fails naming the offset (edge cases)

### Implementation for User Story 6

- [ ] T064 [US6] Hold the read buffer at a fixed size in `gatling/binary/read.go`, refusing to grow it with the log, and reuse the group scratch slice between records per contract §1
- [ ] T065 [US6] Make a stopped read distinguishable from one that reached the end in `gatling/binary/reader.go`: `io.EOF` only at a clean end, and the terminal error returned identically on every later call (FR-022, FR-023)

**Checkpoint**: every user story is functional.

---

## Phase 9: Polish & Cross-Cutting Concerns

- [ ] T066 Repoint `gatling/format_corpus_test.go` at the recorded corpus and delete `testdata/samples/gatling/binary/` — the 64-byte sample proved detection and is superseded, not extended (FR-034)
- [ ] T067 [P] Add the `CHANGELOG.md` entries under Unreleased exactly as drafted in [contracts/gatling-binary.md §6](./contracts/gatling-binary.md)
- [ ] T068 [P] godoc review: every new exported identifier documented, `gatling/binary` states the versions it accepts, and the doc comments match the contract
- [ ] T069 Coverage check: ≥ 90% for `gatling/binary`, ≥ 80% overall via `bash scripts/check-coverage.sh --enforce cover.out`; numbers go in the PR description (SC-012)
- [ ] T070 Run `go test -bench=. -benchmem ./gatling/binary/ ./gatling/text/` and compare with `benchstat`; record throughput, allocs/record and peak memory in plan.md Performance Goals (research R9)
- [ ] T071 Assert `binary.Capabilities()` equals `text.Capabilities()` in `gatling/binary/capability_test.go`, or record the difference and why — asserted, not assumed (FR-021, research R10, SC-011)
- [ ] T072 Verify `model/` and `gatling/` are still stdlib-only (`go list -deps`, the `deps` job in `.github/workflows/verify.yml`) and `go mod tidy` is clean
- [ ] T073 Confirm every test that passed before this feature still passes unchanged (SC-013)
- [ ] T074 Run every scenario in [quickstart.md](./quickstart.md)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: no dependencies, and it gates everything. The recordings are irreversible;
  the probe changes T001–T004 must all land before T005.
- **Phase 2 (Foundational)**: depends on Phase 1 for the logs its tests read. **Blocks all stories.**
  T012 (approval) blocks T013 and T014.
- **Phase 3–8 (Stories)**: all depend on Phase 2.
- **Phase 9 (Polish)**: depends on every story being complete.

### User Story Dependencies

- **US1 (P1)** — starts after Phase 2. The MVP.
- **US2 (P1)** and **US3 (P1)** — start after Phase 2, in parallel with US1 and with each other:
  they harden `strings.go`, which US1 does not touch. They are P1 rather than P2 because US1 is not
  shippable without them — a wrong name and a desynchronised table are both silent, and a silently
  wrong report is worse than a refusal.
- **US4 (P2)** — starts after Phase 2; touches `reader.go` and `capability.go`, so it serialises with
  T031/T032 in US1.
- **US5 (P2)** — depends on US1 and US4: the dispatch row needs both constructors and the range.
- **US6 (P3)** — starts after Phase 2; touches `read.go` and `reader.go`, so it serialises with T031.

### Within Each Story

Tests written and failing before implementation · grammars before readers · readers before anything
that folds their output · story complete before the next priority.

### Parallel Opportunities

- T008–T011 after the recordings; T015 and T017 within Phase 2
- Every test task marked [P] within a story
- US1, US2 and US3 by different people once Phase 2 lands, with `reader.go` (T031) the one contended
  file — US4's T051 and US6's T065 both wait on it
- T067, T068 and T071 in Phase 9

---

## Implementation Strategy

**Record first.** Phase 1 is the long pole and the only step that cannot be redone. A probe change
forgotten before a recording costs a whole re-run of every version.

**MVP = Phase 1 + Phase 2 + Phase 3 (US1).** At that point a current Gatling run is readable and the
milestone's central claim holds. It is not shippable yet: US2 and US3 close the two silent failures.

**Then**: US2 and US3 make it trustworthy · US4 makes it honest about what it has not seen · US5
makes it one library · US6 makes it hold on a soak log.

**One commit.** Per `AGENTS.md`, this is one `feat(gatling): …(#6)` commit, green on its own, with
the spec artefacts committed ahead of it as `docs(speckit): add 005-gatling-binary-decoder
spec/plan/tasks`.
