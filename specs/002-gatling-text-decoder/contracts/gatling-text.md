# Contract: exported API surface

**Feature**: `002-gatling-text-decoder` | **Date**: 2026-09-02 | **Plan**: [plan.md](plan.md)

Everything this feature exports, and nothing it does not. Principle V governs: each identifier
carries a doc comment stating what it does and, for the reader, which Gatling versions it accepts.
The module is pre-v0.1.0, so these may still change between releases — every change is recorded
under Changed or Removed in `CHANGELOG.md`, and this feature's own entry lands under Added in the
implementation PR.

Field meanings are in [data-model.md](data-model.md); the evidence behind them is in
[research.md](research.md).

---

## Package `gatling`

Shared by this codec and the binary codec that milestone v0.0.5 adds. Standard library only.

```go
// Kind identifies which record a simulation.log line carries.
type Kind uint8

const (
    KindUnknown Kind = iota // the zero value; never written by Gatling
    KindRun
    KindUser
    KindRequest
    KindGroup
    KindError
    KindAssertion
)

func (k Kind) String() string

// Status is a request's or group's recorded outcome, preserved as written.
type Status uint8

const (
    StatusUnknown Status = iota // the zero value
    StatusOK
    StatusKO
)

// Event marks whether a user record opens or closes a scenario.
type Event uint8

const (
    EventUnknown Event = iota // the zero value
    EventStart
    EventEnd
)

// Header is the run header every log carries exactly once.
type Header struct {
    SimulationClass string
    RunID           string
    Start           int64  // ms since the Unix epoch, exactly as recorded
    Description     string // empty when the log wrote a lone space
    Version         Version
}

// Record is one decoded line. Kind says which fields are meaningful; the rest are zero.
//
// Groups is backed by a slice the reader reuses. It is valid until the next call to
// Next; copy it to keep it. String fields are immutable and may be shared between
// records that carry the same value; they may be kept freely.
type Record struct {
    Kind                  Kind
    Line                  int
    Groups                []string
    Name                  string
    Scenario              string
    Event                 Event
    Start                 int64
    End                   int64
    Timestamp             int64
    Status                Status
    Message               string
    CumulatedResponseTime int64
    Payload               string
}

// Version is a plain MAJOR.MINOR.PATCH Gatling release.
type Version struct{ Major, Minor, Patch int }

// ParseVersion reads a plain release number. A version carrying any suffix — a
// snapshot, milestone, nightly or vendor marker — is not a release and is rejected.
func ParseVersion(s string) (Version, error)

func (v Version) String() string
func (v Version) Compare(o Version) int

// Verdict is the outcome of the version gate for a log.
type Verdict uint8

const (
    VerdictUnknown    Verdict = iota // the zero value; never returned by Gate
    VerdictRefused                   // below range, not a release, or no header
    VerdictAccepted                  // inside the range covered by the corpus
    VerdictUnverified                // above that range: decodes, and warns
)

// Gate applies the version gate against a codec's range.
func Gate(found, lo, hi Version) Verdict

// Warning is raised for a log this module has no recording for. It is returned in
// the result; it is never only logged. It is a value, not an error: nothing failed.
type Warning struct {
    Version Version
    Min     Version
    Max     Version
}

func (w Warning) String() string

// SyntaxError ends a read. There is no partial result beside it, and no total may
// be derived from records delivered before it.
type SyntaxError struct {
    Line     int    // 1-based
    Expected string
    Found    string
}

func (e *SyntaxError) Error() string

// VersionError ends a read before any record is delivered.
type VersionError struct {
    Found    string  // as written, so an unparseable string can be quoted back
    Version  Version // the release Found parsed as; meaningful only when Parsed
    Parsed   bool    // true: Found is a release below the range. false: not a release at all
    Min, Max Version
}

func (e *VersionError) Error() string
```

## Package `gatling/text`

The codec for the tab-separated `simulation.log` written by Gatling 3.11.5 and 3.12.0. Standard
library only.

