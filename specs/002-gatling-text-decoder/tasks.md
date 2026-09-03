---

description: "Task list for the Gatling 3.11.5–3.12.x text simulation.log decoder"
---

# Tasks: Reading Gatling 3.11.5–3.12.x Text simulation.log Files

**Input**: Design documents from `/specs/002-gatling-text-decoder/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/gatling-text.md](contracts/gatling-text.md), [quickstart.md](quickstart.md)

**Tests**: REQUIRED (constitution Principle III). Every story lists its tests before its implementation, tests are written first and MUST fail before the implementation task starts. In Go a test file that does not compile is a failing test, which is how the first test of each story fails.

**Organization**: grouped by user story. One tracked issue (#3) means one semantic commit at the end — the tasks are the working order, not the commit history. Squash before review.

**Before T001**: commit and merge the spec PR — `docs(speckit): add 002-gatling-text-decoder spec/plan/tasks`, milestone v0.0.2 — on its own, before any `feat` commit. It merges on review alone because CI ignores `specs/`.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: US1–US5 from spec.md
- Every task names the exact file it touches

## Path Conventions

Per [plan.md](plan.md) "Source Code":

- **Shared record types, version, errors**: `gatling/`
- **The codec**: `gatling/text/`
- **Fixtures** (hand-written, malformed): `gatling/text/testdata/fixtures/*.fixture.log` — the word `fixture` in the name is mandatory (FR-023)
- **Golden corpus**: `testdata/corpus/gatling/<version>/` with `simulation.log`, `global_stats.json`, `stats.json`, `records.golden`
- **Sample simulation used to record the corpus**: `testdata/corpus/gatling/simulation/` — not a version directory, and it has no `simulation.log`
- **Integration suite**: build tag `integration`; it MUST fail, not skip, when the corpus is absent (constitution: "an empty run fails")

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: package skeleton, the sample simulation, and the corpus — recorded now or never

- [X] T001 Create `gatling/`, `gatling/text/`, `gatling/text/testdata/fixtures/`, `testdata/corpus/gatling/3.11.5/`, `testdata/corpus/gatling/3.12.0/` and `testdata/corpus/gatling/simulation/`; add `gatling/doc.go` and `gatling/text/doc.go` with the package comments from [contracts/gatling-text.md](contracts/gatling-text.md), each stating the Gatling versions accepted
- [X] T002 Write the sample simulation as a minimal sbt project in `testdata/corpus/gatling/simulation/` that takes the Gatling version from a system property so one project runs unchanged under 3.11.5 and 3.12.0. The single scenario MUST produce, in one run: at least one declared assertion; nested groups two deep; one request outside any group; a group whose name contains a comma; a request that fails a check (status 500 from the stub); a request that fails with an exception (connect to a closed local port, which also yields an `ERROR` record); and an attempt at an error whose message spans two lines. Document the run command per version in `testdata/corpus/gatling/simulation/README.md`
- [X] T003 [P] Write the stub endpoint in `testdata/corpus/gatling/simulation/stub/main.go` (net/http only): `/ok` returns 200, `/fail` returns 500, `/slow` sleeps 1500 ms then returns 200 so response-time range buckets are populated. `go run ./testdata/corpus/gatling/simulation/stub`
- [X] T004 Record the 3.11.5 corpus: run T002 against T003 under Gatling 3.11.5, then copy — unmodified — `simulation.log`, `js/global_stats.json` and `js/stats.json` from the run directory into `testdata/corpus/gatling/3.11.5/`. Before committing, confirm by inspection and note in `testdata/corpus/gatling/3.11.5/RECORDING.md`: the exact Gatling version, OS and its line separator, JVM charset, that `global_stats.json` carries `numberOfRequests` total/ok/ko and `meanNumberOfRequestsPerSecond`, that `stats.json` carries the same per request and per group, whether either file carries anything for virtual users or error records, whether a comma appeared in the group name in the log, and whether the two-line error attempt produced a multi-line `ERROR` record. Requires Gatling 3.11.5 installed; nothing can be added after archiving (Principle III)
- [X] T005 Record the 3.12.0 corpus the same way into `testdata/corpus/gatling/3.12.0/` with its own `RECORDING.md`. Same simulation, same stub, Gatling 3.12.0
- [X] T006 [P] Confirm `.golangci.yml` needs no change for `gatling/` and `gatling/text/`; if it does, add a Complexity Tracking row to `specs/002-gatling-text-decoder/plan.md` before changing it

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the shared types, the version gate, the error types and the bounded line scanner that every story reads through

**⚠️ CRITICAL**: no user story work can begin until this phase is complete

- [X] T007 [P] Write `gatling/version_test.go`: table-driven `ParseVersion` accepts `3.11.5`, `3.12.0`, `3.13.0`, `10.0.0`; rejects `3.13.0-SNAPSHOT`, `3.12.0-M1`, `3.12`, `v3.12.0`, `3.12.0 `, empty and `garbage`, each with the string quoted in the error; `Compare` orders by major, then minor, then patch; `String` round-trips. MUST fail (package does not compile) before T010
- [X] T008 [P] Write `gatling/errors_test.go`: `(*SyntaxError).Error()` contains the 1-based line number, what was expected and what was found; `(*VersionError).Error()` contains the version string as written plus both range bounds; `Warning.Error()` names the version and says no recording covers it. MUST fail before T011
- [X] T009 [P] Implement `gatling/record.go`: `Kind` with the six constants and `String()`, `Status` (`StatusOK`, `StatusKO`), `Event` (`EventStart`, `EventEnd`), `Header`, and the flat `Record` exactly as in [contracts/gatling-text.md](contracts/gatling-text.md), with the per-field kind table from [data-model.md](data-model.md) in the doc comment and the `Groups` ownership rule ("valid until the next call to Next; copy to keep") stated on the field
- [X] T010 [P] Implement `gatling/version.go`: `Version`, `ParseVersion` accepting only `MAJOR.MINOR.PATCH` of decimal digits with nothing before or after, `String`, `Compare`, and `Verdict` with `VerdictRefused`, `VerdictAccepted`, `VerdictUnverified`; `Gate(found Version, min, max Version) Verdict` — below `min` refused, within accepted, above `max` unverified
- [X] T011 [P] Implement `gatling/errors.go`: `SyntaxError{Line, Expected, Found}`, `VersionError{Found, Version, Min, Max}`, `Warning{Version, Min, Max}` with the `Error()` texts T008 asserts; all pointer receivers for the two errors so `errors.As` works
- [X] T012 Write `gatling/text/scan_test.go`: the scanner yields lines with 1-based numbers; strips one trailing `\r`; a line of exactly `MaxLineLen` bytes is returned and one of `MaxLineLen+1` fails with `*gatling.SyntaxError` at that line without the buffer growing past `MaxLineLen` (assert via `testing.AllocsPerRun` on a second pass and via `runtime.MemStats` before/after); a final line with no terminator is reported as unterminated; empty input yields no lines. MUST fail before T013
- [X] T013 Implement `gatling/text/scan.go`: a line scanner over `io.Reader` built on `bufio.Reader` with a fixed buffer, `MaxLineLen = 1 << 20`, that returns each line's bytes, its 1-based number and whether it was terminated; over-long lines return the error without buffering the remainder; `\r\n` handled by stripping the `\r`

**Checkpoint**: `go test -race ./gatling/...` green; nothing in `gatling/text` decodes a record yet

---

## Phase 3: User Story 1 — An archived run becomes readable again (Priority: P1) 🎯 MVP

**Goal**: decode every record of a recorded 3.11.5 run and prove the counts equal the report Gatling produced for that run, to the unit

**Independent Test**: `go test -tags=integration -race -run 'TestGolden|TestReport' ./gatling/text/` against `testdata/corpus/gatling/3.11.5/` passes on exact equality and fails on a difference of one

### Tests for User Story 1 (REQUIRED — write first, MUST fail before implementation) ⚠️

- [X] T014 [P] [US1] Write `gatling/text/parse_test.go`: table-driven over every kind with well-formed lines — `RUN` 6 fields; `USER` 4 with `START` and `END`; `REQUEST` 7 with an empty group path, a one-level path, a three-level path, `OK` with a lone-space message and `KO` with a message; `GROUP` 6; `ERROR` 3; `ASSERTION` 2 — asserting every field of the resulting `gatling.Record` including `Line`; a lone space decodes as `""` for the description and the message; timestamps come back as the exact integers written; the group path splits on commas into the ordered list and a top-level request has `len(Groups) == 0`. MUST fail before T018
- [X] T015 [P] [US1] Write `gatling/text/reader_test.go`: with three `ASSERTION` lines then `RUN` then events, `NewReader` returns with `Header()` populated, `Assertions()` holding the three payloads byte for byte in file order, `Warnings()` empty for 3.11.5; with zero assertions the header on line 1 works the same; `Next` yields events in file order with correct `Line` values and returns `io.EOF` at the end, and keeps returning `io.EOF` when called again; an `ASSERTION` line after the header comes back through `Next` as a `KindAssertion` record. MUST fail before T019
- [X] T016 [P] [US1] Write `gatling/text/golden_test.go` behind `//go:build integration`: for each `testdata/corpus/gatling/*/simulation.log`, decode with `NewReader` and serialise to the canonical form — one line per item, `HEADER` first, then `ASSERTION` lines, then one line per record as `<line> <KIND> field=%q ...` with fields in the [data-model.md](data-model.md) order and strings `%q`-quoted so a lone space and an empty string are distinguishable — and compare against `records.golden` in that directory; `-update` rewrites the golden; the test FAILS, not skips, when no corpus directory is found (constitution: an empty run fails). Provide the canonical serialiser as a test helper in this file for T024 to reuse. MUST fail before T019 and T021
- [X] T017 [P] [US1] Write `gatling/text/report_test.go` behind `//go:build integration`: for each corpus directory, decode the run and assert exact equality between counts derived from the records and the report — `numberOfRequests` total/ok/ko from `global_stats.json`, and the same triple for every request name and every group by walking the `contents` tree of `stats.json` (values there may be JSON strings; parse them as numbers); then the mean request rate: `count / ceil((maxTs - minTs) / 1000)` with `minTs`/`maxTs` taken over **every** timestamped record — user, request, group, error, and the header start — compared to `meanNumberOfRequestsPerSecond` total/ok/ko after rounding ours to the precision the file prints (FR-021b, FR-021c). Fails when the corpus is absent. MUST fail before T019

