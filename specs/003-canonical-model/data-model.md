# Phase 1 — Data Model: The Canonical Result Types

**Feature**: `003-canonical-model` · **Date**: 2026-09-04 · **Plan**: [plan.md](plan.md)

These are results, not wire records. The log's own events keep their types in `gatling/` and stay
exported (FR-014a); nothing here mirrors them field for field, and a consumer of this package never
learns which tool produced the run except by reading `Run.Tool`.

Nothing in this package computes. Counts, timings, percentiles and series are computed in
`galaxio-cli`, from these types; what this package owns is the definitions they are computed from
(constitution v2.0.0).

---

## Outcome

```go
type Outcome uint8

const (
    OutcomeUnknown Outcome = iota // the zero value; never produced by an adapter
    OutcomeSuccess
    OutcomeFailure
)
```

Recorded on the sample itself, never inferred from whether some other field is set (FR-002). The
zero value is `OutcomeUnknown` and no adapter emits it, so a sample that lost its outcome in
conversion is visible rather than silently successful.

---

## Opt[T]

```go
type Opt[T comparable] struct { /* value, set */ }

func Some[T comparable](v T) Opt[T]
func (o Opt[T]) Get() (T, bool)
func (o Opt[T]) Or(fallback T) T
func (o Opt[T]) IsSet() bool
```

One spelling for every value a source may not have recorded. The zero `Opt[T]` is unset, so a field
nobody filled reads as absent rather than as zero — which is what FR-006 requires and what a pointer
would also give, at the cost of an allocation per field per sample in a path measured against a
32 MiB ceiling (research R4).

`Opt[T]` answers *this record does not carry it*. `Capabilities` answers *this source never records
it*. Both exist because neither implies the other.

---

## Sample

One recorded operation — a request, in every tool surveyed so far.

| Field | Type | Notes |
|---|---|---|
| `Groups` | `[]string` | Enclosing groups, outermost first; empty at the top level (FR-005). Binds to `loadtest.group.name`. |
| `Name` | `string` | Binds to `loadtest.request.name`. |
| `Start` | `time.Time` | UTC. Exactly what the source recorded, unrounded and not re-based (FR-012). |
| `Duration` | `Opt[time.Duration]` | Binds to `loadtest.request.duration`. Unset when the source recorded no usable end (FR-020). |
| `Outcome` | `Outcome` | |
| `Failure` | `Opt[Failure]` | Set if and only if `Outcome == OutcomeFailure` (FR-009). |
| `Scenario` | `Opt[string]` | Absent for Gatling text: a `REQUEST` record names no scenario. |
| `ResponseCode` | `Opt[string]` | Absent for Gatling text. A string, because not every protocol's code is an integer. |
| `BytesSent` | `Opt[int64]` | Absent for Gatling text. |
| `BytesReceived` | `Opt[int64]` | Absent for Gatling text. |

**`Groups` ownership**: the slice is valid until the next call to `Next`. A consumer keeping a
sample past that copies it. This is the rule `gatling.Record.Groups` already states, kept identical
so there is one rule and not two.

### Failure

```go
type Failure struct {
    Type    string // what the source called this failure; never invented here
    Message string // what the source recorded, character for character
}
```

The *presence* of a `Failure` is what OpenNFR's only expressible numerator — `{error.type: "*"}` —
tests, which is why FR-009 is about presence and not about the value. `Type` carries what the source
classified the failure as. Gatling text classifies nothing: it writes a free-text message, so `Type`
carries the empty string there and `Capabilities` says the source provides no failure type. Minting a
taxonomy would be the faking Principle I forbids.

---

## GroupSample

One traversal of a group, closing.

| Field | Type | Notes |
|---|---|---|
| `Groups` | `[]string` | The group's own path, outermost first, its own name last. |
| `Start` | `time.Time` | UTC. |
| `Duration` | `Opt[time.Duration]` | Wall clock across the traversal, pauses included. Gatling records the group's start and end, so this is available. |
| `CumulatedDuration` | `Opt[time.Duration]` | The sum of the enclosed operations' durations. Binds to `loadtest.group.duration`. |
| `Outcome` | `Outcome` | Its own, **not** the conjunction of what it encloses (FR-003). |

Gatling's `GROUP` record carries a start, an end, the cumulated response time and a status, so both
durations are recorded. They are distinct quantities and neither is derived from the other: a run
that pauses inside a group is charged for the pause in the wall clock and not in the cumulated
figure. OpenNFR's own note on `loadtest.group.duration` says the format does not adjudicate between
them, which is why the model carries both and names which is which.

---

## UserEvent

| Field | Type | Notes |
|---|---|---|
| `Scenario` | `string` | |
| `Kind` | `UserEventKind` | `UserStart` or `UserEnd`. |
| `At` | `time.Time` | UTC. |

```go
type UserEventKind uint8
const (
    UserEventUnknown UserEventKind = iota
    UserStart
    UserEnd
)
```

Load-bearing rather than decoration (FR-013): the run span every derived rate divides by is bounded
by request, group and user timestamps, so a user event can set either end of it. Mishandling one
shifts every rate computed later.

---

## RunError

| Field | Type | Notes |
|---|---|---|
| `Message` | `string` | Character for character as recorded. |
| `At` | `time.Time` | UTC. |

A failure that belongs to no sample. Gatling writes one when a request's URL could not be built, so
it never reached the wire and produced no request record — visible in no other way (FR-021). It
arrives as a stream item, not on `Run`, because a run may hold any number of them (research R3).

---

## Run

Everything about one execution that does not grow with its length.

