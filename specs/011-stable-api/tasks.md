# Tasks: A stable API

**Input**: Design documents from `/specs/011-stable-api/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md),
[data-model.md](data-model.md), [contracts/](contracts/), [quickstart.md](quickstart.md)

**Tests**: REQUIRED, and written first. Constitution Principle III is non-negotiable: test tasks are
never optional, and a fix ships a regression test that fails without it. Every test task below says
what it fails with on the base commit `fa988e9` (`docs(speckit): amend constitution to v2.4.0`,
itself on `origin/main` `0a55f09`); the few that pass on the base say so and why they exist anyway.
This repository's `tasks-template.md` already resolves the `/speckit-tasks` skill's "tests are
optional" line the same way the constitution does.

**Organization**: by user story, in the priority order spec.md assigns. Sixteen tracked issues map to
sixteen semantic commits, each green on its own. Three orderings are forced and are marked where they
occur: **#58 before #77** (both rewrite both `NewRunReader` bodies), **#107 before #84** (the
`UnsupportedFormatError` reword is what gives #84 an honest error to return), and **#107 before #77**
(the golden inventory must exist before a commit updates it).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel — different files, no dependency on an incomplete task
- **[Story]**: which user story the task belongs to (US1–US8, as numbered in spec.md)
- Exact file paths in every description

## Path Conventions

parsec is a single Go module with packages at the repository root (plan.md, *Source Code*):

- **Packages touched**: `model/`, `gatling/`, `gatling/text/`, `gatling/binary/`, `gatling/simlog/`,
  `gatling/run/`, `internal/wire/`. No package is added and none is renamed.
- **Tests**: beside the code, table-driven on stdlib `testing`. The one new file is `api_test.go` at
  the module root, in package `parsec_test`, because it must see all six packages at once and belongs
  to none of them.
- **Golden data**: `testdata/api/surface.txt` — the exported-identifier inventory. The Gatling corpus
  under `testdata/corpus/` is **unchanged**: this feature records nothing and only reads
  `3.11.5/simulation.log` and `3.14.9/simulation.log` as wrong-codec fixtures.
- **Every surface-changing commit** carries the `breaking` pull-request label and a `[Unreleased]`
  bullet under `### Changed` or `### Removed`, or `scripts/check-compat.sh` refuses it (research R2).

---

## Phase 1: Setup

**Purpose**: a green baseline, the numbers the freeze is measured against, and the docs commit that
must precede every change (constitution: Spec-first).

- [X] T001 [P] Confirm the branch is `011-stable-api` at `fa988e9` with `origin/main` as its ancestor (`git merge-base --is-ancestor origin/main HEAD`), that `fa988e9` is the constitution amendment this plan's Constitution Check cites as v2.4.0, and that the untouched tree is green: `go build ./... && go test -race -shuffle=on ./...`
- [X] T002 [P] Record the baseline the freeze is measured against, into the scratchpad as `surface-before.txt`: the exported-identifier inventory per package (`model` 111, `gatling` 107, `gatling/text` 14, `gatling/binary` 14, `gatling/simlog` 16, `gatling/run` 16; total 278) and `go doc -all` for each of the six packages — T009's generator must reproduce the 278 exactly before it removes anything
- [X] T003 [P] Confirm `.golangci.yml` needs no change for this feature: `api_test.go` is stdlib-only, no new `//nolint` category is introduced, and `testpackage` is satisfied by `package parsec_test`; if any of that is false, justify it in the Complexity Tracking table of [plan.md](plan.md)
- [X] T004 Commit the spec artifacts before any code change: stage exactly `specs/011-stable-api/` (spec, plan, research, data-model, quickstart, `contracts/`, `checklists/`, this file) and `.specify/feature.json`, assert `git diff --cached --name-only` lists nothing else, and commit from a message file as `docs(speckit): add 011-stable-api spec/plan/tasks` with the `Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>` trailer

**Checkpoint**: `git log --oneline -2` shows the docs commit on top of the constitution amendment; the
tree is clean.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: nothing in code. The sixteen changes share no new type, helper or file except the
inventory, which belongs to #107 and is built in its own commit. The one prerequisite is the
constitution's reading rule.

