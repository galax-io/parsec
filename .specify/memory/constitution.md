<!--
Sync Impact Report
==================
Version change: 1.1.0 → 2.0.0
Bump rationale: MAJOR. Two rules are removed, not narrowed, and the module's scope changes with
them: Principle I no longer describes a `stats/` package, and Principle IV no longer pre-approves a
third-party module. The versioning policy calls a removed rule MAJOR. A reader could argue MINOR on
the grounds that nothing already compliant becomes non-compliant — there is no `stats/` package and
nothing imports go-tdigest — but what changes here is what the library is *for*, and that is the
most consequential kind of amendment this document can carry. MAJOR is the honest signal.

Modified principles:
- I. Canonical Model First — the `stats/` bullet is replaced. This module now computes no statistic
  at all: no count, mean, percentile, range or per-interval series. It decodes artefacts into
  `model` types and offers the primitives a consumer computes from — the position a sample was
  recorded at, the bounds of the run, the outcome predicate, a way to walk the stream. A new bullet
  states the line that replaces the old one: this module owns the **definitions** (what counts as a
  failure, what a request position is, where a run begins and ends), the consumer owns the
  arithmetic. The tool-packages-must-not-import-each-other rule is unchanged and kept.
- IV. Minimal, Explicit Dependencies — the `github.com/caio/go-tdigest` pre-approval is removed. It
  existed for percentiles in `stats/`; with the arithmetic gone so is the reason. `go.mod` naming no
  requirement is now the intended steady state rather than a property of a young module.

Modified sections: none. Added sections: none. Removed sections: none. Renamed principles: none.

