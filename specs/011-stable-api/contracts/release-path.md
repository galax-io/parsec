# Contract 5 — the release path

**Applies to**: `.github/workflows/release.yml`, `verify.yml`, `gatling-canary.yml`,
`record-corpus.yml`, `fuzz-nightly.yml`, `.github/dependabot.yml`, and the repository's
`SECURITY.md`.
**Status**: no workflow changes what it does. Issues
[#93](https://github.com/galax-io/parsec/issues/93) and
[#101](https://github.com/galax-io/parsec/issues/101).

## Every action is pinned to a commit

A git tag is movable. `release.yml` grants the `publish` job `contents: write` and hands
`github.token` to `gh release create`, so a moved `v4` on a third-party action runs whatever it now
points at with a token that can write to the repository and cut releases. That needs no access here
at all.

The rule:

```yaml
- uses: orhun/git-cliff-action@3d96a18cc4ec17e9dc69ddcc424ccafaf1f78ce2 # v4.9.0
```

A 40-character commit SHA, with the version in a trailing comment. **MUST** for every third-party
action; **SHOULD**, and done in the same pass, for `actions/*` — a half-pinned file invites the next
contributor to copy the unpinned line. The repository already pins its non-action tooling by exact
version (`golangci-lint v2.12.2`, `govulncheck@v1.7.0`, `jsonschema==4.26.0`, `OPENNFR_REF: v0.8.0`),
so the convention exists and the actions were the gap.

SHAs resolved 2026-09-12 are recorded in research
[R17](../research.md#r17--the-workflow-pins-and-the-two-unguarded-inputs). They are a starting point,
not the pins: resolve them again at implementation time.

`.github/dependabot.yml` needs no change. Its `github-actions` ecosystem updates a SHA pin and keeps
the trailing version comment in step, which is what #93 asks for.

**The rule is enforced, not reviewed.** `scripts/check-pins.sh` fails when any `uses:` in
`.github/workflows/` names anything but a 40-character SHA with a trailing `# vX.Y.Z` comment
(a `docker://` image pins by `@sha256:` digest instead), ignoring local `./…` calls; it refuses any
`${{ … }}` substitution in a `run:` script, whatever context it names, and in the `script:` input of
`actions/github-script`, which evaluates it as JavaScript; and it refuses a file it cannot parse
rather than passing it. It ships with `scripts/check-pins_test.sh` beside it, in the shape of the
four `check-*.sh` pairs already in `scripts/`, and runs in `verify.yml`'s `quick` job alongside them.
Four actions reached movable tags under review alone; this is what stops the fifth.

**Why it parses instead of matching.** The first gate matched lines, and a line matcher cannot tell a
`uses:` key from the same text inside a scalar. Both halves failed. It refused a workflow because a
step was *named* `"assert that nothing uses: a floating tag"`, reporting the English phrase as an
unpinned action — and a gate that refuses correct work gets switched off. And it passed `? uses` /
`: actions/evil@main`, the explicit-key spelling of an ordinary mapping: `actionlint` accepts that
file with no schema error, and PyYAML's event stream for it is byte-identical to the ordinary
spelling, so GitHub's own event-driven reader cannot distinguish them either. Lone `\r` endings, a
`\uXXXX`-escaped key and a flow-style `steps:` failed the same way, the last of them silently turning
the substitution rule off for a whole job while the pin rule passed the unpinned action inside it.

Widening the patterns is not the fix: *refuse the text inside a scalar* and *read the key in a flow
mapping* are contradictory demands on a regular expression and the same demand on a parser. So the
gate parses, with PyYAML pinned in `verify.yml` the way the `nfr` job pins its validator. The scope
it does not cover is stated in its `--help`: a composite action's own `action.yml` elsewhere in the
tree is not read, and `uses: ./…` is exempt.

## No dispatch input reaches a shell unchecked

GitHub substitutes `${{ … }}` textually before bash parses the line, so a value carrying a quote
closes the quoting and runs arbitrary commands on the runner. Two workflows do this today:

| Where | Value |
|---|---|
| `gatling-canary.yml:85` | `-Dgatling.version="${{ matrix.gatling }}"` |
| `fuzz-nightly.yml:70-71` | `${{ matrix.case.target }}`, `${{ inputs.fuzztime \|\| '10m' }}`, `${{ matrix.case.package }}` |

The guard is the one `record-corpus.yml:74-87` already carries, and no second pattern is introduced:
route the value through `env:`, then validate it in a `set -euo pipefail` step that exits 1 with a
message naming the value.

```yaml
- name: the version must look like a version
  env:
    VERSION: ${{ inputs.version }}
  run: |
    set -euo pipefail
    if ! printf '%s' "$VERSION" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
      echo "refusing '$VERSION': a Gatling version is three dot-separated numbers"
      exit 1
    fi
```

Three shapes are needed: a Gatling version (the existing expression), a fuzz target and package (a Go
identifier and an import path), and a duration (`^[0-9]+[smh]$`).

Dispatch requires repository write access, so this half is a privilege escalation from "can dispatch"
to "can run code on the runner", not an anonymous hole. The pinning half is not capped that way.

## A private channel for what this module is for

`SECURITY.md` at the repository root, and GitHub's private vulnerability reporting enabled. The
module decodes files it does not trust, in a process it does not own: four fuzz targets
(`FuzzDetect`, `FuzzDecode`, `FuzzReader`, `FuzzLastRun`), a nightly fuzz workflow and an
allocation-cap regime designed against corrupt length prefixes. Someone who fuzzes a hostile
`simulation.log` into an OOM or a panic has only the public tracker today.

It states which versions receive fixes, in the terms of the release policy in `AGENTS.md`: fixes land
on `main` and are cherry-picked onto the current `release/X.Y.0` branch, so the supported set is the
latest `X.Y` line — `0.1` at this tag.

## Acceptance

| Given | Then |
|---|---|
| `grep -rn 'uses: ' .github/workflows/` | every third-party entry names a 40-character SHA with a version comment |
| a dispatch input containing a quote | the workflow exits 1 before any shell runs it |
| the workflows after the change | they do what they did before |
| `gh api repos/galax-io/parsec/community/profile` | reports `security` |
| a reporter with a crasher | reaches the maintainer without opening a public issue |