### Implementation for User Story 1

- [X] T018 [US1] Implement `gatling/text/parse.go`: one function per kind taking the split fields and the line number and returning `gatling.Record`; field counts exact — 6/4/7/6/3/2 — with any other count returning `*gatling.SyntaxError{Line, Expected: "<kind> with N fields", Found: "M fields"}`; the `ERROR` message is the fields between the kind and the last one joined with `\t` (FR-008b) so its count check is "at least 3"; a lone-space description or message decodes to `""`; `OK`/`KO` and `START`/`END` parsed strictly; integers via `strconv.ParseInt`; the group path split on `,` into a `[]string` that the caller may reuse
- [X] T019 [US1] Implement `gatling/text/reader.go`: `Reader`, `NewReader`, `Header`, `Assertions`, `Warnings`, `Next` per the contract. `NewReader` scans lines: each leading `ASSERTION` payload is appended; the first `RUN` is parsed into `Header` and its version run through `gatling.Gate` against `MinVersion`/`MaxVersion` — refused returns `*gatling.VersionError` (a non-release version string also refuses, quoting it), unverified appends a `gatling.Warning`; any other kind before the header returns `*gatling.SyntaxError{Expected: "ASSERTION or RUN"}`; running out of input before a header returns `*gatling.SyntaxError` naming the last line read, and empty input names line 0. `Next` splits the next line on `\t` and dispatches by kind; it returns `io.EOF` at end and, once any other error has been returned, returns that same error on every later call (sticky failure, FR-014). `Groups` reuses one slice across calls
- [X] T020 [US1] Generate `testdata/corpus/gatling/3.11.5/records.golden` with `go test -tags=integration -run TestGolden -update ./gatling/text/`, then review it line by line against `simulation.log` — every lone space shown as `""`, every group path as a list, every `KO` carrying its message — before committing; the review is the test, since the first golden can only come from the decoder itself
- [X] T021 [US1] Doc comments on every exported identifier in `gatling/` and `gatling/text/`, the reader's stating "accepts Gatling 3.11.5 through 3.12.0"; rewrite the root `doc.go` so it no longer says none of the packages exist

