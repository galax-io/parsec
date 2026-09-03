# Phase 0 Research: Gatling Text simulation.log Decoder

**Feature**: `002-gatling-text-decoder` | **Date**: 2026-09-02 | **Plan**: [plan.md](plan.md)

Every decision below is evidence about an undocumented external format. The evidence is Gatling's
own source at tags `v3.11.5` and `v3.12.0`. None of that code is carried over — what is recorded
here is what the format does, not how their implementation does it, and this module owes it no
structural resemblance. Per Principle III the corpus is the final authority: where a recording
disagrees with anything below, the recording wins and this file is corrected.

---

## R1. Is the on-disk format the same in 3.11.5 and 3.12.0?

**Decision**: Yes. One codec covers both, and User Story 2's cross-version equality is a
verifiable claim rather than an assumption.

**Rationale**: the six format-bearing sources were diffed across the two tags.

| Source | Verdict |
|---|---|
| `gatling-core` `stats/writer/LogFileDataWriter.scala` | differs — refactoring only |
| `gatling-core` `stats/writer/RawRecords.scala` | differs — reader tolerance only, see R4 |
| `gatling-charts` `stats/Records.scala` | identical |
| `gatling-charts` `template/GlobalStatsJsonTemplate.scala` | identical |
| `gatling-charts` `stats/ResultsHolder.scala` | identical |
| `gatling-charts` `stats/buffers/GeneralStatsBuffers.scala` | identical |

The writer diff moves a constructor inline, tightens visibility on constants and helpers, and adds
a name to the writer. Every serializer, every field, every separator and the assertion-before-header
ordering are untouched. Nothing a reader can observe changed.

**Alternatives considered**: recording both versions and diffing the artefacts alone. Rejected as
the *only* method — it proves the two sample runs agree, not that the format does, and a field
absent from the sample would hide a difference. The source diff and the corpus are complementary;
the plan keeps both.

---

## R2. Record layout

**Decision**: six kinds, tab-separated, with these exact field counts.

| Kind | Fields after the kind | Count |
|---|---|---|
| `RUN` | simulation class, run id, start, description, Gatling version | 6 |
| `USER` | scenario, `START`\|`END`, timestamp | 4 |
| `REQUEST` | group path, name, start, end, `OK`\|`KO`, message | 7 |
| `GROUP` | group path, start, end, cumulated response time, status | 6 |
| `ERROR` | message, timestamp | 3 |
| `ASSERTION` | base64 payload | 2 |

**Rationale**: the counts are declared in `RawRecords.scala`; the field order and meaning follow
from the serializers in `LogFileDataWriter.scala` and the parsers in `Records.scala`, which agree.

**Consequences already folded into the spec**:

- An empty run description and an absent request message are each written as a single space, so
  a lone space decodes as empty (FR-003).
- A request's group path is empty for a top-level request; a comma inside a group name is replaced
  with a space on write, so every comma in a path is a separator and the split is lossless (FR-005).
- Only a request's failure message is escaped. Scenario, request and group names and error messages
  are not — see R5.

---

## R3. Where the assertion records are, and how many

**Decision**: assertion records come **before** the run header, one per declared assertion. The
run header is not necessarily line 1, and a reader that assumes it is refuses every log from a
simulation that declares assertions.

**Rationale**: on initialisation Gatling serialises each assertion and only then the run message.
This was the single most consequential correction to the specification — the spec had previously
described one assertion record at the end of the log.

**Design consequence**: `NewReader` walks the preamble, collecting assertion payloads until it
reaches the run header, and refuses any other record kind before the header, since nothing may be
decoded before the version is known. The payloads are held in memory; their number is a property
of the simulation, not of the log, so this does not breach the bounded-memory requirement. That
distinction is stated in the contract rather than left implicit.

**Alternatives considered**: yielding preamble assertions through `Next` and making the header
available only afterwards. Rejected — FR-007 requires the header before any event record, and
replaying a buffered preamble to preserve file order buys nothing a caller can use.

---

## R4. Exact field count, or a minimum?

**Decision**: exact inside the covered range; minimum above it.

**Rationale**: this is the one place the two supported versions genuinely differ.

```text
3.11.5   array.length >= recordLength     // surplus tolerated
3.12.0   array.length == recordLength     // surplus rejected
```

The written format has always carried exactly the required count, so inside the covered range a
surplus field can only be corruption, and the fail-fast rule says corruption ends the read. Above
the covered range the calculus inverts: a future version that appends a field is exactly the case
Principle II covers with "an unknown newer version MUST decode and MUST surface a warning", and
hard-failing on field count would break that MUST while the warning sat unused in the result.

**Alternatives considered**: adopting 3.12.0's strictness everywhere. Rejected — it turns the
version warning into a lie, since the log it warns about would never decode. Adopting 3.11.5's
leniency everywhere was rejected for the opposite reason: it silently accepts corrupt lines inside
the range the corpus actually covers, which is where exactness has to hold.

---

## R5. Unescaped values, and the one exception the error record needs

**Decision**: an error record's message is everything between the kind and the final timestamp
field, however many separators it spans (FR-008b). The other five kinds keep the exact count.

