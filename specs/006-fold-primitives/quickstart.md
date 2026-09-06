# Quickstart: validating the fold primitives and the three fixes

**Feature**: `006-fold-primitives` | **Date**: 2026-09-06 | **Spec**: [spec.md](./spec.md)

Every scenario is runnable and names the success criterion it proves. The values are described in
[data-model.md](./data-model.md), the API in [contracts/model-fold.md](./contracts/model-fold.md)
and [contracts/gatling-fixes.md](./contracts/gatling-fixes.md), the reasons in
[research.md](./research.md).

## Prerequisites

- Go 1.25
- The recorded corpus already in the tree — nothing new is recorded for this feature

## The one command that must be green

```bash
go build ./... && go test -race -shuffle=on ./...
```

---

## Scenario 1 — a position is one value, and two folds agree on it

**Proves**: SC-001, SC-005; FR-001 … FR-007.

```bash
go test -race -shuffle=on -run 'TestPosition|TestTwoConsumersBucketAlike' ./model/
```

**Expected**: green. The table covers a request outside any group against the same name inside
one; a group traversal `a` → `b` against a request `b` inside `a`; a single group named `a,b`
against nested `a` then `b`; names holding a comma, a slash, a tab and a NUL; empty names; the zero
value; and the round trip of `Groups()` and `Name()` for all of them. One case takes a position,
overwrites the backing array it was made from, and requires the position unchanged.

Then over real runs:

```bash
go test -tags=integration -race -run 'TestPrimitiveFoldMatchesTheHandRolledOne' ./gatling/text/ ./gatling/binary/
```

**Expected**: green for all five corpus runs — the primitive fold's positions map one-to-one onto
the keys the suite has kept by hand since spec 002, with identical counts under each.

## Scenario 2 — the bounds are the span the report used

**Proves**: SC-002; FR-008 … FR-015.

```bash
go test -race -shuffle=on -run 'TestBounds' ./model/
```

**Expected**: green. Cases: a user START before the first request sets the start; a user END after
the last request sets the end; an absent sample end leaves the end alone and still sets the start;
a zero start counts nothing; a group's wall-clock end counts and its cumulated time does not; an
error and an assertion move nothing; the same items shuffled give the same bounds; an empty fold
reports both absent.

```bash
go test -tags=integration -race -run 'TestBoundsReproduceTheReportRate|TestBoundsReproduceTheConsoleThroughput' ./gatling/text/ ./gatling/binary/
```

**Expected**: green. For the text runs, `checkRates` reproduces `global_stats.json`'s and
`stats.json`'s mean requests per second from the primitive bounds exactly. For the binary runs, the
console summary's `mean throughput (rps)` line is reproduced for total, OK and KO at the precision
the console printed — 25.5, 21 and 4.5 for 3.15.1 — and for 3.13.1, whose console has no such line,
`global_stats.json`'s `meanNumberOfRequestsPerSecond` is.

## Scenario 3 — the consumer's loop, and nothing computed here

**Proves**: SC-003, SC-010; FR-016 … FR-021.

```bash
go test -race -shuffle=on -run 'Example_fold|TestExportedSurfaceIsGolden' ./model/
```

**Expected**: the example's output matches `// Output:` — 102 requests, 84 ok, 18 ko, a 4 s span,
25.5 / 21 / 4.5 rps — and the exported surface matches `model/testdata/exports.golden`. To see the
check work, add any exported identifier to `model` and run again: it fails naming the addition.
Regenerate deliberately with:

```bash
go test ./model/ -run TestExportedSurfaceIsGolden -update && git diff model/testdata/exports.golden
```

The diff is what a reviewer approves.

## Scenario 4 — both codecs, one answer to bad input

**Proves**: SC-006, SC-007; FR-022 … FR-025.

