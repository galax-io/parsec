# Contract 3 — the `record-corpus` workflow

**Status**: new. The single dispatch that replaces the nine-step manual procedure (spec FR-018).

## Interface

```yaml
on:
  workflow_dispatch:
    inputs:
      version:                  # required — the Gatling version to record, e.g. "3.15.1"
      description:              # optional — recorded in the note scaffold
```

**Output**: one downloadable artefact, `corpus-entry-<version>`, whose contents are placed under
`testdata/corpus/gatling/<version>/` by the maintainer. **The job never commits** (spec FR-019); the
workflow requests no write permission.

## What the job does

1. Build and start the stub the probe talks to; wait for its port.
2. `sbt -batch -Dgatling.version=<version> "Gatling/testOnly io.galaxio.parsec.corpus.CorpusSimulation"`,
   capturing standard output to `console.txt`. The build selects the OpenNFR or plain-DSL assertion
   flavour from the version itself — no input says which.
3. Fail if the run failed, or if it produced no report directory. A run that misses the probe's
   declared expectations, or that cannot read back what it wrote, is not an entry (spec FR-022).
4. Select the entry's files **by presence, not by a version table** (research R7).
5. Generate `records.golden` by running the matching codec's golden test with `-update`.
6. Write a `RECORDING.md` **scaffold** stating the platform, the version, the build command and the
   artefacts kept — plainly marked incomplete.
7. Upload the assembled directory.

## File selection

| Kept when present | Always dropped |
|---|---|
| `simulation.log` | `js/highstock.js`, `js/jquery-*.js` |
| `index.html` | `js/bootstrap.min.js`, `js/highcharts-more.js` |
| `js/global_stats.json`, `js/stats.json` | the rest of `js/` |
| `js/stats.js`, `js/all_sessions.js`, `js/assertions.xml` | all of `style/`, all logos |
| `console.txt` (the redirected run output) | |

Presence-driven, so 3.12.0 keeps its JSON, 3.13.1 keeps JSON and HTML, and 3.15.1 keeps HTML and
console — with no version conditional in the script. The dropped files carry nothing about the run,
are byte-identical across runs of a version, and come to about a megabyte per entry; keeping them
would breach the corpus size ceiling (spec FR-024, SC-005).

## What the job cannot do, and says so

`RECORDING.md` proper is written by the recorder. It states what was checked at capture time, which
absences are the tool's own, and what a later reader should not trust — judgements a job cannot make.
The scaffold carries the mechanical facts and a heading list; the recorder fills it in. Nothing the
job publishes can replace an existing entry's note (spec FR-023).

## Failure modes

| Situation | Job outcome |
|---|---|
| the probe fails its declared expectations | fails, publishes nothing |
| Gatling writes a log it cannot read back (3.13.0) | fails — no report directory, so no entry |
| the version is below or above what the module gates | the run still happens; the golden step reports the gate's verdict, and an above-range version is flagged as a candidate for widening |
| the stub does not come up | fails within the existing 30 s wait |

## Provenance

Entries produced here are recorded on `ubuntu-latest`. The five existing entries were recorded on
macOS/arm64 and say so. Mixed provenance is fine **because every entry states its own** — which is
why the scaffold carries the platform line and why it is not optional.
