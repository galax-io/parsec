---

description: "Task list for 001-ci-release-automation"
---

# Tasks: Continuous Verification, Tag-Driven Release and Dependency Automation

**Input**: Design documents from `/specs/001-ci-release-automation/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/)

**Tests**: Test tasks are REQUIRED (constitution Principle III). Every Go file added here lands in `internal/e2e` and carries table-driven tests written first, which MUST fail before the implementation task starts. The workflow phases carry validation tasks instead of Go tests — a gate that has never been seen to refuse anything is not a gate, so each one is deliberately broken once and observed.

**Organization**: Grouped by user story so each can be implemented, tested and merged independently.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1–US4)
- Every task names the exact file it touches

## Path Conventions

Per [plan.md](./plan.md) "Source Code":

- **Workflows**: `.github/workflows/` — `verify.yml` holds the gate set; `ci.yml` and `release.yml` call it
- **Go**: `internal/e2e/` only. This feature adds no exported identifier (Principle V)
- **Shell**: `scripts/`, matching the style of the existing `scripts/check-linkage.sh`
- **Corpus**: `testdata/corpus/<tool>/<version>/`, recorded from a real run, committed verbatim, with the tool's own report beside it (FR-031)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Spec-first commit, issue tracking, and the directory skeleton everything else lands in

- [ ] T001 Commit the spec artifacts in `specs/001-ci-release-automation/` as `docs(speckit): add 001-ci-release-automation spec/plan/tasks`, before any implementation commit (constitution: Spec-first)
- [ ] T002 Create one GitHub issue per implementation PR below (foundational, US1, US2-harness, US2-corpus, US3, US4, polish) under milestone **v0.0.1 Scaffold**, so each commit can carry `Closes #N` (constitution: 1 issue = 1 commit). `/speckit-taskstoissues` can generate these  
- [ ] T003 [P] Record the corpus layout `testdata/corpus/<tool>/<version>/<format>/` in `README.md` — git tracks files, not directories, so the first real file creates the path; committing `.gitkeep` into directories that receive real files later is the add-then-remove churn AGENTS.md forbids
- [ ] T004 [P] Add `run: build-tags: [integration]` to `.golangci.yml`, or run the `lint` job as a matrix over both build configurations. golangci-lint only loads files whose build constraints are satisfied, so without this every file in `internal/e2e` is invisible to all enabled linters and the job reports an empty package — indistinguishable from clean code, which is why "confirm it needs no change" would return the wrong answer

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: One definition of the gate set, callable from both the pull-request path and the release path

**⚠️ CRITICAL**: US1 and US3 both consume `verify.yml`. Nothing in Phase 3 or Phase 5 can start until it exists.

- [ ] T005 Create `.github/workflows/verify.yml` with `on: workflow_call` and `permissions: contents: read`, carrying the four gates that exist today, moved out of `ci.yml` unchanged in behaviour: `quick` (gofmt, `go mod tidy` clean, `go vet`, `go build`), `lint` (`golangci/golangci-lint-action` at the pinned version), `test` (`go test -race -shuffle=on ./...`), `deps` (stdlib-only boundary for `./model/...` and `./gatling/...`). Job names are a contract — see [contracts/workflows.md](./contracts/workflows.md). Fix the `deps` check while moving it: `go list -deps` prints the queried package itself, so the current pattern flags `github.com/galax-io/parsec/model` as a third-party dependency the moment `model/` exists, and with `model/` absent the `if go list` guard makes it a silent no-op. Exclude the module path, and make the no-package case say so in the output instead of passing quietly
- [ ] T006 Rewire `.github/workflows/ci.yml` to call `verify.yml` via `uses:`, keeping its current triggers and concurrency block for now, so the trunk stays green while Phase 3 rebuilds the trigger logic
- [ ] T007 Validate `.github/workflows/verify.yml` by breaking each of the four gates in turn and confirming each is reported by name: an unformatted file (`quick`), a `//nolint` with no explanation (`lint`), a failing test (`test`), and a third-party import under `model/` (`deps`). The four breakages an earlier draft listed — unformatted file, untidy `go.mod`, vet finding, failing build — are all steps of `quick` alone, and would have left `lint`, `test` and `deps` unexercised at this checkpoint

