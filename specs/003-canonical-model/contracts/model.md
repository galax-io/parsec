# Contract: package `model`

**Feature**: `003-canonical-model` · **Stability**: pre-v0.1.0, may change between releases
(Principle V) · **Dependencies**: standard library only

The canonical result types. Every source is decoded into these, and these are what consumers build
on — the three downstream builds (galaxio-cli, the comet sidecar, the Galaxio backend) import this
package and no tool package. Nothing here computes a count, a timing or a percentile: the arithmetic
is `galaxio-cli`'s, and this package owns the definitions it computes from (constitution v2.0.0).

Field-level semantics are in [data-model.md](../data-model.md). This file is the exported surface and
the doc comment each identifier carries.

---

## Outcome

```go
// Outcome is whether one recorded operation succeeded or failed, as the source
// recorded it. It is never inferred from whether another field is set.
type Outcome uint8

const (
    // OutcomeUnknown is the zero value and is never produced by an adapter.
    OutcomeUnknown Outcome = iota
    OutcomeSuccess
    OutcomeFailure
)

// String returns "success", "failure" or "unknown".
func (o Outcome) String() string
```

## Opt

```go
// Opt is a value the source may not have recorded. The zero Opt is unset, so a
// field nobody filled reads as absent rather than as zero.
//
// Opt says this record does not carry the value. Capabilities says the source
// never records it. Neither implies the other, which is why both exist.
type Opt[T any] struct { /* unexported */ }

// Some returns an Opt holding v.
func Some[T any](v T) Opt[T]

// Get returns the value and whether it was set.
func (o Opt[T]) Get() (T, bool)

// Or returns the value if set, and fallback otherwise. The caller chooses the
// fallback; this package never chooses one for it.
func (o Opt[T]) Or(fallback T) T

// IsSet reports whether a value was recorded.
func (o Opt[T]) IsSet() bool
```

## Failure

```go
// Failure is what the source recorded about an operation that did not succeed.
// Its presence is what distinguishes a failed sample; see Sample.Failure.
type Failure struct {
    // Type is what the source called this failure. It is never invented here:
    // a source that classifies nothing leaves it empty and declares
    // FieldSampleFailureType absent.
    Type string

    // Message is what the source recorded, character for character.
    Message string
}
```

## Sample

```go
// Sample is one recorded operation — a request, in every tool surveyed so far.
type Sample struct {
    // Groups is the enclosing groups, outermost first, and empty for an
    // operation taken outside any group. It binds to the OpenNFR attribute
    // loadtest.group.name.
    //
    // Valid until the next call to Next; copy it to keep it.
    Groups []string

    // Name binds to the OpenNFR attribute loadtest.request.name.
    Name string

    // Start is when the operation began, in UTC, exactly as the source
    // recorded it: unrounded, not re-based against the run's start.
    Start time.Time

    // Duration binds to the OpenNFR metric loadtest.request.duration. It is
    // unset when the source recorded no usable end — never negative.
    Duration Opt[time.Duration]

    Outcome Outcome

    // Failure is set if and only if Outcome is OutcomeFailure.
    Failure Opt[Failure]

    // Scenario, ResponseCode, BytesSent and BytesReceived are recorded by some
    // sources and not others. Ask the run's Capabilities which.
    Scenario      Opt[string]
    ResponseCode  Opt[string]
    BytesSent     Opt[int64]
    BytesReceived Opt[int64]
}
```

## GroupSample

```go
// GroupSample is one traversal of a group, closing.
type GroupSample struct {
    // Groups is the group's own path, outermost first, its own name last.
    // Valid until the next call to Next; copy it to keep it.
    Groups []string

    Start time.Time

    // Duration is wall clock across the traversal, including any pause inside
    // it. CumulatedDuration is the sum of the enclosed operations' durations
    // and binds to the OpenNFR metric loadtest.group.duration. They are
    // distinct quantities and neither is derived from the other.
    Duration          Opt[time.Duration]
    CumulatedDuration Opt[time.Duration]

    // Outcome is the group's own, not the conjunction of what it encloses: a
    // group can fail while every operation inside it succeeded.
    Outcome Outcome
}
```

## UserEvent

