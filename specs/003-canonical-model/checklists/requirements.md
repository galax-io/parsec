# Specification Quality Checklist: A Canonical Model for Load-Test Results, and Requirements Stated Once

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-09-04
**Feature**: [spec.md](../spec.md)

## Content Quality

- [x] No implementation details (languages, frameworks, APIs)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

## Requirement Completeness

- [x] No [NEEDS CLARIFICATION] markers remain
- [x] Requirements are testable and unambiguous
- [x] Success criteria are measurable
- [x] Success criteria are technology-agnostic (no implementation details)
- [x] All acceptance scenarios are defined
- [x] Edge cases are identified
- [x] Scope is clearly bounded
- [x] Dependencies and assumptions identified

## Feature Readiness

- [x] All functional requirements have clear acceptance criteria
- [x] User scenarios cover primary flows
- [x] Feature meets measurable outcomes defined in Success Criteria
- [x] No implementation details leak into specification

## Notes

- Items marked incomplete require spec updates before `/speckit-clarify` or `/speckit-plan`.

### Validation record

The draft was written in one pass and then reviewed against every criterion above. The review found
five defects. All five were fixed and the criteria re-checked; the boxes above record the state
after those fixes.

1. **Implementation detail in FR-001** — it required "the module" to define result types and
   forbade a consumer to "import a tool package". Restated in terms of what a consumer must be able
   to do, not of the structure that lets them.
2. **Implementation detail in FR-016** — it assigned the conversion to "the Gatling package".
   Restated as `System MUST`, which is the house style the template's own examples use and which
   leaves the placement to the plan.
3. **FR-015 was a build rule, not a requirement** — it read "the model MUST depend on the standard
   library only", which is the constitution's Principle IV restated in the wrong document. Rewritten
   as the consequence a stakeholder actually owns: adopting the types must not add a third-party
   dependency to a consumer's build, because three downstream builds inherit it.
4. **A gap: the version warning had no requirement** — Source Coverage said a warned run "carries
   that warning into the canonical run", and nothing required it. Added as FR-016a, because a
   conversion that dropped it would launder an unverified result into one that looks verified.
5. **An edge case overreached** — the entry for a requirement naming a position the run never
   recorded demanded that it not be reported as passing. Neither OpenNFR nor any Gatling assertion
   can state that, and the format says so explicitly. Rewritten to record what actually happens, and
   why the probe's document names exact positions instead of quantifying.

Two further changes were made in the same pass. SC-004 referred to "the budget the decoder already
meets" without naming it, which is not measurable as written; it now names the 32 MiB figure the
project already promises. SC-011 was added, because the recorded-name rule for a group declared
with a comma — an explicit requirement of issue #30, now FR-029 — had no measurable outcome.

### Clarification session 2026-09-04

Five questions were asked and answered; the spec records each in `## Clarifications`. Three of them
settled decisions this checklist had flagged as taken-rather-than-derived, and two arose from facts
found while asking:

1. **A run does not hold its samples.** Key Entities said it did, while FR-017 and SC-004 required
   bounded memory — a contradiction on any log larger than memory, of the same kind the 002 session
   caught between its FR-014 and FR-017. Resolved in favour of streaming; FR-011a added.
2. **The Gatling wire records stay exported**, documented as the format's own events. FR-014a added.
3. **`Aggregate` does not ship here.** Deferred whole to v0.5.0 against issue #4's explicit
   requirement, on Principle VI. User Story 4 (summary-only sources), FR-010 as written, the
   `Aggregate` entity and SC-006 were removed or rewritten; the remaining stories renumbered.
   **Issue #4 must be corrected before the implementation PR merges.**
4. **The OpenNFR renderer already exists and this repository writes none.** This corrected an error
   in the draft: the draft asserted, from `docs/assertions.md`, that no OpenNFR implementation
   existed in any language and that the probe's exact counts were therefore unexpressible. Wrong on
   both counts — `gatling-picatinny` v1.26.0 added `OpenNfrAssertions.fromYaml`, and its renderer
   supports `op: eq` and a failed count. It also already refuses a document totally and loudly, which
   is what FR-024 asks for, and refuses `good`, which is why FR-025 restates successful counts.
5. **User Story 4 is conditional on a version question.** The renderer's line targets Gatling 3.13.x
   and the probe must run 3.11.5 and 3.12.0. Planning verifies first; if it cannot be made to run,
   the story moves whole to v0.0.5 and no substitute renderer is written.

### Findings recorded rather than asked about

The question quota was reached, so two facts found while researching are written into the spec as
constraints instead:

- **The comma rule is not applied for the author.** Issue #30 requires the translation to handle it
  "so an author never has to know it". The renderer addresses Gatling by recorded names and applies
  no substitution, so the probe's document must carry the recorded spelling. FR-029 says so and
  Assumptions records why renaming the probe's group was rejected — spec 002 requires a comma in a
  corpus group name to prove the split is lossless. The fix belongs upstream.
- **The renderer tracks OpenNFR upstream v0.8.0** and places that surface outside its
  binary-compatibility guarantee. The document is written to what the renderer accepts; a divergence
  from the currently published schema is a planning finding, not a run-time surprise.

### Earlier record: decisions taken as informed guesses


These three were taken rather than derived when the spec was written. Items 1 and 2 were put to the
clarification session above: item 1 was confirmed, item 2 was reversed. Item 3 was not asked — the
quota was spent on higher-impact questions — so it stands as taken and is still worth confirming.

1. **The Gatling wire records stay exported**, with the canonical types documented as what consumers
   build on. This resolves the Complexity Tracking row that spec 002 left for this milestone, and it
   is the reading of Principle I with the widest blast radius: it fixes the public surface that
   v0.1.0 will promise stability for.
2. **`Aggregate` ships now**, ahead of the first summary-only source (Locust, v0.5.0). Issue #4
   requires it; Principle VI's ban on speculative abstraction argues against it. The tie-break taken
   here is that admitting a summary-only source after v0.1.0 would be a breaking change for three
   downstream builds.
3. **Run-level errors and the opaque assertion payload live on the run.** Issue #4's type list names
   no home for either, and both are facts about the run rather than about any sample.

### Dependency noted during validation

Issue #4 records a dependency on [opennfr#119](https://github.com/galax-io/opennfr/issues/119),
which is open. The `opennfr` repository itself holds no implementation — its README says nothing
reads a document yet — so the Go-side dependency is on the **vocabulary**, which is published and
settled, and not on code. That is unchanged by the clarification session. What did change is the
Gatling side: the rendering half is not waiting on `opennfr`, because `gatling-picatinny` already
ships it.
