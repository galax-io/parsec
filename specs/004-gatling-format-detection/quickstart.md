# Quickstart: validating format detection and the version policy

**Feature**: `004-gatling-format-detection` | **Date**: 2026-09-05 | **Spec**: [spec.md](./spec.md)

Every scenario below is runnable and each maps to a success criterion. Details of the API live in
[contracts/gatling-detect.md](./contracts/gatling-detect.md); the types are in
[data-model.md](./data-model.md). Nothing here is implementation — it is how you tell, from outside,
whether the feature works.

## Prerequisites

- Go 1.25 (`go.mod` is authoritative)
- The repository's existing corpus: `testdata/corpus/gatling/3.11.5/` and `3.12.0/`
- For scenario 6 only: a JDK and `sbt`, to capture the binary sample

## The one command that must be green

```bash
go build ./... && go test -race -shuffle=on ./...
```

That is the definition of a green commit for this project. Everything below narrows it.

---

## Scenario 1 — a text log is identified as text, whatever it is called

**Proves**: SC-001, FR-001 … FR-003.

```bash
go test -race -run 'TestDetect' ./gatling/...
```

Expected: every corpus `simulation.log` classifies as `FormatText`, including under a renamed copy.
The check that matters most is the ordinary one — both corpus logs open with `ASSERTION\t`, not
`RUN\t`, so a detector keyed on the letter `R` fails here and must.

Quick manual confirmation that the premise still holds:

```bash
head -c 10 testdata/corpus/gatling/3.11.5/simulation.log | od -c | head -1
```

Expected: `A S S E R T I O N \t`.

---

## Scenario 2 — a binary log gets an honest refusal, not a syntax error

**Proves**: SC-002, FR-009, FR-010.

```bash
go test -race -run 'TestNewReader_Binary|TestUnsupportedFormat' ./gatling/simlog/
```

Expected: opening the binary sample through `simlog` fails with a `*gatling.UnsupportedFormatError`
naming the binary format. It must **not** be a `*gatling.SyntaxError` about line 1, which is what
handing the same bytes to `text.NewReader` produces — the test asserts both, side by side, because
the difference is the point of the milestone.

---

## Scenario 3 — the version policy, all six outcomes

**Proves**: SC-003, SC-004, SC-005, FR-013 … FR-022.

```bash
go test -race -run 'TestPolicy|TestGate|TestStrict' ./gatling/ ./gatling/text/ ./gatling/simlog/
```

Expected, for a header naming each version:

| Version | Lenient | Strict |
|---|---|---|
| `3.10.5` | refused, `*VersionError`, names `3.10.5` and `3.11.5` | same |
| `3.11.5`, `3.12.0` | decoded, no warning | identical records |
| `3.99.0` | decoded, exactly **one** warning | refused, `*UnverifiedError` |
| `3.13.0-SNAPSHOT` | refused, `*VersionError`, quotes the string | same |

The warning count is asserted as a number, not as "at least one": FR-016 says exactly one reaches
the caller however many layers handled the log, so the test reads it through `text.NewReader`,
through `text.NewRunReader` and through both `simlog` constructors and expects `1` from each.

---

## Scenario 4 — dispatched reads and direct reads agree exactly

**Proves**: SC-009, FR-011.

```bash
go test -race -run 'TestSimlogMatchesCodec' ./gatling/simlog/
go test -tags=integration -race -run 'TestGolden' ./...
```

Expected: over every corpus log, the record stream from `simlog.NewReader` is identical field for
field to the one from `text.NewReader`, and the item stream from `simlog.NewRunReader` matches
`text.NewRunReader`. Zero differences, not "close".

---

## Scenario 5 — the stream survives detection, in one pass and in chunks

**Proves**: FR-004, FR-005, SC-008, and Principle II's chunked-equals-whole-file rule.

```bash
go test -race -run 'TestChunked|TestDetectWindow' ./gatling/simlog/
```

Expected: reading a corpus log through `simlog` in arbitrary chunk sizes — including a one-byte-at-a-
time reader, which is what makes a swallowed head visible — produces the same records as reading it
in one pass, and the same failure at the same line for a log that fails. The window test asserts that
no more than `DetectSize` bytes are consumed before the codec sees the stream, and that the figure
does not move when the input is made a thousand times larger.

---

## Scenario 6 — capturing the binary sample (once)

**Proves**: FR-031, FR-031a. Run this only when creating the sample; afterwards it is committed.

**Not through the existing probe.** `gatling-picatinny` has no release targeting the 3.14.x or
3.15.x line (research R10), and the probe pins it to render its OpenNFR requirements into
assertions, so the probe cannot run under 3.15.1 at all. Use a throwaway minimal simulation instead:
no picatinny, one request, and a `gatling-sbt` from the 3.15.x column. The sample needs a real
binary log, not this project's probe in particular.