**Checkpoint**: `go test -race -shuffle=on ./...` and `go test -tags=integration -race ./gatling/text/` green on 3.11.5. US1 is demonstrable on its own.

---

## Phase 4: User Story 2 — The same simulation under 3.12.0 yields the same records (Priority: P2)

**Goal**: prove the format did not move between the two versions — no new code, new evidence

**Independent Test**: `go test -tags=integration -race -run 'TestGolden|TestReport|TestCrossVersion' ./gatling/text/` passes with both corpus directories present

### Tests for User Story 2 (REQUIRED — write first, MUST fail before implementation) ⚠️

- [X] T022 [P] [US2] Write `gatling/text/crossversion_test.go` behind `//go:build integration`: decode both `testdata/corpus/gatling/3.11.5/` and `testdata/corpus/gatling/3.12.0/`, serialise both with the T016 helper, mask every timestamp field, the header's `RunID`, `Start` and `Version`, and assert the two masked streams are identical line for line — same kind sequence, same names, same paths, same statuses, same messages; a field present in one and absent in the other MUST show as a difference, never be absorbed by a zero value (spec US2 scenario 3). Fails when either directory is absent. MUST fail before T023 because the 3.12.0 golden does not exist yet

### Implementation for User Story 2

- [X] T023 [US2] Generate `testdata/corpus/gatling/3.12.0/records.golden` with `-update`, review it line by line against its `simulation.log` as in T020, and commit
- [X] T024 [US2] Confirm T016 and T017 iterate both directories and pass on 3.12.0 with no code change to `gatling/text/`; if any code change is needed to pass, the format differs and [research.md](research.md) R1 is wrong — stop and record the difference there before touching `parse.go`

