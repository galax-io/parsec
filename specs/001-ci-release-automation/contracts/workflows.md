# Contract: Verification Workflows

**Feature**: `001-ci-release-automation` | **Date**: 2026-09-02

What the automation promises to contributors and to branch protection. Job names in this document
are the contract — renaming `verify` silently unprotects the trunk.

## Files

| File | Trigger | Purpose |
|---|---|---|
| `.github/workflows/verify.yml` | `on: workflow_call` | the whole gate set, defined once |
| `.github/workflows/ci.yml` | `pull_request`, `push` to `main` | path filter, calls `verify.yml`, publishes the aggregate check |
| `.github/workflows/release.yml` | `push` tags `v*.*.*` | guard → calls `verify.yml` → publish |
| `.github/workflows/renovate.yml` | `schedule`, `workflow_dispatch` | the second updater |

`verify.yml` has exactly one definition of the gate set. Adding a gate means adding a job there and
adding it to the aggregate's `needs` — nowhere else.

## `ci.yml` job graph

`ci.yml` has exactly three jobs. The gates are not among them: they live in `verify.yml`
and reach `ci.yml` as one `uses:` job, because `needs:` accepts only job ids declared in
the same file and a reusable workflow's jobs are not addressable from its caller.

```text
changes ──→ gates (uses: ./.github/workflows/verify.yml)
   │            │
   │            └──┐
   └───────────────┴──→ verify   (if: always())
```

Inside `gates`, `verify.yml` runs its own graph: `quick` first, then `lint`, `test`,
`e2e`, `deps`, `vuln` and `coverage` in parallel behind it.

- `changes` checks out with `fetch-depth: 0` — the base commit is not in a default
  depth-1 clone, and `git diff` against a missing object exits 128 — then computes the
  changed paths and outputs a `code` boolean.
- The diff base is event-specific: `github.event.pull_request.base.sha` on a pull
  request, `github.event.before` on a push. `ci.yml` runs on both, and the pull-request
  field is empty on a push, which would otherwise classify every trunk push as
  documentation-only and report green having verified nothing. When the base cannot be
  resolved at all — a first push, a force-push — `code` is `true`: guessing wrong that
  way costs minutes, guessing wrong the other way lets an unverified change through.
- `gates` carries `if: needs.changes.outputs.code == 'true'`. This is the only `if:` on
  the gate set; the jobs inside `verify.yml` cannot see `needs.changes` and must not try.
- `quick` is the fast-fail gate inside `verify.yml` — formatting, `go mod tidy` clean,
  `go vet`, `go build` — and must report within 3 minutes (SC-002).
- `verify` runs with `if: always()`, needs `[changes, gates]`, and fails when
  `contains(needs.*.result, 'failure')` or `contains(needs.*.result, 'cancelled')`. A
  `uses:` job already fails when any job inside it fails, so one entry carries the whole
  gate set and adding a gate is a one-file edit.

## The single required check

`verify` is the only status check branch protection requires.

| Situation | `gates` | `verify` | Mergeable |
|---|---|---|---|
| Code change, all gates pass | pass | pass | yes |
| Code change, one gate fails | one `failure` | fail | no |
| Documentation-only change | `skipped` | pass | yes |
| Author pushed `[skip ci]` | not run | never reports | no |
| Run cancelled | `cancelled` | fail | no |

The last two rows are FR-005: an author can suppress a run, but suppressing is not passing.

**Why not `paths-ignore`**: when `paths-ignore` matches, GitHub does not run the workflow, so a
required check never reports and a documentation-only pull request is blocked forever. The
`changes` + aggregate pattern is what lets FR-002 and FR-006 both hold. See research D1.

## Gates

| Job | Command | Blocking |
|---|---|---|
| `quick` | `test -z "$(gofmt -l .)"`, `go mod tidy && git diff --exit-code`, `go vet ./...`, `go build ./...` | yes |
| `lint` | `golangci/golangci-lint-action` at the pinned version, over both build configurations | yes |
| `test` | `go test -race -shuffle=on ./...` | yes |
| `e2e` | `go test -tags=integration -race -shuffle=on -json ./internal/e2e/...` ▸ `scripts/e2e-inventory.sh`, under `set -o pipefail` | yes |
| `deps` | stdlib-only boundary for `./model/...` and `./gatling/...`, excluding the module's own packages | yes |
| `vuln` | `go run golang.org/x/vuln/cmd/govulncheck@<pinned> ./...` | yes |
| `coverage` | `go test -coverprofile` ▸ `scripts/check-coverage.sh` | **report only** until the first decoder |
| `nfr` | the corpus probe's OpenNFR document against the published schema, validator and schema ref both pinned | yes |

