# Contract 4 — a codec handed the other format's log

**Applies to**: `text.NewReader`, `text.NewRunReader`, `binary.NewReader`, `binary.NewRunReader`,
and `gatling.UnsupportedFormatError`.
**Status**: the most likely first failure a new consumer meets, currently sorted into the wrong error
type. Issue [#84](https://github.com/galax-io/parsec/issues/84), with #107's decision to keep
`UnsupportedFormatError`; recorded under **Changed**.

## What happens today

Verified against the corpus on 2026-09-12:

```
text.NewReader(testdata/corpus/gatling/3.14.9/simulation.log)   -> *gatling.SyntaxError
    gatling: line 1: expected ASSERTION or RUN before the run header,
    found "\x00\x00\x00\x00\x063.14.9\x00…" (98 bytes)

binary.NewReader(testdata/corpus/gatling/3.11.5/simulation.log) -> *gatling.SyntaxError
    gatling: byte 0: expected the run record, found byte A
```

`*SyntaxError` is documented as a damaged log: "the position it names could not be decoded". The log
is not damaged. A consumer with a mixed archive that catches `*SyntaxError` and quarantines the file
as corrupt quarantines every log of the other format.

## The rule

A codec handed a log of the **other** format returns `*gatling.UnsupportedFormatError`, carrying the
format it found, wrapped with a hint naming the entry point that reads both.

```
text.NewReader   on a binary log -> *gatling.UnsupportedFormatError{Format: FormatBinary, Head: …}
binary.NewReader on a text log   -> *gatling.UnsupportedFormatError{Format: FormatText,   Head: …}
```

A log of the reader's **own** format that is damaged still returns `*SyntaxError`; one cut short still
returns `*TruncationError`; one that is not a Gatling simulation.log at all still reaches
`*FormatError` where it does today. `simlog` is unchanged: it is already right.

## Where the answer comes from

`gatling.Detect` needs `gatling.DetectSize` (10) bytes, and both codecs hold more at the point they
fail — so **no byte is read twice**.

- **Text**: the preamble scanner has read a whole line — 98 bytes in the reproduction above.
- **Binary**: `NewReader` fails at byte 0, having consumed one byte through `reader.u8`. Its
  `reader.src` is a `*bufio.Reader`, so the bytes are in the buffer. The peek is taken **before** the
  `u8` call: `src.Peek(gatling.DetectSize)` consumes nothing and costs no extra read. Peeking after
  the `u8` would hand `Detect` bytes 1..10 and misidentify the log.

## The message

`UnsupportedFormatError.Error` today reads `"gatling: %s simulation.log: this module has no codec for
it yet"`, which is false here: the module reads it, just not in this package. It is reworded to say
what is true of both its uses — the reader in hand does not decode this format. `simlog` already
constructs this type for a known format with no reader (`gatling/simlog/simlog.go:168`, `:184`), a
branch nothing reaches while both formats have codecs, and the reworded message must stay true there.

Each codec wraps it with `%w`, naming `gatling/simlog` as the entry point that reads both formats
without being told which. The routing datum a consumer branches on is `Format`, which the type
already carries; the package name is a hint for a human, and a hint belongs in a message. No field is
added — this feature is shrinking the surface, and `Format` already carries what a program needs.

## What does not change

- Neither codec learns to read the other format (#84's non-goal).
- `simlog`'s behaviour, including its own `UnsupportedFormatError` construction.
- `FormatError`, which stays "these bytes are not a Gatling simulation.log".
- `Head`'s contract: the leading bytes are returned because reading them consumed them, so a caller
  holding an unrewindable stream can still spool the log aside.

## Acceptance

| Given | Then |
|---|---|
| `text.NewReader` on a 3.14.9 binary corpus log | `*UnsupportedFormatError`, `Format == FormatBinary`, message names `gatling/simlog`; **not** a `*SyntaxError` |
| `binary.NewReader` on a 3.11.5 text corpus log | the same in reverse, `Format == FormatText` |
| a damaged log of the reader's own format | `*SyntaxError`, as today |
| a log cut short, of the reader's own format | `*TruncationError`, as today |
| a gzip or any non-Gatling file | unchanged |
| either wrong-format construction | the head bytes were read once |
