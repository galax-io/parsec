# Phase 0 Research: Continuous Verification, Tag-Driven Release and Dependency Automation

**Feature**: `001-ci-release-automation` | **Date**: 2026-09-02 | **Spec**: [spec.md](./spec.md)

Every version below was read from the live source on 2026-09-02, not from memory.

**These numbers are a snapshot, not a specification.** The authoritative values live in
`.github/workflows/verify.yml` and `go.mod`; the point of the table is the *gap*, and the first
successful Renovate run closes it and makes these figures stale by design. Read them as evidence
that the gap existed, never as the current state, and do not treat them as an acceptance value.

## Verified starting state

| Fact | Value | Source |
|---|---|---|
| `actions/checkout` latest | `v7.0.1` | GitHub releases API |
| `actions/setup-go` latest | `v7.0.0` | GitHub releases API |
| `golangci/golangci-lint-action` latest | `v9.3.0` | GitHub releases API |
| `golangci-lint` latest | `v2.13.2` | GitHub releases API |
| `golangci-lint` pinned in `ci.yml` | `v2.12.2` | repository |
| Go stable | `go1.27.1`, `go1.26.8` | go.dev/dl |
| `go` directive in `go.mod` | `1.25` | repository |
| `orhun/git-cliff-action` latest | `v4.9.0` | GitHub releases API |
| `renovatebot/github-action` latest | `v46.2.5` | GitHub releases API |
| Module versions on the proxy | none | proxy.golang.org |

The action majors already in `ci.yml` are current. The two things the repository pins **by hand**
— the linter (`v2.12.2` vs `v2.13.2`) and the toolchain (`1.25` vs `1.27.1`) — are both already
stale. That is the concrete evidence for FR-024 and for the split in FR-027: these are exactly the
families Dependabot does not watch.

---

## D1. A doc-only change must stay mergeable, and a code change must not merge unverified

**Decision**: Drop `paths-ignore` from the workflow trigger. Run the workflow on every pull request,
compute the changed paths in a `changes` job, gate every code job on its output with `if:`, and add
one aggregate job — `verify` — that runs with `if: always()` and fails when any needed job reports
`failure` or `cancelled`. Make `verify` the single required status check.

Two details the mechanism does not work without:

- **`fetch-depth: 0` on the `changes` job.** `actions/checkout` defaults to depth 1, so the base
  commit is not in the clone and `git diff` against it exits 128 with `fatal: bad object` — which
  fails `changes`, fails the aggregate, and makes *every* pull request unmergeable, including the
  documentation-only one this decision exists to unblock.
- **An event-specific base.** `ci.yml` runs on `push: main` as well as `pull_request`, and
  `github.event.pull_request.base.sha` is empty on a push. Unquoted it degrades to a bare `git diff`
  that finds nothing and marks the trunk push documentation-only — a green run that verified
  nothing. The base is `github.event.before` on a push, and `code` is `true` when neither resolves.