**Checkpoint**: the gate set exists once and is callable. US1, US2 and US3 can now proceed in parallel.

---

## Phase 3: User Story 1 - Every change gets one complete verdict (Priority: P1) 🎯 MVP

**Goal**: One required status check that reports on every pull request — green for a documentation-only change, red for a change that breaks any gate, and never absent.

**Independent Test**: break exactly one gate and confirm the pull request is blocked with that gate named; then open a `README.md`-only pull request and confirm it is mergeable with the code gates skipped.

### Tests for User Story 1 (REQUIRED — write first, MUST fail before implementation) ⚠️

- [ ] T008 [P] [US1] Write `scripts/check-coverage_test.sh`: a package with no statements reports `n/a` not `0%`; `--enforce` fails on a package below its floor; without `--enforce` the same input exits 0. Must fail — the script does not exist yet
- [ ] T009 [P] [US1] Record in [quickstart.md](./quickstart.md) US1 the four break-one-gate branches and the exact expected failure text, so T017 and T018 have a pass/fail definition rather than a judgement call

### Implementation for User Story 1

- [ ] T010 [P] [US1] Write `scripts/check-coverage.sh`: build a per-package table from a `go test -coverprofile` profile, report `n/a` for packages with no statements, accept `--enforce` applying the 90% decoder / 80% overall floors, and exit 0 without it (research D10)
- [ ] T011 [P] [US1] Add the `vuln` job to `.github/workflows/verify.yml` running `go run golang.org/x/vuln/cmd/govulncheck@<pinned> ./...` — pinned, never `@latest`, so it never enters `go.mod` (Principle IV)
- [ ] T012 [US1] Add the `coverage` job to `.github/workflows/verify.yml` calling `scripts/check-coverage.sh` **without** `--enforce` and writing its table to `$GITHUB_STEP_SUMMARY` (depends on T010, T011 — same file)
- [ ] T013 [US1] Add the `changes` job to `.github/workflows/ci.yml`: check out with `fetch-depth: 0` (a depth-1 clone does not contain the base commit and `git diff` against a missing object exits 128), compute changed paths with `git diff --name-only` against an event-specific base — `github.event.pull_request.base.sha` on a pull request, `github.event.before` on a push, `code=true` when neither resolves — and output a `code` boolean that is false for changes touching only `**.md`, `docs/`, `specs/` and `.specify/`. `ci.yml` runs on both events and the pull-request field is empty on a push, which would otherwise mark every trunk push documentation-only and report green having verified nothing
- [ ] T014 [US1] Remove `paths-ignore` from the `.github/workflows/ci.yml` triggers and gate the `verify.yml` call on `needs.changes.outputs.code == 'true'` — `paths-ignore` and a required check cannot coexist (research D1)
- [ ] T015 [US1] Add the aggregate `verify` job to `.github/workflows/ci.yml` with `if: always()` and `needs: [changes, gates]`, failing when `contains(needs.*.result, 'failure')` or `contains(needs.*.result, 'cancelled')` and passing when `gates` is `skipped`. `needs` accepts only job ids from the same file, so the gates inside `verify.yml` are neither listed here nor reachable — the `uses:` job already fails when any of them fails
- [ ] T016 [US1] Add `.github/ruleset-main.json` declaring the branch-protection ruleset that requires the `verify` check with bypass disabled, so the maintainer action in [quickstart.md](./quickstart.md) is a single `gh api` call and not a description

### Validation for User Story 1

- [ ] T017 [US1] Break each gate in turn on a branch and confirm `verify` fails naming the gate, per [quickstart.md](./quickstart.md) US1: unformatted `.go` file, third-party import under `model/`, failing test, `//nolint` with no explanation  
- [ ] T018 [US1] Open a branch touching only `README.md` and confirm the gate jobs report `skipped`, `verify` reports green, and the change is mergeable on review alone (FR-002 + FR-006 together)  
- [ ] T019 [US1] Add the `CHANGELOG.md` entry under Unreleased ▸ Added for the verification pipeline and its single required check

**Checkpoint**: US1 is complete and independently valuable — the repository can no longer accept a red change, and documentation still merges freely.

---

## Phase 4: User Story 2 - The end-to-end suite always runs and can never pass by skipping (Priority: P1)

