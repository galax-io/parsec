# Research: Pre-freeze hardening

**Feature**: 010-pre-freeze-hardening | **Date**: 2026-09-10 | **Constitution**: v2.2.0 (v2.3.0
arrives with PR #110 and changes only the toolchain sentence; nothing below depends on it)

Twelve entries. R1–R7 are the seven fixes, one per issue, in the order they land. R8 covers the two
issues already moving in their own pull requests. R9–R11 are cross-cutting: the shape of the commits
and pull requests, how performance and memory are measured, and which skills were read. R12 records
what reading the code corrected in the spec.

Every entry names what was chosen, why, and what was rejected. File and line references are to
`main` at `410dc54`.

---

## R1 — #82: a read that fills the value is a successful read

**Decision**: `readFull` (`gatling/binary/read.go:93`) returns `(n, nil)` the moment it has filled
`buf` with the source's `io.EOF` beside the final bytes, and the source's error when any other
arrived with them. Only a source `io.EOF` that leaves the buffer short becomes `errCutShort`. It is
the loop condition `n < len(buf)`, not the order of the `switch` arms, that says whether a value is
complete. The doc comment keeps its two stated differences from `io.ReadFull` (an end of stream
becomes `errCutShort`; a stalled source ends in `io.ErrNoProgress`) and gains a third: `io.ReadFull`
clears any error once the buffer is full, and here only `io.EOF` is cleared.

*Corrected after review.* The first version cleared every error, as `io.ReadFull` does, on the
reasoning that a source repeats a real failure. bufio's direct-read path breaks that: it returns
`n, b.readErr()`, and `readErr` clears the error it hands over, so a source that reported a failure
once, beside the last bytes of a value of at least 64 KiB, had that failure read as a complete run,
while the same bytes delivered in small reads reported it, because bufio's own fill keeps the error
for the next call.

**Rationale**: `bufio.Reader.Read` has two paths. When its buffer is empty and the caller asks for at
least the buffer's size, it reads straight into the caller's slice and returns the source's `(n,
err)` together; otherwise it fills its own buffer, hands the bytes over with a nil error and keeps
the error for the next call. `reader.fixed` asks for a whole value at once, so any value of at least
`readBufferSize` (64 KiB) takes the first path and sees the source's `io.EOF` beside the bytes. An
`http.Response.Body`, `iotest.DataErrReader` and any transport that announces its end with its last
bytes all produce that shape, which the `io.Reader` contract expressly permits. Today the `err ==
io.EOF` arm fires before the loop can notice the buffer is full, and a complete value is reported as
a log cut short — with zero records delivered and the whole file counted as dropped.

Every existing truncation test uses a source that returns `io.EOF` on a separate call, which is why
the corpus and the chunk matrix never caught it: `chunked` in `gatling/binary/chunk_test.go` is
exactly such a source, and `gatling/text/chunk_test.go` has `iotest.DataErrReader` in its matrix
while the binary one does not.

**Alternatives considered**:

| Rejected | Why |
|---|---|
| Move the `read > 0` arm above the `io.EOF` arm | Not enough on its own, as the issue says: the value's completeness is `n == len(buf)`, and only the loop condition knows it. |
| Clear every error once the buffer is full, as `io.ReadFull` does | The first version. bufio's direct-read path hands a source's error over once and forgets it, so a failure beside the last bytes of a large value was lost and the run read as complete. |
| Keep a non-`io.EOF` error that arrived with a filling read and surface it on the next call | A pending-error field on `reader` to report later what can be reported now: the value is whole, but the stream is not, and nothing is gained by delivering one more value first. Principle VI: no mechanism without a current need. |


**Tests** (all fail on `main`):

- `gatling/binary/read_test.go` (package `binary`): `readFull` over a one-shot source that returns
  every byte and `io.EOF` in one call — filled means `nil`; one byte short means `errCutShort`.
- `gatling/binary/truncation_test.go` or beside the chunk tests (package `binary_test`): a crafted
  log whose trailing assertion payload is 200,000 bytes — built with `builder.runRecord` — read
  through `iotest.DataErrReader` and through the one-shot source; records equal a `bytes.Reader`
  read of the same bytes; the same fixture with a payload below 64 KiB behaves the same (US1
  scenario 2); the same fixture cut for real still ends in a `*gatling.TruncationError` (FR-002); and read
  through the one-call source with a failure in place of `io.EOF`, it ends in that failure, for both
  payload sizes.
- `TestChunkedReadsMatchWholeFile` gains `iotest.DataErrReader` in its matrix, matching the text
  codec's, so the corpus is read through that shape from now on.

`simlog.readHead` takes the same rule in R4, so the two loops are behaviourally equal before #81
(milestone v0.2.0) folds them.

## R2 — #87: the read buffer is the codec's own

**Decision**: `newReader` (`gatling/binary/read.go:67`) wraps its argument in a plain
`struct{ io.Reader }{r}` before `bufio.NewReaderSize`, with the comment `gatling/text/scan.go:33`
already carries. Nothing else changes; `readBufferSize` stays 64 KiB and is not configurable.

**Rationale**: `bufio.NewReaderSize` returns its argument unchanged when that is already a
`*bufio.Reader` at least the size asked for. A caller who buffers its own file — ordinary Go — then
hands the codec a buffer it did not choose: with a 64 MiB caller buffer the "fixed read buffer"
`Reader` documents is 64 MiB, and the trigger for R1's defect moved with it. The text codec has the
guard and explains it; the binary codec was written without it. The wrapper hides
the dynamic type, so `bufio` allocates the codec's own buffer whatever it is given.

**What the test can observe**: with the reader adopted, the codec's small reads (`ReadByte`, a
four-byte `i32`) fill the caller's buffer, so after `NewReader` the caller's `Buffered()` is the rest
of the first 64 KiB — the 3,508 bytes the issue measured. With the wrapper, the codec's first read is
one 64 KiB fill, which the caller's `bufio` (any buffer of at most 64 KiB, including the 4 KiB
default) serves through its large-read path without touching its own buffer, so `Buffered()` is 0.
A caller whose buffer is *larger* than the codec's will see its own reader prefetch on its own behalf
— that is `bufio` doing what the caller asked, not adoption, and it is the caller's memory; the test
uses buffers of the codec's size and below, which is the issue's acceptance.

**Alternatives considered**: an `Option` to choose the buffer size (out of scope; #87's non-goal:
"if that is ever wanted it is an Option, not an accident of the argument's dynamic type"); wrapping
in `io.LimitReader` or `io.MultiReader` (heavier than an empty struct, and each adds a behaviour the
codec does not want).

**Tests**: one test for both codecs, `TestCodecsReadThroughTheirOwnBuffer`, in
`gatling/binary/agreement_test.go` — the harness that already imports both codecs and whose table is
the record of their agreement — handing each constructor `bufio.NewReaderSize(src, 64<<10)` and
`bufio.NewReader(src)` over a corpus log and asserting `Buffered() == 0` afterwards. Fails on `main`
for the binary rows only, which is the asymmetry the issue found.

**Landed elsewhere.** While this branch was open, PR #111 fixed #87 on `main` with the same wrapper
and `TestACallersBufioIsNotAdopted` in `gatling/simlog` — both codecs, a 64 KiB caller buffer over a
real log — and the issue closed on 2026-09-10. This branch's commit for #87, written and verified
as above, was dropped at rebase rather than carried as a duplicate; the analysis stands as the
record of why the fix is right.

## R3 — #83: a source failure never satisfies `errors.Is(err, io.EOF)`

**Decision**: every failure of the source in the three packages is reported through one helper,
`source.Failed` in a new `internal/source` package. It prefixes the cause with the caller's context
and wraps it with `%w`, unless the cause's chain holds `io.EOF`; then it returns an unexported error
whose `Is` answers for the cause's chain except `io.EOF`, whose `As` delegates to the cause, and which
has no `Unwrap`. `identify`'s failure path, `gatling/binary`'s `sourceFailed` and `gatling/text`'s
`readError` all call it, so the rule is stated once.

**Rationale**: `%w` keeps the chain, so a torn upload failing with `fmt.Errorf("upload aborted: %w",
io.EOF)` reaches the backend's `errors.Is(err, io.EOF)` as the clean end of the log — before any
record was decoded, so nothing in the result can contradict it. The package overview
(`gatling/simlog/doc.go:32`) promises the opposite. An error chain cannot have one link removed, so hiding `io.EOF` takes a
type of its own: one without `Unwrap`, answering `Is` and `As` on the cause's behalf.

*Corrected after review.* The first version cut the whole chain with `err.Error()`, as both codecs
had done since v0.0.8, and made `errors.Is(err, cause)` false with it: a consumer telling a
cancelled follow from a broken one by `errors.Is(err, context.Canceled)`, or reaching an
`*fs.PathError` with `errors.As`, lost both, in all three packages. The helper keeps them reachable
everywhere; `TestWrappedEOFIsAStreamFailure` asserts `errors.Is(err, broken)` again, and the
cross-package table asserts that the cause is reachable from every constructor.

**Alternatives considered**:

| Rejected | Why |
|---|---|
| Cut the chain with `err.Error()` when it holds `io.EOF`, the codecs' rule since v0.0.8 and this entry's first decision | Hides every other cause along with the `io.EOF`. The type that replaces it is unexported and lives in `internal/`, so no identifier is added to the surface being frozen. |
| Wrap with `%v` everywhere in `identify` | Cuts the chain for every failure, including ones a consumer may legitimately inspect (`fs.ErrPermission`, a `net.Error`). The codecs cut only the chain that ends in `io.EOF`; `identify` should match them, not exceed them. |

**Tests**: `TestASourceFailureIsNeverTheEndOfTheLog` in `gatling/simlog/errors_test.go`, a table over
six constructors — `simlog.NewReader`, `simlog.NewRunReader`, `binary.NewReader`,
`binary.NewRunReader`, `text.NewReader`, `text.NewRunReader` — and four sources: a plain failure, an
error wrapping `io.EOF`, `io.ErrUnexpectedEOF` by identity, and an error wrapping
`io.ErrUnexpectedEOF`. Each cell asserts an error that is not `io.EOF` by `errors.Is`, not a
`*gatling.TruncationError`, not a `*gatling.FormatError`, whose message contains the cause's text,
and whose chain still reaches the cause. This is FR-006's "one test over all three packages"; `simlog` is where it lives because it
already imports both codecs. The `simlog` rows fail on `main` for the wrapped-`io.EOF` source (this
issue) and for the two `io.ErrUnexpectedEOF` sources (R4); the codec rows already pass and are the
reference.

## R4 — #102: only the stream's own end is a short head

**Decision**: `readHead` (`gatling/simlog/simlog.go:256`) stops returning `io.ErrUnexpectedEOF`.
It returns an unexported sentinel, `errShortHead`, when the *source's* `io.EOF` arrived after some
bytes and before the window was full; `nil` when the window was filled
with the source's `io.EOF` beside the last bytes, and the source's error when any other came with
them (R1's rule, so the two loops agree); `io.EOF` when nothing was read; `io.ErrNoProgress` for a
stalled source; and the source's own error, untouched, otherwise. `identify` treats `err == io.EOF`
and `err == errShortHead` as "the head is what we have" and hands everything else — a source's
`io.ErrUnexpectedEOF` included — to R3's failure path. Both comparisons stay identity comparisons,
for the reason the existing `//nolint:errorlint` states.