**Rationale**: Gatling escapes tab, carriage return and newline only in a request's failure
message. An error record's message is built by concatenating the request name with the raw crash
text and is written unescaped, so a tab inside it reaches the file. Such a record is normal
output, not damage, and refusing it would refuse a log Gatling itself wrote. The trailing field is
a timestamp, which makes the message boundary unambiguous without guessing.

A line break in that text is a different matter: it splits the record across lines on disk, and no
reading rule recovers it. That fails the read, which is the correct outcome — the record really is
broken. The sample simulation should provoke a multi-line error so the corpus records whether this
occurs in practice.

**Alternatives considered**: repairing the value. Rejected — the spec forbids repairing content,
and a repaired message is no longer what the run recorded.

---

## R6. Line terminator

**Decision**: accept both `\n` and `\r\n`; strip a single trailing carriage return.

**Rationale**: Gatling terminates records with the platform line separator, so a run on Windows
produces carriage returns natively. This is not a copying artefact and must not be treated as
damage — under the fail-fast rule, treating it as damage would refuse every Windows-produced log
outright.

---

## R7. What the kept report files carry, and how the mean rate is defined

**Decision**: keep `js/global_stats.json` and `js/stats.json`. Compare request counts and the mean
request rate exactly; verify user and error counts against the golden record stream instead.

**Rationale**: both files carry request counts split total/ok/ko, response-time statistics with
percentiles, response-time range buckets, and the mean number of requests per second — the last of
which is per run in the first file and per request and group in the second. Neither carries
virtual-user counts or error-record counts, which is why FR-021a routes those elsewhere.

The mean rate is the request count divided by the run span in whole seconds, rounded up:

```text
durationInSec      = ceil((maxTimestamp - minTimestamp) / 1000)
meanRequestsPerSec = count / durationInSec
```

Because that is a deterministic function of the very records being decoded, it is asserted exactly
rather than within a tolerance — a mismatch means the records or the span were decoded wrongly, not
that a statistic drifted. The span is bounded exactly as the report bounds it (FR-021c): `minTimestamp`
is the least of request starts, group starts and user START timestamps; `maxTimestamp` is the greatest
of request ends, group ends and user event timestamps of either kind. The run header's start and error
records take no part — confirmed from the first pass of Gatling's own reader, which was also where the
earlier draft of this rule (header included) was found to be wrong.

**Still owed, and unrecoverable if skipped**: this describes the code that generates the report,
not a report a real run produced. The recording task must confirm both files' contents at capture
time, because Principle III makes the moment of recording the only moment this can be captured.

---

## R8. Reader shape

**Decision**: an explicit constructor that establishes the header and the gate, then one record at
a time.

```go
r, err := text.NewReader(f)   // walks preamble, reads header, applies the gate
r.Header()                    // available immediately (FR-007)
r.Assertions()                // preamble payloads, verbatim (FR-006)
r.Warnings()                  // version warning, reachable in the result (FR-011)
for {
    rec, err := r.Next()      // io.EOF at end; any other error ends the read
}
```

**Rationale**: the constructor is the natural home for the gate, because nothing may be decoded
before the version is known, and it is what makes FR-007 structural rather than a convention a
caller must remember. `Next` returning `(Record, error)` is the conventional Go decoder shape,
trivially wrappable by a consumer, and it makes the fail-fast rule the plain reading of the code:
one error, and there is no next record.

**Alternatives considered**: a `Scan`/`Err` pair in the manner of `bufio.Scanner`. Rejected — it
separates the failure from the record that caused it and invites a caller to ignore `Err`, which
is precisely the mistake FR-014 exists to prevent. An `iter.Seq2` iterator was rejected for now as
API surface with no current consumer; it can be added later without breaking `Next`.

---

## R9. Record representation

**Decision**: one flat `Record` struct carrying a `Kind`, returned by value. No interface, no type
per kind.

**Rationale**: a sum type over six kinds would box every record on the heap, and at the throughput
this feature targets that allocation is the dominant cost. The standard library decodes this way
for the same reason. These are wire records with heavily overlapping fields, not a domain model —
the domain model is milestone v0.0.3, and it is free to choose differently. Which fields are
meaningful for which kind is documented per field in the contract and enforced by the parsers.

**Alternatives considered**: an interface with one concrete type per kind. Rejected on allocation
cost and on Principle VI — the second codec has not landed yet, so the abstraction has no second
implementation to justify it. Revisit when the binary codec arrives, which is when a shared
interface would earn its place.

---

## R10. Golden record stream format

**Decision**: commit `records.golden` next to each recorded log — the decoded stream in a canonical
one-record-per-line text form, regenerated by a `-update` test flag and reviewed as a diff.

**Rationale**: Principle III requires decoder output compared field for field against a recorded
stream. A text golden file makes a regression legible in review, which a binary dump or an
in-code table does not. Regeneration must be an explicit flag so that a golden file never updates
itself as a side effect of a passing run.

**Alternatives considered**: asserting against a Go table in the test file. Rejected — for a run
with thousands of records the table becomes unreadable and stops being reviewed, which is the same
as having no golden at all.

---