**Goal**: A live end-to-end gate, proven on day one by a real recorded Gatling run, that fails a run in which no case executed.

**Independent Test**: move the corpus aside and confirm the suite exits non-zero with `no end-to-end case executed` rather than reporting `ok ... [no tests to run]`.

### Tests for User Story 2 (REQUIRED — write first, MUST fail before implementation) ⚠️

- [ ] T020 [P] [US2] Write `internal/e2e/registry_test.go`: table-driven tests that an empty registry is detected as empty, that a registry with one case is not, and that concurrent registration is safe under `-race` (the suite runs with `-shuffle=on` and parallel subtests)
- [ ] T021 [P] [US2] Write `internal/e2e/corpus_test.go`: table-driven tests over corpus discovery — a directory that names no tool or version, an entry with no artefact, an empty artefact, an unreadable artefact, an artefact whose filename marks it a fixture. Each must **fail** the run, never skip (FR-013)

### Implementation for User Story 2

- [ ] T022 [P] [US2] Implement `internal/e2e/corpus.go` with **no build tag**: discover entries under `testdata/corpus/`, deriving tool, version and format from the path. The harness is ordinary code covered by the ordinary unit-test gate; only the cases that read the corpus (`main_test.go`, `scaffold_test.go`) carry `//go:build integration`. Tagging the harness while leaving its tests untagged makes `go test ./...` fail to build, and tagging the whole package makes T020/T021 never run in the only job that runs unit tests, per [data-model.md](./data-model.md). No metadata file — the expected-value format is the first decoder's decision (research D4, FR-032)
- [ ] T023 [P] [US2] Implement `internal/e2e/registry.go`: a concurrency-safe registry holding tool, version, format, `level` (`harness` or `decoder`) and assertion count, plus the exported decision function `TestMain` calls so the empty-registry rule is unit-testable
- [ ] T024 [P] [US2] Write `internal/e2e/doc.go` stating why this suite never skips: the constitution's `t.Skip`-when-the-tool-is-unavailable rule is about an external tool binary, and these cases replay committed recordings, so a missing recording is a failure (research D3). The next reader will otherwise expect a skip
- [ ] T025 [US2] Implement `internal/e2e/main_test.go`: `TestMain` runs `m.Run()`, then exits non-zero printing `no end-to-end case executed` when the registry is empty (depends on T023)
- [ ] T026 [US2] Record a real Gatling run in the 3.11.5 – 3.12.x range into `testdata/corpus/gatling/<version>/`: `simulation.log` **and the run's own statistics report**, both committed byte for byte. The report is unreadable-later information — an archived run cannot be re-run and Gatling stopped producing reports in 3.13.5 (FR-031). **Maintainer action: needs a JVM and a Gatling distribution.** Procedure in [quickstart.md](./quickstart.md) US2  
- [ ] T027 [US2] Implement `internal/e2e/scaffold_test.go`: discover the recorded entry, read the artefact, assert it is present and non-empty, and register the case with `level=harness`. It compares nothing — there is no decoder to compare against, and FR-032 records that comparison as future work rather than guessing at it now (depends on T022, T023, T026)
- [ ] T028 [P] [US2] Write `scripts/e2e-inventory.sh` parsing `go test -json` into the executed-case inventory shown in [contracts/workflows.md](./contracts/workflows.md), and exiting non-zero when the count is zero
- [ ] T029 [US2] Add the `e2e` job to `.github/workflows/verify.yml` running `go test -tags=integration -race -shuffle=on -json ./internal/e2e/...` piped through `scripts/e2e-inventory.sh` into `$GITHUB_STEP_SUMMARY`, with `shell: bash` and `set -o pipefail` on the step. Nothing is added to `ci.yml`: the aggregate needs the `gates` call job, not the individual gates. Without `pipefail` the job takes the script's exit status and discards the test result entirely, so a failing case with a satisfied inventory reports green

### Validation for User Story 2

- [ ] T030 [US2] Move `testdata/corpus/gatling` aside, run `go test -tags=integration ./internal/e2e/...` and confirm it exits non-zero with `no end-to-end case executed` — **not** `ok ... [no tests to run]`. This is the check the whole story exists for
- [ ] T031 [US2] Truncate the recorded artefact to zero bytes and confirm the run fails naming the unreadable artefact rather than skipping (FR-013); restore it with `git checkout`
- [ ] T032 [US2] Add the `CHANGELOG.md` entry under Unreleased ▸ Added for the end-to-end harness and the first corpus recording

