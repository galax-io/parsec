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

   Half the requests fail by design; every declared assertion still holds, so the run exits zero.
   The log and report land in `target/gatling/corpussimulation-<timestamp>/`.

3. Copy, unmodified, into `../<version>/`: `simulation.log`, `js/global_stats.json`, `js/stats.json`.
   Then write `RECORDING.md` there with what `specs/002-gatling-text-decoder/tasks.md` T004 asks
   for. Nothing can be added after the run is archived.
