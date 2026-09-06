# Phase 0 Research: Reading the Binary simulation.log

**Feature**: `005-gatling-binary-decoder` | **Date**: 2026-09-06 | **Spec**: [spec.md](./spec.md)

Constitution in force: **v2.2.0**.

The headline finding is that this feature starts from a settled format rather than a guessed one.
R1 records it and R2 records the two ways a careful decoder still gets it wrong.

---

## R1 — The wire format, and why it is established rather than claimed

**Decision**: implement against the layout below. It was read out of Gatling's own
`gatling-core/.../stats/writer/LogFileDataWriter.scala` at **both bounds of the supported range**,
v3.13.0 and v3.15.1, and then **decoded against the real 64-byte sample this project holds**. All 64
bytes parse, and every field lands where the source says it should.

**Primitives.** Every integer is big-endian, because the writer uses a plain `ByteBuffer` and never
sets its order.

| Written | Bytes |
|---|---|
| `writeByte` | 1 |
| `writeBoolean` | 1 — `1` for true, `0` for false |
| `writeInt` | 4, big-endian |
| `writeLong` | 8, big-endian |
| `writeString`, empty | `int32(0)` **and nothing else** |
| `writeString`, non-empty | `int32(n)`, then `n` bytes, then 1 coder byte |
| `writeCachedString`, first use | `int32(index)` where index > 0, then the string as above |
| `writeCachedString`, later use | `int32(-index)` |
| `writeByteBuffer` | `int32(n)`, then `n` bytes |

`n` is the length of the JVM's **internal byte array**, not the character count: for coder 0
(Latin-1) they are equal, for coder 1 (UTF-16) the array is twice the characters.

**Records.** Each opens with its kind byte: Run 0, Request 1, User 2, Group 3, Error 4 — the values
`RecordHeader.scala` defines, unchanged between the two bounds.

| Record | Fields, in order |
|---|---|
| **Run** (0) | string version · string simulation class · **int64** start, epoch ms · string description · int32 scenario count · that many **plain** strings · int32 assertion count · that many length-prefixed opaque blobs |
| **Request** (1) | int32 group depth · that many cached strings · cached string name · int32 start offset · int32 end offset · boolean OK · cached string message |
| **User** (2) | int32 scenario **index** · boolean isStart · int32 timestamp offset |
| **Group** (3) | int32 group depth · that many cached strings · int32 start offset · int32 end offset · int32 cumulated response time · boolean OK |
| **Error** (4) | cached string message · int32 timestamp offset |

Every offset is milliseconds from the run's `start`, written as `(timestamp - runStart).toInt`.

**Rationale**: the constitution treats an undocumented format as something the corpus settles, and it
still does — the gate is bound to recordings and R8 records them. But a decoder written from a
verified layout and then checked against recordings is a different exercise from one reverse-
engineered against them, and the difference is worth several weeks. Verification against the sample:

```
00 | 00 00 00 06 "3.15.1" 00 | 00 00 00 08 "BinProbe" 00 | int64 1788632969690
   | 00 00 00 00  ← empty description | 00 00 00 01  ← one scenario
   | 00 00 00 08 "binprobe" 00 | 00 00 00 00  ← no assertions
   | 02 | 00 00 00 00 | 01     ← a User record: scenario 0, start
```

**Alternatives considered**:

- **Reverse-engineer from recordings alone**, as issue #6's framing assumes. Rejected now that the
  source is readable: it would rediscover R2's two traps the expensive way, by producing a decoder
  that is subtly wrong on a corpus that cannot say why.
- **Port Gatling's reader.** Rejected — it is Scala, it is `private[gatling]`, and Principle IV puts
  the standard library alone in this package.

---

## R2 — The two ways a careful decoder still gets this wrong

**Decision**: both are pinned by a test before the decoder is written.

