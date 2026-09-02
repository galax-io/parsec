# Feature Specification: Reading Gatling 3.11.5–3.12.x Text simulation.log Files

**Feature Branch**: `002-gatling-text-decoder`

**Created**: 2026-09-02

**Status**: Draft

**Input**: User description: "https://github.com/galax-io/parsec/issues/3 — реализуй парсер лога, тесты должны быть e2e и юниты, проверить соответствие значений с отчётом гатлинга, он должен быть идентичный"

## Clarifications

### Session 2026-09-02

- Q: FR-014 required every unreadable-line report to reach the caller while FR-017 required peak memory to stay independent of log size. The two collide on a log with millions of unreadable lines. How is it resolved? → A: Neither by streaming reports nor by capping them — the read stops at the first unreadable line and fails. A log that cannot be read in full cannot yield counts that match Gatling's own report, and exact counts are the point of the feature, so a partial read is refused rather than reported. This overrides issue #3's "the read continues" acceptance bullet and follows Principle II instead.
- Q: What exactly is committed alongside each corpus `simulation.log` as "the report Gatling produced for that run"? → A: Two files from the generated report — `js/global_stats.json` (run totals) and `js/stats.json` (per-request and per-group breakdown). Nothing else: no `assertions.json`, no console summary, no HTML or bundled assets.
- Q: FR-016 capped the memory used for a single line at "the documented limit" without naming it, and under the fail-fast decision an over-long line now fails the whole read — so too low a cap would reject a valid log. What is the cap? → A: 1 MiB per line.
- Q: The version gate covered only below-range, in-range and above-range. What happens when the run header carries a version string that is not a plain release number — `3.13.0-SNAPSHOT`, `3.12.0-M1`, or something unreadable? → A: Refused, quoting the string found. Only a plain `MAJOR.MINOR.PATCH` release number reaches the gate. Scope of that refusal was checked explicitly: it covers suffixed and unreadable strings only. A clean release number still follows the normal gate — below the range refused, in range clean, above the range decoded with a warning — so Principle II's "an unknown newer version MUST decode and MUST surface a warning" stays intact.
- Q: FR-021 promised that seven counts would match Gatling's report, but the two kept report files carry request and group statistics only — not user starts, user ends or error counts. What is verified against what? → A: FR-021 is narrowed to what the kept files can actually prove: the run's request total with its OK/KO split, and the same three numbers per request name and per group, all exact. User starts, user ends and error records are verified against the recorded golden record stream instead, and the spec no longer claims they agree with Gatling's report.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - An archived run becomes readable again (Priority: P1)

A results engineer holds a `simulation.log` from a Gatling 3.11.5 run archived a year ago. The Gatling installed today refuses to open it, so the run cannot be summarised, compared or re-reported. They hand the file to parsec instead and get back every event the run recorded: the run header, each virtual user starting and ending its scenario, each request with its group path and OK/KO outcome, each closed group with its cumulated response time, each error, and the assertion payload carried through untouched. The totals they count from those records are the same numbers Gatling itself printed when the run finished — not close, the same.

**Why this priority**: this is the entire point of the milestone. Every other story in this feature protects, extends or hardens this one. Without it a whole class of archived results stays unreadable, and no later milestone — the canonical model, statistics, series — has an input to work from.

**Independent Test**: take one recorded 3.11.5 run, committed together with the statistics files Gatling produced for it, decode the log end to end, and compare the request counts and their OK/KO split — for the run as a whole, and for each request name and each group — against those files. The test passes only on exact equality. Delivers value on its own: results that were readable by exactly one Gatling installation become readable by anything.

**Acceptance Scenarios**:

