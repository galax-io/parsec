# Feature Specification: Reading the Binary simulation.log Gatling Writes From 3.13.0

**Feature Branch**: `005-gatling-binary-decoder`

**Created**: 2026-09-06

**Status**: Draft

**Input**: User description: "https://github.com/galax-io/parsec/milestone/5 — binary simulation.log decoder for Gatling 3.13.0+, issue #6" — milestone v0.0.5 *Binary logs, 3.13 and newer*: "Since 3.13.0 Gatling writes an undocumented binary `simulation.log` with a string cache and JVM-native UTF-16, which no third-party tool decodes. It is the format every current user produces."

## Clarifications

### Session 2026-09-06

- Q: The text corpus compares decoded counts against `js/global_stats.json` and `js/stats.json`, and Gatling stopped writing both at 3.13.5. What stands in for them? → A: **Both artefacts a run still produces, kept together.** Checked against the real 3.15.1 run this project recorded. The generated HTML report carries the full statistics table — the run total and one row per request — with each figure in its own semantically classed cell (`value total`, `value ok`, `value ko`), so the numbers are extractable and the per-request breakdown the text corpus relied on survives. It is a *file in the run directory*, so it is recoverable from an archived run. The console summary carries the `---- Global Information ----` block with request count split total/OK/KO and the response-time figures; it is standard output, so it exists only if it was redirected at run time. The HTML is the primary and the console is the cross-check: two independent accounts of the same run from the tool itself, and the second costs nothing but a redirection. Both are captured at recording time, because neither can be recovered afterwards.

- Q: Issue #6 names 3.13.5, 3.14.9 and 3.15.1, but the format changed at 3.13.0 — where does the supported floor sit, and are 3.13.0 through 3.13.4 refused? → A: **The floor is 3.13.0, and 3.13.0 is recorded so that it is the corpus saying so rather than an argument.** Checked live against Gatling's own repository. The five commits that shaped the format — `efd3ec7c` switch to a binary log, `6887b50f` single event type for User, `81fa5343` cache Strings, `9dd6fefe` String writing optimization, `14a6668d` fix: assertions serialization — all land on or before v3.13.0's tag date of 2024-11-13. After it, `LogFileDataWriter.scala` changed only for a deadlock fix, a restructuring into a factory with a fixed buffer size, and copyright years, while `RecordHeader.scala` changed by two lines of copyright and nothing else. The sequence of byte-writing calls is **identical** between v3.13.0 and v3.15.1 — thirty-three calls in the same order — so the wire format has not moved across the whole binary era. Principle II still binds the gate to the corpus and forbids widening it on the belief that a format did not change, and this is evidence rather than belief; the resolution is to record the floor rather than to argue it. The bounds, 3.13.0 and 3.15.1, are mandatory recordings; 3.14.9 is recorded as well, so that a mid-range version is covered by a run and not only by a diff.
  - **Superseded by the recording, 2026-09-06.** 3.13.0 turned out not to be recordable: it cannot read back the assertion records it writes (`IllegalArgumentException: Unknown object coding: 1`, out of boopickle in `AssertionPicklers`), so no run of it generates a report and no run of it can satisfy FR-029. Its writer is sound — a 3.13.0 log parses to its last byte with the same record counts as 3.13.1 — and the source-diff evidence above is unaffected and still correct. The floor is therefore **3.13.1**, the oldest version actually recorded, and a 3.13.0 log is refused. The answer stands as given; the corpus overruled it on a point no diff would have shown, which is what the answer said should happen. See [research.md R8](./research.md) and `testdata/corpus/gatling/3.13.1/RECORDING.md`.


## User Scenarios & Testing *(mandatory)*

### User Story 1 - The format every current user produces becomes readable (Priority: P1)

A results engineer runs Gatling 3.15.1 today, as almost everyone does. The `simulation.log` it writes is an undocumented binary stream that only the same Gatling version can read, so the run cannot be summarised, compared or re-reported by anything else. They hand it to parsec and get back the same things the text codec already gives them for a 3.12.0 run: the run header, every request with its group path and outcome, every group traversal, every virtual user event, every error, and the assertion payloads carried through untouched — as wire records, and as the canonical model a report is written against.

