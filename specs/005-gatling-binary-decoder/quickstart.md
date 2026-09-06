# Quickstart: validating the binary codec

**Feature**: `005-gatling-binary-decoder` | **Date**: 2026-09-06 | **Spec**: [spec.md](./spec.md)

Every scenario is runnable and maps to a success criterion. The grammar is in
[data-model.md](./data-model.md), the API in
[contracts/gatling-binary.md](./contracts/gatling-binary.md), the evidence in
[research.md](./research.md).

## Prerequisites

- Go 1.25
- The recorded corpus (scenario 1 creates it, once)
- A JDK and `sbt` for that recording only

## The one command that must be green

```bash
go build ./... && go test -race -shuffle=on ./...
```

---

## Scenario 1 — record the corpus (once, and it cannot be redone)

**Proves**: FR-028 … FR-032. Everything below depends on this, and none of it can be captured after
a run finishes.

Extend the probe **first** — a run cannot be given these afterwards:

- a request name outside Latin-1, so coder 1 is exercised at all;
- a name repeated far more often than it is introduced, so the string table is a table;
- a group name repeated across users, so a cached string appears inside a group path;
- assertions in Gatling's own DSL for 3.14.x and 3.15.x, because `gatling-picatinny` has no release
  on those lines.

Then, per version:

```bash
cd testdata/corpus/gatling/simulation && sbt -Dgatling.version=3.15.1 "Gatling/testOnly io.galaxio.parsec.corpus.CorpusSimulation" 2>&1 | tee /tmp/console-3.15.1.txt
```

Keep, under `testdata/corpus/gatling/<version>/`: `simulation.log` exactly as written, the generated
`index.html`, and the console output. Write `RECORDING.md` beside them with the machine, the JVM, the
command and what the run exercised — and state that the two statistics files do not exist for this
version, so a later reader knows the absence is Gatling's.

Expected: three recordings, for **3.13.1, 3.14.9 and 3.15.1**. The floor is 3.13.1 because that is
the oldest version actually recorded — Principle II binds the range to the corpus even when the
source proves the format never moved. The format began at 3.13.0, but 3.13.0 cannot generate a
report at all (it fails to read back its own assertion records), so no run of it can carry the
second account of its numbers a corpus entry needs. See [research.md R8](./research.md).

---

## Scenario 2 — the format decodes as the source says it does

**Proves**: SC-001, FR-001 … FR-009.

```bash
go test -tags=integration -race -run 'TestGolden|TestVersionsAgree|TestTheEmptyString|TestCache' ./gatling/binary/
```

Expected: every corpus log decodes to exactly its recorded record stream, field for field. The two
traps from research R2 have their own cases and must fail without the fix:

- an **empty string** is `int32(0)` and no coder byte — the normal case for a description and for a
  successful request's message, so getting it wrong desynchronises most of the file;
- **scenario names are not cached** — feeding them to the table shifts every later index and renames
  every record without failing.

---

## Scenario 3 — the numbers match what Gatling said about the same run

**Proves**: SC-002.

```bash
go test -race -run 'TestDecodedCountsMatchWhatTheRunReported|TestCountsGatlingDoesNotReport' ./gatling/binary/
```

Expected: counts folded from the decoded records equal the figures in that run's own HTML report —
the total row and each request row, `value total` / `value ok` / `value ko` — and the console
summary's Global Information block agrees with them. Two independent accounts from the tool itself;
a difference of one fails.

---

## Scenario 4 — a name survives its alphabet

**Proves**: SC-003, FR-004, research R4.

```bash
go test -race -run 'TestTheCyrillicName|TestOneLogHoldsBothEncodings|TestEveryDecodedString|TestStringsDecodeInBothEncodings' ./gatling/binary/
```

Expected: the non-Latin-1 request name decodes byte-identical to what the simulation declared. A
coder value that is neither 0 nor 1 fails the read naming the offset rather than producing a
replacement character.

---

## Scenario 5 — the string table, and the reference that is not there

**Proves**: FR-010 … FR-013, SC-007.

```bash
go test -race -run 'TestCache|TestNamesAreRepeated|TestADanglingGroupReference|TestOneCorruptedByte' ./gatling/binary/
```