**Rationale**: `paths-ignore` and a required status check are mutually exclusive. When
`paths-ignore` matches, GitHub does not run the workflow at all, so a required check configured on
`static` or `test` never reports and the doc-only pull request sits blocked forever. This is the
direct collision between FR-002 ("doc-only must not trigger code gates") and FR-006 ("must not reach
trunk without a passing verification"). The aggregate-job pattern satisfies both: the code jobs are
genuinely skipped, and `verify` still reports green because a skipped job is not a failure.

It also closes the `[skip ci]` hole in FR-005. GitHub Actions honours `[skip ci]` in a commit message
and skips the whole workflow — but a skipped workflow means `verify` never reports, so branch
protection keeps the pull request unmergeable. The author can suppress the run; they cannot turn it
into a pass.

**Alternatives considered**:
- *Keep `paths-ignore`, require no check* — the honest reading of the current setup, and it fails
  FR-006 outright: nothing stops a red change from merging.
- *Require every job individually* — same blocking problem, multiplied, and every new gate becomes a
  branch-protection edit.
- *`dorny/paths-filter` for the `changes` job* — well-known, but a six-line `git diff --name-only`
  against `github.event.pull_request.base.sha` does the same work with no third-party action in the
  supply chain. Consistent with Principle IV's spirit even though actions are not Go modules.

## D2. The same gate set must run on a pull request and at a tag, without being written twice

**Decision**: Move the gate jobs into `.github/workflows/verify.yml` with `on: workflow_call`.
`ci.yml` calls it for pull requests and trunk pushes; `release.yml` calls it for the tagged commit.

**Rationale**: FR-023 requires the code at a tag to pass the same gates as any merged change. Two
copies of the gate list drift, and the copy that drifts is the one nobody reads — the release one.
One definition, two callers, is the only version of this that stays true.

**Alternatives considered**:
- *Trust that the tagged commit was already verified on trunk* — untrue for a `release/*` branch
  carrying a cherry-pick, which is precisely the path a patch release takes.
- *A composite action* — composite actions cannot express a job matrix or per-job `permissions`;
  a reusable workflow can.

## D3. Making "an empty end-to-end run is a failure" actually true

**Decision**: Enforce it twice, in-language and in the pipeline.

1. `internal/e2e` keeps a package-level registry. Every case registers itself with the tool, version
   and format it covered. `TestMain` runs `m.Run()` and then exits non-zero if the registry is empty,
   printing `no end-to-end case executed`.
2. The e2e job runs `go test -tags=integration -json` and pipes it through a small script that prints
   the executed-case inventory to the job summary (FR-011) and asserts the count is non-zero.

**Rationale**: Go's `testing` package reports a run in which every test skipped as a pass — the exact
failure mode FR-010 exists to prevent. `TestMain` is the only in-language place that sees the whole
run, so the rule holds when a contributor runs the suite locally, not only in CI. The `-json` pass
adds the human-readable inventory FR-011 asks for and catches the case where the harness itself
fails to build.

**Reconciling with the constitution**: the constitution tells integration tests to `t.Skip` with a
reason when the tool is unavailable rather than fake it. That rule is about an external *tool binary*.
Per FR-015 these cases replay recordings committed to the repository, so there is no tool to be
unavailable and nothing legitimately skips. A missing recording is a genuine failure (FR-013), not an
environment gap. No conflict — but the distinction belongs in the harness doc comment, because the
next person will read the constitution and expect a skip.

**Alternatives considered**:
- *Parse `go test -json` only* — the rule then holds in CI and not on a developer's machine, which is
  where the suite is actually run first.
- *A counter file written by each case* — same effect, more moving parts, and it survives across runs
  in a way that can lie.

## D4. What the scaffold case can honestly prove before a decoder exists

**Decision**: One real recorded Gatling run committed under `testdata/corpus/gatling/<version>/`,
together with the tool's own report for that run. The scaffold case discovers the corpus entry from
its path, reads the artefact, checks it is present and non-empty, and registers itself as covering
`gatling/<version>` **with coverage level `harness`**. It compares nothing.

**Rationale**: The maintainer chose a live gate over a dormant one, and the value of that choice is
that corpus discovery, the registry, the inventory and the zero-case rule are all exercised end to
end from day one — the machinery every later decoder plugs into. What it must not do is look like
decoder coverage. The explicit `harness` level in the inventory is what keeps it honest: FR-012
already blocks a new tool or version without a case, and a `harness`-level case is visibly not a
decoder claim.

**No manifest.** An earlier draft of this design put a `manifest.json` beside each recording carrying
declared record counts and comparison tolerances. It was dropped. With no decoder there is nothing to
compare, so every field beyond tool and version would have been a guess at what a future decoder
wants — speculative abstraction, which Principle VI rules out. Tool and version come from the path.
The requirement the manifest was carrying is recorded instead as FR-031 and FR-032, so the first
decoder inherits the obligation and picks its own format for expected values.

**What is still captured now, because it cannot be captured later (FR-031)**: the tool's own report
for the recorded run. Nothing reads it yet. An archived run cannot be re-run, and Gatling stopped
producing statistics reports in 3.13.5 — so a run recorded without its report can never be used to
check a decoder's numbers.

**Path shape**: `testdata/corpus/<tool>/<version>/<format>/`. `format` is a path segment rather than
a field because Gatling writes `simulation.log` as text through 3.12.x and as binary from 3.13.0, and
this repository plans separate codecs for the two; without the segment both recordings of one version
would collide on the same directory and `format` would have nowhere to be read from.

**The report is enforced, not requested**: an entry holding a single file is refused unless it also
carries a `NO-REPORT` file saying which tool version produced none and why. FR-031 exists because the
report cannot be captured later, and a rule enforced only in prose is enforced by whoever remembers
it — which is nobody, on the one recording that is unrepeatable.

**Cost**: recording the run needs a JVM and a Gatling distribution on the maintainer's machine once.
It is not a pipeline dependency — the artefact is committed, and FR-015 rules out ever executing
Gatling inside the automation. Procedure in [quickstart.md](./quickstart.md).

**Alternatives considered**:
- *A synthetic artefact* — proves the harness can pass, which is the one thing not worth proving.
- *Defer the gate until the first decoder* — rejected by the maintainer, and rightly: an unexercised
  gate is discovered to be miswired at the moment it is first relied on.
- *Keep the manifest* — as above: a schema with one speculative consumer and no real one.

## D5. Renovate pull requests must trigger CI, which rules out `GITHUB_TOKEN`

**Decision**: Run Renovate self-hosted through `renovatebot/github-action` on a schedule, authenticated
with a GitHub App token minted per run by `actions/create-github-app-token`.

**Rationale**: This is the finding that decides the design. A pull request opened using the built-in
`GITHUB_TOKEN` does **not** trigger `pull_request` workflows — GitHub suppresses it to prevent
recursion. Renovate authenticated with `GITHUB_TOKEN` would therefore open update proposals that no
gate ever runs on, silently breaking FR-025 in the least visible way possible: the proposals look fine
and are simply unverified. A GitHub App token does trigger workflows. A fine-grained personal access
token also would, but it belongs to a person and dies with their account.

Dependabot is unaffected — Dependabot-authored pull requests do trigger workflows, with a read-only
token. None of our gates need a secret, so they run normally on Dependabot proposals.

**Maintainer action required before the Renovate workflow can run**: install a GitHub App on the
repository with `contents: write`, `pull-requests: write`, `issues: write` and `workflows: write`,
and store its id and private key as repository secrets. Listed in [quickstart.md](./quickstart.md).

`workflows: write` is load-bearing and silent when absent. Every pin the custom managers track is a
version inside `.github/workflows/`, and GitHub refuses an App installation token that creates or
updates a file under that path without the scope. Renovate would find the updates, fail to push the
branch, and present as having found nothing — so the two stale pins that justify adding a second
updater at all would never be proposed.

**Alternatives considered**:
- *The Mend-hosted Renovate App* — no token to manage, but its schedule and configuration live
  outside the repository, and FR-007 wants the automation's configuration reviewable in-repo.
- *`GITHUB_TOKEN`* — breaks FR-025 as described.
- *A maintainer PAT* — works, but ties the automation to one person's account.

## D6. Dividing the tracked families so nothing is proposed twice

**Decision**: Ownership is enforced in configuration, not by convention.

| Family | Owner | Mechanism |
|---|---|---|
| Go module dependencies | Dependabot | `gomod` ecosystem, already configured |
| GitHub Actions used by the workflows | Dependabot | `github-actions` ecosystem, already configured |
| Vulnerability advisories | Dependabot | security updates + `govulncheck` in the gate set |
| Pinned `golangci-lint` version | Renovate | custom regex manager on a `# renovate:` comment |
| Pinned `govulncheck` / `git-cliff` versions | Renovate | same custom regex manager |
| Go toolchain (`go` directive) | Renovate | `gomod` manager, restricted to `depType: golang` |

Renovate's config disables the `github-actions` manager outright and disables `gomod` except for the
toolchain dep type, so the two never see the same dependency.

**Rationale**: The maintainer chose "split by reach". Dependabot keeps what it is good at and remains
the source of security advisories; Renovate exists solely for the version strings Dependabot cannot
see — which, per the verified starting state above, are the two that have already gone stale. Both
configs stay short enough to read.

**Alternatives considered**:
- *Renovate owns everything, Dependabot security-only* — one policy file, but it discards a working
  Dependabot configuration and puts the module's own dependencies behind a token that can expire.
- *Replace Dependabot* — loses native security-advisory integration.

**Note on Renovate config syntax**: `fileMatch` was renamed to `managerFilePatterns` in Renovate 40;
the action is at v46, so the new key is the correct one. Config goes in `renovate.json5` so the
ownership rules can carry comments explaining *why* a manager is disabled — the thing a future
maintainer will otherwise re-enable.

## D7. Release notes from a history that will not always be conventional

**Decision**: `git-cliff` with a committed `cliff.toml`, run through `orhun/git-cliff-action` pinned
by version, with commit parsers matching this repository's actual prefixes (`feat`, `fix`, `docs`,
`ci`, `chore`, `refactor`, `test`, `perf`) and a catch-all group so an unconventional commit is
listed rather than dropped.

