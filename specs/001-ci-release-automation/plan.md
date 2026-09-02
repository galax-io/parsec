# Implementation Plan: Continuous Verification, Tag-Driven Release and Dependency Automation

**Branch**: `001-ci-release-automation` | **Date**: 2026-09-02 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/001-ci-release-automation/spec.md`

## Summary

Close the gap between what this repository *says* is enforced and what actually is. The constitution
publishes a Quality Gates table, `AGENTS.md` documents a tag-driven release process, and
`scripts/check-linkage.sh` implements the milestone rules — but no workflow runs the end-to-end
suite, no workflow runs the linkage script, `release.yml` does not exist, and the two versions the
repository pins by hand (`golangci-lint v2.12.2`, Go `1.25`) are both already stale against
`v2.13.2` and `go1.27.1`.

The approach: one reusable workflow holding the gate set, called by both the pull-request path and
the release path so they can never diverge; a single aggregate status check that reports on every
pull request including documentation-only ones; an end-to-end harness in `internal/e2e` whose
`TestMain` fails a run in which no case executed, proven from day one by a real recorded Gatling run
committed to the corpus; a `release.yml` that refuses a tag before it publishes anything and creates
the GitHub Release as its last act; and Renovate configured to own only what Dependabot cannot see,
authenticated by a GitHub App because `GITHUB_TOKEN`-authored pull requests do not trigger CI.

## Technical Context

**Language/Version**: Go 1.25 (`go.mod` is authoritative). Workflow definitions are GitHub Actions
YAML; gate helpers are POSIX-ish bash matching `scripts/check-linkage.sh`.

**Primary Dependencies**: no new Go module dependency. `go.mod` stays empty of requirements.
`.github/dependabot.yml` gains one line (`update-types: [minor, patch]` on the actions group) so a
major action upgrade cannot join the grouped proposal, against FR-026.
CI tooling is pinned by version and never resolved as `@latest`: `actions/checkout@v7`,
`actions/setup-go@v7`, `golangci/golangci-lint-action@v9`, `actions/create-github-app-token`,
`renovatebot/github-action`, `orhun/git-cliff-action`, and `golang.org/x/vuln/cmd/govulncheck` run
via `go run` at a pinned version so it never enters `go.mod`.

**Storage**: N/A. The corpus is committed files; the pipeline persists nothing between runs beyond
the runner cache.

**Testing**: stdlib `testing`, table-driven. Unit gate `go test -race -shuffle=on ./...`; end-to-end
gate `go test -tags=integration -race -shuffle=on ./internal/e2e/...` with `-json` parsed for the
executed-case inventory. Corpus under `testdata/corpus/<tool>/<version>/`.

**Target Platform**: `ubuntu-latest` GitHub-hosted runners. The module itself remains any Go 1.25
target.

**Project Type**: library (Go module `github.com/galax-io/parsec`). This feature adds no exported
identifier; all Go code lands in `internal/`.

**Performance Goals**: complete verdict on a typical pull request within 10 minutes (SC-002); the
fast gates — formatting, module hygiene, vet, build — report within 3 minutes. Achieved by a
fast-failing `quick` job with the remaining gates in parallel, on `setup-go`'s default module and
build caching.

**Constraints**: fork pull requests get every gate (none needs a secret) and no write permission;
`GITHUB_TOKEN` is read-only by default with write granted per job; the end-to-end suite must never
execute a load-testing tool (FR-015); the release must create the GitHub Release last (FR-020,
FR-022); Renovate must not author pull requests with `GITHUB_TOKEN` (research D5).

**Scale/Scope**: 4 workflow files, 1 Renovate config, 1 git-cliff config, 3 shell helpers, 1 internal
Go package, 1 recorded Gatling run. 7 gates in the reusable set; 6 tracked dependency families across
2 updaters.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Source: `.specify/memory/constitution.md` v1.1.0 — the amendment this feature triggered, so the check below is run against the version that ships with it, not the one it replaced.

**Pre-Phase 0** — evaluated before research; **Post-Phase 1** — re-evaluated after the design below.
Both passes reached the same verdicts; where Phase 1 sharpened a reason it is noted.

- [x] **I. Canonical Model First** — **N/A**. The feature adds no result data and no `model/` type.
      The end-to-end harness lives in `internal/e2e` and exports nothing to consumers; a corpus entry
      describes a recording, not a load-test result. *Post-Phase 1*: confirmed — `data-model.md`
      contains only pipeline and corpus entities, none of which is consumer-facing.
- [x] **II. Version-Gated, Streaming Decoders** — **N/A**. Nothing here decodes a tool artefact. The
      scaffold case reads a recording and decodes no records; the corpus path carries the tool version
      so that when the first decoder lands, the case routes through that decoder's own gate rather
      than around it. *Post-Phase 1*: confirmed — the version gate arrives with the decoder that needs
      it, and FR-032 records the comparison as required future work rather than guessing at it now.
- [x] **III. Golden-Corpus Testing** — **PASS**. This feature builds the corpus mechanism the
      principle depends on and commits the first real recording (research D4). Coverage is measured
      and reported from day one; enforcement is deferred to the first decoder package, which is the
      follow-up the constitution's own Sync Impact Report already records and dates to v0.0.2. The
      deferral is implemented as an unpassed `--enforce` flag, not as absent code (research D10).
- [x] **IV. Minimal, Explicit Dependencies** — **PASS**. No module dependency added; `go.mod` keeps
      no `require` block. `model/` and `gatling/` do not exist yet and are untouched; the `deps` job
      that guards them is carried into the reusable workflow unchanged. CI tools are pinned, and
      `govulncheck` runs through `go run` at a pinned version so it never reaches `go.mod`.
- [x] **V. Compatibility-Sensitive Public API** — **PASS**. No exported identifier is added, changed
      or removed; every Go file lands under `internal/`. A `CHANGELOG.md` entry is required and
      planned: the release process and the gate set are user-visible to contributors and to the three
      downstream consumers who will read the release notes this feature starts generating.
- [x] **VI. Idiomatic, Simple Go** — **PASS**. The Go added is one small internal package: corpus
      discovery, a case registry and `TestMain`. No abstraction beyond what the scaffold case needs —
      the corpus manifest an earlier draft carried was dropped for exactly this reason (research D4). `.golangci.yml` gains `run: build-tags: [integration]`: without it golangci-lint never loads a tagged file, so the lint gate would report an empty package for the only Go this feature adds.
- [x] **Workflow** — **PASS**. Milestone **v0.0.1 Scaffold** (lowest-numbered open milestone; the
      spec's Assumptions record the reasoning). Spec artifacts commit as `docs(speckit): …` before
      any implementation commit. Release-workflow changes are on the constitution's "ask first" list
      — the maintainer's feature request is that authorisation, and the three scope decisions were
      put to them during `/speckit-specify`.

**No Complexity Tracking rows.** No gate is violated.

## Project Structure

### Documentation (this feature)

```text
specs/001-ci-release-automation/
├── plan.md                          # This file
├── spec.md                          # Feature specification
├── research.md                      # Phase 0: 10 decisions, verified starting state
├── data-model.md                    # Phase 1: pipeline and corpus entities
├── quickstart.md                    # Phase 1: record a run, run the gates, cut a release
├── contracts/
│   ├── workflows.md                 # triggers, jobs, the single required check, permissions
│   ├── release.md                   # tag contract, preconditions, outputs, failure modes
│   └── dependency-ownership.md      # family → owner, labels, grouping
├── checklists/requirements.md       # spec quality checklist (complete)
└── tasks.md                         # /speckit-tasks output — NOT created here
```

### Source Code (repository root)

```text
.github/
├── workflows/
│   ├── verify.yml                   # NEW  on: workflow_call — the whole gate set, one definition
│   ├── ci.yml                       # REWRITTEN  path filter + calls verify.yml + aggregate check
│   ├── release.yml                  # NEW  on: push tags v*.*.* — guard → verify → publish
│   └── renovate.yml                 # NEW  schedule + workflow_dispatch, GitHub App token
└── dependabot.yml                   # UNCHANGED — already scoped to gomod + github-actions