**Checkpoint**: the e2e gate is live and has been observed refusing an empty run. Every later decoder plugs into a harness already proven.

---

## Phase 5: User Story 3 - Releasing is pushing a tag (Priority: P2)

**Goal**: A version tag on an allowed branch publishes a release with generated notes; anything else publishes nothing and says why.

**Independent Test**: push a version tag that violates a rule and confirm no Release is created and the refusal names the rule.

**Depends on**: Phase 2 — `publish` must run the same gate set as any merged change (FR-023), which means calling `verify.yml`.

### Tests for User Story 3 (REQUIRED — write first) ⚠️

- [ ] T033 [P] [US3] Record in [quickstart.md](./quickstart.md) US3 the four refusal cases and their expected messages — disallowed branch, minor mismatched to its release branch, milestone with an open issue, version already released — so T039 and T040 have a pass/fail definition
- [ ] T034 [P] [US3] Confirm `scripts/check-linkage_test.sh` passes, and that `scripts/check-linkage.sh --for-tag v0.0.1` fails for the right reasons and names them. It cannot run clean while this feature's own issues are open — `--for-tag` refuses any open issue in the milestone, and T002 files them all under v0.0.1 — so a clean run is only meaningful after T052, and asking for one here would make the task unsatisfiable at the point it is scheduled

### Implementation for User Story 3

- [ ] T035 [P] [US3] Write `cliff.toml` with commit parsers for this repository's actual prefixes (`feat`, `fix`, `docs`, `ci`, `chore`, `refactor`, `test`, `perf`) and a catch-all group, so a commit that ignored the convention is listed rather than dropped from the notes (research D7)
- [ ] T036 [US3] Create `.github/workflows/release.yml` triggered on `push` tags `v*.*.*` with `permissions: contents: read`, and its `guard` job: check out with `fetch-depth: 0` (a tag-triggered checkout creates no remote-tracking branches, so the containment test would find none and refuse every release), resolve the containing branches with `git branch -r --contains`, refuse unless `origin/main` or exactly `origin/release/X.Y.0` for the tag's major.minor, run `scripts/check-linkage.sh --for-tag` with `issues: read`, `pull-requests: read` and `GH_TOKEN` set on the job, and refuse when a Release already exists for the tag
- [ ] T037 [US3] Add the `verify` call to `.github/workflows/release.yml` using the reusable workflow from Phase 2 — the same gate set, not a lighter one, because a `release/*` branch carrying a cherry-pick has a tree that was never verified as such
- [ ] T038 [US3] Add the `publish` job to `.github/workflows/release.yml` with `permissions: contents: write` scoped to that job, needing `guard` and `verify`, in this order: generate notes with `orhun/git-cliff-action` at a pinned version → poll `https://proxy.golang.org/github.com/galax-io/parsec/@v/<tag>.info` until it resolves → re-check the Release does not exist → **create the Release last**. The poll comes before the creation: it depends on the tag, not on the Release, and putting the flakiest step after the irreversible one is what would leave a red run with a Release already created that no re-run can ever clear (research D8)

### Validation for User Story 3

- [ ] T039 [US3] Push a version tag on a feature branch and confirm `.github/workflows/release.yml` publishes nothing and names the branch rule; delete the test tag afterwards, which is safe because nothing was published  
- [ ] T040 [US3] Repeat for a tag whose minor does not match its release branch, and for a tag whose milestone still has an open issue; confirm both are refused before `publish` runs  
- [ ] T041 [US3] Add the `CHANGELOG.md` entry under Unreleased ▸ Added for the tag-driven release workflow

**Checkpoint**: the release process documented in `AGENTS.md` is enforced rather than merely written down.

---

## Phase 6: User Story 4 - Dependency updates arrive as batched proposals (Priority: P2)

**Goal**: Two updaters, weekly, with no family owned by both and every proposal verified like any other change.

**Independent Test**: trigger both and confirm the two already-stale pins are proposed — `golangci-lint v2.12.2 → v2.13.2` and `go 1.25 → 1.27.x` — each labelled, each with a `verify` run against it, and neither proposed twice.

