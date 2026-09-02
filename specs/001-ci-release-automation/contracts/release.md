# Contract: Release

**Feature**: `001-ci-release-automation` | **Date**: 2026-09-02

Pushing a version tag is the only thing that publishes a release. Nothing else does — not a merge,
not a schedule, not a manual trigger (FR-016).

## Trigger

```yaml
on:
  push:
    tags: ['v*.*.*']
```

## What the tag itself already published

The guard runs **after** the tag exists, and for a public Go module that ordering has a limit worth
stating plainly: pushing `vX.Y.Z` is itself a publication. `proxy.golang.org` and `sum.golang.org`
resolve `github.com/galax-io/parsec@vX.Y.Z` from the tag alone, on first request, and record the
tree hash permanently. No workflow runs before that can happen.

So "the guard refused, nothing was published" is true of the GitHub Release and false of the module
version. Two consequences the rest of this contract depends on:

- **A tag is never pushed speculatively.** Preconditions are checked locally first
  (`scripts/check-linkage.sh --for-tag`, `git cliff --unreleased`); the push is the last step, not
  the test. Refusal cases are exercised on a scratch repository, never by pushing a bad tag here.
- **A pushed tag is never deleted and re-pointed.** If anything resolved the module in between, the
  checksum database has already recorded the old tree and every later `go get` of that version fails
  with a checksum mismatch that cannot be repaired — only outlived by burning the version number.

The guard still earns its place: it stops the *Release*, the notes and the announcement, and it
catches a milestone or branch mistake before anyone acts on the version. It simply cannot un-publish
a tag, and this contract does not claim it can.

## Job order — every arrow can refuse

```text
tag pushed ──→ guard ──→ verify (reusable) ──→ publish
```

### `guard` — preconditions, publishes nothing

| Check | Rule | Requirement |
|---|---|---|
| Branch | the tagged commit is contained in `origin/main`, or in exactly `origin/release/X.Y.0` where `X.Y` is the tag's major.minor | FR-017 |
| Version match | `release/1.2.0` accepts `v1.2.0`, `v1.2.1`, … and refuses `v1.3.0` | FR-017 |
| Milestone | `scripts/check-linkage.sh --for-tag <tag>` exits 0 | FR-018 |
| Not already released | no GitHub Release exists for this tag | FR-022 |

The branch name is matched exactly, not as a glob. `origin/release/X.Y.*` would also accept
`release/1.2.1` and `release/1.2.backup`, which the repository's own rule — one `release/X.Y.0`
branch per minor — forbids; a guard looser than the rule it enforces is not a guard.

The branch is resolved with `git branch -r --contains <sha>`. On a tag push `github.ref` is the tag,
not the branch, so it cannot answer this question — see research D9.

Two things this job must set, both of which are silent failures if forgotten:

- **`fetch-depth: 0`.** A tag-triggered `actions/checkout` fetches the tag ref at depth 1 and
  creates no remote-tracking branches at all, so `git branch -r --contains` prints nothing and
  every release — including a correct one — is refused.
- **`issues: read`, `pull-requests: read` and `GH_TOKEN`.** A `permissions:` block sets unnamed
  scopes to `none`, and `check-linkage.sh` calls `gh pr view`, `gh issue view` and the issues API.

`check-linkage.sh` is the milestone rule's only implementation, already documented in the
constitution as the tag gate and until now called by nothing. It audits both halves of FR-018: the
milestone's own issues and pull requests, and — separately, over the commit range since the previous
release — pull requests that carry no milestone at all, which a milestone query cannot see.

### `verify` — the same gates as any merged change

Calls `verify.yml`. Not a lighter set, not a cached verdict from trunk: a `release/*` branch carrying
a cherry-picked patch has a commit tree that was never verified as such (FR-023).

### `publish` — needs `guard` and `verify`

Ordered so that the only irreversible act is last:

1. generate notes with `git-cliff` over the range since the previous tag;
2. poll `https://proxy.golang.org/github.com/galax-io/parsec/@v/<tag>.info` until it resolves,
   failing the run if it does not;
3. re-check that no Release exists for this tag;
4. **create the GitHub Release** ← the only published state this workflow creates, and the last
   thing the workflow does.

The poll comes **before** the Release, not after. Proxy resolution depends on the tag, which the
human already pushed, so nothing about it needs the Release to exist — and putting the flakiest step
(a network call to a third party, whose first fetch of a new module is slow) after the irreversible
one produces the single state FR-020 forbids: a red run with a Release already created, which the
existence check then refuses to retry forever.

The poll both confirms and warms the proxy: `proxy.golang.org` fetches on demand, so it is what
makes SC-007's 15-minute promise a verified fact rather than an assumption.

## Failure and re-run

| Failure point | Published state | Re-run after fixing |
|---|---|---|
| `guard` | none | behaves as a first run |
| `verify` | none | behaves as a first run |
| `publish` before step 4 | none | behaves as a first run |
| `publish` at step 4 | the Release exists | refused by the existence check |

This is FR-020 and FR-022 reconciled by ordering rather than by a flag. Pushing the tag is the
human's act and outside the workflow's control; everything the workflow creates, it creates once.

## Release notes

`git-cliff` with a committed `cliff.toml`. Commit parsers match this repository's actual prefixes —
`feat`, `fix`, `docs`, `ci`, `chore`, `refactor`, `test`, `perf` — and a catch-all group collects
anything that does not match.

**The catch-all is deliberate.** Dropping an unconventional commit from the notes is worse than an
untidy "Other" section: the notes are the only record a downstream consumer reads.

Notes are reproducible locally before tagging:

```bash
git cliff --unreleased --tag v0.0.1
```

## Rules this contract inherits

From the constitution and `AGENTS.md`, enforced by `guard`:

- Every minor version gets its own `release/X.Y.0` branch cut from `main`.
- Tags live only on `main` or a `release/*` branch, and the branch name matches the tag's minor.
- A patch lands on `main` first, then is cherry-picked onto the release branch.
- Never delete a release tag once publication has started. Never reuse a version number.