1. **Given** a recorded 3.11.5 run whose own report states N requests of which K failed, **When** the log is read to the end, **Then** the decoded stream contains exactly N request records, of which exactly K carry the failed outcome.
2. **Given** that run contains nested groups, **When** the log is read, **Then** every request and group record carries its full ordered path of enclosing group names, and a record taken outside any group carries an empty path.
3. **Given** the log's header line, **When** the log is read, **Then** the simulation class, the run identifier, the run start time, the description and the Gatling version are all available to the caller before the first event record is delivered.
4. **Given** a run whose description was left blank and therefore written as a single space, **When** the header is read, **Then** the description is reported as empty, not as a one-character string.
5. **Given** a run whose simulation declares assertions, **When** the log is read, **Then** one assertion record is delivered for each declared assertion, ahead of the run header, and each encoded payload is delivered exactly as written with nothing decoded, validated or interpreted.
6. **Given** the run contains both a request that failed a check and a request that failed with an exception, **When** the log is read, **Then** both requests are reported as failed, the message Gatling wrote for each is preserved character for character, and the exception also appears as its own error record.
7. **Given** a value that contained a tab or newline in the running simulation, **When** the log is read, **Then** the value is returned as Gatling wrote it — with those characters already replaced by spaces — and the reader does not attempt to split it further.

---

### User Story 2 - The same simulation under 3.12.0 yields the same records (Priority: P2)

A maintainer needs to know that moving a project from Gatling 3.11.5 to 3.12.0 does not silently change what its results mean. The same sample simulation is run under both versions, and both logs decode to the same sequence of records: the same requests, in the same order, with the same names, the same group paths and the same outcomes. Only the things that must differ — wall-clock timestamps, the run identifier, the recorded Gatling version — differ.

**Why this priority**: the milestone covers two versions, and cross-version equality is the only evidence that the format genuinely did not change between them rather than that nobody looked. It is P2 rather than P1 because 3.11.5 alone already delivers a working, useful reader.

**Independent Test**: decode the 3.11.5 and the 3.12.0 recording of the same simulation, set aside every timing value (timestamps and cumulated response times), the run identifier, the recorded version and file order, and compare the two record streams as multisets. The comparison must show no difference. File order is set aside because concurrent virtual users interleave differently on every run, so it is not evidence of anything.

**Acceptance Scenarios**:

1. **Given** the same simulation recorded under 3.11.5 and under 3.12.0, **When** both logs are read, **Then** the two record streams are identical as multisets once timing values, run identifier, recorded Gatling version and file order are set aside.
2. **Given** the 3.12.0 recording, **When** it is read, **Then** its counts equal the numbers in the report Gatling 3.12.0 printed for that run, to the unit.
3. **Given** a field that is absent in one version and present in the other, **When** either log is read, **Then** the difference is visible in the record stream comparison and is not absorbed by a default value.

---

### User Story 3 - Nothing is decoded that the project cannot vouch for (Priority: P2)

Someone points the reader at a log written by a Gatling version the project has never recorded. They are not given a plausible-looking answer. A version older than anything the project has evidence for is refused outright, with an error that names the version found and the range that is supported. A version newer than anything recorded still decodes — the caller is not blocked — but the caller is told, in the result itself, that the version has never been verified against a real recording. And a log that does not name a plain release version at all — a snapshot, a milestone build, or an unreadable string — is refused, because there is no released version to place it against.

**Why this priority**: the log format is undocumented and has already changed once. A reader that quietly decodes an unknown version produces numbers nobody can trust and nobody can tell apart from correct ones. This is a MUST in the project constitution, so it ships with the feature rather than after it.

**Independent Test**: feed the reader four headers — one naming a version below the supported range, one naming a covered version, one naming a version above the range, and one carrying a snapshot or milestone suffix — and confirm the four outcomes are refusal, clean acceptance, acceptance carrying a warning, and refusal.

**Acceptance Scenarios**:

