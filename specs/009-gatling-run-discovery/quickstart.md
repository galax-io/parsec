# Quickstart: Finding the run

**Feature**: 009-gatling-run-discovery | **Date**: 2026-09-08

How to validate this feature, and — more usefully — how to make each part of it fail. A test that
has never been seen to fail is not evidence.

## Prerequisites

Go 1.25 or newer (`go.mod` is authoritative) and this repository. Nothing else: discovery reads
directories and one small text file, so every scenario below runs from a `t.TempDir()` with no
Gatling, no JVM and no network.

The one exception is the corpus recording in [scenario 7](#7-the-recording-integration), which needs
Maven and a JDK, and which is skipped when they are absent rather than faked (Principle III).

## Verify

```bash
go build ./... && go test -race -shuffle=on ./gatling/
```

The full gate, as the constitution defines a green commit:

```bash
gofmt -l . && go vet ./... && go test -race -shuffle=on ./... && go test -tags=integration ./...
```

Coverage against the decoder-package floor of 90%:

```bash
go test -cover ./gatling/
```

## The shape every scenario builds

A results root is a directory of run directories, and a run directory is one that holds a
`simulation.log`. That is the whole of it:

```text
<root>/
├── lastRun.txt                                  # optional, and Maven-only (contract 2)
├── corpussimulation-20260906044741110/
│   └── simulation.log
├── corpussimulation-20260906044803343/
│   └── simulation.log
└── corpussimulation-20260906044814356/
    └── simulation.log
```

The names are the real ones, taken from the corpus recordings
(research [R4](research.md#r4--a-run-directory-is-named-simulationid-yyyymmddhhmmsssss)). The
`simulation.log` files can be empty: nothing here opens them, and a test that puts real bytes in
them is testing the wrong thing.

## Scenarios

Each maps to the spec's acceptance criteria and each says how to break it.

### 1. The newest run, with no pointer (US1, SC-001)

Build the tree above with no `lastRun.txt`, give the three **logs** distinct modification times,
call `FindRun(root)`.

**Expect** `Dir` is the newest of the three, and `Found` is `FoundByNewest`.

**Make it fail**, three ways, each of which caught a real defect:

- Stamp the *directories* instead of the logs and regenerate a report into the oldest run. Ordering
  on the directory hands back that old run, which is what US2 exists to prevent.
- Give all three logs the *same* time. One run must still come back, the same one every time.
- Give two runs **different simulation ids** at one shared time — `zzzsim-<January>` beside
  `aaasim-<September>`. Whole-name order is alphabetical by simulation id and returns January; only
  comparing the run id's own UTC stamp returns September.

### 2. The pointer wins over the clock (US2, SC-002)

Same tree, plus a `lastRun.txt` naming the **middle** directory, whose modification time is neither
newest nor oldest.

**Expect** the middle run, and `FoundByLastRun`.

**Make it fail**: delete `lastRun.txt` and re-run — the newest must come back with `FoundByNewest`.
If both runs return the same directory the test proves nothing; check the fixture's timestamps
before believing it.

### 3. A pointer to nothing falls through (US2)

Four variants of the same rule, and all four must reach `FoundByNewest`:

| `lastRun.txt` holds | Why it names nothing |
|---|---|
| a directory that was deleted | no such child |
| a directory with no `simulation.log` | not a run |
| `ExecutionError: java.lang.RuntimeException: boom` | not a name (contract 2) |
| `../../etc` or `/etc` or `sub/dir` | not a bare name (FR-007) |

**Make it fail**: point the escaping variant at a directory that *does* hold a `simulation.log`
outside the root. It must not be selected.

### 4. A path that is already a run (US3)

Call `FindRun` with the run directory, then with the `simulation.log` inside it.

**Expect** both return that run with `FoundByPath`, and identical `Dir` and `Log`.

**Make it fail**: put a newer sibling run next to it in the same parent. The named run must still
win — a path that names a run is never searched past (FR-003).

Also: a directory holding both a `simulation.log` **and** subdirectories that hold one is the run
itself, not a root (FR-005).

### 5. Failures name where they looked (US4, SC-005)

| Given | Expect |
|---|---|
| an empty directory | `*RunNotFoundError`, `Dir` is that directory, no location returned |
| a path that does not exist | `*RunNotFoundError` naming it |
| `FindRun("")` | `ErrNoPath`, before any filesystem access — even when a run *is* sitting in `target/gatling` |
| a regular file that is not a `simulation.log` | `*RunNotFoundError` naming it, not a raw `ENOTDIR` |
| a directory whose permissions forbid reading | the wrapped `*fs.PathError`, reachable with `errors.As`, and **not** a `*RunNotFoundError` |

**Make it fail**: the permissions case is the one that rots. A version that treats any read error as
"no runs here" passes every other scenario and turns a broken mount into a clean "not found"
(FR-012). Skip it when running as root, where the permission cannot be enforced.

### 6. No log is ever opened (SC-004)

Build a root whose `simulation.log` files hold bytes that are not a Gatling log at all — or are a
version far below the gate.

**Expect** `FindRun` resolves normally. The failure, if any, arrives later when the caller opens
`loc.Log`.

**Make it fail**: it is worth asserting positively rather than by absence — make the log unreadable
by permissions while keeping the directory listable. Discovery must not care.

### 7. The recording (integration)

Behind `-tags=integration`. It needs no Maven — the recording is committed — and it **fails** rather
than skips when that recording is missing, because a lost corpus entry is a broken checkout and the
constitution's e2e gate says an empty run fails:

```bash
go test -tags=integration ./gatling/
```

Asserts that the committed `lastRun.txt` recording parses to the run directories beside it —
the only place a real, build-written file meets this code. See research
[R9](research.md#r9--what-the-corpus-already-proves-and-the-one-thing-it-cannot) for why this
recording needs Maven and why the existing sbt corpus cannot supply it.

## The bound

```bash
go test -run '^$' -bench '^BenchmarkFindRun$' -benchmem ./gatling/
```

Over a synthetic root of 1000 runs, resolution must stay one directory read plus one `stat` per
entry, with allocations proportional to the entry count and not to anything inside a run
(research [R10](research.md#r10--no-decoder-benchmark-rule-applies-a-bound-is-stated-anyway)). There
is no throughput figure to hold: this feature decodes nothing.

## Fuzzing

`lastRun.txt` is the only parsed input, so it gets the target:

```bash
go test -run '^$' -fuzz '^FuzzLastRun$' -fuzztime 90s ./gatling/
```

Seeds: an empty file, a bare name, CRLF endings, several names, an `ExecutionError:` line, a name
with a separator, `..`, an absolute path, invalid UTF-8, and a file past the 64 KiB cap. The
property is that no input panics and none selects a directory outside the root.

## What this does not validate

Watching for new runs, deciding a run has ended, reading anything else in a run directory, and every
statistic — all out of scope (spec, *Out of Scope*). If a scenario here needs a `simulation.log` to
contain anything in particular, it has strayed.
