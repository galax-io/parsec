# Recording: Gatling 3.13.1

Recorded 2026-09-06 from a real run of `testdata/corpus/gatling/simulation/`
(`io.galaxio.parsec.corpus.CorpusSimulation`) against the stub in `simulation/stub`.
Everything beside this note is exactly as Gatling wrote it; nothing has been edited.

| Fact | Value |
|---|---|
| Gatling version (from the RUN record) | 3.13.1 |
| Build | `sbt -Dgatling.version=3.13.1 "Gatling/testOnly io.galaxio.parsec.corpus.CorpusSimulation"` |
| Machine | macOS (Darwin 25.6.0), arm64 |
| JVM | Homebrew OpenJDK 17.0.10 |
| Log format | binary — the first version in the corpus that is not tab-separated text |
| `simulation.log` | 3965 bytes |
| Charset | Gatling default, UTF-8 |
| Run description | left empty, so the RUN record's description is a zero-length string |

## Why 3.13.1 and not 3.13.0

3.13.0 is the first Gatling to write a binary log, and it is deliberately **not** in the corpus.
It cannot read back the assertion records it writes: `IllegalArgumentException: Unknown object
coding: 1` out of boopickle in `AssertionPicklers`, raised while `FirstPassParser` parses the run
record. No report is generated and the run fails. The probe was run against it with ten assertions
and with one, and both failed identically, so this is not a property of what the probe asserts.

The record framing it writes is sound — a 3.13.0 `simulation.log` parses cleanly to its last byte
against the grammar in `specs/005-gatling-binary-decoder/data-model.md`, with the same record counts
as this one. What differs is the assertion payload: byte 1 of the first blob is `03` there and `01`
here. parsec carries those payloads through without decoding them, so a 3.13.0 log is readable by
this library; it simply cannot be a corpus entry, because a run that produces no report has no
second, independent account of its own numbers.

## What the run exercised

- 10 declared assertions → 10 assertion payloads in the RUN record (15, 15, 16, 25, 38, 31, 51, 51,
  33 and 30 bytes), all evaluated true by Gatling itself
- 6 virtual users (`atOnceUsers(2)` + `rampUsers(4).during(2s)`) → 12 `USER` records
- 102 `REQUEST` records: 84 OK, 18 KO; one request outside any group, the rest under `outer` and
  `outer / inner, with comma`
- 12 `GROUP` records (two per user)
- 6 `ERROR` records — one per user — from a request whose URL could not be built
- 14 distinct strings in the writer's cache, for 102 requests: `GET /ok` is written once and referred
  to by index 65 more times

Three of those cases exist for the binary format specifically:

- **`Проверка /ok`** is Cyrillic, so it cannot be encoded in Latin-1 and Gatling stores it as UTF-16
  with the other coder byte. It is the only thing in this corpus that exercises that path.
- **`GET /ok` repeated ten times inside `outer`** makes the string cache a table rather than a list.
- **`inner, with comma`** keeps its comma, which the binary format preserves — see below.

## What was checked at capture time

- `js/global_stats.json` and `js/stats.json` are **present**. Gatling still writes them on the 3.13.x
  line; it stops at 3.14.0. So this entry carries the same machine-readable statistics the text
  corpus relies on, and the HTML report and console summary beside them are a third and fourth
  account rather than a replacement.
- `js/assertions.xml` is present as well, and records all ten assertions as passing.
- `global_stats.json` states `numberOfRequests` as total 102, ok 84, ko 18 — the same numbers the
  decoded record stream must yield.
- **The comma in `inner, with comma` survives.** The text corpus records it as `inner  with comma`,
  with the comma replaced by a space, because a text log separates a group path with commas. The
  binary format length-prefixes each group name, so nothing is substituted. Both spellings are
  correct for their own format; neither is a typo.
- The console summary's per-request block spells counts as `102 (OK=84 KO=18)`. From 3.14.0 the same
  block is a table with `|` separators. Anything extracting numbers from `console.txt` has to handle
  both.
- Neither the report nor the console carries a count of virtual users or of `ERROR` records. Those
  counts are pinned by the decoded record stream instead.

## What is kept, and what is not

`simulation.log`, `index.html`, `console.txt` — the complete captured standard output of the run,
sbt's own lines included, because that is what was on the terminal — and `records.golden`, the
decoded record stream this recording is compared against.

**What is not kept**: the vendored JavaScript and stylesheets the report needs only to render, and
Gatling's own logos. They carry nothing about this run, are byte-identical in every run of the same
version, and would be a megabyte of noise per entry. Every file that holds a number, a name or a
timestamp from the run is kept whole and unedited — extraction belongs in the verification suite, so
a later reader can check what the run actually said rather than trusting what was recorded about it.

The consequence is that `index.html` no longer renders as a chart page. Its statistics files are kept beside it —
`js/global_stats.json`, `js/stats.json`, `js/stats.js`, `js/all_sessions.js` and `js/assertions.xml`
— and those hold every figure the page would have drawn, in a form that reads better than the page.

The console summary exists only because it was redirected at run time, and nothing here can be
recovered from a run that has already finished.