1. **Given** a log whose header names a Gatling version below the lowest recorded version, **When** it is read, **Then** the read is refused with an error naming both the version found and the supported range, and no event record is delivered.
2. **Given** a log whose header names a version above the recorded range, **When** it is read, **Then** the records are delivered and the caller receives a warning naming the version found and saying it is not covered by any recording.
3. **Given** such a warning was raised, **When** the caller inspects the result, **Then** the warning is available there — it is never delivered only through a log or a printed message.
4. **Given** a log with no run header at all, **When** it is read, **Then** the read is refused, because no version can be established and therefore no gate can be applied.
5. **Given** a log whose header names a build that is not a plain release — `3.13.0-SNAPSHOT`, `3.12.0-M1`, or a string that is not a version at all — **When** it is read, **Then** the read is refused with an error quoting the string found, whether the numeric part would have fallen inside the covered range or outside it.

---

### User Story 4 - A damaged log is refused, and you are told exactly where (Priority: P2)

An archived log has been through a copy, a truncation and a text editor, and some of its lines are no longer intact. The reader does not hand back what it managed to salvage. It stops at the first line it cannot decode and says which line that was and what it expected there. The engineer learns that the log cannot produce trustworthy numbers, and where the damage starts — instead of receiving totals that quietly undercount and look exactly like a correct answer.

**Why this priority**: the value of this feature is exact numbers. A partial read produces totals that are wrong and that nothing downstream can tell apart from right ones, which would destroy the very guarantee the feature exists to make. Refusing is the only outcome that keeps "identical to Gatling's own report" meaningful. It is P2 because the primary journeys read intact recordings.

**Independent Test**: take a recorded run, corrupt one known line in a copy of it, read the copy, and confirm the read fails naming exactly that line number — and that no record stream is offered as if it were complete.

**Acceptance Scenarios**:

1. **Given** a log containing one line that matches no known record shape, **When** it is read, **Then** the read fails with an error naming that line's number and what was expected there, and no totals are offered.
2. **Given** a line whose record kind is recognised but which carries fewer fields than that kind requires, **When** it is read, **Then** the read fails the same way.
3. **Given** a line carrying more fields than its kind requires and a header version inside the covered range, **When** it is read, **Then** the read fails, because a covered version is known to write an exact field count.
4. **Given** the same line but a header version above the covered range, **When** it is read, **Then** it decodes from the fields its kind defines, the surplus is ignored, and the version warning already in hand explains why the record was unfamiliar.
5. **Given** an error record whose message contains a field separator, **When** it is read, **Then** the message is recovered whole and the read continues — this is normal output, not damage.
6. **Given** a log truncated part-way through its final line, **When** it is read, **Then** the incomplete final line fails the read rather than being decoded from its fragment or dropped in silence.
7. **Given** a log damaged at line 5 and again at line 900, **When** it is read, **Then** the error names line 5 — the first failure — and no line after it is read.
8. **Given** records were delivered before the failing line, **When** the read fails, **Then** the outcome is an error and not a shorter but successful-looking record stream, so no consumer can mistake a partial read for a complete one.
9. **Given** any input at all — empty, truncated, binary, or randomly mutated — **When** it is read, **Then** the reader returns an error and never crashes.

---

### User Story 5 - A log larger than memory can still be read (Priority: P3)

An engineer reads a multi-gigabyte log from a long soak run on an ordinary machine. It works, and it works in constant memory: the reader consumes the log as a stream and never needs the whole file resident. Feeding the same log in one pass or in arbitrary chunks makes no difference to what comes out.

**Why this priority**: soak-run logs are large but are not the common case, and the earlier stories are already testable on small recordings. It is nonetheless mandatory rather than optional — bounded memory is a constitution MUST for every codec.

**Independent Test**: read a large generated log while observing peak memory, then read the same log split at arbitrary boundaries and compare the two record streams and the two sets of reports.

**Acceptance Scenarios**:

1. **Given** a log of any size, **When** it is read, **Then** peak memory stays within a fixed budget that does not grow with the size of the log.
2. **Given** the same log read in one pass and read in chunks split at arbitrary byte boundaries, **When** the two results are compared, **Then** the record streams are identical and, for a log that fails, both fail at the same line number with the same error.
3. **Given** a single line longer than the 1 MiB per-line limit, **When** it is read, **Then** the read fails at that line and the reader does not grow its memory to hold it.

