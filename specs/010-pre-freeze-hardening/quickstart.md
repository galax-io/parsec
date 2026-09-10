# Quickstart: Pre-freeze hardening

How to convince yourself each of the seven fixes works, how to make each fail on purpose, and how to
re-check the two issues that landed elsewhere. Every command runs from the repository root and needs
only the Go toolchain; the golden corpus under `testdata/corpus/gatling/` is the only recorded input,
and every other input is built by the test that uses it.

## Prerequisites

```bash
go version   # go1.26.8, the toolchain go.mod pins since PR #110; go 1.25 is the consumer floor
```

```bash
go build ./... && go test ./...
```

That is the definition of a green tree. Everything below assumes it passes first, and the last
section is the whole gate set.

## 1. A complete upload is never reported as a killed run (US1, #82)

```bash
go test -race -run 'ReadFull|EndsWithItsLastBytes|ChunkedReadsMatchWholeFile' ./gatling/binary/
```

Expected: a run record whose trailing assertion payload is 200,000 bytes decodes identically
through `bytes.Reader`, `iotest.DataErrReader` and a one-shot source that returns everything with
`io.EOF` (only the one-shot shape reaches the read-buffer path; `DataErrReader` hands over a
kilobyte at a time); the same log with a payload under 64 KiB does too; the same log cut for real still ends in a `*gatling.TruncationError`; the same log read through the one-shot source with a
failure in place of `io.EOF` ends in that failure; every corpus file survives the chunk matrix, which now includes
`iotest.DataErrReader`.

**Make it fail on purpose**: in `readFull`, put the `err == io.EOF` arm back ahead of the fullness
check. The 200,000-byte case fails with "the log is cut short: 200058 trailing bytes could not be
decoded" — the issue's own message — while every `chunked` case stays green, which is why nothing
caught it before.

## 2. A broken source is a failure, never an ending and never "not yet" (US2, #83, #102)

```bash
go test -race -run 'SourceFailureIsNeverTheEndOfTheLog|UnexpectedEOF|TruncatedGzip|ShortHead|WrappedEOF' ./gatling/simlog/
```

Expected: for each of the six constructors and each of four sources — a plain failure, an error
wrapping `io.EOF`, `io.ErrUnexpectedEOF` by identity, an error wrapping it — the error is not
`io.EOF` under `errors.Is`, not a `*gatling.TruncationError`, not a `*gatling.FormatError`, its message contains the cause's text, and its chain still reaches
the cause. A tee whose spool fails beside the tenth byte is a failure too. A gzip of a text log cut at 14 of 92 bytes is a failure. Two bytes
then a genuine `io.EOF` is still `FormatError{Short: true}`.

**Make it fail on purpose**: (a) let `source.Failed` wrap with `%w` whatever the cause — the
wrapped-`io.EOF` rows fail on `errors.Is(err, io.EOF) = true`, for every constructor; (b) let
`readHead` return `io.ErrUnexpectedEOF` again — the two `io.ErrUnexpectedEOF` rows for `simlog` come
back as `FormatError{Short: true}`, while the codec rows stay green.

## 3. The memory budget holds for any log (US3, #75, #87)

The bound, measured as the live heap after a collection, alone and un-instrumented — the way CI runs
it:

```bash
go test -tags=integration -count=1 -run 'PeakMemory$' ./gatling/binary/
```

Expected: `TestDistinctScenarioNamesPeakMemory` and `TestDistinctStringsPeakMemory` are refused past
their ceilings with the heap under 32 MiB throughout; `TestTablesAtTheirCeilingsPeakMemory` is
**accepted** with the heap under 32 MiB — the log of the run prints the figure, measured at
23.8 MiB; the existing `TestPeakMemory` family is unchanged.

The ceilings themselves, and the buffer:

```bash
go test -race -run 'BoundedInBytes|CountTheirHeaders|CeilingStaysReachable' ./gatling/binary/ && go test -race -run 'CallersBufioIsNotAdopted' ./gatling/simlog/
```

