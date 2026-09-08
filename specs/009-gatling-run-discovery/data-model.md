# Data Model: Finding the run

**Feature**: 009-gatling-run-discovery | **Date**: 2026-09-08 | **Spec**: [spec.md](spec.md)

Nothing here enters `model/`. A located run is not a result: it is two paths and the reason they
were chosen, and it is discarded the moment the log is opened. `model.Run` remains the only Run this
module has (research [R5](research.md#r5--where-discovery-lives-and-what-it-is-called)).

## Entities

### RunLocation

Where a run's artefacts sit, and how they were found. A value type; no method needs a pointer.

| Field | Type | Meaning | Invariant |
|---|---|---|---|
| `Dir` | `string` | The run directory. | Never empty on success. Cleaned; absolute if the caller's path was, relative if it was not. |
| `Log` | `string` | The `simulation.log` inside `Dir`. | Never empty on success, and always `Dir` joined with `simulation.log`. |
| `Found` | `FoundBy` | Which rule selected it. | Never `FoundByUnknown` on success. |

`Log` is derivable from `Dir`, and is carried anyway: every caller wants it, and the alternative is
three consumers each joining the same constant — the duplication this feature exists to remove.

### FoundBy

Which of the three rules produced the location. An enum in the shape the package already uses:
`...Unknown` at iota 0, a `String()` method, following `Format`, `Kind`, `Status` and `Verdict`.

| Value | Meaning | Reached when |
|---|---|---|
| `FoundByUnknown` | Zero value; never returned. | — |
| `FoundByPath` | The caller named the run. | The path is a `simulation.log`, or a directory holding one. |
| `FoundByLastRun` | Gatling's own record named it. | A `lastRun.txt` line named an existing direct child holding a `simulation.log`. |
| `FoundByNewest` | Selected as the most recent. | No usable `lastRun.txt` line; the newest run in the root by [R6](research.md#r6--the-tie-break-is-load-bearing-not-a-formality)'s ordering. |

`FoundByNewest` is the ordinary outcome for Gradle and sbt users and for any Maven build past
`gatling:verify` (research [R2](research.md#r2--it-is-also-short-lived-so-modification-time-is-the-ordinary-path)),
which is why FR-009 exists: a consumer that says which run it is reading should be able to say it
was a guess.

### RunNotFoundError

The search completed and there was no run. Distinct from a directory that could not be read
(FR-012), which is the underlying `*fs.PathError` wrapped with the path — a failure to look is not
an absence of runs.

| Field | Type | Meaning |
|---|---|---|
| `Dir` | `string` | The directory that was searched. Never empty (FR-011). |
| `Default` | `bool` | True when `Dir` is the default results root rather than one the caller named. |

`Default` is not decoration. A server-side consumer with no working directory gets a relative path
it never chose; the flag is what lets the message say where the path came from instead of only what
it was.

## Resolution

One pass, no recursion, no backtracking. `simulation.log` is the only thing that makes a directory a
run — a report without one is not a run, and a run without a report still is (Gatling stopped
producing reports in 3.13.5).

```
FindRun(path):
    root, isDefault := path, false
    if path == "":
        root, isDefault = "target/gatling", true          # R3

    # 1. Is this already a run? (FR-002, FR-003, FR-004, FR-005)
    switch stat(root):
        case missing:      return RunNotFoundError{root, isDefault}
        case regular file: return run(dir(root)) if base(root) == "simulation.log"
                           else  RunNotFoundError{dir(root), isDefault}
        case directory:    if exists(root/simulation.log):
                               return RunLocation{root, root/simulation.log, FoundByPath}
                           # otherwise it is a results root; fall through

    # 2. Candidates: direct children only, one level deep  (Assumptions)
    entries := readdir(root)                              # error -> wrap, do not swallow (FR-012)
    candidates := [e for e in entries
                   if isDir(e) and exists(root/e/simulation.log)]

    # 3. Gatling's own record, if there is one  (FR-006, FR-007, R7)
    named := [line for line in lines(root/lastRun.txt)    # absent -> skip, R2
              if isBareName(line) and line in candidates]
    if named is not empty:
        return location(newest(named), FoundByLastRun)

    # 4. The newest run  (FR-002, FR-008, R6)
    if candidates is empty:
        return RunNotFoundError{root, isDefault}
    return location(newest(candidates), FoundByNewest)

newest(xs):     max by (modification time, then name), both descending      # R6
isBareName(s):  s == base(s) and s not in {"", ".", ".."} and no separator  # R8
lines(p):       UTF-8, split on LF, each line stripped of a trailing CR and
                surrounding whitespace, blanks dropped                      # R1
```

Step 3 sits **after** step 2 deliberately. `lastRun.txt` names candidates rather than paths, so the
candidate set is what validates a line: a name that is not among them — a deleted run, a directory
with no log, or an `ExecutionError:` message — falls out with no special case (FR-006, R7).

## State transitions

None. `FindRun` is a pure question about the filesystem at one instant, with no handle, no cursor
and nothing to close. A run that appears, is deleted or is rewritten after it returns is not this
function's concern; noticing that is the sidecar's (spec, *Out of Scope*).

## Validation rules

| Rule | Source | Where it bites |
|---|---|---|
| A directory is a run iff it directly contains `simulation.log`. | FR-005, FR-013 | Steps 1 and 2. Never opened, only `stat`-ed. |
| A path naming a run is never searched past. | FR-003 | Step 1 returns; steps 2–4 do not run. |
| A path naming neither run nor root fails; it does not fall back to the default. | FR-003 | Step 1's `missing` case. |
| A `lastRun.txt` line is followed only if it is a bare name. | FR-007, R8 | `isBareName`. |
| A stale, logless or error line is a pointer to nothing, not a failure. | FR-006, R7 | Membership in `candidates`. |
| Ordering is total and platform-independent. | FR-008, R6 | `newest`. |
| An unreadable directory is a failure, not an absence. | FR-012 | Step 2's error path. |
| No `simulation.log` is opened. | FR-013, SC-004 | Every step. |
| No version gate runs. | FR-014 | Every step — the gate belongs to the codec. |
