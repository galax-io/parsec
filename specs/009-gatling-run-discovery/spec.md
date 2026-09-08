# Feature Specification: Finding the run

**Feature Branch**: `009-gatling-run-discovery`

**Created**: 2026-09-08

**Status**: Draft

**Input**: User description: "https://github.com/galax-io/parsec/milestone/10"

**Milestone**: v0.0.9 — Finding the run (parsec#11)

---

## Context

Everything this module reads, it reads from a path someone else worked out. Every entry point takes a
stream of bytes and starts decoding; the question of which file to open has never been asked here.
That was right while there was only one caller and it was a test.

It is wrong now, because the answer is not obvious and it is the same answer three times. Gatling
writes each run into a directory it names itself — the simulation's lowercased class name, a hyphen,
and the run's start as seventeen digits of local date and time — under a results root the build tool
chooses: `target/gatling` under Maven and sbt, `build/reports/gatling` under Gradle, anywhere at all
when the run was configured by hand. Nobody types `corpussimulation-20260906044741110` correctly
the first time, and today everybody has to: `galaxio report` takes the path, the comet sidecar
takes the path, ingestion takes the path. Each of the three is about to grow its own guessing,
and the three will not guess alike.

Gatling has already recorded the answer. It writes `lastRun.txt` into the results root, naming the
directory it just finished, and no consumer reads it. So the situation is not that the information is
missing — it is that it sits in a file this module has never opened, next to a directory layout every
caller re-derives.

What lands here is small and entirely about location: given a path that may be a run, a results root,
or nothing at all, produce the run directory and the `simulation.log` inside it, or a failure that
names where it looked. No log is opened to answer it. The version gate, the codecs and the model are
untouched — this feature ends exactly where opening the log begins.

Two things it deliberately does not become. It does not learn about build tools: reading a `pom.xml`
or a `build.gradle` to find the results root was considered and rejected in #11, because it makes a
library aware of Maven, Gradle and sbt for a value the caller can pass in one argument. And it does
not watch: noticing that a *new* run has appeared, and deciding that a run has ended, belong to the
sidecar (comet#3, comet#4) and were put out of scope in v0.0.8 for the same reason — only something
that can see the writer can tell.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A run is found without its generated name being typed (Priority: P1)

An engineer has just run a load test and wants a report. They point the consumer at the project they
ran it in — or at nothing, and let it use the results root the build tool writes to — and the run
they just finished is what gets read. They never see `corpussimulation-20260906044741110`, never open a
file manager to copy it, and never learn where their build tool puts Gatling output.

**Why this priority**: This is the whole milestone. Everything else here is a rule about *which* run
gets picked; this is the difference between a consumer that can be pointed at a project and one that
can only be pointed at a directory whose name a machine generated.

**Independent Test**: Create a results root holding one finished run, resolve against it with no run
named, and confirm the run directory and its `simulation.log` come back. Delivers the feature's
value with none of the tie-breaking rules built.

**Acceptance Scenarios**:

1. **Given** a results root holding one run directory with a `simulation.log`, **When** a run is
   resolved against that root, **Then** that run directory and the `simulation.log` inside it are
   returned.
2. **Given** a working directory whose default results root holds that run, **When** a run is
   resolved with no path at all, **Then** the same run is returned, and the caller can tell that the
   root was a default rather than one it chose.
3. **Given** a results root holding three runs and no `lastRun.txt`, **When** a run is resolved,
   **Then** the most recently modified of the three is returned.
4. **Given** a project whose results sit under the Gradle layout rather than the default, **When** the
   results root is overridden to that layout, **Then** the run under it is returned and nothing about
   the default is consulted.
5. **Given** a resolved run, **When** the caller asks how it was chosen, **Then** it can tell a run it
   named itself from one taken from `lastRun.txt` from one taken as the newest.

---

### User Story 2 - The run Gatling last wrote is the run that is read (Priority: P2)

The same engineer has three runs in the root. The newest by modification time is not the one they
just finished: a report regeneration touched an older one, a CI cache restore rewrote every timestamp
at once, or a directory was copied in from elsewhere. Gatling itself recorded which run it finished
last, and that recording is what decides — modification time is only consulted when Gatling's own
answer is missing or points at something that is no longer there.

**Why this priority**: Modification time is a proxy, and a bad one: it is set by whatever last wrote
to the directory, which after a `git clone`, an `rsync` or a CI cache restore is a single instant
shared by every run in the root. `lastRun.txt` is the writer's own statement and is the only
evidence in the tree that is actually about the run. Getting this order wrong means silently
reporting on the wrong test, which is worse than reporting on none.

**Independent Test**: Build a root with three runs whose modification times make the middle one
neither newest nor oldest, write a `lastRun.txt` naming the middle one, and confirm the middle one is
returned; delete the file and confirm the newest is. Tests the selection rule alone.

**Acceptance Scenarios**:

1. **Given** a results root with three runs and a `lastRun.txt` naming the middle one, **When** a run
   is resolved, **Then** the middle one is returned.
2. **Given** the same root with `lastRun.txt` removed, **When** a run is resolved, **Then** the newest
   by modification time is returned.
3. **Given** a `lastRun.txt` naming a directory that has been deleted, **When** a run is resolved,
   **Then** the newest surviving run is returned and the caller can tell that the recorded pointer was
   not the one used.
4. **Given** a `lastRun.txt` naming a directory that exists but holds no `simulation.log`, **When** a
   run is resolved, **Then** it is treated the same as a pointer to nothing: the newest run that does
   hold one is returned.
5. **Given** two runs whose modification times are identical, **When** a run is resolved, **Then** the
   same one is returned every time, on every platform, and the rule that chose it is documented.

---

### User Story 3 - A path that is already a run is honoured exactly (Priority: P2)

A CI job knows precisely which directory it wants; an archive holds a single run pulled out of the
tree it was made in; a script has the path to the `simulation.log` itself and nothing else. All three
pass what they have and get it back, with no search, no default and no results root involved.

**Why this priority**: It is what every caller does today, and it must keep working unchanged or the
feature is a migration rather than an addition. It is also the only path an archived run has — a run
lifted out of its results root has no `lastRun.txt` and no siblings to be newest among.

**Independent Test**: Resolve against a directory containing a `simulation.log`, then against the
`simulation.log` file itself, and confirm both return that run with nothing else read.

**Acceptance Scenarios**:

1. **Given** a path to a directory containing a `simulation.log`, **When** a run is resolved, **Then**
   that directory is the run and no results root is searched.
2. **Given** a path to a `simulation.log` file, **When** a run is resolved, **Then** its directory is
   the run and the file is the log.
3. **Given** a path to a directory that contains both a `simulation.log` and subdirectories that also
   contain one, **When** a run is resolved, **Then** the path itself is the run: a directory holding a
   log is a run, not a root.
4. **Given** an explicit path, **When** a run is resolved, **Then** no default results root is
   consulted, whether or not the explicit path succeeds.

---

### User Story 4 - A failure says where it looked (Priority: P3)

Someone points a consumer at the wrong directory — a project that was never run, a root the build tool
does not write to, a path with a typo. What comes back names the directory that was searched, so the
next thing they type is right. Nothing partial comes back with it.

**Why this priority**: The common failure of this feature is a caller in the wrong place, and the
whole cost of that failure is how long it takes to find out where the right place is. It is P3 only
because it is worthless until there is something to fail at.

**Independent Test**: Resolve against an empty directory and against a path that does not exist, and
confirm each failure names the directory searched and returns no run.

**Acceptance Scenarios**:

1. **Given** a results root holding no run, **When** a run is resolved, **Then** the failure names the
   directory that was searched and no run is returned.
2. **Given** a results root that does not exist, **When** a run is resolved, **Then** the failure names
   that path and says it was not there.
3. **Given** that no results root was supplied and the default was used, **When** resolution fails,
   **Then** the failure says the root was a default and which one, so the caller is not shown a path
   they never typed with no explanation of where it came from.
4. **Given** a directory that cannot be read because of its permissions, **When** a run is resolved,
   **Then** that is reported as the failure it is and not as an absence of runs.

---

### Edge Cases

- **`lastRun.txt` names a path with separators, or an absolute path, or `..`.** The file is data read
  off disk and is not trusted to stay inside the tree: a value that does not resolve to a direct child
  of the results root is not followed, and resolution continues as if the pointer were absent.
- **`lastRun.txt` is empty, is whitespace, or carries a trailing newline or carriage return.** A blank
  value is a pointer to nothing; surrounding whitespace and line endings do not change which directory
  is named. What the file actually contains is settled by a recording, not by assumption (see *Source
  Coverage*).
- **`lastRun.txt` holds more than one line.** The shape a real Gatling run writes decides this, and the
  behaviour is stated once that is recorded rather than guessed at now.
- **A run directory is a symbolic link.** An archive that stores a run elsewhere and links it in still
  resolves to a run; a link pointing at nothing is not a run.
- **The results root is itself a run directory.** It holds a `simulation.log`, so it is a run — the
  same rule as an explicit path (US3 scenario 3).
- **Every run in the root shares one modification time,** which is what a `git clone`, an `rsync` or a
  CI cache restore produces. The choice is still deterministic and documented.
- **A run directory holds a report but no `simulation.log`** — Gatling from 3.13.5 produces no report,
  and a report-only directory is the reverse case. Only a `simulation.log` makes a directory a run.
- **The log found is truncated, is a version below the gate, or is not a Gatling log at all.** None of
  these are discovery's to answer: it returns a location, and the existing gate and errors apply
  unchanged when that location is opened.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The module MUST resolve a Gatling run from a path that may be a run directory, a
  `simulation.log` file, a results root, or absent, and return the run directory together with the
  `simulation.log` within it.
- **FR-002**: Resolution MUST first decide what the path it was given denotes. A `simulation.log`
  file, or a directory holding one, is the run, and the search ends there. Anything else is treated
  as a results root and searched within, in a fixed order: the directory named by `lastRun.txt`,
  when that names an existing direct child holding a `simulation.log`; then the most recently
  modified direct child that holds one.
- **FR-003**: A path that denotes a run MUST be honoured exactly and never searched past: no
  results root is consulted for it, and no sibling or child run can displace it. A path that
  denotes neither a run nor a root containing one MUST fail rather than fall back to the default
  root, so a caller that named a place is never quietly answered about a different one.
- **FR-004**: A path naming a `simulation.log` file MUST be accepted, and MUST resolve to the same run
  as its containing directory.
- **FR-005**: A directory that holds a `simulation.log` MUST be treated as a run and never as a
  results root, even when it also holds subdirectories that hold one.
- **FR-006**: `lastRun.txt` MUST take precedence over modification time whenever it names an existing
  directory that holds a `simulation.log`. When it names nothing, names something that no longer
  exists, or names a directory without a log, resolution MUST continue to the modification-time rule
  rather than fail.
- **FR-007**: A value read from `lastRun.txt` MUST NOT be followed outside the results root. Anything
  that does not resolve to a direct child of that root is treated as a pointer to nothing.
- **FR-008**: Selection by modification time MUST be deterministic. Where several candidates share the
  newest time, the tie MUST be broken by a documented rule that yields the same run on every platform
  and on every repetition.
- **FR-009**: The caller MUST be able to learn how the run was chosen — given explicitly, named by
  `lastRun.txt`, or selected as the newest — so a consumer can say which run it is about to read and
  so a pointer that was skipped is visible rather than silent.
- **FR-010**: The results root MUST default to the layout Maven and sbt write to, and MUST be
  overridable by the caller for the Gradle layout and for any configured output directory.
- **FR-011**: When no run is found, the failure MUST name the directory that was searched, MUST state
  whether that directory was the caller's or the default, and MUST return no run alongside it.
- **FR-012**: A directory that cannot be read MUST be reported as that failure, carrying the path
  concerned, and MUST NOT be reported as an absence of runs or silently skipped.
- **FR-013**: Resolution MUST NOT open or read any `simulation.log`. It reads directory entries and,
  at most, `lastRun.txt`.
- **FR-014**: Resolution MUST NOT apply, duplicate or pre-empt the version gate. A run whose log is
  unreadable, truncated or out of range still resolves; the existing errors are raised when the log is
  opened.
- **FR-015**: Every identifier this feature exports MUST carry a doc comment stating what it does and
  which results-root layouts it assumes, and every user-visible change MUST be recorded in
  `CHANGELOG.md`.

### Key Entities

- **Run**: a single finished execution of a simulation, held in one directory. Identified by that
  directory and the `simulation.log` inside it. What else the directory holds — a report, a `js`
  tree, nothing at all — does not decide whether it is a run.
- **Results root**: the directory a build tool collects run directories in. Not itself a run unless it
  holds a `simulation.log`.
- **Last-run pointer**: the record Gatling leaves in the results root naming the directory it finished
  most recently. Evidence, not a guarantee: it can be stale, absent, or point at a deleted run.
- **Resolution**: the outcome of a lookup — the run, plus how it was chosen. The second half exists so
  a consumer can report which run it read and why.

### Source Coverage

- **Tool and versions**: Gatling. The results-root layout, the generated run-directory name and
  `lastRun.txt`, as written across 3.11.5 through 3.15.1 — the range the corpus already covers and the
  range the codecs read.
- **Artefact formats**: directory layout and the `lastRun.txt` text file. Not `simulation.log`, which
  this feature locates and never opens.
- **Version gate**: none applies here, and none is added. Discovery names a location; the gate belongs
  to the codec and runs unchanged when the located log is opened (FR-014).
- **Not provided by this source**: nothing new is declared through `Capabilities`. This feature adds no
  field to the canonical model and computes nothing.
- **Golden corpus**: planning established more from the existing recordings than this section first
  assumed. The run-directory name format and the sbt results root are both evidenced by the three
  recorded console logs, which name real run directories under `simulation/target/gatling/`; the
  format of `lastRun.txt` is evidenced by the plugin that writes it. What no existing entry holds is a
  real `lastRun.txt`, because only `gatling-maven-plugin` writes one and the corpus simulation is an
  sbt project — so the one recording this feature wants is a **Maven** results root, committed with
  its `lastRun.txt` and the run directories it names. That is the single open cost in the feature, and
  the plan carries the decision. Everything beyond that shape (three runs, a stale pointer, an empty
  root, a shared modification time, a symlinked run) is constructed, and is named as a fixture rather
  than corpus.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A run is resolved from a project directory with no generated run-directory name typed by
  hand, for both results-root layouts in use: the one Maven and sbt write to, and Gradle's.
- **SC-002**: The three acceptance cases in #11 — `lastRun.txt` naming the middle of three runs, the
  same root with the file removed, and an empty root — produce the middle run, the newest run, and a
  failure naming the searched path with no run returned.
- **SC-003**: Selection is stable: over repeated resolutions of the same root, including one whose run
  directories all share a single modification time, the same run is returned every time on macOS and
  Linux.
- **SC-004**: No byte of any `simulation.log` is read to resolve a run; a root whose logs are
  unreadable, truncated or of an unsupported version still resolves, and fails only when the log is
  opened.
- **SC-005**: Every failure names the directory that was searched and says whether it was the caller's
  or a default, and no failure returns a run beside it.
- **SC-006**: A `lastRun.txt` whose value points outside the results root selects nothing outside that
  root, for every escaping shape tested.
- **SC-007**: The accepted shape of `lastRun.txt` and of a generated run-directory name match what a
  real Gatling run writes, evidenced by the recorded results root rather than by assumption.
- **SC-008**: Each of the three consumers can drop its own path guessing and call one entry point;
  none needs a Gatling-layout code path of its own.
- **SC-009**: Coverage floors hold — 90% for the decoder packages, 80% for the module — and the
  standard-library-only boundary over `model/` and `gatling/` stays green.

## Assumptions

- **Discovery returns a location, not an open log.** It hands back the run directory and the
  `simulation.log` path; the caller opens it through the entry points that already exist. A second
  way to open a log would be a second public decoding interface to freeze at v0.1.0, which #10 and
  comet#3 both rejected for the same reason, and Principle VI rules out building one before something
  needs it.
- **The default results root is the Maven layout, as #11 directs.** sbt writes to the same
  `target/gatling`, so one default covers two of the three build tools; Gradle's
  `build/reports/gatling` is the documented override rather than a second thing to search. The default
  is resolved relative to the caller's working directory, which is why FR-011 requires a failure to say
  the root was a default — a consumer running as a server has no such directory and should be told so
  rather than shown a relative path it never chose.
- **A results root is searched one level deep.** That is the layout Gatling writes, and walking an
  arbitrary tree turns a wrong path into a long, surprising scan of whatever it was pointed at. A run
  anywhere else is reached by naming it (US3).
- **Modification time is the fallback, per #11.** Its weakness under clones and cache restores is why
  `lastRun.txt` comes first (US2) and why FR-008 requires the tie to break deterministically; it is not
  a reason to change the order the issue settled.
- **What `lastRun.txt` contains is not assumed, and is no longer unknown.** Planning read the plugin
  that writes it: each line is a bare directory name, there is one line per run the build produced, a
  failed run appends a line that is an error message rather than a name, and the separator is the
  platform's. The requirements above are written against that, not against a guess — and the file
  turns out to be absent far more often than present, which is why FR-008's ordering carries more
  weight than its single line suggests.
- **This is a new exported surface in a compatibility-sensitive module.** Before v0.1.0 it may still
  change (Principle V), and the next milestone freezes it — so the names and the shape are chosen here
  as if permanent, and recorded under Added in `CHANGELOG.md`.
- **The standard library is enough.** Directory listing, path handling and reading one small text file
  need no dependency, and `gatling/` may take none (Principle IV).

## Dependencies

- **parsec#11** is the only issue in this milestone; PR #72 is already merged into it. #11's own text
  records that its earlier dependency on #10 was not real, so nothing blocks this work.
- **A new corpus recording** of a results root, made with the existing corpus simulation project. It
  needs a working Gatling toolchain, as every recording does, and it is the one part of this feature
  that cannot be produced later.
- **No new module dependency** (Principle IV): the standard library only.

## Out of Scope

- **Watching for new runs, and deciding that a run has ended.** #11's stated non-goal, and v0.0.8's:
  they belong to the sidecar (comet#3, comet#4), and only something that can see the writer can answer
  them.
- **Reading `pom.xml`, `build.gradle` or `build.sbt` to find the results root.** Evaluated and rejected
  in #11: it makes this library aware of three build tools for a value the caller can pass in one
  argument.
- **Discovering runs for JMeter, k6, Locust or Yandex.Tank.** Each of those tools lays its results out
  differently, and their adapters have not landed. Whether they share a shape with this one is a
  question for the milestone that brings the second of them.
- **Reading anything else in a run directory** — the HTML report, the `js` tree, `console.txt`. This
  feature locates a run; what a consumer does inside it is the consumer's.
- **Any change to the codecs, the version gate, the errors a read can end with, or the canonical
  model.** Nothing in `model/`, `gatling/text/`, `gatling/binary/` or `gatling/simlog/` changes
  behaviour here.
- **Statistics of any kind.** This module computes none (Principle I), and locating a run does not
  begin to.
