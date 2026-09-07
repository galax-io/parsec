# Research: An incomplete record

**Feature**: 008-incomplete-record | **Date**: 2026-09-07 | **Constitution**: v2.2.0

Ten decisions. Each names what was chosen, why, and what was rejected. Two of them (R5, R6) are
findings from reading the code rather than choices, and both change what the milestone has to build.

---

## R1 — How the end of a cut-short read is signalled

**Decision**: a new exported type, `*gatling.TruncationError`, returned from `Next` in place of the
`*gatling.SyntaxError` that ends such a read today. It is terminal, exactly as every other read
failure in these codecs is: once returned, every later call returns it unchanged. It is **not**
`io.EOF` and does not wrap it.

**Rationale**: FR-004 requires the ending of a cut-short read to differ from the ending of a clean
one, and to differ in the direction that fails safe. A consumer written before v0.0.8 breaks its
loop on `errors.Is(err, io.EOF)` and treats anything else as a failed read. With a distinct error
type it keeps doing exactly that — the same outcome it has today — and an updated consumer opts in
with `errors.As`. The reverse choice would silently convert a killed run into a complete one for
every consumer that had not yet been updated, which is the failure this feature exists to remove.

The module already carries this distinction one layer up: `gatling.Detect` returns a
`*FormatError` whose `Short` field means *the bytes ran out while still a possible opening*, as
opposed to *these bytes are not a Gatling log*. "Cut short is not the same as damaged" is therefore
an existing idea in this codebase, and R1 extends it from the opening bytes to the record loop.

**Alternatives considered**:

| Rejected | Why |
|---|---|
| `io.EOF` plus a `Truncated()` accessor | A flag not consulted is a flag that lies. Every existing consumer loop would report a cut run as complete on the day it upgraded, with no compile error and no test failure to warn it. |
| A `gatling.Warning` and a clean end | Warnings are the version gate's channel, and a warning does not end a read. Reusing it would put two unrelated conditions in one list and still end the read cleanly. |
| A `Truncated bool` on the existing `SyntaxError` | `errors.As` could no longer separate the two conditions; a caller would have to match the type and then remember to read a field. Distinct conditions get distinct types. |
| A package-level sentinel (`var ErrTruncated = …`) alone | It carries no position and no byte count, which FR-003 requires. The type can grow an `Is` method later if a sentinel turns out to be wanted; adding one now is a second way to say the same thing. |

## R2 — Whether `TruncationError` unwraps to `*SyntaxError`

**Decision**: no `Unwrap`. The type stands alone.

**Rationale**: it would exist only to keep `errors.As(err, &syntaxErr)` matching for consumers
written against v0.0.7. That match is not load-bearing for safety — an un-updated consumer already
treats any non-EOF error as a failed read, which is the fail-safe outcome R1 is chosen for — and
keeping it would permanently record a cut-short log as a *syntax* problem, entrenching the confusion
#7 exists to end. Principle VI: no abstraction without a current need.

**Alternatives considered**: wrapping the `*SyntaxError` the codec builds today (rejected above);
wrapping `io.ErrUnexpectedEOF` (rejected — it invites `errors.Is(err, io.ErrUnexpectedEOF)` to be
read as "the source failed", which is a different condition, see R5).

## R3 — Where the position and the dropped-byte count come from

**Decision**: each codec fills the fields it can measure exactly, from data it already has.

**Binary**. `reader.fixed` already computes the offset at which the stream actually stopped —
`at + read`, where `read` is `io.ReadFull`'s short count — and its doc comment already explains that
this, not the offset of the enclosing value, is where a reader would open the file. What it lacks is
the offset the *record* began at. `Reader.Next` knows it: it is `r.rd.off` at the moment `atEnd`
reports the stream is not exhausted. A single field on `reader`, set there, gives
`Dropped = stopOffset − recordStart` and `Offset = recordStart`.

**Text**. `scanner.next` already returns the bytes of an unterminated final line together with
`isTerminated == false`; `Reader.Next` discards them today and raises `unterminated(lineNo)`.
`Dropped` is `len(data)` and `Line` is that line number. Nothing new is measured.

**Rationale**: both numbers are exact, both are already computed or already in hand, and neither
requires the reader to hold anything it does not hold today — which is what keeps FR-011 true.

**Alternatives considered**: counting the dropped bytes by draining the source to its end (rejected:
it is unbounded work on a source that may still be growing, and on a blocking source it never
returns); reporting only that a cut happened, with no count (rejected: FR-003, and an operator
comparing a log against its file size has nothing to compare).

## R4 — A cut before the run header

**Decision**: `NewReader` returns a `*TruncationError` too, and returns no reader. FR-007 is
unchanged — there is no run, no version verdict and no record to salvage — but the *kind* of failure
is now stated.

**Rationale**: the sidecar attaching to a run in its first milliseconds sees exactly this file. "The
log was cut short, come back with more bytes" and "these bytes are not a Gatling log" call for
opposite responses, and today they are the same `*SyntaxError`. `Detect` already draws this line for
the opening bytes with `FormatError.Short`; drawing it in the preamble too is the same line, one
layer down.