Why now: statistics belong in `galaxio-cli`, decided 2026-09-05, and the arithmetic moved there in
the same change (parsec#9 into galaxio-cli#51, parsec#12 to galaxio-cli#61).

The obvious objection is that three consumers would then each grow their own statistics — the
"one statistics engine per tool" failure parsec#4 exists to prevent, moved one layer up. It does not
land, because the two implementations are different computations rather than one duplicated:
`galaxio-cli` summarises a finished run in one pass over an archived log, and the `comet` sidecar
aggregates a run while it is still being written, in windows, with its own arithmetic. A single
shared accumulator would have had to serve both and would have been shaped by neither.

What still had to be protected is the part where two computations can silently disagree about *what*
they are measuring, and that is what the new Principle I bullet pins here: what a failure is, what a
request position is, where a run begins and ends. Divergence in arithmetic is a bug someone finds;
divergence in definitions is two correct programs disagreeing.

Templates:
- ✅ .specify/templates/plan-template.md — Technical Context no longer names go-tdigest in `stats/`;
  the Constitution Check gate for I asks about primitives rather than a statistics package; the
  source tree drops the `stats/` line; the Complexity Tracking example no longer suggests a module
  in `stats/`.
- ✅ .specify/templates/spec-template.md — the percentile example FR and the percentile fidelity SC
  are replaced: a spec for this module states what it decodes and hands over, not what it computes.
- ✅ .specify/templates/tasks-template.md — path conventions drop `stats/`; the tolerance-test task
  no longer points at a `stats/` file.
- ✅ .specify/templates/checklist-template.md — no constitution references; no change.
- ✅ AGENTS.md — Structure and Architecture rewritten: no `stats/`, and the statistics sentence
  replaced by what this module hands a consumer instead.
- ✅ README.md — the packages table and "What it will be" no longer promise a statistics engine.
- ✅ doc.go — the planned `stats` package is removed from the package list.
- ✅ .github/workflows/ — the `deps` gate is unchanged and still correct; no job referenced `stats/`.
- ✅ .golangci.yml — no reference; no change.
- ✅ plan-template.md still carries one Constitution Check gate per principle plus a workflow gate.

Follow-up TODOs:
- Milestones re-scoped in the same change: v0.0.7 is now "The primitives a consumer folds" (#8);
  v0.0.8 is retired and closed, its #9 absorbed into galaxio-cli#51; v0.1.0 keeps only the stability
  contract (#13), its #12 moved to galaxio-cli#61. No arithmetic remains in this backlog.
- The coverage-floor follow-up from v1.1.0 is **closed**: CI enforces the floors, and the first
  decoder package landed in v0.0.2.
- Carried forward from v1.1.0, still unresolved: `.claude/skills/speckit-tasks/SKILL.md` says test
  tasks are OPTIONAL, which Principle III contradicts. The file is spec-kit managed and reinstalled
  on upgrade, so it is left alone and recorded here rather than patched — dropping the note would
  make the drift invisible, which is why it survives this amendment.
- `.copier-answers.yml` still records the pre-v2.0.0 Structure and Architecture text, including a
  `stats/` package and the `go-tdigest` pre-approval. The file forbids manual edits and is rewritten
  by `copier update`, so the fix is to answer differently at the next update; until then a template
  update would silently revert `AGENTS.md`.
- Ratification date is the scaffold date (2026-09-02); no earlier constitution existed.
-->
# parsec Constitution

## Core Principles

### I. Canonical Model First

- `model/` is the single source of truth for load-test results. Every tool package
  (`gatling/`, `jmeter/`, `k6/`, `locust/`, `phout/`) MUST convert that tool's artefacts
  into `model` types. A tool package MUST NOT export a result type of its own for
  consumers to depend on.
- **This module computes no statistic.** It decodes artefacts into `model` types and
  offers the primitives a consumer needs to compute from them — the position a sample was
  recorded at, the bounds of the run, the outcome predicate, and a way to walk the stream.
  It MUST NOT export a count, a mean, a percentile, a range or a per-interval series.
  Those are the consumer's, and `galaxio-cli` is where they are computed.
- What this module owns is the **definitions**, not the arithmetic: what counts as a
  failure, what a request position is, and where a run begins and ends. Those are what
  two implementations diverge on, and they are cheap to keep in one place. A number
  derived from them is not.
- Tool packages MUST NOT import each other; shared helpers live in `model/` or
  `internal/`.
- What a source cannot provide MUST be declared through `Capabilities`, never faked. A
  value the source does not carry is reported as absent; it is never filled with a zero,
  an average or a guess. Consumers decide how to render absence.
- A new field in `model` MUST answer a question a report asks, and its plan MUST state
  which sources supply it and which declare it absent.

Rationale: three consumers (galaxio-cli, the comet sidecar, the Galaxio backend) build on
this module. One model means each parser is written once and every consumer compares
tools on equal terms; `Capabilities` keeps an honest line between measured and missing.

### II. Version-Gated, Streaming Decoders

- Every read of a tool artefact MUST pass a version gate before any record is decoded.
  A version below the supported range MUST be refused with an error naming the version
  found and the range supported. An unknown newer version MUST decode and MUST surface a
  warning to the caller through the API; a warning MUST NOT be swallowed or sent only to
  a log.
- A codec's supported range MUST equal the range covered by its golden corpus.
  Supporting a new version means adding corpus files and widening the gate in the same
  change; a gate MUST NOT be widened on the assumption that a format did not change.
- Decoders MUST stream: every codec exposes an `io.Reader` entry point and its memory
  use MUST be bounded independently of artefact size. Length-prefixed reads MUST cap
  allocations so that a corrupt length cannot exhaust memory.
- Chunked reads and a whole-file read of the same artefact MUST produce identical record
  streams.
- Malformed input MUST return an error carrying the byte offset (line number for text
  formats) of the failure. A decoder MUST NOT panic on any input and MUST NOT use
  `recover` as a substitute for validation.

Rationale: Gatling's `simulation.log` is internal, undocumented and has already changed
once (text through 3.12.x, binary from 3.13.0); archives hold both. Refusing what we
cannot read and warning about what we have not verified is the only honest contract for
an external format, and streaming is what lets a multi-gigabyte log be reported on at all.

### III. Golden-Corpus Testing (NON-NEGOTIABLE)

- Every supported tool and version MUST have a golden corpus under
  `testdata/corpus/<tool>/<version>/`, recorded from a real run of that tool version and
  committed as produced. A hand-edited artefact is a fixture, not corpus, and MUST say so
  in its name.
- A recording MUST capture, at the moment the run is made, everything a later comparison
  will need: the artefact exactly as the tool produced it, AND the report that tool
  produced for that same run. None of it is recoverable afterwards — an archived run cannot
  be re-run, and a tool may stop producing a report in a later version, as Gatling did in
  3.13.5. A corpus entry MUST NOT be added without the run's own report unless that tool
  version genuinely produced none, in which case the entry MUST record that fact so a later
  reader knows the absence is the tool's and not the recorder's.
- Decoder output MUST be compared against the recorded record stream byte for byte (field
  for field where the record stream is stored decoded). Statistics MUST be compared
  against the report the tool itself produced for that run, within a tolerance documented
  next to the assertion together with the reason for it.
- Tests are table-driven on the standard `testing` package. The race detector is always
  on: `go test -race -shuffle=on ./...` is both the CI command and the local verify step.
- Coverage floors: 90% for decoder packages, 80% for the module overall. A change that
  takes a package below its floor MUST NOT merge.
- Tests land with the change they cover, and a bug fix MUST include a regression test
  that fails without the fix. Test tasks are never optional in a spec, plan or task list.
- Real artefacts, not mocks: when a tool can produce the artefact, the test uses the
  artefact. Mocking is reserved for what no real integration path can reach.

Rationale: for an undocumented format the corpus *is* the specification. Byte-for-byte
agreement plus report-level tolerances are the only evidence that a decoder is correct
rather than merely plausible, and the race detector is cheap insurance for streaming code.
That evidence has an expiry: the moment a run finishes is the only moment its own report can
be captured, so an entry recorded without one is permanently unable to prove a decoder's
numbers — a gap no amount of later work can close.

### IV. Minimal, Explicit Dependencies

- `model/` and `gatling/` MUST depend on the standard library only. The `deps` job in
  `.github/workflows/ci.yml` enforces this and MUST NOT be weakened or skipped.
- Any other package MAY use a third-party module only when the plan names it, explains
  why the standard library is insufficient and records the decision in `research.md`.
  Adding or upgrading a dependency requires asking first (see Development Workflow).
- **No third-party module is pre-approved.** `github.com/caio/go-tdigest` was, for
  percentiles in a statistics package this module no longer has; with the arithmetic gone
  so is the reason. `go.mod` naming no requirement is the intended steady state, not an
  accident of the module being young.
- `go.mod` is dependency truth: `go mod tidy` MUST leave the tree unchanged, and `main`
  carries no `replace` directive.

Rationale: this module is imported by three downstream builds; every transitive dependency
becomes theirs, along with its licence, its vulnerabilities and its upgrade schedule.

### V. Compatibility-Sensitive Public API

- Before v0.1.0 exported identifiers MAY change between releases; every such change MUST
  be recorded under Changed or Removed in `CHANGELOG.md`.
- From v0.1.0 on, changing the signature or observable behaviour of an exported
  identifier, or any serialized format the module writes, is a breaking change. It MUST
  be called out in the spec, approved before implementation, recorded in `CHANGELOG.md`,
  and released as a new MINOR version while the module is at v0.x and as a new major
  module path from v1.0.0 on.
- Deprecate before removing: a superseded identifier keeps working for at least one MINOR
  release and carries a `// Deprecated:` comment naming its replacement.
- Every exported identifier MUST have a doc comment that states what it does and, for
  decoders, which tool versions it accepts.
- `CHANGELOG.md` follows Keep a Changelog and MUST be updated in the same PR as any
  user-visible change.

Rationale: a silent change here breaks report generation in galaxio-cli, ingestion in the
Galaxio backend and the sidecar at once, usually long after the release that caused it.

### VI. Idiomatic, Simple Go

- Code MUST pass `gofmt`, `gofumpt` with extra rules, `go vet` and the `golangci-lint`
  configuration in `.golangci.yml` with zero findings. A `//nolint` directive MUST name
  the linter and carry an explanation; `nolintlint` enforces this.
- Errors are values: functions return `error`, callers wrap with `%w` and inspect with
  `errors.Is` and `errors.As`. `panic` and `recover` MUST NOT be used for control flow.
- No dead code, no duplicated code, no speculative abstraction: build what the current
  spec needs. A refactor outside the scope of the current issue goes in its own PR.
- Follow the conventions already in the codebase before adding a new one; a new
  convention is named in the plan and applied consistently in the PR that introduces it.
- The complexity limits in `.golangci.yml` (function length, cyclomatic complexity,
  nesting depth) are ceilings, not targets.

Rationale: a decoder library lives for years and is read far more often than it is
written. Predictable Go keeps review fast and lets downstream engineers debug through it.

## Quality Gates & Tooling

Toolchain: Go 1.25. The `go` directive in `go.mod` is authoritative and CI reads it
through `go-version-file`. `golangci-lint` is pinned in `.github/workflows/ci.yml`
(v2.12.2 at ratification) and upgraded only in a dedicated PR, together with any config
change the upgrade requires.

Every PR MUST be green on all CI jobs before merge:

| Gate | Command | CI job |
|------|---------|--------|
| Format | `test -z "$(gofmt -l .)"` | quick |
| Module hygiene | `go mod tidy && git diff --exit-code` | quick |
| Vet | `go vet ./...` | quick |
| Build | `go build ./...` | quick |
| Lint | `golangci-lint` (pinned) with `.golangci.yml`, over every build configuration | lint |
| Tests | `go test -race -shuffle=on ./...` | test |
| End-to-end | `go test -tags=integration` over the golden corpus; an empty run fails | e2e |
| Dependency boundary | stdlib-only check on `./model/...` and `./gatling/...` | deps |
| Vulnerabilities | `govulncheck` (pinned) over the module | vuln |
| Coverage | per-package against the floors below, enforced | coverage |
| Requirements document | the corpus probe's OpenNFR document against the published schema | nfr |

Local equivalents: `gofmt -w .` before every commit; `go vet ./... && go test ./...` to
verify; `go build ./... && go test ./...` is the definition of a green commit;
`go test -tags=integration ./...` runs the integration suite.

Additional constraints:

- Integration tests sit behind the `integration` build tag, use real tool artefacts or
  binaries, and MUST `t.Skip` with a reason when the tool is unavailable rather than
  fake it.
- Coverage floors (Principle III) are measured with `go test -cover`. Until CI enforces
  them, the PR description MUST state the resulting package coverage.
- A plan for a decoder feature MUST state a throughput and peak-memory goal in its
  Technical Context and ship a `testing.B` benchmark over the largest corpus file that
  measures it. A regression against the recorded number MUST be justified in the PR.
- Decoders handle untrusted input. `gosec` stays enabled, and the allocation caps of
  Principle II are exercised by a corrupt-length corpus file.
- CI ignores changes under `**.md`, `docs/`, `specs/` and `.specify/`; a PR that touches
  only those paths merges on review alone.

## Development Workflow & Release Process

`AGENTS.md` holds the step-by-step procedure; the rules below are the ones it MUST NOT
contradict.

- **Spec-first.** Every feature starts as `specs/NNN-<feature>/` (spec, plan, tasks),
  committed as `docs(speckit): add NNN-<feature> spec/plan/tasks` BEFORE any `feat` or
  `fix` commit, and never folded into implementation. Spec work belongs to the milestone
  that owns the spec.
- **Milestones.** Every PR MUST carry the active milestone (the lowest-numbered open
  milestone matching the current spec) before merge; no milestone, no merge. Every issue
  a PR fixes MUST be closed when the PR lands on `main`. `scripts/check-linkage.sh --pr N`
  is the merge gate and `--for-tag vX.Y.Z` the tag gate; the `linkage-guard` hook runs
  the latter on every tag push.
- **Commits.** Semantic messages (`feat(scope): … (#NNN)`); one tracked issue per commit;
  every commit green on its own; format before commit; intent, not path: squash churn
  before review.
- **Branches and PRs.** Branch from `main`. One concern per PR (feature ≠ docs). Rebase
  only: no merge commits in PR branches; stacked PRs are updated with
  `--force-with-lease`. Never force-push to `main`, never commit to `main` directly.
- **Ask first.** New dependencies or upgrades; changes to public API signatures,
  observable behaviour or serialized formats; edits to another repository; release or
  publish workflow changes.
- **Never.** Commit broken code; refactor outside the issue's scope; mock an external
  system where a real integration path exists.
- **Releases.** Trunk-based. Every minor version gets a `release/X.Y.0` branch cut from
  `main`; tags `vX.Y.Z` are pushed only on `main` or `release/*`, and the branch name
  MUST match the tag's minor version. A patch lands on `main` first and is cherry-picked
  onto the release branch. Pushing a tag triggers the release workflow (Go module proxy,
  git-cliff release notes). Never delete a release tag once deployment starts; never
  reuse a version number. Before tagging, every PR merged since the previous tag carries
  the milestone and every issue whose fix is on `main` is closed.

## Governance

- This constitution supersedes every other practice document in the repository.
  `AGENTS.md` (loaded by coding agents through `CLAUDE.md`) is the runtime guidance
  file: it MUST agree with this document, and where the two conflict this document wins
  and `AGENTS.md` is corrected in the same PR.
- **Amendment procedure.** An amendment is a PR that edits
  `.specify/memory/constitution.md`, bumps the version below, rewrites the Sync Impact
  Report comment at the top of the file, and updates `.specify/templates/*` and any
  affected guidance doc in the same PR. It is approved by a maintainer of
  `galax-io/parsec` through PR review and committed as
  `docs(speckit): amend constitution to vX.Y.Z (<summary>)`.
- **Versioning policy.** MAJOR: a principle or governance rule is removed or redefined in
  a backward-incompatible way. MINOR: a principle or section is added, or guidance is
  materially expanded. PATCH: clarification, wording or typo fixes with no semantic
  change.
- **Compliance review.** Every `plan.md` MUST complete the Constitution Check in the plan
  template before Phase 0 and again after Phase 1; each FAIL needs a Complexity Tracking
  row naming the simpler alternative that was rejected. `/speckit-analyze` treats a
  violated MUST as CRITICAL. A reviewer MUST NOT approve a PR that violates a MUST
  without such a recorded justification. At every `release/X.Y.0` cut the maintainer
  re-reads Principles I–VI against the milestone's merged PRs and files an issue for
  each gap in the next milestone.

**Version**: 2.0.0 | **Ratified**: 2026-09-02 | **Last Amended**: 2026-09-05
