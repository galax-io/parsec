# Corpus simulation

The sample simulation the golden corpus under `../3.11.5/` and `../3.12.0/` was recorded from.
Scaffolded with `galaxio template init gatling/scala-sbt`, then reduced to plain Gatling DSL so
the only version-bearing dependency is Gatling itself.

This directory is **not** a corpus entry: it has no `simulation.log`. It exists so a recording can
be reproduced for a future version, and so a reader of the corpus can see exactly what produced it.

## Record a version

1. Start the stub the simulation talks to (from the repository root):

   ```sh
   go run ./testdata/corpus/gatling/simulation/stub
   ```

2. Run the simulation under the version to record (the property selects the Gatling artefacts):

   ```sh
   cd testdata/corpus/gatling/simulation
   sbt -Dgatling.version=3.11.5 "Gatling/testOnly io.galaxio.parsec.corpus.CorpusSimulation"
   sbt -Dgatling.version=3.12.0 "Gatling/testOnly io.galaxio.parsec.corpus.CorpusSimulation"
   ```

   Half the requests fail by design, and the requirements pin exactly which. A broken environment
   fails the run itself rather than producing a log that is merely consistent with its own report.
   The log and report land in `target/gatling/corpussimulation-<timestamp>/`.

3. Copy, unmodified, into `../<version>/`: `simulation.log`, `js/global_stats.json`, `js/stats.json`.
   Then write `RECORDING.md` there with what `specs/002-gatling-text-decoder/tasks.md` T004 asks
   for. Nothing can be added after the run is archived.

## Change what the probe must produce

Everything the run is held to lives in [`src/test/resources/nfr.yaml`](src/test/resources/nfr.yaml),
an OpenNFR `RequirementSet`. Edit that file and nothing else: the Gatling assertions are rendered
from it by `OpenNfrAssertions.fromYaml`, so no number is written twice and no Scala file carries one.

The document names no tool, which is the point — the same expectations can be held against a JMeter
or k6 run of this probe when those adapters land.

Two things in it look like typos and are not, and both say so where they appear:

- **`inner  with comma` carries two spaces.** The simulation declares the group as `inner, with
  comma`; Gatling replaces the comma with a space as it writes the log, and resolves assertion paths
  from what it wrote. OpenNFR defines a `loadtest.group.name` element as a literal *recorded* name,
  so the recorded spelling is what the document must carry. The group keeps its comma deliberately:
  it is what proves the decoder's split on comma is lossless.
- **"18 successes" is written as "36 recorded, 18 failed."** An OpenNFR selector matches presence and
  never absence, so a fraction can name what failed and cannot name what did not. It is the same
  statement about the same run.

A requirement the renderer cannot express refuses the **whole** document, naming every reason, so a
run can never quietly check fewer things than the document states. A requirement that is simply
false fails the run and names itself.

Verified under Gatling 3.11.5 and 3.12.0; see `specs/003-canonical-model/research.md` R1.
