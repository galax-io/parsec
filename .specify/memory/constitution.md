<!--
Sync Impact Report
==================
Version change: 2.0.0 → 2.1.0
Bump rationale: MINOR. A section is added and nothing is removed or redefined. "Engineering
Guidance (Skills)" joins Quality Gates & Tooling; every existing principle, gate and rule is
untouched, and nothing already compliant becomes non-compliant. The versioning policy calls an
added section MINOR.

Modified principles: none.
Added sections: Quality Gates & Tooling → **Engineering Guidance (Skills)**.
Removed sections: none. Renamed principles: none.

Drafted, then immediately refreshed, and folded rather than re-versioned: the skill plugins were
updated while this amendment was still uncommitted — `samber/cc-skills-golang` 1.4.0 → 2.0.1 and
`galaxio/galaxio-gatling` 2.0.0 → 2.4.0 — so the section ships classified against what is installed
rather than against what was installed an hour earlier. No 2.2.0 is issued for that: version numbers
here describe amendments that have landed, and 2.1.0 had not. The refresh added four skills to the
classification (`golang-gopls`, `golang-refactoring`, `golang-pkg-go-dev`, and the always-active
orchestrator `golang-how-to`), named the three new Gatling skills, and made the Review clause
concrete, because the very first update proved the drift it warns about: `galaxio-gatling` 2.4.0
moved `version-lookup.md` into a new `gatling-versions` skill, which left a reference in
`specs/004-gatling-format-detection/research.md` pointing at a file that no longer exists.

Why now: coding agents in this repository carry a third-party Go skill set, and it had been
applied by taste. Reviewing it against the drafted API of spec 004 changed that API three times —
a functional option renamed to the `With` convention, two exported identifiers dropped that no
caller needed, and a written justification forced for returning an interface from a constructor —
and it also surfaced three skills whose advice would have broken a MUST. Both halves are worth
fixing in one place rather than re-deciding per feature.

What the new section does *not* do, deliberately: it does not make a skill a build gate. A skill is
versioned outside this repository, can drift between releases, and may be absent for a contributor.
The gate table above already enforces the outcomes — gofmt, golangci-lint, the stdlib-only `deps`
job, the coverage floors. The skills are how a change is got right the first time; the gates are
what catch it when it is not. A rule CI cannot check and a contributor may be unable to follow would
have been a rule in name only.

Three bounds are stated once and apply to every skill: this document wins where they disagree; a
skill never justifies a dependency, a layout change, a weakened gate or a lowered coverage floor;
and unavailability blocks nothing, because Principles I–VI say the same things in less detail.

On `golang-project-layout`, which is the one classification that moved during drafting: banning it
outright was wrong. Principle I already fixes the tool packages by name and the module's import
paths are published, so the layout questions that skill opens with are answered here and are not
open to revision — `pkg/`, `cmd/`, the architecture question and the dependency-injection question
are all out. What is genuinely open, and grows as `jmeter/`, `k6/`, `locust/` and `phout/` arrive,
is when a helper belongs in `internal/` and where a package boundary should fall once several
adapters share a problem. It is therefore listed with a carve-out rather than forbidden. The
distinction the section keeps throughout is between advice that breaks a MUST and advice for which
no occasion has yet arisen: the second is not a prohibition, and two such skills are named with the
milestone that would promote them.

Templates:
- ✅ .specify/templates/plan-template.md — Technical Context gains an **Engineering guidance** field,
  so every plan names the skills its change requires and any it must not follow. This is the
  propagation that makes the section operative rather than decorative.
- ✅ AGENTS.md — Boundaries gains one Always item and one Never item, agreeing with the section.
- ✅ .specify/templates/spec-template.md — no change; a spec is technology-agnostic by design and
  naming an engineering skill in one would be a layering error.
- ✅ .specify/templates/tasks-template.md — no change; the plan's Technical Context is where the
  skills are named, and a task list inherits it.
- ✅ .specify/templates/checklist-template.md — no constitution references; no change.
- ✅ .github/workflows/ — unchanged, and deliberately: see the "not a build gate" paragraph above.
- ✅ .golangci.yml — unchanged. `errname`, `errorlint`, `wsl_v5` and `godot` already enforce much of
  what the required-reading skills describe, which is why the section is about reading rather than
  about tooling.
- ✅ README.md, doc.go — no user-visible change; the section is contributor guidance.