### Tests for User Story 4 (REQUIRED — write first) ⚠️

- [ ] T042 [P] [US4] Confirm against [contracts/dependency-ownership.md](./contracts/dependency-ownership.md) that `.github/dependabot.yml` covers exactly `gomod` and `github-actions`, that both groups exclude majors so a major arrives on its own (FR-026), and that no family ends up owned by both updaters

### Implementation for User Story 4

- [ ] T043 [P] [US4] Write `renovate.json5`: disable the `github-actions` manager outright, disable `gomod` then re-enable it only for `depTypes: ["golang", "toolchain"]`, declare `customManagers` with `managerFilePatterns` (not the pre-Renovate-40 `fileMatch`) for the pinned tool versions, weekly schedule, `dependencies`+`tooling` labels, minor/patch grouped and major separate. JSON5 so each disabled manager carries the reason it is disabled inline
- [ ] T044 [P] [US4] Add `# renovate: datasource=... depName=...` marker comments above the two pinned versions that are **not** GitHub Actions — `golangci-lint` (an input to an action) and `govulncheck` (a module run with `go run`) — so the custom manager can find them. `git-cliff-action` is pinned with `uses:` and is therefore Dependabot's under the `github-actions` ecosystem; marking it here as well would give one dependency two owners and produce exactly the duplicate proposal FR-027 forbids
- [ ] T045 [US4] Create `.github/workflows/renovate.yml` on `schedule` and `workflow_dispatch`, minting a GitHub App token with `actions/create-github-app-token` and running `renovatebot/github-action` at a pinned version. **Not `GITHUB_TOKEN`**: pull requests authored with it do not trigger `pull_request` workflows, so proposals would arrive unverified while looking entirely normal (research D5)

### Validation for User Story 4

- [ ] T046 [US4] Run `gh workflow run renovate.yml --repo galax-io/parsec` and confirm the two stale pins are proposed, each labelled, each with a `verify` run against it, and that Dependabot has not opened a competing proposal for the same version  
- [ ] T047 [US4] Add the `CHANGELOG.md` entry under Unreleased ▸ Added for the dependency-update automation and the ownership split