**Why this priority**: this is the milestone. Milestone v0.0.2 made archived runs readable; this makes *current* runs readable, and until it lands the library reads only versions nobody runs any more. Every consumer waiting on parsec — galaxio-cli, the comet sidecar, the Galaxio backend — is blocked on this and not on anything else.

**Independent Test**: record one complete Gatling 3.15.1 run, decode its log end to end, and compare the counts against what that run's own Gatling reported for itself. The test passes only on exact equality. Delivers value on its own: a format no third-party tool decodes becomes readable by anything.

**Acceptance Scenarios**:

1. **Given** a recorded 3.15.1 run whose own report states N requests of which K failed, **When** the log is read to the end, **Then** the decoded stream contains exactly N request records, of which exactly K carry the failed outcome.
2. **Given** that run contains nested groups, **When** the log is read, **Then** every request and group record carries its full ordered path of enclosing group names, and a record taken outside any group carries an empty path.
3. **Given** the log's run record, **When** it is read, **Then** the simulation class, the run identifier, the run start time, the description and the Gatling version are all available to the caller before the first event record is delivered.
4. **Given** the same reader, **When** a caller asks for canonical results instead of wire records, **Then** it receives the same `model` values the text codec produces for an equivalent run — no consumer needs a binary-specific code path.
5. **Given** the run's simulation declared assertions, **When** the log is read, **Then** each encoded payload is delivered exactly as written, with nothing decoded, validated or interpreted.

---

### User Story 2 - A name survives whatever alphabet it was written in (Priority: P1)

An engineer runs a simulation whose request names are in Cyrillic, or contain an emoji, or any character outside Latin-1. Those names come back exactly as they went in. Nothing is replaced, transliterated or mangled, and nothing silently becomes a question mark.

**Why this priority**: it is P1 rather than P2 because it is not a nicety — the binary format stores a string as the JVM's internal character array plus a marker saying which of two encodings it used, and a decoder that ignores the marker produces plausible-looking mojibake rather than an error. A wrong name is worse than a refusal: it groups two different requests together, or splits one in two, and the report is confidently wrong. Every consumer in a non-English team hits this on their first run.

**Independent Test**: record a run whose simulation names one request in Cyrillic with at least one character outside Latin-1, decode it, and compare that name byte for byte against the name the simulation declared.

**Acceptance Scenarios**:

1. **Given** a request named in Cyrillic, **When** the log is read, **Then** the decoded name is byte-identical to the name the simulation declared.
2. **Given** a run mixing names that fit Latin-1 with names that do not, **When** the log is read, **Then** both kinds decode correctly, because the encoding is a property of each string and not of the log.
3. **Given** a name carrying a character outside the Basic Multilingual Plane, **When** the log is read, **Then** it survives intact or the read fails with an error naming the offset — it is never silently truncated to a replacement character.

---

### User Story 3 - Repeated names cost nothing, and a broken reference is caught (Priority: P1)

A soak run makes millions of requests across a handful of distinct names. The log stores each distinct string once and refers back to it afterwards, so the file stays small — and the reader has to rebuild exactly the same table the writer built, in the same order, or every name after the first mistake is wrong.

**Why this priority**: it is the one mechanism in this format that makes a decoder wrong *silently and everywhere* rather than at one record. An off-by-one in the table does not fail; it renames every subsequent request. It is also why the log cannot be read from the middle, which constrains every later milestone.

**Independent Test**: decode a recorded run that repeats names heavily and confirm every record's name matches the recorded stream; then corrupt one back-reference in a copy and confirm the read fails naming the offset rather than returning a wrong name.

**Acceptance Scenarios**:

1. **Given** a log in which a name appears once in full and many times by reference, **When** it is read, **Then** every occurrence yields the same string as the first.
2. **Given** a reference to an entry that was never introduced, **When** it is read, **Then** the read fails with an error naming the byte offset, and no record after it is delivered.
3. **Given** a log read from its first byte, **When** the same log is read starting from anywhere else, **Then** that is not offered as an option — the reader requires the start of the stream and says so.

---

### User Story 4 - Nothing is decoded that the project cannot vouch for (Priority: P2)

