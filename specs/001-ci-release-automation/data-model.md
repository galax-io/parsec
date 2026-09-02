# Phase 1 Data Model: Continuous Verification, Tag-Driven Release and Dependency Automation

**Feature**: `001-ci-release-automation` | **Date**: 2026-09-02 | **Spec**: [spec.md](./spec.md)

Nothing here is a load-test result, so nothing here belongs in `model/` (Principle I). These are
pipeline and corpus entities: two of them are real Go types in `internal/e2e`, one is a committed
JSON file, and the rest exist as workflow jobs and GitHub objects. Each says which it is.

---

## Corpus entry — committed files + `internal/e2e.Entry`

One recorded run of one tool at one version in one artefact format. The unit the whole end-to-end
suite is built on, and the thing every future decoder spec will add to.

Everything the harness needs is in the path and the filenames — there is no metadata file. A
manifest was considered and dropped: with no decoder there is nothing to compare against, so any
schema written now would be guessed. FR-032 records the comparison as required future work, and the
first decoder chooses the format for its expected values when it has something to verify.

| Field | Type | Rules |
|---|---|---|
| `dir` | path | `testdata/corpus/<tool>/<version>/<format>/`; the path is the identity |
| `tool` | string | read from the path; matches the package that will decode it (`gatling`, `jmeter`, …) |
| `version` | string | read from the path; the exact tool version that produced the artefact, never a range |
| `format` | string | read from the path; `text` or `binary` |
| `files` | []path | the entry's regular files, sorted — the artefact as produced, plus the report |

**Why `format` is a path segment.** Gatling writes `simulation.log` as text through 3.12.x and as
binary from 3.13.0, and this repository plans separate codecs for the two. Without the segment both
recordings of one version would collide on the same directory, and `format` would have nowhere to
be read from — leaving FR-011's "which artefact format each case covered" unanswerable and FR-012's
"a new artefact format MUST be blocked until a case exists" with no mechanism.

**Validation** — a malformed entry fails the run rather than being skipped (FR-013):

- the path names a tool, a version and a known format;
- at least one file is present, readable and non-empty;
- nothing in it is marked as a fixture;
- the entry holds **two or more files**, or exactly one file plus a `NO-REPORT` file naming the tool
  version and saying why it produced none.

**That last rule is the whole of FR-031.** The constitution requires a recording to carry the report
its tool produced, because an archived run cannot be re-run and Gatling stopped producing reports in
3.13.5 — so the moment of recording is the only moment that file can be captured. A rule enforced
only in prose is enforced by whoever remembers it; a one-file entry without an explicit `NO-REPORT`
is refused, which is what makes a forgotten report and a genuinely report-less tool version
distinguishable rather than identical.

**Invariant**: a hand-made artefact is a fixture, not corpus, and must say so in its name
(Principle III). The harness rejects any entry whose artefact filename is marked as a fixture.

## End-to-end case — `internal/e2e`, registered at run time

One executed comparison. Created by a test, registered with the registry, reported in the inventory.

| Field | Type | Rules |
|---|---|---|
| `tool`, `version`, `format` | string | copied from the corpus entry it ran against |
| `level` | enum | `harness` or `decoder` — see below |
| `assertions` | int | how many checks the case made; zero means the case proved nothing and fails |
| `outcome` | enum | `passed` or `failed`, recorded when the case ends |

**`level` is the honesty field.** `harness` means the case exercised discovery and artefact
readability but decoded and compared nothing — the state of the scaffold case until the first decoder
lands (research D4). `decoder` means the recorded record stream and the derived statistics were
compared against the recording and the tool's own report. The inventory prints the level, so a
`harness` case can never be mistaken for decoder coverage, and FR-012 is checked against `decoder`
cases only.

**State**: a case registers as soon as it starts, and records its outcome when it ends. It counts
cases that **executed**, not cases that passed.

That distinction is load-bearing. If a failing case did not register, a suite of one failing case
would report `no end-to-end case executed` — the message FR-013 reserves for a missing corpus — and
the two situations would be indistinguishable at exactly the moment a maintainer needs to tell them
apart. A run of three cases with one failure reports three executed and names the one that failed.

## Case registry — `internal/e2e`, package-level

The single thing that makes FR-010 true. Collects registered cases during a run; `TestMain` inspects
it after `m.Run()` and exits non-zero when it is empty, printing `no end-to-end case executed`.

**Rules**: append-only within a run; concurrency-safe, because `-shuffle=on` and parallel subtests are
both in play; never persisted — a registry that survives a run could report yesterday's coverage.

## Gate — a job in `verify.yml`

One named check with one pass/fail outcome and a documented reason for existing.