**An empty string carries no coder byte.** `writeString` returns after `putInt(0)` when the string is
empty. A reader that always consumes a trailing coder byte is off by one from the first empty
description or empty failure message onward — and an empty message is the *normal* case, since a
successful request writes `message.getOrElse("")`. This is not a rare path; it is most of the file.
The sample proves it: the probe's blank description is four zero bytes and no fifth.

**Scenario names are not cached.** The Run record writes them with `writeString`, not
`writeCachedString`, so they never enter the table. A decoder that fed them to the cache would shift
every later index by the number of scenarios, and the corruption would show up as *wrong names on
every record*, not as a failure. The cache's first index is 1 precisely because the writer uses the
negation of an index for a hit and zero has no negation.

**Rationale**: neither trap fails loudly. The first desynchronises the byte stream, which at least
usually ends in a nonsense length; the second silently renames records, which does not end in
anything. Both are one-line mistakes and both are cheap to pin.

**Alternatives considered**: discovering them from the corpus. Rejected — that is the expensive way
to learn something the source states plainly.

---

## R3 — Timestamps are 32-bit offsets, and that has a limit

**Decision**: resolve every offset against the run's `start` into the same absolute instant the text
codec reports, and treat the 32-bit range as a documented limit rather than a bug to work around.

**Rationale**: the writer stores `(timestamp - runStart).toInt`. A signed 32-bit millisecond offset
covers about **24.8 days**, after which Gatling's own writer overflows and the log contains a
negative or wrapped offset. Nothing the decoder does can recover the real time; what it must not do
is present a wrapped value as a measurement. An offset that yields an instant before the run's start
is reported as absent, which is the rule the text codec already follows for a duration it cannot
compute.

Two consequences worth stating. The binary format cannot represent a run longer than 24.8 days at
all, so a soak run past that is unreadable by Gatling too. And the offsets are *milliseconds since
this run began*, so a decoded instant is only as good as the run's recorded start — which the text
format does not depend on.

**Alternatives considered**: treating the offset as unsigned to reach 49.7 days. Rejected: the
writer casts to a signed `Int`, so the bits are what a signed overflow left, and reading them as
unsigned invents a timestamp Gatling never meant.

---

## R4 — Latin-1 and UTF-16, and the byte order nothing records

**Decision**: decode coder 0 as Latin-1 and coder 1 as UTF-16 in little-endian order; document the
assumption; refuse a coder value that is neither.

**Rationale**: `StringInternals.value` hands back the JVM's internal array as-is, and `coder`
distinguishes the two compact-string representations. For UTF-16 the JVM stores each character in
the platform's native byte order, and the file records nothing about which that was. Every machine
this project can record on is little-endian, so the corpus cannot prove the other case; a log
written on a big-endian JVM will decode wrongly and there is no marker to detect it. That is a
property of the format. It is recorded in the spec's assumptions and in the package's documentation
rather than hidden behind a heuristic.

Refusing an unknown coder matters more than it looks: guessing would return mojibake, and Principle
II's "no partial result" rule means a name that decodes wrongly is worse than a read that stops.

**Alternatives considered**:

- **Sniff the order** from whether the high or low byte of the first character is zero. Rejected: it
  guesses, it is wrong for scripts where both bytes are non-zero, and it makes the result depend on
  the *content* of a name rather than on the format.
- **Decode UTF-16 leniently**, substituting the replacement character. Rejected by FR-004: a
  silently mangled name groups two requests into one in every report downstream.

---

## R5 — Where the codec lives

**Decision**: a new package `gatling/binary`, as issue #6 proposes, exporting the same shapes
`gatling/text` does: a `Reader` over wire records and a `RunReader` over canonical results.

**Rationale**: `gatling/simlog` already dispatches on format and its table has an entry waiting for
exactly this. A second codec beside the first, with the same two constructors and the same option
type, is what makes the dispatch table a table rather than a special case. Nothing in `gatling/text`
is touched.

**Alternatives considered**:

- **A format switch inside `gatling/text`.** Rejected: the package documents itself as the
  tab-separated codec, and the two share no parsing at all — a text scanner and a binary record
  reader have nothing in common below the record types.
