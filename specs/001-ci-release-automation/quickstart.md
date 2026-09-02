# Quickstart: Validating This Feature

**Feature**: `001-ci-release-automation` | **Date**: 2026-09-02 | **Plan**: [plan.md](./plan.md)

How to prove each user story works, and the three things a maintainer must do by hand because they
are repository settings or need a JVM.

---

## Prerequisites

| Need | For |
|---|---|
| Go 1.25+ | every local gate |
| `gh` and `jq` | `scripts/check-linkage.sh` |
| `git-cliff` | previewing release notes locally |
| A JVM and a Gatling distribution | recording the corpus run, **once** |
| Admin on `galax-io/parsec` | branch protection, labels, the GitHub App |

The JVM is needed once, to record. It is never needed by the pipeline — FR-015 rules out executing a
load-testing tool inside the automation.

---

## US1 — Every change gets one complete verdict

Run the gate set locally; these are the same commands `verify.yml` runs.

```bash
test -z "$(gofmt -l .)" && go mod tidy && git diff --exit-code && go vet ./... && go build ./... && go test -race -shuffle=on ./...
```

**Expected**: silence and exit 0.

The `test -z` wrapper is not decoration: `gofmt -l` **exits 0 even when it lists unformatted files**, so the bare form in an `&&` chain reports success on exactly the input this check exists to reject. CI uses the same wrapper.

Then prove the gates actually block. Break exactly one thing and confirm the pull request is refused
and the report names it:

| Break | Gate that must catch it |
|---|---|
| Add a stray space to a `.go` file | `quick` — formatting |
| Add a third-party import to `model/` or `gatling/` | `deps` |
| Add a failing test | `test` |
| Add `//nolint` with no explanation | `lint` — `nolintlint` |

**Expected in every case**: `verify` fails, and the failing gate names the file.

Then prove a documentation-only change still merges. Open a pull request touching only `README.md`:

**Expected**: gate jobs `skipped`, `verify` green, mergeable on review alone. This is the pairing
that `paths-ignore` cannot express — see research D1.

---

## US2 — The end-to-end suite always runs and cannot pass by skipping

### Record the corpus run — maintainer action, once

Needs a JVM and Gatling. Pick a version in the 3.11.5 – 3.12.x range so the run still produces its
own statistics; Gatling stopped writing them in 3.13.5.

1. Run any small simulation against a local target — a few hundred requests is plenty. The point is a
   real artefact, not a large one.
2. Copy the run directory's `simulation.log` **and its statistics report** into
   `testdata/corpus/gatling/<version>/text/`, byte for byte, unedited. `format` is a path segment
   so that a binary log of the same version can be recorded beside it later.

**The report is the part you cannot get later** (FR-031). Nothing reads it yet — there is no decoder
to check its numbers against. But an archived run cannot be re-run, and Gatling stopped producing
statistics reports in 3.13.5, so a run recorded without its report is permanently unusable for
checking a decoder. Record it now.

There is no metadata file to write. Tool, version and format all come from the directory path; the
format for expected values is the first decoder's decision (research D4).

The harness refuses an entry holding a single file, so forgetting the report is caught rather than
merged. If the tool version genuinely produced no report — Gatling stopped writing them in 3.13.5 —
add a `NO-REPORT` file saying which version and why. Both cases then look different, which is the
whole point of the rule.

**Do not hand-edit the artefact.** A hand-edited artefact is a fixture, not corpus, and the harness
rejects it (Principle III).

### Run the suite

```bash
go test -tags=integration -race -shuffle=on ./internal/e2e/...
```

**Expected**: pass, with an inventory naming the executed case and its level:

```text
end-to-end cases executed: 1
  gatling  3.12.6  text  level=harness
```

`level=harness` is honest and deliberate: until the first decoder exists the case proves the harness,
not a decoder. FR-012 counts `decoder` cases only.

### Prove it cannot pass empty

Move the corpus aside, run the suite, put it back:

```bash
mv testdata/corpus/gatling /tmp/corpus-away && go test -tags=integration ./internal/e2e/... ; mv /tmp/corpus-away testdata/corpus/gatling
```

**Expected**: `no end-to-end case executed`, exit non-zero. **Not** `ok ... [no tests to run]`.

This is the single most important check in this document. Go reports a run in which every test
skipped as a pass, which is exactly the failure mode FR-010 exists to prevent.

Then corrupt rather than remove: truncate the recorded artefact to zero bytes and re-run.

**Expected**: a failure naming the unreadable artefact — never a skip (FR-013). Restore it
afterwards with `git checkout`.

---

## US3 — Releasing is pushing a tag

### Preview before you tag

```bash
scripts/check-linkage.sh --for-tag v0.0.1
```

**Expected**: `PASS` and `Milestone #N is tag-ready`. If it fails it names the issue that is still
open or the pull request that carries no milestone — fix that, do not tag.

```bash
git cliff --unreleased --tag v0.0.1
```

**Expected**: the notes the release will publish, grouped by type. Anything that lands in the
catch-all group is a commit that did not follow the convention — worth fixing before tagging, but it
will be listed either way rather than dropped.