```go
// UserEventKind is which end of a virtual user's life an event marks.
type UserEventKind uint8

const (
    UserEventUnknown UserEventKind = iota
    UserStart
    UserEnd
)

// UserEvent is one virtual user starting or ending a scenario.
//
// These bound the run span that every derived rate divides by, so they are
// load-bearing rather than decoration: mishandling one shifts every rate
// computed from the run.
type UserEvent struct {
    Scenario string
    Kind     UserEventKind
    At       time.Time
}
```

## RunError

```go
// RunError is a failure the source recorded that belongs to no sample — for
// example an operation that never reached the wire, and so produced no record
// of its own.
type RunError struct {
    Message string
    At      time.Time
}
```

## Warning

```go
// Warning is something the source's version gate raised about a run that was
// read anyway — typically that no recording covers the version that wrote it.
// It travels into the run so a result decoded from an unverified version stays
// identifiable as one.
type Warning struct {
    Version string
    Reason  string
}

func (w Warning) String() string
```

## Run

```go
// Run is one execution, described by everything about it that does not grow
// with its length.
//
// It holds no samples, group samples, user events or errors: all four arrive
// through the stream beside it, because a run large enough to matter is larger
// than the memory available to hold it. A consumer that needs all of one kind
// at once collects it and owns that memory.
type Run struct {
    ID          string
    Name        string
    Description string
    Start       time.Time

    // Tool is the tool that produced the run ("gatling"), and ToolVersion the
    // version it stated ("3.11.5").
    Tool        string
    ToolVersion string

    Capabilities Capabilities

    // Warnings is empty for a version the project has a recording for.
    Warnings []Warning

    // Assertions is the opaque payloads the source wrote, one per declared
    // requirement, verbatim. This module does not decode or interpret them.
    Assertions []string
}
```

## Item

```go
// ItemKind selects which field of an Item carries the value.
type ItemKind uint8

const (
    ItemUnknown ItemKind = iota
    ItemSample
    ItemGroup
    ItemUser
    ItemError
)

// String returns "sample", "group", "user", "error" or "unknown".
func (k ItemKind) String() string

// Item is one thing a run's stream yields. Kind selects the field that carries
// the value; every other field holds its zero value.
//
// A discriminated struct rather than an interface, so that streaming a run
// allocates nothing per item.
type Item struct {
    Kind   ItemKind
    Sample Sample      // valid when Kind is ItemSample
    Group  GroupSample // valid when Kind is ItemGroup
    User   UserEvent   // valid when Kind is ItemUser
    Error  RunError    // valid when Kind is ItemError
}
```

## Capabilities

```go
// Field names something a source may or may not record.
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
    FieldRequirements
    FieldIntervalSeries
)

// String returns the field's name, for a report that names what is missing.
func (f Field) String() string

// Capabilities is a source's own statement of what it records. A consumer reads
// it before rendering anything, rather than discovering that every value of a
// column is empty.
//
// The set held is what the source PROVIDES. A field this package gains later is
// therefore absent for every existing adapter until one claims it, which is the
// conservative answer: the opposite storage would report a new field as present
// for sources that never recorded it.
type Capabilities struct { /* unexported */ }

// NewCapabilities returns the capabilities of a source that provides exactly
// the given fields and nothing else.
func NewCapabilities(provided ...Field) Capabilities

// Provides reports whether the source records f.
func (c Capabilities) Provides(f Field) bool

// Absent returns, in a stable order, every known field the source does not
// record.
func (c Capabilities) Absent() []Field
```

---

## What this contract does not include

- **No `Aggregate`, and no summary-only run.** Deferred whole to milestone v0.5.0, where Locust is
  the first source that publishes summaries and no samples. This overrides issue #4; see the
  Complexity Tracking row in [plan.md](../plan.md).
- **No statistics.** No count, mean, percentile, range or series — not in this milestone and not in
  any later one. They are computed in `galaxio-cli`, from these types. What parsec still owes that
  work is the primitives of [#8](https://github.com/galax-io/parsec/issues/8): an addressable
  request position and the bounds of a run.
- **No discovery.** Nothing here finds a run directory or decides which log to open; that is
  v0.0.10.
