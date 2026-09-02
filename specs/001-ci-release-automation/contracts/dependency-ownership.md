# Contract: Dependency Ownership

**Feature**: `001-ci-release-automation` | **Date**: 2026-09-02

Two updaters, no overlap. Ownership is enforced by configuration — each updater's config disables the
managers for families it does not own — not by convention (FR-027).

## The table

| Family | Owner | Mechanism | Labels |
|---|---|---|---|
| Go module dependencies | Dependabot | `gomod` ecosystem | `dependencies`, `go` |
| GitHub Actions in the workflows | Dependabot | `github-actions` ecosystem | `dependencies`, `ci` |
| Vulnerability advisories | Dependabot | security updates, plus `govulncheck` in the gate set | `dependencies`, `go` |
| Pinned `golangci-lint` version | Renovate | custom regex manager | `dependencies`, `tooling` |
| Pinned `govulncheck` and `git-cliff` versions | Renovate | custom regex manager | `dependencies`, `tooling` |
| Go toolchain (`go` directive) | Renovate | `gomod` manager, `depType: golang` only | `dependencies`, `tooling` |

Schedule: weekly, Monday, for both.

## Why this split

Dependabot keeps what it already does well and stays the source of security advisories. Renovate
exists only for version strings Dependabot cannot see. As of 2026-09-02 those are exactly the two
that have already gone stale in this repository:

| Pinned | Repository has | Upstream has |
|---|---|---|
| `golangci-lint` in `ci.yml` | `v2.12.2` | `v2.13.2` |
| `go` directive in `go.mod` | `1.25` | `go1.27.1` |

## Enforcing non-overlap

`renovate.json5`:

- disables the `github-actions` manager outright — Dependabot owns it;
- disables `gomod` and then re-enables it only for `depTypes: ["golang", "toolchain"]`, so Renovate
  sees the toolchain line and never a module requirement;
- declares custom regex managers for the pinned tool versions.

`.github/dependabot.yml` needed exactly one change: its `actions` group matched `patterns: ['*']`
with no `update-types`, so a major action upgrade would join the grouped proposal, against FR-026's
rule that a major is proposed on its own. The group is now restricted to `[minor, patch]`, as its
`gomod` counterpart already was.

**`git-cliff-action` is Dependabot's, not Renovate's.** It is pinned with `uses:` in `release.yml`,
which puts it squarely in the `github-actions` ecosystem that `patterns: ['*']` already matches.
Marking it for a Renovate custom manager as well would give one dependency two owners and produce
the duplicate proposal FR-027 exists to prevent — the failure this whole split is designed around.
Renovate's custom managers cover the two pins that are not actions: `golangci-lint` (an input to an
action, not the action) and `govulncheck` (a module run with `go run`).

**Advisories are not labelled `security`.** Dependabot applies an ecosystem's configured labels to
every pull request it opens for that ecosystem, security updates included, so a security update
arrives labelled `dependencies`, `go` like any other Go module proposal. There is no configuration
that labels only the security ones. They are told apart by Dependabot's own security-update
mechanism and by the `govulncheck` gate, not by a label, and no `security` label is created.

**The config is the enforcement.** A comment saying "Renovate does not handle Go modules" is a wish;
`"matchManagers": ["gomod"], "enabled": false` is a rule. `renovate.json5` rather than
`renovate.json` so the ownership rules can carry the *why* inline — otherwise the next maintainer
re-enables a manager because nothing said not to.

## Marking a pinned version for Renovate

A pinned tool version becomes trackable by preceding it with a comment naming its datasource:

```yaml
# renovate: datasource=github-releases depName=golangci/golangci-lint
version: v2.12.2
```

The custom manager matches the comment and the line below it. Note `managerFilePatterns` is the
current key — `fileMatch` was renamed in Renovate 40, and the action is at v46.

## Grouping and review cost

- Minor and patch updates: one grouped proposal per family per cycle (FR-026).
- Major updates: proposed on their own, so the risk is reviewed on its own terms.
- A scan that finds nothing produces no proposal and no notification (FR-030).
- Nothing is auto-merged. Every proposal goes through the full gate set (FR-025).

Together these are what make SC-009 — under 10 minutes a week of routine review — achievable.

## Authentication

Dependabot needs nothing: its pull requests trigger workflows, with a read-only token, and no gate
here needs a secret.

Renovate runs self-hosted through `renovatebot/github-action` authenticated as a **GitHub App**, not
with `GITHUB_TOKEN`. A pull request authored with `GITHUB_TOKEN` does not trigger `pull_request`
workflows, so Renovate proposals would arrive unverified — FR-025 broken in the least visible way
possible, since the proposals would look entirely normal. See research D5.

**The `tooling` label must exist before Renovate runs.** Renovate applies labels but does not create
them, unlike Dependabot. Creating it is a maintainer action, in [quickstart.md](../quickstart.md).