- **One package with two entry points.** Rejected for the same reason, and because `simlog` already
  provides the single entry point a caller wants.

---

## R6 — What is reused, and what genuinely cannot be

**Decision**: reuse the record types, the model conversion's shape, the version policy, the shared
errors and the option type. Write the reader, the string table and the record decoding fresh.

| Reused | Why it fits |
|---|---|
| `gatling.Record`, `gatling.Header`, `Kind`, `Status`, `Event` | FR-001: one set of wire records for both codecs, so `simlog`'s interfaces need no widening |
| `gatling.Policy`, `Verdict`, `WithStrict`, `Warning` | v0.0.4 made the version decision shared precisely for this |
| `gatling.SyntaxError`, `VersionError`, `UnverifiedError` | FR-022 wants an offset where the text codec wants a line; the type already carries a position |
| `model` conversion | The text codec's `convert` maps records to `model.Item`; the mapping is a property of the records, not of the format |
| `text.Capabilities`'s shape | R11 |

Written fresh: everything below the record types. A text scanner splits lines and fields; a binary
reader reads sized primitives and maintains a table. There is no shared layer to extract, and
inventing one would be the speculative abstraction Principle VI forbids.

**Rationale**: the conversion to `model` is the interesting case. It could be duplicated per codec,
or lifted into a shared helper. It is the same function of the same input, so lifting it is reuse
rather than abstraction — but where it lives is a Phase 1 decision, recorded in
[contracts/gatling-binary.md](./contracts/gatling-binary.md), because moving an exported function is
an API change under Principle V.

---

## R7 — Streaming, the table, and what "bounded" means here

**Decision**: stream record by record; keep the string table for the life of the read; cap every
length read from the file before it is used to allocate.

**Rationale**: Principle II's bounded-memory rule needs restating for this format. Peak memory is
bounded by the number of *distinct* strings a simulation declares — request names, group names,
failure messages — not by the number of records. A run making ten million requests against twenty
named endpoints holds twenty strings. That is the property to assert, and SC-005 asserts it by
lengthening a log without adding names.

The cap has a different job from the text codec's line ceiling. There, an over-long line is
corruption. Here, an `int32` length is read directly from the file and handed to an allocator, so a
single corrupt byte can ask for two gigabytes. The cap is checked *before* allocation and is sized
from what a real log contains: the largest string in a Gatling log is an assertion payload, which
runs to tens of kilobytes, and the largest count is the string-table index. Both are recorded next
to the constant with the reasoning, as the constitution requires.

**Alternatives considered**: trusting the length and letting the allocator fail. Rejected — an
out-of-memory kill is not an error a caller can handle, and Principle II names this case explicitly.

---

## R8 — Recording the corpus, which is the long pole

**Status**: done. The three recordings are in `testdata/corpus/gatling/`. Everything below is what
the recording established, and several of the things it established contradict what this section
said before the runs were made.

**Decision**: record **3.13.1, 3.14.9 and 3.15.1** of the same probe simulation, each kept with every
account of its own numbers Gatling produced.

### The floor is 3.13.1, and 3.13.0 is excluded on evidence

3.13.0 is the first Gatling to write a binary log, and it cannot be recorded. It cannot read back the
assertion records it writes: `IllegalArgumentException: Unknown object coding: 1`, out of boopickle
in `AssertionPicklers`, raised while `FirstPassParser` parses the run record. No report is generated
and the run fails.

Three things were checked before concluding that, because the first two explanations were wrong:

- It is **not** gatling-picatinny. The failure is identical with picatinny dropped from the
  classpath.
- It is **not** the number or shape of the assertions. The probe was run with ten and with one, and
  both failed the same way.
- It is **not** the writer. A 3.13.0 `simulation.log` parses cleanly to its last byte against the
  grammar in [data-model.md](./data-model.md), with the same record counts as the 3.13.1 recording.
  Only the assertion payload differs — byte 1 of the first blob is `03` there and `01` from 3.13.1 on
  — and parsec carries payloads through without decoding them.

