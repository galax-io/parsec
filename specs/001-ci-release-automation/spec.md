# Feature Specification: Continuous Verification, Tag-Driven Release and Dependency Automation

**Feature Branch**: `001-ci-release-automation`

**Created**: 2026-09-02

**Status**: Draft

**Input**: User description: "нужен ci для проверки, релиза по ручному тегу, dependabot, другой отслеживатель зависмостей как scala steward, всегда должны быть интеграционные тесты e2e"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Every change gets one complete verdict before it can merge (Priority: P1)

A contributor pushes a change and, without asking anyone, learns whether it is mergeable. Formatting, module hygiene, static analysis, lint, build, unit tests with data-race detection, the dependency boundary and test coverage are all checked automatically, and the result is a single verdict rather than a pile of logs to interpret. Nothing that the project calls a gate is left to a reviewer to remember.

**Why this priority**: every other story in this feature protects something. This one is the thing being protected. The project already publishes a Quality Gates table as a promise; today that promise is kept by a contributor typing commands locally, which means it is not kept at all.

**Independent Test**: open a change that breaks exactly one gate — an unformatted file, a failing unit test, a third-party import added to a stdlib-only package — and confirm the change is blocked and the report names the gate that failed. Delivers value on its own: the repository becomes unable to accept a red change.

**Acceptance Scenarios**:

1. **Given** a change whose source files are not formatted to the project's canonical style, **When** verification runs, **Then** the change is blocked and the report names the offending files.
2. **Given** a change that adds a third-party import to a package declared standard-library-only, **When** verification runs, **Then** the change is blocked and the report names both the package and the forbidden dependency.
3. **Given** a change that leaves the dependency manifest untidy, **When** verification runs, **Then** the change is blocked and the report shows the difference.
4. **Given** a change that touches only Markdown, `docs/`, `specs/` or `.specify/`, **When** verification runs, **Then** no code gate runs and the change is mergeable on review alone.
5. **Given** a change that lowers a package below its coverage floor, **When** verification runs, **Then** the resulting coverage is reported against that floor.
6. **Given** verification has passed, **When** a maintainer looks at the change, **Then** one required verdict tells them it is mergeable; they do not have to open individual job logs to find out.
7. **Given** a contributor who authored the change, **When** they attempt to merge without a passing verdict, **Then** the merge is refused.

---

### User Story 2 - The end-to-end suite always runs and can never pass by skipping (Priority: P1)

Every supported source has at least one end-to-end journey that is exercised on every change: a real recorded run of the tool goes in, the canonical model and the derived statistics come out, and both are compared against the recording and against the report the tool itself produced for that run. If the environment the suite needs is missing, the run fails loudly and names what was missing — it never reports success because every case skipped.

**Why this priority**: golden-corpus testing is non-negotiable for this project, and an end-to-end journey is the only evidence that a decoder is correct rather than merely plausible. Today no such journey runs automatically, so the only thing between a wrong decoder and a published release is a command a contributor may never type. "Always" is the explicit requirement in the feature request.

**Independent Test**: remove or corrupt the recorded artefact the suite depends on and confirm the run fails naming the missing input, rather than passing with everything skipped.

**Acceptance Scenarios**:

1. **Given** a supported tool and version with a recorded run **and a decoder for it**, **When** the end-to-end suite runs, **Then** the decoded record stream and the derived statistics are compared against the recording and the tool's own report within the documented tolerance, and any difference blocks the change.
2. **Given** the inputs or tooling the suite needs are unavailable, **When** the suite runs, **Then** it fails naming what was missing and the change is blocked.
3. **Given** every case in the suite skipped, **When** the run finishes, **Then** it is reported as a failure, not a success.
4. **Given** a change that adds support for a new tool or a new version of a tool, **When** verification runs, **Then** it is blocked until an end-to-end case covering that tool and version exists.
5. **Given** the suite passed, **When** a maintainer inspects the result, **Then** they can see which tool, version and artefact format each executed case covered, and how many cases ran.
6. **Given** a change that only touches documentation, **When** verification runs, **Then** the end-to-end suite does not run.
7. **Given** the module has no decoder yet, **When** the end-to-end suite runs, **Then** the scaffold case executes against its committed recording, reports itself as harness-level, and the run is green for a real reason rather than because nothing ran.
8. **Given** a recording committed without the report its tool produced and without a statement that the tool version produced none, **When** the suite runs, **Then** the entry is refused — a forgotten report and a genuinely report-less version are never indistinguishable.
9. **Given** a run in which one case executed and failed, **When** the result is read, **Then** it reports one case executed and names the failure, and does not report that no case executed.