**Rationale**: `io.ErrUnexpectedEOF` is what `compress/gzip`, `flate` and `zlib` return *by
identity* when their compressed input was cut — a failure of the source, not of the log. `readHead`
returns the same value for its own conversion of a short read, so `identify` cannot tell the two
apart and reports a torn archive as `FormatError{Short: true}`: "come back with more bytes", the
answer a follower retries on. The binary codec solved the identical problem with `errCutShort`
(`gatling/binary/read.go:78`): a private value that only its own loop can produce. `identify`
needs the same distinction and gets the same shape.

**Alternatives considered**: comparing `err == io.ErrUnexpectedEOF` only when `n > 0` — the source's
`io.ErrUnexpectedEOF` after two bytes has `n > 0` too, so the value alone cannot separate them;
tolerating `io.ErrUnexpectedEOF` but re-checking the source afterwards — a second read the follower
contract does not allow (`doc.go`: the end of input is the caller's statement).

**Tests**: in `gatling/simlog/errors_test.go` — a source returning two bytes then
`io.ErrUnexpectedEOF` is not a `*gatling.FormatError` and not `io.EOF` (both constructors); a real
gzip of a text log, cut inside the detection window and read through `gzip.NewReader`, gives the
same answer (the issue's reproduction: 14 of 92 bytes); a source returning two bytes then `io.EOF`
still yields `FormatError{Short: true}` — asserted on the field, which today's "too short" case in
`TestNoTypedNilOnAnyErrorPath` does not check. The stalled-reader test is unchanged. A tee whose spool fails beside the tenth byte is a failure, not a head.

## R5 — #76: the version is judged before the rest of the run record, in both codecs

**Decision**: both constructors take the same two-step shape: read what carries the version, judge
it, then read the rest.

- **Binary.** `readRun` (`gatling/binary/record.go:68`) splits into `readVersion`, which reads the
  first string and parses it — a string that is not a release is refused there, quoted, as today —
  and `readRunRest`, which decodes the simulation class, the run start with its bounds, the
  description, the scenario names and the assertion payloads. `NewReader`
  (`gatling/binary/reader.go:79`) calls `versionPolicy.Apply` between the two and records the
  warning before anything after the version is read. `readVersion` refuses a version string longer
  than 64 bytes before reading it, so the one field read ahead of the gate cannot cost more than a
  buffer fill.
- **Text.** `parseHeader` (`gatling/text/parse.go:149`) validates the run start *before* it parses
  the version, and `finishPreamble` (`gatling/text/reader.go:177`) applies the gate only after
  `parseHeader` returns whole. Moving the binary gate first would therefore open the reverse
  divergence for the one fault both formats can express — a version below the range beside a run
  start past its ceiling — so the text side takes the same shape: the version field is parsed and
  judged first, the gate applied, and the run start and the other fields parsed after. The field
  count check stays first on the text side; a line with fewer than six fields carries no version to
  judge, which is the analogue of a binary version string that cannot be read at all. An
  `ASSERTION` line with too few fields waits for the gate, as a surplus field already did, so a
  version the gate refuses outranks it; with no version to rule on, the earliest fault in the file is
  reported. An assertion table past its 8 MiB ceiling cannot wait, since waiting is what the ceiling
  prevents, so the text codec refuses it as damage where it meets it: the one exception to the
  same-type rule.

**The precedence, stated once for both codecs**:

| Order | Fault | Result |
|---|---|---|
| 1 | the version cannot be read (too few fields; a binary length past 64 bytes; a cut) | `*SyntaxError` or `*TruncationError`, as today |
| 2 | the version string is not a release | `*VersionError` (`Parsed: false`), quoting it |
| 3 | the version is below the range | `*VersionError` |
| 3 | the version is above the range under `WithStrict` | `*UnverifiedError` |
| 3 | the version is above the range | a `Warning`, and decoding continues |
| 4 | the run start is out of bounds, a count is corrupt, a table exceeds a ceiling | `*SyntaxError` naming the position |

Rows 2 and 3 now precede row 4 on both sides. Nothing the gate *decides* changes (spec Out of
Scope); only its place in the order does.

**Rationale**: Principle II states the gate as a MUST before any record is decoded, and the version
is the first field of the binary run record, so nothing forces the current order. The cost of the
current order is twofold: a refused log is read up to its scenario and assertion ceilings first
(which is what makes R6 compound with this), and the error a consumer gets for one shape of bad log
depends on which format wrote it — the exact thing `simlog` exists to hide. From v0.1.0 that error is
observable behaviour and moving it is a breaking change; today it is a Changed entry.

**Alternatives considered**: leaving the binary order and documenting the divergence (rejected by the
issue: Principle II is a MUST, and `simlog`'s promise is that a consumer cannot tell the codecs
apart); gating in the binary codec after the run start but before the tables (still after a field
that follows the version, and still divergent from a text codec that gates on the header line).

**Tests**:

- `gatling/binary/reader_test.go`: a log naming 1.0.0 whose scenario count is corrupt returns a
  `*gatling.VersionError` (the issue's acceptance; fails on `main` with a `*SyntaxError`).
- `gatling/binary/agreement_test.go`, `TestCodecsGateBeforeTheRestOfTheRunRecord`: a table both
  formats can express — 1.0.0 with a run start past `gatling.MaxRunStart`; a non-release version
  with the same start; 3.99.0 under `WithStrict` with the same start — asserting the *same* error
  type from both codecs. The existing `failuresDiffer` compares only `*SyntaxError`-ness and would
  call two different refusals agreement, so this table asserts the type directly. The same rows
  run again with an assertion that cannot be read in place of the run start.
- `gatling/binary/gate_test.go` (through `binary.ReadBufferSize`): two logs naming 1.0.0 — one with
  a three-name scenario table, one with a 200 KiB table, both padded past 64 KiB — pull the same
  number of bytes from their `*bytes.Reader` before the refusal, and at most `readBufferSize`: one
  `bufio` fill, whatever the tables claim (US3 scenario 6). A version length of 1 MiB is refused
  unread, within the same bound, and one of 64 bytes is still read and judged.
- `TestPeakMemory` and its siblings, the corpus suite and the canary are the proof that every
  accepted log is unchanged.

## R6 — #75: every retained table is bounded in bytes

**Decision**: one accounting rule for the three collections the reader keeps for the life of a read,
applied as the entries arrive: each entry costs its content plus a string header, and a collection
past its ceiling ends the read with the `*gatling.SyntaxError` `readBlobs` already raises, at the
offset of the entry that crossed the line, naming the total and the ceiling.

| Collection | Count ceiling (kept) | Byte ceiling, header-inclusive | Today |
|---|---|---|---|
| assertion payloads | `maxAssertions` 65,536 | `maxAssertionBytes` **8 MiB** (unchanged figure; now counts headers too) | 8 MiB of content, count bounds the headers |
| scenario names | `maxScenarios` 65,536 | `maxScenarioBytes` **1 MiB**, new | count only: up to 64 GiB |
| the string table | — (`maxCacheEntries` removed, see below) | `maxCacheBytes` **12 MiB**, new | count only: up to 1 TiB |

`stringHeader` is a constant 16 bytes — the size of a string header on a 64-bit target; a 32-bit
target retains less, so the bound loosens only in the safe direction — chosen over `unsafe.Sizeof`
so the package keeps `unsafe` out of a decoder of untrusted input (`gosec` is enabled, constitution
Quality Gates). `readStrings`, `readBlobs` and `cache.read` share one helper for the running total,
which is the "one shape" the issue asks for.

**Why headers are counted**: bounding content alone cannot keep the figure. The count ceilings
already let the headers reach 16 MiB (a million cache entries at sixteen bytes each) before a single
byte of content is counted, and Go grows an appended slice by about a quarter, so a content-only rule
would need the count ceilings lowered anyway. Counting what is actually retained — the header and
the bytes behind it — is one rule instead of two, and it is the truth about memory.

**Why `maxCacheEntries` goes**: at 12 MiB header-inclusive, the table cannot exceed 786,432 entries,
so a check at 1,048,576 can never fire. Principle VI forbids dead code; the constant's reasoning —
failure messages embed addresses and status lines, so a run that fails a new way on every request
would otherwise grow the table with the log — moves to `maxCacheBytes`'s doc comment, where it is
now a statement about bytes. `maxScenarios` and `maxAssertions` stay: 65,536 empty names come to
exactly 1 MiB of headers, so both checks remain reachable, and the count check is what stops a
corrupt count sizing anything before the elements behind it read.

**The arithmetic the figures rest on** (retained, not transient; the sampler collects before it
measures, R10):

| What the reader holds | Bound |
|---|---|
| assertion payloads, headers and content | 8 MiB, plus up to 0.25 MiB of slice slack |
| scenario names | 1 MiB, plus up to 0.25 MiB |
| the string table | 12 MiB, plus up to 3 MiB of slack (a quarter of the headers) |
| the read buffer, the scratch (≤ `MaxStringLen`), the group path | ≈ 1.1 MiB |
| **worst case** | **≈ 25.6 MiB**, against the documented 32 MiB; today unbounded |

A 16 MiB table would leave under two megabytes of margin once slack is counted and would keep the
count check reachable only by coincidence; 12 MiB leaves six, and is still over a hundred thousand
distinct strings of realistic length — a run's names and groups are a few hundred, and Gatling
truncates failure messages. The trade-off is stated plainly in the doc comment and the Changed
entry: a log with more than 12 MiB of distinct strings — a check whose expected value differs per
session can write one — is now refused as damaged where it was previously accepted at a cost the
documentation denied. A consumer that needs more is asking for an option, which #87's non-goal
already rules out as an accident and which this feature does not add.

**Doc statements that move together** (FR-008): `Reader`'s "# The budget" paragraph gains the three
tables and their ceilings; `maxScenarios`'s comment stops reasoning about headers alone;
`readBlobs`'s comment points at the shared helper; `MaxStringLen`'s doc is unchanged (it bounds one
field, and still does); `CHANGELOG.md` records the ceilings and the removed count.

**Alternatives considered**: restating the budget as "bounded by the distinct strings a log
introduces" (the issue's direction 2, rejected as the whole answer: a consumer sizing a sidecar
needs a number; it is said *beside* the ceiling instead); content-only accounting with lowered count
ceilings (two rules where one suffices, and the count ceilings are documented figures too);
`unsafe.Sizeof("")` for the header size (correct on every target, but brings `unsafe` into a package
that decodes untrusted input for a constant that only ever overestimates on 32-bit).

**Tests**:

- `gatling/binary/limits_test.go` (no tag): each ceiling at its boundary — exactly at it accepted,
  one byte over refused with a `*gatling.SyntaxError` at the crossing entry's offset naming the
  ceiling — for scenario names, assertion payloads and cache entries; and a log with 65,536 empty
  scenario names still accepted (the count ceiling stays reachable).
- `gatling/binary/memory_test.go` (`integration` tag; names end in `PeakMemory`, R10):
  `TestDistinctScenarioNamesPeakMemory` — 48 names of `MaxStringLen`, refused, live heap under the
  budget throughout; `TestDistinctStringsPeakMemory` — request records each introducing a fresh
  `MaxStringLen` string, refused, same bound; `TestTablesAtTheirCeilingsPeakMemory` — the three
  tables just under their ceilings followed by records that refer back to them, **accepted**, live
  heap under the budget: this is the proof of the figure. All three fixtures are generated readers in
  the shape of `newSynthLog`, never byte slices: a 48 MiB fixture held in the test's heap would be
  measured as if the reader retained it.

## R7 — #88: one ordering key, so the newest run is the newest run

**Decision**: `later` (`gatling/run/find.go:438`) is replaced by `compareRuns(a, b runDir) int`, a
lexicographic comparison over one key — the log's modification time, then the run's recorded stamp
or the empty string when the name carries none, then the name — and `newest` becomes
`slices.MaxFunc(runs, compareRuns).name`. An unstamped name therefore ranks below every stamped one
at an equal modification time; among unstamped names, and among equal stamps, the name decides, as
today.

**Rationale**: the current predicate switches comparison rule per pair — stamps when both carry one,
whole names otherwise — and that switch is what makes it cyclic on a mixed root: `A > B` by stamp,
`B > C` by name, `C > A` by name. A linear maximum over a cyclic relation returns whichever candidate
the scan happened to reach last, and `os.ReadDir`'s name order is what decides. A key is transitive
by construction. Ranking the unstamped name lowest is the one choice the key forces, and it is the
right one: the stamp is the only evidence in a name about *when* the run started (`find.go:432`), and
a name without one says nothing about time. The signals and their precedence — time, stamp, name —
are unchanged (#88's non-goal).

**What changes for a caller**: only a mixed root. `{simAA, simA-2099…}` at an equal modification
time returned `simAA` (by whole-name order) and now returns the stamped run; the three-directory
root in the issue returned the run stamped 2020 and now returns the one stamped 2099. Recorded under
Changed with the rule spelled out, which also gives #97 (the changelog's contradictory statement of
the rule, out of scope here) something exact to converge on.

**Alternatives considered**: sorting the candidates once (`slices.SortFunc`) and taking the last —
the same key, one allocation more, and `newest` is called on a slice that is already the caller's;
normalising every name to a stamp by parsing (out of scope: the stamp is fixed-width text and sorts
as it stands; a parse would invent a time for a name that has none).

**Tests**:

- `gatling/run/find_test.go`: `TestFindMixedStampedAndUnstampedNames` — the issue's root,
  `simA-20990101000000000`, `simB-20200101000000000`, `simAA`, equal log times via `touchLog`;
  expects the run stamped 2099. Fails on `main`, which returns the one stamped 2020.
- `gatling/run/order_test.go` (package `run`, the package's first internal test file): `newest`
  over every permutation of that triple and over a thousand shuffles of a random candidate set
  returns one answer; `compareRuns` is antisymmetric and transitive on sampled triples. Internal
  because the property is about the ordering, and `Find` cannot be made to list a directory in six
  orders.

## R8 — #106 landed, #104 in flight: verified here, built elsewhere

**Decision**: no code for either issue on this branch.

- **#106** — the `compat` job (`.github/workflows/verify.yml:344`), `scripts/check-compat.sh` and
  its shell test landed in PR #109 (commit `410dc54`) and the issue is closed. `scripts/check-compat_test.sh`
  already covers "an empty report exits 2 and says the gate did not run" and "labelled breaking with
  an empty heading under [Unreleased] fails". This feature re-runs the shell-gate suite (it is part
  of `quick`) and lists the acceptance scenarios in [quickstart.md](quickstart.md) so they are
  re-checked, not re-implemented.
- **#104** — PR #110 merged on 2026-09-10 as `d663ca3`: `toolchain go1.26.8` beside `go 1.25` (the
  branch had said go1.27.1; the pin followed the supported line current at merge), the constitution
  amendment to v2.3.0 and the note in spec 001. FR-017 is satisfied. This branch was rebased onto it
  and its gates re-run under go1.26.8; `go.mod` was never edited here.

**Rationale**: the spec records both so that planning verifies rather than rebuilds; "one issue, one
commit" is already satisfied for #106 and will be by PR #110's commit for #104.

## R9 — Landing order, commits and pull requests

**Decision**: the spec artifacts land first, on their own, as
`docs(speckit): add 010-pre-freeze-hardening spec/plan/tasks` — CI ignores `specs/`, so that pull
request merges on review. Then **three stacked pull requests, one contract each, one commit per
issue** (AGENTS.md: 1 issue = 1 commit; 1 concern per PR), the commits in the spec's story order:

| PR | Concern | Commits, in order |
|---|---|---|
| A | contract 1 — endings and source failures | `fix(gatling/binary): a filled read that arrives with io.EOF is a complete value (#82)` · `fix(gatling): a source failure never satisfies errors.Is(err, io.EOF) (#83)` · `fix(gatling/simlog): a source's io.ErrUnexpectedEOF is a failure, not a short head (#102)` |
| B | contract 2 — the binary reader's gate, buffer and budget | `fix(gatling): judge the version before the rest of the run record, in both codecs (#76)` · `fix(gatling/binary): bound the scenario names and the string table in bytes (#75)` — #87's commit landed on `main` first, as PR #111 (R2) |
| C | contract 3 — the run ordering | `fix(gatling/run): order candidates by one key, so the newest run does not depend on directory order (#88)` |

Each commit carries its regression test, its doc-comment changes and its `CHANGELOG.md` entry, and
is green on its own (`go build ./... && go test ./...`). Each pull request carries milestone
**v0.0.10 Pre-freeze hardening** and closes its issues on merge; `scripts/check-linkage.sh --pr N` is the
merge gate. #82 lands before #83 and #102 so the two fill loops are equal from the first commit
that touches either (R1, R4); #76 lands before #75 so the byte accounting is written into the
post-split `readRunRest` once rather than moved; C is independent of A and B.

**Rationale**: the rules allow one pull request per issue as well; three by contract is fewer
reviews for the same commits, each reviewed against the contract it changes, and a reviewer of A
sees both codecs and `simlog` keep one promise together. Grouping by package instead would split
contract 1 across two pull requests and put #87 beside #82 rather than beside the budget it
protects.

**Rejected**: one pull request with seven commits — seven concerns; a single "hardening" commit —
forbidden outright; four pull requests by package, the first draft of this entry — see the
rationale.

## R10 — Performance and how memory is measured

**Decision**: `BenchmarkDecode` (`gatling/binary/bench_test.go:66`, the largest corpus file) is run
before and after each of the four binary commits; the expectation is noise, and a regression is
justified in the pull request or fixed. The changes on the hot path are an interface indirection per
64 KiB fill (R2), a reordered `switch` in `readFull` (R1), and one addition per *introduced* string
(R6) — nothing per record.

Memory is measured as the **live heap after a collection**: `liveHeap` in
`gatling/binary/memory_test.go` runs `runtime.GC()` and reads `HeapAlloc`, sampled `samples` times
through the input and once at the end, against `budget` (32 MiB) with `drift` (1 MiB) between a run
and one ten times longer. That instrument is already on `main` — the spec's dependency on the branch
that introduced it was stale (R12). CI runs the class with `-run 'PeakMemory$'` alone and without
`-race` or coverage (`verify.yml:140`), and excludes it from the instrumented runs with
`-skip 'PeakMemory$'`; every new memory test therefore ends in `PeakMemory` and carries the
`integration` tag, or it is measured instrumented and fails for the wrong reason.

**Rationale**: retention is what the budget promises and what a sidecar is sized to; `HeapAlloc`
without a collection reads the collector's backlog as retention and moves 2× idle and 8× under
contention, as the sampler's own comment records.

## R11 — Skills read, and where they disagreed with the constitution

**Read**: `golang-error-handling` (required: three error paths change), `golang-testing` (required:
every change; its testify sections excluded). Consulted: `golang-safety`, `golang-security` (R6),
`golang-benchmark` (R10), `golang-documentation` (existing doc comments change). Not triggered:
`golang-naming`, `golang-structs-interfaces` — no exported identifier is added or changed; the new
unexported names (`errShortHead`, `stringHeader`, `maxScenarioBytes`, `maxCacheBytes`,
`compareRuns`, `readVersion`, `readRunRest`) follow the conventions beside them.

**Disagreements, the constitution wins** (Quality Gates → Engineering Guidance):

| The skill says | Here | Why the constitution wins |
|---|---|---|
| Errors MUST be wrapped with `%w`; use `%w` internally, `%v` only at system boundaries | R3 hides `io.EOF` behind a type of its own when the cause's chain holds it, and wraps with `%w` otherwise | Principle II and `simlog`'s documented promise: a source failure is never the end of the log. `errors.Is(err, io.EOF)` matching is the failure mode, and a `%w` chain cannot lose one link; the type keeps every other cause reachable through `Is` and `As`. |
| MUST use `errors.Is` for sentinel matching, never direct comparison | `err == io.EOF` and `err == errShortHead` by identity (R1, R4), with `//nolint:errorlint` and the reason | An error that merely *wraps* `io.EOF` is a source failure; identity is the point, and `io.ReadFull` itself converts with `==`. |
| Use `samber/oops` for production errors; log with `slog` | Neither | Principle IV: no module is pre-approved and `gatling/` is stdlib-only. A library that returns errors logs nothing. |
| Returned errors MUST always be checked, never discarded with `_` | `_, warning, err := versionPolicy.Apply(...)` in `NewReader` discards the verdict | Not an error: the verdict is a value the binary codec has no lenient mode for. Kept. |

`golang-testing`'s testify and mock sections are not followed (Principle III fixes the standard
`testing` package; Principle IV forbids the dependency); its table-driven, `t.Parallel` and fixture
guidance is what the tests already do.