Expected: a name written once and referenced many times yields the same string every time; a
back-reference to an entry never introduced fails with the byte offset; a positive index that is not
the next expected one fails, because the writer's counter is strictly sequential and a gap means the
stream desynchronised earlier.

---

## Scenario 6 — nothing wraps, nothing is invented

**Proves**: FR-009, research R3.

```bash
go test -race -run 'TestGolden|TestPrimitivesReadWhatTheWriterWrote' ./gatling/binary/ ./internal/wire/
```

Expected: every offset resolves against the run's start to the same instant the text codec reports
for an equivalent event. An offset that resolves before the run began is reported **absent**, not as
a wrapped negative — the 32-bit millisecond offset covers 24.8 days and Gatling's own writer
overflows past that.

---

## Scenario 7 — a corrupt length cannot ask for the machine

**Proves**: FR-025, SC-008.

```bash
go test -race -run 'TestALengthPast|TestAnAbsurdGroupDepth|TestAnAssertionPayloadPastTheEnd|TestTruncation|TestAScenarioIndexOutside' ./gatling/binary/
go test -race -run '^$' -fuzz FuzzDecode -fuzztime 60s ./gatling/binary/
```

Expected: a length past `MaxStringLen` fails **before** allocating; a negative length fails; at least
10,000 mutated and truncated inputs produce only errors, never a panic and never an allocation past
the cap.

---

## Scenario 8 — chunked equals whole-file

**Proves**: FR-027, SC-006.

```bash
go test -race -run 'TestChunkedReadsMatchWholeFile|TestATruncatedLogFails|TestAFailedReadStaysFailed|TestACleanEndStaysClean' ./gatling/binary/
```

Expected: reading a corpus log one byte at a time, and in arbitrary chunk sizes, produces the same
records as one pass — and a log that fails fails at the same offset with the same error.

---

## Scenario 9 — the caller stops caring which Gatling ran

**Proves**: SC-010, SC-011, FR-017 … FR-019.

```bash
go test -race ./gatling/simlog/
```

Expected: a binary log opened through `simlog` yields records identical, field for field, to opening
it through `gatling/binary` directly; `Supported()` reports the binary format as readable over
3.13.1–3.15.1, derived from the codec; and the same report code renders a text run and a binary run
with no format-specific branch.

---

## Scenario 10 — memory follows names, not records

**Proves**: SC-005, FR-026, FR-013.

```bash
go test -tags=integration -race -run 'TestPeakMemory|TestMemoryFollowsNamesNotRecords' ./gatling/binary/
go test -tags=integration -run '^$' -bench 'BenchmarkDecode' -benchmem ./gatling/binary/ ./gatling/text/
```

Expected: a 1 GB log reads with peak memory under 32 MiB, and that figure does not move when the log
is made ten times longer **with the same set of distinct names**. Allocations per record approach
zero once every name is cached. Compare against `gatling/text` on the same simulation with
`benchstat` and put the numbers in the PR — the binary path should be the faster of the two, and if
it is not, something is copying that should not be.

---

## Scenario 11 — capabilities are compared, not assumed

**Proves**: FR-021, research R10.

```bash
go test -race -run 'TestOneReportRendersBothFormats' ./gatling/binary/
```

Expected: the binary codec's capability set is stated and compared against the text codec's. The
expected answer is that they are identical; the test asserts which it is, so a later divergence in
either format is visible rather than smoothed over.

---

## Scenario 12 — nothing that passed before changed

**Proves**: SC-013.

```bash
go test -race -shuffle=on ./gatling/text/ ./model/
```

Expected: unchanged. The conversion moved to `internal/wire` and the text codec calls it there; if a
test in `gatling/text` had to be edited, the move was not the refactor it claimed to be.

---

## Coverage

```bash
go test -tags=integration -count=1 -skip 'PeakMemory$' -coverpkg=./... -coverprofile=cover.out ./...
bash scripts/check-coverage.sh --enforce cover.out
```

Expected: `gatling/binary` at 90% or above — `*/gatling/*` maps to the decoder floor — and the module
at 80% or above.
