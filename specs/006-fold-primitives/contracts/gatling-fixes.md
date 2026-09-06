# Contract: the three fixes to what v0.0.5 shipped

**Feature**: `006-fold-primitives` | **Issues**: [#56](https://github.com/galax-io/parsec/issues/56), [#57](https://github.com/galax-io/parsec/issues/57), [#55](https://github.com/galax-io/parsec/issues/55) | **Spec**: [spec.md](../spec.md)

None of these adds an exported identifier except `gatling.AbsentTimestamp`, which
[model-fold.md §3](./model-fold.md) carries. What they change is observable behaviour, listed here
so a reviewer can hold the diff to it.

## 1. `gatling/text` gives the binary codec's answer to malformed values (#56) — ask first

| Input, on any event record | `gatling/text` today | `gatling/binary` today | Both, after |
|---|---|---|---|
| a time (start, end, user event time, error time) written as `-` and digits | `*gatling.SyntaxError`, read ends | `AbsentTimestamp` on the record; model time zero (after §4 of model-fold) | the binary behaviour |
| a group's cumulated response time written as `-` and digits | `*gatling.SyntaxError`, read ends | the negative value on the record; model `CumulatedDuration` unset | the binary behaviour |
| a record with no groups, first or later | empty, non-nil `Groups` (verified on `main`) | empty, non-nil `Groups` | unchanged; pinned |
| a time or cumulated value that is not digits, or overflows | `*gatling.SyntaxError` | not expressible | unchanged — outside the spec's scope |

**Observable change**: two refusals become values. A text log that failed to read on `main`
because of a negative field now reads to the end with that field absent. Recorded under Changed.
**Approved by the maintainer on 2026-09-06.**

**Tests**:

- `gatling/binary`: `TestCodecsAgreeOnMalformedInput` — table over the three inputs; each case is
  one binary log from `builder` and one text log as a literal, read through both `NewRunReader`s;
  the items (or the error kinds) must be equal. Fails on `main` for the first two rows.
- `gatling/text`: a table test over the negative-field cases asserting `AbsentTimestamp` and the
  verbatim negative cumulated value, and one asserting non-digit and overflowing values still fail
  with the line number.
- `internal/wire`: `Millis(AbsentTimestamp).IsZero()`, `Millis(0)` is 1970, `span` with an absent
  start is unset.
- The corpus goldens, unchanged: `go test -tags=integration -run TestGolden ./gatling/...`.

## 2. `gatling/binary` gives each untrusted count its own ceiling (#57)

| Count | Ceiling today | Ceiling after |
|---|---|---|
| group nesting depth on a request or group record | `maxGroupDepth` = 1024 | unchanged |
| scenario names in the run record | `maxGroupDepth` = 1024 | `maxScenarios` = 65,536 |
| assertion payloads in the run record | `maxGroupDepth` = 1024 | `maxAssertions` = 65,536 |

**Observable change**: a run record declaring between 1,025 and 65,536 scenarios or assertions
decodes; above that it is refused at the count's offset with the existing message. Below 1,025
nothing changes. Recorded under Fixed. Unexported constants; no API change.

**Tests**:

- `TestALargeAssertionSuiteDecodes`: a `builder` run record with 2,000 one-byte payloads decodes
  and `Assertions()` returns 2,000 (FR-027, SC-008).
- `TestAScenarioListAboveTheGroupDepthDecodes`: 1,025 scenario names decode.
- `TestAnAbsurdGroupDepthIsRefused` (exists): a depth of 1 << 20 still fails naming the offset.
- `TestACorruptCountIsRefusedBeforeAllocating`: a count of `math.MaxInt32` for each of the three is
  refused at its offset, and the test's allocation measurement shows nothing sized by it.

## 3. The README says what the release does (#55)

| Where | Today | After |
|---|---|---|
| `README.md`, package list | no `gatling/binary/` row | a row: `the binary simulation.log codec for 3.13.1 through 3.15.1 (v0.0.5)` |
| `README.md`, Status | "A binary simulation.log — every Gatling from 3.13.0 — is refused with an error naming the format and saying no codec reads it yet" | "A binary simulation.log — every Gatling from 3.13.0 — is read over 3.13.1 through 3.15.1, the range its corpus covers, through the same entry point" |
| `README.md`, Status | "Every source in the table above except the Gatling text log is unimplemented." | "Every source in the table above except Gatling is unimplemented." |
| `README.md`, Status | "the binary codec will share them" | "both codecs share them" |
| `gatling/format.go:53` | "The recording under testdata/samples shows what a real one opens with" | "The 3.15.1 recording (testdata/corpus/gatling/3.15.1/RECORDING.md) shows what a real one opens with" |
| `gatling/format_test.go:28` | "see testdata/samples/gatling/binary/SAMPLE.md" | "see testdata/corpus/gatling/3.15.1/RECORDING.md" |
| `testdata/corpus/gatling/3.15.1/RECORDING.md` | no mention of the opening bytes | one line: the log opens `00 \| 00 00 00 06 \| "3.15.1"` — kind byte, length, release string |

**Not changed**: `Detect` and the six-byte rule (FR-031); `doc.go`, which is already correct and is
what the README is brought into agreement with.

**Checks**: `grep -rn testdata/samples --include='*.go' .` is empty; the README's package list and
`doc.go`'s package list name the same five packages with the same ranges.

## 4. `CHANGELOG.md` — drafted entries

```markdown
### Changed

- `gatling/text` no longer refuses a log for a negative time or a negative cumulated response time.
  It reports the time absent and the duration unset, which is what `gatling/binary` already did and
  what spec 005 requires of both: one bad field does not end a ten-million-record read, and the two
  codecs now give the same answer to the same malformed input. A test drives both with the same
  inputs so a third answer cannot appear unnoticed. (#56)

### Fixed

- `gatling/binary` refused a valid run declaring more than 1,024 assertions or scenarios as
  malformed, because the group-nesting ceiling was bounding two counts it was never meant to. Each
  count now has its own ceiling, named for what it bounds; a run with two thousand assertions
  decodes, and a corrupt count is still stopped before it sizes anything. (#57)
- The README still described a binary `simulation.log` as refused for want of a codec, and two
  comments justified the binary detection rule by a sample directory v0.0.5 deleted. The README now
  matches the package documentation, and the rule's provenance is the 3.15.1 recording. (#55)
```

## 5. Commits

One tracked issue per commit, each green on its own, in this order — #56 first because #8 relies on
the zero-time convention it introduces:

1. `fix(gatling): give both codecs one answer to malformed input (#56)`
2. `fix(gatling): give each untrusted count its own ceiling (#57)`
3. `feat(model): the position and bounds a consumer folds (#8)`
4. `docs(readme): describe the binary codec v0.0.5 shipped (#55)`

Each fixes and closes its issue when it lands on `main`. The four ship in one pull request,
[#64](https://github.com/galax-io/parsec/pull/64), which carries milestone v0.0.6.