Expected: each table is accepted exactly at its ceiling and refused one byte over with a
`*gatling.SyntaxError` at the crossing entry naming the total and the ceiling; 65,536 empty scenario
names are still accepted; both codecs handed a caller's 64 KiB `*bufio.Reader` leave it with nothing
buffered after construction (`TestACallersBufioIsNotAdopted`, landed with PR #111).

**Make it fail on purpose**: (a) count content only — `TestTablesAtTheirCeilingsPeakMemory` still
passes but the boundary test for empty scenario names shows 65,536 names costing 1 MiB of headers
against a ceiling that counted zero; (b) raise `maxCacheBytes` to 24 MiB — the at-the-ceilings test
crosses 32 MiB; (c) drop the `struct{ io.Reader }` wrapper — the binary rows of the buffer test
report a few thousand buffered bytes, the text rows none.

## 4. A refused log is refused first, the same way by both codecs (US4, #76)

```bash
go test -race -run 'GateBeforeTheRestOfTheRunRecord|RefusedBeforeTheScenarioCount|AtMostOneBufferFill|JudgedBeforeTheRunStart|AfterTheVersion' ./gatling/binary/ ./gatling/text/
```

Expected: a binary log naming 1.0.0 with a corrupt scenario count returns a `*gatling.VersionError`;
1.0.0 beside a run start past `MaxRunStart` returns a `*gatling.VersionError` from both codecs; a
non-release version beside that start returns one from both; 3.99.0 under `WithStrict` returns an
`*gatling.UnverifiedError` from both; two logs naming 1.0.0 with a three-name and a 200 KiB scenario
table pull the same number of bytes before refusal, at most one buffer fill.

**Make it fail on purpose**: apply `versionPolicy` after `readRunRest` again — the corrupt-count case
reports "expected the scenario count, found a count of -1", the divergence the issue measured.

## 5. The run found does not depend on directory order (US5, #88)

```bash
go test -race -run 'MixedStampedAndUnstamped|IndependentOfCandidateOrder|TieBreak' ./gatling/run/
```

Expected: the root `simA-20990101000000000`, `simB-20200101000000000`, `simAA` with equal log
times resolves to the run stamped 2099; every permutation and a thousand shuffles of random
candidate sets return one answer; the existing tie-break tests still pass.

**Make it fail on purpose**: restore the pairwise `later` — the mixed root returns
`simB-20200101000000000`, and the permutation test reports two different answers for the same set.

## 6. The freeze's machinery (US6, #106 landed, #104 in flight)

The compat gate's own suite, which `quick` runs on every pull request:

```bash
bash scripts/check-compat_test.sh
```

Expected: every case passes, including "an empty report exits 2 and says the gate did not run" and
"labelled breaking with an empty heading under [Unreleased] fails".

The toolchain — PR #110 merged on 2026-09-10 and this branch is rebased onto it:

```bash
grep -E '^(go|toolchain) ' go.mod
```

Expected: `go 1.25` and `toolchain go1.26.8`. In CI, the `verify` job's setup step reports the
toolchain version; read it with `gh run view <id> --log` (through `rtk proxy`, whose filter mangles
that output otherwise) and confirm it names a supported release.

## 7. Nothing else moved

```bash
go test -race -shuffle=on ./...
```

```bash
go test -tags=integration -count=1 -skip 'PeakMemory$' ./...
```

Expected: every corpus entry decodes byte-for-byte to its recorded stream, chunked and whole-file
reads agree, the codec agreement table is unchanged, and the canary (when Gatling is present) holds
each run to its own report. Then the gates the constitution names:

```bash
test -z "$(gofmt -l .)" && go vet ./... && go mod tidy && git diff --exit-code
```

```bash
golangci-lint run ./...   # v2.12.2, the pinned version in verify.yml
```

Coverage floors — 90% for `gatling/*`, 80% overall — are re-measured for the pull request
description:

```bash
go test -tags=integration -count=1 -skip 'PeakMemory$' -coverpkg=./... -coverprofile=cover.out ./... && bash scripts/check-coverage.sh --enforce cover.out
```

And the benchmark the constitution asks a decoder change to keep, before and after each binary
commit:

```bash
go test -tags=integration -run '^$' -bench 'BenchmarkDecode$' -benchmem -count=5 ./gatling/binary/
```

Expected: the same throughput within noise; a regression is justified in the pull request or fixed.