Someone hands the reader a binary log written by a Gatling version the project has never recorded. They do not get a plausible-looking answer. A version below the recorded range is refused, naming the version found and the range supported. A version above it decodes but says, in the result itself, that no recording covers it. A caller that cannot use an unverified number asks for strictness and gets a refusal instead.

**Why this priority**: the format is undocumented and has already changed once. Constitution Principle II makes the gate a MUST for every codec, and the decision itself already exists — milestone v0.0.4 put it in one place precisely so this codec would inherit it rather than grow a second copy that drifts.

**Independent Test**: feed the reader run records naming a version below the range, inside it, above it, and one that is not a plain release, and confirm the four outcomes are refusal, clean acceptance, acceptance carrying a warning, and refusal.

**Acceptance Scenarios**:

1. **Given** a binary log whose version is below the lowest recorded one, **When** it is read, **Then** the read is refused with an error naming both the version found and the supported range, and no event record is delivered.
2. **Given** a binary log whose version is above the recorded range, **When** it is read, **Then** records are delivered and the caller receives exactly one warning naming the version.
3. **Given** the same log and a caller that asked for strictness, **When** it is read, **Then** it is refused instead.
4. **Given** the codec's supported range, **When** it is compared against the versions the golden corpus covers, **Then** the two are equal — the range is derived from the recordings and cannot be widened without adding one.

---

### User Story 5 - The caller stops having to know which Gatling ran (Priority: P2)

A consumer hands parsec a `simulation.log` from an archive without knowing which version produced it. Until now a binary one came back refused, naming the format and saying no codec read it yet. Now it comes back decoded. Nothing about the caller's code changes: the same entry point, the same result types, the same version gate.

**Why this priority**: this is what milestone v0.0.4 was built for — it identified the format and named the gap. Closing the gap is what turns two codecs into one library. P2 only because it is the consequence of User Story 1 rather than a separate build.

**Independent Test**: open a text log and a binary log through the same entry point and confirm both yield records; confirm the module reports the binary format as readable, over the range its corpus covers, without a decode.

**Acceptance Scenarios**:

1. **Given** a binary log, **When** it is opened through the format-detecting entry point, **Then** records are delivered rather than a "no codec for it yet" refusal.
2. **Given** a consumer asking what this module reads, **When** it inspects the answer, **Then** the binary format is reported as readable with the range its corpus covers, and the reported range equals what the codec accepts.
3. **Given** a text log and a binary log of the same simulation, **When** both are read through the same entry point, **Then** the caller distinguishes them only by the version each names — not by which methods it may call or which types it receives.

---

### User Story 6 - A log larger than memory still reads (Priority: P3)

An engineer reads a multi-gigabyte binary log from a long soak run on an ordinary machine. It works, and it works in memory that does not grow with the run. Feeding the same log in one pass or in arbitrary chunks makes no difference to what comes out.

**Why this priority**: soak logs are large but are not the common case, and the earlier stories are testable on small recordings. It is nonetheless mandatory rather than optional — bounded memory is a constitution MUST for every codec, and this format has a wrinkle the text one does not: the string table is retained for the whole read, so "bounded" has to mean bounded by the *simulation* rather than by the *run*.

**Independent Test**: read a large generated log while observing peak memory, then read the same log split at arbitrary byte boundaries and compare the two record streams.

**Acceptance Scenarios**:

1. **Given** a log of any size, **When** it is read, **Then** peak memory stays within a fixed budget that does not grow with the number of records.
2. **Given** a run that repeats a handful of names across millions of requests, **When** it is read, **Then** memory reflects the number of distinct names, not the number of records.
3. **Given** the same log read in one pass and in chunks split at arbitrary byte boundaries, **When** the two results are compared, **Then** the record streams are identical, and a log that fails fails at the same offset with the same error.
4. **Given** a length prefix that claims more bytes than the file holds, **When** it is read, **Then** the read fails without allocating what the prefix asked for.

---

### Edge Cases