**Rationale**: `AGENTS.md` already names git-cliff as the release-notes tool, so this wires in a
decision already made rather than reopening it. The catch-all answers the spec's edge case directly:
silently dropping a commit from the notes is worse than an untidy "Other" section, because the notes
are the only record a downstream consumer reads.

**Alternatives considered**:
- *GitHub's auto-generated notes* — a flat list of pull request titles, no grouping by change type,
  and it cannot be reproduced locally before tagging.
- *GoReleaser* — built for shipping binaries. This module ships no binaries; its release is a tag plus
  notes plus proxy availability.

## D8. Making a release atomic and re-runnable

**Decision**: Three jobs in strict order — `guard`, then the reusable `verify`, then `publish`. The
GitHub Release is created in the **last** step of `publish`, after notes are generated and after the
"does this version already exist" check.

**Rationale**: FR-020 (no partial state, re-run reproduces a first success) and FR-022 (never
republish) interact, and the ordering is what reconciles them. If the run fails before the release is
created, nothing was published and a re-run succeeds. If it fails after, the existence check refuses
the re-run — which is the correct outcome, because the version is out. Pushing the tag is the human's
act and is outside the workflow's control; everything the workflow itself creates is created once, at
the end.

Proxy availability (FR-021, SC-007) is verified rather than assumed — and verified **before** the
Release is created, not after. Resolution depends on the tag, which the human already pushed, so
nothing about the poll needs the Release to exist; putting the flakiest step in the job (a network
call to a third party whose first fetch of a new module is slow) after the irreversible one is what
would leave a red run with a Release already created, which the existence check then refuses to
retry forever.

