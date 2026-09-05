# Contract: package `gatling/text` — the conversion

**Feature**: `003-canonical-model` · **Stability**: pre-v0.1.0 · **Dependencies**: standard library
only

What this feature **adds** to `gatling/text`. Everything already exported by v0.0.2 — `NewReader`,
`Reader`, `SupportedVersions`, `MaxLineLen`, and the `gatling` package's `Record`, `Header`, `Kind`,
`Status`, `Event`, `Version`, `Verdict`, `Gate`, `SyntaxError`, `VersionError`, `Warning` — keeps its
current signature and behaviour. Nothing is removed and nothing is deprecated.

---

## RunReader

```go
// RunReader reads a Gatling text simulation.log as canonical results.
//
// It is the model-facing counterpart of Reader: same log, same version gate,
// same bounded memory, but it yields model.Item values rather than the log's
// own wire records. Use this unless you specifically need to see what the log
// contained, which is what Reader is for.
//
// The whole run is never resident: Run is available before the first item, and
// items arrive one at a time.
type RunReader struct { /* unexported */ }

// NewRunReader reads the preamble and the run header from r, applies the
// version gate, and returns a reader positioned before the first event.
//
// It accepts Gatling 3.11.5 through 3.12.0. A log written by an older version
// is refused with a *gatling.VersionError naming the version found and the
// range supported. A plain release number above the range is accepted, and the
// warning is on the returned Run. A version string that is not a plain
// MAJOR.MINOR.PATCH release is refused, quoting what was found.
func NewRunReader(r io.Reader) (*RunReader, error)

// Run returns everything about this run that does not grow with its length:
// its identity, the tool and version, what the source can and cannot record,
// any version warning, and the opaque assertion payloads.
//
// It is complete as soon as NewRunReader returns, and does not change as items
// are read.
func (x *RunReader) Run() model.Run

// Next returns the next item of the run, or io.EOF at the end.
//
// A log that cannot be read in full yields a *gatling.SyntaxError naming the
// line and what was expected there, and no item after it: a partial read
// cannot produce counts that match the tool's own report, so it is refused
// rather than reported.
//
// Slices on the returned item are valid until the next call; copy them to keep
// them.
func (x *RunReader) Next() (model.Item, error)
```

## Capabilities of this source

```go
// Capabilities returns what a Gatling text simulation.log records and what it
// never does. It is the same value as Run().Capabilities and is exported so a
// caller can ask before opening anything.
//
// Provided: the duration of a request; a group traversal's wall-clock
// duration, its cumulated response time and its own status.
//
// Absent: the scenario a request ran under, its response code, its byte
// counts, the identity of the virtual user that made it, any classified
// failure type, connect, DNS and TLS timings, the requirements the assertion
// payload encodes, and per-interval series. The format records none of these,
// so nothing derived from them may be reported as measured.
func Capabilities() model.Capabilities
```

---

## Mapping, wire record to item

| Wire record | Item | Notes |
|---|---|---|
| `RUN` | — | Becomes `Run`; never an item. |
| `ASSERTION` | — | Becomes `Run.Assertions`, verbatim. |
| `REQUEST` | `ItemSample` | `Groups`, `Name`, `Start`, `Duration` from start/end, `Outcome`, `Failure` from the recorded message. |
| `GROUP` | `ItemGroup` | `Groups`, `Start`, `Duration` (wall clock, from start and end), `CumulatedDuration`, `Outcome`. Two different quantities, and the record carries both. |
| `USER` | `ItemUser` | `Scenario`, `Kind`, `At`. |
| `ERROR` | `ItemError` | `Message`, `At`. Never attached to a sample. |

Counts are preserved exactly: one item per event record, in source order. The number of items of
each kind equals the number of wire records of the corresponding kind, and the run's totals taken
through the model equal what that run's own report states (FR-018).

### Times

Gatling writes epoch milliseconds. `time.UnixMilli(...).UTC()` converts without loss and prints
deterministically. No value is rounded or re-based against the run's start.

### Durations, and the sentinel

`Duration` is `end - start`, and is **unset** rather than negative or enormous when the end is at or
before the start, or equals the sentinel Gatling's own reader branches on (the minimum signed 64-bit
integer). Whether a 3.11.5 or 3.12.0 run can actually produce the sentinel is unconfirmed; the
conversion does not assume it cannot.

### Failure

A `KO` request yields `Outcome == OutcomeFailure` and a `Failure` whose `Message` is what Gatling
recorded, character for character. `Type` is empty and `FieldSampleFailureType` is declared absent:
Gatling text records a free-text message, not a classification, and inventing one would be faking.

An exception-backed failure produces **both** a failed sample and a separate `ItemError`, because
Gatling writes both. Neither is derived from the other.

---

## Errors

| Condition | Error | From |
|---|---|---|
| Version below the supported range, or not a plain release | `*gatling.VersionError` | `NewRunReader` |
| No run header | `*gatling.SyntaxError` | `NewRunReader` |
| An unreadable line | `*gatling.SyntaxError` with its line number | `Next` |
| End of log | `io.EOF` | `Next` |

All of these are the decoder's own errors, unwrapped and unchanged. The conversion introduces no
error type of its own: it decides nothing the decoder has not already decided.