| Field | Rules |
|---|---|
| `name` | the job id; it is a contract, and renaming one changes what reviewers and rulesets refer to |
| `command` | what it runs |
| `blocking` | whether a failure blocks the change today |

The gate set itself — which gates exist, what each runs, and which are blocking — is listed once, in
[contracts/workflows.md](./contracts/workflows.md). It is deliberately not repeated here: the two
copies of an earlier draft had already worded the `e2e` and `deps` rows differently, which is how a
reader ends up with two normative answers.

**Rule**: the gate set is defined once, in `verify.yml`, and consumed by both `ci.yml` and
`release.yml`. Adding a gate means adding a job there. `ci.yml` calls the whole workflow as a single
`uses:` job, so nothing in `ci.yml` has to change — and nothing can, since a reusable workflow's
jobs are not addressable from its caller.

## Verification run — a `ci.yml` run

The complete verdict for one proposed change. `ci.yml` has three jobs: `changes` (the path filter),
`gates` (one `uses:` call of `verify.yml`, which runs the whole gate set inside it) and `verify`
(the aggregate over the other two).

**States**: `pending` → `passed` | `failed` | `skipped-by-path`.

**Rules**:

- `verify` runs with `if: always()`, needs `[changes, gates]`, and fails when either reports
  `failure` or `cancelled`. A `skipped` job is not a failure — that is what lets a
  documentation-only change pass (FR-002) while `verify` remains the one required check (FR-006).
- A `uses:` job fails when any job inside the called workflow fails, so `gates` carries the verdict
  of all seven gates in one result. The gates cannot be listed individually in `verify`'s `needs`,
  and do not need to be.
- `verify` is the only status check branch protection requires. Its name is a contract: renaming it
  silently unprotects the trunk.
- The author can suppress a run with `[skip ci]`, and then `verify` never reports and the change
  cannot merge. Suppressing is not passing (FR-005).

## Release — a GitHub Release, created once

| Field | Source | Rules |
|---|---|---|
| `tag` | the pushed tag | `vX.Y.Z`; never reused, never deleted once publication starts |
| `branch` | branches containing the tagged commit | `main`, or exactly `release/X.Y.0` for the tag's major.minor |
| `milestone` | `check-linkage.sh --for-tag` | every issue closed, every pull request merged |
| `notes` | `git-cliff` over the range since the previous tag | grouped by type; unconventional commits land in a catch-all, never dropped |
| `proxy` | `proxy.golang.org/…/@v/<tag>.info` | polled **before** the Release is created; the run fails if it does not resolve |

**State transitions**: `tag pushed` → `guard` → `verify` → `publish`. Every arrow can refuse.

The Release object is created in the **last step** of `publish`, after the notes are generated and
after the proxy has been confirmed to resolve, so every step that can fail transiently fails before
anything irreversible exists: a re-run then behaves like a first run (FR-020). A failure at the
creation step itself means the version is out, and the existence check refuses the re-run (FR-022).

The tag is not part of this state machine. For a public Go module the push already published the
version to the module proxy before `guard` ran — see [contracts/release.md](./contracts/release.md).
This entity describes what the workflow creates, which is the Release and nothing else.

## Tracked family — a row in the ownership table

One group of versioned things watched by exactly one updater. Full table in
[contracts/dependency-ownership.md](./contracts/dependency-ownership.md).

| Field | Rules |
|---|---|
| `name` | Go modules, GitHub Actions, advisories, pinned linter, pinned CI tools, Go toolchain |
| `owner` | exactly one of Dependabot or Renovate — enforced in each config, not by convention |
| `schedule` | weekly |
| `labels` | `dependencies` plus one of `go`, `ci`, `tooling` |

**Invariant**: ownership is enforced by disabling the other updater's manager for that family.
Renovate's config disables `github-actions` outright and restricts `gomod` to the toolchain dep type,
so the two cannot see the same dependency (FR-027).

## Update proposal — a pull request from an updater

| Field | Rules |
|---|---|
| `family` | exactly one |
| `grouping` | minor and patch grouped per family per cycle; major on its own (FR-026) |
| `labels` | its family's labels (FR-028) |
| `author` | Dependabot, or the GitHub App Renovate runs as |

**Rule that decides the design**: an update proposal must be verified like any other change (FR-025).
A pull request authored with the built-in `GITHUB_TOKEN` does not trigger `pull_request` workflows,
so Renovate authenticates as a GitHub App instead (research D5). Dependabot is unaffected — its pull
requests trigger workflows, with a read-only token, and no gate here needs a secret.

**Never**: auto-merged. Out of scope for this feature, deliberately.