---

### Edge Cases

- **A version below the supported range**: refused, with an error naming the version found and the range supported; no records delivered (User Story 3).
- **A version above the supported range**: decoded, with a warning surfaced to the caller in the result (User Story 3).
- **A version that is not a plain release number**, such as `3.13.0-SNAPSHOT` or `3.12.0-M1`: refused, quoting the string found — even when its numeric part falls inside the covered range, because a pre-release build is not the build that was recorded.
- **A log whose first line is not the run header**: normal, not an error. Gatling writes one assertion record per declared assertion *before* the run header, so the header is the first line only for a simulation without assertions. The reader locates the header past any leading assertion records.
- **No run header anywhere in the log**: refused — the version gate has nothing to read.
- **A line before the run header that is not an assertion record**: refused, because nothing may be decoded before the version is known.
- **A log with zero event records** (a run that made no requests): the header decodes, the event stream is empty, and this is a success, not an error.
- **A completely empty file**: refused, for the same reason as a missing header.
- **A blank description or a blank failure message**, which Gatling writes as a single space: decoded as empty.
- **A record at the top level**, whose group path is written as an empty field: decoded as an empty path, distinct from a path containing one empty name.
- **A group name that itself contains a comma**: cannot occur. Gatling replaces the comma with a space as it writes the record, so every comma in a group path is a separator. Splitting on the comma is exact, and the reader does not need an escape rule. A corpus run MUST include a group name containing a comma to confirm it.
- **A value containing what looks like a separator**: only the tab separates fields, and a value is never re-split on spaces. Gatling replaces tabs and line breaks with spaces in a request's failure message, but **not** in an error record's message, nor in scenario, request or group names. An error message carrying a tab is therefore normal input and is handled by FR-008b, not treated as damage.
- **A log with Windows line endings**: read normally. Gatling terminates records with the line separator of the machine it runs on, so a run on Windows produces carriage returns natively — this is not a copying artefact and MUST NOT be treated as damage. A trailing carriage return is stripped; nothing else about the content is repaired.
- **A run with no assertions**, where Gatling writes no assertion line at all: a success, with no assertion record in the stream.
- **A truncated final line**: fails the read, naming that line's number (User Story 4).
- **An unrecognised record kind**, such as one added by a newer version: fails the read at that line. If the header named a version above the covered range, the version warning is already in hand and explains why an unfamiliar record kind appeared.
- **A line longer than 1 MiB**: fails the read at that line. No valid Gatling log reaches this length, so the case only arises from corruption or from a file that is not a `simulation.log` at all.
- **A log whose damage begins part-way through**: fails at the first unreadable line. The records read before it are not offered as a result, because counts taken from them would silently disagree with Gatling's report.

## Requirements *(mandatory)*

### Functional Requirements

**Decoding**

- **FR-001**: System MUST decode all six record kinds a Gatling 3.11.5 or 3.12.0 text `simulation.log` contains — run header, user event, request, group, error and assertion.
- **FR-002**: System MUST treat the tab character as the only field separator, and MUST NOT re-split a field value on spaces or any other whitespace.
- **FR-003**: System MUST decode a field written as a single space as an empty value, this being how Gatling writes an absent description or message.
- **FR-004**: System MUST preserve the success/failure outcome of every request and group record exactly as recorded, and MUST preserve the accompanying message character for character.
- **FR-005**: System MUST preserve group hierarchy: every request and group record carries the ordered list of enclosing group names, and a record taken outside any group carries an empty list.
- **FR-006**: System MUST deliver the assertion payload verbatim, without decoding, validating or interpreting its contents.
- **FR-007**: System MUST make the run header — simulation class, run identifier, run start time, description and Gatling version — available to the caller before the first event record is delivered.
- **FR-008**: System MUST preserve every recorded time exactly as written, without rounding, re-basing against the run start, or timezone conversion.
- **FR-008a**: Every record kind has an exact field count: six for the run header, seven for a request, four for a user event, six for a group, three for an error and two for an assertion. For a version inside the covered range a record MUST carry exactly that many fields, and any other number MUST fail the read. For a version above the covered range — which has already produced a version warning — a record carrying more than that many fields MUST decode from the fields its kind defines with the surplus ignored, so a newer version that appends a field still decodes as the version gate promised it would.
- **FR-008b**: The error record is the single exception to the exact count. Its message is written without escaping and can therefore contain a field separator, so the message MUST be taken as everything between the record kind and the final timestamp field, however many separators it spans. A message containing a line break cannot be recovered this way, because the record is genuinely split across lines on disk; that MUST fail the read.

