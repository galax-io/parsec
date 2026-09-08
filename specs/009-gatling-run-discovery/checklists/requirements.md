# Specification Quality Checklist: Finding the run

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-09-08
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

### Validation record — iteration 1 (2026-09-08)

Three items failed on the first pass. All were fixed in `spec.md` and the checklist re-run; iteration
2 passes every item.

1. **No implementation details** — FAILED. The Context section named three Go constructors and an
   interface to describe the status quo. The sibling specs name no Go identifier anywhere, and those
   entry points are exactly what planning is free to change. Rewritten as prose: "every entry point
   takes a stream of bytes and starts decoding".
2. **Requirements are testable and unambiguous** — FAILED, and the two requirements contradicted each
   other. FR-002 read as a fallback chain ("an explicit path first; then `lastRun.txt`; then the
   newest"), while FR-003 said an explicit path is honoured "whether the path resolves or not" — so a
   named path that missed either did or did not fall through to the newest run, depending on which
   requirement was read. FR-002 now classifies the given path before anything else, and the
   `lastRun.txt`/newest order applies only *inside* a results root; FR-003 now states plainly that a
   path naming a run is never searched past and that a path naming nothing fails rather than falling
   back to the default.
3. **All acceptance scenarios are defined** — FAILED. US1 scenario 1 said "resolved with no explicit
   path" against a root that had been supplied, conflating the two cases, and the one that actually
   exercises FR-010's default — no path at all — had no scenario. Split into scenarios 1 and 2, and the
   rest of the story renumbered.

One further inaccuracy was corrected while re-reading: SC-001 listed Maven, sbt and Gradle as three
results-root layouts. Maven and sbt write to the same one, so it now names two.

### Known deviations, kept deliberately

- **SC-009 is not technology-agnostic.** It names the coverage floors and the standard-library-only
  boundary, which are gates rather than outcomes a user can see. Kept because the constitution's
  Quality Gates table makes them an obligation of every feature here, and the sibling spec states them
  the same way (008's SC-006). SC-001 through SC-008 are verifiable without knowing how the module is
  built.
- **Package paths appear in Assumptions and Out of Scope.** `model/`, `gatling/text/`,
  `gatling/binary/` and `gatling/simlog/` are named to say what this feature does *not* touch. That is
  scope bounding, which the checklist asks for, and it is how the sibling specs bound theirs.

### Deliberate judgements, recorded so review can challenge them

These were resolved as documented assumptions rather than as `[NEEDS CLARIFICATION]` markers, because
each has a defensible default and #11 or the constitution settles the direction. Each is stated in the
spec's *Assumptions* section, which is where a reviewer should push back:

- Discovery returns a **location**, not an opened log. #11's "Where" names run-directory discovery,
  and a second way to open a log would be a second public decoding interface — rejected in #10 and
  comet#3.
- The default results root is the **Maven layout**, exactly as #11 directs, rather than a search over
  the known layouts. sbt shares it, so one default covers two of the three build tools.
- A results root is searched **one level deep**, which is the layout Gatling writes.
- **Modification time** is the fallback, per #11, with a deterministic tie-break added because a clone
  or a cache restore flattens every timestamp in the root.
- A **new corpus recording** is required for the results-root shape. The existing entries preserved
  each run directory but not its enclosing root, and Principle III makes that unrecoverable after the
  fact. This is the one item in the feature that cannot be produced later, and the one most worth
  challenging before planning.

### Constitution alignment

- **Principle I** — no statistic, no new `model` field, no `Capabilities` change. Stated in *Source
  Coverage* and *Out of Scope*.
- **Principle II** — no version gate is added or pre-empted (FR-014); discovery reads no log (FR-013).
- **Principle III** — the one new recording is named, with the reason it cannot be produced later.
- **Principle IV** — standard library only, stated in *Assumptions* and *Dependencies*.
- **Principle V** — new exported surface one milestone before the v0.1.0 freeze; doc comments and
  `CHANGELOG.md` are FR-015.