- **A back-reference to an entry that was never introduced.** Refused, naming the byte offset. This is the one corruption that would otherwise rename every later record rather than failing.
- **A length prefix larger than the remaining file, or large enough to exhaust memory.** Refused without allocating it — Principle II requires length-prefixed reads to cap allocations.
- **An encoding marker that is neither of the two known values.** Refused, naming the offset; a decoder that guessed would return mojibake that looks like data.
- **A record kind byte the recorded versions never write.** Refused for a covered version; for a version above the range the warning already in hand explains it, and the read may continue only if the record's length is knowable.
- **A log truncated mid-record**, including mid-string and mid-length-prefix. Refused, naming the offset.
- **An assertion payload whose declared length runs past the end of the file.** Refused rather than skipped.
- **A run record naming no scenarios**, and a request that names no scenario at all.
- **A timestamp offset that would overflow when added to the run start.** Reported as absent rather than wrapped to a plausible negative — the text codec already refuses to wrap and this must match.
- **A text log handed to the binary codec, or the reverse.** Refused on the first bytes rather than decoded into nonsense.
- **An empty file, and a file holding only a truncated run record.** Refused; no version can be established, so no gate can be applied.

## Requirements *(mandatory)*

### Functional Requirements

**Decoding**

- **FR-001**: System MUST decode every record kind a binary `simulation.log` contains for the supported versions — the run record, request, virtual-user event, group traversal and error — into the same wire record types the text codec already produces. It MUST NOT introduce a second, binary-specific record type for consumers to depend on.
- **FR-002**: System MUST read every integer in the byte order the format writes them, and MUST establish that order from a recording rather than assuming it.
- **FR-003**: System MUST decode each string from its length prefix, its bytes and its encoding marker, honouring the marker per string rather than per log.
- **FR-004**: System MUST preserve every name character for character, whatever alphabet it uses. A character outside the encoding the marker names MUST fail the read rather than be replaced.
- **FR-005**: System MUST preserve the success/failure outcome of every request and group exactly as recorded, and the accompanying message character for character.
- **FR-006**: System MUST preserve group hierarchy: every request and group record carries the ordered list of enclosing group names, and a record taken outside any group carries an empty list.
- **FR-007**: System MUST deliver assertion payloads verbatim, skipping them by their declared length without decoding, validating or interpreting them.
- **FR-008**: System MUST make the run record's contents — simulation class, run identifier, run start, description and Gatling version — available to the caller before the first event record is delivered.
- **FR-009**: System MUST resolve every recorded time to the same absolute instant the text codec reports for an equivalent event, whatever representation the binary format uses for it. A value that cannot be resolved MUST be reported as absent, never wrapped or guessed.

**The string table**

- **FR-010**: System MUST rebuild the writer's string table exactly, in the order the writer built it, so that a reference yields the string the writer meant.
- **FR-011**: System MUST require the stream to start at the beginning of the file, and MUST say so in its documentation. Reading from an arbitrary offset MUST NOT be offered, because the table cannot be reconstructed from the middle.
- **FR-012**: A reference to an entry that was never introduced MUST fail the read with an error naming the byte offset. It MUST NOT yield an empty string, a placeholder, or the nearest entry.
- **FR-013**: The memory the table occupies MUST be bounded by the number of distinct strings a simulation declares, not by the number of records the run produced.

**Version gate**

- **FR-014**: System MUST establish the Gatling version from the run record before delivering any event record, and MUST refuse a log whose run record cannot be read.
- **FR-015**: System MUST apply the version policy this module already owns rather than implementing a second one: below the covered range refused naming both the version and the range; inside it decoded cleanly; above it decoded with exactly one warning; above it under strictness refused; and a version string that is not a plain release refused, quoting what was found.
- **FR-016**: The supported version range MUST equal the range the golden corpus covers, MUST be readable programmatically, and MUST NOT be widenable by a caller.

**Dispatch**

- **FR-017**: A caller handing over a `simulation.log` without knowing its format MUST receive a working reader for a binary log, where it previously received a refusal naming the format.
- **FR-018**: The module's reported coverage MUST show the binary format as readable over the range its corpus covers, and that report MUST be derived from the codec rather than restated.
- **FR-019**: Records obtained through the format-detecting entry point MUST be identical, field for field, to those obtained by handing the same log directly to this codec.

**Canonical model**

- **FR-020**: System MUST convert its records into the same `model` types the text codec produces, so a report written against the model works for both formats with no tool-specific branch.
- **FR-021**: What a binary log cannot record MUST be declared through `Capabilities` and reported as absent, never filled with a zero, an average or a guess. Where the binary format carries something the text format does not, or the reverse, the difference MUST be visible in `Capabilities` rather than hidden.

