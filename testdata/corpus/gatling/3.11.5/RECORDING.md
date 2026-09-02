# Recording: Gatling 3.11.5

Recorded 2026-09-03 from a real run of `testdata/corpus/gatling/simulation/`
(`io.galaxio.parsec.corpus.CorpusSimulation`) against the stub in `simulation/stub`.
The three files beside this note are exactly as Gatling wrote them; none has been edited.

| Fact | Value |
|---|---|
| Gatling version (from the RUN record) | 3.11.5 |
| Build | `sbt -Dgatling.version=3.11.5 "Gatling/testOnly io.galaxio.parsec.corpus.CorpusSimulation"` |
| Machine | macOS (Darwin 25.6.0), arm64 |
| JVM | Homebrew OpenJDK 17.0.10 |
| Line separator | LF — `System.lineSeparator` on macOS; the log contains no carriage return |
| Charset | Gatling default, UTF-8 |
| Run description | left empty, so the RUN record carries a lone space in that field |

## What the run exercised

- 3 declared assertions → 3 `ASSERTION` records ahead of the `RUN` header
- 6 virtual users (`atOnceUsers(2)` + `rampUsers(4).during(2s)`) → 12 `USER` records
- 36 `REQUEST` records: 18 OK, 18 KO; one request outside any group, the rest under `outer` and `outer / inner, with comma`
- 12 `GROUP` records (two groups per user), all KO because the inner group holds a failing check
- 6 `ERROR` records — one per user — from a request whose URL could not be built
  (`No attribute named 'undefinedAttribute' is defined`). Every message ends in a trailing space,
  which is how Gatling assembles it; it is preserved.

## What was checked at capture time

- `global_stats.json` carries `numberOfRequests` and `meanNumberOfRequestsPerSecond`, each split
  total/ok/ko, as raw JSON numbers.
- `stats.json` carries the same per request and per group as a tree under `contents`, with raw
  JSON numbers as well.
- Neither file carries a count of virtual users or of `ERROR` records — there is no key naming
  users, sessions or errors. Those counts are pinned by `records.golden` instead.
- The group name `inner, with comma` was written as `inner  with comma`: the comma became a
  space, so every comma in a group path is a separator.
- A two-line error message was attempted (connection refused, unknown host). Neither produced an
  `ERROR` record at all — a failed connection is recorded as a KO request, not as a crash — and the
  one crash that did occur produced a single-line message. No `ERROR` message in this log contains
  a tab or a line break.
- No request carries the never-completed end sentinel.
