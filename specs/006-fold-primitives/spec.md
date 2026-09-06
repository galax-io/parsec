# Feature Specification: The Primitives a Consumer Folds

**Feature Branch**: `006-fold-primitives`

**Created**: 2026-09-06

**Status**: Draft

**Input**: User description: "https://github.com/galax-io/parsec/milestone/7" — milestone v0.0.6 *The primitives a consumer folds*: "S2 — Decoded samples are a stream of events, and a consumer that wants any number must first decide for itself what a request position is and where the run begins and ends. Those are definitions, not arithmetic: two consumers that re-derive them differently disagree about what is being measured, not merely how. The statistics themselves are galaxio-cli's (constitution v2.0.0), and galaxio-cli#51 is waiting on these definitions. Also carries three small fixes to what v0.0.5 shipped." Issues: [#8](https://github.com/galax-io/parsec/issues/8) the primitives; [#56](https://github.com/galax-io/parsec/issues/56), [#57](https://github.com/galax-io/parsec/issues/57) and [#55](https://github.com/galax-io/parsec/issues/55) the three fixes. [#62](https://github.com/galax-io/parsec/issues/62) is in the milestone too and already landed on `main`.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Two consumers bucket the same run by the same key (Priority: P1)

An engineer writing the run summary in galaxio-cli and an engineer writing the live aggregator in the comet sidecar both need to group what a run recorded by *what was requested*: the ordered path of enclosing groups and the request's name. Today the stream hands each of them a path and a name and leaves the rest to them, so each invents a key — one joins the path with a comma, the other with a slash, a third hashes the pair — and two reports of one run can disagree about which rows exist. With this feature every sample and every group traversal carries a position that is one value: comparable, usable directly as a map key, and the same value for the same place in the run whoever asks. Neither consumer spells anything.

**Why this priority**: it is the milestone's reason to exist. A count is only comparable across consumers if both counted the same thing, and "the same thing" is the position. galaxio-cli#51 cannot start until this exists. It is also the cheapest place a divergence can be prevented: one type, instead of one convention per consumer.

**Independent Test**: fold one corpus run twice, in two pieces of code written without reference to each other, each keyed by position; the two sets of keys and the counts under them are identical. Delivers value on its own: any consumer can bucket a run without deciding how.

**Acceptance Scenarios**:

1. **Given** a corpus run, **When** two independently written folds bucket its samples by position, **Then** they produce the same set of positions and the same count under each, having agreed on nothing but the types.
2. **Given** a request taken outside any group and a request of the same name inside one, **When** both are folded, **Then** they land under different positions.
3. **Given** a group traversal whose path is `a` then `b`, and a request named `b` inside group `a`, **When** both are folded, **Then** they land under different positions: a group traversal and a request never share one.
4. **Given** two places one consumer's chosen separator would make read the same — a single group named `a,b` beside two nested groups `a` then `b` — **When** they are folded, **Then** they are different positions.
5. **Given** a position taken from a sample, **When** the reader has advanced to the end of the run, **Then** the position still equals itself and still names the path and the name it was taken from, without the consumer having copied anything.
6. **Given** a position, **When** a report needs to print a row, **Then** the consumer can recover the ordered path and the name from it exactly.

---

### User Story 2 - Two consumers agree where the run begins and ends (Priority: P1)

Every rate a report prints — requests per second for the run, for a request, for a group — divides by the run's span, and the span is not the header's start to the last request. Gatling's own report bounds it by the earliest and latest of the things that count: request and group starts and virtual-user starts on one side; request and group ends and any virtual-user event on the other. A consumer that forgets the user events, or counts the header's start, or the errors, reports a different throughput for the same run and is internally consistent while doing so. With this feature the bounds are a definition a consumer extends one item at a time over the stream, and the answer is the one the tool's own report used.

**Why this priority**: the span is the definition most easily got subtly wrong — it is bounded by user events, not only by requests — and it feeds every rate. Two of the three consumers divide by it.

**Independent Test**: fold every corpus run through the bounds and reproduce the mean request rate each run's own Gatling account printed, exactly, using the rounding the report uses. Delivers value on its own: any consumer can divide by the right span.

**Acceptance Scenarios**:

1. **Given** a corpus run, **When** its bounds are extended item by item over the whole stream, **Then** they equal the span that run's own report used, shown by reproducing the report's mean request rate exactly.
2. **Given** a run whose first event is a virtual user starting before any request begins, **When** the bounds are folded, **Then** the run starts at that user event, not at the first request.
3. **Given** a run whose last event is a virtual user ending after the last request completed, **When** the bounds are folded, **Then** the run ends at that user event.
4. **Given** a sample whose end the source could not record, **When** it is folded, **Then** its start may set where the run begins and it never extends where the run ends.
5. **Given** the same items in a different order, **When** they are folded, **Then** the bounds are the same.
6. **Given** a run that yields no sample, no group traversal and no user event, **When** its bounds are read, **Then** they are absent — not zero, and not the run's recorded start.
7. **Given** the run's recorded start, a run-level error and an assertion payload, **When** they are folded, **Then** none of them moves either bound.

---

### User Story 3 - A consumer folds a run in one pass and this library computes nothing (Priority: P2)

The galaxio-cli engineer writes the summary command: one loop over the stream, keyed by position, extending the bounds, accumulating counts and timings of their own. Nothing in this library retained an item, and nothing in it handed them a number. When a reviewer asks what parsec computes, the answer is verifiable: it exports no count, no mean, no minimum, no maximum, no standard deviation, no percentile, no range and no series. What it exports is what a failure is, what a position is, and where a run begins and ends — and the fold in the package's own documentation shows the consumer's loop using exactly that.

**Why this priority**: P2 because it is the composition of the first two stories rather than a third capability. It is the story that proves the boundary the constitution draws is workable from the consumer's side, and that the boundary holds.

**Independent Test**: an example in the package documentation, run by the test suite, folds a corpus run to per-position counts and a mean rate through the primitives alone; an automated check over the exported surface finds no statistic.

**Acceptance Scenarios**:

1. **Given** a corpus run, **When** the documented example folds it, **Then** it produces per-position success and failure counts and the run's mean request rate, writing no definition of a position or a span of its own, and the counts match the run's own report.
2. **Given** the module's exported surface, **When** it is checked, **Then** no exported identifier returns a count, a mean, an extreme of any measured value, a standard deviation, a percentile, a range or a series, and the check fails if one is added.
3. **Given** a group traversal, **When** a consumer reads its timings, **Then** the wall clock of the traversal and the cumulated response time of what it enclosed are two distinct values, and the consumer must name which one it wants.
4. **Given** a 1 GB log, **When** it is folded through the primitives, **Then** peak memory stays within the budget the codecs already meet: the primitives retain nothing.

---

### User Story 4 - Two logs of one run read the same, even where the input is bad (Priority: P2)

A consumer moves an archived text log and a fresh binary log of the same simulation through the same report and expects one behaviour. v0.0.5 promised that. On three inputs the two codecs disagree: a negative time the binary codec reports as absent and the text codec refuses outright; a negative cumulated group time the binary codec leaves unset and the text codec refuses; a record with no groups, which the binary codec always hands out as an empty path and the text codec hands out as *nothing* for the first such record and as an empty path for every later one. After this feature both codecs give one answer to each — the answer spec 005 already fixed: a value that cannot be resolved is reported absent, never wrapped, guessed or allowed to end a ten-million-record read — and a test drives both codecs with the same inputs so a fourth divergence cannot land unnoticed.

**Why this priority**: it is a promise v0.0.5 shipped and broke in the same release, and the divergence is this repository's own: the binary codec was changed to follow the spec during review and the text codec was not.

**Independent Test**: hand each of the three inputs to both codecs and compare outcomes; then check that every golden record stream in the corpus is byte-identical before and after.

**Acceptance Scenarios**:

1. **Given** a text log and a binary log each carrying a negative time value on an event, **When** both are read, **Then** both deliver the record with that time absent, and neither refuses the read.
2. **Given** a text log and a binary log each carrying a negative cumulated response time on a group, **When** both are read, **Then** both deliver the group with that duration unset.
3. **Given** a text log whose first event record has no groups, **When** it is read, **Then** that record's path is empty exactly as every later group-less record's is, and a consumer asking "is there a path" and "is the path empty" gets the same answer for every record.
4. **Given** a time either codec reported absent, **When** it reaches the canonical model, **Then** it is absent there too — not a plausible-looking instant in the distant past — and it never participates in the run's bounds.
5. **Given** the existing corpus, **When** every run is re-read after the change, **Then** every golden stream is unchanged.
6. **Given** a future change to one codec's handling of a malformed value, **When** the test suite runs, **Then** the agreement test fails until the other codec matches.

---

### User Story 5 - A valid run with a large assertion suite decodes (Priority: P3)

A simulation author asserts over many endpoints — two thousand assertions is unusual but not wrong — and their run record is refused as malformed, naming a count of 2000 as if it were corruption. The number of groups a request can nest inside, the number of scenarios a run declares and the number of assertions it carries are three different quantities with three different natural bounds, and one ceiling written for the first is silently bounding the other two. After this feature each count has its own ceiling, named for what it bounds and justified beside it, and a corrupt count is still stopped before it sizes anything.

**Why this priority**: P3 because the run that hits it is rare. It is not optional: a decoder limit reported to a user as a damaged file is the wrong error, and raising the nesting limit later would silently move two ceilings that were never meant to move with it.

**Independent Test**: decode a run record declaring 2000 assertions and read all 2000 back; decode a request record claiming a nesting depth of 1<<20 and confirm the read fails naming the offset.

**Acceptance Scenarios**:

1. **Given** a run record declaring 2000 assertions, **When** it is read, **Then** the read succeeds and all 2000 payloads are delivered.
2. **Given** a request record claiming a group depth of 1<<20, **When** it is read, **Then** the read fails naming the byte offset, exactly as today.
3. **Given** a run record claiming a scenario count or an assertion count that could only be corruption, **When** it is read, **Then** the read fails naming the offset without allocating for the claimed count.

---

### User Story 6 - The README says what the release does (Priority: P3)

A consumer evaluating parsec reads the README, which says a binary `simulation.log` — every Gatling since 3.13.0 — is refused because no codec reads it yet, and does not list a binary package at all. They conclude the library cannot do the one thing v0.0.5 was released for, while the package documentation in the same tree says it can. A maintainer checking why the six-byte binary detection rule is what it is follows two code comments to a sample directory v0.0.5 deleted. After this feature the README and the package documentation agree, and the rule's provenance points at a recording that exists.

**Why this priority**: P3 because nothing computes wrongly. But the README is the first thing a consumer reads, and it currently denies the headline feature.

**Independent Test**: read the README and the package documentation side by side and confirm they name the same formats and ranges; search the source tree for the deleted sample path and find nothing.

**Acceptance Scenarios**:

1. **Given** the README, **When** a consumer reads what the module reads, **Then** it describes the binary format as readable over 3.13.1 through 3.15.1 and lists the binary codec beside the text one, agreeing with the package documentation.
2. **Given** the comments that justify the binary detection rule, **When** a maintainer follows them, **Then** they reach the 3.15.1 recording, whose log opens with the bytes the rule checks.
3. **Given** the source tree, **When** it is searched for the deleted sample directory, **Then** no source file names it.

---

### Edge Cases

- **A name containing whatever a consumer might have used as a separator** — a comma, a slash, a tab, a NUL. Positions stay distinct for distinct paths; the identity is the value, not a spelling.
- **An empty request name, or an empty group name.** A position is still well-formed and still distinct from a neighbour whose name is not empty.
- **A request and a group traversal at what reads as the same place.** Distinct positions; a consumer can tell which kind it holds.
- **A sample whose start the source could not resolve.** It does not set where the run begins; the model shows the start as absent rather than as a time it did not record.
- **A sample or group whose end is absent.** Its start counts, its end does not.
- **A group traversal's end is its wall clock, not its cumulated time.** Bounds are extended by the wall-clock end; the cumulated response time never moves a bound.
- **A user END that arrives before any user START, or events out of time order.** Bounds are the earliest and latest of what counts, in whatever order it arrives.
- **A run of user events and nothing else.** Bounds come from the user events; there is no request to bound it.
- **A run with nothing that counts** — only a header, errors and payloads. Bounds are absent.
- **A run's recorded start earlier than its first event.** Not a bound; the report does not use it and neither does this.
- **Reading a log in chunks.** The item stream is already identical to a whole-file read, so positions and bounds are too.
- **A version below the supported range, or an unknown newer one.** Unchanged by this feature: the gate refuses the first and warns on the second before any item exists to fold.
- **Malformed input.** Unchanged in kind — the read stops at the first byte it cannot decode, naming its offset — and now identical across the two codecs for every value both can express (User Story 4).
- **A count in a binary run record that is large but honest.** Decodes (User Story 5); a count that could only be corruption is still refused before it sizes an allocation.

## Requirements *(mandatory)*

### Functional Requirements

**Position**

- **FR-001**: Every sample and every group traversal MUST be addressable by a position: one value naming the ordered path of enclosing groups, outermost first, and — for a sample — the sample's name. This is the definition of "what was requested" that every consumer buckets by.
- **FR-002**: Two positions MUST compare equal exactly when they name the same path, the same name and the same kind of thing, so a map keyed by position in one consumer has the same keys as a map keyed by position in another. A consumer MUST NOT need to spell a position to use it as a key.
- **FR-003**: A position MUST be unambiguous for any names a source can record, including names containing any character a consumer might have chosen as a separator: two different paths MUST NOT share a position.
- **FR-004**: A sample's position and a group traversal's position MUST never compare equal, even where the group's path reads as the sample's path plus its name.
- **FR-005**: The ordered path and the name a position was made from MUST be recoverable from it exactly, so a report can print a row without keeping the sample it came from.
- **FR-006**: A position taken from an item MUST remain valid and equal to itself after the reader advances, without the consumer copying anything, so it can be kept as a map key for the whole run. The path on a sample is backed by storage the reader reuses; the position must not be.
- **FR-007**: Any textual rendering of a position this library offers is for display only and MUST NOT be its identity; two consumers that render differently still bucket identically.

**Run bounds**

- **FR-008**: The bounds of a run — the instant it begins and the instant it ends — MUST be derivable from the stream by extending them one item at a time in one pass, retaining nothing but the bounds themselves.
- **FR-009**: The start MUST be the earliest of every sample start, every group traversal start and every virtual-user START. The end MUST be the latest of every sample end, every group traversal end and every virtual-user event, START or END. The run's recorded start, run-level errors and assertion payloads MUST NOT participate. This is the span Gatling's own report uses (spec 002, FR-021c), adopted as the model's definition.
- **FR-010**: A sample or group traversal whose end is absent MUST contribute its start and MUST NOT extend the end. A group traversal's end is its start plus its wall-clock duration; its cumulated response time MUST NOT move a bound.
- **FR-011**: An instant the source could not resolve MUST NOT participate in the bounds.
- **FR-012**: The bounds MUST be the same for any order of the same items.
- **FR-013**: A run that yields nothing that counts MUST report its bounds as absent — never as zero, and never as the run's recorded start.
- **FR-014**: The bounds MUST be absolute instants exactly as the source recorded them, not re-based or rounded; how a span is rounded for a rate is the consumer's, and the verification suite applies the tool's rounding when it checks.
- **FR-015**: For every corpus run, the bounds folded through this primitive MUST reproduce the span-derived figure that run's own kept account carries — the mean request rate in the statistics files of the text runs, the mean throughput in the console summary of the binary runs — exactly, computed by the verification suite with the rounding the tool uses.

**The fold and the boundary**

- **FR-016**: A consumer MUST be able to fold a run in one pass over the existing stream — obtaining each item's position and extending the bounds per item — with this library retaining no item and allocating nothing that grows with the number of items.
- **FR-017**: This library MUST NOT export a count, a mean, a minimum or maximum of any measured value, a standard deviation, a percentile, a range or a per-interval series. The earliest and latest instants that bound a run are the definition of where a run begins and ends, which the constitution names as this module's; they are the only extremes it takes, and they are taken over when things happened, never over how long they took.
- **FR-018**: The absence of any statistic from the exported surface MUST be checked automatically, so that adding one is a deliberate act that fails a check rather than a quiet drift.
- **FR-019**: The two group durations MUST stay distinguishable: the wall clock of a traversal and the cumulated response time of what it enclosed are different quantities, and a consumer holding one MUST be able to tell which. Nothing in this feature may merge them or derive one from the other.
- **FR-020**: What a failure is MUST remain as v0.0.3 defined it — the outcome the source recorded, with a failure present if and only if the outcome is failure — and the primitives MUST NOT introduce a second way to decide it.
- **FR-021**: The package documentation MUST carry an example, run by the test suite, that folds a corpus run to per-position success and failure counts and a mean request rate through the primitives alone, so the consumer's loop is shown rather than described.

**One answer to malformed input (#56)**

- **FR-022**: For every malformed value both log formats can express, the two codecs MUST produce the same outcome — the same kind of error, or the same absent field. Specifically: a negative time value on an event MUST be reported absent by both, not refused; a negative cumulated response time on a group MUST be left unset by both; a record with no groups MUST carry an empty path from both, for the first such record as for every later one.
- **FR-023**: An instant a codec reported absent MUST be absent in the canonical model as well, distinguishable from a recorded instant and never rendered as a plausible time the source did not record.
- **FR-024**: A test MUST drive both codecs with equivalent inputs and assert the outcomes match, so that a change to one codec's handling of a malformed value fails until the other matches.
- **FR-025**: Every golden record stream in the corpus MUST be unchanged by this work.

**One ceiling per count (#57)**

- **FR-026**: Each count read from an untrusted file that sizes an allocation — the nesting depth of a record, the scenarios a run declares, the assertions it carries — MUST have its own ceiling, named for what it bounds and justified beside it.
- **FR-027**: Each ceiling MUST admit any count a simulation author could honestly write — at least 2000 assertions — and MUST still stop a count that could only be corruption before it sizes an allocation.
- **FR-028**: Raising one ceiling MUST NOT move another.

**Documentation that matches the release (#55)**

- **FR-029**: The README MUST describe the binary format as readable over 3.13.1 through 3.15.1, list the binary codec beside the text one, and agree with the package documentation on what the module reads.
- **FR-030**: The provenance cited for the binary detection rule MUST be a recording that exists — the 3.15.1 recording, whose log opens with the bytes the rule checks — and no source file may name the deleted sample directory.
- **FR-031**: The detection rule itself MUST NOT change.

**Evidence**

- **FR-032**: The verification suite MUST fold every corpus run — both text runs and all three binary runs — through the primitives and hold the resulting counts to that run's own report exactly, per position and for the run, and the bounds to the span the report used. The suite computes; the library does not.
- **FR-033**: The verification suite MUST contain two folds written independently — one keyed and bounded through the primitives, one through the tally it already keeps by hand — and they MUST agree on every corpus run.
- **FR-034**: Any hand-written input used to exercise a case no recording produces — the three malformed values, a run record with 2000 assertions, a claimed depth of 1<<20 — MUST be named as a fixture, not as corpus.
- **FR-035**: Every observable change to a released behaviour — a refusal that becomes an absent field, a ceiling that moves, any exported identifier added or changed — MUST be recorded in the changelog in the same change.

### Key Entities *(include if feature involves data)*

- **Position**: where in a run something was recorded — the ordered path of enclosing groups and, for a sample, its name. One comparable value, the same for the same place whoever derives it, distinct between a sample and a group traversal, and valid after the reader has moved on. The definition every consumer buckets by.
- **Bounds**: the instant a run begins and the instant it ends, as the tool's own report bounds it: extended one item at a time by sample, group and user events, and absent for a run in which nothing counted. Distinct from the run's recorded start, which is the header's and does not participate.
- **Fold**: the consumer's single pass over the stream, obtaining a position and extending the bounds per item and accumulating whatever arithmetic it owns. Not a type this library ships; the loop the documented example shows.
- **Group durations**: two quantities on one traversal — wall clock across it, pauses included; and the cumulated response time of what it enclosed — never interchangeable, and only the first bounds a run.
- **Ceiling**: a bound on a count read from an untrusted file, existing so that a corrupt count cannot size an allocation. One per count, named for what it bounds.
- **Fixture**: a hand-written input exercising a case no recording produces. Named as such, never as corpus.

### Source Coverage *(include if the feature reads a tool artefact)*

- **Tool and versions**: Gatling 3.11.5 and 3.12.0 through the text codec, 3.13.1 through 3.15.1 through the binary codec. This feature reads no new artefact and widens no range; the primitives fold what the codecs already produce, and the two codec fixes correct how those codecs read what they already accept.
- **Artefact formats**: the text and binary `simulation.log`, through the existing codecs. No new format.
- **Version gate**: unchanged and not re-implemented. A run the gate refuses yields nothing to fold; a run it warns on folds with the warning carried on the run.
- **Not provided by this source** (declared through Capabilities): unchanged. A position and the bounds are derived from fields every source provides — path, name, start, end, user events — so no capability is added or withdrawn.
- **Golden corpus**: the five recorded runs already under `testdata/corpus/gatling/`. No new recording is needed and none is added; the three malformed inputs and the large-count cases are fixtures.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For 100% of corpus runs, two independently written folds produce identical sets of positions and identical bounds, with zero differences.
- **SC-002**: For every corpus run, the success and failure counts folded by position match that run's own report exactly — per request, per group and for the run — and the report's span-derived rate is reproduced exactly from the folded bounds. The tolerance is zero, as it is today.
- **SC-003**: Zero exported identifiers of the canonical package return a count, a mean, an extreme of a measured value, a standard deviation, a percentile, a range or a series, and an automated check fails when one is added.
- **SC-004**: A 1 GB log folded through the primitives — a position taken for every item, the bounds extended for every item — stays within the peak-memory ceiling the codecs already meet, 32 MiB, and that figure does not change when the log is made ten times longer.
- **SC-005**: A position taken from the first item of a corpus run still equals itself, and still recovers the same path and name, after the reader has delivered every remaining item.
- **SC-006**: For each of the three malformed inputs, both codecs produce the same outcome, and a test that fails when either codec changes alone exists for each.
- **SC-007**: Every golden record stream in the corpus is byte-identical before and after this feature.
- **SC-008**: A run record declaring 2000 assertions decodes and all 2000 are delivered; a claimed group depth of 1<<20 is refused naming the byte offset.
- **SC-009**: The README and the package documentation name the same readable formats and ranges, and a search of the source tree for the deleted sample directory returns zero results.
- **SC-010**: A consumer computing per-position counts and a mean request rate for a corpus run writes no definition of a position or a span of its own: the documented example does it in one loop, and it runs in the test suite.
- **SC-011**: Automated tests exercise at least 90% of each decoder package and at least 80% of the module overall, and every test that passed before this feature still passes after it.

## Assumptions

- **The milestone is the scope, and it is one spec with four issues.** Milestone v0.0.6 is issue #8 and three fixes, and the milestone's own description names all four. The precedent is spec 003, which took its milestone's two issues into one document. Each issue still lands as its own commit under the one-issue-one-commit rule; the spec bundles the definitions, not the pull requests. Issue #62, also in the milestone, already landed on `main` as commit f454217 and is out of scope here.
- **The outcome predicate already exists and is not rebuilt.** The constitution names four primitives — position, bounds, the outcome predicate and a way to walk the stream. v0.0.3 delivered the outcome (a sample's recorded outcome, with a failure present if and only if it failed) and the stream. This feature adds the two that are missing and touches neither of the two that exist.
- **The bounds definition is Gatling's, stated in the model's terms, and is adopted for every source.** Earliest and latest of sample, group and user events is the span the tool's own report uses, and it is expressed in model fields every source provides. A later source whose own report bounds a run differently is that source's milestone to reconcile, against that tool's report; the definition here does not change per tool, because a definition that varies per tool is the divergence this feature exists to prevent.
- **A start the source could not resolve becomes visible as absent, and that may change the shape of a public field.** Today an unresolvable event time reaches the canonical model as an instant in the distant past, because the model's start field cannot say "absent". FR-023 requires that it can, and the bounds must be able to skip it. How is the plan's decision; the constitution's ask-first rule for public API changes applies at that point, and whatever changes is recorded under Changed. Before v0.1.0 this is permitted.
- **The text codec follows the spec on malformed values, not the other way round.** Issue #56 weighed making the binary codec refuse as the text codec does, and rejected it: spec 005 FR-009 is explicit, and one bad field ending a ten-million-record read is the behaviour review already rejected. The refusals the text codec drops are recorded as changed behaviour.
- **The ceilings are numbers the plan chooses; the spec fixes only what they must admit and what they must refuse.** At least 2000 assertions must decode, and a count that can only be corruption must not size an allocation. The nesting ceiling and the string-length ceiling are not changed.
- **Positions and bounds are derived, not decoded, so no capability changes.** Both come from fields every source records; nothing new is read from any log.
- **The stream stays what it is.** A consumer folds by looping over the items the reader already yields; this feature adds what each item can tell it, not a second way to traverse.
- **Statistics stay out, permanently, and the arithmetic has a home.** Counts, extremes, means, standard deviations, percentiles and bands are galaxio-cli#51; per-interval throughput and active users are galaxio-cli#61; the live half is comet's. This feature is what those compute from.
- **No dependency is added.** Everything here is standard library, as the constitution requires of the canonical and Gatling packages.
- **The API may still change before v0.1.0.** Every exported identifier added here is recorded under Added in the changelog, and the linkage between this spec and the milestone is kept as the repository's gates require.