**Checkpoint**: both versions decode identically apart from timestamps and run identity

---

## Phase 5: User Story 3 — Nothing is decoded that the project cannot vouch for (Priority: P2)

**Goal**: all four gate outcomes are real and tested against the reader, not just against `ParseVersion`

**Independent Test**: `go test -race -run TestGate ./gatling/text/` covers refused-below, accepted, warned-above and refused-non-release, plus the header-position rules

### Tests for User Story 3 (REQUIRED — write first, MUST fail before implementation) ⚠️

- [X] T025 [P] [US3] Add fixtures under `gatling/text/testdata/fixtures/`: `version-3.9.0.fixture.log`, `version-3.13.0.fixture.log`, `version-3.13.0-snapshot.fixture.log`, `version-3.12.0-m1.fixture.log`, `version-garbage.fixture.log`, `no-header.fixture.log` (events only), `event-before-header.fixture.log` (a `USER` line ahead of `RUN`), `empty.fixture.log` (zero bytes), and `surplus-field-3.13.0.fixture.log` (a 3.13.0 header followed by a `USER` line with five fields) — each a valid log otherwise, three or four events long
- [X] T026 [P] [US3] Write `gatling/text/gate_test.go` over those fixtures: 3.9.0 → `NewReader` returns `*gatling.VersionError` with `Found == "3.9.0"` and both bounds, and no `Reader`; 3.11.5 and 3.12.0 → no error, `Warnings()` empty; 3.13.0 → no error, `Warnings()` has one entry naming 3.13.0, and `Next` yields the events; the snapshot, milestone and garbage versions → `*gatling.VersionError` quoting the string as written; no header, event before header and empty → `*gatling.SyntaxError` with the expected line; `surplus-field-3.13.0` → decodes with the fifth field ignored (FR-008a above range). MUST fail before T027