**Version gate**

- **FR-009**: System MUST establish the Gatling version from the run header before delivering any event record, and MUST refuse a log whose header cannot be found or read. The header is not necessarily the first line: Gatling writes one assertion record per declared assertion ahead of it. The reader MUST accept assertion records before the header and MUST refuse any other record kind there.
- **FR-009a**: The version MUST be a plain release number of the form MAJOR.MINOR.PATCH. A version string carrying any suffix — snapshot, milestone, nightly or vendor marker — or one that cannot be read as a release number at all MUST be refused with an error quoting the string found. This holds regardless of what the numeric part would have been, because a build that is not a release cannot be placed against any recording.
- **FR-010**: System MUST refuse a log whose version is below the lowest version covered by the golden corpus, returning an error that names both the version found and the supported range, and MUST deliver no event records for it.
- **FR-011**: System MUST decode a log whose version is a plain release number above the covered range, and MUST surface a warning through the result naming the version found and stating that no recording covers it. The warning MUST be reachable by the caller and MUST NOT be delivered only as a log line or printed message.
- **FR-012**: The supported version range MUST equal the range covered by the golden corpus, and MUST be stated in the reader's own documentation so a caller can tell what is accepted, refused and warned about without running anything.

**Resilience**

- **FR-013**: System MUST stop at the first line it cannot decode and MUST fail the read with an error naming that line's number and what was expected there. It MUST NOT skip the line, infer its contents, or read any line after it.
- **FR-014**: A read that stopped on an unreadable line MUST be distinguishable from a read that reached the end. Records delivered before the failure MUST NOT be presentable as a complete result, and no total may be derived from them — a partial read cannot match Gatling's own report and so has no value as a measurement.
- **FR-015**: System MUST NOT crash on any input, including empty, truncated, binary, and randomly mutated content.
- **FR-016**: System MUST bound the memory it uses for a single line at 1 MiB, and a line longer than that MUST fail the read at that line rather than be held in memory. The limit sits far above anything a real log contains — the longest realistic line is the assertion payload, which runs to tens of kilobytes — so it rejects corruption without ever rejecting valid content.

**Streaming**

- **FR-017**: System MUST read the log as a stream, and its peak memory MUST NOT grow with the size of the log.
- **FR-018**: Reading a log in one pass and reading the same log in chunks split at arbitrary byte boundaries MUST produce identical record streams, and for a log that fails MUST fail at the same line number with the same error.

**Evidence**