renovate.json5                       # NEW  custom managers + gomod toolchain only; everything else off
cliff.toml                           # NEW  commit parsers matching this repo, catch-all group

scripts/
├── check-linkage.sh                 # CHANGED  — tag→milestone resolution, empty-milestone refusal,
│                                    #             and the merged-range audit FR-018 needs
├── check-linkage_test.sh            # NEW      — hermetic regression tests (gh replaced by a stub)
├── check-coverage.sh                # NEW  per-package coverage table; --enforce flag, not yet passed
└── e2e-inventory.sh                 # NEW  parses go test -json into the executed-case inventory

internal/e2e/                        # NEW  //go:build integration
├── doc.go                           # why this suite never skips (constitution reconciliation)
├── corpus.go                        # corpus discovery from the path
├── registry.go                      # executed-case registry
├── main_test.go                     # TestMain: empty registry ⇒ exit non-zero
└── scaffold_test.go                 # the harness-level case over the recorded Gatling run

testdata/corpus/gatling/<version>/<format>/  # NEW  one real recorded run; format is a path
                                     #      segment so text and binary logs of one version can coexist
├── simulation.log                   # committed exactly as Gatling produced it
└── <tool report as produced>        # the run's own statistics — captured now, read by the
                                     # first decoder; unrecoverable later (FR-031)

