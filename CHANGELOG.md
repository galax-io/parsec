# Changelog

All notable changes to this project are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.0.2] - 2026-09-04

First published version. v0.0.1 was completed but never tagged, so everything below ships here.

### Added

- `gatling`: the wire records a `simulation.log` carries (`Record`, `Kind`, `Header`, `Status`,
  `Event`), the `Version` type with `ParseVersion` and the version gate (`Gate`, `Verdict`), and the
  errors a read can end with (`SyntaxError` with its line number, `VersionError`, `Warning`).
  Every enum has an explicit unknown sentinel at zero. Shared by every Gatling codec; standard
  library only. (#3)
- `gatling/text`: a streaming `Reader` for the tab-separated `simulation.log` written by Gatling
  3.11.5 through 3.12.0, and `SupportedVersions` reporting that range. It reads from any
  `io.Reader` with memory independent of the log's size (one 1 MiB line buffer and a bounded table
  of the names the log repeats), walks the assertion records that precede the run header, gates on
  the version — refusing anything below 3.11.5 or not a plain release, warning above 3.12.0 — and
  yields records in file order. The first line it cannot decode ends the read with the line number;
  a partial read is never presented as a result. Decodes at about 450 MB/s on one core with no
  allocation per record. (#3)
- Golden corpus `testdata/corpus/gatling/3.11.5/` and `3.12.0/`: one real run each of the same
  simulation, with the two statistics files Gatling generated for it and the decoded record stream,
  plus the simulation and stub that recorded them. (#3)
- Verification pipeline: `verify.yml` holds the gate set once — `quick`, `lint`, `test`, `e2e`,
  `canary`, `deps`, `vuln`, `coverage` — and both `ci.yml` and `release.yml` call it, so adding a
  gate is a one-file edit. `ci.yml` publishes a single required check, `verify`, which is green for
  a documentation-only change, red when any gate fails, and never absent: a cancelled run fails it,
  because suppressing a run is not passing. (#3)
- Tag-driven release: pushing `vX.Y.Z` runs `release.yml` — a guard that refuses a tag outside
  `main` or its own `release/X.Y.0`, refuses a milestone that is not tag-ready, and refuses a
  version already released; then the full gate set; then publication, ordered so the only
  irreversible act is last. Notes come from `cliff.toml`, whose catch-all group lists a commit that
  ignored the convention rather than dropping it.
- `scripts/check-coverage.sh` builds the per-package coverage table from one profile and, with
  `--enforce`, applies the 90% decoder and 80% module floors. A package with no statements reports
  `n/a`, not `0%`. `scripts/e2e-inventory.sh` turns `go test -json` into the executed-case
  inventory and refuses a run in which nothing executed.
- `.github/ruleset-main.json` declares the branch-protection ruleset, so requiring `verify` is one
  `gh api` call rather than a paragraph of instructions.
- Repository scaffold: Go module, linter configuration, CI, licence and the spec-driven development flow.

### Fixed

- `scripts/check-linkage.sh --for-tag` resolves a release tag to its own milestone, falling back to
  the `vX.Y.0` milestone, and matches the version on a boundary so `v0.0.1` cannot select
  `v0.0.10`. It previously looked only for `vX.Y.0` and could not find a milestone for any 0.0.x
  version, refusing every release in that range for a reason unrelated to the release.
- A milestone with no issues and no pull requests is no longer reported tag-ready. It warned before,
  so an empty or mistakenly created milestone could shadow the real one and pass the tag gate.
- The tag gate now audits pull requests merged since the previous release, not only those already
  carrying the milestone. A merged pull request with no milestone is invisible to a milestone query,
  which is precisely the case the rule exists to catch.