- **FR-019**: The golden corpus MUST contain one complete recorded run for each supported version — 3.11.5 and 3.12.0 — of the same sample simulation. Each run MUST be committed together with the two statistics files Gatling generated for it at the moment the run was made: the run totals (`js/global_stats.json`) and the per-request and per-group breakdown (`js/stats.json`). No other part of the generated report is kept.
- **FR-020**: Each recorded run MUST exercise every record kind, nested groups, and both failure kinds: a request that failed a check, and a request that failed with an exception and therefore also produced an error record.
- **FR-021**: Every count the kept report files carry MUST be matched exactly by the decoded records: the run's total request count with its OK/KO split, and those same three numbers for each individual request name and each group. These are integer counts, not derived statistics, so the documented tolerance is zero.
- **FR-021b**: Both kept report files also carry the mean number of requests per second, split total/OK/KO, for the run and for each request and group. Verification MUST reproduce those figures exactly from the decoded records. The tolerance is zero, not because a rate is a count, but because Gatling's rate is a deterministic function of the very records being decoded: the request count divided by the run span in whole seconds, where the span runs from the earliest to the latest timestamp in the log and is rounded up to the next whole second. Anything other than an exact match means the records or the span were decoded wrongly.
- **FR-021c**: The run span that FR-021b divides by MUST be taken exactly as Gatling's own report takes it. Its start is the earliest of every request start, every group start and every user START timestamp; its end is the latest of every request end, every group end and every user event timestamp, START or END. The run header's start and error records do not participate. A user event may therefore be the earliest or the latest thing that counts, so mishandling user events shifts the span and every rate derived from it.
- **FR-021a**: Counts the kept report files do not carry — user starts, user ends and error records — MUST be verified against the recorded golden record stream instead. They MUST NOT be described anywhere as agreeing with Gatling's report, because nothing was kept that could prove it.
- **FR-022**: Verification MUST cover both levels: per-record-kind checks over well-formed and malformed inputs for every field of every record kind, and end-to-end checks that read each complete recorded run and compare it against both that run's kept report files (FR-021) and its golden record stream (FR-021a).
- **FR-023**: Any hand-written input used to exercise malformed handling MUST be named as a fixture, not as corpus, so that a later reader cannot mistake an edited file for a real recording.

### Key Entities *(include if feature involves data)*

- **Run header**: the first line of every log. Identifies the simulation class, the run identifier, the wall-clock start of the run, a free-text description and the Gatling version that wrote the log. It is what the version gate reads, and no event record is meaningful without it.
- **User event**: one virtual user starting or ending a scenario, carrying the scenario name, which of the two it is, and the absolute time it happened.
- **Request record**: one request attempt. Carries the path of enclosing groups, the request name, its start and end times, its success or failure, and the message Gatling recorded for a failure.
- **Group record**: one group closing. Carries the group path, its start and end times, the cumulated response time of the requests inside it, and its success or failure.
- **Error record**: a free-text error message and the time it occurred. An exception-backed request failure produces one of these in addition to its request record.
- **Assertion payload**: an opaque encoded blob, one per declared assertion, written *before* the run header rather than at the end of the log. Carried through untouched; its meaning is not this feature's concern.
- **Unreadable-line failure**: a line number plus what the reader expected at that line. It ends the read; it is never one item among many, and never accompanies a usable result.
- **Version verdict**: the outcome of the gate for a given log — refused, accepted, or accepted with a warning — carrying the version found and the range covered.

### Source Coverage *(include if the feature reads a tool artefact)*