**Resilience**

- **FR-022**: System MUST stop at the first byte it cannot decode and fail the read with an error naming the byte offset and what was expected there. It MUST NOT skip, infer, or read past the failure.
- **FR-023**: A read that stopped MUST be distinguishable from one that reached the end. Records delivered before the failure MUST NOT be presentable as a complete result, and no total may be derived from them.
- **FR-024**: System MUST NOT crash on any input, including empty, truncated, text, and randomly mutated content.
- **FR-025**: Every length read from the file MUST be capped before it is used to allocate, so that a corrupt length cannot exhaust memory. The cap and its reasoning MUST be documented next to it.

**Streaming**

- **FR-026**: System MUST read the log as a stream, and its peak memory MUST NOT grow with the number of records.
- **FR-027**: Reading a log in one pass and reading it in chunks split at arbitrary byte boundaries MUST produce identical record streams, and for a log that fails MUST fail at the same offset with the same error.

**Evidence**

- **FR-028**: The golden corpus MUST contain one complete recorded run for 3.13.1 and for 3.15.1 — the bounds of the supported range — and for 3.14.9, of the same sample simulation, recorded from a real Gatling of that version and committed as produced. The floor is 3.13.1 rather than 3.13.0 because 3.13.0 cannot produce a report at all (see Clarifications), so no run of it can satisfy FR-029.
- **FR-029**: Each recorded run MUST be captured together with two accounts of its numbers that the tool itself produced: the generated HTML report, which carries the run total and a per-request row with each figure in its own classed cell, and the console summary's Global Information block, redirected to a file at run time. From 3.14.0 Gatling stopped writing the two statistics files the text corpus relies on, and these replace them; 3.13.1 still writes both, so that entry keeps them as well and the report and console are a third and fourth account rather than a substitute. Both MUST be captured at recording time — the report is a file in the run directory and survives archiving, the console summary is standard output and exists only if it was redirected, and neither can be recovered from a run that has already finished.
- **FR-029a**: What is kept MUST be the artefacts as the tool wrote them, not a summary of them. Extraction happens in the verification suite, so that a later reader can check what the run actually said rather than trusting what was recorded about it.
- **FR-029b**: A file the report needs only to *render* — a vendored JavaScript library, a stylesheet, the tool's own logo — MUST NOT be kept. It says nothing about the run, is byte-identical in every run of that version, and is recoverable by running the tool. This is not an exception to FR-029a: every file that carries a number, a name or a timestamp from the run is kept whole and unedited. Measured on the recordings this feature adds, the rule removes about 1.4 MB of 1.7 MB, and for 3.14.0 and newer it removes the entire `js/` and `style/` trees, because every figure that run reported is in the markup of `index.html`.
- **FR-030**: Each recorded run MUST exercise every record kind, nested groups, both failure kinds, a name outside Latin-1, and a name repeated often enough to be stored by reference.
- **FR-031**: Verification MUST cover both levels: per-record checks over well-formed and malformed inputs for every field of every record kind, and end-to-end checks that read each complete recording and compare it against that run's kept numbers and its golden record stream.
- **FR-032**: The same sample simulation MUST be recorded under every supported version, so that the runs can be compared against each other with timing, identity and order set aside.
- **FR-033**: Any hand-written input used to exercise a case no recording can produce MUST be named as a fixture, not as corpus.
- **FR-034**: The 64-byte sample this project already holds MUST NOT be used as corpus. It proves that a binary log is identified as binary and nothing else, and it MUST be superseded rather than extended.

### Key Entities *(include if data involved)*

- **Run record**: the first record of every binary log. Carries the Gatling version, the simulation's identity, the run's start, its description and the scenarios it declared, plus the encoded assertion payloads. It is what the version gate reads and what every later record is interpreted against.
- **String table**: the writer's list of distinct strings, rebuilt by the reader in the same order. A string arrives either in full — to be remembered — or as a reference to something already remembered. It is the reason the log must be read from its first byte.
- **Encoding marker**: the per-string value saying which of two character encodings its bytes are in. Not a property of the log; two strings in one record may differ.
- **Record kind**: the leading value of every record, saying which of the five it is and therefore how to read the rest.
- **Request record**: one request attempt — its group path, name, timings, outcome and failure message.
- **Group record**: one group traversal closing, with its path, timings, cumulated response time and outcome.
- **User event**: one virtual user starting or ending a scenario.
- **Error record**: a free-text message and the time it occurred.
- **Assertion payload**: an opaque encoded blob inside the run record, skipped by length and carried through unread.
- **Version verdict**: what the shared policy decided for the version the run record names — refused, accepted, or accepted-but-unverified — carrying the version found and the range that decided it.

