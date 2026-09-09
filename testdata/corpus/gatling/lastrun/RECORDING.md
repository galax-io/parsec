# Recording: a Maven results root, with `lastRun.txt`

Recorded 2026-09-09 from three real runs of `testdata/corpus/gatling/simulation/`
(`io.galaxio.parsec.corpus.CorpusSimulation`) against the stub in `simulation/stub`, driven by
**Maven** rather than sbt. Everything under `results/` is exactly as Gatling and the plugin wrote it;
nothing has been edited.

This is **not a decoder corpus entry.** The logs here are Gatling 3.15.1, which
`testdata/corpus/gatling/3.15.1/` already covers with its own report and `records.golden`. What is
recorded here is the *results root* — the directory layout a build tool leaves behind, and the
`lastRun.txt` inside it — which is what `gatling.FindRun` reads. See
`specs/009-gatling-run-discovery/`.

| Fact | Value |
|---|---|
| Gatling version | 3.15.1 |
| Build tool | Maven 3.9.15, `gatling-maven-plugin` **4.21.10** |
| Scala | 2.13.18, via `scala-maven-plugin` 4.9.5 |
| Machine | macOS (Darwin 25.6.0), arm64 |
| JVM | Homebrew OpenJDK 17.0.10 (`JAVA_HOME` pinned; Maven's own default here is 26.0.1) |
| Results root | `target/gatling` — the plugin's `resultsFolder` default, `${project.build.directory}/gatling` |
| `lastRun.txt` | 35 bytes, one line, LF |

## Why it took three runs

`lastRun.txt` is written by `GatlingMojo.saveSimulationResultToFile`, and `execute()` calls it **only
when `failOnError` is false**. That parameter defaults to `true`, so an ordinary `mvn gatling:test`
writes no such file — which run 1 below confirms directly, and which is the sharpest limit on how
often this file exists at all.

| Run | Command | Effect |
|---|---|---|
| 1 | `mvn -B test-compile gatling:test -Dgatling.simulationClass=…` | created `corpussimulation-20260909022556930`; **no** `lastRun.txt` |
| 2 | same, plus `-Dgatling.failOnError=false` | created `corpussimulation-20260909022708912`; wrote `lastRun.txt` naming it |
| 3 | `mvn -B gatling:test -Dgatling.simulationClass=…` | created `corpussimulation-20260909022727230`; left `lastRun.txt` untouched |

The result is the shape [parsec#11](https://github.com/galax-io/parsec/issues/11)'s acceptance case
asks for, produced rather than constructed: **a results root with three runs, whose `lastRun.txt`
names the middle one.** Run 3 does not rewrite the file because the plugin writes only the
directories that appeared during *its own* execution, and it writes nothing at all when `failOnError`
is left at its default.

`gatling:verify` was deliberately never run: `VerifyMojo.verifyLastRun` reads the file and then
**deletes** it, so an ordinary `mvn gatling:test gatling:verify` would have left nothing to record.

## What `lastRun.txt` holds

```text
corpussimulation-20260909022708912
```

35 bytes: the bare directory name and a single LF. Not a path, not absolute, no separator — which is
what `GatlingMojo` writes, one `File.getName()` per run directory the execution created, joined with
`System.lineSeparator()` (so CRLF on Windows, LF here). A run that failed would append a further line
carrying an error message rather than a name; that shape is not in this recording, because all three
runs passed their assertions.

## What this recording settled

Two things planning had wrong, both now corrected in `specs/009-gatling-run-discovery/`:

1. **`lastRun.txt` needs `failOnError=false`.** Research first recorded only that the file is
   Maven-only and deleted by `gatling:verify`. It is rarer still: the default configuration never
   writes one.
2. **A run directory is named in UTC, not local time.** `corpussimulation-20260909022708912` is the
   run's start as `yyyyMMddHHmmssSSS` in **UTC** — the machine was at +04:00, and the run's own RUN
   record decodes to `20260909022708.912` UTC / `20260909062708.912` local. The same holds for the
   sbt-made entries: `testdata/corpus/gatling/3.13.1/` decodes to `20260906044741.110` UTC and its
   console names `corpussimulation-20260906044741110`. So this is Gatling's behaviour and not the
   Maven plugin's, and the ordering rule's tie-break on the name is a UTC ordering.

## What was kept, and what was not

`results/` holds, per run, exactly what the sibling corpus entries hold: `simulation.log` and
`index.html`, byte for byte as produced. The report's `js/` and `style/` asset trees and its
per-request HTML pages were **not** copied — the same choice every existing entry under
`testdata/corpus/gatling/` already makes, and they are 2.1 MB of Gatling's own static assets
duplicated three times. `records.golden` is absent on purpose: this is not a decoder entry, and
3.15.1's own entry carries that evidence.

`console.txt` is the full Maven output of all three runs, including run 1's dependency resolution,
with a header naming the exact command each time.

## Reproducing it

```sh
cd testdata/corpus/gatling/simulation
go run ./stub &                       # the endpoint the simulation talks to, on :8089
export JAVA_HOME=$(/usr/libexec/java_home -v 17)
mvn -B test-compile gatling:test -Dgatling.simulationClass=io.galaxio.parsec.corpus.CorpusSimulation
mvn -B gatling:test -Dgatling.simulationClass=io.galaxio.parsec.corpus.CorpusSimulation -Dgatling.failOnError=false
mvn -B gatling:test -Dgatling.simulationClass=io.galaxio.parsec.corpus.CorpusSimulation
```

The run directory names will differ — they carry the run's start — so a test must read them from the
directory rather than hard-code them. `pom.xml` takes `-Dgatling.version` the same way `build.sbt`
takes it.