CHANGELOG.md                         # Unreleased ▸ Added: verification, release and update automation
```

**Structure Decision**: every Go file lands in `internal/e2e`, so the module's public API surface is
unchanged and Principle V is satisfied by construction. The harness is one package rather than a
`corpus/` + `e2e/` pair because nothing needs the split yet (Principle VI); when a second tool joins
the corpus and the discovery logic grows, splitting it out is a mechanical move. `testdata/corpus/`
follows the layout the constitution already fixes, so the first decoder finds its recordings where
it expects them. No top-level `e2e/` package: that would be public API for a test harness.

## Phase sequencing and what blocks what

```text
1. verify.yml + ci.yml rewrite      ── the aggregate check; everything else is verified by it
2. internal/e2e harness             ── TestMain + registry; fails loudly with an empty corpus
3. record the Gatling run           ── maintainer-run, needs a JVM; unblocks 2's scaffold case
4. check-coverage.sh                ── reporting half; --enforce stays off until v0.0.2
5. release.yml + cliff.toml         ── depends on 1 for the reusable verify call
6. renovate.json5 + renovate.yml    ── depends on the GitHub App secrets (maintainer action)
```

Steps 3 and 6 need the maintainer at a keyboard — a Gatling run on a machine with a JVM, and a
GitHub App installed with its credentials stored as secrets. Neither blocks the others; both are
called out in [quickstart.md](./quickstart.md). Step 2 is written so that until step 3 lands the
suite fails with `no end-to-end case executed`, which is the correct state for a gate that is live
and has nothing to run — not a reason to soften the rule.

## Repository settings this feature cannot change from a file

Three things live in repository settings rather than in the tree, and they are what turn FR-006 from
a statement into a fact. Exact commands are in [quickstart.md](./quickstart.md); they are left for
the maintainer to run because they alter repository configuration:

1. A branch-protection ruleset on `main` requiring the `verify` status check, with bypass disabled.
2. The labels Renovate applies — Renovate does not create labels, unlike Dependabot.
3. Dependabot security updates enabled, which is the FR-029 half that `govulncheck` does not cover.

Until (1) exists the pipeline reports honestly but nothing enforces the report, and SC-010 is not met.