### Cut the release

**Run the preview above first.** For a public Go module the tag push is itself the publication:
`proxy.golang.org` and `sum.golang.org` resolve the version from the tag alone, before any workflow
starts. A tag is never pushed to see whether it will be accepted.

Do not rely on a hook to catch a bad push. `linkage-guard.sh` is a Claude Code PreToolUse hook
registered in `.claude/settings.json`; it intercepts an agent's tool calls. `core.hooksPath` is
unset and no git hook is installed, so a push typed in your own terminal is not checked by anything.

During 0.0.x the tag goes on `main` — `AGENTS.md` permits tags on the trunk, and its
`release/X.Y.0` rule would put all ten 0.0.x releases on one `release/0.0.0` branch, which is not
what those milestones mean. Release branches begin at v0.1.0, where the API becomes stable and there
is something to stabilise. Tag `v0.0.1` on `main`, push the tag.

**Expected**: `guard` → `verify` → `publish`, a GitHub Release with generated notes, and the proxy
poll resolving. Then confirm the promise from a clean directory:

```bash
GOPROXY=https://proxy.golang.org go list -m github.com/galax-io/parsec@v0.0.1
```

**Expected**: resolves within 15 minutes of the tag push (SC-007).

### Prove the refusals

Exercise these on a scratch repository, never here. A version tag pushed to this repository is
published to the module proxy the moment anything resolves it, and deleting it afterwards is what
produces a permanent checksum mismatch for that version.

| Tag pushed | Expected |
|---|---|
| a version tag on a feature branch | refused, naming the branch rule |
| a minor that does not match its release branch | refused, naming tag and branch |
| a tag whose milestone has an open issue | refused, naming the issue |
| a tag whose range contains a merged PR with no milestone | refused, naming that PR |
| the same tag twice | refused as already released |

**Expected in every case**: no Release created, and the refusal names the rule.

---

## US4 — Dependency updates arrive as batched proposals

```bash
gh pr list --repo galax-io/parsec --label dependencies
```

**Expected**: grouped proposals appearing weekly, each labelled by family, each with a `verify` run
against it. Confirm no update ever appears twice — that is the whole point of the ownership split in
[contracts/dependency-ownership.md](./contracts/dependency-ownership.md).

The first two proposals are predictable, because both pins are already stale: `golangci-lint`
`v2.12.2` → `v2.13.2`, and the `go` directive `1.25` → `1.27.x`.

To test without waiting a week, trigger Renovate by hand:

```bash
gh workflow run renovate.yml --repo galax-io/parsec
```

---

## Maintainer actions — repository settings, not files

These three are what turn the reports into enforcement. They change repository configuration, so they
are listed rather than run.

### 1. Require the `verify` check on `main`

Without this, the pipeline reports honestly and nothing acts on the report — SC-010 is not met.

```bash
gh api -X POST repos/galax-io/parsec/rulesets --input .github/ruleset-main.json
```

The ruleset must require the `verify` status check and disable bypass. `verify` is the only check to
require: it aggregates the rest, so new gates need no branch-protection edit.

### 2. Create the `tooling` label

Renovate applies labels but does not create them; Dependabot does. A missing label means silently
unlabelled proposals. Only `tooling` is needed — advisories arrive labelled `dependencies`, `go`
like any other Go module proposal, because Dependabot has no way to label only the security ones.

```bash
gh label create tooling --repo galax-io/parsec --color 0366d6 --description "CI tooling and toolchain versions"
```

### 3. Install the GitHub App for Renovate

Renovate must not author pull requests with `GITHUB_TOKEN` — those do not trigger `pull_request`
workflows, so its proposals would arrive unverified while looking entirely normal (research D5).

Install an app on the repository with `contents: write`, `pull-requests: write`, `issues: write` and
`workflows: write`, then store `RENOVATE_APP_ID` and `RENOVATE_APP_PRIVATE_KEY` as repository
secrets.

`workflows: write` is the one that is easy to miss and silent when missing: every pin Renovate
tracks lives in `.github/workflows/`, and GitHub refuses an App token that writes there without it.
Renovate would then find the updates, fail to push the branch, and look like it had found nothing. Also enable
Dependabot security updates in the repository's security settings — that is the half of FR-029 that
`govulncheck` does not cover.

---

## Definition of done

- [ ] Every gate in the constitution's Quality Gates table runs in `verify.yml`
- [ ] A documentation-only pull request is mergeable; a red code pull request is not
- [ ] `go test -tags=integration ./internal/e2e/...` fails when the corpus is absent
- [ ] One real Gatling run is committed under `<tool>/<version>/<format>/`, with the tool's own report beside it (FR-031)
- [ ] `scripts/check-linkage.sh --for-tag` runs in `release.yml` and can refuse a tag
- [ ] A version tag on a disallowed branch publishes nothing
- [ ] The module resolves from `proxy.golang.org` after a release
- [ ] Both updaters run weekly with no family owned by both
- [ ] `CHANGELOG.md` records this under Unreleased ▸ Added
- [ ] `verify` is a required check with bypass disabled
