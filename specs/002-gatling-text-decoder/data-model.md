# Data Model: Gatling Text simulation.log Decoder

**Feature**: `002-gatling-text-decoder` | **Date**: 2026-09-02 | **Plan**: [plan.md](plan.md)

These are the log's wire records, not the module's canonical result model. Nothing here derives a
count, a timing or a percentile, and no `Capabilities` claim is made. The canonical model and the
conversion into it are milestone v0.0.3; see the Complexity Tracking row in [plan.md](plan.md).

Field evidence and its limits are in [research.md](research.md) R2.

---

## Kind

Which record a line carries. Closed set of six, from the literal that opens the line.

| Kind | Line prefix |
|---|---|
| `KindRun` | `RUN` |
| `KindUser` | `USER` |
| `KindRequest` | `REQUEST` |
| `KindGroup` | `GROUP` |
| `KindError` | `ERROR` |
| `KindAssertion` | `ASSERTION` |

A line whose prefix is none of these fails the read at that line.

---

## Header

The run header, decoded from the single `RUN` record. Available before any event record (FR-007).

| Field | Meaning | Notes |
|---|---|---|
| `SimulationClass` | the simulation's fully qualified class name | as written |
| `RunID` | the run's identifier | as written |
| `Start` | wall-clock start of the run | milliseconds since the Unix epoch, preserved exactly |
| `Description` | free-text run description | a lone space decodes as empty (FR-003) |
| `Version` | the Gatling version that wrote the log | drives the gate |

**Validation.** Exactly six fields. `Start` must parse as a non-negative integer. `Version` must be
a plain `MAJOR.MINOR.PATCH` release; a suffixed or unreadable version fails the read (FR-009a).
A log with no header anywhere fails the read.

---

## Record

One flat struct returned by value for every kind (see [research.md](research.md) R9). The `Kind`
field says which of the others are meaningful; the rest are zero.

| Field | Kinds | Meaning |
|---|---|---|
| `Kind` | all | which record this is |
| `Line` | all | 1-based line number this record was decoded from |
| `Groups` | request, group | ordered enclosing group names; empty at top level |
| `Name` | request | request name |
| `Scenario` | user | scenario name |
| `Event` | user | `Start` or `End` |
| `Start` | request, group | start timestamp, ms since epoch |
| `End` | request, group | end timestamp, ms since epoch |
| `Timestamp` | user, error | event timestamp, ms since epoch |
| `Status` | request, group | `OK` or `KO` |
| `Message` | request, error | failure or error text; a lone space decodes as empty |
| `CumulatedResponseTime` | group | cumulated response time of the requests inside the group |
| `Payload` | assertion | the encoded blob, verbatim and uninterpreted (FR-006) |

**Ownership.** `Groups` is backed by a slice the reader reuses between calls. It is valid until the
next call to `Next`; a caller keeping it must copy it. This is the standard contract for a
streaming decoder and it is what keeps the group path off the heap per record. String fields are
independently allocated and outlive the call.

**Validation, per kind.** Field counts are exact inside the covered range and a minimum above it
(FR-008a, [research.md](research.md) R4). Every timestamp must parse as a non-negative integer.
`Status` must be `OK` or `KO`. `Event` must be `START` or `END`. Any failure ends the read with the
line number.

**Two format quirks the parsers must carry.**

- An error record's `Message` is everything between the kind and the trailing timestamp, however
  many separators it spans (FR-008b, [research.md](research.md) R5). Its exact field count does not
  apply.
- A request's `End` may be a sentinel — the minimum signed 64-bit integer — marking an event that
  never completed. Nothing may assume `End >= Start`. Whether a run of these versions actually
  emits one is unconfirmed and the recording task must check.

---

## Status and Event

`Status` is `OK` or `KO`, preserved exactly as recorded, never inferred from the presence of a
message. `Event` is `Start` or `End`, from the literals `START` and `END`.

---

## Version and VersionVerdict

`Version` is a plain `MAJOR.MINOR.PATCH` release number, ordered by component.

`VersionVerdict` is the gate's outcome, carrying the version found and the supported range:

| Verdict | When | Result |
|---|---|---|
| Refused | below the range, or not a plain release, or no header | error; no record delivered |
| Accepted | inside the range | records; no warning |
| AcceptedUnverified | above the range | records, plus a warning reachable in the result |

The supported range equals the corpus coverage — 3.11.5 through 3.12.0, which is every released
version in the 3.11.5–3.12.x range. Widening it means recording first.

---

## SyntaxError

What ends a read. Carries the 1-based line number, what was expected there, and what was found.
It is never one item among many: there is no partial result alongside it, and no total may be
derived from records delivered before it (FR-013, FR-014).

**Distinct from `VersionError`**, which ends a read before any record is delivered, and from the
version **warning**, which ends nothing and travels with a successful result.

---

## Relationships

```text
Reader
  ├── Header          exactly one, from the RUN record, before any event record
  ├── Assertions      zero or more, from the preamble ahead of the header, verbatim
  ├── Warnings        zero or one, from the version gate
  └── Next() Record   zero or more, in file order, until io.EOF or a SyntaxError

Record ──Groups──> ordered group path, outermost first, empty at top level
```

A group record closes a path; a request record names the path it ran inside. Neither carries a user
identity — Gatling 3.11.5 and 3.12.0 do not record one — so a request cannot be attributed to the
virtual user that made it. That absence is a property of the source and must be reported as absent
rather than filled in when milestone v0.0.3 maps these records onto the canonical model.