## R11. Performance targets

**Decision**: ≥ 100 MB/s on one core, peak heap under 32 MiB for a log of any size, 1 MiB per line.

**Rationale**: the work per line is a tab split and a handful of integer parses, so throughput is
bounded by scanning and by allocation, not by parsing. 100 MB/s leaves clear headroom over what
byte-wise scanning costs while still failing loudly if a per-record allocation creeps in. The
memory ceiling comes from SC-004 and the line ceiling from FR-016; together they mean a corrupt
length cannot exhaust memory. The benchmark records the number, and a regression against it must be
justified in the PR rather than absorbed.

**Alternatives considered**: no throughput target, memory only. Rejected — the constitution
requires a plan for a decoder feature to state both, and a memory-only target passes a decoder
that is correct and unusably slow on a soak-run log.

---

## R12. Two measured optimisations, and one that was not one

**Decision**: keep a bounded name table and a single-pass field split. Both were applied one at
a time, each measured against the previous state with `benchstat` over six runs of the 64 MiB
synthetic log.

| Step | sec/op | B/s | allocs/op | Verdict |
|---|---|---|---|---|
| baseline | 157.8 ms ± 4% | 405.6 MiB/s | 2,067,796 | — |
| + name table | 162.0 ms ± 3% | 395.1 MiB/s | 28 | speed unchanged (p=0.093); garbage −96% |
| + single-pass split | 142.4 ms ± 2% | 449.5 MiB/s | 27 | **−12.1% time, +13.8% throughput (p=0.002)** |

**Rationale**: the CPU profile put a third of the time in the byte search inside `split` and a fifth
in string conversion plus its garbage collection. The name table shares one string per distinct
scenario, request, group and message — a log repeats a small vocabulary tens of thousands of
times — and turned two million allocations per read into twenty-eight. It did **not** make the
read faster: the hash lookup costs what the allocation cost. It stays because the garbage it
removes is paid by the consumer's collector, not by this benchmark's: a sidecar or a backend
embedding this reader beside a large heap is the case that matters. The table is capped at 4,096
values of at most 256 bytes, so memory stays independent of the log's size even when every line
carries a name never seen before (FR-017).

The single-pass split replaced one vectorised `bytes.IndexByte` per field with one walk over
the line. On fields ten bytes long the per-call overhead outweighed the search, and the walk was
twelve percent faster end to end. That was the intuition-free change: the profile named the
function, the benchmark confirmed it, and nothing else was touched in the same measurement.

**Alternatives considered**: a `sync.Pool` of records — rejected, records are returned by value and
allocate nothing. Hand-rolled integer parsing already existed. `unsafe` string views over the
line buffer — rejected outright: the buffer is reused, so a kept string would silently change
under the caller, and the constitution forbids exactly that class of surprise.

---

## R13. Naming: `NewReader`, not `New`

**Decision**: the constructor is `text.NewReader`, although the package has one primary type.

**Rationale**: it constructs an io-style reader, and the standard library names those `NewReader`
without exception — `bufio`, `csv`, `gzip`, `zlib`, `tar`. A Go reader looks for that name, and
`text.New(f)` says nothing about what it makes. The community convention this deviates from is
noted beside the declaration, as the convention itself asks.

---

## R14. The canary: a fresh Gatling per version, held to its own report

**Decision**: a workflow that starts every supported Gatling release for real, runs the probe
simulation, and holds the decoder to the report each run generated — reusable from `ci.yml`
on every change, startable by hand with any version list, and scheduled weekly (issue #15,
pulled into v0.0.2).

**Rationale**: the recorded corpus is immutable evidence about the past and cannot be re-made;
a fresh run is the only evidence about the tool as it is today, and the only way to try a
release that did not exist when the corpus was recorded. The two are not interchangeable: a
fresh run cannot be compared to the golden stream, because concurrent users interleave
differently and every timing differs — it can only be compared to its own report and to the
other fresh runs. So the canary reuses exactly the checks the corpus suite already makes
(`decodeTally`, `checkCounts`, `checkRates`, `maskedSorted`), now shared under
`//go:build integration || canary`, and applies them to whatever directories
`PARSEC_CANARY_RUNS` names.

**Design points**: the version list lives in the workflow; `TestCanaryCoversSupportedRange` fails
when `SupportedVersions` is widened without the new bound being run, so the two cannot drift. A
version above the range decodes unverified and is written to the job summary as a candidate —
the range is never widened by a machine. Without Gatling the tests skip with a reason, as the
constitution requires of a test that needs a real tool, and the job fails when no canary test
passed, so a skip cannot pass for a run.

**Alternatives considered**: one job per version through a matrix with artifacts handed to a
comparison job — the scaling shape once the binary codec adds 3.13.x and later, deferred until
there is more than one codec to run. Comparing fresh runs to the golden stream — rejected, see
above. Auto-widening the range from a passing canary — rejected by issue #15 itself.

**Scope note**: Gatling 3.13.0 and later write the binary format (milestone v0.0.5). Under this
canary they would be refused at the first line, so the newest release is not on the default
list yet; the "newest release" probe in issue #15 becomes meaningful when both codecs exist.
