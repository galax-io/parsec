# Specification Quality Checklist: Reading Gatling 3.11.5–3.12.x Text simulation.log Files

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-09-02
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

**Iteration 1** — one failure found and fixed:

- *Success criteria are technology-agnostic*: SC-009 read "Test coverage for the decoding
  packages is at least 90%, and the module overall stays at or above 80%". "Packages" and
  "module" name the project's code organisation. Rewritten as "Automated tests exercise at
  least 90% of the log-reading code and at least 80% of the project overall, measured on
  every change." The constitution's floors (Principle III) are unchanged; only the wording is.

**Iteration 2** — all items pass. No [NEEDS CLARIFICATION] markers were ever raised: every
open question in issue #3 had a default forced by the constitution or settled by the issue
text, and each one is recorded in the spec's Assumptions section rather than deferred.

**Iteration 3 (after `/speckit-clarify`, 2026-09-02)** — five clarifications recorded, all items
still pass. Two of them removed placeholders the checklist had let through as "unambiguous"
when they were not: FR-016's unnamed per-line cap (now 1 MiB) and FR-019's unnamed "statistics
report" (now `js/global_stats.json` and `js/stats.json`). One removed a real contradiction
between FR-014 and FR-017. One added a fourth version-gate outcome. One narrowed FR-021, which
had been promising evidence the kept corpus files cannot supply.

### Constitution cross-check

Not part of the standard checklist; recorded here because `/speckit-plan` will gate on it.

- **I. Canonical model first** — this feature deliberately stops short of the canonical model
  (milestone v0.0.3). The Assumptions section states that no consumer is asked to depend on
  these record types as a result model, which is the constraint Principle I actually imposes.
  The plan's Constitution Check must confirm this boundary holds in the design.
- **II. Version-gated, streaming decoders** — FR-009, FR-009a, FR-010 to FR-012 (gate),
  FR-013 to FR-016 (malformed input, no crash, 1 MiB per-line cap), FR-017 and FR-018
  (streaming, chunked equals whole-file).
  - **Resolved against issue #3.** Issue #3's acceptance list says a malformed line is reported
    and "the read continues"; Principle II says malformed input returns an error carrying the
    line number. Clarification reversed the issue: FR-013 now fails the read at the first
    unreadable line. Rationale in the spec — a partial read cannot produce counts equal to
    Gatling's report, which is the feature's entire guarantee. The constitution supersedes the
    issue text, so no Complexity Tracking row is needed, but **issue #3 must be corrected** so
    the tracked acceptance criteria and the spec do not disagree. **Text to apply to issue #3**,
    replacing the third Acceptance bullet verbatim:

    > Given a malformed line, then the read fails with an error naming its line number and no
    > partial result is returned.

    A maintainer applies it; editing the issue is a publishing action and is not done from here
    without confirmation.
  - **Checked, not violated.** Refusing a version string that is not a plain release
    (FR-009a) does not touch Principle II's "an unknown newer version MUST decode and MUST
    surface a warning": a clean release number above the range still decodes with a warning
    (FR-011). This scope was confirmed explicitly during clarification.
- **III. Golden-corpus testing** — FR-019 to FR-023 and SC-001 to SC-003. This feature creates
  the project's first corpus entry, so it is also the first test of the recording rule added
  in constitution v1.1.0.
  - **What the kept report files carry**, read from Gatling 3.11.5's report templates
    (`charts/template/GlobalStatsJsonTemplate.scala`, `StatsJsTemplate.scala`): request counts
    total/ok/ko, response-time statistics with percentiles, response-time range buckets, and the
    mean number of requests per second. The rate was missed when FR-021 was first narrowed; FR-021b
    now compares it. Neither file carries virtual-user counts or ERROR-record counts, which is why
    FR-021a routes those to the golden record stream. Confirmation against 3.12.0 is still owed,
    and Principle III makes it unrepeatable — it happens while the run is made or not at all.
  - **Why the rate is asserted exactly.** Traced to its definition (`stats/ResultsHolder.scala`
    with `stats/buffers/GeneralStatsBuffers.scala`): the request count divided by the run span in
    whole seconds, rounded up. That is a deterministic function of the very records being decoded,
    so FR-021b asserts it exactly rather than within a tolerance, and FR-021c pins the span to
    every timestamped record kind so a user event at either end of the run is not dropped from the
    denominator. The definition is cited as evidence about the format; no code is carried over.
  - **Two unknowns the recording task must settle, both unrecoverable after archiving.**
    (1) Gatling 3.11.5 sanitises tab/CR/LF only in a request's failure message — error messages,
    scenario, request and group names are written raw — so a log Gatling wrote could contain a
    broken line, which under the fail-fast rule ends the read. (2) Gatling's reader branches on a
    request end timestamp equal to the minimum signed 64-bit integer, treating it as an event that
    never completed; whether a run of these versions can produce one is unconfirmed. Both are
    recorded in the spec's Assumptions.
- **IV. Minimal dependencies** — no dependency is implied by any requirement; the reader lives
  under `gatling/`, which is standard-library-only.
- **V. Compatibility-sensitive API** — pre-v0.1.0, so exported names may still change; the
  changelog entry lands with the implementation PR, not with this spec.
- **VI. Idiomatic, simple Go** — nothing in the spec forces a non-idiomatic shape.
