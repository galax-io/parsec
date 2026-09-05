# Changelog

All notable changes to this project are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Telling which Gatling wrote a log, before anything tries to read it.

### Added

- `gatling.Detect`, with `gatling.Format` and `gatling.DetectSize`: names a `simulation.log` as the
  tab-separated text format or the binary one written from 3.13.0, from a fixed ten leading bytes
  and never from the file's name. A text log is recognised by `RUN` or `ASSERTION` followed by a
  tab — a simulation that declares assertions writes those ahead of the run header, so both are
  ordinary openings, and both recorded runs in the corpus begin with the second.
- `gatling/simlog`: opens a `simulation.log` without being told which Gatling wrote it. It
  identifies the format, hands the stream to the codec that reads it, and yields records identical
  to that codec's own. A binary log is refused with a `*gatling.UnsupportedFormatError` naming the
  format, rather than failing as a syntax error on line 1.
- `gatling/simlog.Supported`: what this module reads, per format, so a consumer reports it instead
  of hard-coding a range that goes stale. The binary format is reported as known and not yet
  readable, which is a different answer from an unknown one.
- `gatling.Policy` and `Policy.Apply`: the version policy in one place, applied by every codec
  before any record is decoded. It returns the `Verdict` alongside the warning, so a codec reads
  leniency from the version decision rather than inferring it. `gatling.Gate` is unchanged and is
  what `Apply` is built on.
- `gatling.WithStrict`, with `gatling.Option`: refuses a version no recording covers instead of
  decoding it with a warning. The default is unchanged and lenient; strictness only ever tightens
  the gate, and cannot reach a version inside or below the supported range.
- `gatling.FormatError`, `gatling.UnsupportedFormatError` and `gatling.UnverifiedError`. With the
  existing `VersionError` and `SyntaxError`, every way a read can be refused now has its own type,
  so a caller branches with `errors.As` rather than on message text.

### Changed

- `gatling/text.NewReader` and `gatling/text.NewRunReader` take `opts ...gatling.Option`. Every
  existing *call* compiles and behaves exactly as before — passing no options is the lenient
  default — and the version decision moved out of the codec into `gatling.Policy` without changing
  any outcome. What does break is a caller that stored either constructor in a variable of type
  `func(io.Reader) (*text.Reader, error)`: a variadic parameter changes the function's type, so a
  registry built that way stops compiling. Permitted before v0.1.0 (Principle V) and recorded here
  rather than described as fully compatible.
- `gatling.Warning.String` returns the empty string for the zero `Warning`. The zero value is how
  "no warning" travels, so rendering it as a warning about version 0.0.0 put a false alarm in the
  log of every healthy run.

## [0.0.3] - 2026-09-05

The canonical model, and the corpus probe's expectations stated once in OpenNFR.

### Added

- `model` package: the canonical result types every source is decoded into — `Sample`,
  `GroupSample`, `UserEvent`, `RunError`, `Run`, `Item` and `Capabilities`, with `Opt[T]` for a value
  a source may not have recorded. Standard library only, and what the three downstream builds depend
  on instead of any tool package.
- `gatling/text.NewRunReader`: reads a Gatling text `simulation.log` as canonical results. Same log,
  same version gate and same bounded memory as `NewReader`; a run carries only what does not grow
  with its length, and samples, group traversals, user events and run-level errors stream beside it.
- `gatling/text.Capabilities`: what a Gatling text log records and what it never does, readable
  before a log is opened.
- `model.ItemAssertion`: an opaque payload a source wrote among the events rather than ahead of
  them. A source that writes them all in its preamble puts them on `Run.Assertions` instead; this
  kind exists so neither placement loses them.
- `model.FieldsKnown`: every field the package names, so a caller walks the set instead of
  hardcoding its last constant.

The Gatling wire records (`gatling.Record`, `gatling.Header` and the rest) are unchanged and stay
exported. They are the log's own events rather than a result; the canonical types are what a
consumer builds on.

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
