# Recording: Gatling 3.14.9

Recorded 2026-09-06 from a real run of `testdata/corpus/gatling/simulation/`
(`io.galaxio.parsec.corpus.CorpusSimulation`) against the stub in `simulation/stub`.
Everything beside this note is exactly as Gatling wrote it; nothing has been edited.

| Fact | Value |
|---|---|
| Gatling version (from the RUN record) | 3.14.9 |
| Build | `sbt -Dgatling.version=3.14.9 "Gatling/testOnly io.galaxio.parsec.corpus.CorpusSimulation"` |
| Machine | macOS (Darwin 25.6.0), arm64 |
| JVM | Homebrew OpenJDK 17.0.10 |
| Log format | binary |
| `simulation.log` | 3952 bytes |
| Charset | Gatling default, UTF-8 |
| Run description | left empty, so the RUN record's description is a zero-length string |

## What the run exercised

The same simulation as every other entry in this corpus, unchanged — that is what lets the runs be
compared against each other with timing, identity and order set aside.

- 10 declared assertions → 10 assertion payloads in the RUN record (15, 15, 16, 25, 38, 31, 51, 51,
  33 and 30 bytes), all evaluated true by Gatling itself
- 6 virtual users (`atOnceUsers(2)` + `rampUsers(4).during(2s)`) → 12 `USER` records
- 102 `REQUEST` records: 84 OK, 18 KO; one request outside any group, the rest under `outer` and
  `outer / inner, with comma`
- 12 `GROUP` records (two per user)
- 6 `ERROR` records — one per user — from a request whose URL could not be built
- 14 distinct strings in the writer's cache, for 102 requests: `GET /ok` is written once and referred
  to by index 65 more times
- `Проверка /ok` is Cyrillic, so it is stored as UTF-16 with the other coder byte
- `inner, with comma` keeps its comma: the binary format length-prefixes each group name, so nothing
  is substituted. The text corpus records the same group as `inner  with comma`, because a text log
  separates a group path with commas. Both spellings are correct for their own format.

## What was checked at capture time

- **Gatling writes no `global_stats.json` and no `stats.json` here.** It stopped at 3.14.0; the
  3.13.1 entry in this corpus still has them. The generated HTML report and the redirected console
  summary are what replace them, and both were captured at run time — the report is a file in the
  run directory and survives archiving, the console summary is standard output and exists only
  because it was redirected. This is the constitution's Principle III exemption, and the absence is
  Gatling's, confirmed on this run.
- `index.html` carries the run total and a per-request row with each figure in its own classed cell,
  with the numbers baked into the markup. On the 3.13.x line the same table is filled in by
  JavaScript at page load instead, so anything extracting from a report has to handle both shapes.
- The console summary's Global Information block states `request count` as total 102, OK 84, KO 18 —
  the same numbers the decoded record stream must yield. From 3.14.0 that block is a table with
  `|` separators; on 3.13.x it is `102 (OK=84 KO=18)`.
- Neither the report nor the console carries a count of virtual users or of `ERROR` records. Those
  counts are pinned by the decoded record stream instead.
- The check failure message reads `status.find.is(200), found 500`. On 3.13.1 the same check reads
  `status.find.is(200), but actually found 500`, 13 bytes longer, which is the whole difference in
  file size between that entry and this one. A cross-version comparison has to set message text
  aside.

## What is kept, and what is not

`simulation.log`, `index.html`, `console.txt` — the complete captured standard output of the run,
sbt's own lines included, because that is what was on the terminal — and `records.golden`, the
decoded record stream this recording is compared against.

**What is not kept**: the vendored JavaScript and stylesheets the report needs only to render, and
Gatling's own logos. They carry nothing about this run, are byte-identical in every run of the same
version, and would be a megabyte of noise per entry. Every file that holds a number, a name or a
timestamp from the run is kept whole and unedited — extraction belongs in the verification suite, so
a later reader can check what the run actually said rather than trusting what was recorded about it.

The consequence is that `index.html` no longer renders as a chart page. Its tables are baked into the markup from 3.14.0 on, so
opening the file still shows every figure this run reported; only the charts are gone.

The console summary exists only because it was redirected at run time, and nothing here can be
recovered from a run that has already finished.
