# Contract 2 — `PARSEC_CANARY_RUNS`

**Status**: existing, extended. Backward compatible — every value that works today still works.

## Shape

```
PARSEC_CANARY_RUNS="<version>=<run dir>;<version>=<run dir>;…"
```

One `version=dir` pair per fresh run, separated by `;`. `dir` is a Gatling run directory —
the one holding `simulation.log`. Already documented in `AGENTS.md` under *canary*.

## Behaviour

| Condition | Outcome |
|---|---|
| unset | every canary test **skips with a reason**. The constitution requires a test needing a real tool to say so rather than fake it. |
| set, entry not `version=dir` | the test **fails** naming the malformed entry |
| set, version unparseable | the test **fails** naming it |
| set and well-formed | each pair is decoded and held to its own report |

A skip can never pass for a run: the workflow counts passing tests and fails the job when the count
is zero. That check exists today and is unchanged.

## What is extended

**Both codecs read the same variable.** Today only `gatling/text` does. `gatling/binary` gains its
own canary reading the identical format, so one exported variable drives both and a maintainer
learns one thing.

**Runs of both formats may appear in one value.** The cross-format comparison
(spec FR-003) needs runs from either side of 3.13.0 in a single invocation:

```
PARSEC_CANARY_RUNS="3.12.0=/runs/v3.12.0;3.15.1=/runs/v3.15.1"
```

Each codec's canary selects the runs whose version its own gate accepts and ignores the rest, so
neither fails on a run meant for the other.

## Which tests read it

| Test | Package | Asserts |
|---|---|---|
| `TestCanary` | `text`, `binary` | each fresh run matches the account that run gave of itself |
| `TestCanaryCrossVersion` | `text`, `binary` | fresh runs of the same probe agree as multisets, timing/identity/order set aside |
| `TestCanaryCoversSupportedRange` | `text`, `binary` | both bounds of that codec's `SupportedVersions()` were among the runs |
| `TestCanaryCrossFormat` | `binary` (imports `text`) | a text run and a binary run of the same probe agree, group-name spelling normalised |

`TestCanaryCoversSupportedRange` is what stops the gate and the canary drifting apart: widening a
range without recording and running the new bound fails this test (spec FR-002).

## Local use

```bash
PARSEC_CANARY_RUNS="3.15.1=/path/to/run" go test -tags=canary -race -count=1 ./gatling/binary/
```

See [quickstart.md](../quickstart.md) for producing a run to point it at.