```bash
go test -race -shuffle=on -run 'TestCodecsAgreeOnMalformedInput' ./gatling/binary/
go test -race -shuffle=on -run 'TestParseRecord|TestAnAbsentTimeIsTheZeroTimeNotADateInThePast' ./gatling/text/
go test -race -shuffle=on ./internal/wire/
```

**Expected**: green. On `main` before the fix, the agreement test fails for the negative time and
the negative cumulated value — text refuses, binary accepts — and passes for the group-less record,
which both already return empty and non-nil.

Then the goldens, without `-update`:

```bash
go test -tags=integration -race -run 'TestGolden' ./gatling/text/ ./gatling/binary/
```

**Expected**: green — every recorded record stream is byte-identical.

## Scenario 5 — a large assertion suite decodes

**Proves**: SC-008; FR-026 … FR-028.

```bash
go test -race -shuffle=on -run 'TestALargeAssertionSuiteDecodes|TestAScenarioListAboveTheGroupDepthDecodes|TestAnAbsurdGroupDepthIsRefused|TestACorruptCountIsRefusedBeforeAllocating' ./gatling/binary/
```

**Expected**: green — 2,000 assertions come back; 1,025 scenarios come back; a depth of 1 << 20
fails naming its offset; a count of `math.MaxInt32` for each of the three fails at its offset with
no allocation sized by it.

## Scenario 6 — the README agrees with the package documentation

**Proves**: SC-009; FR-029 … FR-031.

```bash
grep -rn 'testdata/samples' --include='*.go' . ; echo "exit $?"
grep -n 'gatling/binary' README.md doc.go
go test -race -run 'TestDetect' ./gatling/
```

**Expected**: the first grep prints nothing and exits 1; the second shows the binary codec in both
files with the range 3.13.1 through 3.15.1; the detection tests are unchanged and green.

## Scenario 7 — memory does not move

**Proves**: SC-004.

```bash
go test -tags=integration -count=1 -run 'PeakMemory$' ./gatling/binary/
```

**Expected**: green — a 256 MiB synthetic log folded through `Position()` and `Bounds.Extend` for
every item peaks under 32 MiB, and a log ten times larger peaks within twice that figure (measured:
3.6 MiB and 3.7 MiB). `-short` skips the larger run. Run it without `-race`, as CI's
bounded-memory step does: the race detector multiplies the time by about seven and measures nothing
about the heap, and the race job skips every `PeakMemory$` test for that reason.

## Scenario 8 — the cost of the fold

**Proves**: the performance goal in [research.md R12](./research.md).

```bash
go test -tags=integration -run '^$' -bench 'BenchmarkDecodeToModel|BenchmarkFold' -benchmem -benchtime=2s ./gatling/binary/
```

**Expected**: `BenchmarkFold/synthetic-64MiB` reports at most one allocation per position beyond
`BenchmarkDecodeToModel`'s count, and its `ns/op` divided by `items/op` is under twice the decode
figure (107 ns/item on the reference machine). Record both lines in the PR.

## Scenario 9 — coverage and the gates

**Proves**: SC-011; the constitution's Quality Gates.

```bash
gofmt -l . ; go vet ./... ; go mod tidy && git diff --exit-code go.mod go.sum
go test -race -shuffle=on -cover ./model/ ./gatling/... ./internal/...
```

**Expected**: nothing to format, nothing to tidy, and coverage at or above 90% for `model`,
`gatling/text` and `gatling/binary`, 80% overall. The integration suite runs the way CI runs it, in
two halves — the race job skips the memory tests and the memory step runs them without `-race` —
because the two memory tests of `gatling/binary` alone take eleven minutes under the race detector,
past `go test`'s default ten-minute limit:

```bash
go test -tags=integration -race -shuffle=on -count=1 -skip 'PeakMemory$' ./...
go test -tags=integration -count=1 -run 'PeakMemory$' ./...
```

Before each PR:

```bash
scripts/check-linkage.sh --pr <N>
```

**Expected**: the PR carries milestone v0.0.6 and names the issue its one commit closes.