```bash
sbt "Gatling/testOnly *"   # in the throwaway project, Gatling 3.15.1
```

Then take the head of the log the run wrote:

```bash
head -c 256 target/gatling/*/simulation.log > testdata/samples/gatling/binary/3.15.1-head.bin
```

Expected: the first byte is `0x00`. **If it is not, the spec is wrong and the recording wins** —
research R4 says so explicitly; correct `Detect` and the spec rather than the sample.

Write `testdata/samples/gatling/binary/SAMPLE.md` beside it with the release, the machine, the JVM
and the exact command, the way each corpus `RECORDING.md` does. Say in the first line that this is a
**sample, not a corpus entry**: it holds no complete run and no report, and nothing may compare a
decoder against it.

Widening the probe itself to 3.15.x is out of scope here, and may not be possible while picatinny
has no release on those lines — that belongs to v0.0.5 with the corpus it records.

---

## Scenario 7 — nothing crashes, on anything

**Proves**: SC-007, FR-028.

```bash
go test -race -run 'TestDetectFuzz|FuzzDetect' ./gatling/
go test -race -run 'TestRobustness' ./gatling/simlog/
```

Expected: at least 10,000 mutated, truncated and arbitrary inputs produce a classification or an
error, never a panic. A reader that fails mid-head must surface that failure as its own error, not
as a misclassification.

---

## Scenario 8 — a consumer can report what parsec reads

**Proves**: SC-006, FR-023 … FR-026.

```bash
go test -race -run 'TestSupported' ./gatling/simlog/
```

Expected: `Supported()` returns the text format as readable over 3.11.5 … 3.12.0 and the binary
format as known-but-not-readable. The test derives the expected range from
`text.SupportedVersions()` rather than restating it, so widening the corpus without widening the
range fails, and so does the reverse.

---

## Scenario 9 — the text codec did not change under anyone

**Proves**: SC-010, FR-032.

```bash
go test -race -shuffle=on ./gatling/text/
```

Expected: every test that passed before this feature passes after it, unchanged. If a test in
`gatling/text/` had to be edited to keep passing, the change was not the refactor it claimed to be.

---

## Scenario 10 — the cost is constant

**Proves**: SC-008, and the constitution's benchmark requirement.

```bash
go test -run '^$' -bench 'BenchmarkDetect|BenchmarkOpen' -benchmem ./gatling/ ./gatling/simlog/
```

Expected: `Detect` allocates nothing and does not vary with input size; opening through `simlog`
costs at most one extra allocation over opening the codec directly, and throughput over the largest
corpus log is indistinguishable from the direct path.

Benchmarks use `b.Loop()` and `b.ReportAllocs()`, which is what the repository's existing ones
already do. Compare against the direct path with `benchstat` rather than by eye, and paste its
output into the PR body — that is how the constitution's "a regression against the recorded number
MUST be justified in the PR" is satisfied:

```bash
go test -run '^$' -bench 'BenchmarkOpen' -benchmem -count=10 ./gatling/simlog/ > new.txt && go run golang.org/x/perf/cmd/benchstat@latest new.txt
```

---

## Scenario 11 — no typed nil hiding inside a non-nil interface

**Proves**: FR-028, and the hazard research R13 flagged.

```bash
go test -race -run 'TestNilOnError' ./gatling/simlog/
```

Expected: every error path of `NewReader` and `NewRunReader` returns a reader that is `== nil` when
compared against the interface. Returning a typed nil `*text.Reader` would produce an interface that
is *not* nil, so a caller's `if rd != nil` would pass and the next call would panic. The test
asserts the comparison directly for each error path — unknown format, unsupported format, version
refused, strict refusal, failing stream.

---

## Scenario 12 — the package reads well on pkg.go.dev

**Proves**: Principle V's doc-comment requirement, and the library convention that a package ships a
runnable example.

```bash
go test -race -run 'Example' ./gatling/simlog/
go doc -all ./gatling/simlog | head -40
```

Expected: `ExampleNewRunReader` compiles, runs and matches its `// Output:` block, the way
`gatling/text` and `model` already do. Every exported identifier in `gatling/simlog` and every one
added to `gatling` has a doc comment that says what it does and, where it gates, which versions it
accepts.

---

## Coverage

```bash
go test -tags=integration -count=1 -skip 'PeakMemory$' -coverpkg=./... -coverprofile=cover.out ./...
bash scripts/check-coverage.sh --enforce cover.out
```

Expected: `gatling/` and `gatling/simlog/` at 90% or above — `scripts/check-coverage.sh` maps
`*/gatling/*` to the decoder floor, so the new package is held to it from its first commit — and the
module at 80% or above.
