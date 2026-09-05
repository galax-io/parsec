# Feature Specification: A Canonical Model for Load-Test Results, and Requirements Stated Once

**Feature Branch**: `003-canonical-model`

**Created**: 2026-09-04

**Status**: Draft

**Input**: User description: "https://github.com/galax-io/parsec/milestone/3" — milestone v0.0.3 *A canonical model*: [#4](https://github.com/galax-io/parsec/issues/4) there is no tool-agnostic model for load-test results, and [#30](https://github.com/galax-io/parsec/issues/30) the probe's assertions are hand-written Gatling DSL rather than OpenNFR requirements.

## Clarifications

### Session 2026-09-04

- Q: Key Entities described a run as carrying its samples, while FR-017 required peak memory to stay independent of log size and SC-004 fixed a 32 MiB ceiling on a 1 GB log. The two collide on any log larger than memory. How is it resolved? → A: A run carries only what a run has a bounded number of — its identity, the tool and version, capabilities, the version warning, user events, aggregates and the errors that belong to no sample. Samples are delivered as a stream and are never materialised in full. This is the shape the decoder already has, so the ceiling stays reachable and Principle II is not weakened. A consumer that needs every sample at once collects them itself and owns that memory.
- Q: Principle I forbids a tool package to export a result type consumers depend on, and spec 002 deferred to this milestone the question of what then happens to the Gatling wire records. Do they stay exported? → A: Yes. They are the log's events, not results — nothing derives a count, a timing or a percentile from them — so the prohibition does not reach them, and the binary codec in v0.0.5 shares the same types. They are documented as wire records, the canonical types are documented as what consumers build on, and the Complexity Tracking row stays as a recorded reading of Principle I rather than as a deferral.
- Q: Issue #4 requires the model to accept pre-aggregated sources and names `Aggregate` among this milestone's types, while Principle VI forbids building a shape for which no source exists — the first is Locust, four milestones after the model becomes a stability promise. Does `Aggregate` ship here? → A: No. The type and the summary-only distinction are both deferred in full to v0.5.0, where there is a real artefact to design against. This milestone owes v0.5.0 only that the model never assumes every source records individual operations and that no existing type must change meaning to admit one (FR-010). The cost is accepted: admitting such a source after v0.1.0 is a breaking change for three downstream builds. Issue #4 is overridden and MUST be corrected before the implementation PR merges.
- Q: Should the probe's assertions be produced through the Galaxio Gatling DSL library rather than by a renderer written here? → A: Yes, and this repository writes no renderer. `gatling-picatinny` v1.26.0 added `OpenNfrAssertions.fromYaml`, which reads an OpenNFR `RequirementSet` and renders Gatling assertions, as the replacement for the deprecated `assertionFromYaml`; it tracks OpenNFR upstream v0.8.0. It already refuses a document totally and loudly when one predicate is unrenderable, renders `op: eq` and a failed count — which is what the probe's exact numbers need — and refuses `good`, which is why an expectation about successful requests is restated as a total plus a failure count. Two implementations of one translation in one organisation was the alternative, and it was rejected.
- Q: That renderer arrived in a line targeting Gatling 3.13.x, while the probe must run 3.11.5 and 3.12.0 — the versions whose text logs this project decodes. What is the scope of the probe story here? → A: Planning verifies first whether the current renderer runs under 3.11.5 and 3.12.0 — Gatling is a `Provided` dependency there, so the host project supplies it and it may. If it does, the story is delivered in this milestone. If it does not, the story moves whole to v0.0.5, where the binary codec brings Gatling 3.13 into the probe and the versions line up on their own. No substitute renderer is written in either case, and the canonical model ships regardless.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A run can be read without knowing which tool produced it (Priority: P1)

An engineer building a report is handed a finished load-test run. They read its requests, its groups and its virtual users, count them, and separate what succeeded from what failed — and at no point do they name Gatling, import anything shaped like Gatling, or write a branch that exists because this particular run came from Gatling. When the second tool lands, their code does not change.

**Why this priority**: this is the milestone. Three consumers are waiting on it — the `galaxio report` commands, the platform's ingestion and the live-metrics sidecar — and every one of them would otherwise be written against one tool's vocabulary and rewritten for the next. Everything else in this feature makes this one honest; nothing else can be built until this exists.

**Independent Test**: decode a recorded Gatling run into the canonical types and produce the run's request total and its success/failure split from those types alone, in code that imports no tool package. The counts must equal the ones the tool's own report states. Delivers value on its own: the decoder that shipped in v0.0.2 becomes consumable by something other than itself.

**Acceptance Scenarios**:

1. **Given** a recorded Gatling 3.11.5 run, **When** it is read into the canonical types, **Then** every request the log recorded is present as one sample, every group closing as one group sample, and every virtual-user start and end as one user event.
2. **Given** those canonical types, **When** a consumer counts the samples and splits them by outcome, **Then** the totals equal the ones that run's own report states, exactly.
3. **Given** a consumer written against the canonical types only, **When** it is compiled, **Then** it does not depend on any tool package, and nothing it reads is named after a tool.
4. **Given** a request that ran inside two nested groups, **When** it is read as a sample, **Then** it carries the full ordered path of enclosing group names, outermost first, and a request taken outside any group carries an empty path.
5. **Given** a run identifier, a simulation name, a start time and the tool and version that produced the run, **When** the run is read, **Then** all of them are available from the canonical types without reading a tool-specific header.
6. **Given** a log larger than available memory, **When** it is read into the canonical types, **Then** the samples arrive as a stream and peak memory does not grow with the size of the run.

---

### User Story 2 - What the source could not measure is declared, never filled in (Priority: P1)

Someone renders a report from a Gatling run and asks it for response codes. They are told, before anything is rendered, that this source does not record them — not given zeroes, not given a blank column that looks like every code was empty. They decide how to show the absence; the library does not decide for them by inventing a value.

**Why this priority**: it ships with User Story 1 or the model is dishonest from its first release. A model that fills an absent field with a zero produces reports that are wrong in a way nobody can see, and the whole reason for one shared model is that two tools can then be compared on equal terms. This is a MUST in the project constitution, not a preference.

**Independent Test**: ask a Gatling run's capabilities which fields the source provides, and confirm the answer names response code, scenario on a request, byte counts, connection and TLS timings and per-interval series as absent — and that nothing in the decoded run carries a zero standing in for any of them.

**Acceptance Scenarios**:

1. **Given** a run decoded from a Gatling text log, **When** its capabilities are inspected, **Then** every field the text `simulation.log` does not record is reported as absent, by name.
2. **Given** a field declared absent, **When** a consumer reads that field on any sample of the run, **Then** it is distinguishable from a recorded zero, an empty string and an average.
3. **Given** a consumer that renders a column for a field, **When** the field is declared absent for that source, **Then** the consumer can tell before rendering, from the run itself, rather than by discovering every value is empty.
4. **Given** two runs from sources that differ in what they record, **When** both are read, **Then** each declares its own absences and neither borrows the other's.

---

### User Story 3 - A failure can never be counted as a success (Priority: P2)

An engineer computes the 95th percentile of what succeeded. Adding a thousand failed requests to the same run does not move that number by a microsecond, and no consumer can reach a statistic that has quietly pooled the two. The distinction survives every step between the log and the report.

**Why this priority**: this is the correctness failure the ecosystem has already made, which is why the milestone that computes statistics calls it out by name. The model is where it is either made impossible or left available. It is P2 rather than P1 because the shape User Story 1 delivers is what makes it checkable, but the guarantee is not optional — a model that permits the mistake has not solved the problem it exists to solve.

**Independent Test**: take a decoded run, select the successful samples, then add every failed sample of that run back into the input and select again. The two selections must be the same multiset, and the statistics computed from them identical.

**Acceptance Scenarios**:

1. **Given** a run mixing successful and failed samples, **When** the successful ones are selected, **Then** the result is unchanged by how many failed samples the run contains.
2. **Given** any sample, **When** it is read, **Then** its outcome is recorded on the sample itself and is not inferred from the presence or absence of another field.
3. **Given** a failed sample, **When** it is read, **Then** the failure carries what the source recorded about it, and a sample that succeeded carries no failure at all.
4. **Given** a group sample, **When** it is read, **Then** it carries its own outcome, because a group can fail while every request inside it succeeded.

---

### User Story 4 - The probe's expectations are stated once, in a form no tool owns (Priority: P2)

A maintainer changes what the corpus probe must produce — say the number of requests expected to fail. They edit one requirements document and nothing else. Every Gatling the canary starts is held to exactly that document, and the assertions Gatling evaluates are the document's requirements rather than a second copy of them that someone typed into Scala. The document names no tool, so when the JMeter and k6 adapters land the same expectations can be held against their runs of the same probe.

**Why this priority**: the library's own position is that requirements belong in OpenNFR — it is why the assertion payload in a log is carried through unread. Today the probe contradicts that position in the repository's own test data, and the numbers it asserts are written twice: once in the Scala DSL, once implicitly in what the canary expects. It is P2 because the canonical model is what the milestone is for.

**Independent Test**: change one threshold in the requirements document, run the probe under every supported Gatling version without touching any other file, and confirm each run is held to the new number. Then make one requirement false and confirm the run fails naming it.

**Acceptance Scenarios**:

1. **Given** the requirements document, **When** the probe runs under each supported Gatling version, **Then** the run passes and the assertions Gatling evaluates are the document's requirements.
2. **Given** a threshold changed in the document alone, **When** the probe runs, **Then** it is held to the new threshold, and no Scala file was edited to achieve it.
3. **Given** a requirement deliberately made false, **When** the probe runs, **Then** the run fails and the failure names that requirement.
4. **Given** a document carrying one requirement that cannot be rendered as a Gatling assertion, **When** the probe starts, **Then** no assertion at all is produced and every reason is listed, so a run can never check fewer requirements than its document states.
5. **Given** the document, **When** it is validated, **Then** it validates against the published OpenNFR schema, and a document carrying an unknown field is rejected naming the field.
6. **Given** any supported Gatling version, **When** the probe runs under it, **Then** the same document renders to the same assertions with the same verdicts, so moving between supported versions changes nothing about what the probe is held to.

---

### Edge Cases

- **A run that recorded no requests at all**: a valid run with an empty sample stream, not an error. Its counts are zero and its capabilities are unchanged — the source records what it records whether or not anything happened.
- **A run that recorded no requests, and a source that records none**: different facts, and the model MUST NOT let them collapse into one. An empty sample stream says this run did nothing; capabilities say what this source can never record. A consumer distinguishes them from the run alone.
- **A field a source cannot provide**: absent, and distinguishable from a recorded zero. A Gatling request records no response code, so a report asking for one is told the source has none — never shown `0`.
- **A request whose end timestamp is the sentinel Gatling's own reader branches on**: its duration cannot be computed. It is carried as a sample whose duration is absent rather than as a negative or enormous number. Whether a 3.11.5 or 3.12.0 run can produce one is still unconfirmed, so the conversion MUST NOT assume the end is at or after the start.
- **An error record with no request it belongs to**: Gatling writes one for a request whose URL could not be built, which never reached the wire and produced no request record. It is a fact about the run, not about any sample, and MUST NOT be attached to an unrelated sample or silently dropped.
- **A group whose declared name contained a comma**: the model carries the name as the source recorded it, with the comma already replaced by a space. So does the requirements document, because the renderer addresses Gatling by recorded names and applies no substitution — the difference from the simulation's declared spelling is documented beside the requirement instead (FR-029).
- **A requirement naming a request position the run never recorded**: nothing is checked there and the run ends green. OpenNFR records this as a property of the target rather than something a document can state, and no Gatling assertion can state it either. The probe's document therefore names exact positions rather than quantifying over whatever the run happened to record, so an expectation that silently matched nothing would be a position that vanished from the probe — which the probe's own request total would catch.
- **A requirement the OpenNFR schema accepts but Gatling cannot assert**: the whole document is refused and every reason is listed, so the run produces no assertions rather than fewer than its document states. A run that silently checks nine of ten requirements is the failure mode this story exists to prevent.
- **A statement about successful requests**: not directly writable. An OpenNFR selector matches presence, never absence, so a fraction can name what failed and cannot name what did not. Such an expectation is restated as a total together with a failed count or share, which is the same statement about the same run.

## Requirements *(mandatory)*

### Functional Requirements

**The canonical model**

- **FR-001**: System MUST provide one set of result types that every source is decoded into, and those types MUST be what consumers depend on. No consumer may be required to name a tool to read a result.
- **FR-002**: Every sample MUST carry its own success or failure outcome, recorded on the sample rather than inferred from whether some other field is present.
- **FR-003**: A group MUST carry its own outcome independently of the samples it encloses, because a group can fail while every request inside it succeeded.
- **FR-004**: Selecting the successful samples of a run MUST yield the same result regardless of how many failed samples the run contains, and no entry point may return a statistic in which the two have been pooled.
- **FR-005**: A sample MUST carry the ordered path of enclosing group names, outermost first, and an empty path for a sample taken outside any group.
- **FR-006**: A value the source does not record MUST be reported as absent, and MUST be distinguishable from a recorded zero, an empty string, an average and a guess.
- **FR-007**: Each source MUST declare, for the run as a whole, which fields it cannot provide. A consumer MUST be able to read that declaration before rendering anything, rather than inferring it from finding every value empty.
- **FR-008**: Attribute names MUST be taken from the OpenNFR `loadtest.*` vocabulary wherever it names the quantity, so that a requirement and a result use one language. A name MUST NOT be minted where the vocabulary already has one.
- **FR-009**: A failed sample MUST be distinguishable by the presence of an error attribute, and a successful sample MUST NOT carry one, so that a requirement written as a fraction of failed requests binds to the model without a second vocabulary.
- **FR-010**: The model MUST NOT assume that every source records individual operations, and MUST NOT be shaped so that admitting a summary-only source later requires changing the meaning of an existing type. Representing such a source is milestone v0.5.0 and is deliberately not built here; what this milestone owes it is only that nothing in the model contradicts it.
- **FR-011**: A run MUST carry its identity — the run identifier, the simulation or scenario name, the run's start, and the tool and version that produced it — and a consumer MUST reach all of it without reading a tool-specific header.
- **FR-011a**: A run MUST carry only what does not grow with the length of the run: its identity, its capabilities, any version warning, and the opaque payloads the source wrote once per declared requirement. Everything that grows — samples, group traversals, virtual-user events and run-level errors alike — MUST be delivered as a stream beside it. A consumer that needs all of one kind at once collects it and owns that memory; nothing in the model may require a whole run to be resident in order to read it.
- **FR-012**: Every recorded time MUST be preserved exactly as the source recorded it, without rounding, re-basing against the run start, or timezone conversion.
- **FR-013**: Virtual-user starts and ends MUST be carried as events of the run. They bound the run span that every derived rate divides by, so they are load-bearing rather than decoration.
- **FR-014**: The model MUST carry what the source recorded that belongs to no individual sample: run-level errors, as stream items because a run may hold any number of them, and any opaque payload the source wrote that this module does not interpret, on the run itself because there is one per declared requirement.
- **FR-014a**: The records a decoder reads off the wire MAY stay reachable, and MUST be documented as the source format's own events rather than as a result. The canonical types MUST be documented as what a consumer builds on. A count, a timing or a percentile MUST NOT be derived from a wire record by this module or asked of one by a consumer.
- **FR-015**: Adopting the canonical types MUST NOT add a third-party dependency to a consumer's build. Three downstream builds inherit whatever this module depends on, along with its licence, its vulnerabilities and its upgrade schedule.

**Converting Gatling into it**

- **FR-016**: System MUST convert a decoded Gatling text `simulation.log` into the canonical types, and MUST declare through that run's capabilities what the source cannot provide.
- **FR-016a**: A warning the version gate raised MUST travel into the canonical run. A run decoded from a version no recording covers MUST stay identifiable as such after conversion, or the conversion would launder an unverified result into one that looks verified.
- **FR-017**: The conversion MUST be a stream: reading a run into the canonical types MUST NOT require the whole run to be resident, and peak memory MUST NOT grow with the size of the log.
- **FR-018**: The conversion MUST preserve counts exactly. The number of samples, group samples and user events produced MUST equal the number of corresponding records the decoder read, and the run's totals taken from the canonical types MUST equal the ones that run's own report states.
- **FR-019**: The capabilities a Gatling text run declares MUST name every field the format does not record: response code, the scenario a request ran under, request and response byte counts, connection, DNS and TLS timings, per-request throughput, the requirements the assertion payload encodes, per-interval series, and the identity of the virtual user that made a request.
- **FR-020**: A request whose end is the sentinel value Gatling's own reader branches on MUST yield a sample whose duration is absent, never a negative or nonsensical one.
- **FR-021**: A Gatling error record that belongs to no request MUST be carried as a run-level error, not attached to an unrelated sample and not dropped.

**Requirements stated once**

- **FR-022**: The corpus probe's expectations MUST be stated once, in an OpenNFR `RequirementSet` document, and that document MUST be what the Gatling run is held to. Changing an expectation MUST NOT require editing Scala.
- **FR-023**: The document MUST be rendered into Gatling assertions by the existing OpenNFR renderer in the Galaxio Gatling DSL library. No renderer MUST be written in this repository: one already exists, is maintained beside the format, and duplicating it would put two implementations of one translation in one organisation.
- **FR-024**: A requirement the renderer cannot express MUST produce no assertions at all, with every reason listed. A run MUST NOT check fewer requirements than its document states. This is what the existing renderer already does; the requirement is recorded so that a replacement would have to keep it.
- **FR-025**: An expectation about successful requests MUST be stated as a total together with a failed count or share. A selector matches presence and never absence, so a fraction can name what failed and cannot name what did not; the restatement says the same thing about the same run.
- **FR-026**: The document MUST be validated against the published OpenNFR schema on every change, and a document carrying an unknown field MUST be rejected naming the field.
- **FR-027**: A requirement deliberately made false MUST fail the probe run, naming that requirement.
- **FR-028**: Every Gatling version the canary starts MUST be held to the same document, and the recording procedure for any future corpus entry MUST use it. The two recordings already made are not re-recorded and keep the assertions their runs evaluated.
- **FR-029**: Where a group's recorded name differs from the name it was declared with — Gatling replaces a comma with a space as it writes — the document MUST carry the recorded spelling, and the difference MUST be documented beside it. The renderer addresses Gatling by recorded names and applies no substitution, so an author writing the declared spelling would produce a requirement that silently matches nothing.

### Key Entities *(include if feature involves data)*

- **Sample**: one recorded operation — a request, in every tool surveyed so far. Carries the path of enclosing groups, the operation's name, when it started, how long it took, whether it succeeded, and what the source recorded about the failure when it did not. Optionally carries what only some sources record: a response code, byte counts, the scenario it ran under.
- **User event**: one virtual user starting or ending a scenario, with the scenario's name and the time it happened. Bounds the run span that every derived rate divides by.
- **Group sample**: one traversal of a group closing. Carries the group's path, its timing, the cumulated duration of the operations it enclosed, and its own outcome — which is not the conjunction of the outcomes inside it.
- **Run**: one execution, described by everything that does not grow with its length — its identity, the tool and version that produced it, the capabilities of the source, any warning the version gate raised, and the opaque payloads the source wrote once per declared requirement. It does **not** hold its samples, its group traversals, its virtual-user events or its errors: all four are delivered as a stream beside it, because a run large enough to matter is larger than the memory available to hold it.
- **Capabilities**: the source's own statement of what it cannot provide, read before anything is rendered. The line between measured and missing, and the reason a report can compare two tools without pretending they measured the same things.
- **Requirement document**: the probe's expectations as an OpenNFR `RequirementSet` — the single place they are written, rendered into Gatling assertions by the library that already does that translation, and the thing against which any later tool's run of the same probe can be held.

### Source Coverage *(include if the feature reads a tool artefact)*

- **Tool and versions**: Gatling 3.11.5 and 3.12.0, through the decoder that shipped in v0.0.2. This feature reads no artefact itself; it converts what that decoder produced, so the supported range is that decoder's range and this feature does not widen it.
- **Artefact formats**: the tab-separated text `simulation.log`, by way of the existing decoder. No new format is read.
- **Version gate**: unchanged and not re-implemented. The gate belongs to the decoder; a run that the decoder refused never reaches the conversion, and a run it accepted with a warning carries that warning into the canonical run.
- **Not provided by this source** (declared through Capabilities, never filled in): response codes, the scenario a request ran under, request and response body sizes, connection, DNS and TLS timings, per-request throughput, the requirements the assertion payload encodes, per-interval series, and the identity of the virtual user that made a request.
- **Golden corpus**: the two runs already recorded under `testdata/corpus/gatling/3.11.5/` and `testdata/corpus/gatling/3.12.0/`, each with the two statistics files its own Gatling generated. No new recording is required and none can be added to these two — the constitution's capture-at-recording-time rule has already been met for them. The probe project those runs were made from gains its requirements document, and the recording procedure changes to render the assertions from it, so a future recording of a new version is held to the same document. Both supported versions render the document identically, so the procedure is the same for either (research.md R1).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A consumer can read a complete Gatling run — its requests, groups, virtual users, run identity and run-level errors — through the canonical types alone, in code that imports no tool package.
- **SC-002**: For each of the two recorded runs, every count taken from the canonical types equals the count the decoder produces from the same log, and equals what that run's own report states: the request total and its success/failure split, for the run and for each request name and each group. A difference of one fails.
- **SC-003**: Selecting the successful samples of a run returns an identical multiset whether or not the run's failed samples are present, for every corpus run and for generated runs mixing the two in any proportion.
- **SC-004**: Reading a 1 GB log into the canonical types completes within the peak-memory budget the decoder already meets — under 32 MiB — and that figure does not change when the log is made ten times larger.
- **SC-005**: Every field named as absent for Gatling text has an automated check confirming that the run declares it absent and that no sample carries a substituted value for it.
- **SC-006**: A run that recorded nothing is distinguishable from a source that records nothing, from the run alone, without scanning its samples.
- **SC-007**: Changing one threshold in the probe's requirements document, with no other file edited, changes what every Gatling version the canary starts is held to.
- **SC-008**: A requirement deliberately made false fails the probe run under every supported Gatling version, and the failure names the requirement.
- **SC-009**: The probe's requirements document validates against the published OpenNFR schema on every change, and a document with an unknown field is rejected naming the field. A document carrying one unrenderable requirement yields no assertions at all and names every reason.
- **SC-010**: Automated tests exercise at least 90% of the conversion code and at least 80% of the project overall, measured on every change.
- **SC-011**: The requirement written against the group whose declared name contains a comma resolves to the request Gatling recorded, under every version the canary starts, and the document records beside it why its spelling differs from the simulation's.

## Assumptions

- **The Gatling wire records stay exported, and the model is what consumers are pointed at.** Settled in clarification. The v0.0.2 plan recorded the absence of a `model/` conversion as a Complexity Tracking row and said this milestone resolves it. It is resolved by adding the conversion, not by hiding the records: they are the log's own events, the binary codec in v0.0.5 shares them, and a caller who needs to see exactly what a log contained has no other route. What changes is which types are documented as the thing to build on. Making them unexported was rejected because it would delete the only honest view of an undocumented format for the sake of a rule aimed at result types, and because the binary codec would have to re-export them a milestone later. Deprecating them now and removing them at v0.1.0 was rejected for the same reason: it promises to delete what v0.0.5 will need.
- **Run-level errors and the assertion payload live on the run.** Issue #4's type list names `Sample`, `UserEvent`, `GroupSample`, `Aggregate`, `Run` and `Capabilities`, and none of them is a home for a Gatling `ERROR` record that belongs to no request, or for the opaque assertion payload the decoder carries through unread. Both are facts about the run, so the run carries them. Dropping them was rejected: the decoder went to some trouble to produce them, and a request whose URL could not be built is visible in no other way. Attaching them to a neighbouring sample was rejected as an invention.
- **`Aggregate` is not built here, and this overrides issue #4.** Settled in clarification. Issue #4 requires that "the model MUST accept pre-aggregated sources without pretending they carry raw samples" and names `Aggregate` among the types this milestone adds. Both are deferred in full to milestone v0.5.0, where Locust is the first source that publishes summaries and no samples. Principle VI decides it: no such source is decoded before then, and a shape designed against no real artefact is the speculative abstraction that principle forbids — v0.5.0's own description says the model "has never been exercised against" one, which is an argument for designing it when there is finally something to design against. The cost is accepted and named rather than argued away: the model becomes a stability promise at v0.1.0, four milestones before Locust, so admitting a summary-only source afterwards is a breaking change for three downstream builds and takes a new MINOR version. What this milestone still owes v0.5.0 is FR-010 — nothing in the model may assume that every source records individual operations, and no existing type may have to change meaning to admit one. **Issue #4 MUST be corrected before the implementation PR merges**, or its tracked requirements and the shipped model disagree on the record.
- **Absence is declared twice, at two different granularities, and the two are not redundant.** `Capabilities` says what the *source* never records — a Gatling request has no response code, for any run, ever. An optional field on a sample says what *this* sample lacks — a source that usually records a response code did not record one here. A model with only the first cannot represent a partially populated field; one with only the second forces every consumer to scan every sample to learn what the source could never provide.
- **The renderer already exists, in the Galaxio Gatling DSL library, and this feature writes none.** Settled in clarification, correcting an earlier reading of this specification. OpenNFR's own repository has no implementation — its README says nothing reads a document yet — but `gatling-picatinny` does: `OpenNfrAssertions.fromYaml` reads an OpenNFR `RequirementSet` and renders Gatling assertions, added in its v1.26.0 as the replacement for the deprecated `assertionFromYaml`, and tracking OpenNFR upstream v0.8.0. It already refuses a document totally and loudly when one predicate is unrenderable, which is FR-024. It refuses `good`, which is why FR-025 restates successful counts. It renders `op: eq` and a failed count, which is what the probe's exact numbers need. Writing a second renderer here was rejected: one organisation would then carry two implementations of one translation, and ours would be the poorer of them. The Go-side dependency on opennfr#119 is on the **vocabulary**, which is published and settled, not on code.
- **A statement about successful requests is restated as a total and a failure count.** An OpenNFR selector matches presence and never absence, which is why a fraction can name what failed and cannot name what did not, and the renderer refuses `good` for exactly that reason. The probe currently asserts 18 successful and 18 failed requests; as a document that becomes a total of 36 with 18 failed, and a per-request "100% successful" becomes "0% failed". The same statement about the same run, in the vocabulary the format has. A translation decision, not a weakening.
- **User Story 4 is unconditional: the renderer runs under both supported Gatling versions.** The clarification session left this for planning to settle, and planning settled it by running it — see [research.md](research.md) R1. `gatling-picatinny` 1.27.0's `OpenNfrAssertions.fromYaml` renders the probe's document under Gatling 3.11.5 and 3.12.0 alike, producing the nine assertions the hand-written DSL produces, with the same verdicts. The concern that its assertion model belonged only to the 3.13 line was wrong: `gatling-shared-model` carries that model for 3.11.5 (0.0.6) and 3.12.0 (0.0.7) as well, and the assertion API's class inventory and signatures are identical across all three. The fallback of moving this story to v0.0.5 is not taken.
- **The comma rule is not handled for the author, and issue #30 is overridden on that point.** Issue #30 requires the translation to apply the recorded-name rule "so an author never has to know it". The renderer does not: it addresses Gatling by recorded group and request names and applies no substitution. So the probe's document carries `inner  with comma` — the recorded spelling, two spaces — with a note beside it explaining why, and FR-029 says so. Renaming the probe's group to avoid the comma was rejected: spec 002 requires a corpus run to contain a comma in a group name precisely to prove that the split on comma is lossless. **Nor is there anything to fix upstream**: OpenNFR itself defines a `loadtest.group.name` element as "a literal recorded name" (README § the attribute table, and the same words in `GLOSSARY.md`), so a document carrying the recorded spelling is what the format specifies rather than a workaround for a renderer that lacks a feature. Issue #30's requirement rests on a misreading of the format and is dropped, not deferred.
- **The probe's expectations are the ones already in the simulation.** Six virtual users, thirty-six recorded requests, eighteen of them failures, the three failing positions failing entirely and the three succeeding positions succeeding entirely, and a ceiling on the slowest response. Nothing is added or relaxed while moving them into a document; the point of the move is where they are written, not what they say.
- **The two recorded corpus runs are not re-recorded, so User Story 4 reaches only the canary and future recordings.** They were captured with the reports their own Gatling generated, which is the half that cannot be recovered, and the assertions Gatling evaluated at recording time are already frozen in their logs. What the document changes is what the canary holds a fresh run to on every change, and how any future recording derives its assertions. Re-recording so that the two runs' assertion payloads match a newly rendered block was rejected: it would discard evidence that cannot be recreated, to gain nothing the payload is even read for — the decoder carries it through unread.
- **The requirements document is validated in this repository against the published schema.** OpenNFR's gate validates OpenNFR's own corpus; a document living here is checked here, against the schema file that repository publishes, so a schema change this document violates is caught by this project's own pipeline rather than by nobody. The renderer tracks OpenNFR upstream v0.8.0 and states that this surface sits outside its binary-compatibility guarantee, so the document is written to what the renderer accepts, and a divergence between that and the current published schema is a finding for planning rather than something to discover at run time.
- **Statistics remain out of scope, and after 2026-09-05 permanently so.** Counts taken to compare against a report are the verification suite's, as they were in v0.0.2. Percentiles, ranges and series are computed in `galaxio-cli` rather than in any later milestone of this library; this model is what they consume, and it computes nothing. Constitution v2.0.0 carries the rule, and milestone v0.0.7 is re-scoped to the primitives that arithmetic needs — an addressable request position and the bounds of a run.
- **No second tool is decoded here.** JMeter, k6, Locust and Yandex.Tank are their own milestones. The model is designed so that adding one is an adapter and a capabilities declaration rather than a change to the model, and the only evidence for that claim available in this milestone is that a summary-only run is representable.
- **The canary and corpus verification keep proving the decoder as well as the conversion.** The existing checks read wire records and hold them to each run's own report; they stay. The conversion adds a second layer held to the same numbers, so a conversion bug and a decoder bug remain distinguishable.
