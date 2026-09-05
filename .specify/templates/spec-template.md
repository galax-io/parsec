# Feature Specification: [FEATURE NAME]

**Feature Branch**: `[###-feature-name]`

**Created**: [DATE]

**Status**: Draft

**Input**: User description: "$ARGUMENTS"

## User Scenarios & Testing *(mandatory)*

<!--
  IMPORTANT: User stories should be PRIORITIZED as user journeys ordered by importance.
  Each user story/journey must be INDEPENDENTLY TESTABLE - meaning if you implement just ONE of them,
  you should still have a viable MVP (Minimum Viable Product) that delivers value.

  Assign priorities (P1, P2, P3, etc.) to each story, where P1 is the most critical.
  Think of each story as a standalone slice of functionality that can be:
  - Developed independently
  - Tested independently
  - Deployed independently
  - Demonstrated to users independently
-->

### User Story 1 - [Brief Title] (Priority: P1)

[Describe this user journey in plain language]

**Why this priority**: [Explain the value and why it has this priority level]

**Independent Test**: [Describe how this can be tested independently - e.g., "Can be fully tested by [specific action] and delivers [specific value]"]

**Acceptance Scenarios**:

1. **Given** [initial state], **When** [action], **Then** [expected outcome]
2. **Given** [initial state], **When** [action], **Then** [expected outcome]

---

### User Story 2 - [Brief Title] (Priority: P2)

[Describe this user journey in plain language]

**Why this priority**: [Explain the value and why it has this priority level]

**Independent Test**: [Describe how this can be tested independently]

**Acceptance Scenarios**:

1. **Given** [initial state], **When** [action], **Then** [expected outcome]

---

### User Story 3 - [Brief Title] (Priority: P3)

[Describe this user journey in plain language]

**Why this priority**: [Explain the value and why it has this priority level]

**Independent Test**: [Describe how this can be tested independently]

**Acceptance Scenarios**:

1. **Given** [initial state], **When** [action], **Then** [expected outcome]

---

[Add more user stories as needed, each with an assigned priority]

### Edge Cases

<!--
  ACTION REQUIRED: The content in this section represents placeholders.
  Fill them out with the right edge cases. For anything that reads a tool artefact the
  constitution (Principle II) already fixes the answers for version-below-range,
  unknown-newer-version and malformed input; state them here so they become acceptance
  scenarios rather than surprises.
-->

- What happens when [boundary condition, e.g., "the artefact is truncated mid-record"]?
- How does the system handle [error scenario, e.g., "a version below the supported range"]?
- What happens when [unknown newer version, e.g., "a log written by a version not yet in the corpus"]?
- What happens when [empty or degenerate input, e.g., "a run with zero requests"]?

## Requirements *(mandatory)*

<!--
  ACTION REQUIRED: The content in this section represents placeholders.
  Fill them out with the right functional requirements.
-->

### Functional Requirements

- **FR-001**: System MUST [specific capability, e.g., "decode every record kind of a Gatling 3.12.x text simulation.log"]
- **FR-002**: System MUST [gate behaviour, e.g., "refuse a log below version 3.11.5 with an error naming the version found and the range supported"]
- **FR-003**: Consumers MUST be able to [key interaction, e.g., "ask which fields this source provides before rendering a report"]
- **FR-004**: System MUST [data requirement, e.g., "report a field the source does not carry as absent, never as zero"]
- **FR-005**: System MUST [behaviour, e.g., "produce identical records for chunked and whole-file reads"]

*Example of marking unclear requirements:*

- **FR-006**: System MUST expose [primitive, e.g., "the position a sample was recorded at"] [NEEDS CLARIFICATION: is this a definition this module owns, or arithmetic that belongs to the consumer?]
- **FR-007**: System MUST bucket per-interval series at [NEEDS CLARIFICATION: interval length not specified]

### Key Entities *(include if feature involves data)*

- **[Entity 1]**: [What it represents, key attributes without implementation]
- **[Entity 2]**: [What it represents, relationships to other entities]

### Source Coverage *(include if the feature reads a tool artefact)*

<!--
  Constitution Principles I–III: every decoder declares what it accepts, what it cannot
  provide, and which real recordings prove it. State these as requirements, not design.
-->

- **Tool and versions**: [e.g., "Gatling 3.11.5 through 3.12.x"]
- **Artefact formats**: [e.g., "text simulation.log; binary simulation.log from 3.13.0"]
- **Version gate**: [what is refused, what decodes with a warning]
- **Not provided by this source** (declared through Capabilities): [e.g., "per-request bytes, group timings"]
- **Golden corpus**: [runs to record under testdata/corpus/<tool>/<version>/, each committed with that run's own tool report for tolerance checks — the report is captured at recording time or never (Principle III); say so explicitly if the tool version produces none]

## Success Criteria *(mandatory)*

<!--
  ACTION REQUIRED: Define measurable success criteria.
  These must be technology-agnostic and measurable.
-->

### Measurable Outcomes

- **SC-001**: [Correctness metric, e.g., "Every corpus file in the covered version range decodes to exactly the recorded record stream"]
- **SC-002**: [Fidelity metric, e.g., "Counts taken from the decoded records match the tool's own report exactly" — the verification suite computes them; this module does not]
- **SC-003**: [Resource metric, e.g., "A 1 GB log decodes with peak memory under 64 MiB"]
- **SC-004**: [Consumer metric, e.g., "galaxio report renders the run without a tool-specific code path"]

## Assumptions

<!--
  ACTION REQUIRED: The content in this section represents placeholders.
  Fill them out with the right assumptions based on reasonable defaults
  chosen when the feature description did not specify certain details.
-->

- [Assumption about inputs, e.g., "The run directory is complete; a truncated log is an error, not a partial result"]
- [Assumption about scope boundaries, e.g., "Binary simulation.log (3.13.0+) is out of scope for this feature"]
- [Assumption about data/environment, e.g., "Corpus runs can be recorded with the tool version pinned in the sample project"]
- [Dependency on existing work, e.g., "Requires the model types and Capabilities introduced by spec 001"]