`nfr` arrives with [spec 003](../../003-canonical-model/plan.md), not with this feature; it is listed
here because this table is where a reviewer looks to learn which gates exist, and a table that omits
a required check is worse than one that names where it came from.

`coverage` writes its table to `$GITHUB_STEP_SUMMARY` and exits 0. Turning it blocking is passing
`--enforce`; the floors (90% decoder packages, 80% overall) are already in the script. See research
D10 and the constitution's own recorded follow-up.

Four of these rows are exact for a reason, because the obvious shorter form of each is wrong:

- **`gofmt -l` exits 0 even when it prints files.** The bare form reports success on precisely the
  input the gate exists to reject, so the gate must assert on empty output, not on exit status.
- **`golangci-lint` only loads files whose build constraints are satisfied.** With no
  `--build-tags`, every file in `internal/e2e` is invisible to all enabled linters and the job
  reports an empty package — indistinguishable from clean code. The job runs a matrix over the
  default and `integration` configurations.
- **A pipeline's exit status is its last command's.** GitHub Actions' default shell is `bash -e`
  without `pipefail`, so `go test | e2e-inventory.sh` would discard a test failure whenever the
  inventory itself is satisfied. The step sets `shell: bash` and `set -o pipefail`.
- **`go list -deps` prints the queried package itself.** Matching any path with a dot in the first
  segment therefore flags `github.com/galax-io/parsec/model` — the module's own package — so the
  check must exclude the module path before deciding. Today, with `model/` and `gatling/` absent,
  the job is a deliberate no-op and says so in its output rather than passing silently.

## End-to-end inventory

The `e2e` job writes to the job summary, so FR-011 is answerable without opening a log:

```text
end-to-end cases executed: 1
  gatling  3.12.6  text  level=harness
```

`level=harness` means the case exercised discovery and artefact readability and compared nothing.
`level=decoder` means the record stream and the derived statistics were compared against the
recording and the tool's own report — the work FR-032 defers to the first decoder. FR-012 counts
`decoder` cases only.

The count is of cases that **executed**, not cases that passed — a failing case still registers, so
a run with one failure reports `1` and the failure is attributed to that case rather than being
reported as an empty run. That distinction is what keeps FR-013 answerable: "no end-to-end case
executed" means the corpus is missing, and it never doubles as the message for a case that ran and
failed.

An empty inventory fails the job, and `TestMain` has already failed the run before the script sees
it. The script's own non-zero exit is not the load-bearing mechanism — `pipefail` plus `go test`'s
status is — it is the second reading that also catches a suite which failed to build and so never
gave `TestMain` a chance to have an opinion.

## Permissions

`GITHUB_TOKEN` is read-only at the workflow level; write is granted per job, never globally.

| Workflow | Permissions |
|---|---|
| `verify.yml` | `contents: read` |
| `ci.yml` | `contents: read` |
| `release.yml` | `contents: read`; **`issues: read` + `pull-requests: read` on `guard`**; `contents: write` on `publish` only |
| `renovate.yml` | `contents: read`; Renovate itself uses a GitHub App token, not `GITHUB_TOKEN` |

A `permissions:` block sets every scope it does not name to `none`, so `contents: read` alone
revokes the issue and pull-request access `scripts/check-linkage.sh` needs; the guard would then
refuse every release with "Resource not accessible by integration". The `guard` job also sets
`GH_TOKEN: ${{ github.token }}`, which `gh` requires inside Actions and does not infer.

**Fork pull requests** get every gate. None needs a secret, and the read-only token they receive is
enough. A fork contributor gets the same verdict as anyone else.

## Concurrency

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: ${{ github.event_name == 'pull_request' }}
```

In-progress pull-request runs are superseded; trunk and tag runs are never cancelled. A cancelled run
fails `verify`, so a superseded run cannot be mistaken for a passing one.
