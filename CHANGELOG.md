# Changelog

All notable changes to this project are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- `gatling/binary` reported a complete log as cut short — every byte counted as dropped, no record
  delivered — when the source returned the file's last bytes together with `io.EOF` in one call and
  the value being read was at least the 64 KiB read buffer, which only a trailing assertion payload
  is. An `http.Response.Body` ends a sized body exactly that way and the `io.Reader` contract
  permits it; `io.ReadFull`, whose contract the fill loop claims, clears the error in that case, and
  the loop judged the error before the count. A read that fills its value and ends the stream in the
  same call now succeeds, a failure that arrives with the last bytes is still reported as the
  source's failure, and a stream that leaves the value short is still a
  `*gatling.TruncationError`. (#82)
- `gatling/simlog` returned an error satisfying `errors.Is(err, io.EOF)` when the source failed with
  an error wrapping `io.EOF` — a torn upload, a closed transport — so an ingest handler that breaks
  its loop on the end of the log booked the failure as an empty run, before a single record had been
  decoded to contradict it. The package documentation promised the opposite in the list of what a
  follower may rely on. Only `io.EOF` is hidden now: the message keeps the cause's text, and every
  other cause in the chain stays reachable through `errors.Is` and `errors.As` — in `simlog`, and in
  both codecs, which until now cut the whole chain and so hid a `context.Canceled` or an
  `*fs.PathError` along with the `io.EOF`. (#83)
- `gatling/simlog` reported a source's `io.ErrUnexpectedEOF` — what a truncated gzip, flate or zlib
  stream returns by identity — as a stream too short to identify, the `*gatling.FormatError` with
  `Short` set that tells a follower to come back with more bytes; a torn archive or a broken
  decompressor inside the first ten bytes was retried rather than reported. Only the end of the
  stream itself now produces that answer: the head loop keeps a sentinel of its own for a head that
  ran short, as the binary codec's record loop already did. (#102)

## [0.0.9] - 2026-09-09

Finding the run: which `simulation.log` to open, answered once instead of three times.

Callers had to know the exact path of a run directory, and the three consumers of this module —
`galaxio report`, the comet sidecar and platform ingestion — were each about to work it out
differently. `gatling/run` answers it: give it the log, the directory holding one, or a results root
to search, and it returns where the run's artefacts sit and which rule chose them. It stops exactly
where reading begins, so a run whose log is truncated, damaged or outside the supported range still
resolves and fails only when a codec is handed the file.

What shaped it was reading `gatling-maven-plugin` and `gatling-core` rather than their documentation,
and then recording a real Maven results root, which corrected the reading twice. `lastRun.txt` is not
Gatling's: only the Maven plugin writes one, only when `failOnError` is turned off against its
default, and `gatling:verify` deletes it again — so for almost every caller the rule the issue framed
as a fallback is the only rule there is. And a run directory is named in UTC, not local time, which is
what makes the ordering's tie-break an ordering by time at all.

### Added

- **`run.Find` locates a run**, so the three consumers of this module stop each working out
  where a build tool put its results. It takes a path that may be the run itself — a
  `simulation.log`, or a directory holding one — or a results root to search, and returns the run
  directory with the log inside it and the rule that chose them. It opens no log and applies no
  version gate: a run whose log is truncated, damaged or outside the supported range still resolves,
  and fails when the log is read.

  **A path is required.** `run.DefaultResultsRoot` publishes `target/gatling`, where Maven and
  sbt both write, for a caller to pass; Gradle's `build/reports/gatling` and a hand-configured output
  directory are passed the same way, because reading a `pom.xml` or a `build.gradle` would make this
  library aware of three build tools for a value the caller already has. An empty path returns
  `run.ErrNoPath` before anything is read, rather than falling back to the working directory:
  `""` is the zero value of every unset flag, absent configuration field and omitted request member,
  and a server that quietly lost its path is better told so than handed a confident report about an
  unrelated run.

  `Dir` and `Log` are cleaned, so every spelling of one run — a trailing separator, a `./` segment,
  the log's own path — yields one `Location`, which is comparable and safe to use as a map key.

- **`Location`, `FoundBy` and `NotFoundError`** beside it. `FoundBy` says which of the three
  rules selected the run — the caller's own path, `lastRun.txt`, or the most recently modified run in
  the results root — so a consumer that reports which run it read can say how it was chosen.

  `FoundByNewest` is the ordinary answer rather than the exceptional one, and that is worth knowing.
  A `lastRun.txt` exists only when three things hold at once: the build is Maven, its `failOnError`
  parameter was turned off against its default, and `gatling:verify` has not run since — that goal
  reads the file and deletes it. Neither the Gradle plugin nor Gatling itself writes one at all. So for most callers the
  rule the issue framed as a fallback is the only rule there is, which is why the ordering is total —
  modification time first, then the directory name, both descending. The tie-break is not a
  formality: a `git clone`, an `rsync` without `-t`, a CI cache restore and a container image build
  each give every run in a root one modification time, and a Gatling run directory is
  `<simulationId>-<yyyyMMddHHmmssSSS>`, so descending name is descending run start.

  `NotFoundError` names the directory that was searched, which is always one the caller gave. A
  path that exists but is not a directory — an archive, a mistyped filename — reaches it too, rather
  than surfacing a raw `ENOTDIR`. A directory that could not be *read* is reported as that failure
  instead, wrapping its `*fs.PathError`, and that holds at every depth: the results root, a candidate
  run directory inside it, and the pointer file. A broken mount is never an absence of runs.

  Which run is newest is decided by the **log's** modification time, not the run directory's, so
  regenerating a report into an old run does not make it look like the latest test. Ties break on the
  run id's own UTC stamp before the directory name, which matters as soon as a root holds two
  simulations: whole-name order is alphabetical by simulation id first and would return a run that
  started months earlier.

  Nothing in `lastRun.txt` is trusted: a line is followed only when it is a bare name that matches a
  run actually present in the root, so a stale pointer, a deleted run, a directory without a log and
  the error line the Maven plugin appends after a failed run all cost one `stat` and fall through to
  the clock. No error text is matched, so the plugin is free to reword its messages.

## [0.0.8] - 2026-09-07

An incomplete record: what a run killed mid-flight leaves behind, and what a follower may rely on.

Gatling writes `simulation.log` through a buffer flushed in blocks, so a test stopped by a signal,
an OOM kill, a CI timeout or a full disk always ends inside a record. Both codecs answered that with
the same fatal error they give bytes that are not a Gatling log at all, and the documentation told
the caller to discard what had already been delivered — so the whole run was lost exactly when its
data mattered most, and the operator was left reading console scrollback.

Separating that ending from a damaged file made a second thing visible: a rule this module stated in
four places and held in two. A source that broke is not a log that ended, and four ways of confusing
the two are closed here, one of which left the binary decoder spinning with no error and nothing to
cancel. The other half of the milestone needed no code at all — a log still being written already
decoded to exactly what the finished file decodes to — so it ships as the promise a follower reads
and the test that keeps it.

### Changed

- A log **cut short** now ends a read with a new `*gatling.TruncationError` instead of the
  `*gatling.SyntaxError` it raised before, and the records delivered before the cut are the ones the
  run recorded rather than something a caller was told to discard. Gatling writes `simulation.log`
  through a buffer flushed in blocks, so a run stopped by a signal, an OOM kill, a CI timeout or a
  full disk always ends inside a record: that shape is the ordinary ending of an aborted test, and
  answering it with the same fatal error as bytes that are not a Gatling log threw the whole run
  away exactly when its data mattered most.

  The error carries where the incomplete record began — a byte offset for the binary format, a line
  number for the text one — and how many trailing bytes could not be decoded; for a binary log the
  stream stopped at `Offset+Dropped`. It reaches the canonical model path unchanged, so a consumer
  of `RunReader` learns it without dropping to the wire records. `Expected` says what the record
  still needed, which is not what sits at the position the error names — the position is pinned to
  the record's start while the value being read moves through it — and the message states the two
  apart for that reason.

  **A consumer that has not been updated needs no change.** It breaks its loop on `io.EOF` and
  treats everything else as a failed read, which is exactly what it does today — the new type is
  deliberately neither `io.EOF` nor a wrapper around one, so a killed run cannot be mistaken for a
  complete one. A consumer that wants the partial run reaches for the type with `errors.As`. What
  may be derived from a run whose log was cut short stays the consumer's decision; this module
  states the fact and computes nothing from it. (#7)

- A damaged log is still refused, and is now told apart from a cut one: an undefined record kind, a
  length prefix past the allocation cap and an unparseable field each still end the read with a
  `*gatling.SyntaxError` naming the position. Two cases cannot be distinguished and are documented
  rather than guessed at — a binary log cut exactly on a record boundary is a shorter valid log and
  ends cleanly, and a corrupted length prefix claiming more bytes than the file holds is
  indistinguishable from a file cut mid-value. (#7)

### Added

- The **blocking-source contract** is now stated in the `gatling/simlog` package documentation and
  pointed at from every constructor: a source that is still being written is supported, a `Read`
  that blocks is a wait rather than an end, a record split across reads is delivered exactly once,
  memory held while waiting stays bounded, and the end of input is the caller's statement — nothing
  here polls, reopens or stats a file to guess whether the writer is still alive. What a follower
  must do in return is stated beside it.

  All five recordings decode identically through such a source today, on both codecs, and did so
  before this release: no read loop changed. What was missing was the promise and the test. A
  follower — the comet sidecar is the one this was written for — can now build on a guarantee
  rather than on behaviour that happened to hold, and a later change to a buffer fails the test
  instead of removing it in silence. (#10)

### Fixed

- A source that stopped making progress — a `Read` returning `(0, nil)` forever, which the
  `io.Reader` contract permits and says must not be taken for an end — left the binary decoder
  spinning inside a record with no error and nothing to cancel. `io.ReadFull` loops on an empty read
  and `bufio`'s own guard lives in `fill()`, which its `Read` does not use, so neither caught it.
  The codec now reads through its own loop and ends such a read with `io.ErrNoProgress`, as
  `gatling/simlog`'s documented contract says it does. (#7)

- `io.ErrUnexpectedEOF` arriving *from the source* was reported as a Gatling log cut short.
  `compress/gzip`, `flate` and `zlib` all return that sentinel by identity when their compressed
  input was cut, so a broken decompressor was described as a killed run — with `Offset` and
  `Dropped` in decompressed coordinates that match no byte of the file on disk. Only the codec's own
  read loop can now establish that the log ran out. (#7)

- A source failure whose cause ends in `io.EOF` still satisfied `errors.Is(err, io.EOF)`, because
  the failure wrapped its cause with `%w`. Every loop that breaks on the clean end of a log — this
  module's own tests among them — read a broken transport as a complete run. The cause's text is
  kept and its chain is not, in both codecs. (#7)

- The text codec never got the identity rule the binary one did: `scanner.next`, `preambleError`
  and `Reader.Next` all matched the end of input with `errors.Is`, so a source failing with an error
  that wraps `io.EOF` was reported as a clean end, as a log cut short, or as a missing run header —
  three different ways of blaming the file for a transport fault. All three now compare by
  identity. (#7)

- `text.Reader` did not latch `io.EOF`, so a source that returned it and then more bytes — a file
  still being appended to — delivered records after the end the reader had already declared, while
  `binary.Reader` on the same source returned `io.EOF` forever. Both codecs sit behind
  `simlog.RecordReader` and now agree with the contract each of them states: every ending is
  terminal. (#7)

- A failure of the *source* that merely wrapped `io.EOF` — a truncated decompressor, a closed
  transport — was reported by the binary codec as a truncated log, sending a caller to re-record a
  run that was never damaged. It decided with `errors.Is` where its three siblings compare with
  identity and each carry the reason for doing so. It now compares with identity too, and a source
  failure is reported as itself with its cause wrapped. (#7)

## [0.0.7] - 2026-09-07

The corpus and the canary: what makes both Gatling codecs safe to freeze.

Support for five versions was asserted more than demonstrated. A binary recording was held to three
numbers — a run's total, ok and ko — so a decoder that renamed every request or moved one between
groups passed; the canary ran two text versions, so nothing exercised the binary codec against a
Gatling that had just finished; and no workflow ever passed `-fuzz`, so the check that enforces the
no-panic invariant only ever replayed its seed corpus.

### Changed

- `gatling/binary.MaxStringLen` is **1 MiB**, down from 8 MiB. A string or assertion payload
  between the two sizes is now refused with a `*gatling.SyntaxError` where it previously decoded.
  No Gatling log is affected: the longest field the format carries is an assertion payload, and the
  ten in the 3.14.9 recording are between 15 and 51 bytes, while a failure message is truncated by
  Gatling long before either ceiling. The constant's purpose — refusing a corrupt length prefix
  that claims gigabytes — is unchanged, and 1 MiB serves it as flatly as 8 MiB did.

  The reason for the change is the peak-memory bound. Decoding one field costs the read buffer and
  then the result, and a Latin-1 field above ASCII doubles on the way out, so a log carrying one
  field at the ceiling in each of the three encodings peaked at **44 MiB** — against a 32 MiB
  budget, on a log of 24 MiB. At 1 MiB the same shape peaks under 6 MiB. (#61)

### Added

- Every per-request and per-group figure a recording's own Gatling report states is now compared
  against the same figure folded from the decoded records, for every recorded version. A binary
  recording was previously held to three numbers — the run's total, ok and ko — so a decoder that
  renamed every request, moved one between groups, or flipped an outcome and compensated with
  another, passed. Reports are read through a new internal package that gives `stats.json`, the
  generated `index.html` and the captured console summary one shape, because Gatling stopped
  writing machine-readable statistics at 3.14.0. (#14)
- Every `Fuzz*` target in the module now runs under `-fuzz` on every pull request, one CI job per
  target, discovered from `go test -list '^Fuzz'` rather than from a list that would silently stop
  covering a target added later. A crasher fails the job and is uploaded; nothing generated is
  committed. A longer scheduled run sits beside it. Three targets state the constitution's no-panic
  invariant, and no workflow had ever passed `-fuzz`, so CI only ever replayed their seed corpus —
  the one check that reliably finds this class of defect was the one nothing ran. The leg is gated
  behind a `verify.yml` input so it covers pull requests and not the release path: fuzzing is
  nondeterministic, and a tag must not be blocked by a finding that depends on the seed. (#60)
- The 32 MiB peak-heap budget is now **stated** in `gatling/binary.Reader`'s documentation, where
  it previously existed only inside the tests, and `TestStringCeilingPeakMemory` and
  `TestAssertionCeilingPeakMemory` hold the codec to it for the two shapes that cost most per
  record — a field at `MaxStringLen` in each encoding, and a run record whose assertion payloads
  fill their own ceiling. The existing bound was measured on a synthetic log whose longest field
  was seven ASCII bytes, so it described that run's shape rather than the format's. (#61)

## [0.0.6] - 2026-09-06

The two definitions a consumer folds a run by: where something was recorded, and where the run
begins and ends.

### Added

- `model.Position`, with `NewSamplePosition`, `NewGroupPosition`, `Sample.Position` and
  `GroupSample.Position`: where in a run something was recorded, as one comparable value a consumer
  buckets by. Two consumers keying a map by it produce the same keys without agreeing on a spelling;
  distinct paths never collide whatever the names contain; a group traversal and a sample never
  share one; and it stays valid after the reader advances, unlike the `Groups` slice it is made
  from. (#8)
- `model.Bounds`: where a run begins and ends, exactly as Gatling's own report bounds it — by
  sample, group and virtual-user events, never by the run's recorded start or its errors — folded
  one item at a time with `Extend`. It is the span every rate divides by, and the one definition a
  consumer was most likely to get subtly wrong: on the 3.15.1 recording it reproduces the console
  summary's 25.5, 21 and 4.5 requests per second to the digit. (#8)
- The package example in `model`: the consumer's loop over a real recording, checked against that
  run's own console summary. This library still computes nothing; the example shows what a consumer
  computes from, and `testdata/exports.golden` pins the exported surface so that a statistic cannot
  be added unnoticed. (#8)
- `gatling.MaxRunStart`: the latest run start either codec accepts. Every event time is resolved
  against the run start and the binary format adds a 32-bit offset to it, so a later start could not
  carry one without running past the end of the int64 range. Both codecs bound the field here, so a
  value both formats can express can no longer be read by one and refused by the other. (#56)
- `gatling.AbsentTimestamp`: the one value both codecs write on a wire record for a time the log
  could not resolve — a negative offset in a binary log, a negative value in a text one, and the
  sentinel Gatling itself writes for a request that never completed. `gatling/binary` used it
  privately; it is now named, shared and documented on `Record`. (#56)

### Changed

- An instant a codec could not resolve reaches the canonical model as the zero `time.Time` rather
  than as a date 292 million years in the past. `Sample.Start`, `GroupSample.Start`, `UserEvent.At`
  and `RunError.At` say so, and a recorded instant is never the zero time: nothing recorded is before
  1970, and the zero time is the year 1. (#56)
- `gatling/text` no longer refuses a log for a negative time or a negative cumulated response time.
  It reports the time absent and the duration unset, which is what `gatling/binary` already did and
  what spec 005 requires of both: one bad field does not end a ten-million-record read, and the two
  codecs now give the same answer to the same malformed input. A test drives both with the same
  inputs so a third answer cannot appear unnoticed. (#56)
- `model.Bounds` reports no span rather than a wrong one. A fold that met an item it could not place
  in time reports neither bound, because such an item's recorded end is unreachable from a model that
  carries no end without a start, and a span that silently excluded it would be too short and every
  rate derived from it too high. The end is never reported before the start either: a virtual-user
  END extends only the end and a sample with no recorded end extends only the start, so the two can
  cross. `Start` and `End` now take a value receiver, so bounds kept in a map can be read. (#8)
- `model.Position` renders a group traversal with its kind, so a group at `a / b` and a request named
  `b` inside group `a` no longer print the same label for two rows the type keeps distinct. (#8)
### Fixed

- `gatling/binary` refused a valid run declaring more than 1,024 assertions or scenarios as
  malformed, because the group-nesting ceiling was bounding two counts it was never meant to. Each
  count now has its own ceiling, named for what it bounds; a run with two thousand assertions
  decodes, and a corrupt count is still stopped before it sizes anything. (#57)
- `gatling/text` read the negative half of a timestamp field without bounding it, so a value far too
  wide for an int64 passed as an absent time while the identical positive magnitude was refused, and
  `-0` read as an absence rather than as the epoch instant it spells. A line whose fields had shifted
  could therefore clear the only structural check a timestamp gets. Both signs are now bounded alike,
  and the most negative int64 — the one negative number Gatling writes — is carried in the cumulated
  response time instead of being refused there while being accepted one field earlier. (#56)
- Above the supported range, the reader scans back for the field that reads as an error record's
  timestamp. It probed with a predicate that refused negatives while the parser accepted them, so it
  walked past a real timestamp and refused the read on the field behind it. (#56)
- `gatling/binary` bounded the assertion payloads by count alone, but a payload is retained for the
  life of the read, so 65,536 of them read under `MaxStringLen` could hold hundreds of megabytes
  against the budget the package documents. What they come to in total is now checked as they
  arrive, and an untrusted count no longer sizes its slice before the elements behind it read. (#57)
- The README still described a binary `simulation.log` as refused for want of a codec, and two
  comments justified the binary detection rule by a sample directory v0.0.5 deleted. The README now
  matches the package documentation, and the rule's provenance is the 3.15.1 recording. (#55)

## [0.0.5] - 2026-09-06

Reading the binary `simulation.log` — the format every current Gatling writes.

### Added

- `gatling/binary`: decodes the binary `simulation.log` Gatling writes from 3.13.0 — the format
  every current run produces — and yields the same wire records and the same canonical results the
  text codec does, so a report written against `model` cannot tell which log it was reading. It
  accepts **3.13.1 through 3.15.1**, the range its golden corpus covers.
  - `3.13.0` itself is refused, although this codec could read its logs. That version cannot read
    back the assertion records it writes, so no run of it generates a report, so no run of it can
    carry the second account of its own numbers a corpus entry needs. The range follows the corpus,
    which is what the constitution asks and what this case exists to demonstrate: the format's own
    source diff says nothing changed between 3.13.0 and 3.15.1, and it is right, and 3.13.0 is still
    not recordable.
  - The stream must begin at the first byte of the file. The format replaces a repeated string with
    a back-reference into a table the reader rebuilds as it goes, and that table cannot be
    reconstructed from the middle.
- `gatling.SyntaxError.Offset` and `gatling.SyntaxError.Format`: the byte offset for a binary log,
  beside `Line` for a text one, and the format that says which of the two to read. Both fields are
  additive, and `Format`'s zero value renders as a line, so an error built before they existed reads
  exactly as it did. `Format` is not redundant: a binary log can fail at byte 0 and a text log fails
  before it has a line, so both positions are legitimately zero and neither can discriminate alone.

### Changed

- `gatling/simlog` reads a binary `simulation.log` instead of refusing it with a
  `*gatling.UnsupportedFormatError`, and `Supported` reports the binary format as readable over
  3.13.1 through 3.15.1. Both change together because both read the same table.
- `gatling/text` now sets `SyntaxError.Format` on every error it raises. The field is what says
  which of `Line` and `Offset` to read, and until now only the binary codec filled it in, so a
  consumer switching on it fell through to the unknown case for every text error.
- `gatling.Record.Line` is documented as the text format's. A binary log has no lines and leaves it
  0; the position of a failure in one is a byte offset, which `SyntaxError.Offset` carries.
  `gatling.UnsupportedFormatError` can no longer be returned, because this module now has a codec
  for both formats Gatling writes; it is kept for a third format rather than removed.

### Fixed

These are defects in `gatling/binary` found by review before it shipped, listed because several are
promises the package documents and a reader of this file should be able to rely on them.

- A cache index of `math.MinInt32` panicked the decoder: negated inside an `int32` it stays negative
  and slipped past a bounds check into an index. Four bytes of a corrupt log took the caller's
  process down, through `gatling/simlog` as well.
- A group path was handed out with its spare capacity, so a caller appending to `Record.Groups` wrote
  into the reader's own array and saw the next record's groups appear in its slice. It is also
  always empty and non-nil now, rather than nil for the first record without groups and empty for
  the rest.
- An error merely *wrapping* `io.EOF` was read as the clean end of the log, so a truncated
  decompressor or a closed transport reported a partial run as complete.
- Every source failure was reported as `found end of input`, telling a caller their log was truncated
  when the transport or the disk had failed, and discarding the cause. The cause is wrapped now, and
  a truncation names the first byte that was not there.
- The run start was unvalidated, so a corrupt one made every timestamp in the log wrap to a plausible
  instant in the distant past. It is bounded once, when it is read.
- An unpaired UTF-16 surrogate became U+FFFD. A name that decodes to a different name regroups a
  report without saying so; it is refused with an offset.
- An offset that cannot be resolved is reported absent rather than ending the read.
- `binary.RunReader.Run` handed out the reader's own slices where `gatling/text` clones them, so two
  consumers of one reader could corrupt each other.
- The string table had no ceiling, although failure messages go through it and Gatling builds those
  from text that embeds addresses and ports — so a run that failed differently every time grew a
  table that followed the record count.

## [0.0.4] - 2026-09-06

Telling which Gatling wrote a log, before anything tries to read it.

### Added

- `gatling.Detect`, with `gatling.Format` and `gatling.DetectSize`: names a `simulation.log` as the
  tab-separated text format or the binary one written from 3.13.0, from a fixed ten leading bytes
  and never from the file's name. A text log is recognised by `RUN` or `ASSERTION` followed by a
  tab — a simulation that declares assertions writes those ahead of the run header, so both are
  ordinary openings, and both recorded runs in the corpus begin with the second.
- `gatling/simlog`: opens a `simulation.log` without being told which Gatling wrote it. It
  identifies the format, hands the stream to the codec that reads it, and yields records identical
  to that codec's own. A binary log is refused with a `*gatling.UnsupportedFormatError` naming the
  format, rather than failing as a syntax error on line 1.
- `gatling/simlog.Supported`: what this module reads, per format, so a consumer reports it instead
  of hard-coding a range that goes stale. The binary format is reported as known and not yet
  readable, which is a different answer from an unknown one.
- `gatling.Policy` and `Policy.Apply`: the version policy in one place, applied by every codec
  before any record is decoded. It returns the `Verdict` alongside the warning, so a codec reads
  leniency from the version decision rather than inferring it. `gatling.Gate` is unchanged and is
  what `Apply` is built on.
- `gatling.WithStrict`, with `gatling.Option`: refuses a version no recording covers instead of
  decoding it with a warning. The default is unchanged and lenient; strictness only ever tightens
  the gate, and cannot reach a version inside or below the supported range.
- `gatling.FormatError`, `gatling.UnsupportedFormatError` and `gatling.UnverifiedError`. With the
  existing `VersionError` and `SyntaxError`, every way a read can be refused now has its own type,
  so a caller branches with `errors.As` rather than on message text.

### Changed

- `gatling/text.NewReader` and `gatling/text.NewRunReader` take `opts ...gatling.Option`. Every
  existing *call* compiles and behaves exactly as before — passing no options is the lenient
  default — and the version decision moved out of the codec into `gatling.Policy` without changing
  any outcome. What does break is a caller that stored either constructor in a variable of type
  `func(io.Reader) (*text.Reader, error)`: a variadic parameter changes the function's type, so a
  registry built that way stops compiling. Permitted before v0.1.0 (Principle V) and recorded here
  rather than described as fully compatible.
- `gatling.Warning.String` returns the empty string for the zero `Warning`. The zero value is how
  "no warning" travels, so rendering it as a warning about version 0.0.0 put a false alarm in the
  log of every healthy run.

### Fixed

- `scripts/check-linkage.sh` requires a closing issue only from the change types a milestone is a
  manifest of — `feat`, `fix` and `perf`. It demanded one from every pull request, which is stricter
  than the rule it enforces: the constitution asks that every issue a pull request *fixes* be
  closed, not that one exist. Spec artifacts, constitution amendments and out-of-scope refactors
  land without an issue by design, and the gate refused a milestone that was otherwise ready. Every
  pull request still needs a milestone, and a waived one is now reported as checked rather than
  omitted from the audit.

## [0.0.3] - 2026-09-05

The canonical model, and the corpus probe's expectations stated once in OpenNFR.

### Added

- `model` package: the canonical result types every source is decoded into — `Sample`,
  `GroupSample`, `UserEvent`, `RunError`, `Run`, `Item` and `Capabilities`, with `Opt[T]` for a value
  a source may not have recorded. Standard library only, and what the three downstream builds depend
  on instead of any tool package.
- `gatling/text.NewRunReader`: reads a Gatling text `simulation.log` as canonical results. Same log,
  same version gate and same bounded memory as `NewReader`; a run carries only what does not grow
  with its length, and samples, group traversals, user events and run-level errors stream beside it.
- `gatling/text.Capabilities`: what a Gatling text log records and what it never does, readable
  before a log is opened.
- `model.ItemAssertion`: an opaque payload a source wrote among the events rather than ahead of
  them. A source that writes them all in its preamble puts them on `Run.Assertions` instead; this
  kind exists so neither placement loses them.
- `model.FieldsKnown`: every field the package names, so a caller walks the set instead of
  hardcoding its last constant.

The Gatling wire records (`gatling.Record`, `gatling.Header` and the rest) are unchanged and stay
exported. They are the log's own events rather than a result; the canonical types are what a
consumer builds on.

## [0.0.2] - 2026-09-04

First published version. v0.0.1 was completed but never tagged, so everything below ships here.

### Added

- `gatling`: the wire records a `simulation.log` carries (`Record`, `Kind`, `Header`, `Status`,
  `Event`), the `Version` type with `ParseVersion` and the version gate (`Gate`, `Verdict`), and the
  errors a read can end with (`SyntaxError` with its line number, `VersionError`, `Warning`).
  Every enum has an explicit unknown sentinel at zero. Shared by every Gatling codec; standard
  library only. (#3)
- `gatling/text`: a streaming `Reader` for the tab-separated `simulation.log` written by Gatling
  3.11.5 through 3.12.0, and `SupportedVersions` reporting that range. It reads from any
  `io.Reader` with memory independent of the log's size (one 1 MiB line buffer and a bounded table
  of the names the log repeats), walks the assertion records that precede the run header, gates on
  the version — refusing anything below 3.11.5 or not a plain release, warning above 3.12.0 — and
  yields records in file order. The first line it cannot decode ends the read with the line number;
  a partial read is never presented as a result. Decodes at about 450 MB/s on one core with no
  allocation per record. (#3)
- Golden corpus `testdata/corpus/gatling/3.11.5/` and `3.12.0/`: one real run each of the same
  simulation, with the two statistics files Gatling generated for it and the decoded record stream,
  plus the simulation and stub that recorded them. (#3)
- Verification pipeline: `verify.yml` holds the gate set once — `quick`, `lint`, `test`, `e2e`,
  `canary`, `deps`, `vuln`, `coverage` — and both `ci.yml` and `release.yml` call it, so adding a
  gate is a one-file edit. `ci.yml` publishes a single required check, `verify`, which is green for
  a documentation-only change, red when any gate fails, and never absent: a cancelled run fails it,
  because suppressing a run is not passing. (#3)
- Tag-driven release: pushing `vX.Y.Z` runs `release.yml` — a guard that refuses a tag outside
  `main` or its own `release/X.Y.0`, refuses a milestone that is not tag-ready, and refuses a
  version already released; then the full gate set; then publication, ordered so the only
  irreversible act is last. Notes come from `cliff.toml`, whose catch-all group lists a commit that
  ignored the convention rather than dropping it.
- `scripts/check-coverage.sh` builds the per-package coverage table from one profile and, with
  `--enforce`, applies the 90% decoder and 80% module floors. A package with no statements reports
  `n/a`, not `0%`. `scripts/e2e-inventory.sh` turns `go test -json` into the executed-case
  inventory and refuses a run in which nothing executed.
- `.github/ruleset-main.json` declares the branch-protection ruleset, so requiring `verify` is one
  `gh api` call rather than a paragraph of instructions.
- Repository scaffold: Go module, linter configuration, CI, licence and the spec-driven development flow.

### Fixed

- `scripts/check-linkage.sh --for-tag` resolves a release tag to its own milestone, falling back to
  the `vX.Y.0` milestone, and matches the version on a boundary so `v0.0.1` cannot select
  `v0.0.10`. It previously looked only for `vX.Y.0` and could not find a milestone for any 0.0.x
  version, refusing every release in that range for a reason unrelated to the release.
- A milestone with no issues and no pull requests is no longer reported tag-ready. It warned before,
  so an empty or mistakenly created milestone could shadow the real one and pass the tag gate.
- The tag gate now audits pull requests merged since the previous release, not only those already
  carrying the milestone. A merged pull request with no milestone is invisible to a milestone query,
  which is precisely the case the rule exists to catch.
