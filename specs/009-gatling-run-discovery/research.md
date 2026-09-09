# Research: Finding the run

**Feature**: 009-gatling-run-discovery | **Date**: 2026-09-08 | **Spec**: [spec.md](spec.md)

Everything below was read off primary sources on the machine this was planned on: the Gatling
plugin bytecode in `~/.m2` and `~/.gradle`, and the corpus recordings in this repository. Nothing
here is quoted from documentation, because the behaviour that matters is not documented.

The headline is that the spec's central assumption was half wrong, and the correction makes the
*fallback* path the main one. It is written up first.

---

## R1 — `lastRun.txt` is written by the Maven plugin, not by Gatling

**Decision**: treat `lastRun.txt` as an artefact of `gatling-maven-plugin`, present for some builds
and absent for most.

**Evidence**. `gatling-core` 3.11.5 and 3.13.5 were unpacked and searched: no class mentions
`lastRun`. Every hit across the 126 Gatling jars on this machine is in `gatling-maven-plugin`
(every version from 3.0.5 to 4.21.10). In 4.21.10:

- `io/gatling/mojo/AbstractGatlingExecutionMojo` declares
  `static final String LAST_RUN_FILE = "lastRun.txt"` and
  `static final String LAST_RUN_FILE_ERROR_LINE = "ExecutionError:"`.
- `io/gatling/mojo/GatlingMojo.saveSimulationResultToFile(Set<File>, Exception)` writes it:

  ```java
  Path p = resultsFolder.toPath().resolve("lastRun.txt");
  try (BufferedWriter w = Files.newBufferedWriter(p)) {
      for (File dir : runDirectories())          // the directories present after the run
          if (!before.contains(dir))             // ...that were not there before it
              w.write(dir.getName() + System.lineSeparator());
      if (e != null)
          w.write((e instanceof GatlingSimulationAssertionsFailedException
                   ? "Gatling simulation assertions failed!"
                   : getRecursiveCauses(e)) + System.lineSeparator());
  }
  ```

**What this fixes in the spec's model of the file.** Four things, none of which were guessable:

| Property | Ground truth |
|---|---|
| What a line holds | `File.getName()` — a **bare directory name**. Never a path, never absolute. |
| How many lines | **One per newly created run directory.** Maven's `runMultipleSimulations` produces several. |
| Trailing content | An optional **error line** that is not a directory name at all. |
| Encoding, endings | UTF-8 (`Files.newBufferedWriter` with no charset), `System.lineSeparator()` — so CRLF on Windows. |

