# Feature Specification: An incomplete record

**Feature Branch**: `008-incomplete-record`

**Created**: 2026-09-07

**Status**: Draft

**Input**: User description: "https://github.com/galax-io/parsec/milestone/6"

**Milestone**: v0.0.8 — An incomplete record (parsec#7, #10)

---

## Context

Both codecs read a file that has stopped changing. When the bytes run out in the middle of a
record, the read ends with the same fatal error the codecs raise for bytes that are not a Gatling
log at all, and that ending is terminal by design: the reader documentation states that the records
already delivered "are not a result: no total may be derived from them". A run that was killed
therefore reports nothing whatsoever.

That is not an unusual file. Gatling writes `simulation.log` through a buffer flushed in blocks, so
a test that is stopped — Ctrl-C, an OOM kill, a CI timeout, a full disk — leaves its last record
half-written. The shape appears exactly when a run failed and an operator most wants to see how far
it got, and today the only account of it is the console scrollback.

A half-written record is also what a reader sees while a test is still running, which is why this
milestone holds two issues. The second turns out to be far smaller than it first looked:

1. **#7 — the bytes stopped for good.** Everything decoded before the cut is true and should be
   handed over; the fact of the cut must reach the caller; and a log that is *damaged* rather than
   *cut short* must still be refused.
2. **#10 — the bytes have not stopped.** Measured on 2026-09-07 against all five corpus recordings,
   both codecs: the existing readers already decode a source that blocks between appends to exactly
   the record stream a whole-file read yields, with no change to any read loop — the string cache,
   the group path and the version gate already live across records. What is missing is not code but
   a **contract**. Nothing states that a blocking source is supported and no test holds it, so the
   sidecar (comet#3) builds on an accident that the next change to a buffer can remove in silence.

Whether the writer is still alive is not this module's to know, and does not become so here:
polling, run discovery, cancellation and the judgement that a run has ended belong to the sidecar.
The binary format makes that division permanent — it carries no end marker, so a cut landing exactly
on a record boundary is a shorter valid log and cannot be told apart from a complete one by any
reader, however careful. Only something that can see the writer can tell the difference.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A killed run still reports what it recorded (Priority: P1)

An operator's hour-long test is stopped at minute fifty — by a timeout, an OOM kill, or a hand on
Ctrl-C. The log ends inside a record. Reading it yields every request, group, user and error the
file completed, together with the plain statement that the log was cut short and how much of it
could not be decoded. Nothing is invented, and nothing usable is thrown away.

**Why this priority**: This is the failure the milestone exists for. Today the whole run is lost at
the moment its data matters most, and the operator falls back to console scrollback. It is also the
half of the milestone that cannot be moved anywhere else: the cut is detected inside the read loop
and is invisible from outside it.

**Independent Test**: Cut a corpus log at an offset inside its final record, read it, and confirm
the records before the cut arrive and the cut is reported. Delivers the whole of #7 with nothing
else in this feature built.

**Acceptance Scenarios**:

1. **Given** a corpus log cut inside its final record, **When** it is read, **Then** every record
   that decoded completely before the cut is delivered in file order, and the read reports that the
   log was cut short.
2. **Given** the same cut log, **When** the read ends, **Then** the caller can learn how many
   trailing bytes were left undecoded and where the incomplete record began.
3. **Given** an intact corpus log, **When** it is read, **Then** no cut is reported and the record
   stream is exactly what the previous release produced.
4. **Given** a log cut before its run header decoded, **When** it is read, **Then** the read fails:
   with no header there is no run, no version verdict and no records to salvage.
5. **Given** a consumer written against the previous release, **When** it reads a cut log, **Then**
   it does not mistake the run for a complete one.

---

### User Story 2 - A running test can be followed on a stated promise (Priority: P2)

The sidecar attaches to a run that is still being written, hands the reader a source that blocks
while there are no new bytes, and receives each record as it completes. It does so against a
documented guarantee and a test that fails if a later change breaks it — not against behaviour that
happens to work today.

**Why this priority**: Live following is what comet#3 is built on, and it works now. The value here
is preventing its silent loss, which is worth less than restoring a lost run but more than anything
else in the milestone. It also settles the shape of the division of labour before the sidecar is
written against a guess.

**Independent Test**: Deliver a corpus log to the reader in small pieces with waits in between, from
a source that blocks rather than reporting the end, and compare the result with a whole-file read.
Passes or fails on its own, with #7 unbuilt.

**Acceptance Scenarios**:

1. **Given** a corpus log delivered in 300-byte pieces by a source that blocks between them, **When**
   it is read to the end, **Then** the record stream is identical to a whole-file read of the same
   log, for every supported version and both formats.
2. **Given** a record whose bytes arrive across two separate appends, **When** the rest arrives,
   **Then** the record is delivered exactly once and is not duplicated or lost.
3. **Given** a follower waiting between appends, **When** the wait is long and the log already long,
   **Then** memory held does not grow with the records already delivered.
4. **Given** the published documentation, **When** a consumer reads it, **Then** it states that a
   source whose reads block is supported and what the end of input means.

---

### User Story 3 - A damaged log is still refused (Priority: P3)

A file that is corrupt rather than cut short — a record kind the format never defined, a length
prefix claiming gigabytes, bytes that are not a Gatling log at all — still fails the read with the
offset that failed. Salvage does not become a licence to guess.

**Why this priority**: It protects the value of the other two stories. A consumer that cannot tell
"the run was cut short" from "this file is not decodable" cannot act on either; treating corruption
as a short run would put invented results into a report, which is worse than reporting nothing.

**Independent Test**: Feed logs whose defects are not truncation and confirm each still fails,
naming its offset, and that none is reported as a cut-short run.

**Acceptance Scenarios**:

1. **Given** a log containing a record kind the format does not define, **When** it is read, **Then**
   the read fails naming the offset, and the failure is not reported as a cut-short log.
2. **Given** a length prefix larger than the codec will allocate, **When** it is read, **Then** the
   read fails without attempting the allocation, as it does today.
3. **Given** a source that fails with an error merely wrapping an end-of-input condition, **When**
   it is read, **Then** the failure is reported as a failure and never as the clean end of a log.

---

### Edge Cases

- **A cut exactly on a record boundary (binary).** The format has no end marker, so the file is a
  shorter valid log and is indistinguishable from a complete one. It decodes cleanly and reports no
  cut. This is documented as a property of the format, not treated as a defect to detect.
- **A text log whose final line has no terminator.** That is the text form of the same cut: the
  records before it are delivered and the cut is reported.
- **A log cut inside the preamble.** Assertions precede the header; a cut among them leaves no
  header, so the read fails (US1 scenario 4).
- **A log holding only a header.** Zero records, a clean end, no cut reported.
- **A version above the supported range, in a cut log.** Both the version warning and the cut reach
  the caller; neither hides the other.
- **A cut of length zero.** An empty file is not a Gatling log and fails as one, unchanged.
- **A cut at every offset in the last 200 bytes.** No offset panics, and each yields the records
  that fit.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Both codecs MUST deliver every record that decoded completely before the bytes ran
  out, in file order.
- **FR-002**: A read that ended inside a record MUST be distinguishable by the caller from a read
  that ended cleanly, and from a read that failed because the artefact is not decodable.
- **FR-003**: A cut-short read MUST report how many trailing bytes were left undecoded, and the
  position at which the incomplete record began — the byte offset for the binary format, the line
  number for the text format.
- **FR-004**: The signal that ends a cut-short read MUST differ from the signal that ends a clean
  one, so that a consumer written before this change treats a cut run as a failure rather than
  silently as a complete run.
- **FR-005**: A defect that is not truncation — an undefined record kind, an over-large length
  prefix, an unparseable field — MUST still end the read with an error naming its position, and MUST
  NOT be reported as truncation.
- **FR-006**: A failure of the source that is not the end of the artefact, including one that merely
  wraps an end-of-input condition, MUST NOT be reported as the end of the log or as truncation.
- **FR-007**: Bytes that ran out before the run header decoded MUST remain a failed read: no header
  means no run, no version verdict and no records.
- **FR-008**: A source whose reads block while the artefact is still being written MUST be
  supported, and a record split across separate reads MUST be delivered exactly once, when complete.
- **FR-009**: The blocking-source contract MUST be stated in the documentation a consumer reads, not
  only exercised by a test.
- **FR-010**: This module MUST NOT poll, watch, reopen or otherwise infer whether the writer is
  still alive; the end of input is the caller's statement, and this module acts on it as given.
- **FR-011**: Memory held MUST stay bounded while a reader waits between appends, and MUST NOT grow
  with the number of records already delivered.
- **FR-012**: Both reader shapes MUST carry the cut-short signal — the wire records and the
  canonical run items alike. A consumer of the canonical model MUST NOT have to drop to the wire
  records to learn that a run was cut.
- **FR-013**: No input MUST cause a panic, at any truncation offset, in either codec.
- **FR-014**: Records delivered before a cut MUST be exactly the records a whole-file read of the
  same prefix delivers; truncation MUST NOT change how any complete record decodes.

### Key Entities

- **Cut-short report**: what the caller learns about a read that ran out of bytes inside a record —
  that it happened, where the incomplete record began, and how many trailing bytes went undecoded.
  It travels with the result and is never only logged.

### Source Coverage

- **Tool and versions**: Gatling 3.11.5 and 3.12.0 (text); 3.13.1, 3.14.9 and 3.15.1 (binary).
  Unchanged by this feature.
- **Artefact formats**: text and binary `simulation.log`. Both are affected identically; a cut is a
  cut in either.
- **Version gate**: unchanged. A version below the range is still refused before any record decodes,
  and a version above it still decodes with a warning — including in a cut log.
- **Not provided by this source** (declared through Capabilities): unchanged. Additionally, whether
  a binary log ended by design or by a cut on a record boundary is not knowable from the artefact
  and is therefore not reported.
- **Golden corpus**: no new recordings. Cut inputs are derived at read time from the five existing
  recordings; a hand-cut log is a fixture and not corpus (Principle III), so none is committed under
  `testdata/corpus/`. The growing-log input is a corpus log delivered over time by a blocking
  source.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Each of the five corpus logs, cut at every offset within its last 200 bytes, delivers
  every record that fits, reports the cut, and never panics — 1000 cut positions in total.
- **SC-002**: Every intact corpus log reports no cut, and its decoded record stream is byte-for-byte
  what v0.0.7 produced.
- **SC-003**: Each corpus log delivered 300 bytes at a time by a source that blocks between appends
  yields a record stream identical to a whole-file read, for all five versions and both formats.
- **SC-004**: A consumer written against v0.0.7 does not report a cut run as complete: the ending of
  a cut-short read is distinguishable from a clean end without reading any new documentation.
- **SC-005**: Peak memory over the largest corpus log is unchanged from the recorded v0.0.7 figure,
  and a follower that waits between appends holds no more than its fixed buffers.
- **SC-006**: Coverage floors hold: 90% for the decoder packages, 80% for the module.
- **SC-007**: The sidecar follows a live run through the same entry point a whole-file consumer
  uses; no second decoding interface is added to the public surface.

## Assumptions

- The polling loop, run discovery, cancellation, and the judgement that a run has ended belong to
  the sidecar (comet#3, comet#4). This module is told where the bytes end and never infers it.
- Records delivered before a cut are complete and true. Whether a run whose log was cut may be used
  at all is the consumer's policy; this feature only makes that decision possible, and computes
  nothing over the partial stream (Principle I).
- The signal ending a cut-short read is chosen to fail safe: a consumer that does not know about it
  sees a failure, as it does today, rather than a silently shorter run. This is a change in
  observable behaviour, **approved 2026-09-07**, permitted before v0.1.0 and recorded under Changed
  in `CHANGELOG.md` (Principle V); the freeze that #13 asks for comes after.
- No new public parser interface is added. #10 is settled as contract plus test, as its rewritten
  issue and comet#3's first direction both state; a push-style parser fed byte slices is rejected.
- The exact block size Gatling flushes through is not a requirement of this feature; only that an
  aborted run ends inside a record.
- Existing corpus recordings are sufficient. No Gatling run needs to be recorded for this work.

## Dependencies

- **parsec#7** (a truncated log) and **parsec#10** (the blocking-source contract) are both open in
  this milestone and neither depends on the other. #7 owns the detection and the salvage; #10 states
  the contract around it and adds no detection of its own.
- **comet#3** depends on #10's contract and is not blocked by this repository's schedule beyond it.
- No new module dependency (Principle IV): the standard library only.

## Out of Scope

- **Repairing a damaged log**, or recovering a record whose bytes are missing. What was not written
  is gone.
- **Watching the filesystem, discovering run directories, and deciding when a run has ended.** Those
  are the sidecar's (comet#3, comet#4) and parsec `v0.0.9` (#11).
- **A push-style parser fed byte slices.** Rejected in #10 and in comet#3: a second public decoding
  interface to freeze at v0.1.0, buying nothing the existing readers do not already do.
- **Any statistic over a partial record stream.** This module computes none at all (Principle I);
  what a consumer may derive from a cut run is `galaxio-cli`'s decision.
- **Distinguishing a binary log that ended by design from one cut on a record boundary.** The format
  makes this impossible from the artefact alone.
- **New corpus recordings or a widened version gate.** The supported range is untouched.