- **Tool and versions**: Gatling 3.11.5 and 3.12.0. These are every released version in the range issue #3 calls "3.11.5–3.12.x": Gatling published no 3.11.6 and no 3.12 patch after 3.12.0, so the two readings of that range describe the same set of releases.
- **Artefact formats**: the tab-separated text `simulation.log`. The binary `simulation.log` written from 3.13.0 onwards is not read by this feature.
- **Version gate**, four outcomes: below 3.11.5 — refused, naming the version found and the range. 3.11.5 and 3.12.0 — decoded with no warning. Above 3.12.0 — decoded, with a warning that no recording covers that version. Not a plain release number, whether suffixed or unreadable — refused, quoting the string found. A log with no readable header is refused for the same reason as an unreadable version: nothing can be gated.
- **Not provided by this source** (declared as absent, never filled in): request and response body sizes, HTTP status codes, connection, DNS and TLS timings, per-request throughput, the requirements the assertion payload encodes, and any per-interval series. A text `simulation.log` records none of these, so nothing derived from them may be reported as measured.
- **Golden corpus**: `testdata/corpus/gatling/3.11.5/` and `testdata/corpus/gatling/3.12.0/`, one complete run each of the same sample simulation. Each run is committed with `js/global_stats.json` and `js/stats.json` from the report Gatling generated for it, captured at the moment the run was made. Between them these two files carry the run totals and the per-request and per-group breakdown, which is what every later tolerance check needs; the rest of the generated report is bundled assets and is not kept. The files cannot be added later, and both of these versions do produce them, so the constitution's "the tool version produced none" exemption does not apply here.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every corpus log in the covered range decodes to exactly the recorded record stream, field for field, with zero differences.
- **SC-002**: For each recorded run, every count the kept report files carry is matched exactly by the decoded records — the run's request total and its OK/KO split, and the same three numbers per request name and per group. A difference of one fails the check. The mean request rate reproduces the report's figure exactly, to the precision the report states it in. User starts, user ends and error-record counts are checked against the golden record stream instead, because the kept report files do not carry them.
- **SC-003**: The 3.11.5 and 3.12.0 recordings of the same simulation produce identical record multisets once timing values, run identifier, recorded Gatling version and file order are set aside — the same kinds, names, paths, statuses, messages, scenarios and events, in the same quantities.
- **SC-004**: A 1 GB log is read end to end with peak memory under 32 MiB and in under 60 seconds on a single core of a current developer machine, and the peak memory figure does not change when the log is made ten times larger.
- **SC-005**: For 100% of corpus files, reading in one pass and reading in arbitrary chunks produce identical records, and identical failures where the input fails.
- **SC-006**: A copy of a recorded run with exactly one corrupted line fails the read naming exactly that line's number, for every corruption position tested — first event line, a line in the middle, and the last line — and in no case is a partial record stream returned as a success.
- **SC-007**: A randomised robustness run over mutated and truncated corpus files completes with only errors and no crash, across at least 10,000 mutations.
- **SC-008**: Every accepted, refused and warned version outcome named in Source Coverage has at least one automated check asserting it.
- **SC-009**: Automated tests exercise at least 90% of the log-reading code and at least 80% of the project overall, measured on every change.

## Assumptions