So a 3.13.0 log is readable by this library and cannot be a corpus entry: a run that produces no
report has no second, independent account of its own numbers (FR-029). 3.13.1 through 3.15.1 all
generate a report with every assertion passing. The gate's floor follows the corpus, as Principle II
requires, so it is 3.13.1 — and a 3.13.0 log is refused, which is a real cost and the honest one.

### The flavour split is the log format, not the tool version

The probe now states its expectations two ways, chosen by source directory in `build.sbt`.

An OpenNFR `loadtest.group.name` is the literal **recorded** group name, and the two Gatling log
formats record one differently. A text `simulation.log` separates a group path with commas, so
Gatling replaces a comma inside a name with a space before writing it; the binary format
length-prefixes each name, so the declared name survives. The probe declares `inner, with comma`
deliberately, so one document cannot address that group in both formats: `nfr.yaml` must spell it one
way or the other. The text versions render assertions from the document; the binary versions state
the same expectations in Gatling's own DSL, and the two are kept in step by hand.

gatling-picatinny is **not** the reason, which is what this section claimed before the runs. 1.27.0
resolves and runs correctly on 3.13.1 — verified directly, report generated. It simply has no release
above 3.13.x, so it could not have covered the binary range in any case, and using it for one version
of that range would record that version through a different mechanism than the rest (FR-032).

### What had to change in the probe, and what it bought

None of it can be added to a run after the run is made.

