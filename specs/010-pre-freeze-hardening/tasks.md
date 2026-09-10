# Tasks: Pre-freeze hardening

**Input**: Design documents from `/specs/010-pre-freeze-hardening/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md),
[data-model.md](data-model.md), [contracts/](contracts/), [quickstart.md](quickstart.md)

**Tests**: REQUIRED, and written first. Constitution Principle III is non-negotiable: test tasks are
never optional, and a bug fix ships a regression test that fails without it. Every test task below
says what it fails with on `main` (`410dc54`); the two that pass on `main` say so and why they exist
anyway. The `/speckit-tasks` skill's "tests are optional" line is the known conflict the
constitution records against it, and this repository's `tasks-template.md` resolves it the same way.

**Organization**: by user story. The stories are independent in code — different packages or
different functions — and each phase ends in one commit per issue, green on its own. Two orderings
are deliberate and explained where they occur: US4 (#76) precedes US3 (#87, #75) although both are
P2, and #82 precedes #83/#102 (research R9).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an incomplete task)
- **[Story]**: which user story the task belongs to (US1–US6, as numbered in spec.md)
- Exact file paths in every description

## Path Conventions

parsec is a single Go module with packages at the repository root (plan.md, *Source Code*):

- **Packages touched**: `gatling/binary/`, `gatling/simlog/`, `gatling/text/` (one reorder),
  `gatling/run/`. No package is added.
- **Tests**: beside the code, table-driven on stdlib `testing`. External test packages
  (`binary_test`, `simlog_test`, `run_test`) unless the property needs an unexported name:
  `gatling/binary/read_test.go` is `package binary`, and `gatling/run/order_test.go` (new) is
  `package run`. Unexported constants a test needs are exposed through the package's existing
  `export_test.go`; ceilings are asserted with literals plus a comment naming the constant, so a
  test compiles — and fails — before the constant exists.
- **Fixtures**: built by the test that uses them — `builder` in `gatling/binary/build_test.go`,
  `textPreamble` in `agreement_test.go`, `textLog(...)` in `gatling/simlog`, `mkRun`/`touchLog` in
  `gatling/run/find_test.go`, `t.TempDir()` roots, `iotest` readers, a real `gzip`. Memory fixtures
  are **generated readers** in the shape of `newSynthLog` (`gatling/binary/synth_test.go`), never
  byte slices: a fixture held in the heap is measured as retention. Every one of them is a fixture,
  not corpus (Principle III).
- **Corpus**: `testdata/corpus/gatling/**`, unchanged; it is the proof nothing else moved.
- **Scratch**: baseline measurements go to the session scratchpad directory, never into the tree.

---

## Phase 1: Setup

**Purpose**: a green baseline, a throughput baseline, and the docs commit that must precede every
fix (constitution: Spec-first).

- [X] T001 [P] Confirm the branch is `010-pre-freeze-hardening` at `origin/main` (`git rev-parse HEAD origin/main` agree, or `git merge-base --is-ancestor origin/main HEAD`) and that the untouched tree is green: `go build ./... && go test -race -shuffle=on ./...`
- [X] T002 [P] Record the throughput baseline for the decoder benchmark the constitution holds a decoder change to: `go test -tags=integration -run '^$' -bench 'BenchmarkDecode$' -benchmem -count=5 ./gatling/binary/`, saved as `bench-before.txt` in the scratchpad; the after-figures are compared in T046
- [X] T003 [P] Confirm `.golangci.yml` needs no change for this feature (no new linter, no new `//nolint` category — every identity comparison on `io.EOF` reuses the existing `errorlint` exemption with its reason); if it does, justify it in the Complexity Tracking table of specs/010-pre-freeze-hardening/plan.md
- [X] T004 Commit the spec artifacts before any fix: stage exactly `specs/010-pre-freeze-hardening/` (spec, plan, research, data-model, quickstart, contracts/, checklists/, this file) and `.specify/feature.json` (the docs commits of 006–009 all carry it), assert `git diff --cached --name-only` lists nothing else, and commit from a message file as `docs(speckit): add 010-pre-freeze-hardening spec/plan/tasks` with the `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>` trailer

**Checkpoint**: `git log --oneline -1` shows the docs commit on top of `origin/main`; the tree is
clean.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: nothing in code — the seven fixes share no new type, helper or file. The one
prerequisite is the constitution's reading rule.

- [X] T005 Read the required-reading skills before the first test is written (constitution, Quality Gates → Engineering Guidance): `golang-testing` (its testify and mock sections are not followed — Principles III and IV) and the R11 record of where `golang-error-handling` is overruled, in specs/010-pre-freeze-hardening/research.md; consult `golang-safety` and `golang-security` before T035, and `golang-benchmark` before T034

**Checkpoint**: no code has changed; every story below can start.

---

## Phase 3: User Story 1 — A complete upload is never reported as a killed run (Priority: P1) 🎯 MVP

**Goal**: a source that returns its final bytes together with `io.EOF` completes the value it was
filling; a complete binary log read that way decodes exactly as from a file (#82; contract 1 §3;
research R1).

**Independent Test**: `go test -race -run 'ReadFull|CutShortWhenTheSource|ChunkedReadsMatchWholeFile' ./gatling/binary/`
— the 200,000-byte-payload log decodes identically through `bytes.Reader`, `iotest.DataErrReader`
and a one-shot source.

### Tests for User Story 1 (write first, MUST fail before T009) ⚠️

- [X] T006 [US1] Unit test in gatling/binary/read_test.go (package `binary`): add a `oneShot` source that returns all its bytes and `io.EOF` in a single `Read`; `TestReadFullAcceptsAFillThatArrivesWithEOF` asserts `readFull` over a buffer of exactly the source's length returns `(len, nil)`, and over a buffer one byte longer returns `errCutShort` with `len` bytes read — **fails on `main`**: the first case returns `errCutShort`
- [X] T007 [P] [US1] Regression test in gatling/binary/truncation_test.go: `TestACompleteLogIsNotCutShortWhenTheSourceEndsWithItsLastBytes` builds `(&builder{}).runRecord("3.15.1", []string{"s"}, []string{strings.Repeat("a", 200_000)}).request("GET /x", true).bytes()` and reads it through `bytes.NewReader` (reference), `iotest.DataErrReader(bytes.NewReader(raw))` and a one-shot source that hands everything over in one call — the payload must be the file's last bytes, and only the one-shot shape reaches the read-buffer path, `DataErrReader` handing over a kilobyte at a time; records (via `records(t, r)`) and `Assertions()` must be equal across the three; a subtest repeats it with a 1,000-byte payload (US1 scenario 2); a subtest cuts the log seven bytes short and requires a `*gatling.TruncationError` from all three sources (FR-002) — **fails on `main`**: the 200,000-byte case through the one-shot source returns "the log is cut short: 200058 trailing bytes could not be decoded"
- [X] T008 [P] [US1] Parity with the text codec's chunk matrix in gatling/binary/chunk_test.go: `TestChunkedReadsMatchWholeFile` also reads each corpus file through `iotest.DataErrReader(bytes.NewReader(raw))` and compares the records as the sized chunks are compared — **passes on `main`** (corpus fields are small); it exists so the corpus is read through this shape from now on

### Implementation for User Story 1

- [X] T009 [US1] Fix `readFull` in gatling/binary/read.go: after `n += read`, a filled buffer (`n == len(buf)`) returns `(n, nil)` when the error beside the last bytes is `io.EOF` by identity, and that error otherwise (review corrected the first version, which cleared every error as `io.ReadFull` does); the `err == io.EOF` arm becomes `errCutShort` only for a short buffer; rewrite the doc comment to state the two differences from `io.ReadFull` that remain (an end of stream becomes `errCutShort`; a stalled source ends in `io.ErrNoProgress`) and the third, which contract 1 §3 fixes: only `io.EOF` is cleared beside the last bytes of a value
- [X] T010 [US1] Verify: T006–T008 pass; `go test -race ./gatling/binary/` and `go test -tags=integration -count=1 -skip 'PeakMemory$' ./gatling/binary/` are green (every corpus entry unchanged, FR-019); the existing `TestASourceReturningUnexpectedEOFIsNotATruncation` and the stalled-source tests still pass
- [X] T011 [US1] CHANGELOG.md: under `## [Unreleased]` add `### Fixed` with the #82 entry as worded in specs/010-pre-freeze-hardening/contracts/endings.md ("What changes for a consumer"), citing `(#82)`
- [X] T012 [US1] Gate and commit: `gofmt -l .` and `gofumpt -l .` empty, `go vet ./...`, `golangci-lint run --build-tags=integration ./gatling/binary/` zero issues; stage exactly gatling/binary/read.go, gatling/binary/read_test.go, gatling/binary/truncation_test.go, gatling/binary/chunk_test.go, CHANGELOG.md; assert the staged list; commit from a message file as `fix(gatling/binary): a filled read that arrives with io.EOF is a complete value (#82)` with the trailer

**Checkpoint**: US1 is complete and independently verifiable; one commit, green on its own.

---

## Phase 4: User Story 2 — A broken source is a failure, never an ending and never "not yet" (Priority: P1)

**Goal**: every constructor reports a failing source as a failure: never `errors.Is(err, io.EOF)`
(#83), never a short head for a source's `io.ErrUnexpectedEOF` (#102); contract 1 §1–2; research R3,
R4. Two commits, each green: #83's table has two sources, #102 adds two more.

**Independent Test**: `go test -race -run 'SourceFailureIsNeverTheEndOfTheLog|WrappedEOF|UnexpectedEOF|TruncatedGzip|ShortHead' ./gatling/simlog/`
— six constructors × four sources, plus the gzip and the genuine short stream.

### Tests for #83 (write first, MUST fail before T015) ⚠️

- [X] T013 [US2] Table test in gatling/simlog/errors_test.go: `TestASourceFailureIsNeverTheEndOfTheLog` over six constructors — `simlog.NewReader`, `simlog.NewRunReader`, `binary.NewReader`, `binary.NewRunReader`, `text.NewReader`, `text.NewRunReader` — and, for now, two sources built with `iotest.ErrReader`: a plain `errors.New("transport closed")` and `fmt.Errorf("upload aborted: %w", io.EOF)`; each cell asserts `err != nil`, `!errors.Is(err, io.EOF)`, not `*gatling.TruncationError`, not `*gatling.FormatError`, and `strings.Contains(err.Error(), <cause text>)` — **fails on `main`**: the two `simlog` rows for the wrapped `io.EOF` source satisfy `errors.Is(err, io.EOF)`; the four codec rows pass and are the reference
- [X] T014 [US2] Extend `TestWrappedEOFIsAStreamFailure` in gatling/simlog/errors_test.go: keep the `errors.Is(err, broken)` assertion — the cause stays reachable, research R3 — add `!errors.Is(err, io.EOF)` and an `*fs.PathError` reached through `errors.As`, and keep the `*gatling.FormatError` assertion (review corrected the first version, which cut the chain and asserted on the text)

### Implementation for #83

- [X] T015 [US2] Fix `identify` in gatling/simlog/simlog.go: on the failure path, report a failure of the source through `source.Failed` in a new `internal/source` package, which hides `io.EOF` and keeps every other cause reachable, and route gatling/binary's `sourceFailed` and gatling/text's `readError` through it too (review corrected the first version, which cut the whole chain)
- [X] T016 [US2] Verify `go test -race ./gatling/simlog/` green; CHANGELOG.md `### Fixed` gains the #83 entry from contracts/endings.md (including the sentence that `errors.Is` no longer reaches such a cause from `simlog`, as it already did not from the codecs); gate (`gofmt`, `gofumpt`, `go vet`, `golangci-lint run --build-tags=integration ./gatling/simlog/`); stage exactly gatling/simlog/simlog.go, gatling/simlog/errors_test.go, CHANGELOG.md; commit as `fix(gatling): a source failure never satisfies errors.Is(err, io.EOF) (#83)` with the trailer

### Tests for #102 (write first, MUST fail before T018) ⚠️

- [X] T017 [US2] Extend gatling/simlog/errors_test.go: add `io.ErrUnexpectedEOF` (by identity) and `fmt.Errorf("gunzip: %w", io.ErrUnexpectedEOF)` to `TestASourceFailureIsNeverTheEndOfTheLog`'s sources; add `TestATruncatedGzipInsideTheWindowIsAFailure` — gzip `textLog("3.12.0")` with `compress/gzip`, keep the first 14 bytes, open through `gzip.NewReader` (its header parses) and `simlog.NewReader`: not a `*gatling.FormatError`, not `io.EOF`; add `TestAGenuinelyShortStreamIsStillAShortHead` — a source of two bytes then `io.EOF` (`strings.NewReader("RU")`) yields a `*gatling.FormatError` with `Short == true` from both `simlog` constructors, and a source of two bytes then `io.ErrUnexpectedEOF` (a small custom reader) yields neither a `*gatling.FormatError` nor `io.EOF` — **fails on `main`**: the `io.ErrUnexpectedEOF` rows and the gzip case come back as `FormatError{Short: true}`

### Implementation for #102

- [X] T018 [US2] Fix `readHead` and `identify` in gatling/simlog/simlog.go: add an unexported `errShortHead` sentinel; `readHead` returns it when the source's `io.EOF` (identity) arrived after some bytes and before the window was full, `nil` when the window was filled with the source's `io.EOF`, and the source's error when any other came with the last bytes (R1's rule, so the two loops agree), `io.EOF` when nothing was read, `io.ErrNoProgress` when stalled, and the source's error untouched otherwise; rewrite its doc comment to state those differences from `io.ReadFull` instead of claiming its contract; `identify` treats `err == io.EOF` and `err == errShortHead` as "the head is what we have" and everything else as a source failure through T015's path (identity comparisons keep the `//nolint:errorlint` with the reason)
- [X] T019 [US2] Verify `go test -race ./gatling/simlog/` green (T013 now over four sources, T017, the stalled-reader test unchanged) and `go test -tags=integration -count=1 -skip 'PeakMemory$' ./gatling/simlog/`; CHANGELOG.md `### Fixed` gains the #102 entry from contracts/endings.md; gate; stage exactly gatling/simlog/simlog.go, gatling/simlog/errors_test.go, CHANGELOG.md; commit as `fix(gatling/simlog): a source's io.ErrUnexpectedEOF is a failure, not a short head (#102)` with the trailer

**Checkpoint**: US1 and US2 are complete — contract 1 holds end to end; three commits.

---

## Phase 5: User Story 4 — A refused log is refused first, and the same way by both codecs (Priority: P2)

**Goal**: both codecs judge the version before the rest of the run record; a below-range or
non-release version is a `*gatling.VersionError` whatever follows; a refused binary log pulls at
most one buffer fill (#76; contract 2 "The gate, first"; data-model §2; research R5). Taken before
US3 so #75's byte accounting is written into the post-split `readRunRest` once.

**Independent Test**: `go test -race -run 'GateBeforeTheRestOfTheRunRecord|BeforeTheScenarioCount|JudgedBeforeTheRunStart|AtMostOneBufferFill' ./gatling/binary/ ./gatling/text/`

### Tests for User Story 4 (write first, MUST fail before T024) ⚠️

- [X] T020 [US4] Regression test in gatling/binary/gate_test.go: `TestAVersionBelowTheRangeIsRefusedBeforeTheScenarioCount` — `(&builder{}).u8(0).str("1.0.0").str("io.example.Sim").i64(runStart).str("").i32(-1)` returns a `*gatling.VersionError` naming 1.0.0 and the range; a second case with `str("3.x")` returns one with `Parsed == false` — **fails on `main`**: "expected the scenario count, found a count of -1"
- [X] T021 [P] [US4] Parity table in gatling/binary/agreement_test.go: `TestCodecsGateBeforeTheRestOfTheRunRecord` — for each row build the text line and the binary bytes with a run start of `math.MaxInt64` (past `gatling.MaxRunStart`), open both through their `NewReader`, and assert the **same** error type via `errors.As` on each: `1.0.0` → `*gatling.VersionError`; `3.x` → `*gatling.VersionError` with `Parsed == false`; `3.99.0` with `gatling.WithStrict()` → `*gatling.UnverifiedError`; `3.99.0` without it → `*gatling.SyntaxError` (the gate passed, the start is judged next) — **fails on `main`**: the first three rows return a `*gatling.SyntaxError` from both codecs (`failuresDiffer` would have called that agreement, which is why this table asserts the type)
- [X] T022 [P] [US4] Bytes-before-refusal test in gatling/binary/gate_test.go, with `const ReadBufferSize = readBufferSize` added to gatling/binary/export_test.go: `TestARefusedVersionPullsAtMostOneBufferFill` wraps two logs naming 1.0.0 in a counting reader — one with three scenario names, one with a 200 KiB scenario table — both padded past 64 KiB with request records; after `NewReader` refuses each, the bytes read from the source are equal and at most `binary.ReadBufferSize` — **fails on `main`**: the 200 KiB-table log pulls its whole table before the refusal
- [X] T023 [P] [US4] Text-side test in gatling/text/gate_test.go: `TestAVersionBelowTheRangeIsJudgedBeforeTheRunStart` — a `RUN` line with version `1.0.0` and start `9223372036854775807` returns a `*gatling.VersionError`; the same line with `3.12.0` still returns a `*gatling.SyntaxError` for the start — **fails on `main`**: the first case returns the start's `*gatling.SyntaxError`

### Implementation for User Story 4

- [X] T024 [US4] Split the binary run record in gatling/binary/record.go and gatling/binary/reader.go: `readVersion(r) (string, gatling.Version, error)` reads the first string and parses it, refusing a non-release with `*gatling.VersionError{Found, Min, Max}` (Parsed false) exactly as `readRun` does today; `readRunRest(r, version) (runHeader, error)` decodes the simulation class, the run start with its bounds, the description, the scenario names and the assertion payloads; `NewReader` reads the kind byte, calls `readVersion`, applies `versionPolicy.Apply` and records the warning, then calls `readRunRest`; update `NewReader`'s doc comment ("reads the version and gates on it before the rest of the run record") and `Reader`'s first paragraph
- [X] T025 [US4] Give the text codec the same two steps in gatling/text/parse.go and gatling/text/reader.go: `parseHeader` becomes `parseHeaderVersion(line, lineNo) (fields [][]byte, n int, version gatling.Version, err error)` — the field-count check stays first, then the version field is parsed (non-release → `*gatling.VersionError`) — and `parseHeaderRest(fields, version, lineNo) (gatling.Header, error)` — the run start with its bounds, class, run id, description; `finishPreamble` calls the first, applies `versionPolicy.Apply`, then the second, then the exact-count rule as today; update both functions' doc comments
- [X] T026 [US4] Verify: T020–T023 pass; `go test -race ./gatling/...` green; `TestCodecsAgreeOnMalformedInput` unchanged; `go test -tags=integration -count=1 -skip 'PeakMemory$' ./gatling/binary/ ./gatling/text/ ./gatling/simlog/` green (every corpus entry, header, assertion list and warning unchanged — US4 scenario 4); CHANGELOG.md gains `### Changed` under `[Unreleased]` with the #76 entry from contracts/binary-budget.md (both codecs, including the text-side start case); gate (`gofmt`, `gofumpt`, `go vet`, `golangci-lint run --build-tags=integration ./gatling/...`); stage exactly gatling/binary/record.go, gatling/binary/reader.go, gatling/binary/gate_test.go, gatling/binary/agreement_test.go, gatling/binary/export_test.go, gatling/text/parse.go, gatling/text/reader.go, gatling/text/gate_test.go, gatling/text/parse_test.go (its internal header table now drives the two halves through one helper), CHANGELOG.md; commit as `fix(gatling): judge the version before the rest of the run record, in both codecs (#76)` with the trailer

**Checkpoint**: US4 complete; the gate precedes every later field in both codecs; four commits.

---

## Phase 6: User Story 3 — The memory budget the binary reader documents holds for any log (Priority: P2)

**Goal**: the read buffer is the codec's own (#87), and every table the reader retains is bounded
in bytes so the documented 32 MiB holds by construction (#75); contract 2; data-model §3; research
R2, R6. Two commits, #87 then #75.

**Independent Test**: `go test -race -run 'OwnBuffer|BoundedInBytes|CountTheirHeaders|StaysReachable' ./gatling/binary/`
then `go test -tags=integration -count=1 -run 'PeakMemory$' ./gatling/binary/` — the at-the-ceilings
log is accepted under 32 MiB live heap.

### Tests for #87 (write first, MUST fail before T028) ⚠️

- [X] T027 [US3] One test for both codecs in gatling/binary/agreement_test.go: `TestCodecsReadThroughTheirOwnBuffer` — for `binary.NewReader` over a built binary log and `text.NewReader` over `textPreamble` plus enough records to pass 64 KiB, and for each caller buffer `bufio.NewReaderSize(src, 64<<10)` and `bufio.NewReader(src)`, construct the reader and assert the caller's `Buffered() == 0` — **fails on `main`** for the binary rows, which report a few thousand buffered bytes (the codec read through the caller's buffer); the text rows pass and are the reference

### Implementation for #87

- [X] T028 [US3] Fix `newReader` in gatling/binary/read.go: pass `struct{ io.Reader }{r}` to `bufio.NewReaderSize`, with the comment gatling/text/scan.go:33 carries — `bufio.NewReaderSize` hands back its argument when that is already a `*bufio.Reader` of at least the size asked for
- [X] T029 [US3] Verify T027 and `go test -race ./gatling/binary/` green; CHANGELOG.md `### Fixed` gains the #87 entry from contracts/binary-budget.md; gate; stage exactly gatling/binary/read.go, gatling/binary/agreement_test.go, CHANGELOG.md; commit as `fix(gatling/binary): read through the codec's own buffer, never the caller's (#87)` with the trailer — **superseded**: PR #111 landed the same wrapper and a two-codec test (`gatling/simlog/bufio_test.go`) on `main` while this branch was open; the commit written here was dropped at rebase

### Tests for #75 (write first, MUST fail before T032) ⚠️

- [X] T030 [US3] Boundary tests in gatling/binary/limits_test.go, ceilings written as literals with a comment naming the constant (`1 << 20` scenario bytes, `12 << 20` string-table bytes, `8 << 20` assertion bytes, 16-byte header): `TestScenarioNamesAreBoundedInBytes` — 16,384 distinct names of 48 bytes (exactly 1 MiB with headers) accepted, one more name refused with a `*gatling.SyntaxError` at that name's offset whose message names the total and the ceiling; `TestTheStringTableIsBoundedInBytes` — request records introducing 98,304 distinct names of 112 bytes (exactly 12 MiB with headers) accepted, the next introduction refused at its index; `TestAssertionPayloadsCountTheirHeaders` — eight payloads of `1<<20 - 16` bytes (exactly 8 MiB with headers) accepted, a ninth payload of one byte refused; `TestTheScenarioCountCeilingStaysReachable` — 65,536 empty names (exactly 1 MiB of headers) accepted, 65,537 refused by count — **fails on `main`**: the first two accept the over-ceiling logs and the third accepts the ninth payload
- [X] T031 [P] [US3] Memory tests in gatling/binary/memory_test.go (build tag `integration`, names ending in `PeakMemory` — CI runs that class alone and un-instrumented, research R10) over generated readers added to gatling/binary/synth_test.go in the shape of `newSynthLog` — a generator that streams a run record with N distinct scenario names of length L and P assertion payloads, then M request records each introducing a fresh string, then K records referring back: `TestDistinctScenarioNamesPeakMemory` — 48 names of `binary.MaxStringLen`, the read is refused with a `*gatling.SyntaxError` and the sampled live heap stays under 32 MiB throughout; `TestDistinctStringsPeakMemory` — request records introducing 48 fresh `MaxStringLen` strings, the same two assertions; `TestTablesAtTheirCeilingsPeakMemory` — names just under 1 MiB, payloads just under 8 MiB, distinct strings just under 12 MiB, then 100,000 referring records: **accepted**, live heap under 32 MiB, the figure logged — **the first two fail on `main`** (accepted, about 49 MiB retained); the third passes on `main` and is the proof of the figure, not a regression test

### Implementation for #75

- [X] T032 [US3] Bound the tables in gatling/binary/record.go, gatling/binary/strings.go and gatling/binary/reader.go: add `stringHeader = 16` (with the 64-bit note, research R6), `maxScenarioBytes = 1 << 20`, `maxCacheBytes = 12 << 20`; add one shared running-total helper (a small `retained` type holding total, limit and the name of what it bounds, whose `add(r *reader, at int64, n int) error` counts `stringHeader + n` and returns `r.syntax(at, what, "<total> bytes, past the ceiling of <limit>")` past the limit) and use it in `readStrings` (scenario names), `readBlobs` (assertion payloads — `maxAssertionBytes` keeps its figure and now counts headers), and `cache.read` (a `retained` field on `cache`); delete `maxCacheEntries` and its check, moving its reasoning about failure messages into `maxCacheBytes`'s doc comment; rewrite `Reader`'s "# The budget" paragraph to name the three tables, their ceilings and the stated trade-off (contract 2 "The budget"); correct `maxScenarios`'s comment (it reasoned about headers alone) and point `readBlobs`'s comment at the shared helper; `MaxStringLen`'s doc is unchanged
- [X] T033 [US3] Verify: T030 and `go test -race ./gatling/binary/` green; `go test -tags=integration -count=1 -run 'PeakMemory$' ./gatling/binary/` green including the existing `TestPeakMemory` family and `TestStringCeilingPeakMemory` / `TestAssertionCeilingPeakMemory`; `go test -tags=integration -count=1 -skip 'PeakMemory$' ./gatling/binary/` green (corpus unchanged)
- [X] T034 [US3] Measure: `go test -tags=integration -run '^$' -bench 'BenchmarkDecode$' -benchmem -count=5 ./gatling/binary/` against the T002 baseline — the expectation is noise (one addition per introduced string); a regression is justified in the pull request or fixed before T035
- [X] T035 [US3] CHANGELOG.md `### Changed` gains the #75 entry from contracts/binary-budget.md (the two ceilings, the header accounting, the removed count, the figure now holding by construction, the stated trade-off); gate (`gofmt`, `gofumpt`, `go vet`, `golangci-lint run --build-tags=integration ./gatling/binary/`); stage exactly gatling/binary/record.go, gatling/binary/strings.go, gatling/binary/reader.go, gatling/binary/export_test.go (the ceilings and the header, exported for the boundary tests), gatling/binary/limits_test.go, gatling/binary/memory_test.go, gatling/binary/synth_test.go, CHANGELOG.md; commit as `fix(gatling/binary): bound the scenario names and the string table in bytes (#75)` with the trailer

**Checkpoint**: US3 and US4 complete — contract 2 holds; six commits.

---

## Phase 7: User Story 5 — The run that is found does not depend on the order a directory is listed (Priority: P2)

**Goal**: `run.Find`'s tie-break is a total order over one key, so a mixed root returns the run with
the latest stamp whatever the read order (#88; contract 3; data-model §4; research R7).

**Independent Test**: `go test -race -run 'MixedStampedAndUnstamped|IndependentOfCandidateOrder|TieBreak' ./gatling/run/`

### Tests for User Story 5 (write first, MUST fail before T038) ⚠️

- [X] T036 [US5] Regression test in gatling/run/find_test.go: `TestFindMixedStampedAndUnstampedNames` — `mkRun` the three directories `simA-20990101000000000`, `simB-20200101000000000`, `simAA` in a `t.TempDir()` root, `touchLog` each to one instant, and require `Find(root)` to return `simA-20990101000000000` with `Found == run.FoundByNewest` — **fails on `main`**: returns `simB-20200101000000000`
- [X] T037 [P] [US5] Property test in a new gatling/run/order_test.go (package `run`, the package's first internal test file): `TestNewestIsIndependentOfCandidateOrder` calls `newest` over every permutation of the three `runDir` values above (equal `mod`) and over a thousand `rand.Shuffle`s of a random candidate set mixing stamped and unstamped names across a few distinct times, requiring one answer per set — written against `newest`, which exists on `main`, so it compiles and **fails on `main`** with two different answers for the mixed triple

### Implementation for User Story 5

- [X] T038 [US5] Fix the ordering in gatling/run/find.go: replace `later` with `compareRuns(a, b runDir) int` — `a.mod.Compare(b.mod)`, then `strings.Compare` on `runStart`'s stamp or `""` when absent, then `strings.Compare` on the names — and make `newest` return `slices.MaxFunc(runs, compareRuns).name`; move `later`'s block comment to `compareRuns`, adding the sentence that an unstamped name ranks below every stamped one because a stamp is the only evidence of a start time; update `FoundByNewest`'s doc comment (gatling/run/find.go:77–99) to state the full rule
- [X] T039 [US5] Verify: T036, T037 and the existing tie-break tests (`TestFindTieBreakAcrossSimulationIds`, `TestFindTieBreakWithoutRunIDStamp`, `TestFindIgnoresDirectoryMtime`) pass under `go test -race ./gatling/run/`; `go test -tags=integration -count=1 ./gatling/run/` green (the `lastrun` recording still resolves to the middle run); `BenchmarkFind` allocations unchanged (`go test -run '^$' -bench 'BenchmarkFind$' -benchmem ./gatling/run/`)
- [X] T040 [US5] CHANGELOG.md `### Changed` gains the #88 entry from contracts/run-ordering.md (the rule spelled out, the unstamped-lowest clause, the one shape whose answer changes); gate (`gofmt`, `gofumpt`, `go vet`, `golangci-lint run --build-tags=integration ./gatling/run/`); stage exactly gatling/run/find.go, gatling/run/find_test.go, gatling/run/order_test.go, CHANGELOG.md; commit as `fix(gatling/run): order candidates by one key, so the newest run does not depend on directory order (#88)` with the trailer

**Checkpoint**: US5 complete — contract 3 holds; seven fix commits on top of the docs commit.

---

## Phase 8: User Story 6 — The freeze is enforced, and certified on a supported toolchain (Priority: P3)

**Goal**: verify, do not rebuild — the compat gate landed in PR #109 (#106) and the toolchain pin is
PR #110 (#104); research R8.

**Independent Test**: `bash scripts/check-compat_test.sh`; `gh pr view 110 --json state`.

- [X] T041 [P] [US6] Run the shell-gate suite as `quick` does — `for t in scripts/*_test.sh .claude/hooks/*_test.sh .githooks/*_test.sh; do bash "$t"; done` — and confirm scripts/check-compat_test.sh reports the two cases US6 scenarios 2–3 name: "an empty report exits 2 and says the gate did not run" and "labelled breaking with an empty heading under [Unreleased] fails"; record the result in the completion report
- [X] T042 [P] [US6] Check PR #110 (`rtk proxy gh pr view 110 --json state,mergedAt,mergeCommit`): if merged, `git fetch origin && git rebase origin/main`, confirm `grep -E '^(go|toolchain) ' go.mod` shows `go 1.25` and `toolchain go1.27.1`, and re-run `go build ./... && go test ./...`; if still open, record "FR-017 pending on PR #110" in the completion report and touch nothing in go.mod — this branch never edits it (checked 2026-09-10: PR #110 open, `go.mod` untouched; merged later that day as `d663ca3` with `toolchain go1.26.8`, the branch rebased onto it and re-verified, FR-017 satisfied)

**Checkpoint**: both machinery issues verified; nothing built for them here.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: the full gate set, the measurements the plan promised, and the commit shape the rules
require.

- [X] T043 [P] Doc review against the contracts: `binary.Reader` (the budget paragraph), `binary.NewReader` (the gate first), `readFull` and `newReader` comments, `readHead` and `identify` comments, `run.FoundByNewest` and `compareRuns` — each states the behaviour as contracts/endings.md, contracts/binary-budget.md and contracts/run-ordering.md word it; `go doc ./gatling/binary Reader` reads correctly
- [X] T044 [P] CHANGELOG.md `[Unreleased]` complete and in Keep a Changelog order — `### Changed` (#76, #75, #88) before `### Fixed` (#82, #83, #102, #87) — every entry citing its issue; no entry for #106 or #104 (neither ships module behaviour, and #104 belongs to PR #110)
- [X] T045 Full gate: `test -z "$(gofmt -l .)"`, `gofumpt -l .` empty, `go vet ./...`, `go mod tidy && git diff --exit-code`, `go build ./...`, `golangci-lint run --build-tags=integration ./...` zero issues, `go test -race -shuffle=on ./...`, `go test -tags=integration -count=1 -skip 'PeakMemory$' ./...`, `go test -tags=integration -count=1 -run 'PeakMemory$' ./...`, and the shell-gate suite; `go list -deps ./model/... ./gatling/...` names nothing outside the standard library
- [X] T046 Coverage and throughput for the pull-request descriptions and the plan: `go test -tags=integration -count=1 -skip 'PeakMemory$' -coverpkg=./... -coverprofile=cover.out ./... && bash scripts/check-coverage.sh --enforce cover.out` (90% decoder packages, 80% overall; note `gatling/binary`, `gatling/simlog`, `gatling/text`, `gatling/run` and the total); `BenchmarkDecode` after (five runs) beside the T002 baseline, via `benchstat` if installed; write both into a "Measured" block in the Technical Context of specs/010-pre-freeze-hardening/plan.md, as 009's plan did
- [X] T047 Each commit green on its own: for each of the seven fix commits (`git log --format=%h origin/main..HEAD`, skipping the docs commit), `git worktree add <scratchpad>/c-<sha> <sha>`, run `go build ./... && go test ./gatling/...` there, then `git worktree remove` it; record the seven results
- [X] T048 Fold the plan's measurement update into the docs commit so the history stays "docs first, then seven fixes": `git commit --fixup=<docs commit sha>` with only specs/010-pre-freeze-hardening/plan.md staged, then `git rebase --autosquash <docs commit sha>^` (Git 2.50 applies autosquash without `-i`); confirm `git log --oneline origin/main..HEAD` lists the docs commit followed by the seven fix commits in R9's order, and `git status` is clean
- [X] T049 Run specs/010-pre-freeze-hardening/quickstart.md sections 1–7 end to end as the final validation and note any command whose name pattern no longer matches a test, correcting quickstart.md in the docs commit via the same fixup route

---

## Phase 10: Publication — gated on the maintainer

**Purpose**: what leaves the machine. Pushing a branch and opening pull requests are outward-facing
and are **not** performed by `/speckit-implement` without the maintainer's explicit go-ahead; the
implementation report stops here and asks.

- [X] T050 Push `010-pre-freeze-hardening` (`git push -u origin 010-pre-freeze-hardening`) — with the maintainer's go-ahead
- [X] T051 Open the pull requests research R9 fixes, each with milestone `v0.0.10 Pre-freeze hardening`, `Fixes #NNN` lines, the coverage and benchmark figures from T046, and the generated footer: the docs pull request (the docs commit alone); **PR A** contract 1 (#82, #83, #102) stacked on it; **PR B** contract 2 (#76, #87, #75) stacked on A; **PR C** contract 3 (#88); one branch per pull request cut at its last commit; `scripts/check-linkage.sh --pr N` green for each — with the maintainer's go-ahead; done as one pull request for the branch, #112, as the maintainer asked, whose commit order still allows the R9 stack to be cut from it
- [ ] T052 After each merge: rebase the stack with `--force-with-lease`, confirm the issues closed, and when the last lands run `scripts/check-linkage.sh` with no argument to see the milestone's remaining open items

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: T001–T003 in parallel; T004 last, and before any code.
- **Foundational (Phase 2)**: T005 only; blocks nothing but the first test.
- **Stories (Phases 3–8)**: independent in code. The order taken — US1 → US2 → US4 → US3 → US5 →
  US6 — is research R9's commit order; two constraints inside it are real: #82 (US1) before #83/#102
  (US2), so the two fill loops are equal from the first commit that touches either; #76 (US4) before
  #75 (US3), so the byte accounting is written into `readRunRest` once.
- **Polish (Phase 9)**: after every story; T043–T044 in parallel, then T045 → T046 → T047 → T048 →
  T049.
- **Publication (Phase 10)**: only with the maintainer's go-ahead.

### Story Dependencies

| Story | Depends on | Independently testable by |
|---|---|---|
| US1 (#82) | — | T007's three source shapes against `bytes.Reader` |
| US2 (#83, #102) | #82 landed (equal loops) | T013 over six constructors × four sources |
| US4 (#76) | — | T021's parity table |
| US3 (#87, #75) | #76 landed (accounting written once) | T027; T030; the at-the-ceilings memory test |
| US5 (#88) | — | T036 and T037 |
| US6 (#106, #104) | — (verification) | T041, T042 |

### Within Each Story

- Tests first; every regression test named above fails on `main` with the message the task quotes.
- One commit per issue at the end of its sub-phase, staged by explicit file list and asserted; the
  `CHANGELOG.md` entry travels in that commit (Principle V: same change).
- The gate before every commit: `gofmt`, `gofumpt`, `go vet`, `golangci-lint` on the packages
  touched; the race-detected package tests; the corpus suite for the package.

### Parallel Opportunities

- Phase 1: T001, T002, T003.
- US1: T007 and T008 beside T006 (three files).
- US4: T021, T022, T023 beside T020 (four files, two packages).
- US3: T031 beside T030 (two files).
- US5: T037 beside T036.
- US6: T041 and T042.
- Polish: T043 and T044.

---

## Parallel Example: User Story 4

```bash
# Write the four tests together — four files, two packages:
Task: "T020 gatling/binary/gate_test.go — 1.0.0 with a corrupt scenario count → *VersionError"
Task: "T021 gatling/binary/agreement_test.go — the codec-parity table over the run start"
Task: "T022 gatling/binary/gate_test.go + export_test.go — at most one buffer fill before a refusal"
Task: "T023 gatling/text/gate_test.go — 1.0.0 beside a bad run start → *VersionError"

# Then the two implementations — one per codec — and the single commit:
Task: "T024 gatling/binary/record.go, reader.go — readVersion / readRunRest around the gate"
Task: "T025 gatling/text/parse.go, reader.go — parseHeaderVersion / parseHeaderRest around the gate"
```

---

## Implementation Strategy

### MVP First (the two P1 stories)

1. Phase 1, then T005.
2. US1 (#82): the backend's ingest path stops reporting complete uploads as killed runs.
3. US2 (#83, #102): a torn transport is a failure from every constructor.
4. **STOP and VALIDATE**: contract 1 holds; three commits, each green; quickstart §1–2.

### Incremental Delivery

1. US4 then US3 → contract 2: the gate first, the buffer the codec's, the budget by construction.
2. US5 → contract 3: the ordering total.
3. US6 → both machinery issues verified, FR-017 closed or recorded as pending on PR #110.
4. Polish → the full gate, the measurements in the plan, seven commits each green alone.
5. Publication → with the maintainer's go-ahead: the docs pull request, then A, B, C.

### Parallel Team Strategy

Three people could take contracts 1, 2 and 3 as branches off the docs commit — the packages are
disjoint — provided #82 precedes #83/#102 inside contract 1 and #76 precedes #75 inside contract 2.

---

## Commit Mapping (AGENTS.md: 1 issue = 1 commit)

| # | Commit | Tasks | Files |
|---|---|---|---|
| 0 | `docs(speckit): add 010-pre-freeze-hardening spec/plan/tasks` | T004 (+ T048 fixup) | specs/010-pre-freeze-hardening/**, .specify/feature.json |
| 1 | `fix(gatling/binary): a filled read that arrives with io.EOF is a complete value (#82)` | T006–T012 | gatling/binary/{read.go, read_test.go, truncation_test.go, chunk_test.go, export_test.go}, CHANGELOG.md |
| 2 | `fix(gatling): a source failure never satisfies errors.Is(err, io.EOF) (#83)` | T013–T016 | internal/source/source.go, gatling/simlog/{simlog.go, errors_test.go}, gatling/binary/read.go, gatling/text/reader.go, CHANGELOG.md |
| 3 | `fix(gatling/simlog): a source's io.ErrUnexpectedEOF is a failure, not a short head (#102)` | T017–T019 | gatling/simlog/{simlog.go, errors_test.go}, CHANGELOG.md |
| 4 | `fix(gatling): judge the version before the rest of the run record, in both codecs (#76)` | T020–T026 | gatling/binary/{record.go, reader.go, strings.go, gate_test.go, agreement_test.go, export_test.go}, gatling/text/{parse.go, parse_test.go, reader.go, gate_test.go}, CHANGELOG.md |
| 5 | *landed on `main` first as PR #111 (`afa9798`); the duplicate written here (T027–T029) was dropped at rebase* | T027–T029 | — |
| 6 | `fix(gatling/binary): bound the scenario names and the string table in bytes (#75)` | T030–T035 | gatling/binary/{record.go, strings.go, reader.go, limits_test.go, memory_test.go, synth_test.go}, CHANGELOG.md |
| 7 | `fix(gatling/run): order candidates by one key, so the newest run does not depend on directory order (#88)` | T036–T040 | gatling/run/{find.go, find_test.go, order_test.go}, CHANGELOG.md |

#106 has its commit already (`410dc54`, PR #109); #104 gets its own from PR #110. Every commit
carries the `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>` trailer and is written from
a message file. Pull requests: docs; A = 1–3; B = 4 and 6; C = 7 (research R9).

## Gate Summary (filled in by T045–T047, 2026-09-10)

| Gate | Result |
|---|---|
| Format | `gofmt -l .` clean; `gofumpt -l .` names three pre-existing files this feature does not touch |
| Module hygiene | `go mod tidy && git diff --exit-code` clean |
| Vet / Build | `go vet ./...` and `go build ./...` clean |
| Lint | `golangci-lint` 2.12.2 `--build-tags=integration ./...`: 0 issues |
| Tests | `go test -race -shuffle=on ./...`: every package ok |
| End-to-end | `go test -tags=integration -skip 'PeakMemory$' ./...`: every package ok |
| Bounded memory | `go test -tags=integration -run 'PeakMemory$' ./...`: ok (binary 52.8 s, text 24.5 s) |
| Dependency boundary | `go list -deps` over `model/`, `gatling/`: standard library only |
| Coverage | gatling 98.1%, binary 98.9%, run 96.4%, simlog 91.3%, text 97.6%, model 96.8%, overall 94.9% |
| Benchmark | `BenchmarkDecode` corpus 276.5 → 281.6 MB/s, synthetic 239.0 → 242.5 MB/s; +1 alloc/op from #87's wrapper, per reader |
| Each commit alone | seven worktree runs, all green |
| Shell gates | five suites, all rc=0; the compat suite carries the empty-report and breaking-label cases |
| PR #110 (#104) | merged (`d663ca3`, `toolchain go1.26.8`) while the gates ran; the branch is rebased onto it and re-verified under that toolchain |

## Notes

- [P] tasks = different files, no dependencies
- Verify every test fails on `main` before its fix, with the message the task quotes
- One tracked issue = one semantic commit, green on its own (`go build ./... && go test ./...`)
- Stage by explicit file list and assert `git diff --cached --name-only` before every commit
- Nothing in `go.mod`, `.github/`, `.specify/memory/` or `internal/` changes on this branch
- Stop at any checkpoint to validate the story independently