Follow-up TODOs:
- The classification is re-read at every `release/X.Y.0` cut together with the Principles I–VI
  compliance review, and on every skill-plugin update. It is pinned to
  `samber/cc-skills-golang` 2.0.1 and `galaxio/galaxio-gatling` 2.4.0.
- `golang-gopls` and `golang-pkg-go-dev` are classified but currently inert: `gopls` is not on the
  machine and the `godig` plugin is not installed. Both are listed anyway, because the question each
  answers — every call site before a rename, a proposed module's licence and CVEs — is asked by
  Principles V and IV whether or not the tool is there to answer it.
- Carried forward, still unresolved: `.claude/skills/speckit-tasks/SKILL.md` says test tasks are
  OPTIONAL, which Principle III contradicts. The file is spec-kit managed and reinstalled on
  upgrade, so it is left alone and recorded here rather than patched — dropping the note would make
  the drift invisible, which is why it survives this amendment too.
- Carried forward: `.copier-answers.yml` still records the pre-v2.0.0 Structure and Architecture
  text, including a `stats/` package and the `go-tdigest` pre-approval. The file forbids manual
  edits and is rewritten by `copier update`, so the fix is to answer differently at the next update;
  until then a template update would silently revert `AGENTS.md`.
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

### Engineering Guidance (Skills)

Coding agents here have a third-party Go skill set (`samber/cc-skills-golang`, classified against
**2.0.1**) and `galaxio/galaxio-gatling` (**2.4.0**). What follows is a rule about **reading**, not
about tooling, and it MUST NOT become a build gate: a skill is versioned outside this repository,
can drift, and may be absent for a contributor. The gate table above enforces the outcomes; the
skills are how a change is got right the first time.

**An orchestrator does not override this classification.** `golang-how-to` describes itself as
always active and loads other skills by task shape. It may therefore surface a skill this section
forbids, in a context where it looks apt. The classification still binds: a skill reached through
an orchestrator is the same skill, and *Must not be followed* means the advice is not taken however
it arrived.

Three bounds apply to all of them:

- **This document wins.** Where a skill and this constitution disagree, the constitution is followed
  and the disagreement is recorded in the feature's `research.md` — the rule already in force for
  `AGENTS.md`.
- **A skill never justifies a dependency, a layout change, a weakened gate or a lowered coverage
  floor.** Those are Principles III and IV, against which a skill has no standing.
- **Unavailability blocks nothing.** Without the skill set a contributor follows Principles I–VI,
  which say the same things in less detail.

**Required reading.** Before writing code in the area named, the skill MUST be read.

| When a change… | Skill | Why it is required |
|---|---|---|
| adds, renames or changes any exported identifier | `golang-naming` | Principle V: from v0.1.0 a name is effectively permanent — changing one costs a deprecation window and a MINOR release. |
| adds or changes an error, or a path that returns one | `golang-error-handling` | Principle II requires errors carrying an offset; Principle VI requires errors as values, `%w`, `errors.Is`/`errors.As`, and no control flow by panic. |
| adds or changes a test — which is every change | `golang-testing` | Principle III is NON-NEGOTIABLE. Its third-party sections are excluded; see *Must not be followed*. |
| adds an exported identifier | `golang-documentation` | Principle V requires a doc comment on each one, stating for a decoder which tool versions it accepts. |
| adds an exported type, interface or method set | `golang-structs-interfaces` | Principle I turns on what a tool package exports, and interface placement decides who imports whom. |

**Consult when the occasion arises.** SHOULD, with the occasion stated so it is a trigger rather
than a suggestion.