```go
// SupportedVersions returns the oldest and newest Gatling release this codec
// accepts without a warning. The range equals the versions covered by the
// golden corpus; widening it means recording a new corpus entry first. It is
// a function, not a pair of variables, so that no caller can widen the gate.
func SupportedVersions() (oldest, newest gatling.Version)

// MaxLineLen is the ceiling on one line, in bytes. A longer line fails the read
// rather than being held in memory. No valid log approaches it.
const MaxLineLen = 1 << 20

// Reader decodes a Gatling 3.11.5 or 3.12.0 text simulation.log from a stream.
//
// NewReader consumes the preamble and the run header and applies the version gate,
// so Header, Assertions and Warnings are available before the first Next. Records
// then arrive one at a time in file order. Peak memory does not grow with the log.
//
// The first line that cannot be decoded ends the read with a *gatling.SyntaxError
// naming its line number. Records delivered before that point are not a result:
// no total may be derived from them.
type Reader struct{ /* unexported */ }

// NewReader reads the preamble and the run header and gates on the version it
// names. It fails when no header can be found, when the version is below the
// supported range, and when the version is not a plain release. A version above
// the range succeeds and records a warning.
func NewReader(r io.Reader) (*Reader, error)

// Header returns the run header. Valid as soon as NewReader returns.
func (r *Reader) Header() gatling.Header

// Assertions returns the payloads written ahead of the header, in file order,
// verbatim and uninterpreted. Their number is a property of the simulation, not
// of the log.
func (r *Reader) Assertions() []string

// Warnings returns what the version gate raised, empty for a covered version.
func (r *Reader) Warnings() []gatling.Warning

// Next returns the next record. It returns io.EOF at the end of the log. Any
// other error ends the read: there is no next record after it.
func (r *Reader) Next() (gatling.Record, error)
```

---

## Behavioural contract

What a caller may rely on. Each line is an assertion some test must make.

| # | Guarantee | Requirement |
|---|---|---|
| C1 | Header, assertions and warnings are available before the first `Next` | FR-007, FR-006, FR-011 |
| C2 | The header need not be line 1 — assertion records precede it | FR-009 |
| C3 | A version below the range, a non-release version, or a missing header fails `NewReader`, and no record is delivered | FR-009a, FR-010 |
| C3a | `SupportedVersions` reports the range the gate applies, and nothing exported can change it | FR-012 |
| C4 | A version above the range succeeds, and `Warnings` is non-empty | FR-011 |
| C5 | `Next` yields records in file order until `io.EOF` | FR-001 |
| C6 | The first undecodable line returns a `*SyntaxError` with its 1-based line number, and no later line is read | FR-013 |
| C7 | A failed read is distinguishable from a complete one; records already delivered are not a result | FR-014 |
| C8 | A lone space decodes as an empty string | FR-003 |
| C9 | Group paths split on comma, losslessly; a top-level record has an empty path | FR-005 |
| C10 | Assertion payloads are returned byte for byte, never decoded | FR-006 |
| C11 | Timestamps are returned exactly as recorded — no rounding, re-basing or timezone conversion | FR-008 |
| C12 | Field counts are exact inside the range, a minimum above it | FR-008a |
| C13 | An error record's message may contain separators and is recovered whole | FR-008b |
| C14 | A line over `MaxLineLen` fails the read without being buffered | FR-016 |
| C15 | Peak memory is independent of log size | FR-017 |
| C16 | Chunked and whole-file reads produce identical records and identical failures | FR-018 |
| C17 | No input causes a panic | FR-015 |

## Not exported, and why

- **No conversion to a canonical result model.** `model/` is milestone v0.0.3. These are wire
  records; nothing derives a statistic from them here.
- **No counts, rates or percentiles.** Milestones v0.0.7 and v0.0.8. The mean request rate appears
  in the end-to-end tests as verification arithmetic, not as API.
- **No run-directory discovery, and no text-versus-binary sniffing.** Milestones v0.0.10 and
  v0.0.4. This reader is handed an open text log.
- **No `iter.Seq2` iterator.** It would be API surface with no consumer today, and it can be added
  later without disturbing `Next`.
- **No assertion decoding.** The payload's encoding is a Scala serialisation format and the
  requirements it carries are expressed in OpenNFR instead.
