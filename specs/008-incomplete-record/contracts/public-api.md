# Contract 1 — public API

**Status**: **approved 2026-09-07.** One addition and one observable change, permitted before
v0.1.0 (Principle V) and recorded under **Changed** in `CHANGELOG.md` in the same PR. The freeze [parsec#13](https://github.com/galax-io/parsec/issues/13)
asks for comes after this milestone, which is why the change lands now rather than being deferred
past it.

## Added

```go
package gatling

// TruncationError ends a read whose bytes ran out inside a record: the log was
// cut short rather than damaged. Every record decoded before the cut has already
// been delivered and is exactly what a whole-file read of the same prefix yields.
//
// It is what a run killed mid-flight leaves behind — Gatling writes the log
// through a buffer, so an aborted test always ends inside a record — and it is
// not io.EOF: a caller that treats any non-EOF error as a failed read keeps
// doing so, and one that wants the partial run asks for this type by name.
//
// Whether a run whose log was cut short may be used at all is the caller's to
// decide. This package states the fact and computes nothing from it.
type TruncationError struct {
    // Format is the codec that was reading.
    Format Format
    // Line is the 1-based line the incomplete record began on, for a text log.
    Line int
    // Offset is the byte the incomplete record began at, for a binary log.
    Offset int64
    // Dropped is how many trailing bytes could not be decoded. It is always
    // positive: a cut with nothing dropped is a clean end and is reported as one.
    Dropped int64
    // Expected is what the codec was reading when the bytes ran out.
    Expected string
}

func (e *TruncationError) Error() string
```

There is no `Unwrap`: the type does not wrap `io.EOF`, `io.ErrUnexpectedEOF` or `*SyntaxError`
(research [R2](../research.md)). `errors.As` is how a caller reaches it.

## Changed

| Entry point | v0.0.7 | v0.0.8 |
|---|---|---|
| `text.Reader.Next`, `binary.Reader.Next`, `simlog.RecordReader.Next` | a log cut inside a record ends with `*SyntaxError{Found: "end of input"}`, and the records already delivered "are not a result" | ends with `*TruncationError`; the records already delivered **are** what the log recorded, and the doc comment says so |
| `text.RunReader.Next`, `binary.RunReader.Next`, `simlog.RunReader.Next` | as above | the same value, passed through unchanged |
| `text.NewReader`, `binary.NewReader`, `simlog.NewReader` / `NewRunReader` | a cut inside the preamble or run record ends with `*SyntaxError` | ends with `*TruncationError`, and still returns no reader |
| `binary` source failures wrapping `io.EOF` | reported as a truncation | reported as the source failure it is, cause wrapped (research [R5](../research.md)) |

## What a consumer has to do

| Consumer | Action needed |
|---|---|
| Written for v0.0.7, not updated | **None.** It breaks its loop on `io.EOF` and treats everything else as a failed read — the same outcome it gets today. A cut run cannot be mistaken for a complete one. |
| Wants the partial run | `errors.As(err, &truncErr)` after the loop ends, then decide its own policy. |
| Matches `*SyntaxError` to report a damaged file | Still correct, and now more nearly true: a cut log no longer arrives as a syntax error. |

## Doc-comment obligations (FR-009)

The blocking-source contract in [contract 2](blocking-source.md) is stated on the package
documentation of `gatling/simlog` and on both `NewReader` doc comments — not only in a test. Every
new exported identifier carries a doc comment saying what it does (Principle V).
