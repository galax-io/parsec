# Contract 2 — the `lastRun.txt` contract

**Status**: an assumption about someone else's file, recorded so it can be re-checked rather than
rediscovered. Read from `gatling-maven-plugin` **4.21.10** on 2026-09-08; the constant and the
writing method are unchanged in every version from 3.0.5 to 4.21.10 present on that machine.

This is not a format this module decodes. It is a hint, and every rule below exists so that a wrong
or stale hint costs nothing.

## Who writes it

`gatling-maven-plugin`, and nothing else — not `gatling-core`, not the Gradle plugin. See research
[R1](../research.md#r1--lastruntxt-is-written-by-the-maven-plugin-not-by-gatling) and
[R2](../research.md#r2--it-is-also-short-lived-so-modification-time-is-the-ordinary-path) for the
bytecode this was read from.

| | |
|---|---|
| **Location** | `<resultsFolder>/lastRun.txt`, where `resultsFolder` defaults to `${project.build.directory}/gatling` |
| **Written by** | `GatlingMojo.saveSimulationResultToFile`, at the end of a `gatling:test` execution |
| **Deleted by** | `VerifyMojo.verifyLastRun`, during `gatling:verify` — so it does not survive an ordinary Maven build |
| **Not written by** | the Gradle plugin (verified), Gatling itself (verified), sbt (assumed — the plugin was not available to check) |

## What it holds

One line per run directory that appeared during the execution, written as `File.getName()`, followed
by an optional error line when the run failed.

```text
corpussimulation-20260906044741110
```

```text
myfirstsimulation-20260906044741110
mysecondsimulation-20260906044803343
ExecutionError: java.lang.RuntimeException: something failed | ...
```

| Property | Value | Why it matters here |
|---|---|---|
| Line content | a bare directory name | never a path, so any line containing a separator is not something this plugin wrote |
| Line count | one per new run directory | `runMultipleSimulations` produces several; a single-line reader would be wrong for those builds |
| Extra line | an error message | `ExecutionError: ...`, or `Gatling simulation assertions failed!` — not a directory name |
| Encoding | UTF-8 | `Files.newBufferedWriter` with no charset argument |
| Line ending | `System.lineSeparator()` | CRLF on Windows, so a trailing CR must be stripped |

## What this module promises about reading it

1. **It is a hint, never an instruction.** A line is used only if it names an existing direct child
   of the results root that holds a `simulation.log`. Everything else — a deleted run, a directory
   with no log, an error message, a blank line — is a pointer to nothing and costs a `stat`.
2. **No error text is matched.** `ExecutionError:` is never searched for. An error line fails the
   existence check like any other non-name, so the plugin can reword its messages freely
   (research [R7](../research.md#r7--a-multi-line-lastruntxt-needs-no-special-case)).
3. **Nothing outside the root is followed.** A line is used only if it is a bare name: equal to its
   own base, not `.` or `..`, and free of separators. This is a guard against a stale or garbage
   file, **not a security boundary** — the file is written by the caller's own build into the
   caller's own results root, and anyone who can write it can write the run directories beside it.
   A bare name that is a symlink is followed, deliberately: an archive that stores runs elsewhere and
   links them in is a shape the spec's edge cases require to work
   (research [R8](../research.md#r8--containment-is-textual-so-a-symlinked-run-still-resolves)).
4. **Absence is normal, not an error.** No `lastRun.txt` means the modification-time rule, which is
   what Gradle and sbt users get always and Maven users get after `gatling:verify`.
5. **It is never written, moved or deleted.** Discovery only reads. `VerifyMojo` deletes this file
   and would be racing anything that did otherwise.
6. **Several usable lines resolve like several directories.** The newest wins, by the same ordering
   the root scan uses — so a multi-simulation build yields the run that finished last.

## What would break this, and what happens then

| Change upstream | Effect here |
|---|---|
| The plugin writes paths instead of names | Every line fails `isBareName`; discovery falls back to modification time. Degrades, does not break. |
| The plugin changes the error-line wording | Nothing. No error text is matched. |
| The plugin stops deleting the file in `verify` | `FoundByLastRun` becomes common where `FoundByNewest` is today. No code change. |
| The Gradle or sbt plugin starts writing one | It is read, with no change, provided the lines are bare names. |
| The file grows large | It is read with a cap; a `lastRun.txt` over 64 KiB is treated as absent rather than loaded. |

Re-read this contract when a Gatling plugin major version lands, and record the version checked in
the header above.
