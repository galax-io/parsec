# Specification Quality Checklist: Telling Which Gatling Wrote a simulation.log

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-09-05
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

### The open item, now closed

**FR-031** carried the single `[NEEDS CLARIFICATION]` marker: does milestone v0.0.4 record complete
binary Gatling runs, or only the minimum artefact needed to prove that a binary log is identified as
binary?

**Resolved 2026-09-05: the minimum.** One sample of the leading bytes of a real binary
`simulation.log` proves identification; the complete 3.13.5, 3.14.9 and 3.15.1 recordings belong to
v0.0.5, which needs them for its decoder and cannot ship without them. Format is identified for all
five versions issue #5 names, version for the two this module can decode. FR-031 now states the
scope, FR-031a says what the sample must be and forbids it being counted as a corpus entry, and
FR-031b forbids the feature from claiming a binary log's version is identified at all. The
Clarifications section records the question, the answer, the rejected alternative and why, and
Assumptions carries the residual risk in writing rather than leaving it implicit.

This narrows issue #5's acceptance bullet, so issue #5 should be amended — the same treatment spec
002 gave issue #3.

### Findings from validation, all resolved in the spec

- **Issue #5's proposed detection rule is falsified by this repository's own corpus.** The issue says
  `'R'` starts the text `RUN` line. Both `testdata/corpus/gatling/3.11.5/simulation.log` and
  `3.12.0/simulation.log` begin with `A` — Gatling writes an `ASSERTION` record per declared
  assertion ahead of the run header. The spec states the corrected rule in FR-003, lists the case as
  the first Edge Case, and records the correction in Assumptions so the issue can be fixed. Caught by
  reading the corpus rather than the issue.
- **Double-warning was unspecified.** Identification, dispatch and the codec could each raise the
  above-range warning, giving a caller three copies of one fact. FR-016 and SC-004 now pin it to
  exactly one, counted, through every path.
- **The three refusals were collapsible.** "Not a Gatling log", "a Gatling log this module cannot
  read yet" and "a Gatling log whose version is unsupported" are different answers with different
  remedies, and a consumer that cannot tell them apart cannot tell a user what to do. FR-010 requires
  them to be distinguishable by a program without matching message text; US2 scenario 3 asserts it.
- **Stream consumption was a correctness risk, not an ergonomic one.** Issue #6 records that the
  binary codec reconstructs a string cache from byte 0, so a detector that swallowed its peeked bytes
  would silently corrupt every binary read. FR-004 states it as a correctness requirement and says
  why.
- **The public API change was unflagged.** Strictness has to be expressible where a read begins,
  which changes the text codec's constructor — an `AGENTS.md` ask-first item and a Principle V
  changelog item before v0.1.0. The spec names it in Assumptions and requires the plan to state the
  exact signature for approval; FR-032 and SC-010 hold existing behaviour fixed.
- **`0x00` as the binary opening byte is unproven here.** It is issue #6's reading of the layout and
  nothing in this repository has ever read a binary log. Recorded in Assumptions as a claim that a
  recording settles, in the same form spec 002 used for its source-derived claims. Under the resolved
  scope, the sample required by FR-031a is what settles it.

### On the implementation-details items

Repository paths (`gatling/`, `testdata/corpus/gatling/…`, `CHANGELOG.md`) appear only in
Source Coverage and Assumptions, which the spec template asks to be concrete, and follow the
precedent set by specs 002 and 003. The Requirements and Success Criteria sections state behaviour
and outcomes only. The one Assumptions bullet that touches package structure — dispatch cannot live
in `gatling/` without an import cycle — says explicitly that where it does live is the plan's
decision, not a requirement.

### Status

**Complete — 16 of 16 items pass.** No clarification markers remain and the scope is settled, so
`/speckit-clarify` has nothing left to ask. Ready for `/speckit-plan`.