- [X] T005 Read the required-reading skills before the first test is written (constitution v2.4.0, Quality Gates → Engineering Guidance): `golang-naming` (identifiers are withdrawn and one is added), `golang-error-handling` (#84 gives `UnsupportedFormatError` producers), `golang-testing` (six structural test repairs in #95), `golang-documentation` (doc comments change in all six packages) and `golang-structs-interfaces` (the two `simlog` interfaces freeze two-sided); consult `golang-safety` and `golang-security` before Phase 10 (#93). Record in [research.md](research.md) R18 any point where a skill and the constitution disagree — as of v2.4.0 there is no prohibition to record against, and the testify judgement for this feature is already written there

**Checkpoint**: no code has changed; every story below can start.

---

## Phase 3: User Story 1 — The promise is a list, and nothing outside it is reachable (Priority: P1) 🎯 MVP

**Goal**: the frozen surface is a generated list, not a description; the two identifiers that should
not be frozen are gone; the three questions #13 left open are answered in the doc comments of the
identifiers they govern (#107, #13; [contracts/public-api.md](contracts/public-api.md); research R1,
R3, R4).

**Independent Test**: `go test -run TestExportedSurface ./...` is green, `tail -1
testdata/api/surface.txt` reads `TOTAL 276`, and `go doc -all ./gatling | grep -E '\bGate\b|MaxRunStart'`
prints nothing.

### Tests for #107 (write first, MUST fail before T012) ⚠️

- [X] T006 [US1] New test in api_test.go (package `parsec_test`): `TestExportedSurface` walks `model`, `gatling`, `gatling/text`, `gatling/binary`, `gatling/simlog` and `gatling/run` with `go/parser` (test files excluded, `parser.ParseDir` with a filter on `_test.go`), collects every exported func, method on an exported receiver, interface method, type, struct field, const and var, renders them sorted under a `## <package>  (<count>)` header exactly as [contracts/public-api.md](contracts/public-api.md) shows, and compares byte for byte with `testdata/api/surface.txt`; on a mismatch it prints the diff and names the file to regenerate — **fails on the base**: the golden file does not exist
- [X] T007 [P] [US1] In the same file, `TestExportedSurfaceCountsAreRecorded` asserts the per-package counts in the golden file parse as integers and sum to the `TOTAL` line, so a hand-edit that adds a name without touching a count is caught — **fails on the base** for the same reason
- [X] T008 [P] [US1] In the same file, `TestGateAndMaxRunStartAreNotExported` asserts neither `Gate` nor `MaxRunStart` appears in the `gatling` section of the inventory, as a named regression for the two withdrawals rather than a silent line in a golden file — **fails on the base**: both are exported (`gatling/version.go:103`, `gatling/record.go:108`)
- [X] T009 [P] [US1] Regression test for the gate's behaviour after the rename, in gatling/version_test.go: the table that called `gatling.Gate(tt.found, minV, maxV)` directly at `:118` drives `Policy.Apply` instead and asserts the same three verdicts, so the rule stays pinned through the entry point `gatling/policy.go` documents as "the single place the outcomes are decided" — **passes on the base** once rewritten, and exists so the unexporting in T011 does not lose coverage

### Implementation for #107

- [X] T010 [US1] Unexport the gate in gatling/version.go: `Gate` becomes `gate`, keeping its doc comment as an unexported one; `gatling/policy.go:41` calls `gate(...)`; nothing else changes about what it decides
- [X] T011 [US1] Move the run-start ceiling to internal/wire/wire.go as `MaxRunStart`, with its doc comment; delete it from gatling/record.go; update `gatling/binary/record.go:147` and `gatling/text/parse.go:483,487` to `wire.MaxRunStart`, and the comment reference in gatling/text/parse_test.go:51. The refusal stays documented on `binary.NewReader` and `text.NewReader`, so a consumer still learns the ceiling from the API it calls
- [X] T012 [US1] Generate testdata/api/surface.txt from the generator in T006, once T010 and T011 have withdrawn the two identifiers it must not list: `TOTAL 276` (`model` 111, `gatling` 105, `gatling/text` 14, `gatling/binary` 14, `gatling/simlog` 16, `gatling/run` 16). Verify against T002's baseline that the pre-withdrawal run reproduces 278 exactly, per package — if it does not, the generator is wrong and the count is not the issue's
- [X] T013 [US1] State the three decisions in the doc comments of the identifiers they govern (contracts/public-api.md, *The three decisions #13 left open*): `gatling.UnsupportedFormatError` — kept because a Gatling log this reader does not decode needs an error that is not `FormatError`, with the message reworded per [contracts/wrong-format.md](contracts/wrong-format.md) and the "no input produces one today" sentence removed, since #84 gives it producers; `gatling.SyntaxError` — `Format` is the discriminator two legitimately-zero positions need, which is why the position stays three fields; `gatling.Record.Line` — 0 for every record of a binary log is the contract, because that offset serves no seek and a failure's position is carried by `SyntaxError.Offset`
- [X] T014 [US1] State the two-sided freeze on gatling/simlog/simlog.go: the doc comments of `RecordReader` and `RunReader` say their method sets are final from v0.1.0, because a consumer's test double implements them and an added method breaks the implementer, not only the caller
- [X] T015 [US1] CHANGELOG.md: under `## [Unreleased]` add `### Removed` with the `gatling.Gate` and `gatling.MaxRunStart` entries as [contracts/public-api.md](contracts/public-api.md) words them, and `### Changed` for the `UnsupportedFormatError` message, each citing `(#107)`
- [X] T016 [US1] Gate and commit #107: `gofmt -l .` and `gofumpt -l .` empty, `go vet ./...`, `golangci-lint run ./...` zero issues, `go test -race ./...` green; stage exactly api_test.go, testdata/api/surface.txt, gatling/version.go, gatling/policy.go, gatling/version_test.go, gatling/record.go, gatling/errors.go, gatling/simlog/simlog.go, internal/wire/wire.go, gatling/binary/record.go, gatling/text/parse.go, gatling/text/parse_test.go, CHANGELOG.md; assert the staged list; commit from a message file as `refactor(gatling): withdraw Gate and MaxRunStart before the surface freezes (#107)` with the trailer. The pull request carries the `breaking` label

### Implementation for #13

- [X] T017 [US1] Declare the contract in CHANGELOG.md under `## [Unreleased]`: a compatibility statement naming what becomes stable at v0.1.0 (the identifiers [contracts/public-api.md](contracts/public-api.md) lists), what a version bump means below v1 (a breaking change needs a MINOR bump, a changelog entry and a `// Deprecated:` window of at least one MINOR release), that nothing outside the list is public, and the supported Gatling range — 3.11.5 through 3.12.0 (text), 3.13.1 through 3.15.1 (binary), read from `simlog.Supported()` rather than prose. Stage exactly CHANGELOG.md; commit as `docs: declare the v0.1.0 stability contract (#13)` with the trailer

**Checkpoint**: US1 complete; two commits, each green on its own. The inventory now exists, and every
later commit that changes the surface updates it.

---

## Phase 4: User Story 2 — One name, one rule, one copy (Priority: P1)

**Goal**: a consumer branching on the tool, rendering an enum it received as an integer, or reading a
version warning gets one answer whichever package it arrived through (#58, #77, #78;
[contracts/enum-rendering.md](contracts/enum-rendering.md); research R5, R6, R7).

**Independent Test**: `grep -rn 'Tool = "gatling"' --include='*.go' .` finds one declaration;
`go test -run 'TestEnumRendering|TestWarningsAgree' ./model/ ./gatling/ ./gatling/run/ ./gatling/text/ ./gatling/binary/`
is green; `diff` of the two `NewRunReader` bodies shows only the constructor call and the capability
set.

### Tests for #58 (write first, MUST fail before T020) ⚠️

- [X] T018 [US2] New test in internal/wire/wire_test.go: `TestWarningsBuildsOneReasonForEveryCodec` calls `wire.Warnings(ws, oldest, newest)` with one above-range warning and asserts the `model.Warning` carries `Version` as the release string and the reason `"no recording covers it — the verified range is <oldest> through <newest>, so the records decode unverified"`; a second case with no warnings asserts a **nil** slice, not an empty one, so `len(w) == 0` and `w == nil` keep agreeing as both codecs document — **fails on the base**: `wire.Warnings` does not exist
- [X] T019 [P] [US2] Cross-codec test in gatling/binary/agreement_test.go (package `binary_test`, which already imports both codecs and `model` and carries the `textPreamble` fixture this needs): `TestBothCodecsWordTheSameWarningIdentically` builds a text log and a binary log naming the same above-range version, opens each through its own `NewRunReader`, and asserts `Run().Warnings` are deeply equal — **passes on the base** (the two copies are byte-identical today) and exists so the property is pinned once the prose lives in one place

### Implementation for #58

- [X] T020 [US2] Add to internal/wire/wire.go: `Warnings(ws []gatling.Warning, oldest, newest gatling.Version) []model.Warning` carrying the loop, the comment explaining why the reason names neither the version nor the package, and the nil-when-empty behaviour; and `Run(h gatling.Header, caps model.Capabilities, ws []model.Warning, assertions [][]byte) model.Run` carrying the whole `model.Run` literal, with the `ID` comment stated once (the identifier is the simulation's, and `Start` is what tells two runs apart)
- [X] T021 [US2] Rewrite `NewRunReader` in gatling/text/model.go and gatling/binary/model.go to call `wire.Warnings` and `wire.Run`; `Capabilities` stays per-codec and is passed in, as #58 and #77 both leave deliberately. `gatling/text`'s existing tests must pass **unedited**, as they did for the v0.0.5 extraction
- [X] T022 [US2] CHANGELOG.md: `### Changed` entry for #58 — the warning a consumer reads is now built in one place, so the two formats cannot drift apart in wording
- [X] T023 [US2] Gate and commit #58: full gate as T016; stage exactly internal/wire/wire.go, internal/wire/wire_test.go, gatling/text/model.go, gatling/binary/model.go, gatling/binary/agreement_test.go, CHANGELOG.md; assert the staged list; commit as `refactor(internal/wire): build the run header and its warnings once, for both codecs (#58)`

### Tests for #77 (write first, MUST fail before T025) ⚠️

- [X] T024 [US2] Test in gatling/tool_test.go (package `gatling_test`): `TestToolIsNamedOnce` asserts `gatling.Tool == "gatling"` and that a consumer can name the tool without importing either codec (the test file imports only `gatling`); extend the api_test.go inventory expectation by regenerating testdata/api/surface.txt to `TOTAL 275` — **fails on the base**: `gatling.Tool` does not exist and the inventory carries two codec-local `const Tool`

### Implementation for #77

- [X] T025 [US2] Add `Tool` to gatling/, beside `Header` and `Version`, with the doc comment both copies carried ("Tool is what this source is called in [model.Run].Tool"); delete `gatling/text/model.go:83` and `gatling/binary/capability.go:9`; `wire.Run` reads `gatling.Tool` itself, so neither codec names it any more (this is why #58 lands first — research R5)
- [X] T026 [US2] Regenerate testdata/api/surface.txt: `TOTAL 275`, `gatling` 106, `gatling/text` 13, `gatling/binary` 13
- [X] T027 [US2] CHANGELOG.md: `### Removed` for `text.Tool` and `binary.Tool`, `### Added` for `gatling.Tool`, citing `(#77)` — Principle V allows the removal without a deprecation window only while the module is below v0.1.0, and says so
- [X] T028 [US2] Gate and commit #77: full gate; stage exactly gatling/tool.go (or the file the constant lands in), gatling/tool_test.go, gatling/text/model.go, gatling/binary/capability.go, internal/wire/wire.go, testdata/api/surface.txt, CHANGELOG.md; assert the staged list; commit as `refactor(gatling): one exported Tool constant for both codecs (#77)`. The pull request carries the `breaking` label

### Tests for #78 (write first, MUST fail before T030) ⚠️

- [X] T029 [US2] Table test walking all eleven exported enums at a value outside the known set, in model/enum_test.go and gatling/enum_test.go and gatling/run/find_test.go (each package tests its own, so no package imports another's test helper): `gatling.Kind(99)`, `Format(99)`, `Verdict(99)`, `Status(99)`, `Event(99)`, `run.FoundBy(99)`, `model.Outcome(99)`, `ItemKind(99)`, `PositionKind(99)`, `UserEventKind(99)`, `Field(9999)` must each render as the type name and the number; a second table walks the **zero** value of all eleven and requires `"unknown"`, unchanged — **fails on the base**: the five `model` enums render `"unknown"` for an out-of-range value (verified: `model.Outcome(99)` = `"unknown"`, `model.Field(9999)` = `"unknown"`)

### Implementation for #78

- [X] T030 [US2] Convert the seven switch-based `String()` methods to the bounds-checked table idiom the other four already use: model/sample.go (`Outcome`, `UserEventKind`), model/run.go (`ItemKind`), model/position.go (`PositionKind`), gatling/version.go (`Verdict`), gatling/record.go (`Status`, `Event`). `model.Field` keeps its `>= fieldCount` form, which is the same check written against its sentinel. `unknownName` stays in all three packages that declare it — each is unexported and package-local
- [X] T031 [US2] Every affected `String()` doc comment states the rule: the named values, `"unknown"` for the zero value, and the type name with the number for anything else
- [X] T032 [US2] CHANGELOG.md `### Changed` for #78, and gate and commit: full gate; stage exactly model/{sample.go,run.go,position.go,capability.go,enum_test.go}, gatling/{version.go,record.go,format.go,enum_test.go}, gatling/run/{find.go,find_test.go}, CHANGELOG.md; assert the staged list; commit as `refactor: render an out-of-range enum as the type name and the number (#78)`. The pull request carries the `breaking` label — `String()` output on an exported type is observable behaviour

**Checkpoint**: US2 complete; three commits in the order #58 → #77 → #78, each green on its own.

---

## Phase 5: User Story 3 — A log handed to the wrong reader is told so, not called damaged (Priority: P1)

**Goal**: a codec handed the other format's log returns an error naming the format found, not one
documented as a damaged log (#84; [contracts/wrong-format.md](contracts/wrong-format.md); research
R8). **Depends on T013**, which reworded `UnsupportedFormatError` into something true of this case.

**Independent Test**: `go test -race -run TestWrongFormat ./gatling/text/ ./gatling/binary/`.

### Tests for #84 (write first, MUST fail before T035) ⚠️

- [X] T033 [US3] Test in gatling/text/wrongformat_test.go: `TestTextReaderRefusesABinaryLog` opens `testdata/corpus/gatling/3.14.9/simulation.log` through `text.NewReader` and `text.NewRunReader` and requires a `*gatling.UnsupportedFormatError` with `Format == gatling.FormatBinary`, a non-empty `Head`, and a message naming `gatling/simlog`; `errors.As` for `*gatling.SyntaxError` must be false — **fails on the base**: it returns `*gatling.SyntaxError`, `gatling: line 1: expected ASSERTION or RUN before the run header, found "\x00…" (98 bytes)`
- [X] T034 [P] [US3] Test in gatling/binary/wrongformat_test.go: `TestBinaryReaderRefusesATextLog` does the reverse with `testdata/corpus/gatling/3.11.5/simulation.log`, requiring `Format == gatling.FormatText` — **fails on the base**: `*gatling.SyntaxError`, `gatling: byte 0: expected the run record, found byte A`. A second subtest feeds a genuinely damaged log of the reader's own format (a corpus prefix with a byte flipped inside a record) and requires a `*gatling.SyntaxError` still, so the fix does not widen into the damage case (FR-020); a third feeds a gzip header and requires the unchanged `*gatling.FormatError` path

### Implementation for #84

- [X] T035 [US3] gatling/binary/reader.go — in `NewReader`, take `head, _ := rd.rd.src.Peek(gatling.DetectSize)` **before** the first `u8`, because the constructor fails at byte 0 with one byte consumed and a peek taken afterwards would hand `Detect` bytes 1..10 and misidentify the log (research R8). When `kind != kindRun`, run `gatling.Detect(head)`; if it names `FormatText`, return the wrong-format error instead of `rd.rd.syntax(...)`. The peek consumes nothing and costs no additional read — the `bufio.Reader` is already filled
- [X] T036 [US3] gatling/text/reader.go — at the preamble failure that today yields `Expected: "ASSERTION or RUN before the run header"`, run `gatling.Detect` over the leading bytes the scanner has already read (98 in the reproduction, far past `DetectSize`); if it names `FormatBinary`, return the wrong-format error instead
- [X] T037 [US3] Both codecs wrap the `*gatling.UnsupportedFormatError` with `%w`, naming `github.com/galax-io/parsec/gatling/simlog` as the entry point that reads both formats without being told which. No field is added to the type: the routing datum a program branches on is `Format`, which it already carries
- [X] T038 [US3] CHANGELOG.md `### Changed` for #84, and gate and commit: full gate, plus `go test -tags=integration ./gatling/...`; stage exactly gatling/binary/reader.go, gatling/binary/wrongformat_test.go, gatling/text/reader.go, gatling/text/wrongformat_test.go, CHANGELOG.md; assert the staged list; commit as `fix(gatling): a codec handed the other format's log names the format, not damage (#84)`. The pull request carries the `breaking` label — the error type a consumer branches on changes

**Checkpoint**: US3 complete; one commit, green on its own. `simlog` is untouched.

---

## Phase 6: User Story 4 — A span never ends before something it counted began (Priority: P2)

**Goal**: an item the fold counted whose end the source did not record extends the run's end to its
own start (#103; [contracts/bounds.md](contracts/bounds.md)).

**Independent Test**: `go test -race -run TestBounds ./model/`.

### Tests for #103 (write first, MUST fail before T041) ⚠️

- [X] T039 [US4] Test in model/bounds_test.go: `TestAnItemWithNoRecordedEndExtendsTheEnd` folds a sample at `t0+10s` with `Some(5s)` and a sample at `t0+20s` with no duration, and requires `Start() == t0+10s` and `End() == t0+20s` — **fails on the base**: `End()` returns `t0+15s` (reproduced against the tree)
- [X] T040 [P] [US4] In the same file, `TestEndIsNeverBeforeACountedStart` walks a table of folds — end-less item first, end-less item last, a `UserEnd` earlier than every sample start, a group with no duration — and asserts `End()`, when `ok`, is never `Before` the start of any item the table fed in; plus the unchanged cases: an item with a zero start still makes both report nothing, an empty fold still reports nothing, and `UserStart`/`UserEnd` still move the bounds exactly as today — **fails on the base** on the first two rows

### Implementation for #103

- [X] T041 [US4] model/bounds.go — `cover` calls `b.finish(start)` unconditionally after `b.begin(start)`, then extends by the duration when one was recorded and is not negative. This makes `cover` consistent with `coverUser`'s `UserStart`, which already does `begin(u.At); finish(u.At)`
- [X] T042 [US4] model/bounds.go — remove the `b.end.Before(b.start)` guard from `End()`, now unreachable (every path that calls `begin(x)` also calls `finish(≥x)`, and `finish` only raises, so `end ≥ max(begun) ≥ start`), and remove the doc paragraph that explains the crossing. Replace the type's paragraph "a sample or group end that is absent — such an item still contributes its start" with the rule: such an item is known to have been running at its own start instant, which is what Gatling's own arithmetic does with a request that never completed
- [X] T043 [US4] CHANGELOG.md `### Changed` for #103 — it moves a number every consumer divides by — and gate and commit: full gate; stage exactly model/bounds.go, model/bounds_test.go, CHANGELOG.md; assert the staged list; commit as `fix(model): an item with no recorded end extends the run's end to its own start (#103)`. The pull request carries the `breaking` label

**Checkpoint**: US4 complete; one commit, green on its own.

---

## Phase 7: User Story 5 — What the module publishes about itself is true, and it routes to the entry point (Priority: P2)

**Goal**: every one of the six package pages can be believed, and each leads to `run.Find` →
`simlog.NewRunReader` → a fold over `Next` (#79, #85, #86, #96; research R9–R12).

**Independent Test**: `go test -race -run 'TestDocComments' ./...`, then `go doc` read for each of the
six packages and for the two `Warning` types.

### Tests for #79 (write first, MUST fail before T045) ⚠️

- [X] T044 [US5] Extend api_test.go with `TestDocCommentsRenderCleanly`: for every exported identifier in the six packages, take its doc comment through `go/doc`'s comment parser and fail when the rendered text contains a literal `//` or a paragraph that lost its break — the defect that published `...instead.//` on pkg.go.dev. Add `TestNoDocCommentRepeatsItself`, which fails when one comment states the binary-boundary-cut fact twice — **the first fails on the base only if the defect is reintroduced** (it was fixed while #76 moved the gate, research R9), so it is a guard, not a repair; **the second fails on the base**: `gatling/errors.go:69` and `:83` both state it

### Implementation for #79

- [X] T045 [US5] gatling/errors.go — rewrite `SyntaxError.Error`'s doc comment at `:45`. It says "Error names the line, what was expected there and what was found", above a method that branches on `Format` and renders `byte %d` for a binary log; it must describe both renderings or defer to the type's comment, which already gets it right. A consumer reading it alone must render a binary failure as a byte offset
- [X] T046 [US5] gatling/errors.go — remove one of the two boundary-cut paragraphs from `TruncationError`'s doc comment (`:69` and `:83` state the same fact), keeping the fuller one. It is a true and important fact stated once too many inside one comment
- [X] T047 [US5] CHANGELOG.md `### Fixed` for #79, and gate and commit: full gate; stage exactly gatling/errors.go, api_test.go, CHANGELOG.md; assert the staged list; commit as `docs(gatling): SyntaxError.Error names both renderings, and the truncation fact once (#79)`

### Tests for #85 (write first, MUST fail before T049) ⚠️

- [X] T048 [US5] Extend api_test.go with `TestEveryReaderStatesTheConcurrencyRule`: for `text.Reader`, `text.RunReader`, `binary.Reader`, `binary.RunReader`, `simlog.RecordReader` and `simlog.RunReader`, the type's doc comment must state that a value may be used by one goroutine at a time — **fails on the base**: the word "concurrent" appears once in non-test code, at `gatling/run/find.go:345`, about a concurrent build appending to a file

### Implementation for #85

- [X] T049 [US5] Add the sentence to all six doc comments, beside the aliasing rule they already carry, saying what a concurrent call costs rather than only that it is unsupported: the text codec's interner is a map (`gatling/text/intern.go`), so a concurrent `Next` can end the process with `fatal error: concurrent map read and map write`, which `recover` cannot catch. Files: gatling/text/reader.go, gatling/text/model.go, gatling/binary/reader.go, gatling/binary/model.go, gatling/simlog/simlog.go
- [X] T050 [US5] gatling/simlog/doc.go — the follower contract states the same constraint, since the sidecar is the consumer most likely to reach for a second goroutine and it holds only the interface, where the concrete type carrying the map is invisible
- [X] T051 [US5] CHANGELOG.md `### Fixed` for #85, and gate and commit: full gate; stage exactly the five reader files, gatling/simlog/doc.go, api_test.go, CHANGELOG.md; assert the staged list; commit as `docs(gatling): every exported reader states it is single-goroutine (#85)`

### Tests for #86 (write first, MUST fail before T053) ⚠️

- [X] T052 [US5] Extend api_test.go with `TestTheTwoWarningsLinkToEachOther`: `gatling.Warning`'s doc comment must name `model.Warning` and `model.Warning`'s must name `gatling.Warning`, and `model.Warning.Version`'s comment must say why it is a string — **fails on the base**: neither comment mentions the other (`grep` for `model.Warning` in gatling/errors.go and for `gatling.Warning` in model/run.go finds nothing)

### Implementation for #86

- [X] T053 [US5] gatling/errors.go — `Warning`'s doc comment says the canonical form is `[github.com/galax-io/parsec/model.Warning]`, whose `Reason` is this type's `String()`, and that the two are one `simlog` call apart
- [X] T054 [US5] model/run.go — `Warning`'s doc comment names `[…/gatling.Warning]` as what a Gatling source builds it from, and `Version`'s comment says it is the release as text because the model is tool-agnostic and other tools do not number like Gatling — so a consumer does not order two of them with `strings.Compare` and get `"3.9.0" < "3.11.0"`, the bug `gatling.Version` exists to prevent
- [X] T055 [US5] CHANGELOG.md `### Fixed` for #86, and gate and commit: full gate; stage exactly gatling/errors.go, model/run.go, api_test.go, CHANGELOG.md; assert the staged list; commit as `docs: the two Warning types name each other and the crossing between them (#86)`

### Tests for #96 (write first, MUST fail before T057) ⚠️

- [X] T056 [US5] Extend api_test.go with `TestPackageOverviews`: the root `doc.go` names every package in the module including `gatling/run`; no overview contains a phrase claiming a shipped package or type is future work (assert the absence of the literal "later milestone" and "arrive in"); `gatling/text/doc.go` names `RunReader`; the root and `model/doc.go` name the three-call path — **fails on the base** on all four: `gatling/doc.go:10` says the model and its conversion "arrive in a later milestone" (they arrived in v0.0.3 and v0.0.5), the root lists five packages and omits `gatling/run`, `gatling/text/doc.go` mentions `RunReader` zero times, and no overview names the entry point

### Implementation for #96

- [X] T057 [US5] Rewrite the four defective overviews: gatling/doc.go (the model and the conversion exist; point at `gatling/simlog` as the entry point, which it currently does not name), doc.go (list `gatling/run` and name `run.Find` → `simlog.NewRunReader` → a fold over `Next`), gatling/text/doc.go (name `RunReader` as the model-facing counterpart, as `gatling/binary/doc.go` already asserts it exists), model/doc.go (name which package and which call the items arrive through, and the same three-call path)
- [X] T058 [US5] gatling/simlog/doc.go — stop deflecting. The recommendation "Reach for [RunReader] unless you need to see what the log actually held" sits on `RecordReader`'s comment, which a reader reaches only after choosing the wrong path; state it in the overview. Doc-link every identifier named in package prose, consistently across all six overview files — `model/doc.go` has 15 links and `gatling/binary/doc.go` 4, while `doc.go`, `gatling/doc.go`, `gatling/text/doc.go` and `gatling/run/doc.go` have none
- [X] T059 [US5] CHANGELOG.md `### Fixed` for #96, and gate and commit: full gate; stage exactly doc.go, gatling/doc.go, gatling/text/doc.go, gatling/simlog/doc.go, model/doc.go, gatling/run/doc.go, api_test.go, CHANGELOG.md; assert the staged list; commit as `docs: the package overviews route to the entry point and claim nothing unshipped (#96)`

**Checkpoint**: US5 complete; four commits, each green on its own. No type-level documentation is
rewritten — #96 calls it good, and only the overviews misroute.

---

## Phase 8: User Story 6 — Every test that pins the frozen surface can go red (Priority: P2)

**Goal**: six tests that cannot fail, or stop testing what they name, fail when the thing they name
breaks (#95; research R13). Principle III is NON-NEGOTIABLE and the corpus is the specification, so a
test that cannot go red is a gap in it.

**Independent Test**: for each of the six, break the production code it names and watch it fail. The
break is recorded in the pull-request body; a green suite is not the evidence here.

### Tests for #95 (this story *is* tests; each task is its own repair) ⚠️

- [X] T060 [US6] gatling/binary/reader_test.go — `TestTheGroupPathIsReusedBetweenRecords` (`:146`) has three exits: `t.Skip` at `:172` when the reuse is gone, a bare `return` when it holds, and `t.Fatal` only if the corpus lacks two records with a 2-deep path. Rewrite it to **copy** before comparing (`held = slices.Clone(rec.Groups)` — today `held = rec.Groups` aliases, so its claim that "copying it really does keep it" is untested) and to assert the reuse rather than skip out of it. **Demonstrate**: make `sealGroups` return a fresh slice and watch the test go red
- [X] T061 [P] [US6] model/outcome_test.go — `TestSuccessSelectionIsUnchangedByFailures` (`:114`) filters with `successes` (`:99`), which keeps samples whose `Outcome == OutcomeSuccess`, then asserts the filtered result is unchanged. `model.Sample.Outcome` is a plain struct field with no constructor or validator (`model/sample.go:72`), so nothing in the module can make it fail. Rewrite it to assert the module's actual claim — that a failed sample is never counted as a success and never silently dropped from a fold — against `model`'s own primitives. **Demonstrate**: change what `Outcome` means to the fold and watch it go red
- [X] T062 [P] [US6] gatling/text/model_selection_test.go — `:143` is the corpus-backed twin: it drops `OutcomeFailure` samples, then compares `successesOf(all)` with `successesOf(withoutFailures)`, which is the same sequence for any decoder output. Rewrite it to assert the file's own headline claim — "this is the correctness failure the ecosystem has already made, and the model is where it is either possible or not" — against the decoded corpus
- [X] T063 [P] [US6] model/item_test.go — `TestItemKindSelectsOneField` (`:12`) builds `model.Item` literals setting one field and asserts the others are zero, which restates the literals three lines above; `Item` has no constructor. Rewrite it to assert what `Kind` means to a consumer: that `Bounds.Extend` and any dispatch on `Kind` read the field the kind selects and no other
- [X] T064 [P] [US6] gatling/text/model_review_test.go — `:65` asserts `d < 0` after `ok` is false, which is unreachable: `Opt.value` is unexported, so outside `model` an `Opt` is `Some(v)` or the zero value, and `d` is exactly 0. Remove the dead assertion and either test the wrap-to-negative danger where it is testable — inside `model`, on the conversion — or drop the claim from the test's name and comment. The `ok` half stays
- [X] T065 [P] [US6] gatling/run/find_test.go — `TestFindNeverOpensTheLog` (`:829`) builds a log at mode `0o000` "that cannot be opened" and does not call `requireUnixPermissions` (`:82`), which its three siblings at `:760`, `:996` and `:1029` do. As root — an ordinary Docker CI container — or on Windows, mode 000 does not deny. Add the call, or restructure so both fixtures are still tested when the mode bit is inert. **Demonstrate**: run the suite as root and confirm it skips or still tests both
- [X] T066 [US6] Record the six demonstrations: for each, the break applied, the test name, and the failure message it produced. This is the only evidence that a test which could not fail now can, and it goes in the pull-request body
- [X] T067 [US6] CHANGELOG.md `### Fixed` for #95 — the tests now cover what they name, including the documented `Groups` reuse guarantee that freezes at v0.1.0
- [X] T068 [US6] Gate and commit #95: full gate, plus `go test -race -shuffle=on -count=2 ./...` (shuffled twice, because two of these tests now depend on fixture order); stage exactly gatling/binary/reader_test.go, model/outcome_test.go, gatling/text/model_selection_test.go, model/item_test.go, gatling/text/model_review_test.go, gatling/run/find_test.go, CHANGELOG.md; assert the staged list; commit as `test: six tests that could not fail now fail when what they name breaks (#95)`

**Checkpoint**: US6 complete; one commit, green on its own, with six recorded red-then-green
demonstrations.

---

## Phase 9: User Story 7 — A stranger installs it, calls it, and is told the truth about versions (Priority: P3)

**Goal**: someone with a `simulation.log` and no prior contact with the project reads `README.md`,
runs `go get`, pastes the first program, and reads their run; the changelog states the rule the code
implements; a researcher with a crasher has somewhere private to send it (#97, #99, #101; research
R14–R16). **#99 depends on T017**, whose compatibility statement it renders for consumers.

**Independent Test**: `go test -run Example ./gatling/simlog/` compiles the program the README
publishes; `gh api repos/galax-io/parsec/community/profile --jq '.files | keys'` reports `security`.

### Tests for #97

- [X] T069 [US7] Read `gatling/run/find.go:433-450` — `later`'s own comment is the authority — and confirm the three-level rule: modification time, then the run id's UTC stamp via `runStart(name)`, then the whole name. **Verified on the base**: `CHANGELOG.md:145-149` states two levels ("modification time first, then the directory name … descending name is descending run start"), contradicting the code; `:157-161` states the three-level rule correctly, contradicting the first

### Implementation for #97

- [X] T070 [US7] CHANGELOG.md — correct the first statement in the 0.0.9 entry to the three-level rule, keeping the paragraph it sits in (the argument that the tie-break is not a formality still needs the rule stated) and leaving the correct statement at `:157-161`. A reader must be able to predict what `Find` returns for a root of mixed simulation ids with equal modification times — Maven's `runMultipleSimulations` is the shape
- [X] T071 [US7] Gate and commit #97: stage exactly CHANGELOG.md; assert the staged list; commit as `docs(changelog): the 0.0.9 run-ordering rule states what the code does, once (#97)`

### Tests for #99 (write first, MUST fail before T073) ⚠️

- [X] T072 [US7] The README's first program is the body of gatling/simlog/example_test.go, which `go test` already compiles and output-checks — so it cannot rot. Add a check to api_test.go, `TestReadmeNamesTheEntryPoint`: `README.md` contains `go get github.com/galax-io/parsec`, names `simlog.NewRunReader`, contains at least one fenced code block, states the binary range as `3.13.1` through `3.15.1`, and contains no per-package version stamp (`(v0.0.` must not appear) — **fails on the base** on every clause: `grep -c '```' README.md` is 0, there is no `go get` line, the table at `:24` reads `Gatling 3.13.0 … 3.15.x`, `:53` names `gatling/text.NewRunReader`, and version stamps run from `:23` to `:39`

### Implementation for #99

- [X] T073 [US7] Rewrite README.md to open with what the library is, `go get github.com/galax-io/parsec`, the minimum Go version (the `go` directive, 1.25 — the consumer floor, not the `toolchain` line), and a copy-pasteable first program inlined from gatling/simlog/example_test.go
- [X] T074 [US7] README.md — `simlog.NewRunReader` is the named default; the codec packages are presented as the shortcut when the version is already known. This is what the file's own opening promises to rescue: a run archived last year, in the binary format `gatling/text` cannot read
- [X] T075 [US7] README.md — the compatibility table reads 3.11.5–3.12.0 (text) and 3.13.1–3.15.1 (binary), carries the reason 3.13.0 is refused (it writes the format but cannot generate a report, so no run of it can carry the second account of its own numbers a corpus entry needs — Principle III), and points at `simlog.Supported()` so a reader takes the range from the API rather than from prose
- [X] T076 [US7] README.md — delete the per-package version stamps and the "Status" narrative (`CHANGELOG.md` is where that lives and it is good), and add the compatibility section T017 wrote, which `doc.go:22` already points at and which does not exist yet
- [X] T077 [US7] Gate and commit #99: `go test -run Example ./gatling/simlog/` green so the inlined program is the compiled one; stage exactly README.md, api_test.go; assert the staged list; commit as `docs(readme): open with what it is, how to install it, and a program that runs (#99)`
- [X] T078 [US7] GitHub settings, no diff to review: set the repository description so it does not promise statistics (`gh api repos/galax-io/parsec --jq .description` currently ends "with decoders and statistics", which contradicts `README.md`'s "This library computes no statistic" and Principle I), and set `homepage` to the pkg.go.dev URL. Record the before and after in the pull-request body

### Implementation for #101

- [X] T079 [US7] Write SECURITY.md at the repository root: how to report privately and what to expect; which versions receive fixes, in the terms of the release policy in AGENTS.md (fixes land on `main` and are cherry-picked onto the current `release/X.Y.0`, so the supported set is the latest `X.Y` line — `0.1` at this tag); and the trust boundary plainly — this module decodes files it does not trust, in a process it does not own, with four fuzz targets, a nightly fuzz workflow and an allocation-cap regime designed against corrupt length prefixes
- [X] T080 [US7] Enable GitHub's private vulnerability reporting in repository settings, and confirm `gh api repos/galax-io/parsec/community/profile --jq '.files | keys'` now reports `security` — **fails on the base**: the profile lists `code_of_conduct`, `contributing`, `issue_template`, `license`, `pull_request_template`, `readme` and no security file
- [X] T081 [US7] Gate and commit #101: stage exactly SECURITY.md; assert the staged list; commit as `docs: add SECURITY.md for a module that decodes untrusted input (#101)`

**Checkpoint**: US7 complete; three commits plus two settings changes. `CODE_OF_CONDUCT.md`, a PR
template and an issue template are out of scope — #101's own non-goals.

---

## Phase 10: User Story 8 — The release path cannot be moved under it (Priority: P3)

**Goal**: the job that cuts releases runs only code this repository chose, and a dispatch input
carrying a quote never reaches a shell (#93; [contracts/release-path.md](contracts/release-path.md);
research R17).

**Independent Test**: `grep -rn 'uses: ' .github/workflows/ | grep -v '\./\.github' | grep -vE '@[0-9a-f]{40} #'`
prints nothing.

### Tests for #93 (write first, MUST fail before T084) ⚠️

- [X] T082 [US8] New shell gate scripts/check-pins.sh with scripts/check-pins_test.sh beside it, in the shape of the existing `scripts/check-*.sh` pairs: it fails when any `uses:` in `.github/workflows/` names anything but a 40-character SHA with a trailing `# vX.Y.Z` comment, ignoring `./.github/workflows/*` local calls, and it refuses a `${{ inputs.* }}` or `${{ matrix.* }}` substitution inside a `run:` body that no validation step guards — which is what makes FR-046 a gate rather than a review note. Its own test covers a pinned line, a tag line, a SHA with no comment, a local call, a guarded interpolation and an unguarded one — **fails on the base**: ten `uses:` entries name tags, four of them third-party (`orhun/git-cliff-action@v4` in the job holding `contents: write`, `golangci/golangci-lint-action@v9`, `sbt/setup-sbt@v1` twice)
- [X] T083 [P] [US8] Add scripts/check-pins.sh to the `quick` job's shell-gate loop in .github/workflows/verify.yml, beside `check-compat_test.sh`, `check-coverage_test.sh`, `check-linkage_test.sh` and the hook tests, so the pin rule is enforced on every pull request rather than by review

### Implementation for #93

- [X] T084 [US8] Pin every third-party action to a full commit SHA with the version in a trailing comment (MUST), and every `actions/*` in the same pass (SHOULD — a half-pinned file invites the next contributor to copy the unpinned line). **Resolve the SHAs at implementation time**; research R17's table, resolved 2026-09-12, is a starting point and a proof that each tag resolves, not the pins themselves. Files: .github/workflows/{release,verify,gatling-canary,record-corpus,fuzz-nightly,ci}.yml
- [X] T085 [US8] Route the two unguarded dispatch interpolations through `env:` and validate them with the guard record-corpus.yml:74-87 already carries — a `set -euo pipefail` step that exits 1 with a message naming the value. No second pattern is introduced. .github/workflows/gatling-canary.yml:85 (`matrix.gatling`, a Gatling version: `^[0-9]+\.[0-9]+\.[0-9]+$`) and .github/workflows/fuzz-nightly.yml:70-71 (`matrix.case.target` a Go identifier, `matrix.case.package` an import path, `inputs.fuzztime` a duration: `^[0-9]+[smh]$`)
- [X] T086 [US8] Confirm .github/dependabot.yml needs no change: its `github-actions` ecosystem updates a SHA pin and keeps the trailing version comment in step, which is what #93 asks for, and the existing file already groups actions weekly. If that turns out to be false, change it here and say why
- [X] T087 [US8] Gate and commit #93: `bash scripts/check-pins.sh` and `bash scripts/check-pins_test.sh` green, the whole shell-gate loop green, and each touched workflow still does what it did before (no step added or removed beyond the validation steps); stage exactly .github/workflows/*.yml, scripts/check-pins.sh, scripts/check-pins_test.sh; assert the staged list; commit as `ci: pin every action to a commit and validate dispatch inputs before they reach a shell (#93)`

**Checkpoint**: US8 complete; one commit, green on its own. No workflow changes what it does.

---

## Phase 11: Polish & Cross-Cutting Concerns

**Purpose**: the full gate set, the measurements the plan promised, and the commit shape the rules
require.

- [X] T088 [P] Doc review against the contracts: every `String()` comment states the enum rule ([contracts/enum-rendering.md](contracts/enum-rendering.md)); `Bounds`, `Bounds.End` and `cover` state the span rule ([contracts/bounds.md](contracts/bounds.md)); `UnsupportedFormatError`, `SyntaxError` and `Record.Line` state the three decisions ([contracts/public-api.md](contracts/public-api.md)); the six readers state the concurrency rule; the two `Warning`s link. Read them as `go doc` renders them, not as source
- [X] T089 [P] CHANGELOG.md `[Unreleased]` complete and in Keep a Changelog order — `### Added` (#77's `gatling.Tool`), `### Changed` (#107, #77, #78, #84, #103), `### Removed` (#107, #77), `### Fixed` (#79, #85, #86, #95, #96, #97) — every entry citing its issue, plus the #13 compatibility statement. No entry for #93, #99 or #101: none ships module behaviour
- [X] T090 Full gate: `test -z "$(gofmt -l .)"`, `gofumpt -l .` empty, `go vet ./...`, `go mod tidy && git diff --exit-code`, `go build ./...`, `golangci-lint run --build-tags=integration ./...` zero issues, `go test -race -shuffle=on ./...`, `go test -tags=integration -count=1 -skip 'PeakMemory$' ./...`, `go test -tags=integration -count=1 -run 'PeakMemory$' ./...`, and the shell-gate suite including the new scripts/check-pins_test.sh; `go list -deps ./model/... ./gatling/...` names nothing outside the standard library
- [X] T091 The inventory is the count the contract promises: `tail -1 testdata/api/surface.txt` reads `TOTAL 275`, `go test -run TestExportedSurface ./...` is green, and the file matches [contracts/public-api.md](contracts/public-api.md) line for line. If the generator reports a different number, the generator is right — correct the contract and data-model.md, and say so in the pull-request body
- [X] T092 Re-derive FR-003 and SC-003 instead of inheriting them: for every identifier in testdata/api/surface.txt, establish whether anything outside this module calls it — against the three consumers' recorded plans (galaxio-cli#50 and #61, comet#3, backend#230) and every contract under specs/*/contracts/ — and list any whose only callers are inside parsec and which no plan names. #107's inventory found two (`Gate`, `MaxRunStart`); this either confirms the list is now empty or names what it missed, while a removal still costs a commit rather than a deprecation window. Record the finding and the method in the pull-request body
- [X] T093 Coverage for the pull-request descriptions: `go test -tags=integration -count=1 -skip 'PeakMemory$' -coverpkg=./... -coverprofile=cover.out ./... && bash scripts/check-coverage.sh --enforce cover.out` (90% decoder packages, 80% overall). No benchmark comparison is required — this feature changes no decode path beyond a `bufio` peek into a filled buffer — but run `BenchmarkDecode` once to confirm nothing regressed, and say so
- [X] T094 Each commit green on its own: for each of the sixteen commits (`git log --format=%h origin/main..HEAD`, skipping the docs and constitution commits), `git worktree add <scratchpad>/c-<sha> <sha>`, run `go build ./... && go test ./...` there, then `git worktree remove` it; record the sixteen results
- [X] T095 Run [quickstart.md](quickstart.md) sections 1–8 end to end as the final validation, and correct any command whose name pattern no longer matches a test by folding the fix into the docs commit (`git commit --fixup=<docs sha>` with only specs/011-stable-api/ staged, then `git rebase --autosquash <docs sha>^`)

---

## Phase 12: Publication — gated on the maintainer

**Purpose**: what leaves the machine. Pushing a branch, opening pull requests and cutting a release
are outward-facing and are **not** performed without the maintainer's explicit go-ahead; the
implementation report stops here and asks.

- [X] T096 Push `011-stable-api` and the constitution branch `docs/constitution-v2.4.0` it is stacked on — with the maintainer's go-ahead (done 2026-09-12)
- [X] T097 Open the pull requests — done as **two**, not five, following the maintainer's own precedent on v0.0.10 (one pull request for the branch, PR #112): [#115](https://github.com/galax-io/parsec/pull/115) carries the constitution amendment alone, because `AGENTS.md`'s agreement rule and "1 concern per PR" both require it, and [#116](https://github.com/galax-io/parsec/pull/116) carries the milestone. Both hold milestone `v0.1.0 A stable API`; #116 carries the `breaking` label and sixteen `Fixes` lines. #116 targets `main` rather than the amendment branch: GitHub registers a closing reference only for a pull request into the default branch, so a stacked base left all sixteen links dead and `scripts/check-linkage.sh --pr 116` failing. Both orders of merge work — merging #115 first shrinks #116's diff. `scripts/check-linkage.sh --pr 115` and `--pr 116`: **PASS**, and CI is green on both — 22 checks on #116 including compat, deps, coverage, e2e, the five canary runs and the four fuzz targets. The commit order still allows research R19's three-contract stack to be cut from the branch
- [ ] T098 After the last merge: run `scripts/check-linkage.sh` with no argument and confirm milestone #11 has no open issue whose fix is on `main` and no PR without a milestone. Then the release, per AGENTS.md — `git checkout -b release/0.1.0 main`, `git push -u origin release/0.1.0`, `git tag v0.1.0`, `git push origin v0.1.0` — with the maintainer's go-ahead, and never before T091 has confirmed the inventory

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies — the docs commit must land before any code change (Spec-first)
- **Foundational (Phase 2)**: nothing in code; the reading rule only. Blocks nothing mechanically and everything by rule
- **US1 (Phase 3)**: first. It builds the inventory every later surface commit updates, and T013's reword is what #84 needs
- **US2–US8 (Phases 4–10)**: all depend on Phase 3 for the inventory; otherwise independent of each other except where the file-conflict table below says so
- **Polish (Phase 11)**: depends on every story
- **Publication (Phase 12)**: depends on Polish and on the maintainer

### Story Dependencies

- **US1 (P1)** → everything. `testdata/api/surface.txt` does not exist before it, so T026 has nothing to update; and T013 is what makes `UnsupportedFormatError` honest for US3
- **US2 (P1)**: internally ordered #58 → #77 (research R5). #78 is independent of both but shares four files with US1 and US5
- **US3 (P1)**: after T013 only
- **US4 (P2)**, **US6 (P2)**, **US8 (P3)**: independent of every other story and of each other
- **US5 (P2)**: after US1 (`api_test.go` exists) and after #78, which rewrites doc comments in the same enum files
- **US7 (P3)**: #99 after T017 (#13); #97 and #101 independent of everything

### Files touched by more than one story — sequence, never parallel

| File | Tasks |
|---|---|
| `gatling/errors.go` | T013 (#107), T037 (#84), T045/T046 (#79), T053 (#86) |
| `internal/wire/wire.go` | T012 (#107), T020 (#58), T025 (#77) |
| `model/run.go` | T030 (#78), T054 (#86) |
| `gatling/record.go`, `gatling/version.go` | T011/T012 (#107), T030 (#78) |
| `gatling/text/model.go` | T021 (#58), T025 (#77) |
| `api_test.go` | T006–T008 (#107), T044 (#79), T048 (#85), T052 (#86), T056 (#96), T072 (#99) |
| `CHANGELOG.md` | nearly every commit — different subsections, but append in commit order |
| `testdata/api/surface.txt` | T010 (#107), T026 (#77) |

### Within Each Story

- Tests are written first and MUST fail before the implementation task named in the phase heading
- The doc comment is part of the change, not a follow-up: Principle V freezes it with the identifier
- The `CHANGELOG.md` entry lands in the same commit as the change it records
- One tracked issue = one semantic commit, green on its own (`go build ./... && go test ./...`)

### Parallel Opportunities

- Phase 1: T001–T003 in parallel
- Genuinely independent stories, workable at the same time by different people: **US4** (`model/bounds.go`), **US6** (six test files), **US8** (`.github/`, `scripts/`), and #97 and #101 within US7
- Within a story, the `[P]` tests are parallel; the implementation tasks usually are not, because they share a file
- **Not parallel**: anything in the table above

---

## Parallel Example: the three independent stories

```bash
# After Phase 3 lands, three people can take these with no file overlap:
#   US4  — model/bounds.go, model/bounds_test.go                      (T039–T043)
#   US6  — six test files across binary, text, model, run             (T060–T068)
#   US8  — .github/workflows/*.yml, scripts/check-pins*.sh            (T082–T087)
# Each ends in one commit, green on its own, touching no file the others touch.
```

---

## Implementation Strategy

### MVP First (the three P1 stories)

US1, US2 and US3 are the freeze itself: the list exists, nothing on it has two names or two
renderings, and the first error a newcomer meets is honest. A v0.1.0 cut with only those three is a
defensible tag — the surface is bounded and truthful — but it is **not** the shipped scope: every
story below blocks the tag, because each freezes a defect if it does not land.

### Incremental Delivery

1. Phase 1–2 → docs commit on top of the constitution amendment, nothing in code
2. Phase 3 (US1) → the inventory and the withdrawals; from here every surface change is a red test until its golden file is updated
3. Phase 4–5 (US2, US3) → one name, one rule, one copy; the wrong-codec failure
4. Phase 6–8 (US4, US5, US6) → the span, the documentation, the tests
5. Phase 9–10 (US7, US8) → the landing pages and the release path
6. Phase 11 → gates, coverage, sixteen worktree runs
7. Phase 12 → push, pull requests, and the tag, each on the maintainer's word

### Parallel Team Strategy

One person can take Phase 3 alone; it is short and everything waits on it. After it, three tracks run
without touching each other: the `model`/`gatling` surface work (US2, US3, US5), the independent
fixes (US4, US6), and the repository-level work (US7, US8). The file table above is the contract
between them.

---

## Commit Mapping (AGENTS.md: 1 issue = 1 commit)

| # | Commit | Tasks | Files |
|---|---|---|---|
| — | `docs(speckit): amend constitution to v2.4.0 (skills propose, Principles III-VI decide)` | done (`fa988e9`) | .specify/memory/constitution.md, AGENTS.md |
| 0 | `docs(speckit): add 011-stable-api spec/plan/tasks` | T004 (+ T095 fixup) | specs/011-stable-api/**, .specify/feature.json |
| 1 | `refactor(gatling): withdraw Gate and MaxRunStart before the surface freezes (#107)` | T006–T016 | api_test.go, testdata/api/surface.txt, gatling/{version,policy,record,errors}.go, gatling/version_test.go, gatling/simlog/simlog.go, internal/wire/wire.go, gatling/binary/record.go, gatling/text/{parse.go,parse_test.go}, CHANGELOG.md |
| 2 | `docs: declare the v0.1.0 stability contract (#13)` | T017 | CHANGELOG.md |
| 3 | `refactor(internal/wire): build the run header and its warnings once, for both codecs (#58)` | T018–T023 | internal/wire/{wire.go,wire_test.go}, gatling/text/model.go, gatling/binary/{model.go,agreement_test.go}, CHANGELOG.md |
| 4 | `refactor(gatling): one exported Tool constant for both codecs (#77)` | T024–T028 | gatling/tool.go, gatling/tool_test.go, gatling/text/model.go, gatling/binary/capability.go, internal/wire/wire.go, testdata/api/surface.txt, CHANGELOG.md |
| 5 | `refactor: render an out-of-range enum as the type name and the number (#78)` | T029–T032 | model/{sample,run,position,capability,enum_test}.go, gatling/{version,record,format,enum_test}.go, gatling/run/{find.go,find_test.go}, CHANGELOG.md |
| 6 | `fix(gatling): a codec handed the other format's log names the format, not damage (#84)` | T033–T038 | gatling/binary/{reader.go,wrongformat_test.go}, gatling/text/{reader.go,wrongformat_test.go}, CHANGELOG.md |
| 7 | `fix(model): an item with no recorded end extends the run's end to its own start (#103)` | T039–T043 | model/{bounds.go,bounds_test.go}, CHANGELOG.md |
| 8 | `docs(gatling): SyntaxError.Error names both renderings, and the truncation fact once (#79)` | T044–T047 | gatling/errors.go, api_test.go, CHANGELOG.md |
| 9 | `docs(gatling): every exported reader states it is single-goroutine (#85)` | T048–T051 | gatling/text/{reader.go,model.go}, gatling/binary/{reader.go,model.go}, gatling/simlog/{simlog.go,doc.go}, api_test.go, CHANGELOG.md |
| 10 | `docs: the two Warning types name each other and the crossing between them (#86)` | T052–T055 | gatling/errors.go, model/run.go, api_test.go, CHANGELOG.md |
| 11 | `docs: the package overviews route to the entry point and claim nothing unshipped (#96)` | T056–T059 | doc.go, gatling/doc.go, gatling/text/doc.go, gatling/simlog/doc.go, model/doc.go, gatling/run/doc.go, api_test.go, CHANGELOG.md |
| 12 | `test: six tests that could not fail now fail when what they name breaks (#95)` | T060–T068 | gatling/binary/reader_test.go, model/{outcome_test.go,item_test.go}, gatling/text/{model_selection_test.go,model_review_test.go}, gatling/run/find_test.go, CHANGELOG.md |
| 13 | `docs(changelog): the 0.0.9 run-ordering rule states what the code does, once (#97)` | T069–T071 | CHANGELOG.md |
| 14 | `docs(readme): open with what it is, how to install it, and a program that runs (#99)` | T072–T078 | README.md, api_test.go (+ GitHub description and homepage, no diff) |
| 15 | `docs: add SECURITY.md for a module that decodes untrusted input (#101)` | T079–T081 | SECURITY.md (+ private vulnerability reporting, no diff) |
| 16 | `ci: pin every action to a commit and validate dispatch inputs before they reach a shell (#93)` | T082–T087 | .github/workflows/*.yml, scripts/check-pins.sh, scripts/check-pins_test.sh |

Every commit is written from a message file and carries the
`Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>` trailer. Commits 1, 4, 5, 6 and 7 change the
surface or its observable behaviour: their pull request carries the `breaking` label and the
`[Unreleased]` bullet `scripts/check-compat.sh` demands.

## Deviations recorded during implementation

Each is a place the plan met the code and the code won. Every one is a judgement the
implementation was allowed to make; none changes what an issue asked for.

- **#77 landed before #58**, reversing research R5. Extracting first would have given `wire.NewRun` a
  `tool` parameter that #77 removed one commit later — an add-then-remove the "intent, not path" rule
  sends to the bin.
- **The surface walker already existed.** `model/exports_test.go` held `model`'s exported surface
  against a golden with a `-update` flag. It moved to `internal/exports`, gained interface methods,
  and now runs over all six public packages; the per-package golden it replaced was deleted rather
  than kept, because it would have been a second copy of `model`'s 111 lines. `api_test.go` in the
  plan is `exports_test.go` and four `docs_*_test.go` files, split so one issue is one commit.
- **`wire.Run` is `wire.NewRun`** — `golang-naming`'s constructor rule; a function called `Run` in
  package `wire` reads as "execute" (research R18).
- **`gatling.Tool` lives in `gatling/tool.go`**, a new file, so the test file matches the source file
  and the constant sits with neither the records nor the version gate (analysis U2).
- **#78 was `model`-only.** The six `gatling`-side enums already rendered `Type(N)` and already had
  the assertions; the test to rewrite was the existing `model/unknown_test.go`, not a new file
  (research R18).
- **#93 covered two sites the issue did not name**: the same fuzz invocation in `verify.yml`, and a
  summary line in the canary. `scripts/check-pins.sh` found them, which is the argument for the gate.
- **The fuzzers caught the other half of #84.** `gatling/binary`'s error taxonomy was widened to admit
  the wrong-format answer during the fix; `gatling/text`'s own, in `mutation_test.go`, was not, and
  `FuzzReader` found an input that opens like a binary log within seven seconds. Verified locally
  afterwards: 60 s and 6.9 million executions on `FuzzReader`, 45 s on `FuzzDecode`, both clean. A
  taxonomy assertion in one codec and not the other is exactly the divergence `simlog` exists to
  prevent, and only running the fuzzer found it.
- **CI caught a defect in #93's own guard.** The first run failed all four fuzz jobs in five seconds:
  the package check required a relative import path (`^\./…`), and the matrix is built by `go list`,
  which emits full ones. The guard now accepts this module's import paths and nothing else, which is
  the shape that was wanted. Written up here rather than quietly amended: a validator that refuses
  the values it exists to pass is worse than no validator, and only running it found that.
- **#79's stray `//` was already fixed** on `main` (research R9); the commit adds the guard instead.
- **T078 and T080 are done** (2026-09-13), as settings rather than files. The description no longer
  promises statistics — it now reads "…and the version-gated decoders that produce it. Computes no
  statistic — it owns the definitions a consumer folds" — and `homepage` is
  `https://pkg.go.dev/github.com/galax-io/parsec`. Private vulnerability reporting is enabled
  (`gh api repos/galax-io/parsec/private-vulnerability-reporting` reports `enabled=true`).

  One half of T080 cannot be confirmed yet and is not a failure: the community profile reads the
  default branch, and `SECURITY.md` is on `011-stable-api` until #116 merges. Re-run
  `gh api repos/galax-io/parsec/community/profile --jq '.files | keys'` after the merge and expect
  `security` in the list.

## Notes

- `[P]` tasks = different files, no dependency on an incomplete task
- Verify every test fails on the base before its fix, with the message the task quotes; the tasks that
  say **passes on the base** explain why they exist anyway
- Stage by explicit file list and assert `git diff --cached --name-only` before every commit
- Nothing here records a corpus entry. `testdata/corpus/` is read, never written: `3.14.9` and
  `3.11.5` are US3's wrong-codec fixtures, and US6 repairs the corpus-backed selection test in place
- Two tasks have no diff to review — T078 (repository description and homepage) and T080 (private
  vulnerability reporting). Both are GitHub settings, and both are recorded in the pull-request body