| Field | Type | Notes |
|---|---|---|
| `ID` | `string` | The identifier the tool assigned this run. |
| `Name` | `string` | The simulation or scenario name. |
| `Description` | `string` | Free text; empty where the source recorded none. |
| `Start` | `time.Time` | UTC. |
| `Tool` | `string` | `"gatling"` here. |
| `ToolVersion` | `string` | `"3.11.5"`, as the source stated it. |
| `Capabilities` | `Capabilities` | What this source can and cannot record. |
| `Warnings` | `[]Warning` | Carried from the version gate (FR-016a). Empty for a covered version. |
| `Assertions` | `[]string` | Opaque payloads, verbatim, one per declared requirement. Never decoded. |

**It holds no samples, group samples, user events or errors.** All four arrive through the stream
beside it (FR-011a). A consumer that needs all of one kind at once collects it and owns that memory.
`Assertions` stays here because a source writes one per declared requirement — a handful, bounded.

```go
type Warning struct {
    Version string // the version found
    Reason  string // why it is unverified
}
```

---

## Item

One thing the stream yields. A discriminated struct rather than an interface: it allocates nothing
per item, and it is the shape `gatling.Record` already set, which Principle VI asks for before a new
convention (research R3).

```go
type ItemKind uint8

const (
    ItemUnknown ItemKind = iota
    ItemSample
    ItemGroup
    ItemUser
    ItemError
)

type Item struct {
    Kind   ItemKind
    Sample Sample      // valid when Kind == ItemSample
    Group  GroupSample // valid when Kind == ItemGroup
    User   UserEvent   // valid when Kind == ItemUser
    Error  RunError    // valid when Kind == ItemError
}
```

Reading a field whose `Kind` does not select it yields that field's zero value. The doc comment says
so on every field, and the tests assert it, so a consumer that switches on `Kind` correctly can
never observe a stale value from the previous item.

---

## Capabilities

The source's own statement of what it records, read before anything is rendered.

```go
type Field uint16

const (
    FieldUnknown Field = iota
    FieldSampleDuration
    FieldSampleScenario
    FieldSampleResponseCode
    FieldSampleBytesSent
    FieldSampleBytesReceived
    FieldSampleFailureType
    FieldSampleUserIdentity
    FieldGroupDuration
    FieldGroupCumulatedDuration
    FieldGroupOutcome
    FieldConnectTiming
    FieldDNSTiming
    FieldTLSTiming
    FieldRequirements     // what an opaque assertion payload encodes
    FieldIntervalSeries
)

type Capabilities struct { /* the set of fields the source provides */ }

func NewCapabilities(provided ...Field) Capabilities
func (c Capabilities) Provides(f Field) bool
func (c Capabilities) Absent() []Field   // sorted, for a report that wants to name them
```

**The set stored is what the source *provides*.** Issue #4 words it the other way — "declares per
source what is absent" — and this inverts it deliberately, because of which way the mistake falls.
Store the absences, and a field added to the model later is listed by no existing adapter, so a
consumer is told it is present when it is not. Store what is provided, and the same addition reads
as absent everywhere until an adapter claims it: conservative, and honest. `Absent()` answers the
question FR-007 asks in the issue's own words.

### What a Gatling text run declares

Provided: `FieldSampleDuration`, `FieldGroupDuration`, `FieldGroupCumulatedDuration`, `FieldGroupOutcome`.

Absent, and named as such (FR-019, research R7): the sample's scenario, response code, bytes sent
and received, failure type and user identity; connect, DNS and TLS timings; the requirements the
assertion payload encodes; and per-interval series.

**A `GROUP` record carries a start and an end**, so a group's wall-clock duration is recorded and is
provided. This document's first draft listed it as absent, confusing what the *log records* with what
Gatling's *assertion interface reaches* — which is only the cumulated figure. `Capabilities` answers
the first question. Settled during implementation by a corpus record:
`GROUP outer,inner  with comma 1788379665736 1788379667251 1505 KO` is 1515 ms of wall clock beside
1505 ms cumulated, two different numbers on one line.

---

## Relationships

```text
Run                          one per log, available before the first item
 ├── Capabilities            what this source records, for the whole run
 ├── Warnings                from the version gate; empty for a covered version
 └── Assertions              opaque, one per declared requirement

stream of Item               zero or more, in source order, until io.EOF
 ├── ItemSample  → Sample ──Groups──> enclosing groups, outermost first, empty at top level
 ├── ItemGroup   → GroupSample
 ├── ItemUser    → UserEvent
 └── ItemError   → RunError
```

A group sample closes a path; a sample names the path it ran inside. Neither carries a user identity:
Gatling 3.11.5 and 3.12.0 record none, so a request cannot be attributed to the virtual user that
made it — declared through `Capabilities`, never filled in.

---

## Validation rules

Stated here because a test asserts each one, not because the types enforce them at compile time.

1. `Failure` is set if and only if `Outcome == OutcomeFailure`.
2. No adapter emits `OutcomeUnknown`, `ItemUnknown` or `UserEventUnknown`.
3. A field the run's `Capabilities` does not provide is unset on every item of that run.
4. `Duration` is unset rather than negative: an end at or before the start, or at the sentinel
   Gatling's own reader branches on, yields no duration (FR-020).
5. Counts through the model equal counts through the wire records equal the run's own report, for
   the run and per request name and per group (FR-018, SC-002).
6. Selecting `OutcomeSuccess` samples returns the same multiset whatever failures the run contains
   (FR-004, SC-003).
7. Reading a run in one pass and in arbitrary chunks yields identical item streams — inherited from
   the decoder, re-asserted through the model so a conversion that buffered would fail.