### Implementation for User Story 3

- [X] T027 [US3] In `gatling/text/reader.go` and `gatling/text/parse.go`: thread a `lenient bool` from the gate verdict (`VerdictUnverified`) into the per-kind parsers so a surplus field is ignored above the range and rejected inside it (FR-008a); confirm the version string is passed through `gatling.ParseVersion` untouched so a suffix refuses; export `MinVersion` and `MaxVersion` as in the contract

**Checkpoint**: every row of the Source Coverage gate table in spec.md has a passing test

---

## Phase 6: User Story 4 — A damaged log is refused, and you are told exactly where (Priority: P2)

**Goal**: the first undecodable line ends the read with its number; nothing after it is read; no input can crash the reader

**Independent Test**: `go test -race -run 'TestSyntax|TestMutation' ./gatling/text/` — every corruption position names its own line, and 10,000 mutations produce only errors

### Tests for User Story 4 (REQUIRED — write first, MUST fail before implementation) ⚠️

- [X] T028 [P] [US4] Add fixtures: `unknown-kind.fixture.log`, `user-three-fields.fixture.log`, `request-eight-fields.fixture.log` (in-range header), `error-tab-in-message.fixture.log` (an `ERROR` whose message contains two tabs), `error-newline-in-message.fixture.log` (an `ERROR` split across two lines), `truncated-last-line.fixture.log` (final line cut mid-field, no terminator), `unterminated-last-line.fixture.log` (final line complete but no terminator), `bad-timestamp.fixture.log`, `bad-status.fixture.log`, `bad-event.fixture.log`, `damage-at-5-and-900.fixture.log` (generated by a test helper from a valid template: 1,000 lines, lines 5 and 900 corrupted)
- [X] T029 [P] [US4] Write `gatling/text/syntax_test.go`: each fixture above returns a `*gatling.SyntaxError` from `Next` (or `NewReader` when the damage is in the preamble) whose `Line` is exactly the damaged line and whose `Expected`/`Found` are non-empty; for `damage-at-5-and-900` the error names 5 and no record with `Line > 4` is ever returned; every later `Next` returns the same error (C7); `request-eight-fields` fails in range (FR-008a) while `error-tab-in-message` succeeds with the message recovered whole including both tabs (FR-008b); `error-newline-in-message` fails at the continuation line; both truncated and unterminated final lines fail naming the last line, because the writer always terminates a record; a `SyntaxError` never carries partial totals. MUST fail before T030
- [X] T030 [P] [US4] Write `gatling/text/mutation_test.go`: with a seeded `math/rand/v2` PRNG apply 10,000 mutations to the fixtures and, behind the `integration` tag, to the corpus logs — single-byte flips, tab insertion and deletion, truncation at a random offset, line duplication, `\r` insertion — and assert that `NewReader`+`Next`-to-completion never panics (recover in the test harness only, never in the reader) and returns either `io.EOF` or a typed error (SC-007, FR-015); also add `FuzzReader` in the same file seeded from every fixture for local `go test -fuzz=FuzzReader` runs

### Implementation for User Story 4

- [X] T031 [US4] In `gatling/text/scan.go` and `gatling/text/reader.go`: surface the scanner's "unterminated final line" as `*gatling.SyntaxError{Expected: "line terminator", Found: "end of input"}` on that line; make sure every parse failure path sets the sticky error; ensure `Expected` and `Found` are set on every `SyntaxError` the package constructs
- [X] T032 [US4] Run `go test -race -run TestMutation -count=1 ./gatling/text/` until it is green, fixing any panic found in `parse.go` or `scan.go` with a regression fixture named for it under `gatling/text/testdata/fixtures/` (a bug fix ships its regression test — Principle III)

