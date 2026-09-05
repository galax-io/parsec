# Contract: format detection, the version policy and the dispatching reader

**Feature**: `004-gatling-format-detection` | **Date**: 2026-09-05 | **Spec**: [spec.md](../spec.md)

Every identifier below is new unless marked *(existing)*. Principle V requires a doc comment on each
one stating what it does and, for decoders, which tool versions it accepts; the comments sketched
here are the contract, not decoration.

> **Approval required before implementation.** One item on this page changes an existing exported
> signature: the constructors in `gatling/text` gain `opts ...gatling.Option`. `AGENTS.md` lists
> changing a public API signature as ask-first, and Principle V requires the change in
> `CHANGELOG.md`. See [§5 Approval](#5-approval) for exactly what is being asked.

---

## 1. `package gatling` — additions

Stdlib only (Principle IV). No existing identifier changes meaning or signature.

### 1.1 Format

```go
// Format is the on-disk shape of a Gatling simulation.log. It is a property of
// the bytes in the file and never of its name.
type Format uint8

const (
    // FormatUnknown is the zero value: nothing has been detected. Detect never
    // returns it with a nil error.
    FormatUnknown Format = iota
    // FormatText is the tab-separated log written through Gatling 3.12.0.
    FormatText
    // FormatBinary is the binary stream written from Gatling 3.13.0.
    FormatBinary
)

// String names the format: "text", "binary", or "unknown" for the zero value.
func (f Format) String() string

// DetectSize is the number of leading bytes Detect examines. It is the length
// of the longest opening a text log may have, "ASSERTION\t"; a caller doing its
// own buffering can size it from here.
const DetectSize = 10

// Detect names the format a simulation.log is written in, from its leading
// bytes alone. head may be shorter than DetectSize; Detect decides as soon as
// the bytes are conclusive.
//
// A binary log opens with the run record's kind byte, 0x00. A text log opens
// with RUN or ASSERTION followed by a tab — a simulation that declares
// assertions writes those ahead of the run header, so both are ordinary.
//
// It returns a *FormatError when the bytes are not a Gatling simulation.log,
// and one whose Short field is set when the input ended before the format
// could be told. It never consults a file name, and it never guesses.
func Detect(head []byte) (Format, error)
```

**Behaviour table** — every row is an automated check (FR-029):

| `head` | `Format` | `error` |
|---|---|---|
| `{0x00, …}` | `FormatBinary` | `nil` |
| `"RUN\tsim…"` | `FormatText` | `nil` |
| `"ASSERTION\tAAEB…"` | `FormatText` | `nil` |
| `"ASSERT"`, input ended | `FormatUnknown` | `*FormatError{Short: true}` |
| `""` | `FormatUnknown` | `*FormatError{Short: true}` |
| `"RUNX\t…"` | `FormatUnknown` | `*FormatError{Short: false}` |
| `"<html>…"`, `{0x1f, 0x8b, …}` | `FormatUnknown` | `*FormatError{Short: false}` |
| `"RUN "` (space, not tab) | `FormatUnknown` | `*FormatError{Short: false}` |

### 1.2 Read options

```go
// Option varies one read. Every Gatling codec accepts the same options, so a
// caller states its policy once whichever format it turns out to be reading.
//
// The configuration an Option writes to is this package's own: a caller uses
// the options defined here and cannot invent one, which keeps the vocabulary of
// a read in the package that has to honour it.
type Option func(*readOptions)

// WithStrict makes a read refuse a Gatling version no recording covers, rather
// than decoding it with a warning. Use it where an unverified number is worth
// less than no number — a release gate, an automated comparison.
//
// It changes nothing else: a version inside the range reads identically, and
// one below it is refused either way.
func WithStrict() Option
```

Two exported identifiers, not four. `readOptions` is unexported deliberately (research R13): a
codec forwards `opts ...Option` to `Policy.Apply` and never inspects them, and a consumer defining
its own option would be defining behaviour this package has to honour. `WithStrict` carries the
`With` prefix Go uses for functional options — mixing `With*`, `Set*` and `Use*` across one codebase
is the anti-pattern the convention exists to prevent.

### 1.3 Version policy

```go
// Policy is one codec's version policy: the range its golden corpus covers.
// Widening it means recording a new corpus entry first.
type Policy struct {
    // Min and Max bound the range this codec accepts without a warning.
    Min, Max Version
}

// Apply resolves the policy for the version a log names. It is the single place
// the outcomes are decided, so that two codecs cannot drift apart on them, and
// it is called before any record is decoded.
//
// It returns the zero Warning and a nil error for a version inside the range;
// a Warning and a nil error for a version above it; a *VersionError for a
// version below it; and an *UnverifiedError for a version above it under
// WithStrict.
func (p Policy) Apply(found Version, opts ...Option) (Warning, error)
```

`Gate(found, lo, hi) Verdict` *(existing)* is unchanged and is what `Apply` is built on.

### 1.4 Errors

```go
// FormatError ends a read before anything is decoded: the bytes at the start of
// the stream are not a Gatling simulation.log.
type FormatError struct {
    // Head is the leading bytes that were examined, at most DetectSize of them.
    Head []byte
    // Short says the input ended before the format could be told, rather than
    // being long enough and matching neither format.
    Short bool
}

func (e *FormatError) Error() string

// UnsupportedFormatError ends a read: the stream is a Gatling simulation.log in
// a format this module cannot read yet. It is not a damaged log and not an
// unknown one.
type UnsupportedFormatError struct {
    // Format is the format that was detected.
    Format Format
}

func (e *UnsupportedFormatError) Error() string

// UnverifiedError ends a read that asked for strictness: the log names a
// version above the range any recording covers, so nothing proves the records
// would be decoded correctly.
type UnverifiedError struct {
    // Version is the release that wrote the log.
    Version Version
    // Min and Max bound the range recordings cover.
    Min, Max Version
}

func (e *UnverifiedError) Error() string
```

Message shapes, all prefixed `gatling: ` like the existing ones:

| Type | Message |
|---|---|
| `FormatError` (long enough) | `not a Gatling simulation.log: found "<html><hea" at the start of the stream` |
| `FormatError` (short) | `not a Gatling simulation.log: the input ended after 6 bytes, before the format could be told` |
| `UnsupportedFormatError` | `binary simulation.log: this module has no codec for it yet` |
| `UnverifiedError` | `version 3.99.0 is above the verified range 3.11.5 through 3.12.0: no recording covers it, and this read is strict` |

`Head` is rendered quoted and printable, so a gzip stream produces a readable message rather than a
spray of bytes.

---

## 2. `package gatling/text` — changed signatures

```go
func NewReader(r io.Reader, opts ...gatling.Option) (*Reader, error)
func NewRunReader(r io.Reader, opts ...gatling.Option) (*RunReader, error)
```

Everything else in the package is untouched: `Reader.Header`, `Assertions`, `Warnings`, `Next`,
`RunReader.Run`, `RunReader.Next`, `SupportedVersions`, `Capabilities`, `Tool`, `MaxLineLen`.

**Observable behaviour is unchanged for every existing call site** (FR-032, SC-010): `NewReader(r)`
compiles and behaves exactly as before, because passing no options is the lenient default. The
only internal change is that `finishPreamble` calls `Policy.Apply` instead of switching on `Gate`
itself — the same three outcomes, decided in one place instead of two.

---

## 3. `package gatling/simlog` — new

```go
// Package simlog opens a Gatling simulation.log without being told which
// Gatling wrote it: it identifies the format from the file's leading bytes and
// hands the stream to the codec that reads it.
//
// Use it where the version is unknown — an archived run, a log someone sent.
// Where it is known, the codec package is one call shorter.
package simlog
```

### 3.1 Readers

```go
// RecordReader yields a log's own wire records. *text.Reader satisfies it, and
// so will the binary codec.
type RecordReader interface {
    Header() gatling.Header
    Assertions() []string
    Warnings() []gatling.Warning
    Next() (gatling.Record, error)
}

// RunReader yields canonical results. *text.RunReader satisfies it.
type RunReader interface {
    Run() model.Run
    Next() (model.Item, error)
}

// NewReader identifies the format of the log in r and returns a reader for its
// wire records. The records are identical to those the codec for that format
// yields when handed the same log directly.
//
// It returns a *gatling.FormatError when r is not a Gatling simulation.log, and
// a *gatling.UnsupportedFormatError when it is one in a format this module
// cannot read yet — today, every binary log. The version gate is the codec's
// and is applied once: a version below the supported range is refused with a
// *gatling.VersionError, and one above it decodes with exactly one warning, or
// is refused with a *gatling.UnverifiedError under gatling.WithStrict.
func NewReader(r io.Reader, opts ...gatling.Option) (RecordReader, error)

// NewRunReader is NewReader for canonical results: the same identification, the
// same gate, model.Item values instead of wire records.
func NewRunReader(r io.Reader, opts ...gatling.Option) (RunReader, error)
```

### 3.2 Coverage

```go
// Support is what this module does with one Gatling log format.
type Support struct {
    // Format is the format described.
    Format gatling.Format
    // Readable says whether this module has a codec for it.
    Readable bool
    // Oldest and Newest bound what that codec accepts without a warning. They
    // hold their zero value when Readable is false.
    Oldest, Newest gatling.Version
}

// Supported returns one entry per Gatling log format this module knows about,
// in a fixed order, so a consumer can report what parsec reads without naming a
// format or a version itself. A format with no codec yet is reported as exactly
// that, which is a different answer from an unknown format.
//
// The ranges are the codecs' own, bound to the golden corpus; a caller cannot
// widen one.
func Supported() []Support
```

**Why these constructors return an interface.** Go's default is to return a concrete type, and to
wait for a second implementation before extracting an interface. Both are set aside here on purpose,
and the reason is in the function's job: `NewReader` picks the codec, so it cannot name one concrete
type in its signature without lying about what it does. Returning `*text.Reader` today would have to
become an interface the moment the binary codec lands, which is a breaking change scheduled for
v0.0.5 rather than a hypothetical one. The interfaces are also declared where they are consumed —
in `simlog`, which returns them — not beside the implementation, so `text` does not import them and
the binary codec will not have to either.

Today's return value:

| `Format` | `Readable` | `Oldest` | `Newest` |
|---|---|---|---|
| `FormatText` | `true` | 3.11.5 | 3.12.0 |
| `FormatBinary` | `false` | zero | zero |

---

## 4. What is deliberately *not* here

- **No binary version reading.** FR-031b: nothing in this feature decodes a binary run header, so
  `Supported()` reports the binary format as not readable and says nothing about its versions.
- **No path- or name-based entry point.** FR-002. Locating a run directory is v0.0.10 (issue #11).
- **No new `model/` field, no new `Capabilities` bit, no new record kind.**
- **No statistic.** Principle I.
- **No `binary` package.** v0.0.5 (issue #6).

---

## 5. Approval

Two decisions need a maintainer's yes before implementation begins:

1. **`text.NewReader` and `text.NewRunReader` gain `opts ...gatling.Option`.** Source-compatible:
   every existing call site compiles unchanged and behaves identically. It breaks only a caller that
   assigned the constructor to a `func(io.Reader) (*text.Reader, error)` variable — nothing in this
   repository does, and Principle V permits the change before v0.1.0 provided it is recorded.
2. **A new package `gatling/simlog`.** The dispatcher cannot live in `gatling` (import cycle,
   research R1). The name and the two constructor names are the part worth objecting to now rather
   than after they are published.

## 6. `CHANGELOG.md` plan (Keep a Changelog, same PR)

**Added**

- `gatling.Format`, `gatling.Detect` and `gatling.DetectSize`: identify a `simulation.log` as text
  or binary from its leading bytes, never from its name.
- `gatling.Option` and `gatling.WithStrict`: a caller can refuse a Gatling version no recording
  covers instead of decoding it with a warning.
- `gatling.Policy` and `Policy.Apply`: the version policy in one place, shared by every codec.
- `gatling.FormatError`, `gatling.UnsupportedFormatError` and `gatling.UnverifiedError`.
- `gatling/simlog`: opens a `simulation.log` without being told which Gatling wrote it, and
  `simlog.Supported` reports what this module reads.

**Changed**

- `text.NewReader` and `text.NewRunReader` take `opts ...gatling.Option`. Existing calls compile and
  behave unchanged; the default is lenient, as before.