## R12 — What reading the code corrected in the spec

1. **The memory-test dependency was stale.** The spec listed the worktree branch
   `test/peak-memory-collector-allowance` as unmerged work that changes how peak memory is sampled.
   `git diff origin/main..<branch>` shows the branch is *behind* `main` — it lacks `gatling/run` and
   the `lastrun` recording — and `main` already carries `liveHeap`, `budget` and `drift`. Nothing
   waits on it. The spec's Dependencies bullet is corrected in the same commit as this plan.
2. **Parity for #76 reaches the text codec.** FR-011 says both codecs return the same error type for
   the same fault. The text header parser judges the run start before the version, so moving the
   binary gate first alone would create the reverse divergence for a fault both formats express. R5
   gives both codecs the same two-step shape; the spec's Dependencies note the reach. The text
   change is observable for exactly one shape (a below-range version beside a run start past the
   ceiling) and is covered by the same Changed entry.
3. **#83's chain cut reached the cause, and review reversed it.** FR-004 says the cause's *text* is
   kept, which held; the first version also made `errors.Is(err, cause)` stop matching for a cause
   that wraps `io.EOF`, because it cut the whole chain. Review showed a consumer loses
   `context.Canceled` and `*fs.PathError` with it, and R3's shared helper now keeps them reachable
   in all three packages. No requirement changes.
4. **Two of the nine landed on `main` while the branch was open.** PR #110 merged the toolchain pin
   (#104) as `toolchain go1.26.8`, not the go1.27.1 the spec quoted from the branch, and PR #111
   fixed #87 with the same wrapper this branch had written and a test of its own. The branch was
   rebased onto both, its #87 commit dropped as a duplicate, and the gates re-run under the pinned
   toolchain. Neither changes a requirement; FR-010 and FR-017 are satisfied by those merges.
