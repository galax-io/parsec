# Feature Specification: Pre-freeze hardening

**Feature Branch**: `010-pre-freeze-hardening`

**Created**: 2026-09-10

**Status**: Draft

**Input**: User description: "только эти задачи #104 #106 #87 #82 #76 #75 #83 #102 #88" — *only these
issues*: nine of the twenty-nine then in milestone v0.1.0.

**Milestone**: v0.0.10 — Pre-freeze hardening (parsec milestone #19). Its nine issues were filed
against v0.1.0 — A stable API (#11) and ship first, ahead of the freeze; see *Out of Scope* for the
rest of v0.1.0.

---

## Context

v0.1.0 turns documentation into contract. From that tag, the signature and the observable behaviour
of every exported identifier — the error a read returns, the memory it holds, the run it picks — are
what Principle V freezes, and moving any of them afterwards costs a Changed entry, a deprecation
window and a MINOR release. Before the tag they cost a commit.

A max-effort review of the surface about to be frozen found nine places where the documentation and
the code disagree, and where the documentation is right. None is a missing feature: each is a promise
already made in a doc comment, a package overview or the changelog that the code does not keep, and
each is cheap now and expensive in a month. They fall into four groups.

**What the binary reader says about its memory is not what it enforces** (#75, #87, #76).
`binary.Reader` documents a 32 MiB peak-heap budget for any log it accepts and says its own tests
assert it. Two of the three collections it keeps for the whole read — the scenario names and the
string cache — are bounded in count and not in bytes, so a 48 MiB log of distinct 1 MiB strings
retains 49 MiB (#75). Its "fixed read buffer" is the caller's whenever the caller hands over a
buffered reader of its own, so a 64 MiB buffer becomes the codec's without anyone choosing it (#87).
And the version gate — the cheapest decision the codec makes, and the one Principle II says comes
first — runs after the entire run record has been read and retained: a log that will be refused still
costs whatever its scenario and assertion tables claim, and a corrupt count after an out-of-range
version is reported as damage by this codec and as a version by the other (#76).

**What the readers say about the end of a stream is sometimes false** (#82, #83, #102). A source that
returns its final bytes together with `io.EOF` in one call — which the `io.Reader` contract permits
and an HTTP body does — makes a complete, intact binary log come back as a truncation claiming every
byte was lost and zero records delivered (#82). The `simlog` package overview promises a follower that
a failure of the source is reported as a failure, "including one that merely wraps io.EOF, and never
as the end of the log", and breaks that promise twice: a wrapped `io.EOF` keeps its chain, so
`errors.Is(err, io.EOF)` is true for a torn upload (#83); and a source's own `io.ErrUnexpectedEOF` —
what a truncated gzip returns by identity — is reported as "not enough bytes yet", the answer a
follower retries on (#102). The Galaxio backend ingests from an HTTP request body; these are
precisely its source shapes.

**Which run is found depends on the order a directory is listed in** (#88). The tie-break `run.Find`
uses when every run shares a modification time — which is every clone, `rsync` and CI cache restore,
and the only rule most callers ever reach — is not transitive once a root mixes Gatling-stamped names
with renamed ones. Adding one unrelated directory changes the answer from the run stamped 2099 to the
run stamped 2020, and the caller is told the ordinary rule applied.

**The freeze needs a mechanism and a supported toolchain** (#106, #104). Nothing failed a pull
request that renamed an exported identifier; that gate has since landed (PR #109, merged 2026-09-10)
and is recorded here so that it is verified rather than rebuilt. And CI certifies the module on the
last patch of a Go line that no longer receives security fixes, because `go.mod` pins only the
consumer floor; the toolchain directive that fixes it landed in PR #110 while this feature was being
implemented, and this spec verifies it rather than duplicates it.

What lands here is small and entirely about honouring what is already written down. Nothing gains a
field, an option or an entry point; a few doc comments gain a sentence; `CHANGELOG.md` gains a
Changed entry for each behaviour a consumer could observe moving.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A complete upload is never reported as a killed run (Priority: P1)

The Galaxio backend receives a binary `simulation.log` as an HTTP upload and decodes it straight from
the request body. The log is complete and intact. The reader delivers every record and ends cleanly,
exactly as it would reading the same bytes from a file — however the transport splits its reads, and
whether or not it announces the end of the stream in the same call as the last bytes.

**Why this priority**: A truncation is a positive claim — "the log was cut short, and what was
delivered is exactly what an intact log would have given" — and the documented consumer response is
to keep what arrived and report a run that was killed. Today that claim is made about a whole,
undamaged run, with zero records, from the exact source shape the backend uses. That is wrong data
reaching users, which ranks above every other finding here.

**Independent Test**: Take a complete binary log whose last field is at least 64 KiB, read it through
a source that returns its final bytes together with `io.EOF`, and compare the record stream with a
whole-file read of the same bytes. Delivers its value with nothing else in this spec built.

**Acceptance Scenarios**:

1. **Given** a complete binary log whose trailing field is at least the codec's read-buffer size,
   **When** it is read through a source that returns its last bytes and `io.EOF` in one call, **Then**
   the read succeeds and yields records identical to those a plain byte reader yields for the same
   bytes.
2. **Given** the same log with a trailing field smaller than the read buffer, **When** it is read
   through the same source shape, **Then** the outcome is the same: the fix does not depend on where a
   field falls against the buffer.
3. **Given** a binary log genuinely cut inside a record, **When** it is read through either source
   shape, **Then** a `*gatling.TruncationError` is still returned and the records before the cut are
   still delivered.
4. **Given** the whole golden corpus, **When** every entry is decoded after the change, **Then** every
   record stream is unchanged.

---

### User Story 2 - A broken source is a failure, never an ending and never "not yet" (Priority: P1)

The same backend handler, and the comet sidecar following a log it did not finish receiving, classify
what the reader returns: the end of the log, a log cut short, a log that has not arrived yet, or a
failure of the transport. Every constructor of this module — `simlog`'s two and each codec's own —
gives the same honest answer for the same broken source: a failure, carrying the cause's text, that no
`errors.Is(err, io.EOF)` check mistakes for the end of the log and no short-head check mistakes for a
stream that needs more bytes.

**Why this priority**: The `simlog` package overview promises exactly this, in the list of what a
follower may rely on. It is the package a caller is told to prefer, and identification runs before any
record is decoded, so there is nothing else in the result to contradict a wrong classification. A torn
upload booked as an empty run, or retried forever as "not yet", is a silent loss.

**Independent Test**: Hand each constructor a source that fails with an error wrapping `io.EOF`, and
one that hands over two bytes and then fails with `io.ErrUnexpectedEOF` (the shape of a gzip cut
inside the detection window), and inspect what comes back.

**Acceptance Scenarios**:

1. **Given** a source that fails with an error wrapping `io.EOF`, **When** any constructor opens it,
   **Then** the error returned does not satisfy `errors.Is(err, io.EOF)` and its message still
   contains the cause's text.
2. **Given** a source that returns two bytes and then `io.ErrUnexpectedEOF`, **When** `simlog` opens
   it, **Then** the error is not a `*gatling.FormatError`, does not satisfy `errors.Is(err, io.EOF)`,
   and names the cause.
3. **Given** a gzip of a text log cut off inside the first bytes of the detection window, **When** it
   is opened through a gzip reader and `simlog`, **Then** the outcome of scenario 2 holds.
4. **Given** a stream that genuinely ends after two bytes — `io.EOF` from the source itself — **When**
   `simlog` opens it, **Then** a `*gatling.FormatError` with `Short` set is still returned, so a
   follower still knows to come back with more.
5. **Given** the rule that no error returned for a source failure satisfies `errors.Is(err, io.EOF)`,
   **When** the test suite runs, **Then** one test asserts it across all three packages, rather than
   each package restating it in a comment.

---

### User Story 3 - The memory budget the binary reader documents holds for any log (Priority: P2)

An operator sizes the comet sidecar's container from the figure `binary.Reader` documents: peak heap
under 32 MiB for any log the codec accepts. They pass the codec whatever reader they already have —
often a buffered one — and feed it whatever a Gatling run wrote, or whatever an attacker uploaded in
its place. The sidecar is never killed for exceeding the figure, and no log that Gatling actually
writes is refused because of the bound that guarantees it.

**Why this priority**: Principle II asks a decoder for memory "bounded independently of artefact size"
on untrusted input, and the budget is stated in four places — the reader's doc, `MaxStringLen`'s, the
scenario ceiling's and the changelog. A log of about 35 MiB breaks it today, and a caller's buffer
size becomes the codec's without the caller knowing. P2 only because it takes a crafted log to
trigger; nothing Gatling writes does.

**Independent Test**: Decode a crafted 48 MiB log whose scenario names are all distinct and each at
the string ceiling, then one built the same way from string-cache entries, sampling the live heap
after collection; and hand each codec a caller-buffered reader and count the bytes left in the
caller's buffer afterwards.

**Acceptance Scenarios**:

1. **Given** a crafted 48 MiB log of distinct scenario names at `MaxStringLen`, **When** it is
   decoded, **Then** the retained heap stays inside the documented budget, because the log is refused
   as damaged once its names exceed a byte ceiling and before the budget is reached.
2. **Given** the same shape built from distinct string-cache entries introduced by request records,
   **When** it is decoded, **Then** the same holds.
3. **Given** every log in the golden corpus, and a crafted log whose distinct strings come to well
   under the ceiling, **When** decoded, **Then** none is refused and every record stream is unchanged.
4. **Given** `binary.NewReader` handed a `*bufio.Reader` with a 64 KiB buffer, **When** the run record
   has been read, **Then** the caller's reader has zero buffered bytes: the codec read through its own
   buffer, not the caller's.
5. **Given** the same for the text codec, **When** asserted by the same test, **Then** both codecs
   behave identically.
6. **Given** a binary log naming a version below the supported range, **When** it is refused, **Then**
   the bytes pulled from the source before the refusal do not depend on the size its scenario or
   assertion tables claim.

---

### User Story 4 - A refused log is refused first, and the same way by both codecs (Priority: P2)

A consumer opens an archived log through `simlog`, which exists so that it cannot tell which codec
read it. The log names a Gatling version outside the supported range. Whatever else is wrong with it,
the answer is the version error — the version found and the range supported — for a text log and a
binary log alike.

**Why this priority**: From v0.1.0 the error a read returns is observable behaviour. Today the two
codecs disagree about a log with an out-of-range version and a corrupt field after it — one says
version, the other says damage — and that disagreement becomes a contract in a month. Principle II
states the order as a MUST: the gate before any record is decoded.

**Independent Test**: Build a binary log naming version 1.0.0 with a corrupt scenario count, and its
text equivalent, and open both through `simlog`.

**Acceptance Scenarios**:

1. **Given** a binary log naming 1.0.0 whose scenario count is corrupt, **When** it is opened, **Then**
   the error is a `*gatling.VersionError`, as it is for the equivalent text log.
2. **Given** a binary log naming a version above the range, followed by a malformed field, **When** it
   is opened without strict mode, **Then** the warning is recorded and the malformed field is reported
   as it is today; **and when** it is opened with strict mode, **Then** the strict refusal comes
   first.
3. **Given** a binary log whose version string is not a release at all, **When** it is opened, **Then**
   the version error quotes what was written, whatever follows it.
4. **Given** the golden corpus, **When** decoded, **Then** every record stream, header, assertion list
   and warning is unchanged.
5. **Given** `CHANGELOG.md`, **When** the change lands, **Then** the moved precedence is recorded under
   Changed.

---

### User Story 5 - The run that is found does not depend on the order a directory is listed (Priority: P2)

An engineer's CI job restores a results root from cache — every run directory now shares one
modification time — and the root holds runs of two simulations plus a directory an archive renamed.
`run.Find` returns the run with the latest recorded start, every time, on every platform, whatever
order the filesystem lists the entries in. Adding an unrelated directory never changes the answer.

**Why this priority**: For the callers with no `lastRun.txt` — almost all of them — the tie-break is
the whole selection, and the resolution reports that the ordinary rule applied, so a wrong pick is
invisible. Reporting on a run months older than the one that just finished is the failure the
ordering exists to prevent.

**Independent Test**: Create `simA-20990101000000000`, `simB-20200101000000000` and `simAA` with
equal log modification times, resolve under every permutation of directory order, and check the
result; then shuffle random candidate sets.

**Acceptance Scenarios**:

1. **Given** that root, **When** the run is resolved under every permutation of entry order, **Then**
   `simA-20990101000000000` is returned each time.
2. **Given** any set of candidates with equal modification times, **When** it is shuffled and resolved
   repeatedly, **Then** the same run is returned every time.
3. **Given** a root whose names all carry a stamp, or none do, **When** resolved, **Then** the result is
   what it is today: latest stamp, then greatest name.
4. **Given** runs with different modification times, **When** resolved, **Then** the most recently
   modified still wins, whatever the names.

---

### User Story 6 - The freeze is enforced, and certified on a supported toolchain (Priority: P3)

A maintainer tags v0.1.0 knowing two things: that a pull request changing an exported signature
cannot merge green by accident, and that every gate that certified the module ran on a Go release
that still receives fixes.

**Why this priority**: Both are machinery rather than behaviour, and both are already moving — the
compatibility gate is on `main` (#106, PR #109) and the toolchain pin followed it there (#104,
PR #110, merged 2026-09-10). This story records them so that they are verified as part of this
feature and not built twice.

**Independent Test**: Rename an exported identifier on a branch and open a pull request; read
`go version` in the verify job's log.

**Acceptance Scenarios**:

1. **Given** a pull request that renames an exported identifier, **When** CI runs, **Then** the verify
   workflow fails and names the change.
2. **Given** the same change carrying the `breaking` label and a Changed or Removed entry, **When** CI
   runs, **Then** it passes; **and given** an unchanged tree, **Then** it passes.
3. **Given** the gate's own shell test suite, **When** the report the gate reads is empty, **Then** the
   suite fails: an empty report is a broken gate, not a clean one.
4. **Given** the verify job's log, **When** `go version` is read, **Then** it names a currently
   supported Go release, while `go.mod`'s consumer floor still reads 1.25 and `go mod tidy` leaves the
   tree unchanged.
5. **Given** a consumer building on Go 1.25, **When** they depend on this module, **Then** it builds.

---

### Edge Cases

- **A source returns the final bytes of a value together with a failure that is not the end of the
  stream.** The value is complete, and the failure is still reported as the source's failure. It is
  not left for the source to repeat: bufio hands such an error over once and forgets it, so a source
  that does not repeat itself would have a broken read taken for a complete one. (US1)
- **The trailing field lands exactly on the read-buffer boundary.** Today the defect fires only at or
  above the buffer size; the fix and its test cover both sides of the boundary. (US1)
- **A stream is empty, or ends inside the run record.** Unchanged: an empty stream is still a syntax
  error, and a cut inside the run record is still a truncation described from byte 0. (US1)
- **A source returns `(0, nil)` repeatedly.** Unchanged: the no-progress backstop still ends the read
  rather than spinning. (US2)
- **A source fails with `io.ErrUnexpectedEOF` after zero bytes.** A source failure, not an empty
  stream; the same answer as after two bytes. (US2)
- **Over-budget strings that a real log could plausibly carry.** The byte ceilings are sized so that no
  log Gatling writes trips them — scenario names are class names, and Gatling truncates failure
  messages — and the refusal names what was found and the ceiling, as the assertion ceiling's does.
  (US3)
- **A caller's buffered reader already holds bytes when handed over.** They are consumed through its
  `Read` like any other bytes; what the codec never does is fill that buffer. (US3)
- **A version above the range under strict mode, followed by a corrupt field.** The strict refusal is
  the gate's decision and precedes the field. (US4)
- **A root holding only unstamped directories, or two stamped with the same stamp.** Name order
  decides, as today. (US5)
- **A stamped and an unstamped name with equal modification times.** The stamped one is the later: a
  stamp is the only evidence of a start time, and a name without one says nothing about when. This is
  the one shape whose answer changes, and it is recorded under Changed. (US5)
- **A contributor's local Go is older than the pinned toolchain but not below the floor.** They still
  build: the pin is what CI and a toolchain-aware local build use; the floor is what the module
  requires. (US6)

## Requirements *(mandatory)*

### Functional Requirements

*Endings and failures*

- **FR-001**: A read that fills the value being read and reports `io.EOF` in the same call MUST be a
  successful read. A complete log read through such a source MUST yield exactly the records a
  whole-file read yields. (#82)
- **FR-002**: A log genuinely cut inside a record MUST still be reported as a
  `*gatling.TruncationError`, with the records before the cut delivered, whatever the source's read
  shape. (#82)
- **FR-003**: Where the binary reader's fill loop documents that it follows `io.ReadFull`'s contract,
  it MUST do so; where it differs, the difference MUST be documented instead. (#82)
- **FR-004**: No error this module returns for a failure of the source MAY satisfy
  `errors.Is(err, io.EOF)`, from any constructor; the cause's text MUST be kept in the message.
  (#83)
- **FR-005**: A source's `io.ErrUnexpectedEOF` MUST be reported as a source failure, never as a short
  head; only the stream itself ending — `io.EOF` from the source, by identity — MAY produce a
  `FormatError` with `Short` set. (#102)
- **FR-006**: FR-004 MUST be enforced by one test over all three packages (`simlog`, `binary`,
  `text`), and FR-001 and FR-005 by regression tests that fail without their fixes, including a gzip
  cut inside the detection window. (#82, #83, #102; Principle III)

*The binary reader's budget*

- **FR-007**: Every collection the binary reader retains for the life of a read MUST be bounded in
  bytes, not only in entries. The assertion payloads already are; the scenario names and the string
  cache MUST be. (#75)
- **FR-008**: The bounds MUST be consistent with the budget `Reader` documents. If that figure must
  move, it MUST move in the same change everywhere it is stated: the reader's doc, `MaxStringLen`'s,
  the scenario ceiling's and `CHANGELOG.md`. (#75)
- **FR-009**: A log that exceeds a byte ceiling MUST be refused as damaged, naming what was found and
  the ceiling; the ceilings MUST be sized so that no log Gatling writes is refused. (#75)
- **FR-010**: The read buffer between the caller's source and either codec MUST be the codec's own,
  whatever the caller passes; a caller's buffered reader MUST be left with nothing buffered by the
  codec. Both codecs MUST behave alike, asserted by one test. *Satisfied on `main` by PR #111 (merged
  2026-09-10) while this feature was open; this branch's own commit for it was dropped at rebase.* (#87)
- **FR-011**: The version gate MUST run before any field after the version string is decoded. A log
  naming a version below the supported range MUST be refused with a `*gatling.VersionError` whatever
  follows the version; both codecs MUST return the same error type for the same fault; and the bytes
  pulled before a refusal MUST NOT depend on the size of the scenario or assertion tables. The one
  exception to the same-type rule is forced by the text format: its `ASSERTION` lines come before the
  `RUN` line, so an assertion table past its byte ceiling is refused as damage before the version can
  be read. (#76; Principle II)
- **FR-012**: Regression tests MUST cover a log whose strings are all distinct (the shape no current
  memory test builds), a codec handed a caller-buffered reader, and an out-of-range version followed
  by a corrupt count, each failing without its fix. (#75, #87, #76)

*Run discovery*

- **FR-013**: The ordering that selects the newest run MUST be a total order — transitive, so the
  result never depends on the order entries are read in — while keeping the signals and their
  precedence: the log's modification time, then the run's recorded stamp, then the name, all
  descending. (#88)
- **FR-014**: A run whose stamp is newer MUST NOT lose to one whose stamp is older because a third,
  unstamped directory is present. When exactly one of two candidates carries a stamp, the stamped one
  MUST rank as the later. (#88)
- **FR-015**: A regression test MUST cover a root mixing stamped and unstamped names with equal
  modification times, under every order of reading, and MUST fail without the fix. (#88)

*The freeze's machinery*

- **FR-016**: A pull request that removes, renames or changes the type of an exported identifier MUST
  fail the verify workflow, naming the change. A deliberate breaking change MUST be possible through
  an explicit, visible override and MUST be recorded in `CHANGELOG.md`. The gate MUST add no module
  dependency and MUST be covered by the shell-gate suite, where an empty report fails. *Satisfied on
  `main` by PR #109 (commit 410dc54); kept here to be verified, not rebuilt.* (#106)
- **FR-017**: CI MUST run a currently supported Go release. The consumer floor (`go 1.25`) MUST NOT
  move, `go mod tidy` MUST leave the tree unchanged, and the constitution's toolchain statement MUST
  distinguish the floor from the toolchain CI runs. *Satisfied by PR #110 (merged 2026-09-10,
  `toolchain go1.26.8`); verified on this branch after rebasing onto it.* (#104)

*Across the feature*

- **FR-018**: Every behaviour a consumer can observe moving — the error precedence, the mixed-root
  ordering, a refusal a log could newly meet — MUST be recorded under Changed in `CHANGELOG.md` in
  the same change, and every doc comment that stated the old behaviour MUST state the new one.
  (Principle V)
- **FR-019**: Every corpus entry MUST decode to exactly the recorded record stream after every change
  here, chunked and whole-file reads MUST still agree, and the race detector MUST pass. (Principle
  III)

### Key Entities

- **End of stream**: the source itself reporting `io.EOF`, by identity, with nothing more to come. The
  only thing a reader may call the clean end of a log or, inside a record, a cut.
- **Source failure**: any other error from the source — including one that wraps `io.EOF`, and
  `io.ErrUnexpectedEOF` from a decompressor. Reported as a failure carrying its text; never as an
  ending, a cut or a short head.
- **Short head**: the answer identification gives when the stream ended while its bytes were still a
  possible opening — a follower's cue to come back with more. Produced only by the end of the stream.
- **Retained collection**: what the binary reader holds for the whole read — assertion payloads,
  scenario names, the string cache — each with a count ceiling and, after this feature, a byte
  ceiling, whose sum fits the documented budget.
- **Read buffer**: the fixed buffer between a caller's source and a codec. The codec's own, never
  adopted from the caller.
- **Ordering key**: what the newest-run rule compares — modification time, then stamp (absent for a
  renamed directory), then name — as one key, rather than a comparison rule chosen per pair.
- **Consumer floor and CI toolchain**: two different versions in `go.mod` — what a consumer needs, and
  what CI and a toolchain-aware local build run — that today are one line.

### Source Coverage

- **Tool and versions**: Gatling, unchanged — text 3.11.5 through 3.12.0, binary 3.13.1 through
  3.15.1, and the run-directory layout across the same range. No version is added or removed.
- **Artefact formats**: text and binary `simulation.log`, and the results-root layout `run.Find`
  reads. No format changes.
- **Version gate**: what it decides is unchanged. When it runs changes for the binary codec — before
  anything after the version string — and the error precedence that moves is recorded under Changed.
- **Not provided by this source**: nothing new is declared through `Capabilities`; no model field is
  added and nothing is computed.
- **Golden corpus**: no new recording. Every input this feature needs is a constructed shape — a
  source that returns bytes with `io.EOF`, a gzip cut inside the window, a 48 MiB log of distinct
  strings, a log naming 1.0.0 with a corrupt count, a mixed results root — and each is a fixture, named
  as one (Principle III), not corpus. The existing corpus is the proof that nothing else moved
  (FR-019).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every entry in the golden corpus decodes to the recorded record stream, byte for byte,
  after every change here, and chunked and whole-file reads agree.
- **SC-002**: A complete binary log with a trailing field of at least 64 KiB, read through a source
  returning its final bytes with `io.EOF`, decodes with zero truncation errors and a record stream
  identical to a plain byte reader's.
- **SC-003**: For each constructor, a source wrapping `io.EOF` and a source failing with
  `io.ErrUnexpectedEOF` each produce an error that is not the end of the log, not a truncation and not
  a short head, and that carries the cause's text — one test, three packages.
- **SC-004**: A 48 MiB log of distinct scenario names at the string ceiling, and one of distinct cache
  entries, both keep retained heap under the documented budget; every collection retained across
  records has a byte bound.
- **SC-005**: Both codecs, handed a caller's 64 KiB buffered reader, leave zero bytes in the caller's
  buffer.
- **SC-006**: A binary log naming 1.0.0 with a corrupt scenario count returns a version error of the
  same type the text codec returns; the bytes consumed before a refusal are the same whatever the
  tables claim.
- **SC-007**: The mixed root of `simA-20990101000000000`, `simB-20200101000000000` and `simAA` with
  equal modification times resolves to `simA-20990101000000000` under all six orderings, and shuffling
  any candidate set never changes the result.
- **SC-008**: The verify job's log reports a supported Go release; `go.mod`'s `go` line still reads
  1.25; `go mod tidy` is clean.
- **SC-009**: A pull request renaming an exported identifier fails verify with the change named; with
  the `breaking` label and a Changed or Removed entry it passes; an unchanged tree passes; the shell
  suite fails on an empty report.
- **SC-010**: Coverage floors hold (90% for decoder packages, 80% for the module), the race-detected
  test run passes, the standard-library-only boundary stays green, and `CHANGELOG.md` carries a Changed
  entry for each observable move.

## Assumptions

- **The 32 MiB budget is kept, not restated.** #75 offers two directions and rejects the second as the
  whole answer; this spec takes the first: byte ceilings on the scenario names and the string cache,
  sized so that the three retained collections together stay under the figure. Planning fixes the
  numbers by measurement; if measurement shows the figure must move, FR-008 moves it everywhere at
  once.
- **An over-budget log is damaged, not large.** No log Gatling writes has scenario names or distinct
  strings in the tens of megabytes, so the refusal is the one already used for a count past its
  ceiling. A consumer wanting a different ceiling is a deliberate option, not this feature.
- **Stamped outranks unstamped on equal modification time.** The natural total order is a key of
  modification time, stamp-or-empty and name; an empty stamp ranks lowest, so a renamed directory
  yields to any Gatling-named run. This changes the answer only for a root mixing the two — which
  today is either wrong or accidental — and is recorded under Changed.
- **Error precedence may move before v0.1.0.** Principle V allows an observable change before the
  freeze with a Changed entry and no deprecation window; that is the point of doing it now.
- **A filled read that ends the stream succeeds, as it does for `io.ReadFull`; one that arrives with
  any other failure is reported as that failure.** `io.ReadFull` drops such a failure and relies on
  the source to repeat it. Nothing here can rely on that: bufio hands the error over once and forgets
  it on the path a large value is read by.
- **#106, #104 and #87 are done on `main`.** The compat gate (PR #109), the toolchain pin with its
  constitution amendment and spec-001 note (PR #110) and the buffer-ownership fix with its two-codec
  test (PR #111) all landed there; this feature verifies them and re-plans none.
- **Fixes for #83 and #102 land in place.** #81 (deduplicating the two fill loops) is in milestone
  v0.2.0; #82 is fixed first, then #83 and #102 in `simlog` as it stands, so that a later extraction
  inherits correct behaviour from both copies.
- **The standard library is enough.** Nothing here needs a dependency, and `gatling/` may take none
  (Principle IV).
- **No new corpus recording is required.** Every triggering input is a constructed fixture; the
  existing corpus proves nothing else changed.

## Dependencies

- **PR #109 (merged)** — the compat gate for #106, on `main` at 410dc54; SC-009 re-verifies it.
- **PR #110 (merged 2026-09-10)** — `toolchain go1.26.8` for #104, with the constitution amendment to
  v2.3.0 and the spec-001 note; FR-017 is satisfied.
- **PR #111 (merged 2026-09-10)** — the buffer-ownership fix for #87 with a two-codec test; FR-010 is
  satisfied, and this branch's own commit for it was dropped at rebase.
- **The live-heap measurement is already on `main`.** `gatling/binary/memory_test.go` samples the
  heap after a collection (`liveHeap`, a 32 MiB `budget`, a 1 MiB `drift`), which is the instrument
  #75's regression tests use. The worktree branch `test/peak-memory-collector-allowance` that once
  carried that change is behind `main`, not ahead of it, and nothing waits on it (plan, research R12).
- **Ordering among the issues**: #82 before #83 and #102 (the two fill loops must be behaviourally
  equal before #81 folds them, which is out of scope here); #87 together with #82, since the caller's
  buffer size decides when #82's trigger fires; #76 alongside #75, since gating late is what makes a
  refused log cost its tables. #76's parity requirement reaches the text codec too: its header parser
  judges the run start before the version today, so the version is judged first on both sides (plan,
  research R5).
- **A constitution amendment** rides with #104 in PR #110, not with this spec.

## Out of Scope

- **Every other issue in milestone v0.1.0**: #13, #58, #77, #78, #79, #84, #85, #86, #93, #95, #96,
  #97, #99, #101, #103 and #107. The user selected these nine; the rest are specified separately.
- **#81**, the deduplication of the two fill loops, which is in milestone v0.2.0. #82, #83 and #102 are
  fixed in place so that whichever copy survives is correct.
- **Changing `MaxStringLen`, the read-buffer size, or letting a caller choose either.** #75 and #87
  both name this a non-goal; a chosen buffer is an option, not an accident of an argument's dynamic
  type.
- **Changing what the version gate decides, or strict mode.** #76 moves when the gate runs, not what
  it says.
- **Changing format detection, or how the codecs classify their own endings.** #102's non-goal; the
  identity comparisons are correct and documented.
- **Changing which signal wins in run ordering, or parsing the stamp as a time.** #88's non-goal; the
  stamp is fixed-width text and sorts as it stands.
- **Raising the consumer floor, or pinning a patch a bot must chase.** #104's non-goal.
- **Guarding behaviour or `internal/` with the compatibility gate.** The corpus guards behaviour;
  #106's non-goal.
- **Any new exported identifier, option, model field or statistic.** This feature adds nothing to the
  surface being frozen; it makes that surface honest.