---

### User Story 3 - Releasing is pushing a tag, and nothing else (Priority: P2)

A maintainer publishes a version by pushing a version tag on an allowed branch. Everything after that happens by itself: the tag is validated against the branch it sits on, the project's milestone and issue-linkage rules are checked, release notes are generated from the commit history, a release is published, and the version becomes fetchable by downstream consumers. If any precondition fails, nothing is published and the reason is stated.

**Why this priority**: nothing is releasable until there is something to release, so this ranks below verification. But the release procedure is already written down as though the automation existed, and a documented process with no enforcement behind it is a trap — the first release is exactly when a mistake becomes permanent.

**Independent Test**: push a tag that violates a rule — a tag on a branch that is not allowed to carry one, a version that does not match its release branch, a milestone with an unclosed issue — and confirm nothing is published and the refusal names the rule.

**Acceptance Scenarios**:

1. **Given** a version tag pushed on an allowed branch with all preconditions met, **When** the release runs, **Then** a release is published with notes generated from the commits since the previous tag, and the version is resolvable by a downstream consumer.
2. **Given** a version tag pushed on a branch that is not allowed to carry one, **When** the release runs, **Then** it is refused, nothing is published, and the refusal names the rule.
3. **Given** a version tag whose version does not match the release branch it sits on, **When** the release runs, **Then** it is refused naming both the tag and the branch.
4. **Given** a milestone with an open issue whose fix is already merged, or a change merged since the previous tag that carries no milestone, **When** the release runs, **Then** it is refused naming the offending issue or change.
5. **Given** a release published, **When** the maintainer opens it, **Then** the notes list the changes grouped by type and need no manual editing.
6. **Given** a release that failed partway, **When** the maintainer fixes the cause and re-runs it, **Then** the outcome is the same as a first successful run — no duplicate or half-published version.
7. **Given** a version that has already been released, **When** the same version is pushed again, **Then** it is refused rather than overwriting the existing release.
8. **Given** any event other than a version tag — a merge, a schedule, a manual trigger — **When** it occurs, **Then** no release is published.

---

### User Story 4 - Dependency updates arrive as batched, reviewable proposals (Priority: P2)

Updates to the module's dependencies, to the automation's own building blocks, to the pinned linter and to the language toolchain arrive as proposed changes on a predictable schedule, grouped so that routine review is a few minutes rather than an afternoon. Each proposal goes through exactly the same verification as any other change, and nothing is ever applied without one.

**Why this priority**: an unreadable log format is a bigger risk to this project than a stale dependency, so this ranks below the decoding gates. But three downstream builds inherit every transitive dependency here, along with its vulnerabilities and its upgrade schedule, and the two things the project pins by hand today — the linter version and the language toolchain — are precisely the ones nobody notices going stale.

**Independent Test**: for each tracked family of dependencies, confirm a proposal appears on schedule, is labelled by family, and is blocked when it fails verification.

**Acceptance Scenarios**:

1. **Given** new minor or patch releases exist for tracked dependencies, **When** the scheduled scan runs, **Then** one grouped proposal per family is opened, labelled, and verified like any other change.
2. **Given** a new major release of a direct dependency, **When** the scan runs, **Then** it is proposed on its own, separate from the grouped proposal, so its risk is reviewed on its own terms.
3. **Given** both updaters are configured, **When** they scan, **Then** each tracked family is owned by exactly one of them and no update is ever proposed twice.
4. **Given** a published vulnerability affecting something in the dependency tree, **When** verification runs, **Then** it is surfaced against the affected change, naming the advisory.
5. **Given** an update proposal that fails verification, **When** a maintainer looks at it, **Then** it is blocked exactly as any other failing change would be.
6. **Given** a scheduled scan that finds nothing to update, **When** it finishes, **Then** it produces no proposal and no notification.
7. **Given** two open update proposals that both edit the dependency manifest and one is merged, **When** verification re-runs on the other, **Then** the conflict is surfaced rather than silently merged.

---

### Edge Cases

