# Corpus simulation

The sample simulation every golden corpus entry under `../<version>/` is recorded from. Scaffolded
with `galaxio template init gatling/scala-sbt`, then reduced to plain Gatling DSL so the only
version-bearing dependency is Gatling itself.

This directory is **not** a corpus entry: it has no `simulation.log`. It exists so a recording can
be reproduced for a future version, and so a reader of the corpus can see exactly what produced it.

## Record a version

1. Start the stub the simulation talks to (from the repository root):

   ```sh
   go run ./testdata/corpus/gatling/simulation/stub
   ```

2. Run the simulation under the version to record, **capturing standard output**. The property
   selects the Gatling artefacts; the redirection captures the console summary, which from 3.13.0
   is one of the only two accounts Gatling gives of its own numbers and exists nowhere on disk:

   ```sh
   cd testdata/corpus/gatling/simulation
   sbt -Dgatling.version=3.15.1 "Gatling/testOnly io.galaxio.parsec.corpus.CorpusSimulation" \
     2>&1 | tee /tmp/console.txt
   ```

   Half the requests fail by design, and the requirements pin exactly which. A broken environment
   fails the run itself rather than producing a log that is merely consistent with its own report.
   The log and report land in `target/gatling/corpussimulation-<timestamp>/`.

3. Copy into `../<version>/`, unmodified. **What to keep depends on the version**, because Gatling
   changed what it writes:

   | Version | Keep |
   |---|---|
   | up to 3.12.0 | `simulation.log`, `js/global_stats.json`, `js/stats.json` |
   | 3.13.x | `simulation.log`, `index.html`, `console.txt`, and `js/{global_stats.json,stats.json,stats.js,all_sessions.js,assertions.xml}` |
   | 3.14.0 and newer | `simulation.log`, `index.html`, `console.txt` |

   From 3.14.0 Gatling no longer writes `global_stats.json` or `stats.json` — the 3.13.x line still
   does. The HTML report and the console summary replace them as the run's own account of its
   numbers: the report carries the run total and a per-request row with each figure in its own
   classed cell, and the console summary carries the Global Information block.

   Keep the artefacts **as Gatling wrote them**, not a summary of them — extraction belongs in the
   verification suite, so a later reader can check what the run actually said rather than trusting
   what was recorded about it.

   **Do not keep what the report needs only to render**: `js/highstock.js`, `js/jquery-*.js`,
   `js/bootstrap.min.js`, `js/highcharts-more.js`, the rest of `js/`, and the whole of `style/`.
   Those files carry nothing about the run, are byte-identical in every run of that version, and come
   to about a megabyte an entry. The report will not draw its charts without them; from 3.14.0 the
   tables are in the markup, so every figure is still there, and on 3.13.x the JSON beside it holds
   them.

   Then write `RECORDING.md` there with what `specs/005-gatling-binary-decoder/tasks.md` T008–T010
   asks for. **If the probe has changed since an existing entry was recorded, say so in that
   entry's note** rather than re-recording it: the older recordings under `../3.11.5/` and
   `../3.12.0/` predate the v0.0.5 probe and do not satisfy the current `nfr.yaml`, and both say so. Nothing can be added after the run is archived: the console output exists only if it
   was redirected, and neither account can be recovered from a run that has already finished.

## Change what the probe must produce

**Up to Gatling 3.12.x**, everything the run is held to lives in
[`src/test/resources/nfr.yaml`](src/test/resources/nfr.yaml), an OpenNFR `RequirementSet`. Edit that
file and nothing else: the Gatling assertions are rendered from it by `OpenNfrAssertions.fromYaml`,
so no number is written twice and no Scala file carries one.

The document names no tool, which is the point — the same expectations can be held against a JMeter
or k6 run of this probe when those adapters land.

**From 3.13.0**, the versions that write a binary log state the same expectations in Gatling's own
assertion DSL, in [`src/test/scala-plain`](src/test/scala-plain). `build.sbt` selects one flavour or
the other by source directory; the simulation itself is one file for every version and carries no
number.

**The split is the log format, not the tool version.** An OpenNFR `loadtest.group.name` is the
literal *recorded* group name, and the two formats record `inner, with comma` differently — the text
log replaces the comma with a space, the binary log keeps it. One document cannot spell the group
both ways, so it serves the text versions and the DSL serves the binary ones. `gatling-picatinny` is
not the obstacle: 1.27.0 resolves and runs correctly on 3.13.1, verified directly. It simply has no
release above 3.13.x, so it could not have covered the binary range in any case.

**The two flavours must be kept in step by hand.** Changing an expectation means editing both
`nfr.yaml` and `CorpusAssertions.scala` under `scala-plain`, and each assertion there names the
requirement it mirrors so the pairing is visible. That cost is what
`specs/005-gatling-binary-decoder/research.md` R8 accepted when it split the two flavours.

Two things in `nfr.yaml` look like typos and are not, and both say so where they appear:

- **`inner  with comma` carries two spaces.** The simulation declares the group as `inner, with
  comma`; Gatling replaces the comma with a space as it writes the log, and resolves assertion paths
  from what it wrote. OpenNFR defines a `loadtest.group.name` element as a literal *recorded* name,
  so the recorded spelling is what the document must carry. The group keeps its comma deliberately:
  it is what proves the decoder's split on comma is lossless.
- **The successes are written as "102 recorded, 18 failed."** An OpenNFR selector matches presence
  and never absence, so a fraction can name what failed and cannot name what did not. It is the same
  statement about the same run.

A requirement the renderer cannot express refuses the **whole** document, naming every reason, so a
run can never quietly check fewer things than the document states. A requirement that is simply
false fails the run and names itself.

## What the run exercises, and why each case is there

Beyond every record kind, nested groups and both failure kinds, three cases exist for the binary
format specifically and none of them can be added to a run after it is made:

- **A request named in Cyrillic** (`Проверка /ok`), so at least one string in the log is stored as
  UTF-16 rather than Latin-1. Without it the corpus proves nothing about the half of the format that
  turns a wrong decoder into plausible mojibake instead of an error.
- **`GET /ok` repeated ten times inside the outer group**, so the name is written into the log once
  and referred to by index every other time. A decoder that rebuilds the writer's cache wrongly then
  renames the majority of the log rather than one record.
- **A group name that repeats** across users and across those repeats, so a cached string appears
  inside a group path and not only in a request name.

## Gatling 3.13.0 cannot be recorded

`3.13.0` is the first version to write a binary log and it is **not** in the corpus, because it
cannot read back the assertion records it writes: `IllegalArgumentException: Unknown object coding: 1`
out of boopickle in `AssertionPicklers`, while `FirstPassParser` parses the run record. No report is
generated and the run fails. One assertion is enough to trigger it; the probe was run with ten and
with one, and both failed the same way.

Its *writer* is fine — a 3.13.0 `simulation.log` parses cleanly to its last byte against the grammar
in `specs/005-gatling-binary-decoder/data-model.md` — but a run that produces no report has no
second, independent account of its own numbers, so it cannot be a corpus entry. 3.13.1 and every
version after it are unaffected.

Verified under Gatling 3.11.5 and 3.12.0 (see `specs/003-canonical-model/research.md` R1), and under
3.13.1, 3.14.9 and 3.15.1 (see `specs/005-gatling-binary-decoder/research.md` R8).
