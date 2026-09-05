---
description: "Task list for 004-gatling-format-detection"
---

# Tasks: Telling Which Gatling Wrote a simulation.log

**Input**: Design documents from `/specs/004-gatling-format-detection/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/gatling-detect.md](./contracts/gatling-detect.md), [quickstart.md](./quickstart.md)

**Milestone**: [v0.0.4 Which Gatling wrote this log](https://github.com/galax-io/parsec/milestone/4) · **Issue**: [parsec#5](https://github.com/galax-io/parsec/issues/5)

**Tests**: REQUIRED, not optional. Constitution Principle III is NON-NEGOTIABLE and says test tasks are never optional in a task list. Every story phase writes its tests first, and they MUST fail before its implementation tasks start.

**Organization**: grouped by user story so each is independently implementable and testable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel — different files, no dependency on an incomplete task
- **[Story]**: which user story the task serves (US1 … US5, numbered as in spec.md)
- Exact file paths in every description

## Path Conventions

parsec is a single Go module with packages at the repository root:

- **Packages**: `gatling/` (detection, version policy, shared errors), `gatling/text/` (the text codec), `gatling/simlog/` (**new** — dispatch), `model/` (untouched by this feature)
- **Tests**: `<pkg>/<file>_test.go` beside the code, table-driven on stdlib `testing`
- **Golden corpus**: `testdata/corpus/gatling/<version>/` — untouched; this feature records no corpus entry
- **Sample**: `testdata/samples/gatling/binary/` — **new**, and deliberately not corpus (FR-031a)

## Phase order is by dependency, not by priority

US1, US2 and US3 are all P1. They are sequenced **US1 → US3 → US2 → US4 → US5** because that is the
dependency order: US2 cannot dispatch without US1's `Detect`, US4 adds a branch to the policy US3
builds, and US5 lives in the package US2 creates. The `[USn]` labels keep spec.md's numbering.

---

## Phase 1: Setup

**Purpose**: unblock everything that a later phase cannot recover from — the approval, and the one artefact that can only be captured by running a real Gatling.

- [X] T001 Read the required-reading skills for this change per the constitution (Quality Gates & Tooling → Engineering Guidance (Skills)): `golang-naming`, `golang-error-handling`, `golang-testing`, `golang-documentation`, `golang-structs-interfaces`. This change triggers every row — it adds exported identifiers, errors, tests, doc comments and exported types. Record any disagreement with the constitution in `specs/004-gatling-format-detection/research.md` rather than resolving it silently.
- [X] T002 Obtain maintainer approval for the two decisions in `specs/004-gatling-format-detection/contracts/gatling-detect.md` §5 — the variadic-options widening of `gatling/text`'s constructors, and the new package name `gatling/simlog`. **BLOCKING**: `AGENTS.md` lists a public API signature change as ask-first, and no implementation task starts until this is answered.
- [X] T003 Capture the binary sample into `testdata/samples/gatling/binary/3.15.1-head.bin`: run a throwaway minimal Gatling 3.15.1 simulation (**not** the corpus probe — `gatling-picatinny` has no release on the 3.14.x/3.15.x line, research R10) and keep the first 256 bytes of the `simulation.log` it writes. Confirm the first byte is `0x00`; **if it is not, the recording wins** — correct `Detect` and the spec, not the sample.
- [X] T004 [P] Write `testdata/samples/gatling/binary/SAMPLE.md` with the release, machine, JVM and exact command, in the shape of a corpus `RECORDING.md`. Its first line MUST state that this is a **sample, not a corpus entry**: no complete run, no report, and nothing may compare a decoder against it (FR-031a).
- [X] T005 [P] Confirm no CI change is needed: `scripts/check-coverage.sh` maps `*/gatling/*` to the 90% floor so `gatling/simlog` inherits it, and `verify.yml`'s `deps` job excludes this module's own packages so `simlog` importing `model` passes. Verify by reading both files; if either is wrong, that is a Complexity Tracking row in plan.md.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the shared option vocabulary every constructor signature references. Behaviour-neutral on its own.

**⚠️ CRITICAL**: no user story starts until this phase and T002 are complete.

- [X] T006 Add `Option`, `WithStrict` and the unexported `readOptions` to `gatling/options.go` with doc comments. `Option` is `func(*readOptions)` over an **unexported** config — a codec forwards options it never inspects, and a consumer cannot define one this package would have to honour (research R6, R13).
- [X] T007 Widen `NewReader` in `gatling/text/reader.go` and `NewRunReader` in `gatling/text/model.go` to `(r io.Reader, opts ...gatling.Option)`, storing the options unused for now. Every existing test in `gatling/text/` MUST pass **unchanged** — if one needed editing, this was not the behaviour-neutral widening it claims to be (FR-032, SC-010).

**Checkpoint**: signatures settled; US1 and US3 can start in parallel.

---

## Phase 3: User Story 1 — A log says what it is before anything reads it (Priority: P1) 🎯 MVP

**Goal**: identify a `simulation.log` as text, binary or neither from its leading bytes alone, never from its name, leaving the stream readable from byte 0.

**Independent Test**: hand `Detect` one text log, one binary log and several files that are neither, and confirm each is classified correctly and that the "neither" cases are named rather than forced into a format.

### Tests for User Story 1 (write first, MUST fail) ⚠️

- [X] T008 [P] [US1] Table test for `Detect` in `gatling/format_test.go` covering every row of the contract's behaviour table: `0x00…` → binary; `RUN\t…` and `ASSERTION\t…` → text; a proper prefix of `ASSERTION\t` then end of input → `*FormatError{Short: true}`; empty → short; `RUNX\t…`, `RUN ` (space, not tab), `<html>…`, `{0x1f,0x8b}` → `*FormatError{Short: false}`.
- [X] T009 [P] [US1] Evidence test in `gatling/format_corpus_test.go`: both `testdata/corpus/gatling/3.11.5/simulation.log` and `3.12.0/simulation.log` detect as `FormatText`, and `testdata/samples/gatling/binary/3.15.1-head.bin` as `FormatBinary`. The corpus logs open with `ASSERTION`, which is what falsifies issue #5's first-byte-`R` rule — assert the opening bytes explicitly so the test says why it exists.
- [X] T010 [P] [US1] Name-independence test in `gatling/format_corpus_test.go`: the same bytes classify identically whatever the file is called, asserted by reading through an `io.Reader` that carries no name at all (FR-002).
- [X] T011 [P] [US1] `FuzzDetect` in `gatling/format_fuzz_test.go`: no input panics, and `Detect` never reads past `DetectSize` (FR-028, SC-007, SC-008).

### Implementation for User Story 1

- [X] T012 [US1] Add `FormatError` to `gatling/errors.go` with `Head []byte` and `Short bool`, and an `Error()` that renders `Head` quoted and printable so a gzip stream produces a readable message rather than a spray of bytes.
- [X] T013 [US1] Add `Format`, `Format.String`, `DetectSize` and `Detect` to `gatling/format.go` (depends on T012). Test `0x00` first — it cannot collide with an ASCII literal — then the two text openings, each with its tab. Decide as soon as the bytes are conclusive so a one-byte binary input classifies without waiting for nine more.
- [X] T014 [US1] Doc comments on every identifier added above, stating what `Detect` does *not* do: it never consults a file name and never guesses (Principle V). Add the `Added` entries for them to `CHANGELOG.md` under Unreleased.

**Checkpoint**: a caller can classify a log. Nothing dispatches yet.

---

## Phase 4: User Story 3 — One version policy, applied the same way by every codec (Priority: P1)

**Goal**: the refuse/accept/warn decision exists once, is applied before any record is decoded, and no codec carries its own copy.

**Independent Test**: drive the policy directly with a version below, inside and above the range and confirm refusal-naming-both, clean acceptance, and acceptance with exactly one warning — then confirm the text codec's observable behaviour is unchanged.

### Tests for User Story 3 (write first, MUST fail) ⚠️

- [X] T015 [P] [US3] Table test for `Policy.Apply` in `gatling/policy_test.go`: below `Min` → zero `Warning` and a `*VersionError` naming both versions; inside → zero `Warning`, nil error; above `Max` → the `Warning`, nil error. Assert the error type with `errors.As`, never by message text.
- [X] T016 [P] [US3] Regression test in `gatling/text/gate_test.go`: the three outcomes a caller sees through `text.NewReader` are byte-identical to what the existing tests already assert — the same error strings, the same warning, the same records. This is the test that proves the refactor moved the decision without changing it.
- [X] T017 [P] [US3] Single-application test in `gatling/text/gate_test.go`: an above-range log yields exactly **one** warning — counted, not "at least one" — through `text.NewReader` and through `text.NewRunReader` (FR-016, SC-004).

### Implementation for User Story 3

- [X] T018 [US3] Add `Policy` (fields `Min`, `Max`) and `Policy.Apply(found Version, opts ...Option) (Warning, error)` to `gatling/policy.go`, built on the existing `Gate`. Implement the three non-strict outcomes only; the strict branch is US4. Leave `Gate` untouched — keeping it is what makes this change purely additive (research R7).
- [X] T019 [US3] Rewrite `finishPreamble` in `gatling/text/reader.go` to call `Policy.Apply` instead of switching on `Gate` itself, feeding it the `minVersion`/`maxVersion` from `gatling/text/parse.go`. The lenient surplus-field path stays tied to the unverified verdict.
- [X] T020 [US3] Doc comments for `Policy` and `Apply` stating that it is the single place the outcomes are decided and that it runs before any record is decoded. `CHANGELOG.md` `Added` entries.

**Checkpoint**: one policy, one place. The binary codec in v0.0.5 inherits it rather than reimplementing it.

---

## Phase 5: User Story 2 — The right reader is chosen, or you are told why there is none (Priority: P1)

**Goal**: hand over a log once and receive a reader for it, or an error that names the real cause — never a syntax error about line 1.

**Independent Test**: open a covered text log through `simlog` and confirm the records equal those from the codec directly; open a binary log and confirm the error names the binary format and the absence of a codec.

### Tests for User Story 2 (write first, MUST fail) ⚠️

- [X] T021 [P] [US2] Equivalence test in `gatling/simlog/simlog_test.go`: over every corpus log, records from `simlog.NewReader` are identical field for field to `text.NewReader`, and items from `simlog.NewRunReader` identical to `text.NewRunReader`. Zero differences (FR-011, SC-009).
- [X] T022 [P] [US2] Honest-refusal test in `gatling/simlog/errors_test.go`: opening `testdata/samples/gatling/binary/3.15.1-head.bin` through `simlog` yields a `*gatling.UnsupportedFormatError` naming the binary format — asserted **side by side** with `text.NewReader` on the same bytes yielding a `*gatling.SyntaxError` about line 1, because the difference between the two is the point of the milestone (SC-002).
- [X] T023 [P] [US2] Discrimination test in `gatling/simlog/errors_test.go`: the three refusals — unknown format, known format without a codec, unsupported version — are told apart with `errors.As` alone, with no message-text matching anywhere in the test (FR-010).
- [X] T024 [P] [US2] Typed-nil test in `gatling/simlog/errors_test.go`: on **every** error path the returned reader compares `== nil` against the interface. Returning a nil `*text.Reader` would produce a non-nil interface and a caller's `if rd != nil` would pass before panicking (research R13; quickstart scenario 11).
- [X] T025 [P] [US2] Stream-preservation test in `gatling/simlog/chunk_test.go`: reading a corpus log through `simlog` one byte at a time — which is what makes a swallowed head visible — and in arbitrary chunk sizes produces the same records as one pass, and the same failure at the same line for a log that fails (FR-004, Principle II).
- [X] T026 [P] [US2] `ExampleNewRunReader` in `gatling/simlog/example_test.go` that compiles, runs and matches its `// Output:` block, as `gatling/text` and `model` already have (quickstart scenario 12).

### Implementation for User Story 2

- [X] T027 [US2] Add `UnsupportedFormatError` to `gatling/errors.go` with a `Format` field, and an `Error()` reading `binary simulation.log: this module has no codec for it yet`.
- [X] T028 [US2] Create `gatling/simlog/doc.go`: what the package is for, and that a caller who already knows the version is one call shorter using the codec directly.
- [X] T029 [US2] Implement `gatling/simlog/simlog.go` — the `RecordReader` and `RunReader` interfaces, `NewReader` and `NewRunReader`. Read up to `gatling.DetectSize` bytes with `io.ReadFull` into a fixed array, classify, then hand the codec `io.MultiReader(bytes.NewReader(head[:n]), r)` so the stream still begins at byte 0. Map `io.EOF` and `io.ErrUnexpectedEOF` to the short-input refusal; return any other read error wrapped with `%w`, never as a misclassification (research R2, FR-028).
- [X] T030 [US2] Doc comments on the interfaces and both constructors, naming every error a caller can get and stating that the version gate is the codec's and is applied once. `CHANGELOG.md` `Added` entries.

**Checkpoint**: a binary log now produces the error issue #5 exists for, instead of a syntax error nobody can act on.

---

## Phase 6: User Story 4 — A caller who cannot accept an unproven result can refuse it (Priority: P2)

**Goal**: strict mode turns the above-range warning into a refusal, and changes nothing else.

**Independent Test**: read an above-range log twice, strict and lenient, and confirm refusal and one-warning success; confirm strictness changes nothing for a version inside or below the range.

### Tests for User Story 4 (write first, MUST fail) ⚠️

- [X] T031 [P] [US4] Matrix test in `gatling/policy_test.go`: for each of below-range, in-range, above-range and a non-release string, assert the outcome with and without `WithStrict`. Only the above-range cell differs; the other three are identical, which is FR-021 asserted rather than assumed.
- [X] T032 [P] [US4] Discrimination test in `gatling/policy_test.go`: `*UnverifiedError` and `*VersionError` are distinct types under `errors.As` — they describe opposite evidence gaps, too new against too old (FR-022).
- [X] T033 [P] [US4] End-to-end strictness test in `gatling/simlog/strict_test.go`: `WithStrict` reaches the policy through all four constructors — `text.NewReader`, `text.NewRunReader`, `simlog.NewReader`, `simlog.NewRunReader` — and an in-range log read strictly yields records byte-identical to the lenient read (SC-005).

### Implementation for User Story 4

- [X] T034 [US4] Add `UnverifiedError` to `gatling/errors.go` with `Version`, `Min`, `Max`, and an `Error()` that says the version is above the verified range **and that this read is strict**, so the message distinguishes itself from the plain warning.
- [X] T035 [US4] Extend `Policy.Apply` in `gatling/policy.go` with the strict branch: above `Max` under `WithStrict` returns the zero `Warning` and an `*UnverifiedError`. Strictness MUST NOT be able to reach the other two verdicts — keep the branch inside the unverified case so that is true by construction.
- [X] T036 [US4] Thread the stored options from T007 into the `Policy.Apply` call in `gatling/text/reader.go`, and forward `opts` untouched in `gatling/simlog/simlog.go`. `CHANGELOG.md` entries for `WithStrict` and `UnverifiedError`.

**Checkpoint**: a release gate can refuse a number no recording proves.

---

## Phase 7: User Story 5 — A consumer can state what parsec reads without running it (Priority: P2)

**Goal**: per-format coverage readable programmatically, including the honest answer that the binary format has no codec yet.

**Independent Test**: read the advertised range and confirm it equals the corpus-bound range; confirm the binary format reports as known-and-not-readable.

### Tests for User Story 5 (write first, MUST fail) ⚠️

- [X] T037 [P] [US5] Test in `gatling/simlog/support_test.go` that `Supported()` returns the text format as readable over the range `text.SupportedVersions()` reports — **derived, not restated**, so widening the corpus without widening the range fails and so does the reverse (FR-024, SC-006).
- [X] T038 [P] [US5] Test in `gatling/simlog/support_test.go` that the binary format is reported with `Readable: false` and zero versions, that this is a third answer distinct from an unknown format, and that the entry order is fixed so a consumer can rely on it (FR-025, FR-026).

### Implementation for User Story 5

- [X] T039 [US5] Implement `Support` and `Supported()` in `gatling/simlog/support.go`, taking the text range from `text.SupportedVersions()` and never from a literal. Doc comment states that a caller cannot widen a range. `CHANGELOG.md` entry.

**Checkpoint**: all five stories independently functional.

---

## Phase 8: Polish & Cross-Cutting Concerns

- [X] T040 [P] Benchmarks in `gatling/format_bench_test.go` and `gatling/simlog/bench_test.go` using `b.Loop()` and `b.ReportAllocs()`, as the repository's existing benchmarks already do: `Detect` allocates nothing and does not vary with input size; opening through `simlog` costs at most one extra allocation over the codec directly. Compare with `benchstat` and paste its output into the PR body — that is how the constitution's "a regression against the recorded number MUST be justified in the PR" is satisfied.
- [X] T041 [P] Update the package list in `doc.go` and the Packages block in `README.md` to include `gatling/simlog/` with its milestone (v0.0.4), and say in the README's Status section that a log can now be opened without knowing which Gatling wrote it.
- [X] T042 [P] FR-029 audit across `gatling/format_test.go`, `gatling/format_corpus_test.go`, `gatling/policy_test.go`, `gatling/text/gate_test.go` and `gatling/simlog/errors_test.go`: confirm every one of the eleven outcomes named in the spec's Source Coverage — text identified, binary identified, neither, too short, empty, below range, in range, above range lenient, above range strict, non-release string, known format without a codec — has at least one automated check, and that each check fails when its rule is removed. Record the outcome-to-test map in the PR description.
- [X] T043 godoc review across `gatling/` and `gatling/simlog/`: every exported identifier documented, and `CHANGELOG.md` complete under Unreleased with the `Added` and `Changed` entries drafted in contracts §6.
- [X] T044 Coverage: `go test -tags=integration -count=1 -skip 'PeakMemory$' -coverpkg=./... -coverprofile=cover.out ./...` then `bash scripts/check-coverage.sh --enforce cover.out`. `gatling/` and `gatling/simlog/` at 90% or above, module at 80% or above; the numbers go in the PR description.
- [X] T045 Boundary checks: `go list -deps` confirms `model/` and `gatling/...` are still stdlib-only, and `go mod tidy` leaves the tree unchanged.
- [X] T046 Run every scenario in [quickstart.md](./quickstart.md) end to end, including scenario 9 — every test that passed in `gatling/text/` before this feature still passes unchanged.
- [X] T047 Commit as one green `feat(gatling): tell which Gatling wrote a simulation.log (#5)` — `go build ./... && go test ./...` passing on its own. Assign the PR to milestone **v0.0.4** and close [parsec#5](https://github.com/galax-io/parsec/issues/5) when it lands on `main`.
- [ ] T048 Amend [parsec#5](https://github.com/galax-io/parsec/issues/5) with the two corrections this feature established: its first-byte-`R` detection rule is falsified by this repository's own corpus (both logs open with `ASSERTION`), and its acceptance bullet is met for **format** in all five named versions but for **version** only in the two this module can decode — the binary three arrive with their codec in v0.0.5.

---

## Dependencies & Execution Order

### Phase dependencies

- **Setup (Phase 1)** — no dependencies. T002 (approval) and T003 (the sample) are the long poles: T002 blocks every implementation task, T003 blocks T009 and T022.
- **Foundational (Phase 2)** — depends on T002. Blocks all five stories.
- **US1 (Phase 3)** and **US3 (Phase 4)** — independent of each other; both may start once Phase 2 is done.
- **US2 (Phase 5)** — depends on US1 (`Detect`, `Format`, `FormatError`).
- **US4 (Phase 6)** — depends on US3 (`Policy.Apply`) and T007.
- **US5 (Phase 7)** — depends on US2 (the `simlog` package exists) and US1 (`Format`).
- **Polish (Phase 8)** — depends on every story it audits.

### Within each story

- Tests are written first and MUST fail before implementation starts.
- Error types before the code that returns them (T012 before T013; T027 before T029; T034 before T035).
- `gatling/` before `gatling/simlog/` — the dependency runs one way only.

### Parallel opportunities

- T004 and T005 while T003's Gatling run is in progress.
- Every test task inside one story carries `[P]`: they live in different files and share no state.
- US1 and US3 are genuinely parallel — different files, no shared identifier — and are the two halves that make the biggest phase (US2) possible.
- T040, T041 and T042 in Polish are independent of each other.

---

## Parallel Example: User Story 1

```bash
# All four US1 tests together — different files, no shared state:
Task: "Detect table test in gatling/format_test.go"
Task: "Corpus and sample evidence test in gatling/format_corpus_test.go"
Task: "Name-independence test in gatling/format_corpus_test.go"
Task: "FuzzDetect in gatling/format_fuzz_test.go"
```

```bash
# US1 and US3 in parallel once Phase 2 is done:
Track A: T008-T014  (gatling/format.go, gatling/errors.go)
Track B: T015-T020  (gatling/policy.go, gatling/text/reader.go)
```

---

## Implementation Strategy

### MVP — US1 alone

1. Phase 1 Setup, including the approval and the sample
2. Phase 2 Foundational
3. Phase 3 US1
4. **STOP and validate**: a log can be classified from its bytes, and the corpus proves the rule issue #5 got wrong

That is a real increment: it is the smallest thing that answers "which Gatling wrote this", and every later phase builds on it.

### Incremental delivery

1. Setup + Foundational → signatures settled, sample captured
2. + US1 → a log identifies itself (**MVP**)
3. + US3 → one policy, which is what v0.0.5's binary codec will inherit
4. + US2 → the honest refusal, which is the outcome issue #5 asks for
5. + US4 → a release gate can refuse an unproven number
6. + US5 → a consumer can report coverage without hard-coding it

### What to watch

- **T003 is irreplaceable and risky.** It needs a real Gatling 3.15.1 and the corpus probe cannot be used (no picatinny release on that line). If it cannot be run at all, US1's binary half and US2's refusal test have no evidence and the feature stalls — raise it immediately rather than substituting a hand-written fixture, which FR-031 forbids.
- **T002 gates everything.** Writing implementation before the API change is approved risks throwing it away.
- **`0x00` is a claim until T003 confirms it.** If the sample's first byte differs, the recording wins: correct `Detect`, the contract and the spec, and say so in research R4.

---

## Notes

- `[P]` = different files, no dependency on an incomplete task.
- One tracked issue = one semantic commit, green on its own (`go build ./... && go test ./...`).
- This feature records **no corpus entry**. The sample under `testdata/samples/` is evidence for identification and nothing else; the complete binary recordings belong to v0.0.5.
- Spec artifacts are committed as `docs(speckit): add 004-gatling-format-detection spec/plan/tasks` **before** any `feat` commit.


---

## What actually happened

Every task above is done except T048, which edits a GitHub issue and is held for confirmation.
Three things went differently from the plan, and each is recorded where it belongs rather than
quietly absorbed.

**T003 kept 64 bytes, not 256.** The whole log a 3.15.1 run produced is 154 bytes, so "the first
256" would have been the entire file — exactly the shape FR-031a forbids the sample from having,
because a complete log invites being read as a recording. 64 is six times the detection window and
unmistakably a fragment. The reason is in `testdata/samples/gatling/binary/SAMPLE.md`.

**T007 landed with T019, and T031–T032 were written before T018.** Widening the constructors and
storing options the codec did not yet read would have left a dead field, and implementing
`Policy.Apply` with an options parameter it ignored would have left a dead parameter — both are
states the linter and Principle VI reject. The two pairs were done together, tests still first.

**The performance claim was wrong and was corrected, not explained away.** The plan predicted
dispatch would cost at most one extra allocation. It costs four, and 124 bytes: `io.MultiReader`,
its slice, the `bytes.Reader` over the replayed head, and the head escaping to the heap. The
property that mattered held — the cost is constant and the size of the log cannot reach it, with
`Detect` measuring 5.928 ns on 14 bytes and 5.949 ns on 1 MiB — so plan.md, research.md and
quickstart.md now carry the measurement instead of the guess.

## The claim this milestone turned into evidence

`0x00` as the binary marker was a reading of issue #6 that nothing in this repository had checked.
A real Gatling 3.15.1 log opens:

```
00 | 00 00 00 06 | "3.15.1" | 00 | 00 00 00 08 | "BinProbe" | ...
```

The first byte is `0x00`, so `Detect` is right for the reason it claimed. The rest of the shape —
a string is a 4-byte length, the bytes, then a coder byte — is recorded in `SAMPLE.md` for v0.0.5
and decoded by nothing here.

## Gates, all green

| Gate | Result |
|---|---|
| `gofmt` | clean |
| `go vet ./...` | no issues |
| `golangci-lint run ./...` | 0 issues, every build configuration |
| `go test -race -shuffle=on ./...` | pass |
| `go test -tags=integration -race ./...` | pass |
| stdlib-only (`model/`, `gatling/...`) | clean |
| `go mod tidy` | no change |
| Coverage | `gatling` 100.0%, `gatling/simlog` 96.4%, `gatling/text` 96.7%, `model` 97.5%, overall 97.3% |

## FR-029: the eleven outcomes, each with a check

| Outcome | Test |
|---|---|
| text identified | `TestDetectCorpusIsText` |
| binary identified | `TestDetectBinarySampleIsBinary` |
| neither | `TestDetect/html`, `/gzip`, `/json` |
| too short | `TestDetect/prefix_of_ASSERTION`, `/prefix_of_RUN` |
| empty | `TestDetect/empty` |
| below range | `TestPolicyApply/far_below_the_range` |
| in range | `TestPolicyApply/the_oldest_covered_version` |
| above range, lenient | `TestPolicyApply/the_next_minor` |
| above range, strict | `TestPolicyApplyStrictness`, `TestStrictReachesTheGateThroughEveryConstructor` |
| not a release string | `TestGateNotARelease` (spec 002, still passing unchanged) |
| known format, no codec | `TestBinaryIsRefusedByFormatNotBySyntax` |