- What happens when a change is proposed from a fork, where the automation cannot be trusted with the repository's credentials — does the contributor still get a verdict, and which gates are withheld?
- What happens when a version tag is deleted and re-pushed while publication is already under way?
- What happens when a recording the end-to-end suite depends on is missing, truncated or unreadable — is that distinguishable from a genuine decoder failure?
- What happens when the module has no packages yet and coverage is therefore undefined rather than low?
- What happens when the automation's own configuration is the thing being changed — is a change that breaks a gate visible before it merges, or only after?
- What happens when verification takes longer than a contributor is willing to wait — is there a fast subset that fails early on the common mistakes?
- What happens when a scheduled scan and a release run at the same time?
- What happens when the release notes span a range containing commits that do not follow the message convention?

## Requirements *(mandatory)*

### Functional Requirements

#### Verification

- **FR-001**: Every proposed change to code MUST be verified against the project's full published gate set — formatting, dependency-manifest hygiene, static analysis, lint, build, unit tests executed with data-race detection enabled and in randomised order, and the standard-library-only boundary for the packages that declare it — before it becomes mergeable.
- **FR-002**: A change that touches only Markdown, `docs/`, `specs/` or `.specify/` MUST NOT trigger code gates, and MUST remain mergeable on review alone.
- **FR-003**: Verification MUST report each package's test coverage against the floor that applies to it, and MUST make a floor breach visible on the change itself.
- **FR-004**: A verification result MUST identify which gate failed and where, precisely enough for a contributor to act without first reproducing the failure locally.
- **FR-005**: Verification MUST NOT be skippable, overridable or re-triggerable into a pass by the author of the change.
- **FR-006**: A change MUST NOT be able to reach the trunk without a passing verification result.
- **FR-007**: The automation's own configuration MUST be changed only through the same proposal-and-review path as any other change, and a change to it that breaks a gate MUST be visible before it merges.

#### End-to-end testing

- **FR-008**: An end-to-end case MUST take a real recorded run of a supported tool as its input. Once a decoder for that tool exists, the case MUST compare both the decoded record stream and the derived statistics against the recording and against the report that tool produced for that same run; until then it exercises the harness and reports itself as doing only that (FR-014, FR-032). A case MUST declare which of the two it did, so a harness-level case can never be counted as decoder coverage.
- **FR-009**: The end-to-end suite MUST run as part of the verification of every proposed change to code.
- **FR-010**: An end-to-end run in which no case executed MUST be reported as a failure. Skipping MUST NOT be a path to a green result.
- **FR-011**: An end-to-end run MUST report how many cases executed — executed, not passed — and which tool, version, artefact format and coverage level each one covered. A case that ran and failed MUST be reported as executed and failed, never as absent, so that "no case executed" keeps its single meaning.
- **FR-012**: A change that adds support for a new tool, a new version of a tool, or a new artefact format MUST be blocked until an end-to-end case covering it at decoder level exists. Because a registry of executed cases can only report what ran and never what was never written, this MUST be checked by comparing the declared set of supported tools, versions and formats against the executed decoder-level cases, and failing on any declared entry with no case.
- **FR-013**: When the end-to-end suite cannot read a recording it needs, it MUST fail naming the missing or unreadable input, and that failure MUST be distinguishable from a decoding mismatch.
- **FR-014**: The end-to-end gate MUST be live from the first release of this feature, not deferred. Before any decoder exists it MUST be proven by a scaffold case: one real recorded run committed to the corpus, exercised end to end by the harness, so that the rule in FR-010 is enforceable on day one and every later decoder plugs into a harness already shown to work.
- **FR-015**: An end-to-end case MUST be the replay of a committed recording of a real run: the recorded artefact is the input, the recording and the tool's own report for that run are the expected output. It MUST NOT depend on executing the load-testing tool during the run, so that every case is deterministic and cheap enough to run on every proposed change.

#### Corpus recording

- **FR-031**: A corpus recording MUST capture, at the moment the run is made, everything a later
  comparison will need: the artefact exactly as the tool produced it, and the tool's own report for
  that same run. None of this is recoverable afterwards — an archived run cannot be re-run, and
  Gatling stopped producing statistics reports in 3.13.5, so a run recorded without its report can
  never be used to check a decoder's numbers. This MUST be enforced mechanically, not by review: a
  recording that carries no report MUST be refused unless it also carries an explicit statement that
  the tool version produced none, so that a forgotten report and a genuinely report-less version are
  never indistinguishable.
- **FR-032**: The comparison itself — decoded records against the recording, derived statistics
  against the tool's report within a documented tolerance — MUST be declared as required future work
  rather than built by this feature. What this feature delivers is a live gate, a harness, and a
  recording complete enough that the comparison can be written later without recording the run again.