**Checkpoint**: fail-fast lands on the right line for every position tested, and nothing panics

---

## Phase 7: User Story 5 — A log larger than memory can still be read (Priority: P3)

**Goal**: peak memory independent of log size; chunked and whole-file reads agree; the benchmark records the baseline

**Independent Test**: `go test -tags=integration -race -run 'TestChunked|TestPeakMemory' ./gatling/text/` and `go test -tags=integration -bench=BenchmarkReader -benchmem ./gatling/text/`

### Tests for User Story 5 (REQUIRED — write first, MUST fail before implementation) ⚠️

- [X] T033 [P] [US5] Write `gatling/text/chunk_test.go`: for every fixture and, behind the `integration` tag, every corpus log, decode once from a `bytes.Reader` and once each through `iotest.OneByteReader`, `iotest.HalfReader` and a seeded random-chunk reader whose chunk sizes range 1..4096; assert the record sequences are identical by `reflect.DeepEqual` after copying `Groups`, and that a failing input fails at the same `Line` with the same `Expected` and `Found` through every reader (FR-018, C16)
- [X] T034 [P] [US5] Write `gatling/text/memory_test.go` behind `//go:build integration`: an `io.Reader` that synthesises a valid log of a requested size from a template (header, assertions, then repeated realistic events with increasing timestamps) without materialising it; decode 256 MiB while sampling `runtime.MemStats.HeapAlloc` every 4 MiB of input and assert the peak stays under 32 MiB; repeat at ten times the size (2.5 GiB; skip only when `-short`) and assert the peak does not exceed the first run's by more than 4 MiB — the slack covers garbage-collector timing, not growth (SC-004, FR-017). MUST fail before T036 if the reader retains anything per record
- [X] T035 [P] [US5] Write `gatling/text/bench_test.go` behind `//go:build integration`: `BenchmarkReader` over the largest `testdata/corpus/gatling/*/simulation.log`, reading from memory, `b.SetBytes` to the file size, `b.ReportAllocs`; the target is ≥ 100 MB/s on one core and the allocation count per record MUST be reported so a regression shows

### Implementation for User Story 5

- [X] T036 [US5] In `gatling/text/scan.go`, `gatling/text/parse.go` and `gatling/text/reader.go`: fixed-size `bufio.Reader`; no per-record allocation other than the strings the record carries; `Groups` backed by a slice reused across `Next` calls; assertion payloads the only state that grows, bounded by the simulation, and documented as such on `Assertions`
- [X] T037 [US5] Run the benchmark and record throughput (MB/s), allocs per record and peak memory in `specs/002-gatling-text-decoder/plan.md` under Technical Context as the regression baseline; if under 100 MB/s, profile with `-cpuprofile` and fix before recording

**Checkpoint**: all five stories pass independently; the baseline is written down

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: the gates the PR must pass, and the two actions outside the code that the plan flagged