### Source Coverage *(include if the feature reads a tool artefact)*

- **Tool and versions**: Gatling 3.13.1 through 3.15.1. The floor is a recording, as Principle II requires, and it is 3.13.1 rather than the format's own boundary at 3.13.0 because 3.13.0 cannot be recorded: it fails to read back the assertion records it writes, so no run of it generates a report (see Clarifications). 3.13.1 and 3.15.1 are the mandatory bounds, with 3.14.9 recorded as well so a mid-range version rests on a run rather than on a source diff. **A 3.13.0 log is therefore refused**, although this codec could read it — the cost of binding the range to the corpus, paid rather than argued away.
- **Artefact formats**: the binary `simulation.log` written from Gatling 3.13.0. The tab-separated text log is milestone v0.0.2's and is untouched here.
- **Version gate**, five outcomes: below the covered range — refused, naming the version and the range. Inside the range — decoded, no warning. Above the range — decoded with one warning. Above the range under strictness — refused. Not a plain release number — refused, quoting the string.
- **Not provided by this source** (declared through `Capabilities`, never filled in): to be established from the recordings rather than assumed. The text log carries no response code, no body sizes, no connection timings, no per-request throughput and no per-interval series; whether the binary format carries any of them is a question the corpus answers, and any difference between the two formats MUST appear in `Capabilities` rather than being smoothed over.
- **Golden corpus**: `testdata/corpus/gatling/<version>/` for 3.13.1, 3.14.9 and 3.15.1, one complete run each of the same sample simulation, each committed with the report's own data files and the redirected console summary (FR-029, FR-029b). The existing 64-byte sample under `testdata/samples/` is not corpus and is superseded by these.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every corpus log in the covered range decodes to exactly the recorded record stream, field for field, with zero differences.
- **SC-002**: For each recorded run, every count that run's own Gatling reported is matched exactly by the decoded records — the request total and its OK/KO split, and the same three numbers per request name and per group. A difference of one fails the check.
- **SC-003**: A request named outside Latin-1 decodes byte-identical to the name the simulation declared, for 100% of such names in the corpus.
- **SC-004**: Recordings of the same simulation under different supported versions produce identical record multisets once timing values, run identifier, recorded version and file order are set aside.
- **SC-004a**: The advertised range equals the versions the corpus covers, checked automatically; widening the corpus without the range fails, and so does the reverse.
- **SC-005**: A 1 GB binary log is read end to end with peak memory under 32 MiB, and that figure does not change when the log is made ten times longer with the same set of distinct names.
- **SC-006**: For 100% of corpus files, reading in one pass and reading in arbitrary chunks produce identical records, and identical failures where the input fails.
- **SC-007**: A copy of a recording with exactly one corrupted byte fails the read naming that byte's offset, for every corruption position tested — in a length prefix, in a string's bytes, in an encoding marker, in a back-reference and in a record kind — and in no case is a partial record stream returned as a success.
- **SC-008**: A randomised robustness run over mutated and truncated corpus files completes with only errors and no crash, across at least 10,000 mutations, and in no case allocates more than the documented cap.
- **SC-009**: Every accepted, refused and warned version outcome named in Source Coverage has at least one automated check asserting it, and each check fails when its rule is removed.
- **SC-010**: A binary log opened through the format-detecting entry point yields records identical, field for field, to opening it through this codec directly — zero differences over the whole corpus.
- **SC-011**: A report written against the canonical model renders a binary run and a text run with no tool-specific or format-specific branch.
- **SC-012**: Automated tests exercise at least 90% of the decoder package and at least 80% of the project overall.
- **SC-013**: Every test that passed before this feature still passes after it, unchanged.

## Assumptions