#### Release

- **FR-016**: Pushing a version tag MUST be the only trigger that publishes a release. No merge, schedule or manual trigger MUST publish one. For a public module whose registry resolves versions from tags on demand, the tag push itself already exposes the version before any check can run; the release preconditions therefore MUST also be runnable locally, before the tag is pushed, and the documented procedure MUST run them there rather than using a pushed tag as the test.
- **FR-017**: A release MUST be refused unless the tag sits on the trunk or on a release branch whose name matches the tag's major and minor version.
- **FR-018**: A release MUST be refused unless every change merged since the previous tag carries the release's milestone, and every issue in that milestone whose fix is already on the trunk is closed. The first clause MUST be checked against the commit range since the previous release, not against the milestone's own membership: a change carrying no milestone is by definition absent from a milestone query, so a milestone-only check cannot see the case the clause exists to catch.
- **FR-019**: A release MUST publish notes derived from the commit history since the previous tag, grouped by change type, requiring no manual editing.
- **FR-020**: A refused or failed release MUST leave no partially published state, and re-running it after the cause is fixed MUST produce the same outcome as a first success. Every step that can fail transiently MUST therefore run before the first irreversible one, so that no transient failure can leave a state the "already released" check will refuse to retry.
- **FR-021**: A released version MUST be resolvable by a downstream consumer using the module's published path and that version, with no further action by the maintainer.
- **FR-022**: A version that has already been released MUST NOT be republished or overwritten.
- **FR-023**: A release MUST be refused unless the code at the tag passes the same gate set required of any merged change.

#### Dependency tracking

- **FR-024**: The system MUST propose updates on a recurring schedule for the module's own dependencies, for the building blocks the automation itself uses, for the pinned lint tooling, and for the pinned language toolchain version.
- **FR-025**: Every proposed update MUST arrive as a reviewable change subject to the same verification as any other; no update MUST be applied without one.
- **FR-026**: Minor and patch updates MUST be grouped into a single proposal per family per cycle; a major update MUST be proposed on its own.
- **FR-027**: Each tracked family MUST be owned by exactly one updater, so that no update is ever proposed twice. The first updater MUST own the module's own dependencies and the automation's building blocks, and MUST remain the source of vulnerability advisories. The second updater MUST own everything the first cannot reach — the pinned lint tooling, the pinned language toolchain version, and version strings embedded in the automation's own configuration — and MUST NOT propose updates for a family the first one owns.
- **FR-028**: Update proposals MUST be labelled by family so they can be filtered and triaged apart from feature work.
- **FR-029**: A published vulnerability affecting anything in the dependency tree MUST be surfaced against the affected change, naming the advisory.
- **FR-030**: A scheduled scan that finds nothing MUST produce no proposal and no notification.

### Key Entities

- **Verification run**: the complete verdict for one proposed change. Composed of gates; has one outcome that decides mergeability; identifies which gate failed and where.
- **Gate**: one named, independently reportable check with a pass/fail outcome and a documented reason for existing. Belongs to the project's published gate set.
- **End-to-end case**: one recorded real run of one tool at one version in one artefact format, together with the report that tool produced for it and the tolerance within which derived statistics must agree.
- **Release**: one immutable published version, identified by a version tag, carrying generated notes covering the range since the previous release, and resolvable by downstream consumers.
- **Release precondition**: a rule the tag, its branch and the milestone state must satisfy before anything is published.
- **Tracked family**: one group of versioned things watched by exactly one updater — the module's dependencies, the automation's building blocks, the pinned lint tooling, the language toolchain.
- **Update proposal**: one reviewable change raised by an updater, belonging to one family, labelled, grouped by update size, and verified like any other change.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every gate the project publishes as required is enforced automatically; none depends on a reviewer remembering to check it.
- **SC-002**: A contributor receives a complete verdict on a typical change within 10 minutes of proposing it, and a formatting or build mistake is reported within 3 minutes.
- **SC-003**: Every supported tool and version is covered by at least one end-to-end case; no supported version is covered by unit tests alone.
- **SC-004**: An end-to-end run that executes no case is reported as a failure every time it happens.
- **SC-005**: A maintainer publishes a release by pushing one tag and performing no other manual step, and the generated notes need no editing.
- **SC-006**: Every release attempt that violates a branch, version or milestone rule is refused before anything is published.
- **SC-007**: A released version is resolvable by a downstream consumer within 15 minutes of the tag being pushed.
- **SC-008**: Updates are proposed at least weekly for every tracked family, and no update is ever proposed twice.
- **SC-009**: Routine dependency review costs a maintainer under 10 minutes a week in steady state.
- **SC-010**: No change reaches the trunk without a passing verification run.
- **SC-011**: A first-time contributor can tell from the verdict alone what to fix, without asking a maintainer, for every gate in the set.
- **SC-012**: Every corpus recording carries the report its tool produced, or an explicit statement that the tool version produced none; a recording missing both is refused every time.
- **SC-013**: A failed end-to-end case and an absent corpus are distinguishable from the reported result alone, without opening a log.

