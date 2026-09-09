# Contract 1 — public API

**Status**: a new package, `github.com/galax-io/parsec/gatling/run`, with six exported identifiers.
No identifier that existed before v0.0.9 changes. Permitted before v0.1.0
(Principle V) and recorded under **Added** in `CHANGELOG.md` in the same PR. The freeze
[parsec#13](https://github.com/galax-io/parsec/issues/13) asks for is the next milestone, so these
names are chosen as if permanent — see research
[R5](../research.md#r5--where-discovery-lives-and-what-it-is-called) for why none of them is `Run`.

## Added

```go
package run // github.com/galax-io/parsec/gatling/run

// DefaultResultsRoot is where Maven and sbt both write Gatling's output. It is
// exported rather than applied automatically: the same relative path means
// "this project" to a CLI standing in one and something arbitrary to a server.
const DefaultResultsRoot = "target/gatling"

// ErrNoPath is returned by Find when it is given an empty path. "" is the zero
// value of every unset flag, configuration field and omitted request member, so
// guessing a root for it would turn missing input into a confident report about
// an unrelated run.
var ErrNoPath = errors.New("gatling: no path given; …")

// Find locates a Gatling run: a simulation.log, a directory holding one, or a
// results root to search. It opens no log and applies no version gate.
func Find(path string) (Location, error)

// Location is where a Gatling run's artefacts sit, and how they were found. Dir
// is cleaned, so every spelling of one run yields one value.
type Location struct {
    Dir   string
    Log   string
    Found FoundBy
}

// FoundBy is how a run was chosen: FoundByPath, FoundByLastRun, FoundByNewest,
// behind FoundByUnknown at iota 0. String() renders "path", "lastRun.txt",
// "newest", "unknown".
type FoundBy uint8

// NotFoundError ends a search that found no run. It names only a directory the
// caller gave: none is ever substituted.
type NotFoundError struct {
    Dir string
}
```

`FoundByUnknown` is the zero value and is never returned beside a nil error; every failing branch
returns the zero `Location`.

## Changed

Nothing outside this feature. No identifier that existed before v0.0.9 changes signature or
behaviour, and no serialized format is touched; a consumer built against v0.0.8 compiles and behaves
identically against v0.0.9.

Within the feature, review changed the shape twice before release. **The package moved**: discovery
was first put in the root `gatling` package and now lives in `gatling/run`, which imports nothing else
in this module — see research [R5](../research.md#r5--where-discovery-lives-and-what-it-is-called) for
why the original reasoning did not hold. The names shortened with it, because the package name carries
`run`: `FindRun` → `Find`, `RunLocation` → `Location`, `RunNotFoundError` → `NotFoundError`. The last
of those existed only to dodge a collision with `model.Run` that does not arise here.

And the empty-path default went: `Find("")` no longer means
`target/gatling`. That convenience is withdrawn because `""` is what an unset flag, an absent
configuration field and an omitted request member all look like, and a server that lost its path
would have been handed a plausible run from its own working directory instead of an error. The
layout is published as `DefaultResultsRoot` for a caller to pass deliberately. `NotFoundError`
loses its `Default` field with it: nothing is substituted any more, so there is nothing to disclose.

## What a consumer does with it

| Consumer | Before | After |
|---|---|---|
| `galaxio report`, given a project | works out `target/gatling`, lists it, picks by name or mtime | `run.Find(run.DefaultResultsRoot)`, then opens `loc.Log` |
| `galaxio report`, given a path | opens it, or joins `simulation.log` itself | `run.Find(path)` — both shapes accepted |
| the comet sidecar | its own results-root guessing before it can follow | `run.Find(root)` for the starting run; watching for the *next* one stays its own (spec, *Out of Scope*) |
| the Galaxio backend | is handed a path by its uploader | unchanged; `Find(dir)` normalises a directory to its log if it wants |

The composition this feature is shaped around, and the whole of the integration:

```go
loc, err := run.Find(run.DefaultResultsRoot)
if err != nil { return err }
f, err := os.Open(loc.Log)
if err != nil { return err }
defer f.Close()
r, err := simlog.NewRunReader(f)
```

`Find` deliberately stops one line short of opening the file. Returning a reader would put a
second way to open a log on the public surface — rejected in
[#10](https://github.com/galax-io/parsec/issues/10) and comet#3 for the same reason — and would make
the version gate fire inside a function whose job is to name a directory.

## Doc-comment obligations (FR-015)

Every identifier above carries the doc comment shown. Three facts must appear and must not drift,
because a caller who has to discover them by experiment has learned nothing this feature was for:

1. `Find` opens no log and runs no version gate.
2. `DefaultResultsRoot` is Maven's and sbt's, not Gradle's — and it is a value to pass, never a
   substitution, so an absent path is `ErrNoPath` rather than a guess.
3. `FoundByNewest` is a guess, and it is the common case — with the reason (contract 2).

The package doc comment of `gatling` widens from "what every Gatling codec shares" to cover finding
the run, since discovery is not shared by the codecs but sits before them.
