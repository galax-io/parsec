# Contract 1 — Endings and source failures

**Applies to**: `simlog.NewReader`, `simlog.NewRunReader`, `binary.NewReader`, `binary.NewRunReader`,
`text.NewReader`, `text.NewRunReader`, and the `Next` methods of what they return.
**Status**: what the package documentation already promises (`gatling/simlog/doc.go`, "Following a
log that is still being written"); this feature makes the code keep it. Issues #82, #83, #102.

## The promise

A read of a `simulation.log` ends in exactly one of four ways, and a caller can tell which without
reading a message string:

| Ending | How it arrives | What the caller may keep |
|---|---|---|
| **Clean** | `io.EOF` from `Next` | everything |
| **Cut short** | `*gatling.TruncationError` | everything before the cut |
| **Not yet identifiable** (`simlog` only) | `*gatling.FormatError` with `Short: true` | nothing; come back with more bytes |
| **Failed** | any other error — `*SyntaxError`, `*VersionError`, `*UnverifiedError`, `*FormatError` with `Short: false`, or a failure of the source | nothing |

## What decides between them

1. **Only the stream itself ending — `io.EOF` from the source, compared by identity — may produce
   Clean, Cut short or Not yet identifiable.** An error that merely wraps `io.EOF`, and
   `io.ErrUnexpectedEOF` however it arrives, is a failure of the source and ends the read as Failed.
2. **No error returned for a failure of the source satisfies `errors.Is(err, io.EOF)`.** The message
   keeps the cause's text, and every other cause in the chain stays reachable through `errors.Is` and
   `errors.As` — a `context.Canceled`, an `*fs.PathError` — while `io.EOF` alone is hidden.
3. **A read that fills the value being read and reports `io.EOF` in the same call is a successful
   read**: the source has ended, and the log ends Clean at the next record boundary. Any other error
   the source returns beside the last bytes is a failure, and the read ends as Failed. It is not left
   for the source to repeat, because bufio hands such an error over once and forgets it.
4. **Cut short is claimed only by the codec's own loop.** A `*gatling.TruncationError` is raised
   when the bytes ran out inside a record and every record before it was delivered; it is never
   raised for a source failure, and it is never the answer to a complete log (#82).

## What a follower may rely on, restated

- A `Read` that blocks is a wait. A `Read` that returns fewer bytes than asked for is a wait. A
  `Read` that returns bytes *and* `io.EOF` is the last read, and the bytes count.
- A torn transport, a truncated archive, a broken decompressor — inside the first ten bytes or in
  the last record — is reported as a failure carrying its cause's text, never as an end and never as
  "not yet".

## What changes for a consumer, in `CHANGELOG.md` terms

**Fixed**

- A complete binary log read through a source that returns its final bytes together with `io.EOF`
  — an HTTP body, `iotest.DataErrReader` — was reported as cut short with every byte counted as
  dropped and no record delivered, whenever a value of at least 64 KiB ended the file. (#82)
- `simlog` returned an error satisfying `errors.Is(err, io.EOF)` for a source that failed with an
  error wrapping `io.EOF`, so a torn upload read as the end of the log. Only `io.EOF` is hidden now:
  the message keeps the cause's text, and every other cause stays reachable through `errors.Is` and
  `errors.As` — in `simlog`, and in both codecs, which until now cut the whole chain. (#83)
- `simlog` reported a source's `io.ErrUnexpectedEOF` — what a truncated gzip returns — as a stream
  too short to identify, the answer a follower retries on. It is a source failure. (#102)

## Verified by

`TestASourceFailureIsNeverTheEndOfTheLog` (six constructors × four sources, `gatling/simlog`), the
`readFull` unit test and the 200,000-byte-payload fixture (`gatling/binary`), the truncated-gzip case
and the `Short` assertion (`gatling/simlog`), and `iotest.DataErrReader` in the binary chunk matrix.
See [quickstart.md](../quickstart.md) §1–2.