## Assumptions

- **Existing gates stay authoritative.** The verification set is the one already published in the project constitution's Quality Gates table; this feature automates and completes it rather than redefining it. Where the two disagree, the constitution wins.
- **Coverage floors are reported before they are enforced.** With no packages in the module yet, coverage is undefined rather than low, so a hard floor would fail on an empty tree. Coverage is measured and reported from day one and becomes blocking with the first decoder package. This matches the follow-up already recorded in the constitution.
- **Release mechanics follow the documented process.** Trunk-based with `release/X.Y.0` branches, tags only on the trunk or a release branch, notes generated from commit history, distribution through the language's own module proxy. The existing linkage-check script is the milestone precondition; this feature wires it in rather than reinventing it.
- **Fork contributions get the untrusted-input treatment.** Changes proposed from forks receive every gate that needs no repository credentials; anything that does is withheld until a maintainer approves the run. A fork contributor still gets a verdict.
- **The second updater exists to reach what the first cannot.** The point of adding a second dependency tracker is coverage of things outside the first one's reach — pinned tool versions, the language toolchain, version strings embedded in the automation — not a second opinion on the same dependencies.
- **The corpus recording rule now lives in the constitution.** FR-031 outlives this feature: it
  applies to every tool and version anyone records from here on, so it was landed in Principle III
  as constitution amendment **v1.1.0** rather than left as a requirement of this spec alone.
  Principle III already required statistics to be compared against the tool's own report but did not
  say that report must be committed at recording time — the half that is impossible to fix later.
  FR-031 remains here as this feature's own obligation; the constitution is what binds every feature
  after it.
- **The recorded corpus is the input, not a mock.** End-to-end cases replay artefacts produced by real runs of the real tool, committed to the repository. The suite never executes the load-testing tool itself, so a case is deterministic, needs no external runtime, and is cheap enough to run on every change.
- **The scaffold case is a real recording, not a stub.** The case that proves the harness before the first decoder exists is built on an artefact from a genuine tool run committed to the corpus, and it stays in the suite once real decoder cases join it. A synthetic placeholder would prove only that the harness can pass.
- **This work belongs to the scaffold milestone.** It is infrastructure for everything that follows and blocks nothing in the decoding backlog; it should be linked to the lowest-numbered open milestone (v0.0.1 Scaffold) unless a maintainer places it elsewhere.
- **Verification runs on the automation's own configuration.** Changes to workflow definitions, updater configuration and gate scripts are verified before merge, so the gate set cannot be quietly weakened.

## Out of Scope

- Publishing binaries, container images or any artefact other than the module itself — this is a library.
- Deploying anything to a runtime environment.
- Watching upstream load-testing tool releases for format drift and warning when a new version changes the log format. That is the "canary" already tracked separately (milestone v0.2.0, issue #15) and depends on decoders that do not exist yet.
- Executing a load-testing tool inside the automation to generate a fresh artefact. Every end-to-end case replays a committed recording; detecting format drift on a new tool release is the separate "canary" work above.
- Verifying that a downstream consumer resolves and builds against a released version. The release makes the version resolvable (FR-021); proving a specific consumer still compiles is that consumer's pipeline.
- Comparing decoded output against a recording, and derived statistics against the tool's own report.
  There is no decoder to compare, so there is nothing to write the assertion against; the requirement
  is recorded in FR-031 and FR-032 instead of being guessed at now. Any expected-value format the
  comparison needs — a manifest, a naming convention, assertions in Go — is the first decoder's
  decision, made when there is something to verify.
- Signing or attestation of released artefacts.
- Automatically merging any proposal, dependency updates included.
- Changing the gate set itself, the coverage floors, or the release procedure. Those live in the constitution; this feature enforces them.