- **What the layout rests on, in three tiers.** *Observed in a real log*: 64 bytes of a 3.15.1 run show the run record opening with a kind byte of `0x00`, a string encoded as a four-byte big-endian length then its bytes then a one-byte encoding marker, big-endian integers, and an eight-byte epoch-millisecond value after the simulation's name. *Read out of Gatling's own writer at both bounds of the range*: the record kinds are Run 0, Request 1, User 2, Group 3, Error 4, exactly as issue #6 states; the writer's vocabulary is fourteen integer writes, five strings, four booleans, three bytes and two longs; and the string cache starts its index at 1 because the negative of an index marks a cache hit and zero has no negative. The booleans are new information — issue #6 does not mention them. *Still a claim*: the back-reference scheme's exact encoding, timestamps stored as offsets from the run start, requests naming a scenario by absence, and how assertion payloads are framed. Where a recording disagrees with any of it, the recording wins and the spec is corrected.
- **The wire format has not moved since 3.13.0, and that is checked rather than hoped.** The byte-writing sequence in Gatling's `LogFileDataWriter.scala` is identical at v3.13.0 and v3.15.1 — the same thirty-three calls in the same order — and `RecordHeader.scala` differs between them only in its copyright year. Everything that shaped the format landed on or before v3.13.0's tag date. This is why one decoder can serve eighteen releases. It is not, however, why the gate opens where it does: Principle II binds the range to the corpus and forbids widening it on the belief that a format did not change, so the floor is the oldest version actually recorded. That turned out to be 3.13.1, and no source diff would have shown why — which is the concrete vindication of the rule.
- **The sample is not a corpus entry and cannot become one.** It holds 64 bytes of a throwaway one-request run with no report. It proved format detection for milestone v0.0.4 and is superseded here.
- **The version policy is inherited, not rebuilt.** Milestone v0.0.4 moved the refuse/accept/warn decision into one place and returns the verdict a codec acts on, specifically so this codec would not grow a second copy. This feature adds a range and a corpus, not a rule.
- **Format detection already exists and is not revisited.** A binary log is already identified from its leading bytes without consulting the file's name. This feature makes the codec that detection dispatches to.
- **Byte order for the wide encoding is the writer's, and the log does not record it.** Where a string is stored in the JVM's wide encoding, the byte order is the writing JVM's, and nothing in the file says which that was. Every machine this project can record on is little-endian, so the corpus cannot prove the other case. The decoder assumes little-endian, documents the assumption, and a log written on a big-endian JVM will decode wrongly with no way to detect it. This is a limitation of the format, not of the reader, and it is recorded rather than hidden.
- **Recording is the long pole and cannot be recovered later.** Three Gatling versions must be run for real, and each run's two accounts of its own numbers captured at that moment.
- **The probe drops picatinny above the line picatinny supports, and writes plain Gatling assertions there.** `gatling-picatinny` has no release targeting the 3.14.x or 3.15.x line — established while capturing the format sample — so for those recordings the probe expresses its assertions in Gatling's own DSL instead. The consequence is deliberate and worth naming: those runs no longer state their expectations in OpenNFR, which milestone v0.0.3 made the single source for them, so the same expectations exist in two forms and can drift. Keeping the runs recordable is worth that, because an unrecorded version is a version this module cannot support at all; carrying the OpenNFR rendering forward is a matter for whenever picatinny reaches those lines.
- **The probe must be extended before it is recorded.** The existing sample simulation exercises what the text corpus needed. This corpus additionally needs a name outside Latin-1 and a name repeated often enough to be stored by reference — neither is present today, and neither can be added to a run after it is made.
- **Statistics stay out of this module.** Counting records to compare against Gatling's own numbers is the verification suite's work. The decoder offers records and the primitives to fold them; the arithmetic belongs to the consumer.
- **Reading a growing log is out of scope.** The string table makes a mid-file start impossible, which shapes milestone v0.0.9 but is not solved here.
- **Decoding the assertion payload is out of scope**, as it is for the text codec. Its encoding is a Scala serialisation format, and the requirements it carries are expressed in OpenNFR instead.
- **This builds on v0.0.2, v0.0.3 and v0.0.4.** The wire records, the canonical model, `Capabilities`, the version policy, the shared errors and format detection are all in place. This feature adds a codec and its evidence.