**What the tag already published.** For a public Go module the push itself is a publication:
`proxy.golang.org` and `sum.golang.org` resolve and permanently cache from the tag before any
workflow starts. So `guard` protects the Release, the notes and the announcement — it cannot
un-publish a version. That is why the preconditions are runnable locally and the documented
procedure runs them there, instead of using a pushed tag as the test. `proxy.golang.org` fetches on demand, so this both confirms and warms it.

**Alternatives considered**:
- *Create the release first, then validate* — inverts the failure mode into the one that cannot be
  undone, given that release tags must never be deleted.
- *Assume the proxy will pick the tag up* — SC-007 is a 15-minute promise to three downstream builds;
  an unverified promise is not one.

## D9. Branch and tag validation

**Decision**: `guard` resolves which branches contain the tagged commit with
`git branch -r --contains`, and refuses unless one of them is `origin/main` or
exactly `origin/release/X.Y.0` for the tag's major.minor — matched as a name, not a glob, so
`release/1.2.1` and `release/1.2.backup` are refused like any other branch. It then runs
`scripts/check-linkage.sh --for-tag <tag>` for the milestone precondition.

**Rationale**: FR-017 and FR-018 are already written as executable rules — `check-linkage.sh`
implements the milestone half and is documented in the constitution as the tag gate. It has no
workflow calling it today, which is the gap this feature closes.

Three things this needs that are easy to leave out, each silent or misleading when missing:

- **`fetch-depth: 0`.** A tag-triggered checkout fetches the tag ref at depth 1 and creates no
  remote-tracking branches, so `git branch -r --contains` prints nothing and every release is
  refused — including a correct one.
- **`issues: read` and `pull-requests: read`, plus `GH_TOKEN` on the job.** A `permissions:` block
  sets unnamed scopes to `none`, and `gh` inside Actions does not infer a token. `contents: read`
  alone makes the script fail on its first API call.
- **A range audit.** The script's milestone query can only see work that already carries the
  milestone, so it was structurally blind to FR-018's other half. It now also walks the commit range
  since the previous release and refuses a merged pull request carrying no milestone or the wrong
  one.

**Alternatives considered**:
- *Validate the branch from `github.ref`* — on a tag push `github.ref` is the tag, not the branch;
  it cannot answer the question.
- *Reimplement the linkage rules in the workflow* — a second implementation of a rule that already
  has one, guaranteed to disagree with it eventually.

## D10. Coverage: reported now, blocking at the first decoder

**Decision**: `scripts/check-coverage.sh` computes per-package coverage from
`go test -coverprofile`, writes a table to the job summary, and takes an `--enforce` flag that is not
passed yet. Packages with no statements report `n/a`, not `0%`.

**Rationale**: The constitution's own Sync Impact Report already records this deferral and names the
milestone that ends it (v0.0.2, the first decoder). This feature builds both halves so that turning
enforcement on is a one-word diff rather than a project. Reporting `0%` for a package with no
statements would be false and would make the first real reading meaningless.

**Alternatives considered**:
- *Enforce now* — fails on an empty module, and the first response to a gate that fails for a bad
  reason is to disable it.
- *Leave it to the pull request description, as today* — that is the status quo the constitution
  itself flagged as a follow-up.

## Deferred to implementation, not unknowns

- Exact runner wall-clock against SC-002's 10-minute budget. Measurable only once the jobs exist;
  `setup-go` caches modules and the build cache by default, and the gate jobs run in parallel behind
  a fast-failing `quick` job.
- Repository settings that are not files: the branch-protection ruleset making `verify` required,
  the labels Renovate needs (Renovate does not create labels; Dependabot does), and enabling
  Dependabot security updates. Commands are in [quickstart.md](./quickstart.md); they change
  repository settings and are left for the maintainer to run.
