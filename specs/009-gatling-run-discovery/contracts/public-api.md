# Contract 1 — public API

**Status**: four additions, no change to any existing identifier. Permitted before v0.1.0
(Principle V) and recorded under **Added** in `CHANGELOG.md` in the same PR. The freeze
[parsec#13](https://github.com/galax-io/parsec/issues/13) asks for is the next milestone, so these
names are chosen as if permanent — see research
[R5](../research.md#r5--where-discovery-lives-and-what-it-is-called) for why none of them is `Run`.

## Added

```go
package gatling

// FindRun locates a Gatling run. It returns where the run's artefacts sit and
// which rule chose them; it opens no simulation.log and applies no version
// gate, so a run whose log is truncated, damaged or out of the supported range
// still resolves and fails only when the log is read.
//
// path may name the run itself — a simulation.log, or a directory holding one —
// in which case it is returned as given and nothing else is searched. Otherwise
// path is a results root and its immediate children are searched: the run named
// by lastRun.txt if that file names one that is still there, and failing that
// the most recently modified run in the root.
//
// An empty path means the results root Maven and sbt write to, target/gatling,
// relative to the working directory. Gradle writes to build/reports/gatling and
// a run configured by hand writes wherever it was told to; pass either as path.
//
// When no run is found it returns a *RunNotFoundError naming the directory that
// was searched and saying whether that directory was the default. A directory
// that cannot be read is reported as that failure, wrapping the *fs.PathError,
// and never as an absence of runs.
func FindRun(path string) (RunLocation, error)

// RunLocation is where a Gatling run's artefacts sit, and how they were found.
type RunLocation struct {
    // Dir is the run directory.
    Dir string
    // Log is the simulation.log inside Dir. It is always Dir joined with
    // "simulation.log": a directory is a run because it holds one.
    Log string
    // Found is the rule that selected this run.
    Found FoundBy
}

// FoundBy is how a run was chosen. A caller that reports which run it read
// should say this too: FoundByNewest is a guess from modification times, and it
// is the ordinary outcome, because only gatling-maven-plugin writes a
// lastRun.txt and it deletes it again during gatling:verify.
type FoundBy uint8

const (
    // FoundByUnknown is the zero value and is never returned.
    FoundByUnknown FoundBy = iota
    // FoundByPath means the caller named the run.
    FoundByPath
    // FoundByLastRun means lastRun.txt named it and it was still there.
    FoundByLastRun
    // FoundByNewest means it was the most recently modified run in the results
    // root. Ties break on the directory name, descending, which is run-start
    // order for the names Gatling generates.
    FoundByNewest
)

// String returns the rule's name.
func (f FoundBy) String() string

// RunNotFoundError ends a search that completed and found no run. It is not
// raised for a directory that could not be read: a failure to look is not an
// absence of runs, and that failure is returned wrapping its *fs.PathError.
type RunNotFoundError struct {
    // Dir is the directory that was searched.
    Dir string
    // Default reports whether Dir is the default results root rather than one
    // the caller named. A consumer with no meaningful working directory — a
    // server — gets a relative path it never chose, and this says so.
    Default bool
}

func (e *RunNotFoundError) Error() string
```

## Changed

Nothing. No existing identifier changes signature or behaviour, and no serialized format is touched.
A consumer built against v0.0.8 compiles and behaves identically against v0.0.9.

## What a consumer does with it

| Consumer | Before | After |
|---|---|---|
| `galaxio report`, given a project | works out `target/gatling`, lists it, picks by name or mtime | `gatling.FindRun("")`, then opens `loc.Log` |
| `galaxio report`, given a path | opens it, or joins `simulation.log` itself | `gatling.FindRun(path)` — both shapes accepted |
| the comet sidecar | its own results-root guessing before it can follow | `gatling.FindRun(root)` for the starting run; watching for the *next* one stays its own (spec, *Out of Scope*) |
| the Galaxio backend | is handed a path by its uploader | unchanged; `FindRun(dir)` normalises a directory to its log if it wants |

The composition this feature is shaped around, and the whole of the integration:

```go
loc, err := gatling.FindRun("")
if err != nil { return err }
f, err := os.Open(loc.Log)
if err != nil { return err }
defer f.Close()
r, err := simlog.NewRunReader(f)
```

`FindRun` deliberately stops one line short of opening the file. Returning a reader would put a
second way to open a log on the public surface — rejected in
[#10](https://github.com/galax-io/parsec/issues/10) and comet#3 for the same reason — and would make
the version gate fire inside a function whose job is to name a directory.

## Doc-comment obligations (FR-015)

Every identifier above carries the doc comment shown. Three facts must appear and must not drift,
because a caller who has to discover them by experiment has learned nothing this feature was for:

1. `FindRun` opens no log and runs no version gate.
2. The default root is `target/gatling`, and it is Maven's and sbt's, not Gradle's.
3. `FoundByNewest` is a guess, and it is the common case — with the reason (contract 2).

The package doc comment of `gatling` widens from "what every Gatling codec shares" to cover finding
the run, since discovery is not shared by the codecs but sits before them.
