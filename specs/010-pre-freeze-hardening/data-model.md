# Data Model: Pre-freeze hardening

**Feature**: 010-pre-freeze-hardening | **Date**: 2026-09-10

No type in `model/` or `gatling/` changes, no field is added, and no exported identifier appears.
What this feature models is *behaviour that is already documented and will be frozen*: how the end
of a stream is classified, in what order the run record is judged, what the binary reader is allowed
to retain, and how one run is chosen over another. Each is stated here as the thing a test asserts.

---

## 1. What a source does, and what every constructor says about it

The four ways a source can stop, and the answer each of the six constructors gives —
`simlog.NewReader`, `simlog.NewRunReader`, `binary.NewReader`, `binary.NewRunReader`,
`text.NewReader`, `text.NewRunReader`. Rows marked **changes** are what this feature fixes; every
other row is unchanged and is asserted so it stays that way.

| The source… | Meaning | Codecs today | `simlog` today | After |
|---|---|---|---|---|
| returns `io.EOF`, by identity, on a call of its own, at a record boundary | **end of stream** | `io.EOF` from `Next` (clean) | `FormatError{Short: true}` when inside the window; else the codec's answer | unchanged |
| returns `io.EOF`, by identity, inside a value | **cut short** | `*TruncationError` | `FormatError{Short: true}` inside the window | unchanged |
| returns its **final bytes together with `io.EOF`** in one call, completing the value | end of stream | **`*TruncationError` for a value ≥ 64 KiB** (#82) | tolerated | **the value is complete; the log ends cleanly** — R1 |
| returns its **final bytes together with another failure** in one call | source failure | a failure | a failure | **unchanged: a failure** — R1 |
| fails with an error that **wraps** `io.EOF` | **source failure** | a failure; chain cut; text kept | **`errors.Is(err, io.EOF)` is true** (#83) | **a failure; `io.EOF` hidden, every other cause kept; text kept** — R3 |
| fails with `io.ErrUnexpectedEOF` (a torn gzip, flate or zlib stream) | source failure | a failure, wrapped with `%w` | **`FormatError{Short: true}`** (#102) | **a failure, wrapped with `%w`** — R4 |
| fails with any other error | source failure | a failure, wrapped with `%w` | a failure, wrapped with `%w` | unchanged |
| returns `(0, nil)` repeatedly | stalled | `io.ErrNoProgress` | `io.ErrNoProgress` | unchanged |

**Invariants** (contract [endings.md](contracts/endings.md)):

1. No error returned for a source failure satisfies `errors.Is(err, io.EOF)`, from any constructor.
2. Only `io.EOF` from the source itself, by identity, may become a clean end, a cut, or a short head.
3. A source failure's message contains the cause's text, and every cause in its chain but `io.EOF`
   stays reachable through `errors.Is` and `errors.As` (research R3).
4. A read that fills the value it is reading and reports `io.EOF` in the same call is a successful
   read; any other error that arrives with the last bytes is reported as the source's failure
   (research R1).

## 2. The order the run record is judged in

One order for both codecs (research R5). Each row is reached only if every row above it passed.

| Step | What is read | Fault | Result |
|---|---|---|---|
| 1 | enough of the header to reach the version — six fields on the text `RUN` line; the first string of the binary run record | cannot be read; a binary version string past 64 bytes | `*SyntaxError` / `*TruncationError` (unchanged) |
| 2 | the version string | not a release | `*VersionError{Parsed: false}`, quoting it |
| 3 | the version, against the codec's range | below | `*VersionError` |
| 3 | | above, under `WithStrict` | `*UnverifiedError` |
| 3 | | above | `Warning` recorded; continue leniently |
| 4 | the rest of the header: simulation class, run id, run start (bounded by `MaxRunStart`), description — and, binary only, the scenario names and assertion payloads | out of bounds, corrupt count, a table past its ceiling | `*SyntaxError` naming the position |

**What moves**: steps 2–3 now precede step 4 in the binary codec (today they follow it), and precede
the run-start check in the text codec (today the start is validated first). The text codec also rules
on an `ASSERTION` line with too few fields after step 3 rather than where it reads it, so a version
the gate refuses outranks it; with no version to rule on, the earliest fault in the file is reported,
as today. An assertion table past its 8 MiB ceiling is the exception: the text codec meets it before
the version and refuses it as damage there. The decision at step 3 is unchanged.

**Cost before a refusal** (binary): at most one read-buffer fill — 64 KiB — whatever the scenario and
assertion tables claim, and whatever length the version string claims, since one past 64 bytes is
refused unread. Today a refused log is read to those tables' ceilings first.

## 3. What the binary reader retains, and its ceilings

The reader holds three tables for the life of a read. Each entry is accounted as **its content plus
a 16-byte string header**, as it arrives; a table past its ceiling ends the read (research R6).

| Table | Written by | Count ceiling | Byte ceiling (header-inclusive) | Refusal |
|---|---|---|---|---|
| assertion payloads | the run record | `maxAssertions` = 65,536 | `maxAssertionBytes` = 8 MiB | `*SyntaxError` at the payload's length prefix: "N bytes, past the ceiling of M" |
| scenario names | the run record | `maxScenarios` = 65,536 | `maxScenarioBytes` = 1 MiB *(new)* | `*SyntaxError` at the name's offset, same wording |
| the string table | every record that introduces a string | *(none — `maxCacheEntries` removed; the byte ceiling binds first at 786,432 entries)* | `maxCacheBytes` = 12 MiB *(new)* | `*SyntaxError` at the entry's index, same wording |

Beside the tables: the fixed 64 KiB read buffer (the codec's own — never a caller's, research R2),
a scratch buffer of at most `MaxStringLen` (1 MiB), and the reused group path.

**Invariants** (contract [binary-budget.md](contracts/binary-budget.md)):

1. Retained memory is bounded by the ceilings above, independent of the log's length: about 26 MiB
   in the worst case, under the 32 MiB the reader documents.
2. No log the golden corpus holds, and no log Gatling writes for a simulation whose names, groups and
   failure messages come to under 12 MiB of distinct text, is refused by a ceiling.
3. A log past a ceiling is refused as damaged, at the entry that crossed it, before anything more is
   allocated.
4. The read buffer is allocated by the codec whatever the caller passes; a caller's `*bufio.Reader`
   of the codec's size or smaller has nothing buffered after `NewReader`.

## 4. The ordering key for `run.Find`

When several runs are candidates — every run in the root, or every run `lastRun.txt` names — the
newest is the maximum under one key (research R7):

| Position | Component | Source | Direction |
|---|---|---|---|
| 1 | modification time of the run's `simulation.log` | `stat` | later wins |
| 2 | the 17-digit UTC stamp the run id ends with, or `""` for a name without one | the directory name | greater wins; an unstamped name ranks below every stamped one |
| 3 | the directory name | the directory name | greater wins |

**Invariants** (contract [run-ordering.md](contracts/run-ordering.md)):

1. The comparison is a total order: transitive, antisymmetric, and independent of the order the
   directory was read in.
2. A run with a newer stamp never loses to one with an older stamp because a third, unstamped
   directory is present.
3. The signals and their precedence — time, stamp, name — are the ones documented since v0.0.9.
   What changes is only that a stamped name always outranks an unstamped one at the same time,
   where today the pair was compared by whole name.

## 5. What does not change

`model.Run`, `model.Item`, `gatling.Header`, `gatling.Record`, every error type and every
constructor signature. `Capabilities` declares nothing new. The corpus decodes to the same record
streams (FR-019), and chunked and whole-file reads still agree — now through one more source shape,
`iotest.DataErrReader`, in the binary chunk matrix.