**And it is gated.** `GatlingMojo.execute()` calls `saveSimulationResultToFile` only when
`failOnError` is **false** — the bytecode branches `getfield failOnError; ifne` straight past the
call — and that parameter is declared `@Parameter(property = "gatling.failOnError", defaultValue =
"true")`. So an ordinary `mvn gatling:test` writes no `lastRun.txt` at all. This was found by
recording rather than by reading: the first Maven run made for
[R9](#r9--what-the-corpus-already-proves-and-the-one-thing-it-cannot) produced a run directory and no
file, which sent the search back to `execute()`. It is the sharpest limit on how often the file
exists, and the reading alone had missed it.

**Alternatives considered**: assuming a single line holding a bare name, which is what the spec
implied. It is right in the common case and silently wrong for a multi-simulation build and for a
failed one.

---

## R2 — It is also short-lived, so modification time is the ordinary path

**Decision**: design for `lastRun.txt` being **absent**, and treat the modification-time rule as the
primary path rather than a fallback.

**Evidence**. `io/gatling/mojo/VerifyMojo.verifyLastRun()`:

```java
Path p = resultsFolder.toPath().resolve("lastRun.txt");
if (p.toFile().exists()) {
    List<String> lines = Files.readAllLines(p);
    p.toFile().delete();                       // the file does not survive gatling:verify
    for (String line : lines) checkError(line); // fails the build on contains("ExecutionError:")
}
```

So the file is gone after `mvn gatling:verify`, which is the ordinary Maven lifecycle. Beyond that:

- **Gradle** — `gatling-gradle-plugin` 3.15.1.2 unpacked and searched: no class mentions `lastRun`.
  It never writes one.
- **sbt** — `gatling-sbt` 4.19.1 (the version in the corpus project's `project/plugins.sbt`) is not
  cached on this machine, so this is **not verified directly**; nothing in the three recorded console
  logs mentions the file. Treated as "assume absent", which is the safe direction: the code path that
  handles absence is exercised either way.

**Consequence, and it is the important one.** For a Gradle user, an sbt user, any Maven user on the
default `failOnError` (R1), and any Maven user who ran `gatling:verify`, `lastRun.txt` is not there.
The rule the issue frames as the fallback is what almost every caller will actually hit — which is
why [R6](#r6--the-tie-break-is-load-bearing-not-a-formality) matters far more than its one line in
the spec suggests.

---

## R3 — The results roots, confirmed for all three build tools

**Decision**: default to `target/gatling`; document `build/reports/gatling` as the Gradle override.
This is what the spec assumed, and it now has evidence.

| Build tool | Results root | Evidence |
|---|---|---|
| Maven | `target/gatling` | `AbstractGatlingExecutionMojo.resultsFolder` carries `@Parameter(defaultValue = "${project.build.directory}/gatling")`; `${project.build.directory}` is `target`. |
| Gradle | `build/reports/gatling` | The constant `reports/gatling` in `io/gatling/gradle/GatlingRecorderTask`, resolved against Gradle's build directory. |
| sbt | `target/gatling` | **Measured, not inferred.** All three binary recordings' `console.txt` end with `.../testdata/corpus/gatling/simulation/target/gatling/<runId>/index.html`. |

So one default covers two of the three build tools, which is the justification the spec claimed and
could not yet show.

---

## R4 — A run directory is named `<simulationId>-<yyyyMMddHHmmssSSS>`

**Decision**: rely on the name being a single path segment; rely on it sorting in time order; rely on
nothing else about it.

**Evidence**, two independent sources that agree:

- `gatling-core` 3.13.5, `io/gatling/core/stats/writer/RunMessage` — string constants
  `yyyyMMddHHmmssSSS` and a two-slot interpolation template joined by a hyphen.
  `io/gatling/core/config/GatlingFiles.resultDirectory(runId, results)` is
  `results.resolve(runId)`, so the run directory *is* the run id, one segment deep.
- The corpus itself. The recorded console output holds three real run ids from three real runs:
  `corpussimulation-20260906044741110`, `corpussimulation-20260906044803343`,
  `corpussimulation-20260906044814356` — and the RUN record of the 3.11.5 log shows where
  `corpussimulation` comes from: the simulation id beside the class name,
  `io.galaxio.parsec.corpus.CorpusSimulation` then `corpussimulation`.

**Correction to the spec.** Its Context said the name ends in "the run's start in epoch
milliseconds". It does not: it is seventeen digits of `yyyyMMddHHmmssSSS`, and — measured, after
[R9](#r9--what-the-corpus-already-proves-and-the-one-thing-it-cannot)'s recording made it checkable —
in **UTC**, not local time as this section first claimed. The Maven run recorded on a machine at
+04:00 produced `corpussimulation-20260909022708912` from a run whose own RUN record decodes to
`20260909022708.912` UTC and `20260909062708.912` local; the sbt-made `3.13.1` entry agrees, its
console naming `corpussimulation-20260906044741110` for a start of `20260906044741.110` UTC. Two
build tools, so this is Gatling's behaviour and not a plugin's JVM argument.

The difference is not cosmetic. Epoch millis and `yyyyMMddHHmmssSSS` sort the same way, but a *local*
`yyyyMMddHHmmssSSS` would not sort monotonically across a daylight-saving fall-back — an hour of run
ids would repeat and the tie-break below would silently prefer the earlier run. In UTC it is a total
order for good.

---

## R5 — Where discovery lives, and what it is called

**Decision**: the `gatling` root package. One function and one result type.

```go
func FindRun(path string) (RunLocation, error)
```

**Rationale**. Issue #11 says "**Where**: `gatling`: run-directory discovery and the results-root
defaults"; `AGENTS.md` "Structure" and the plan template's source-tree block both already list run
discovery under `gatling/`. The package holds `Version`, `Format`, `Policy` and the error types —
the cross-cutting things that are not any one codec's — and discovery is another. Its doc comment
widens from "what every Gatling codec shares" to say so.

**Alternative rejected**: a `gatling/run` subpackage, symmetric with `gatling/simlog`. It is cleaner
in the abstract and would keep `gatling` free of filesystem code, but it buys separation this
feature has no use for and adds a second package to freeze at v0.1.0 — an abstraction ahead of its
need (Principle VI).

**Naming** (`golang-naming`, required reading). `model.Run` already exists and is *the* canonical
result; a `gatling.Run` meaning "a directory we located" would sit next to it in every consumer that
imports both and mean something else entirely. Principle V makes that near-permanent one milestone
from now. Hence `RunLocation`, and `FoundBy` for how it was chosen — not `Source`, which in this
module already means the tool that produced the artefact.

**No options.** The results root is the argument: `FindRun(DefaultResultsRoot)` asks for the Maven
and sbt layout, `FindRun("build/reports/gatling")` is the Gradle one, `FindRun(dir)` names a run. A
functional-option parameter was considered and dropped — there is one knob and it is already the
first argument.

**Revised in review**: `FindRun("")` originally meant the default. It no longer does, because `""` is
the zero value of every unset flag and omitted request field, and a library that guesses for it turns
missing input into a confident answer about an unrelated run. The layout is published instead.

---

## R6 — The tie-break is load-bearing, not a formality

**Decision**: order candidates by modification time descending, and break ties by **directory name,
descending**.

**Rationale**. FR-008 asks only for determinism, and any rule would satisfy it. This one is also
*correct*, because of [R4](#r4--a-run-directory-is-named-simulationid-yyyymmddhhmmsssss):
`yyyyMMddHHmmssSSS` sorts lexicographically in time order, so descending name is descending run
start for every directory Gatling generated.

That matters because of [R2](#r2--it-is-also-short-lived-so-modification-time-is-the-ordinary-path).
A `git clone`, an `rsync -r` without `-t`, a CI cache restore or a container image build gives every
run in a root the *same* mtime — and those are the same users who have no `lastRun.txt`. For them
the tie-break is not a tie-break at all: it is the selection. Ordering by mtime alone would pick
arbitrarily among identical timestamps, which is exactly the "silently reports the wrong test"
failure US2 exists to prevent.

**Corrected after implementation.** Two things in this section were wrong, and review caught both.

The timestamp compared is the **log's**, not the run directory's: a directory's time moves whenever
anything is written into it, and regenerating a report into an old run made that run the newest —
the failure US2 exists to prevent, arriving through the very rule meant to prevent it.

And "descending name is run order" holds only *within one simulation id*. A run directory is
`<simulationId>-<yyyyMMddHHmmssSSS>`, so whole-name order is alphabetical by simulation id first;
a root holding two simulations — Maven's `runMultipleSimulations` — returned the run that started
months earlier. The tie-break now compares the stamp itself, falling back to the whole name when a
directory carries none.

**Alternatives considered**: name first and mtime as the tie-break, which is arguably more faithful
to "the newest run" for archived trees. Still rejected: #11 settles modification time as the rule,
and the stamp comparison recovers the accuracy that motivated the alternative without overturning
the issue's decision.

---

## R7 — A multi-line `lastRun.txt` needs no special case

**Decision**: consider **every** line that names an existing direct child holding a `simulation.log`,
and choose among those by the same order as [R6](#r6--the-tie-break-is-load-bearing-not-a-formality).

**Rationale**. It collapses three cases into one rule:

- one line (the common case) yields that run;
- several lines, from `runMultipleSimulations`, yield the newest of the ones this build just wrote,
  which is what "the last run" means;
- an `ExecutionError:` line, or `Gatling simulation assertions failed!`, names no directory, fails
  the existence check, and drops out. No string matching on error text, so nothing breaks when the
  plugin rewords a message.

**Alternative rejected**: take the last line. It is one line of code, and it is wrong precisely when
the run failed — the last line is then the error message.

---

## R8 — Containment is textual, so a symlinked run still resolves

**Decision**: a line from `lastRun.txt` is followed only if it is a **bare name** — equal to its own
base, not `.` or `..`, containing no separator. Whether that child is a symlink is not this module's
business.

**Rationale, and the conflict it resolves.** `os.Root` (Go 1.24, available under this `go 1.25`
directive) confines every operation to a directory tree by construction and would satisfy FR-007
with no validation at all. It also **refuses a symlink that leaves the root** — and the spec's edge
cases require the opposite: "an archive that stores a run elsewhere and links it in still resolves to
a run". The two cannot both hold, so `os.Root` is not used here.

Textual validation is what FR-007 actually asks for ("does not resolve to a direct child"), and it
is proportionate to the threat: `lastRun.txt` is written by the caller's own build tool into the
caller's own results root. FR-007 guards against a **stale or garbage file** sending discovery
somewhere surprising, not against an adversary who can already write into the tree being searched.
That threat model is stated in the contract so a later reader does not mistake it for a security
boundary.

---

## R9 — What the corpus already proves, and the one thing it cannot

**Decision**: the spec's requirement for a new recording **narrows to one purpose**, and that purpose
needs a build tool the corpus project did not have. It was recorded; see the resolution below.

What the existing recordings already establish, with no new run:

- the sbt results root is `target/gatling` (R3);
- the run-directory name format, from three real run ids (R4);
- that a real root held **three runs at once** — 3.13.1, 3.14.9 and 3.15.1 were recorded into
  `simulation/target/gatling/` within 33 seconds of each other on 2026-09-06. The three-run root the
  issue's acceptance case describes actually existed; it was flattened when the entries were
  committed.

What they cannot establish, and no later work can recover: a real `lastRun.txt`. Per R1 and R2 only
`gatling-maven-plugin` writes one, and `testdata/corpus/gatling/simulation/` is an sbt project.

**Resolved 2026-09-09: recorded.** `testdata/corpus/gatling/simulation/pom.xml` was added beside the
existing `build.sbt` — same sources, same stub, a second build tool — and three Maven runs were made
into one results root, committed as `testdata/corpus/gatling/lastrun/` with their console output and
a `RECORDING.md`. The recording paid for itself twice over, and neither return was the one expected:

1. **It found the `failOnError` gate.** The first run produced a run directory and *no*
   `lastRun.txt`, which is what sent the search back into `GatlingMojo.execute()` and turned up the
   branch now recorded in [R1](#r1--lastruntxt-is-written-by-the-maven-plugin-not-by-gatling). Reading
   the writer had been enough to know the file's *format* and not enough to know when it exists at
   all — precisely the gap Principle III's "real artefacts, not mocks" exists to close.
2. **It disproved this document's own claim about the run-directory name.**
   [R4](#r4--a-run-directory-is-named-simulationid-yyyymmddhhmmsssss) said local time; the recording
   says UTC, and the sbt entries agree.

It also produced #11's acceptance case as an artefact rather than a fixture: three real run
directories whose `lastRun.txt` names the **middle** one. That took three runs, because the plugin
writes only the directories its own execution created — run 1 (default) wrote no file, run 2
(`failOnError=false`) wrote one naming itself, and run 3 (default again) added a directory and left
the file alone.

What was *not* captured: the multi-line shape (it needs a second simulation class and
`runMultipleSimulations`) and the error line (it needs a failing run). Both are held by unit tests
written against R1's bytecode, which shows every branch of the writer rather than one instance.

Everything beyond that — a stale pointer, an empty root, a shared modification time, a symlinked run,
an escaping line — is constructed under `t.TempDir()` and is a **fixture, not corpus**, exactly as the
cut logs were in v0.0.8.

---

## R10 — No decoder benchmark rule applies; a bound is stated anyway

**Decision**: no throughput figure. State and measure a bound on the work instead.

The constitution requires a throughput and peak-memory goal from "a plan for a **decoder** feature",
measured over the largest corpus file. This feature decodes nothing and, by FR-013, opens no
`simulation.log` at all — there is no stream and no corpus file to measure against.

What it does do is filesystem work proportional to the size of the results root, so the bound is
stated in those terms and held by a benchmark over a synthetic root: **one directory read, at most
one `stat` per entry, and one read of a file under 64 KiB**, with peak memory proportional to the
entry count and not to anything inside a run. That is what gives SC-004 teeth.

---

## R11 — Skills read, and where they disagreed

Required-reading rows triggered (constitution, Engineering Guidance):

| Row | Why it fires | What it changed |
|---|---|---|
| `golang-naming` | four new exported identifiers | The `model.Run` collision in R5, and `FoundBy` over `Source`. |
| `golang-error-handling` | a new error type and every path that returns one | Errors as values, `%w`, `errors.As`; a "not found" and an "unreadable" are different errors (FR-011 vs FR-012), not one error with a flag. |
| `golang-documentation` | four new exported identifiers | Doc comments stating the default root and which build tools write `lastRun.txt`. |
| `golang-structs-interfaces` | a new exported struct and enum | A plain struct, no interface: there is one implementation and no second one in view. |
| `golang-testing` | every change | Table-driven over `t.TempDir()`; real directories, real mtimes, real symlinks. |

Consulted, occasion arisen: `golang-safety` and `golang-security` for R8's containment question, and
`golang-benchmark` for R10's bound.

**Disagreements, recorded as the constitution requires.** The skill set recommends `stretchr/testify`
for assertions and `samber/*` for helpers; both are forbidden here (Principle IV, and the `deps` job
enforces it on `gatling/`), and neither is used. Nothing else in the required rows conflicted.