- [X] T038 [P] `CHANGELOG.md`: under Unreleased → Added, one entry for the `gatling` package (record types, version gate) and one for `gatling/text` (the reader, versions accepted, fail-fast behaviour, 1 MiB line ceiling), Keep a Changelog style
- [X] T039 [P] godoc pass over `gatling/` and `gatling/text/`: every exported identifier documented; `Reader` states the versions it accepts; the `Groups` ownership rule and the sticky-error rule appear on `Next`; `go vet ./...` and `golangci-lint run` clean with zero findings and no new `//nolint`
- [X] T040 Coverage: `go test -cover ./gatling/... ` ≥ 90% for `gatling` and `gatling/text`, `go test -cover ./...` ≥ 80% overall; paste the numbers into the PR description (CI does not enforce the floors yet)
- [X] T041 [P] Dependency boundary: `go list -deps ./gatling/... | grep -vE '^[a-z0-9/]+$'` prints nothing (standard library only); `go mod tidy && git diff --exit-code go.mod go.sum` clean
- [X] T042 Run every command in [quickstart.md](quickstart.md) top to bottom on a clean checkout; fix the document, not the reader, if a step's expected output is wrong
- [X] T043 Record the corrected acceptance bullet for issue #3 in `specs/002-gatling-text-decoder/checklists/requirements.md` under the Principle II cross-check — replace "then it is reported with its line number and the read continues" with "then the read fails with an error naming its line number and no partial result is returned" — and hand that text to a maintainer to apply on the issue before the PR merges; editing the issue is a publishing action and is not done from here without confirmation
- [X] T044 Squash to one commit `feat(gatling): decode Gatling 3.11.5–3.12.0 text simulation.log (#3)` (the corpus and its `RECORDING.md` files ride in the same commit — they are the evidence for the code); open the PR against `main` with milestone v0.0.2, `Closes #3`, the coverage numbers from T040 and the benchmark numbers from T037 in the description; rebase only, `--force-with-lease` on updates

---

## Phase 9: User Story 6 — Every supported Gatling is re-run and re-checked (Priority: P2)

