# Tasks: Finding the run

**Input**: Design documents from `/specs/009-gatling-run-discovery/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md),
[data-model.md](data-model.md), [contracts/](contracts/), [quickstart.md](quickstart.md)

**Tests**: REQUIRED, and written first. Constitution Principle III is non-negotiable and says test
tasks are never optional in a spec, plan or task list; the `/speckit-tasks` skill's generic "tests
are optional" line is the known conflict the constitution records against it, and this repository's
own `tasks-template.md` resolves it the same way. Every test task below must fail before the
implementation task it guards.

**Organization**: by user story, so each is an independently testable increment. Note the honest
caveat this feature has and v0.0.8 did not: all four stories are increments of **one function** in
one file, so they are independently *testable* but not independently *implementable in parallel* —
see [Dependencies](#dependencies--execution-order).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an incomplete task)
- **[Story]**: which user story the task belongs to
- Exact file paths in every description

## Path Conventions

parsec is a single Go module with packages at the repository root (plan.md, *Source Code*):

- **Package**: `gatling/` — no package is added by this feature
- **Tests**: `gatling/discover_*_test.go`, beside the code, table-driven on stdlib `testing`
- **Fixtures**: built in `t.TempDir()`; real directories, real modification times, real symlinks —
  never mocks, and never committed. A hand-built tree is a fixture, not corpus (Principle III)
- **Corpus**: `testdata/corpus/gatling/lastrun/` — the one new recording, if it is taken

---

## Phase 1: Setup

**Purpose**: establish the green baseline and settle the one open decision. No package is created.

- [X] T001 [P] Confirm the tree is green before any edit: `go build ./... && go test -race -shuffle=on ./...`
- [X] T002 [P] Confirm `.golangci.yml` needs no change for this feature; if it does, justify it in the Complexity Tracking table of specs/009-gatling-run-discovery/plan.md
- [X] T003 Record the Maven results root into testdata/corpus/gatling/lastrun/: add a `pom.xml` beside testdata/corpus/gatling/simulation/build.sbt, run two simulations into one root with `mvn gatling:test` (not `verify`, which deletes the file), and commit `lastRun.txt`, both run directories, the console output and a RECORDING.md stating the plugin version, the Gatling version and the machine — capturable at recording time or never (Principle III)

**Checkpoint**: baseline recorded; the corpus decision was taken and the recording made.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the four exported identifiers every story is written against.

**⚠️ CRITICAL**: no user story can begin until this phase is complete — every story's tests name
these types.

- [X] T004 [P] Write the enum test in gatling/discover_test.go: `FoundBy.String()` renders all four values, `FoundByUnknown` is the zero value, and the constants are in the order contracts/public-api.md states — MUST fail, the type does not exist yet
- [X] T005 [P] Write the error test in gatling/errors_test.go: `RunNotFoundError.Error()` names `Dir`, says so when `Default` is set, and `errors.As` reaches the type — MUST fail
- [X] T006 Add `RunLocation` (Dir, Log, Found) and `FoundBy` with its four constants and `String()` to gatling/discover.go, with the doc comments contracts/public-api.md fixes verbatim
- [X] T007 Add `RunNotFoundError` (Dir, Default) with its doc comment and `Error()` to gatling/errors.go, beside the five error types already there
- [X] T008 Widen the package doc comment in gatling/doc.go: the package holds what the codecs share **and** how a run is found, which sits before them

**Checkpoint**: `go test ./gatling/` green; the four identifiers exist and are documented.

---

## Phase 3: User Story 1 — A run is found without its generated name being typed (Priority: P1) 🎯 MVP

**Goal**: point at a project or a results root and get the run back, with no generated directory name
typed by hand.

**Independent Test**: build a results root holding one run, resolve against it with no run named, and
confirm the run directory and its `simulation.log` come back.

### Tests for User Story 1 (write first, MUST fail) ⚠️

- [X] T009 [P] [US1] Newest-run test in gatling/discover_test.go: three runs with distinct modification times resolve to the newest, with `Found == FoundByNewest` and `Log == filepath.Join(Dir, "simulation.log")`
- [X] T010 [P] [US1] Determinism test in gatling/discover_test.go: three runs whose modification times are **identical** — the shape a clone, an rsync or a CI cache restore produces — resolve to the same run on every repetition, and it is the highest directory name (research R6). This is the test that catches an ordering that is merely "newest" and not total
- [X] T011 [P] [US1] Default-root test in gatling/discover_test.go: with `t.Chdir` into a tree holding `target/gatling`, `FindRun("")` resolves it, and the result reports that the root was a default
- [X] T012 [P] [US1] Override test in gatling/discover_test.go: `FindRun("build/reports/gatling")` resolves the Gradle layout and consults no default
- [X] T013 [P] [US1] Candidate test in gatling/discover_test.go: a child holding a report but no `simulation.log` is not a run, and a child that is a plain file is not a candidate at all

### Implementation for User Story 1

- [X] T014 [US1] Implement the results-root scan in gatling/discover.go: read the root's entries once, keep directories that directly hold a `simulation.log`, order by modification time then name, both descending, and return the first with `FoundByNewest` (data-model.md steps 2 and 4)
- [X] T015 [US1] Implement the default results root in gatling/discover.go: an empty path means `target/gatling` relative to the working directory, and the fact that it was a default is carried so a failure can say so (research R3)
- [X] T016 [US1] Return `*RunNotFoundError` naming the searched directory when the root holds no run, in gatling/discover.go, with the zero `RunLocation` beside it

**Checkpoint**: US1 is shippable on its own — `galaxio report` can be pointed at a project. `lastRun.txt` is not read yet, which for Gradle and sbt users is already the whole feature (research R2).

---

## Phase 4: User Story 2 — The run Gatling last wrote is the run that is read (Priority: P2)

**Goal**: when a `lastRun.txt` is present and still true, it decides — and when it is stale, garbage
or an error message, it costs a `stat` and nothing else.

**Independent Test**: three runs whose modification times make the middle one neither newest nor
oldest, plus a `lastRun.txt` naming the middle; then the same root with the file removed.

### Tests for User Story 2 (write first, MUST fail) ⚠️

- [X] T017 [P] [US2] Precedence test in gatling/discover_test.go — the acceptance case from #11: `lastRun.txt` naming the middle of three runs returns the middle with `FoundByLastRun`; removing the file returns the newest with `FoundByNewest`. Assert the two results differ, or the fixture proves nothing
- [X] T018 [P] [US2] Pointer-to-nothing table in gatling/discover_test.go: a deleted directory, a directory with no `simulation.log`, an `ExecutionError: ...` line, `Gatling simulation assertions failed!`, and a blank line — each falls through to `FoundByNewest` rather than failing (FR-006, research R7)
- [X] T019 [P] [US2] Containment table in gatling/discover_test.go: `../../etc`, an absolute path, `sub/dir`, `.` and `..` are never followed — asserted against a tree where the escape target **does** hold a `simulation.log`, so a passing test means the rule bit (FR-007, research R8)
- [X] T020 [P] [US2] Shape table in gatling/discover_test.go, from contracts/lastrun-file.md: CRLF line endings, surrounding whitespace, several names, invalid UTF-8, and a file past the 64 KiB cap — which is treated as absent, not as an error
- [X] T021 [P] [US2] Multi-line selection test in gatling/discover_test.go: several lines naming valid runs resolve to the newest of *those*, not to the newest in the root (research R7)
- [X] T022 [P] [US2] Three-way `FoundBy` test in gatling/discover_test.go, completing US1 scenario 5: a run named by the caller, one taken from `lastRun.txt` and one taken as newest are distinguishable from each other
- [X] T023 [US2] Integration test in gatling/discover_corpus_test.go behind `-tags=integration`: the recorded `lastRun.txt` from T003 resolves to a run directory beside it, and `t.Skip` with a reason when the recording is absent rather than faking one

### Implementation for User Story 2

- [X] T024 [US2] Implement the `lastRun.txt` read in gatling/discover.go: a bounded read capped at 64 KiB, UTF-8, split on LF, each line stripped of a trailing CR and surrounding whitespace, blanks dropped; an absent or oversized file is simply no pointer (contracts/lastrun-file.md)
- [X] T025 [US2] Implement the bare-name rule and candidate membership in gatling/discover.go: a line is used only if it equals its own base, is not `.` or `..`, holds no separator, and names a candidate found by T014's scan; the newest matching line wins and returns `FoundByLastRun` (data-model.md step 3)

**Checkpoint**: #11's three acceptance cases all pass. No error text is ever matched, so the plugin can reword its messages.

> **T022 was implemented in Phase 5, not here.** It asserts that the three `FoundBy` values are
> distinguishable, which needs `FoundByPath` — and that is US3's early return. Planning put it in
> this phase by oversight; the test lives beside the US3 cases it depends on.

---

## Phase 5: User Story 3 — A path that is already a run is honoured exactly (Priority: P2)

**Goal**: a directory that is a run, or the `simulation.log` itself, is returned as given — no search,
no default, no sibling able to displace it.

**Independent Test**: resolve against a run directory, then against the `simulation.log` inside it,
and confirm both return that run.

### Tests for User Story 3 (write first, MUST fail) ⚠️

- [X] T026 [P] [US3] Named-run test in gatling/discover_test.go: a run directory and the `simulation.log` inside it both resolve to the same `Dir` and `Log`, with `Found == FoundByPath`
- [X] T027 [P] [US3] Never-searched-past test in gatling/discover_test.go: a newer sibling run beside a named run does not displace it (FR-003)
- [X] T028 [P] [US3] Run-not-root test in gatling/discover_test.go: a directory holding both a `simulation.log` and subdirectories that also hold one is the run itself (FR-005)
- [X] T029 [P] [US3] Symlink test in gatling/discover_test.go: a run directory that is a symlink into another tree resolves, and a dangling link is not a run; skip with a reason where the platform cannot create one (research R8)

### Implementation for User Story 3

- [X] T030 [US3] Implement path classification as step 1 of gatling/discover.go: a path naming a `simulation.log`, or a directory directly holding one, returns `FoundByPath` before any scan runs — the early return sits ahead of T014's code even though it lands after it (data-model.md step 1)

**Checkpoint**: an archived run, a CI-supplied path and a results root all work through one call.

---

## Phase 6: User Story 4 — A failure says where it looked (Priority: P3)

**Goal**: every failure names the directory searched, says whether that directory was the caller's or
a default, and never dresses a failure to look as an absence of runs.

**Independent Test**: resolve against an empty directory and against a path that does not exist, and
confirm each names the directory searched and returns no run.

### Tests for User Story 4 (write first, MUST fail) ⚠️

- [X] T031 [P] [US4] Failure table in gatling/discover_test.go: an empty root, a path that does not exist, and `FindRun("")` with no `target/gatling` present — each returns a `*RunNotFoundError` naming the directory, with `Default` true only for the third, and the zero `RunLocation` beside it
- [X] T032 [P] [US4] Unreadable-directory test in gatling/discover_test.go: a directory whose permissions forbid reading returns the wrapped `*fs.PathError`, reachable with `errors.As`, and **not** a `*RunNotFoundError` (FR-012); `t.Skip` when running as root, where the permission cannot be enforced. This is the test that rots — a version treating every read error as "no runs here" passes every other case in this file
- [X] T033 [P] [US4] No-fallback test in gatling/discover_test.go: a path naming neither a run nor a root containing one fails rather than falling back to the default root (FR-003)

### Implementation for User Story 4

- [X] T034 [US4] Set `RunNotFoundError.Default` on every failure path and wrap read failures with `%w` in gatling/discover.go, keeping the two endings distinct: a completed search that found nothing, and a search that could not be made

**Checkpoint**: all four stories green independently; a caller in the wrong place is told where the code looked and where that path came from.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T035 [P] Add `FuzzLastRun` in gatling/discover_fuzz_test.go with the seeds quickstart.md lists — empty, a bare name, CRLF, several names, an `ExecutionError:` line, a separator, `..`, an absolute path, invalid UTF-8, and a file past the cap — asserting no panic and nothing selected outside the root
- [X] T036 [P] Add `BenchmarkFindRun` in gatling/discover_bench_test.go over a synthetic 1000-run root, and record the figures in the Performance Goals section of specs/009-gatling-run-discovery/plan.md — one directory read, at most one `stat` per entry, allocations proportional to the entry count (research R10)
- [X] T037 [P] Assert FR-013 positively in gatling/discover_test.go: a root whose `simulation.log` files hold bytes that are not a Gatling log — or are unreadable by permission — still resolves, because discovery opens none of them
- [X] T038 [P] Add the runnable example in gatling/discover_example_test.go showing `FindRun` composed with `os.Open` and `simlog.NewRunReader`, exactly as contracts/public-api.md renders it
- [X] T039 [P] Update CHANGELOG.md under **Added**: `gatling.FindRun`, `RunLocation`, `FoundBy` and `RunNotFoundError`, with the note that only `gatling-maven-plugin` writes a `lastRun.txt` so `FoundByNewest` is the ordinary outcome
- [X] T040 [P] Update the packages table and Status section of README.md: `gatling/` gains run discovery at v0.0.9
- [X] T041 Measure `go test -cover ./gatling/` against the 90% decoder-package floor and the 80% module floor, and record both in the PR description
- [X] T042 Run the full gate: `gofmt -l .`, `go vet ./...`, `go test -race -shuffle=on ./...`, `go test -tags=integration ./...`, and `go mod tidy && git diff --exit-code`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies. T003 is the open decision and gates only T023
- **Foundational (Phase 2)**: after Setup — blocks every story, because every story's tests name its types
- **US1 (Phase 3)**: after Foundational
- **US2 (Phase 4)**: after US1 — T025 selects among the candidates T014 produces
- **US3 (Phase 5)**: after Foundational. Its early return is independent of the scan, so it can be pulled ahead of US2 if the trivial case is wanted first
- **US4 (Phase 6)**: after US1 — it sharpens the error T016 already returns
- **Polish (Phase 7)**: after the stories it covers; T041 and T042 last

### The honest caveat

Unlike v0.0.8, where a story lived in its own package, all four stories here edit
`gatling/discover.go`. They are independently **testable** — each story's tests pass with only its
own implementation in place — but they are increments of one function, so two people cannot take two
stories without conflicting. Plan for one implementer on the function and parallelism in the tests,
which are the bulk of the work.

### Within Each Story

- Every test task precedes the implementation task it guards, and must fail first
- T006 and T007 before every test that names the types, or nothing compiles
- T014 before T025 (a `lastRun.txt` line is validated against the candidate set the scan produces)
- T030's early return goes ahead of T014's scan in the final file, though it lands later

### Parallel Opportunities

- T001 and T002 together; T003 whenever the decision is made
- T004 and T005 together — different files
- **All test tasks within a story together** — T009–T013; T017–T023; T026–T029; T031–T033
- T035–T040 together — six different files, no shared state

## Parallel Example: User Story 2

```bash
# Every rule about lastRun.txt, written at once before a line of it is read:
Task: "Precedence: the middle run wins in gatling/discover_test.go"
Task: "Pointers to nothing fall through in gatling/discover_test.go"
Task: "Containment: nothing outside the root in gatling/discover_test.go"
Task: "Shape: CRLF, whitespace, several names, the 64 KiB cap in gatling/discover_test.go"
Task: "Multi-line: the newest of the named runs in gatling/discover_test.go"
Task: "The three FoundBy values are distinguishable in gatling/discover_test.go"
Task: "The recorded lastRun.txt in gatling/discover_corpus_test.go"
```

---

## Commit Mapping (AGENTS.md: 1 issue = 1 commit)

| Commit | Tasks | Green on its own |
|---|---|---|
| `docs(speckit): add 009-gatling-run-discovery spec/plan/tasks` | the `specs/` artifacts — lands **before** any code | n/a |
| `feat(gatling): find a run without being told its generated name (#11)` | T003–T040 | `go build ./... && go test ./...` |

T001, T002, T041 and T042 are checks and measurements, not changes. This milestone holds one issue,
so there is one implementation commit; nothing else lands in this PR, and an improvement found on
the way that is not in scope goes to its own PR (AGENTS.md, *Never: opportunistic refactors outside
scope*).

If T003 is declined, **two** tasks drop — T003 and T023 — and nothing else changes: every other test
builds its own tree under `t.TempDir()`. Record that decision and its reasoning in the PR, since the
plan's Principle III row is ticked on the assumption the recording is taken.

#11 belongs to milestone **v0.0.9 Finding the run** and is closed by the commit that lands on `main`.
`scripts/check-linkage.sh --pr N` is the merge gate.

---

## Implementation Strategy

### MVP (User Story 1 only)

1. Phase 1 Setup → 2. Phase 2 Foundational → 3. Phase 3 US1 → **stop and validate**: point
`galaxio report` at a project and it reads the newest run. For every Gradle and sbt user that is
already the whole feature, because they never have a `lastRun.txt` to prefer (research R2).

### Incremental Delivery

1. Setup + Foundational → the four identifiers exist and are documented
2. US1 → a results root resolves → the MVP, and the shippable half
3. US3 → a named run and a named log work → the migration path for today's callers
4. US2 → Gatling's own record is preferred where a Maven build left one → #11's headline case
5. US4 → failures say where they looked
6. Polish → fuzz, benchmark, example, CHANGELOG, README, coverage, the full gate

US3 can be pulled ahead of US2: it is an early return that touches no code US2 needs.

---

## Implementation record (2026-09-08, completed 2026-09-09)

**All 42 tasks are done.** T003 and T023 were left open at first because recording a real
`lastRun.txt` needs a Maven build and the corpus simulation was an sbt project. The maintainer chose
to record, so `testdata/corpus/gatling/simulation/pom.xml` was added — the same sources, the same
stub, a second build tool — and three Maven runs were made into one results root.

**The recording changed the feature, which is the argument for having made it.** Two things planning
had settled by reading `gatling-maven-plugin`'s bytecode turned out to be wrong or incomplete, and
neither was reachable by more reading:

| What was believed | What the recording showed |
|---|---|
| `mvn gatling:test` writes a `lastRun.txt` | It does not. The write is gated on `failOnError` being **false**, against a default of true — found because run 1 produced a run directory and no file. |
| A run directory is named in **local** time | It is **UTC**. The +04:00 machine produced `corpussimulation-20260909022708912` for a run starting `20260909022708.912` UTC; the sbt entries agree, so it is Gatling's behaviour and not a plugin's JVM argument. |

Both are corrected in research R1 and R4, in `spec.md`, in contract 2, in the API doc comments and in
`RECORDING.md`. The UTC correction also strengthens the ordering rule: a *local* `yyyyMMddHHmmssSSS`
would not sort monotonically across a daylight-saving fall-back, and a UTC one does.

The recording is also #11's acceptance case as an artefact rather than a fixture — three real run
directories whose `lastRun.txt` names the **middle** one — which took three runs, because the plugin
writes only the directories its own execution created.

**Gates at completion** — every row of the constitution's Quality Gates table:

| Gate | Result |
|---|---|
| Format | `gofmt -l .` clean |
| Module hygiene | `go mod tidy` leaves the tree unchanged (there is no `go.sum`: the module has no dependencies) |
| Vet | clean |
| Build | `go build ./...` succeeds |
| Lint | `golangci-lint` 2.12.2 — the pinned CI version — `--build-tags=integration ./...`: 0 issues |
| Tests | `go test -race -shuffle=on ./...`: 999 pass, up from 928 |
| End-to-end | `go test -tags=integration ./...`: 1075 pass, including four over the new recording |
| Dependency boundary | `gatling/` still imports the standard library only |
| Coverage | `gatling/` 96.8% against the 90% decoder floor; every package above its floor |
| Shell gates | all four `*_test.sh` suites pass |

`FuzzLastRun` ran 30s / ~198 000 executions with no crash. Benchmark figures are recorded in
[plan.md](plan.md) under Performance Goals.
