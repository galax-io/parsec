# Feature Specification: Telling Which Gatling Wrote a simulation.log

**Feature Branch**: `004-gatling-format-detection`

**Created**: 2026-09-05

**Status**: Draft

**Input**: User description: "https://github.com/galax-io/parsec/milestone/4" — milestone v0.0.4 *Which Gatling wrote this log*, issue [#5](https://github.com/galax-io/parsec/issues/5): "A `simulation.log` carries no magic number and no format version, so a reader cannot tell a 3.12 text log from a 3.15 binary one, nor refuse a version it does not understand. What happens on an unknown future version is a policy decision, not a patch."

## Clarifications

### Session 2026-09-05

- Q: Issue #5's acceptance names five versions — 3.11.5, 3.12.0, 3.13.5, 3.14.9, 3.15.1 — and asks that format *and version* be identified for each, but this repository has never recorded a binary Gatling run, and Principle III makes a recording irreplaceable. Does this milestone record the complete binary runs, or only the minimum needed to prove identification? → A: The minimum. One sample of the leading bytes of a real binary `simulation.log` proves that a binary log is identified as binary; the complete 3.13.5, 3.14.9 and 3.15.1 recordings belong to v0.0.5, which needs them for its decoder and cannot ship without them. Format is therefore identified for all five versions and version for the two this module can decode, which narrows issue #5's acceptance bullet — the narrowing is recorded in Assumptions and issue #5 should be amended. The alternative considered and rejected was recording all three runs now on the ground that a recording can never be made later; it was rejected because v0.0.5 is one milestone away and puts the same deadline on the same recordings, while doing it here would pull a decoder's worth of binary-layout work into a milestone about identification. The residual risk is written down as an assumption rather than left implicit.


## User Scenarios & Testing *(mandatory)*

### User Story 1 - A log says what it is before anything tries to read it (Priority: P1)

An engineer holds a `simulation.log` pulled out of an archive. Nobody remembers which Gatling produced it, the file is named the same whichever version wrote it, and it carries no magic number and no format version anyone can look at. They hand it to parsec and are told, before a single record is decoded, which of the two formats it is — the tab-separated text log or the binary stream — or that it is not a Gatling `simulation.log` at all. The answer comes from the bytes in the file. It is not read off the file's name, its extension, the directory it sits in, or anything the caller asserted.

**Why this priority**: every other story here depends on this one. A caller that cannot tell the two formats apart must either know in advance which Gatling ran — the thing nobody remembers — or guess. Both failures of the guess are bad, and the quiet one is worse: a binary log fed to the text reader fails at line 1 with a message about a missing tab, which tells the user nothing about the real cause and sends them looking for corruption that is not there.

**Independent Test**: hand the identifier one text log, one binary log, and a handful of files that are neither — an empty file, a compressed archive, an HTML report, a text file whose first character happens to be `R` — and confirm each is classified correctly, that no classification consults the file's name, and that the "neither" cases are named as such rather than forced into one of the two formats.

**Acceptance Scenarios**:

1. **Given** a text `simulation.log` recorded from Gatling 3.11.5, **When** it is identified, **Then** it is reported as the text format.
2. **Given** the same file renamed to `results.bin`, and a binary log renamed to `simulation.log.txt`, **When** each is identified, **Then** each is reported as the format its bytes say it is, and the names change nothing.
3. **Given** a text log whose simulation declares assertions — so the file's first line is an `ASSERTION` record and not the run header — **When** it is identified, **Then** it is reported as the text format. Both recordings in the project's own corpus open this way, so this is the ordinary case, not an edge one.
4. **Given** an empty file, **When** it is identified, **Then** the result names it as empty rather than as either format, and nothing is decoded.
5. **Given** a file that is neither format — a gzip stream, an HTML report, a JSON document, arbitrary bytes — **When** it is identified, **Then** it is refused with an error saying what was found at the start of the file, and no layout is guessed for it.
6. **Given** identification has run, **When** the caller then reads the log, **Then** the reader sees the stream from its very first byte: nothing the identifier consumed is missing, and the caller did not have to reopen, rewind or seek the file to make that true.

---

### User Story 2 - The right reader is chosen, or you are told plainly why there is none (Priority: P1)

The same engineer wants the records, not a classification. They hand the log over once and get back a reader for it. When the module has a codec for the format the file turns out to be, they get records. When it does not — today, every binary log — they get an error that names the format, says parsec cannot read it yet, and points at the milestone that will. What they never get is a syntax error about a missing tab on line 1, which is what a binary log handed to the text reader produces today.

**Why this priority**: this is the outcome the milestone exists for. Issue #5's stated consequence is that a caller must know in advance which Gatling wrote a file, and that being handed an unexpected one yields either silent nonsense or an error that says nothing about the real cause. Classification alone does not fix that; classification plus dispatch plus an honest refusal does.

**Independent Test**: hand the entry point a covered text log and confirm records come back identical to reading that log through the text codec directly; hand it a binary log and confirm the error names the binary format and the absence of a codec, and is distinguishable by a program from both "not a Gatling log" and "damaged log".

**Acceptance Scenarios**:

1. **Given** a text log from a version inside the covered range, **When** it is opened through the identifying entry point, **Then** the records delivered are identical, field for field, to those the text codec delivers when handed the same log directly.
2. **Given** a binary `simulation.log`, **When** it is opened, **Then** the read is refused with an error naming the binary format and stating that no codec for it exists yet, and no record is delivered.
3. **Given** that refusal, **When** a program inspects it, **Then** it can tell "a Gatling log in a format parsec cannot read yet" apart from "not a Gatling log" and from "a Gatling log that is damaged", without matching on message text.
4. **Given** a text log whose version is below the supported range, **When** it is opened, **Then** the refusal is the version refusal of User Story 3 — not the no-codec refusal — because the format was recognised and the version is what disqualified it.

---

### User Story 3 - One version policy, written once, applied the same way by every codec (Priority: P1)

A results platform reads logs from many teams and many Gatling versions. It needs the same answer to the same question every time, whichever codec ends up handling the file: a version older than anything the project has evidence for is refused, and the refusal names both the version found and the lowest one supported. A version the project has recorded decodes with nothing said. A version newer than anything recorded decodes, and says so exactly once — in the result the caller holds, never only in a log line.

**Why this priority**: issue #5 calls this out as the heart of the milestone — "what happens on an unknown future version is a policy decision, not a patch." Constitution Principle II makes it a MUST. Today the policy is spelled out inside the text codec; the moment a second codec exists, one of the two will drift, and the drift will be invisible because both will look correct in isolation.

**Independent Test**: drive the policy directly with a version below the floor, a version inside the range, and a version above it, and confirm the three outcomes are refusal naming both versions, clean acceptance, and acceptance carrying exactly one warning. Then confirm the text codec's own observable behaviour for those three cases is unchanged from before this feature.

**Acceptance Scenarios**:

1. **Given** a log whose version string is `3.10.5`, **When** it is read, **Then** the read fails with a message naming both `3.10.5` and the lowest supported version, and no record is emitted.
2. **Given** a log whose version string is `3.99.0`, **When** it is read, **Then** records are decoded and exactly one warning is raised, naming `3.99.0` and the range no recording covers it in.
3. **Given** that warning, **When** the caller inspects the result, **Then** it is available there as a value. It is not printed, not logged, and not discoverable only by reading standard error.
4. **Given** a log whose version string is not a plain release — `3.13.0-SNAPSHOT`, `3.12.0-M1`, or a string that is not a version at all — **When** it is read, **Then** it is refused with the string quoted back, whatever its numeric part would have gated to.
5. **Given** the identifying entry point and the codec both handle the same log, **When** an above-range version is read through the entry point, **Then** exactly one warning reaches the caller. The policy is applied once, not once per layer.

---

### User Story 4 - A caller who cannot accept an unproven result can refuse it (Priority: P2)

A release-gating pipeline compares today's run against last week's and fails the build on a regression. It cannot afford a number produced by a decoder that has never been checked against a real recording of that Gatling version — a silently wrong percentile is worse to it than no answer at all. It asks parsec for strictness, and from then on a version above the recorded range is refused instead of warned. An interactive tool asks for nothing and keeps the lenient default, because a person reading a warning can decide for themselves.

**Why this priority**: issue #5 names it — "refusing only when the caller asks for strictness" — and it is what makes the lenient default defensible. Without a way to opt out, the project would be choosing on every caller's behalf that a plausible answer beats no answer, which is untrue for at least one real consumer. It is P2 because the lenient path is what every caller gets by default and is exercised by the stories above.

**Independent Test**: read a log whose version is above the covered range twice, once with strictness requested and once without, and confirm the two outcomes are a refusal naming the version and the range, and a successful read carrying one warning. Confirm that requesting strictness changes nothing for a version inside the range and nothing for a version below it.

**Acceptance Scenarios**:

1. **Given** a log whose version is above the covered range and a caller that asked for strictness, **When** it is read, **Then** the read is refused with an error naming the version found and the range recordings cover, and no record is delivered.
2. **Given** the same log and a caller that asked for nothing, **When** it is read, **Then** records are delivered with one warning. Lenient is the default; strictness is opted into.
3. **Given** a log inside the covered range, **When** it is read strictly, **Then** the outcome is identical to reading it leniently: no warning, no refusal, the same records.
4. **Given** a log below the covered range, **When** it is read leniently, **Then** it is still refused. Strictness never makes the gate looser, only tighter.
5. **Given** a log above the covered range read strictly, **When** the refusal is inspected, **Then** it is distinguishable from the below-range refusal, because the two say opposite things about what evidence is missing.

---

### User Story 5 - A consumer can state what parsec reads without running it (Priority: P2)

A CLI prints `--version` and a support page lists what the tool accepts. Neither wants to hard-code "Gatling 3.11.5 through 3.12.0" into its own source, because that string goes stale the first time parsec widens its range and nobody notices until a user is told their supported log is unsupported. Both ask parsec instead, and get, per format, the range that format's codec covers — including the honest answer that the binary format's codec covers nothing yet.

**Why this priority**: issue #5 requires it — "the supported range MUST be exposed programmatically so consumers can report it" — and it is what stops three downstream repositories each carrying their own copy of a range that only this repository can keep true. P2 because it is small, and because nothing else in this feature waits on it.

**Independent Test**: read the advertised range programmatically and confirm it equals the range the golden corpus covers; then change the corpus coverage in a test and confirm the advertised range moves with it rather than being asserted separately.

**Acceptance Scenarios**:

1. **Given** a consumer that wants to report what is accepted, **When** it asks parsec, **Then** it receives, for each format, the oldest and newest Gatling release that format's codec accepts without a warning.
2. **Given** the advertised range, **When** it is compared against the versions the golden corpus covers, **Then** the two are equal — the range is derived from the corpus and cannot be widened by a caller.
3. **Given** a format for which no codec exists yet, **When** a consumer asks about it, **Then** it is told that the format is known and unsupported, which is a different answer from an unknown format and from a supported one.

---

### Edge Cases

- **A text log that opens with `ASSERTION` rather than `RUN`.** Gatling writes one assertion record per declared assertion *ahead of* the run header. Both recordings in `testdata/corpus/gatling/` open with the byte `A`, so a rule keyed on the letter `R` — which is the rule issue #5 proposes — misclassifies the ordinary case. Identification must recognise a text log by any record-kind literal that may legally open one.
- **A file shorter than the bytes identification wants to look at.** A one-byte file, a two-byte file, a file that is exactly `RU`. Each is refused as unidentifiable rather than classified on a partial match.
- **An empty file.** Named as empty, refused, distinguishable from "not a Gatling log".
- **A file whose first byte matches by accident.** A plain-text note beginning `Ran the suite again` is not a text `simulation.log`. Identification confirms enough of the opening to make a coincidence unlikely, and whatever it does not confirm is caught by the codec, which then reports a syntax error on line 1 — an honest outcome for a file that genuinely looked like one.
- **A non-seekable stream.** Identification happens on a pipe, a network body or a decompressor's output that cannot be rewound. Whatever it consumed to decide is still there for the codec, which for the binary format must read from byte 0 for its string cache to be correct.
- **A version string that is not a plain release.** Refused with the string quoted, per the rule spec 002 already established; strictness does not enter into it.
- **A log with no run header at all** — truncated before it, or containing only assertion records. Refused: no version can be established, so no gate can be applied.
- **A version above the range, read strictly, in a log that is also damaged.** The version refusal comes first, because it is decided before any record is read.
- **A binary log whose version cannot be reached** because the file is truncated inside its header. Reported as a damaged binary log, not as an unknown format and not as an unsupported version.

## Requirements *(mandatory)*

### Functional Requirements

**Identification**

- **FR-001**: System MUST identify, from the leading bytes of a `simulation.log` alone, whether it is the tab-separated text format, the binary format introduced in Gatling 3.13.0, or neither.
- **FR-002**: Identification MUST NOT consult the file's name, its extension, its containing directory, its size, its modification time, or any assertion made by the caller.
- **FR-003**: Identification MUST recognise a text log by any record-kind literal that may legally open one, followed by a field separator. `ASSERTION` is the ordinary case — Gatling writes assertion records ahead of the run header, and both corpus recordings begin with one — so a rule that recognises only `RUN` is wrong and MUST NOT be used.
- **FR-004**: Identification MUST leave the stream readable from its first byte. A caller MUST NOT have to reopen, rewind or seek to hand the log to a codec afterwards, and the bytes identification examined MUST still be delivered to that codec. This is a correctness requirement, not a convenience: the binary codec reconstructs a string cache from the start of the file and is wrong if a byte is missing.
- **FR-005**: The number of bytes identification examines MUST be fixed and MUST NOT grow with the size of the file.
- **FR-006**: Input too short to identify, including empty input, MUST be refused with an error saying so, distinguishable by a program from a file that was long enough and matched neither format.
- **FR-007**: A file matching neither format MUST be refused with an error reporting what was found at the start of the file. No layout MUST be guessed for it, and it MUST NOT be handed to either codec on the chance that it might work.

**Dispatch**

- **FR-008**: A caller MUST be able to hand over a `simulation.log` once and receive a reader for it, without knowing in advance which format or version wrote it.
- **FR-009**: When the format is recognised but no codec for it exists in this module yet, the read MUST be refused with an error naming the format and stating that it is not yet readable. It MUST NOT be reported as a damaged log, an unknown format, or an unsupported version.
- **FR-010**: The three refusals — unknown format, known format with no codec, and known format with an unsupported version — MUST be distinguishable by a program without matching on message text.
- **FR-011**: Records obtained through the identifying entry point MUST be identical, field for field, to those obtained by handing the same log directly to the codec for its format.

**Version policy**

- **FR-012**: The version policy MUST exist once, be applied by every Gatling codec, and be the only place the three outcomes are decided. A codec MUST NOT carry its own copy of the rule.
- **FR-013**: A log whose version is below the lowest version covered by that format's golden corpus MUST be refused with an error naming both the version found and the lowest supported one, and MUST yield no records.
- **FR-014**: A log whose version lies inside the covered range MUST decode with no warning.
- **FR-015**: A log whose version is above the covered range MUST decode and MUST raise exactly one warning, naming the version found and the range no recording covers it in. The warning MUST reach the caller as a value in the result; it MUST NOT be delivered only through a log line, a printed message or a callback the caller can miss.
- **FR-016**: Exactly one warning MUST reach the caller for one above-range log, however many layers handled it. Identification, dispatch and the codec MUST NOT each raise their own.
- **FR-017**: A version string that is not a plain `MAJOR.MINOR.PATCH` release MUST be refused with the string quoted back, whatever its numeric part would have gated to. This restates the rule spec 002 established and MUST NOT be weakened by anything in this feature.
- **FR-018**: The policy MUST be applied before any record is decoded, so that a refusal never arrives after records have been delivered.

**Strictness**

- **FR-019**: A caller MUST be able to request strictness for a read. In strict mode a version above the covered range MUST be refused with an error naming the version found and the range recordings cover, instead of decoded with a warning.
- **FR-020**: Lenient MUST be the default: a caller that asks for nothing gets the decode-and-warn behaviour of FR-015.
- **FR-021**: Strictness MUST NOT change any other outcome. A version inside the range reads identically strict or lenient; a version below the range is refused either way; a non-release version string is refused either way. Strictness only ever tightens the gate.
- **FR-022**: The strict refusal MUST be distinguishable by a program from the below-range refusal, because the two describe opposite evidence gaps — one version is older than anything recorded, the other newer.

**Introspection**

- **FR-023**: The supported version range MUST be readable programmatically, per format, so that a consumer can report what parsec accepts without running a decode and without hard-coding a range of its own.
- **FR-024**: The advertised range MUST equal the range that format's golden corpus covers, and MUST NOT be settable or widenable by a caller.
- **FR-025**: A format that is known but has no codec yet MUST be reportable as exactly that — a third answer, distinct from a supported format and from an unknown one.
- **FR-026**: A consumer MUST be able to enumerate the formats this module knows about without naming them itself, so that a format gaining a codec becomes visible to consumers without their changing code.

**Errors**

- **FR-027**: The error and warning types this feature introduces MUST be usable by both codecs, so that the binary codec arriving in v0.0.5 reports a refused version, an unverified version and an unreadable format in the same words and the same shapes as the text codec does.
- **FR-028**: No input MUST be able to make identification or dispatch panic — empty, truncated, binary, randomly mutated, or a stream that fails mid-read. A failing underlying stream MUST surface as an error that names the failure rather than as a misclassification.

**Evidence**

- **FR-029**: Every outcome named in Source Coverage — text identified, binary identified, neither, too short, empty, below range, in range, above range lenient, above range strict, non-release string, known format without a codec — MUST have at least one automated check asserting it.
- **FR-030**: Identification MUST be proved against the real recordings the project holds, not only against hand-written bytes. Any hand-written input used to exercise a case no recording can produce MUST be named as a fixture rather than as corpus, so a later reader cannot mistake an edited file for a real recording.
- **FR-031**: A binary sample sufficient to prove that a binary log is identified as binary MUST be obtained from a real Gatling 3.13.0-or-newer run rather than constructed by hand, because a constructed sample proves only that identification agrees with whoever wrote the sample. One such sample is enough for this feature; the complete recorded runs belong to v0.0.5, which needs them for its decoder.
- **FR-031a**: That sample MUST be recorded as the leading bytes of a real binary `simulation.log`, MUST record which Gatling release produced it and how it was captured, and MUST be named so that no later reader can mistake a truncated sample for a complete recorded run. It is evidence for identification and for nothing else — it MUST NOT be presented as, or counted as, a corpus entry for the binary format.
- **FR-031b**: This feature MUST NOT claim that a binary log's version is identified. Nothing here reads a version out of a binary header, so the supported range advertised for the binary format is empty until its codec lands, and FR-025's "known format, no codec yet" is the whole of what a consumer is told about it.
- **FR-032**: The text codec's observable behaviour for every case it already handles MUST be unchanged by this feature. Moving it onto the shared policy is a refactor at the codec's boundary, not a change to what a caller sees.

### Key Entities *(include if feature involves data)*

- **Log format**: which of the two on-disk shapes a `simulation.log` has — the tab-separated text log written through Gatling 3.12.0, or the binary stream written from 3.13.0. A third value covers a file that is neither. It is a property of the bytes, never of the file's name.
- **Identification outcome**: the format decided for a stream, together with the stream still readable from its first byte. For a file that is neither format, an error saying what was found instead.
- **Version verdict**: what the policy decided for a version — refused as too old, accepted, or accepted-but-unverified — carrying the version found and the range that decided it. Strictness turns the third into a refusal.
- **Strictness**: a caller's declaration that an unverified result is worth less than no result. It affects exactly one verdict and never loosens any.
- **Codec coverage**: for a known format, either the version range its codec accepts without a warning, or the statement that no codec for it exists yet. This is what a consumer reports.
- **Unreadable-format failure**: a file that is not a Gatling `simulation.log`, or is one in a format this module cannot read yet. The two are different answers and are never collapsed into one.

### Source Coverage *(include if the feature reads a tool artefact)*

- **Tool and versions**: Gatling. Identification covers both formats the tool has written: text through 3.12.0, binary from 3.13.0. Decoding, in this module today, covers 3.11.5 and 3.12.0 only.
- **Artefact formats**: the text `simulation.log` (tab-separated, opening with `ASSERTION` or `RUN`) and the binary `simulation.log` (3.13.0 and newer). Everything else is refused as neither.
- **Version gate**, six outcomes: below the covered range — refused, naming the version found and the range. Inside the range — decoded, no warning. Above the range, lenient — decoded, one warning. Above the range, strict — refused, naming the version and the range. Not a plain release number — refused, quoting the string. No readable header — refused, because nothing can be gated.
- **Format gate**, three outcomes: text — handed to the text codec. Binary — refused as a known format with no codec yet, until v0.0.5. Neither — refused, naming what was found.
- **Not provided by this feature**: nothing about the log's *contents*. Identification reads the opening bytes and the version, and reports no count, timing, name or outcome. It does not validate that the rest of the file is well formed; that is the codec's job and stays there.
- **Golden corpus**: the existing `testdata/corpus/gatling/3.11.5/` and `3.12.0/` entries prove text identification, and their opening `ASSERTION` line is what proves FR-003 — no new text recording is needed. Binary identification is proved by one sample of the leading bytes of a real 3.13.0-or-newer `simulation.log` (FR-031a), which is evidence for identification and is not a corpus entry: it holds no complete run and no report, and nothing may compare a decoder against it. The complete binary recordings — 3.13.5, 3.14.9 and 3.15.1, each with the report that run produced or an explicit record that the version produced none, as Gatling stopped from 3.13.5 — belong to v0.0.5 together with the codec they prove.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every corpus `simulation.log` the project holds is identified as its true format, with zero misclassifications, and no identification consults a file name.
- **SC-002**: A binary `simulation.log` handed to parsec produces an error naming the binary format and the absence of a codec — in 100% of attempts, and in no attempt a syntax error about the log's first line.
- **SC-003**: Every one of the six version outcomes and three format outcomes in Source Coverage is asserted by at least one automated check, and each check fails when the corresponding rule is removed.
- **SC-004**: An above-range log read leniently yields exactly one warning, counted, through every path a caller can take to read it — directly through a codec or through the identifying entry point.
- **SC-005**: The same above-range log read strictly is refused, and a log inside the range yields identical records read strictly or leniently, byte for byte.
- **SC-006**: The range a consumer reads programmatically equals the range the golden corpus covers, checked automatically; a test that widens the corpus without widening the range fails, and so does the reverse.
- **SC-007**: A randomised robustness run over mutated, truncated and arbitrary inputs completes with only classifications and errors and no crash, across at least 10,000 inputs.
- **SC-008**: Identification examines a bounded, fixed number of bytes, and the measured figure does not change when the input is made a thousand times larger.
- **SC-009**: Every record obtained through the identifying entry point over the whole corpus is identical, field for field, to the record obtained by using the codec directly — zero differences.
- **SC-010**: Every test that passed against the text codec before this feature still passes after it, unchanged.
- **SC-011**: Automated tests exercise at least 90% of the identification and policy code and at least 80% of the project overall.

## Assumptions

- **The first byte alone is not enough, and the corpus is what says so.** Issue #5 proposes detecting by first byte — `0x00` for binary, `'R'` for the text `RUN` line. The second half is falsified by this project's own recordings: both `testdata/corpus/gatling/3.11.5/simulation.log` and `3.12.0/simulation.log` begin with `A`, because Gatling writes an `ASSERTION` record for each declared assertion ahead of the run header. Identification therefore keys on the record-kind literals that may legally open a text log, followed by the field separator, rather than on a single letter. The issue's acceptance is unaffected — the logs it names still identify correctly — but its proposed mechanism is not, and issue #5 should be corrected.
- **`0x00` as the binary opening byte is a claim, not yet evidence.** It comes from issue #6's reading of the binary layout: records open with a kind byte and `0` is the run record. Nothing in this repository has ever read a binary log. Whether a binary `simulation.log` truly begins with that byte — whether there is a preamble, a length, or anything else ahead of the first record — is settled by a recording and by nothing else. Where a recording disagrees with this spec, the recording wins and the spec is corrected.
- **The version policy already half exists and is being completed, not invented.** The refuse/accept/warn decision, the version type, and the refusal and warning shapes live in `gatling/` today and are already shared; the text codec calls them. What this feature adds is strictness, format identification, dispatch, and a module-level view of coverage — and it moves the text codec onto the completed policy so that the binary codec inherits the same behaviour rather than a second implementation of it.
- **The text codec's constructor grows an option, and that is an API change to be approved.** Strictness has to be expressible where a read begins. The intended shape is an optional argument that leaves every existing call site compiling and every existing default behaviour identical, with the change recorded in `CHANGELOG.md` as Principle V requires before v0.1.0. `AGENTS.md` lists changing a public API signature as an ask-first item, so the plan states the exact signature and it is approved before implementation.
- **Dispatch cannot live in `gatling/`.** `gatling/text` imports `gatling`, so `gatling` cannot import a codec back without a cycle. Issue #5 places "format detection and the version policy" in `gatling`, which is right for the parts that carry no codec dependency; the entry point that hands a stream to a codec must therefore sit elsewhere. Where exactly is a design decision for the plan, not a requirement here — this spec states only what the caller gets.
- **Identification does not validate the log.** It reads the opening bytes and stops. A file that opens like a text log and is garbage afterwards is identified as text and then fails in the codec with a syntax error naming its line. That is the correct division: identification answers "which reader", the reader answers "is it intact".
- **The binary format is identified but not read, in this milestone.** The binary codec is v0.0.5 and depends on this one. Until it lands, a binary log's honest outcome is a refusal naming the format — which is already the improvement issue #5 asks for, since the alternative today is a syntax error about a missing tab.
- **The version a binary log names is not read here, and this narrows issue #5.** Reaching it means decoding the binary run header far enough to reach a string, which is the binary codec's own work and needs the binary codec's own evidence. Issue #5's acceptance bullet — "given logs from 3.11.5, 3.12.0, 3.13.5, 3.14.9 and 3.15.1, then format and version are identified correctly for each" — is therefore met for **format** in all five cases and for **version** in the two this module can decode; the remaining three arrive with their codec in v0.0.5. This was decided rather than assumed (see Clarifications) and issue #5 should be amended to say so, in the same way spec 002 amended issue #3.
- **Deferring the binary recordings is a judged risk, not an oversight.** Constitution Principle III makes a recording irreplaceable — a run's own report is captured at recording time or never — so postponing three recordings postpones something that cannot be recovered if those Gatling releases become hard to run. The risk is accepted because v0.0.5 cannot ship without exactly those recordings, so the deadline is one milestone away rather than open-ended, and because recording them here would put a decoder's worth of binary-layout work inside a milestone whose subject is identification. If v0.0.5 slips far, this is the assumption to revisit first.
- **The floor is 3.11.5 because that is where the evidence starts.** Issue #5 states the floor directly, and it coincides with the oldest version the golden corpus covers. The two agree today; if they ever disagree, the corpus wins, because Principle II binds the gate to the corpus and not to a number anyone wrote down.
- **Locating the log is out of scope.** Finding a run directory, reading `lastRun.txt`, and knowing where Maven and Gradle put results is milestone v0.0.10 (issue #11). This feature is handed an already-opened stream.
- **Statistics are out of scope, as everywhere in this module.** Nothing here counts, averages or summarises. Identification and the version policy produce a classification and a verdict.
- **Reading a growing log is out of scope.** Identification assumes the bytes it examines are already written; a log still being appended to is milestone v0.0.9 (issue #10).
- **This builds on spec 002 and spec 003.** The text codec, the version type, the gate, the shared errors and the canonical model are in place. This feature adds no record kind, no model field and no capability.