**Checkpoint**: all four stories are independently functional.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [ ] T048 [P] Document the three maintainer actions — the `verify` ruleset, the `tooling` label, the Renovate GitHub App and Dependabot security updates — in `README.md`, since a contributor reading only the repository cannot otherwise tell that the enforcement half lives in repository settings
- [ ] T049 [P] Review `internal/e2e` doc comments: every type and function documented, and `doc.go` stating the never-skip rule (Principle V applies to `internal/` by convention here, since the next decoder author reads this first)
- [ ] T050 Confirm the `verify` check is green on the feature's own pull request. The gates it runs are the same ones a human would re-run by hand, and a manual run can only ever disagree with the pipeline when the pipeline is broken — at which point the human's "confirmed" is the answer that gets believed
- [ ] T051 Run the full [quickstart.md](./quickstart.md) "Definition of done" list and record the resulting package coverage numbers in the PR description, as the constitution requires until the coverage gate becomes blocking  
- [ ] T052 Confirm `scripts/check-linkage.sh` passes for milestone **v0.0.1 Scaffold**: every PR from this feature carries the milestone and every issue from T002 is closed
- [ ] T053 Fix `scripts/check-linkage.sh --for-tag` (issue #23): resolve the milestone titled exactly `vX.Y.Z` before falling back to `vX.Y.0`; match on a version boundary so `v0.0.1` cannot select `v0.0.10`, `v0.0.1.2` or `v0.0.1-rc1`; refuse a milestone with no issues and no pull requests instead of reporting it tag-ready; paginate the milestone queries; and audit pull requests merged since the previous release, which a milestone query cannot see (FR-018)
- [ ] T054 Write `scripts/check-linkage_test.sh`: hermetic regression tests with `gh` replaced by a fixture-backed stub, covering every case above. Constitution Principle III is non-negotiable here — a bug fix ships with a test that fails without the fix, and this one must fail against both the original script and the first partial fix

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies — starts immediately. T001 must land before any implementation commit
- **Foundational (Phase 2)**: depends on Setup. **BLOCKS US1 and US3**, which both consume `verify.yml`
- **US1 (Phase 3)**: depends on Phase 2. Independent of US2, US3, US4
- **US2 (Phase 4)**: depends on Phase 2 only for the `e2e` job in T029; T020–T028 are pure Go and can start as soon as Phase 1 lands
- **US3 (Phase 5)**: depends on Phase 2 for the reusable `verify` call
- **US4 (Phase 6)**: depends on Phase 2 for the files T044 annotates, and on the GitHub App secrets (maintainer action)
- **Polish (Phase 7)**: depends on every story being complete

### The two maintainer-action tasks

**T026** (record a Gatling run, needs a JVM) and **T045** (Renovate GitHub App secrets) both need a human at a keyboard. Neither blocks anything outside its own story:

- Until T026 lands, the e2e suite fails with `no end-to-end case executed`. That is the correct state for a live gate with nothing to run — not a reason to soften the rule
- Until the App secrets exist, `renovate.yml` cannot run. Dependabot is unaffected

### Within Each User Story

- Tests are written first and MUST fail before the implementation task starts
- Types before the code that uses them (T022, T023 before T027)
- The corpus recording before the case that reads it (T026 before T027)
- Scripts before the jobs that call them (T010 before T012; T028 before T029)
- Validation last — a gate is not done until it has been seen to refuse something

### Same-file serialisation (no [P])

- `.github/workflows/verify.yml`: T005 → T011 → T012 → T029
- `.github/workflows/ci.yml`: T006 → T013 → T014 → T015 → T029
- `.github/workflows/release.yml`: T036 → T037 → T038
- `CHANGELOG.md`: T019, T032, T041, T047 land with their own story's PR, never batched

---

## Parallel Example: User Story 2

```bash
# Tests first — different files, no dependencies:
Task: "internal/e2e/registry_test.go — empty-registry detection, concurrent registration under -race"
Task: "internal/e2e/corpus_test.go — reject every malformed corpus entry"

# Then implementation — three different files in parallel:
Task: "internal/e2e/corpus.go — corpus discovery from the path"
Task: "internal/e2e/registry.go — concurrency-safe registry and the empty-registry decision"
Task: "internal/e2e/doc.go — why this suite never skips"

# scripts/e2e-inventory.sh is independent of all of the above and can run alongside them.
```

## Parallel Example: across stories, once Phase 2 lands

```bash
Developer A: US1 — ci.yml trigger logic, the aggregate check, coverage reporting
Developer B: US2 — the internal/e2e harness (T026 blocks only T027)
Developer C: US3 — cliff.toml and release.yml
Developer D: US4 — renovate.json5 and the marker comments
```

---

## Implementation Strategy

### MVP (US1 only)

1. Phase 1 Setup
2. Phase 2 Foundational — the gate set, defined once
3. Phase 3 US1 — the single required check
4. **STOP and VALIDATE**: break a gate, confirm the block; open a docs-only pull request, confirm it merges
5. Enable the branch-protection ruleset

At this point the repository can no longer accept a red change — the largest single gain in the feature, and it stands alone.

### Incremental delivery

1. Setup + Foundational → one gate set, callable
2. **US1** → nothing red merges (MVP)
3. **US2** → decoder correctness has teeth before the first decoder exists
4. **US3** → the documented release process becomes the enforced one
5. **US4** → the two stale pins stop being stale, and stay that way

US1 and US2 are both P1 and both worth having alone. US1 first because US2's `e2e` job is one more gate inside the set US1 makes required.

### Ordering constraint worth respecting

T016 writes the ruleset file, but the maintainer action that applies it should come **after** US2 lands. Making `verify` required while the `e2e` job is red from a missing corpus would block every pull request. Either land T026 first, or apply the ruleset after Phase 4.

---

## Notes

- `[P]` = different files, no dependencies on incomplete tasks
- One tracked issue = one semantic commit, green on its own (`go build ./... && go test ./...`)
- Validation tasks are not optional: a gate that has never refused anything is a gate nobody has tested
- Every `CHANGELOG.md` entry lands in its own story's PR, per the constitution's same-PR rule
- Avoid: batching CHANGELOG entries, editing `.github/dependabot.yml` (the split works because it stays as it is), and widening `verify.yml` in more than one task at a time
