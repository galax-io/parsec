# Specification Quality Checklist: The corpus and the canary

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-09-06
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

**Status: all items pass.** Ready for `/speckit-plan`.

### Iteration 1 — 2026-09-06

Two [NEEDS CLARIFICATION] markers were raised, both scope-level, both resolved by the user before
this checklist was finalised:

| # | Question | Resolution |
|---|----------|------------|
| 1 | Reducing the exported string ceiling is an observable-behaviour change to a compatibility-sensitive API immediately before the v0.1.0 freeze, and `AGENTS.md` requires asking first. Lower it, or restate the documented budget? | **Measure first, decide in the plan.** The spec fixes the invariant (documented budget = asserted budget, and it holds for a ceiling-sized field) and fixes neither number. FR-027 requires the measurement; FR-028 requires exactly one of the two outcomes, recorded in the changelog, with approval before the exported ceiling moves. |
| 2 | Does the one-command recording run locally or on a CI runner? | **A CI workflow.** FR-018–FR-023 and User Story 4 were rewritten around a dispatch. Two consequences are now explicit requirements: the job publishes for download and never commits (FR-019), and an entry represents the runner's platform, so its note must say which machine produced it (FR-023). The five existing entries stay as recorded on macOS/arm64. |

### Iteration 2 — 2026-09-06

Re-validated after both resolutions. Zero `[NEEDS CLARIFICATION]` markers; FR-001 – FR-028 and
SC-001 – SC-008 contiguous; every mandatory section present.

### Validation detail

- **No implementation details**: the spec names no language, workflow file, test function or
  library. Tool artefacts are described by what they carry ("the account a run gave of itself"),
  not by filename — except in Source Coverage, where the corpus paths are the subject matter.
- **Testable**: every FR is stated as a MUST with an observable outcome, and each maps to at least
  one acceptance scenario. FR-027/FR-028 are stated as outcomes ("measured before either number is
  fixed"; "exactly one of two outcomes, recorded in the changelog") rather than as instructions to
  a later phase, so both remain checkable.
- **Measurable success criteria**: SC-001 (5 of 5 versions live, up from 2), SC-002 (zero uncompared
  report figures), SC-004 (zero manual copy steps, zero local tools — down from nine and three),
  SC-005 (under 5 MB), SC-006 (the reference defect fails a PR unaided), SC-007 (two numbers equal).
- **Bounded scope**: Out of Scope carries the five exclusions named by the three issues' non-goals.
