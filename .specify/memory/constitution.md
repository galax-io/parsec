<!--
Sync Impact Report
==================
Version change: unversioned spec-kit template → 1.0.0
Bump rationale: initial ratification. Every placeholder in the template has been replaced
with rules derived from AGENTS.md, README.md, doc.go, .golangci.yml and
.github/workflows/ci.yml. From here on the MAJOR/MINOR/PATCH policy in Governance applies.

Modified principles (template slot → ratified principle):
- slot PRINCIPLE_1 → I. Canonical Model First
- slot PRINCIPLE_2 → II. Version-Gated, Streaming Decoders
- slot PRINCIPLE_3 → III. Golden-Corpus Testing (NON-NEGOTIABLE)
- slot PRINCIPLE_4 → IV. Minimal, Explicit Dependencies
- slot PRINCIPLE_5 → V. Compatibility-Sensitive Public API
- (no template slot) → VI. Idiomatic, Simple Go

Added sections:
- Quality Gates & Tooling (fills template section 2)
- Development Workflow & Release Process (fills template section 3)
- Governance (rules, amendment procedure, versioning policy, compliance review)

Removed sections: none. Template guidance comments removed once replaced.

Templates:
- ✅ .specify/templates/plan-template.md — Constitution Check lists gates I–VI plus a
  workflow gate; Technical Context carries the module's standing facts; source layout is
  the Go module layout; Complexity Tracking examples are parsec-specific.
- ✅ .specify/templates/spec-template.md — conditional "Source Coverage" subsection (tool,
  versions, formats, gate behaviour, Capabilities gaps, corpus); domain edge cases,
  requirement and success-criteria examples.
- ✅ .specify/templates/tasks-template.md — test tasks REQUIRED per Principle III; Go path
  conventions; corpus, version-gate, Capabilities, doc-comment, CHANGELOG and benchmark
  tasks in the phases; Go parallel example.
- ✅ .specify/templates/checklist-template.md — no constitution references; no change.
- ✅ .claude/skills/speckit-*/SKILL.md — reviewed for agent-specific references; every
  command loads .specify/memory/constitution.md generically; no change required.
- ⚠ .claude/skills/speckit-tasks/SKILL.md ("Task Generation Rules") still says tests are
  OPTIONAL. The tasks template now says REQUIRED for parsec; the skill file is spec-kit
  managed and is reinstalled on upgrade, so it was left untouched. Patch it or accept it.
- ✅ AGENTS.md, README.md, CLAUDE.md — consistent with the principles; nothing renamed.

Follow-up TODOs:
- CI does not yet enforce the coverage floors of Principle III (90% decoder packages,
  80% overall). Add the gate when the first decoder package lands (v0.0.2 milestone);
  until then reviewers check the go test -cover numbers in the PR description.
- Ratification date is the scaffold date (2026-09-02); no earlier constitution existed.
-->

# parsec Constitution

## Core Principles

### I. Canonical Model First

- `model/` is the single source of truth for load-test results. Every tool package
  (`gatling/`, `jmeter/`, `k6/`, `locust/`, `phout/`) MUST convert that tool's artefacts
  into `model` types. A tool package MUST NOT export a result type of its own for
  consumers to depend on.
- `stats/` MUST consume `model` types only and MUST NOT import a tool package. Tool
  packages MUST NOT import each other; shared helpers live in `model/` or `internal/`.
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

### IV. Minimal, Explicit Dependencies

- `model/` and `gatling/` MUST depend on the standard library only. The `deps` job in
  `.github/workflows/ci.yml` enforces this and MUST NOT be weakened or skipped.
- Any other package MAY use a third-party module only when the plan names it, explains
  why the standard library is insufficient and records the decision in `research.md`.
  Adding or upgrading a dependency requires asking first (see Development Workflow).
- The one pre-approved third-party module is `github.com/caio/go-tdigest`, for
  percentiles in `stats/`.
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
| Format | `test -z "$(gofmt -l .)"` | static |
| Module hygiene | `go mod tidy && git diff --exit-code` | static |
| Vet | `go vet ./...` | static |
| Lint | `golangci-lint` (pinned) with `.golangci.yml` | static |
| Build | `go build ./...` | test |
| Tests | `go test -race -shuffle=on ./...` | test |
| Dependency boundary | stdlib-only check on `./model/...` and `./gatling/...` | deps |

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

**Version**: 1.0.0 | **Ratified**: 2026-09-02 | **Last Amended**: 2026-09-02
