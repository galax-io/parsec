# Data Model: An incomplete record

**Feature**: 008-incomplete-record | **Date**: 2026-09-07

No type in `model/` changes. The feature adds one type to `gatling/` and gives the three possible
endings of a read three distinguishable shapes.

---

## The three endings of a read

Every read of a `simulation.log` now ends in exactly one of these, and a consumer can tell which
without inspecting a message string.

| Ending | What it means | How it arrives |
|---|---|---|
| **Clean** | The log ended at a record boundary. Every record it held was delivered. | `io.EOF` from `Next` |
| **Cut short** | The bytes ran out inside a record. Every record that completed was delivered; the rest of that record was never written. | `*gatling.TruncationError` from `Next`, or from the constructor when the cut is in the preamble |
| **Failed** | The artefact is not decodable, or the source itself failed. Nothing may be derived from what was delivered. | `*gatling.SyntaxError`, `*gatling.FormatError`, `*gatling.VersionError`, `*gatling.UnverifiedError`, or a wrapped source error |

The endings are ordered by how much the caller may keep: everything, everything up to the cut, and
nothing. Each is terminal — the same value is returned by every later call, which is the contract
both readers already state.

The binary format cannot distinguish **Clean** from **Cut short** when the cut lands exactly on a
record boundary: it carries no end marker, so such a file is a shorter valid log. That is a property
of the artefact, recorded here so that no consumer reads *clean* as *complete*.

## Entity: `gatling.TruncationError`

The account of a read that ran out of bytes inside a record. It travels as the error that ends the
read; it is never only logged.

| Field | Type | Meaning | Text | Binary |
|---|---|---|---|---|
| `Format` | `gatling.Format` | which codec was reading | `FormatText` | `FormatBinary` |
| `Line` | `int` | 1-based line the incomplete record began on | set | zero |
| `Offset` | `int64` | byte offset the incomplete record began at | zero | set |
| `Dropped` | `int64` | trailing bytes that could not be decoded | length of the unterminated final line | stop offset − record start |
| `Expected` | `string` | what the codec was reading when the bytes ran out | the line's kind, where known | the value being read, e.g. `"a request record's name"` |

**Invariants**

1. `Dropped > 0`. A cut with nothing dropped is not a cut record, and is not reported as one. What
   it is instead depends on where it lands, and neither case is knowable from the artefact:
   - In the record stream, a binary log is a shorter valid log and a text log ends on a line
     boundary; both end cleanly with `io.EOF`.
   - In the preamble, a text log that ends on a line boundary without a run header is a log with no
     header, reported as a `*SyntaxError` — there is no partial line to wait for. An empty stream
     handed to `binary.NewReader` is the same case.

   This is the same indistinguishability the binary format's missing end marker creates, one layer
   up: a cut that lands on a boundary leaves no evidence of itself.
2. Exactly one of `Line` and `Offset` is set, chosen by `Format`, matching `SyntaxError`'s existing
   convention so the two read alike.
3. The position names where the **incomplete record began**, not where the stream stopped. That is
   the offset a reader would open the file at to see what was lost; the stop offset is
   `Offset + Dropped`.
4. It never wraps `io.EOF`, `io.ErrUnexpectedEOF` or a `*SyntaxError` (research R2), so
   `errors.Is(err, io.EOF)` is false and a consumer written before v0.0.8 sees a failed read.
5. It is raised only by the record loop and the preamble — never for a source failure, which keeps
   its own error and its own cause (research R5).

**Lifecycle**

```text
                      ┌──────────────► io.EOF                     (clean)
   NewReader ──► Next ┼──────────────► *TruncationError           (cut short, records kept)
        │             └──────────────► *SyntaxError / source err  (failed)
        │
        └── cut inside a preamble line ► *TruncationError, no Reader (cut short, nothing kept)
```

## What each reader shape carries

| Shape | Carries the cut how |
|---|---|
| `text.Reader`, `binary.Reader`, `simlog.RecordReader` | the terminal error from `Next` |
| `text.RunReader`, `binary.RunReader`, `simlog.RunReader` | the same value, passed through unchanged — both `RunReader.Next` implementations return the wire reader's error verbatim, so FR-012 costs no plumbing |

`model.Run` is unchanged. A cut is a property of the read, not of the run the log describes, and
`Run()` is answerable before the first record is decoded — long before a cut can be known.

## What cannot be told apart

Two shapes are reported as something other than a cut, and no reader can do better. Both are
documented rather than guessed at:

| Shape | Reported as | Why nothing better is possible |
|---|---|---|
| A binary log cut exactly on a record boundary | a clean end | The format has no end marker, so the file *is* a shorter valid log. Only something that can see the writer knows otherwise. |
| A corrupted length prefix claiming more bytes than the file holds | a cut short | It is byte-for-byte what a file cut mid-value looks like. Either way the value is not delivered, which is what matters to the caller. |

## What does not change

- Every field of every record, and the order records arrive in. A truncated read delivers exactly
  what a whole-file read of the same prefix delivers (FR-014).
- `model.Capabilities`, the version gate, `MaxStringLen`, `MaxLineLen`, and the peak-memory bound.
- `gatling.Warning`, which stays the version gate's channel alone.