**Goal**: a real Gatling per supported version on every change, held to its own fresh report (issue #15, pulled into v0.0.2)

**Independent Test**: `PARSEC_CANARY_RUNS=... go test -tags=canary -race -count=1 ./gatling/text/` over two fresh runs passes and names both versions; the workflow's shell steps run locally end to end

### Tests for User Story 6 (REQUIRED — write first, MUST fail before implementation) ⚠️

- [X] T045 [US6] Move the report, tally and masked-comparison helpers out of `gatling/text/golden_test.go`, `report_test.go` and `crossversion_test.go` into `gatling/text/helpers_test.go` under `//go:build integration || canary`, leaving only the tests behind; the corpus suite must pass unchanged
- [X] T046 [US6] Write `gatling/text/canary_test.go` under `//go:build canary`: `canaryRuns` parses `PARSEC_CANARY_RUNS` ("version=dir" pairs) and skips with a reason when unset; `TestCanary` per version asserts the header names the version asked for, surfaces any gate warning to the job summary, and applies `checkCounts`/`checkRates`/`walk` against that run's `js/global_stats.json` and `js/stats.json`; `TestCanaryCrossVersion` holds the runs to each other as masked multisets; `TestCanaryCoversSupportedRange` fails when a bound of `SupportedVersions` was not run

### Implementation for User Story 6

- [X] T047 [US6] Write `.github/workflows/gatling-canary.yml`: `workflow_call` and `workflow_dispatch` with a `versions` input defaulting to the supported list, a weekly `schedule`; `setup-java` 17 with the sbt cache and `setup-sbt`; start the stub and wait for it; run the probe under every version, failing with the version named; export `PARSEC_CANARY_RUNS`; run the canary tests with `-count=1 -json` and fail when none passed
- [X] T048 [US6] Add the `canary` job to `.github/workflows/ci.yml` as `uses: ./.github/workflows/gatling-canary.yml`; add the `canary` build tag to `.golangci.yml` so the file is linted
- [X] T049 [US6] Run the workflow's shell steps locally end to end — stub, both versions under sbt, the canary tests — and confirm every version is named in the summary
- [X] T050 [US6] Document the canary in `specs/002-gatling-text-decoder/quickstart.md` and `research.md` (R14); note in `plan.md` that the constitution's gate table needs a `canary` row, which is an amendment for its own PR

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: starts immediately. T004 and T005 need Gatling installed and are the critical path — every integration test below is red until they land, and they cannot be repeated later
- **Foundational (Phase 2)**: needs T001 only; T012–T013 need nothing from Phase 1 beyond the directory
- **US1 (Phase 3)**: needs Phase 2 complete and T004 for its integration tests; its unit tests (T014, T015) need only Phase 2
- **US2 (Phase 4)**: needs US1 (T016, T017, T019) and T005
- **US3 (Phase 5)**: needs T019; independent of US2
- **US4 (Phase 6)**: needs T019; independent of US2 and US3
- **US5 (Phase 7)**: needs T019; T034 and T035 need T004 for corpus files
- **Polish (Phase 8)**: needs every story that is in scope

### Within Each Story

- Tests written and failing before the implementation task starts
- `gatling/` types before `gatling/text/` code
- `parse.go` before `reader.go`
- Golden files generated only after the decoder that produces them has passed its unit tests, and reviewed as a diff before commit

### Shared-file caution

US3 (T027), US4 (T031) and US5 (T036) all edit `gatling/text/reader.go`, `parse.go` and `scan.go`. Their **tests** are parallel; their **implementation** tasks are serial in story order to avoid conflicting edits. US2 edits no code at all.

### Parallel Opportunities

- Phase 1: T002, T003 and T006 together; T004 and T005 together once T002 and T003 exist
- Phase 2: T007, T008, T009, T010, T011 together; then T012, then T013
- US1: T014, T015, T016, T017 together, then T018, then T019
- After T019: T022, T025, T026, T028, T029, T030, T033, T034, T035 can all be written in parallel — nine test files, no shared code
- Phase 8: T038, T039, T041 together

---

## Parallel Example: User Story 1

```bash
# All four US1 test files at once — different files, all fail until parse.go and reader.go exist:
Task: "Write gatling/text/parse_test.go — every kind, every field, lone space, group split"
Task: "Write gatling/text/reader_test.go — preamble, header, assertions, Next order, io.EOF"
Task: "Write gatling/text/golden_test.go — canonical form, records.golden, -update, fails on empty corpus"
Task: "Write gatling/text/report_test.go — counts and mean rate exact against the two JSON files"

# Then, serially:
Task: "Implement gatling/text/parse.go"
Task: "Implement gatling/text/reader.go"
Task: "Generate and review testdata/corpus/gatling/3.11.5/records.golden"
```

---

## Implementation Strategy

### MVP First (User Story 1 only)

1. Phase 1 — including the recordings, which cannot wait
2. Phase 2 — types, gate, errors, scanner
3. Phase 3 — decode 3.11.5, counts and rate exact against its own report
4. **STOP and VALIDATE**: `go test -tags=integration -race ./gatling/text/` against 3.11.5 alone
5. This is a shippable reader for the version most archives were made with

### Incremental Delivery

1. Setup + Foundational → nothing decodes yet, everything compiles and the gate is unit-tested
2. US1 → 3.11.5 readable, counts proved (MVP)
3. US2 → 3.12.0 proved identical; zero code
4. US3 → all four gate outcomes tested against the reader; surplus-field leniency above range
5. US4 → fail-fast at the right line; 10,000 mutations without a panic
6. US5 → bounded memory proved at two sizes; baseline recorded
7. Polish → changelog, godoc, coverage, dependency boundary, issue #3 corrected, one commit, one PR

### One-person order (the expected case)

T001 → T002 → T003 → T004 → T005 → T007…T011 → T012 → T013 → T014…T017 → T018 → T019 → T020 → T021 → T022 → T023 → T024 → T025 → T026 → T027 → T028…T030 → T031 → T032 → T033…T035 → T036 → T037 → T038…T044

---

## Notes

- [P] = different files, no dependency on an incomplete task
- The corpus is recorded once and never edited; anything hand-written is a fixture and says so in its name
- A test that reads the corpus MUST fail when the corpus is missing, never skip — an empty integration run is a failure by constitution
- The reader never calls `recover`; only the mutation and fuzz harnesses do, in test code, to turn a panic into a failing assertion
- One tracked issue (#3) = one green semantic commit; squash the working history before review
- `Groups` reuse is a documented contract, not an accident — do not "fix" it by allocating per record, the benchmark will show why
