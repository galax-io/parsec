# Contract: the binary codec and what it shares

**Feature**: `005-gatling-binary-decoder` | **Date**: 2026-09-06 | **Spec**: [spec.md](../spec.md)

Every identifier is new unless marked *(existing)*. Principle V requires a doc comment on each one
stating what it does and, for a decoder, which tool versions it accepts.

> **Approved 2026-09-06.** Two items change shared code: `gatling.SyntaxError` gains a field, and
> the record-to-model conversion moves out of `gatling/text`. `AGENTS.md` lists a public API change
> as ask-first; both were put to the maintainer and both were approved. See [§5](#5-approval).

---

## 1. `package gatling/binary` — new

Stdlib only (Principle IV). The shapes mirror `gatling/text` exactly, so `gatling/simlog` gains a
table row rather than a special case.

```go
// Package binary decodes the simulation.log Gatling writes from 3.13.0: an
// undocumented binary stream with a string cache and JVM-compact strings. It
// accepts 3.13.1 through 3.15.1, the range its golden corpus covers; 3.13.0
// writes this format but cannot be recorded, so it is refused.
//
// A Reader takes an io.Reader, reads the run record, gates on the version it
// names, then yields one record at a time in file order with memory that does
// not grow with the log. The stream must begin at the first byte of the file:
// the format replaces a repeated string with a back-reference into a table the
// reader rebuilds as it goes, and that table cannot be reconstructed from the
// middle.
package binary

// MaxStringLen is the ceiling on one string or assertion payload, in bytes. A
// length past it fails the read rather than being allocated: the length is read
// straight from the file, so a single corrupt byte would otherwise ask for
// gigabytes. No real log approaches it — the longest field is an assertion
// payload, which runs to tens of kilobytes.
const MaxStringLen = 8 << 20

// SupportedVersions returns the oldest and newest Gatling release this codec
// accepts without a warning: 3.13.1 through 3.15.1, the range the golden corpus
// covers. Widening it means recording a new corpus entry first.
//
// The floor is 3.13.1 and not 3.13.0, which is where the format began: Gatling
// 3.13.0 cannot read back the assertion records it writes, so no run of it
// produces the report a corpus entry needs. A 3.13.0 log is well formed and this
// codec could read it, but the range follows the corpus, so it is refused.
func SupportedVersions() (oldest, newest gatling.Version)

// Capabilities returns what a Gatling binary simulation.log records, and by
// omission what it never does.
func Capabilities() model.Capabilities

// Tool is what this source is called in model.Run.Tool.
const Tool = "gatling"

// Reader decodes a Gatling 3.13.1 through 3.15.1 binary simulation.log.
//
// NewReader consumes the run record and applies the version gate, so Header,
// Assertions and Warnings are available before the first Next.
type Reader struct{ /* … */ }

// NewReader reads the run record and gates on the version it names. It fails
// when the record cannot be read, when the version is below the supported
// range, and when the version is not a plain release. A version above the range
// succeeds and records a warning — or, under gatling.WithStrict, fails with a
// *gatling.UnverifiedError.
//
// The reader holds the run header, the scenario names, a bounded read buffer and
// the string table. The table grows with the number of distinct strings the
// simulation declares, not with the number of records, so a run making millions
// of requests against a handful of names costs a handful of strings.
func NewReader(r io.Reader, opts ...gatling.Option) (*Reader, error)

func (r *Reader) Header() gatling.Header
func (r *Reader) Assertions() []string
func (r *Reader) Warnings() []gatling.Warning

// Next returns the next record, or io.EOF at the end. Any other error ends the
// read: there is no next record after it, the same error is returned on every
// later call, and the records already delivered are not a result.
//
// The returned record's Groups slice is valid until the next call; copy it to
// keep it.
func (r *Reader) Next() (gatling.Record, error)

// RunReader is the model-facing counterpart, exactly as in gatling/text.
type RunReader struct{ /* … */ }

func NewRunReader(r io.Reader, opts ...gatling.Option) (*RunReader, error)
func (x *RunReader) Run() model.Run
func (x *RunReader) Next() (model.Item, error)
```

Both readers satisfy `simlog.RecordReader` and `simlog.RunReader` without an adapter, because the
method sets are the text codec's.

---

## 2. `package gatling` — two fields added

```go
type SyntaxError struct {
    // Line is the 1-based line number for a text log. It is 0 for a binary one.
    Line int
    // Offset is the 0-based byte offset for a binary log. It is 0 for a text
    // one, where Line carries the position instead.
    //
    // Exactly one of the two is meaningful, and Format says which. Principle II
    // asks for "the byte offset (line number for text formats)", so both
    // positions are real and the type carries both rather than pretending one is
    // the other.
    Offset int64
    // Format is the log format the failing decoder was reading. It says which of
    // Line and Offset to read, and it is not redundant with them.
    //
    // The zero value is FormatUnknown, which renders as a line, so an error
    // constructed before this field existed reads exactly as it did.
    Format Format
    Expected string
    Found    string
}
```

`Error()` renders by `Format`: `gatling: line 12: expected …` or
`gatling: byte 4096: expected …`.

**Why `Format` and not just `Offset`.** Neither position discriminates, because both are
legitimately zero. A binary log can fail at byte 0 — an empty file, or a bad first record kind — and
the text codec's empty-input error is `Line: 0` today, verified against the shipped code:

```text
gatling: line 0: expected a run header, found end of input
```

Choosing the rendering from a non-zero `Offset` would therefore print `line 0` for a binary log that
failed at its first byte. The discriminator is the thing actually being discriminated on, so it is
the field.

**Why not a second type.** A caller catching a malformed log wants one `errors.As` for both formats,
and a decoder that fails is the same event whichever grammar it was reading. Two types would make
every consumer branch on the format to ask the same question.

---

## 3. `package internal/wire` — new, not public

The record-to-model conversion, today unexported in `gatling/text`, moves here so both codecs share
one mapping.

```go
// Package wire converts a Gatling wire record into the canonical model.
//
// It is one function of one input, shared by the codecs rather than written
// twice: the mapping is a property of the records, not of the format they were
// read from, so two copies could disagree about what a record means while both
// looked correct.
package wire

// Item fills it from one wire record, returning false for a record that is not
// an event of the run.
func Item(it *model.Item, rec *gatling.Record) bool
```

`internal/` is where Principle I puts a helper shared across packages, and it keeps the conversion
out of the public API where it has never belonged.

**Migration**: `gatling/text`'s `convert` is deleted and its call site changed. No exported
identifier of `gatling/text` changes, and its tests must pass unedited.

---

## 4. `package gatling/simlog` — one table row

```go
{
    format:   gatling.FormatBinary,
    versions: binary.SupportedVersions,
    records:  func(r io.Reader, opts ...gatling.Option) (RecordReader, error) { … },
    run:      func(r io.Reader, opts ...gatling.Option) (RunReader, error) { … },
},
```

No other change. `Supported()` reports the binary format as readable because the row now has
constructors, and the dispatch routes to them because it reads the same row —
the single-statement property v0.0.4 built the table for.

`simlog`'s `NewReader` doc comment stops saying "today, every binary log" is refused.

---

## 5. Approval

**Both approved by the maintainer on 2026-09-06.** Implementation may proceed; task T012 in
[tasks.md](../tasks.md) is satisfied.

1. **`gatling.SyntaxError` gains `Offset int64` and `Format Format`.** Additive: every existing
   construction and every `errors.As` keeps working, `Line` keeps its meaning, and `Format`'s zero
   value renders as a line so existing errors are unchanged. `Format` was not in the first version of
   this contract; it was added during implementation, when `Offset` alone turned out not to be able
   to say which rendering to use (see §2), and approved on 2026-09-06 like the rest. Pre-v0.1.0,
   recorded in `CHANGELOG.md`.
2. **The record-to-model conversion moves to `internal/wire`.** No exported identifier moves — the
   function is unexported today — but it changes where a shared behaviour lives, and Principle I
   names `internal/` as the right home for exactly this.

## 6. `CHANGELOG.md` plan (Keep a Changelog, same PR)

**Added**

- `gatling/binary`: decodes the binary `simulation.log` Gatling writes from 3.13.0, accepting 3.13.0
  through 3.15.1 — the range its golden corpus covers — and yielding the same wire records and
  canonical results as the text codec. 3.13.0 itself is refused: it cannot generate a report, so it
  cannot be recorded, and the range follows the corpus.
- `gatling.SyntaxError.Offset` and `gatling.SyntaxError.Format`: the byte offset for a binary log,
  beside `Line` for a text one, and the format that says which of the two to read.

**Changed**

- `gatling/simlog` now reads a binary `simulation.log` instead of refusing it with
  `*gatling.UnsupportedFormatError`, and `Supported` reports the binary format as readable over
  3.13.1 through 3.15.1.