1. **A name outside Latin-1** — `Проверка /ok`. Confirmed in the recorded logs: it is stored with
   coder 1 and decodes back byte-identical. This is the only thing in the corpus that exercises the
   half of [R4](#r4--latin-1-and-utf-16-and-the-byte-order-nothing-records) that can go wrong.
2. **A name repeated far more often than it is introduced** — `GET /ok` ten times inside `outer`.
   Confirmed: 14 distinct strings carry 102 requests, and that name is written once and referred to
   by index 65 more times.
3. **A group whose name repeats** across users and across those repeats, so a cached string appears
   inside a group path and not only in a request name.
4. **`logback.xml` had to be flattened.** The logback Gatling ships from 3.13.0 cannot parse the
   scaffolded template's nested `<if>/<then>/<else>`: it raises `EmptyStackException` in
   `ElseModelHandler` before any appender is created, and the run dies at startup. Every branch was
   already false for this probe, so nothing about what is logged changed. Nothing about
   `simulation.log` was ever at stake — Gatling writes it itself.

### What the recordings actually hold

The artefact boundary is **3.14.0, not 3.13.0**, which is the other thing this section had wrong.

| Version | `global_stats.json` / `stats.json` | HTML report | Console summary |
|---|---|---|---|
| 3.13.1 | **yes**, plus `assertions.xml` | yes | yes |
| 3.14.9 | no | yes | yes |
| 3.15.1 | no | yes | yes |

So the 3.13.1 entry keeps the same machine-readable statistics the text corpus relies on, and the
report and console are a third and fourth account rather than a replacement. Only from 3.14.0 does
the Principle III exemption actually apply, and each `RECORDING.md` says which accounts its own run
produced.

**Extraction is two-shaped, and the verification suite has to know it** (T021):

- **The report.** From 3.14.0 the per-request figures are baked into `index.html` as classed cells.
  On the 3.13.x line the same table is filled in by JavaScript at page load, so the markup carries
  `'value total col-2">' + request.stats.numberOfRequests.total` rather than a number.
- **The console.** From 3.14.0 the Global Information block is a table with `|` separators; on 3.13.x
  it reads `102 (OK=84 KO=18)`.

### One difference between the recordings, fully accounted for

3.13.1's log is 3965 bytes and the other two are 3952. The 13 bytes are one string: the check failure
message reads `status.find.is(200), but actually found 500` on 3.13.1 and
`status.find.is(200), found 500` from 3.14.0. Record counts are identical across all three — 1 RUN,
12 USER, 102 REQUEST, 12 GROUP, 6 ERROR — so the cross-version comparison (T026) must set message
text aside along with timing, identity and order.

**Rationale**: the recording is irreversible and the artefacts are only produced at run time. The
HTML report is a file in the run directory and survives archiving; the console summary is standard
output and exists only if redirected. Capturing everything costs one redirection.

**Alternatives considered**: recording only 3.15.1 and arguing the rest from the source diff.
Rejected by Principle II, which forbids widening the gate on the belief that a format did not
change — and by the fact that the belief, however well evidenced, is not what the rule accepts. The
3.13.0 finding is the concrete vindication: the source diff says the writer is unchanged, and it is,
and the version is still unrecordable for a reason no diff would have shown.

---

## R9 — Performance

**Decision**: state throughput and peak memory against the largest corpus log, and ship benchmarks
that measure both. Target: at least as fast as the text codec on the same simulation, with peak
memory independent of record count.

**Rationale**: the constitution requires a decoder plan to state both and ship a `testing.B`. The
binary format should be *faster* than the text one — no line splitting, no field parsing, no
integer-from-string — and the string table means repeated names cost an index read rather than an
allocation. If it is not faster, something is copying that should not be. The figure to watch is
allocations per record, which should approach zero for a run whose names are all cached.

**Alternatives considered**: none; a decoder without a measured throughput is one the next change
can silently halve.

---

## R10 — Capabilities: do the two formats differ?

**Decision**: derive `Capabilities` for this source from what the records above actually carry, and
expect it to equal the text codec's — but assert the equality rather than assume it.

**Rationale**: comparing the field lists, the binary format carries exactly what the text one does:
group path, request name, start and end, outcome, failure message, user scenario and event, group
cumulated time, error message. It records no response code, no body sizes, no connection timings and
no per-interval series, the same absences the text codec declares. One difference exists and is not
a capability: the binary format stores a *scenario index* on user events where the text format
stores a name, which is a representation difference the decoder resolves and the model never sees.

So the expected answer is "identical", and FR-021 asks for the difference to be visible if there is
one. A test comparing the two capability sets states which it is, and fails if a later version of
either format changes the answer.

**Alternatives considered**: assuming equality and sharing one `Capabilities` function. Rejected —
that would make a real divergence invisible, which is the failure `Capabilities` exists to prevent.

---

## R11 — Skills required for this change

Per the constitution's Engineering Guidance, this change triggers every required-reading row: it
adds exported identifiers, errors, tests, doc comments and exported types.

| Skill | What it governs here |
|---|---|
| `golang-naming` | a new package's exported surface, permanent from v0.1.0 |
| `golang-error-handling` | the offset-carrying errors, `errors.As` discrimination |
| `golang-testing` | golden corpus, table-driven cases, fuzzing the decoder |
| `golang-documentation` | doc comments stating the versions accepted; an example |
| `golang-structs-interfaces` | the reader types and whether they satisfy `simlog`'s interfaces |

Situational and expected to apply: `golang-benchmark` (R9), `golang-safety` (the allocation caps and
slice handling in R7), `galaxio-gatling-pro` and `gatling-versions` (R8's recordings).

Forbidden as always: testify and the `samber/*` family — Principle IV keeps `gatling/` stdlib-only,
and `gatling/binary` is inside it.

---

## Open questions carried into implementation

| # | Question | Settled by |
|---|---|---|
| 1 | Does a real log ever carry coder 1, and does little-endian decode it correctly? | R4/R8 — the non-Latin-1 name in the probe, at recording time |
| 2 | Does `Capabilities` differ from the text codec's? | R10 — a test that compares the two sets |
| 3 | Where does the record-to-model conversion live once two codecs need it? | R6 — a Phase 1 API decision, in contracts |

Nothing else in Technical Context is unknown. The format itself no longer is.