**Alternatives considered**: leaving the preamble on `*SyntaxError` so that a `*TruncationError`
always implies records were delivered (rejected: it makes the error type mean *salvage happened*
rather than *the log was cut short*, and the caller can already tell the two apart — a constructor
that fails returned no reader, so no record can have been delivered).

## R5 — A source failure that wraps `io.EOF` is currently read as a truncation

**Finding, not a choice.** `binary/read.go`'s `sourceFailed` decides with `errors.Is`:

```go
if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) { return r.truncated(...) }
```

while its own doc comment says those two "alone become a truncation", and its three siblings —
`reader.atEnd`, `simlog.identify` and `simlog.readHead` — all compare with `==` and each carries a
recorded reason for doing so: an error that merely *wraps* `io.EOF` is a source reporting a failure
of its own, such as a truncated decompressor or a closed transport, and must not be read as the end
of the artefact. `bufio.Reader.Read` passes the source's error through unchanged, so such an error
reaches `sourceFailed` and is converted into a truncation today.

**Decision**: compare with identity, matching the siblings and the comment that is already there.
This lands with #7, because #7 is what makes the distinction observable: before it, both outcomes
were the same fatal error and the defect could not be seen from outside.

**Alternatives considered**: leaving it and documenting the behaviour (rejected: FR-006 states the
opposite, and a consumer told its log is truncated will re-record a run that was never damaged —
the reason `sourceFailed` wraps its cause in the first place).

## R6 — #10 needs no change to any read loop

**Finding, measured 2026-09-07.** A probe was written against the public entry point
(`simlog.NewReader`) with a source that blocks while it has no bytes, appends 300 at a time, and
reports the end only when the writer is done. All five corpus recordings decode to a record stream
identical to a whole-file read: 66 records for 3.11.5 and 3.12.0, 132 for 3.13.1, 3.14.9 and 3.15.1.
No source file was changed to make that pass.

It works because the state a follower needs already lives across records: the binary codec's string
cache and group path are fields of `Reader`, the version gate runs once in the constructor, and both
codecs read through a `bufio.Reader` whose fill is a single `Read` — so a short read is a wait, not
an end. The reciprocal hazard, a source that returns `(0, nil)` forever instead of blocking, is
already guarded twice: `simlog.readHead` counts empty reads and gives up with `io.ErrNoProgress`,
and `bufio` does the same inside `fill`.

**Decision**: #10 ships as a **contract and a test**, not as code: the guarantee stated in the
documentation a consumer reads, and a test that fails if a later change removes it. The probe
becomes that test.

**Alternatives considered**: a push-style parser fed byte slices (rejected in the issue and in
comet#3 — a second public decoding interface to freeze at v0.1.0, buying nothing the pull readers do
not already do); re-reading the file each interval (rejected: quadratic in run length, and the
string cache would be rebuilt every time).

## R7 — Where cut logs come from

**Decision**: cut inputs are produced in-test by truncating the committed recordings, and none is
committed.

**Rationale**: Principle III — a hand-edited artefact is a fixture, not corpus, and must say so in
its name. Deriving them at read time keeps the corpus what it claims to be: five runs exactly as
Gatling produced them. It also gives the coverage FR-013 needs — every offset in the last 200 bytes
of every recording, 1000 reads — which no committed set of files would.

**Alternatives considered**: committing a handful of pre-cut logs under a `fixtures/` tree
(rejected: one file per interesting offset is either a huge tree or a thin sample, and the
interesting offsets are exactly the ones nobody thinks to pick).

## R8 — Fuzzing the cut

**Decision**: extend the seed corpora of the two decoding targets, `binary.FuzzDecode` and
`text.FuzzReader`, with prefixes of the recordings. `gatling.FuzzDetect` needs nothing: short input
is its native case and `FormatError.Short` is already its answer.

**Rationale**: Principle II forbids a panic on any input, and a truncated prefix is the input class
this feature creates handling for. Seeds direct the fuzzer at it from the first run rather than
leaving it to find the shape by mutation. The pull-request fuzz leg added in v0.0.7 gives them a
budget on every change.

## R9 — Engineering guidance actually consulted

Required-reading rows this change triggers, per the constitution's Engineering Guidance:
`golang-error-handling` (a new error type and the identity-vs-`errors.Is` decision in R5),
`golang-naming` (one new exported identifier), `golang-documentation` (its doc comment, and the
contract text FR-009 requires), `golang-structs-interfaces` (an exported struct type),
`golang-testing` (the table-driven cut-offset sweep and the blocking-source test).

Disagreements with this constitution, recorded as the constitution requires: the Go skill set
recommends `stretchr/testify` for assertions and `samber/*` helpers. Both are forbidden here
(Principle IV; AGENTS.md *Never*), and neither is used. No other disagreement arose.

## R10 — Performance

**Decision**: the budget is unchanged, and the benchmarks that measure it are the ones already in
`gatling/binary/bench_test.go` and `gatling/text`.

**Rationale**: the change adds one `int64` assignment per record on the binary path and nothing at
all on the text path, allocates only when a read is already ending, and holds no additional memory
while a follower waits. A measurable regression would mean something other than this design was
built.