- **User events are load-bearing, not decoration.** The run span that every derived rate divides by is bounded by request, group and user timestamps — not by the run header, and not by error records — so a user event can set either end of it. Decoding user events exactly is therefore a correctness requirement for the rates, not only a completeness requirement for the counts. A rate that comes out right while user events are mishandled is right by luck.
- **The format described here was read out of Gatling 3.11.5's own source, and the corpus is what settles it.** Field layouts and minimum record lengths come from `gatling-core` (`stats/writer/RawRecords.scala`, `stats/writer/LogFileDataWriter.scala`); the tolerances a reader may apply from `gatling-charts` (`stats/Records.scala`, `stats/LogFileReader.scala`); the contents of the kept report files from `charts/template/GlobalStatsJsonTemplate.scala` and `StatsJsTemplate.scala`; the rate formula from `stats/ResultsHolder.scala` together with `stats/buffers/GeneralStatsBuffers.scala`. What is recorded above is what the format does, not how that code does it — nothing is transcribed from it, and this module owes it no structural resemblance. Every such statement stays a claim until a recording confirms it: where a recording disagrees with this spec, the recording wins and the spec is corrected. The 3.12.0 sources MUST be diffed against the 3.11.5 ones during planning rather than assumed identical.
- **The field layout is version-specific, and there is public evidence of that.** A third-party exporter in wide use reads an older Gatling text log in which a user event carried six fields and two timestamps and a request carried a user identifier — neither of which exists in 3.11.5. That is independent confirmation that the layout has already moved once, and it is why the gate is bound to the corpus rather than to a version range someone believed was safe.
- **Not every field is sanitised on write, and this is confirmed rather than suspected.** Gatling replaces tab, carriage return and newline with a space only in a request's failure message. Scenario names, request names, group names and error messages are written without that treatment, and an error record's message is assembled from the request name and the raw crash text, so whatever the crash produced reaches the file unchanged. A tab in that text is recoverable, which is why FR-008b exists. A line break is not: it splits the record across lines on disk, and under the fail-fast rule it ends the read. The sample simulation SHOULD provoke a multi-line error so the corpus records whether this happens in practice.
- **A request end timestamp may be a sentinel.** Gatling's own reader branches on an end value equal to the minimum signed 64-bit integer, treating it as an event that never completed rather than a normal request. Whether a 3.11.5 or 3.12.0 run can actually produce one is unconfirmed; the reader MUST NOT assume the end timestamp is always at or after the start, and the recording task MUST check whether the case appears.
- **The kept report files must still be checked while the run is being made.** Their contents are established for 3.11.5 from the report templates cited above: between them they carry the run's request totals, the per-request and per-group breakdown, and the mean request rate, and no file in the generated report carries virtual-user or error-record counts. What is not established is 3.12.0, and neither is anything about a real generated report as opposed to the code that generates it. The recording task MUST confirm both at capture time and, if a kept file turns out not to carry what FR-021 and FR-021b compare, capture whatever does — after the run is archived nothing can be added.
- **No recording exists yet.** The constitution records that no corpus entry has been created, so the 3.11.5 and 3.12.0 runs are produced as part of this feature: the same sample simulation, run under each pinned Gatling version, each captured together with the statistics report that run generated. A run archived without its report can never prove a decoder's numbers afterwards, so the report is captured at recording time or never.
- **The corpus fixes the gate.** The accepted range is written from the corpus, not from the issue's version wording. Because 3.12.0 is the only released 3.12 patch, the corpus range and issue #3's "3.11.5–3.12.x" agree; if Gatling ever publishes another 3.12 patch, widening the gate means recording it first.
- **Corpus layout follows the constitution.** Issue #3 writes the fixture paths as `testdata/corpus/3.11.5/`; the constitution's `testdata/corpus/<tool>/<version>/` layout applies, so the runs land under `testdata/corpus/gatling/`.
- **These records are the log's events, not the canonical result model.** Converting them into the project's canonical model, and declaring through Capabilities what this source cannot provide, is milestone v0.0.3. Nothing here assumes the shape those types will take, and no consumer is asked to depend on these record types as a result model.
- **Deciding which log to read is out of scope.** Locating a run directory is milestone v0.0.10 and telling a text log from a binary one before opening it is milestone v0.0.4. This feature is handed an already-opened text log and gates on the version its header names.
- **Statistics are out of scope.** Counts, timings, percentiles and series are milestones v0.0.7 and v0.0.8. Counting records to compare against Gatling's report is done by the verification suite; the reader itself offers no totals.
- **The log has stopped growing.** Reading a log while the run is still writing it is milestone v0.0.9.
- **Exactness beats salvage, and this overrides issue #3.** Issue #3's acceptance list says a malformed line "is reported with its line number and the read continues". That was reconsidered during clarification and reversed: the read fails at the first unreadable line. A log read only in part cannot produce counts that equal Gatling's own report, and those counts are the whole point of the feature, so a salvaged partial result would be a wrong answer wearing the shape of a right one. This also matches Principle II, which requires malformed input to return an error carrying the line number; the constitution supersedes the issue text, and issue #3 should be corrected to match.
- **The assertion payload stays opaque.** Its encoding is a Scala serialisation format, and the requirements it carries are expressed in OpenNFR instead; decoding it was considered and rejected in issue #3.
- **Both failure kinds are producible.** The sample simulation can be pointed at a controllable endpoint that yields both a failed check and a connection-level exception, so a single recorded run exercises both without hand-editing.
- **Group paths split losslessly.** A comma cannot survive inside a group name: Gatling replaces it with a space on write, so a comma in a group path is always a separator and the split is exact. This is write-side behaviour in 3.11.5, not an assumption about typical data, and the corpus run MUST include a group name containing a comma to prove it.
- **This builds on the scaffold.** The module, CI gates, licence and spec-driven flow from issue #2 (milestone v0.0.1) are in place; this feature adds the first code and the first corpus entry the project has ever had.