| Occasion | Skill | Assessment |
|---|---|---|
| a plan states a throughput or peak-memory figure | `golang-benchmark` | The benchmark rule above is a MUST; this is how it is measured and how a regression is argued in the PR. |
| decoding untrusted input — every artefact this module reads | `golang-safety`, `golang-security` | `gosec` is enabled, and Principle II's allocation caps are exactly this subject. |
| designing a constructor or a streaming API | `golang-design-patterns` | Functional options and streaming recur here. Its dependency-injection half is out (below). |
| `golangci-lint` objects, or a linter is added or upgraded | `golang-lint`, `golang-code-style` | `.golangci.yml` is the authority; the skills explain what a linter is asking for. |
| a profile shows a real bottleneck | `golang-performance` | After measurement, never before it. |
| a bug resists the obvious explanation | `golang-troubleshooting` | Cheap to reach for, and only then. |
| an exported identifier is renamed, or every call site of one must be found | `golang-gopls` | Principle V makes a rename expensive; finding the call sites mechanically beats grepping. Needs `gopls` on the machine — without it the skill is inert, which is not a reason to skip the search. |
| existing code is restructured, or an import cycle must be broken | `golang-refactoring` | Principle VI sends an out-of-scope refactor to its own PR; this is how one is kept behaviour-preserving and split into reviewable pieces. Import cycles are a recurring shape here — a codec imports the shared package, so the shared package cannot dispatch to a codec. |
| a dependency is proposed under the ask-first rule | `golang-pkg-go-dev` | Principle IV admits a module only with a stated reason; licence, CVEs and the real API are what that reason has to survive. Needs the `godig` plugin, which is not installed — the questions stand regardless. |
| a new tool package lands, or a shared helper needs a home | `golang-project-layout` | **Carve-out below** — parts of it are settled here and are not open. |
| the probe simulation, a corpus recording, or a Gatling version question | `galaxio-gatling-pro`, `gatling-versions`, `gatling-build`, `gatling-migration`, `scala-pro` | The corpus is Principle III's evidence and the probe is the only Scala here. `gatling-versions` carries the artefact-per-Gatling-line table, which is what decides whether a version can be recorded at all — it already ruled out running the probe under 3.14.x or 3.15.x, because no `gatling-picatinny` release targets those lines. |
| the Go toolchain is bumped | `golang-modernize` | At the bump, not between them. |

**`golang-project-layout` — what is settled and what is open.** Principle I already fixes the tool
packages by name and the module's import paths are published, so the questions that skill opens with
are answered here and are not open to revision:

- packages live at the repository root. `pkg/` is not used, and moving to it would change every
  import path of a module three builds depend on;
- a `main` package lives with the thing it serves — the corpus stub sits under
  `testdata/corpus/gatling/simulation/stub/`, not under `cmd/`, and that is correct;
- the architecture question and the dependency-injection question are settled: this is a library of
  packages, and Principle IV rules out a container.

What is genuinely open, and grows as `jmeter/`, `k6/`, `locust/` and `phout/` arrive: when a helper
belongs in `internal/` rather than in a tool package, and where a package boundary falls once
several adapters share a problem. Principle I names `internal/` already; the skill is how to use it
well.

**Must not be followed.**

| Skill | The MUST it breaks |
|---|---|
| `golang-stretchr-testify`, and the testify sections of `golang-testing` | Principle III fixes the standard `testing` package; Principle IV forbids the dependency and the `deps` job enforces it. |
| `golang-samber-*`, `golang-popular-libraries` | Principle IV: no module is pre-approved, and `model/` and `gatling/` are stdlib-only. |
| `golang-dependency-injection`, `golang-google-wire`, `golang-uber-dig`, `golang-uber-fx`, `golang-samber-do` | A container is both a dependency (IV) and an abstraction with no current need (VI). This module is imported, not wired. |

**No occasion has arisen** for the service, transport, storage and telemetry skills —
`golang-cli`, `golang-spf13-cobra`, `golang-spf13-viper`, `golang-grpc`, `golang-graphql`,
`golang-swagger`, `golang-database`, `golang-observability`, `golang-samber-slog`,
`golang-concurrency`, `golang-context`, `golang-data-structures` — nor for
`golang-dependency-management` (Principle IV means `go.mod` names no requirement, and `go mod tidy`
is already a gate), `golang-continuous-integration` (the pipeline exists and the gate table above is
authoritative) or `golang-stay-updated`. This is a synchronous `io.Reader` library with no server,
no transport and no store. **These are not prohibitions.** Two are worth watching: decoding a log
while it is still being written (v0.0.9) is the milestone most likely to make `golang-concurrency`
and `golang-context` relevant, and when it lands they move up a tier deliberately rather than being
reached for by surprise.

**Review.** This classification is re-read at every `release/X.Y.0` cut, alongside the Principles
I–VI compliance review and in the same place, and whenever a skill plugin is updated. A
classification made against one version of a skill set is not evidence about the next, and this is
not theoretical: the first update after this section was written added four skills, one of them an
always-active orchestrator, and moved a reference a feature's `research.md` was citing by name. A
refresh diffs the skill inventory, re-checks the rules the tiers rest on, updates the versions
pinned above, and fixes every `research.md` left pointing at a file that moved.

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

**Version**: 2.1.0 | **Ratified**: 2026-09-02 | **Last Amended**: 2026-09-05
