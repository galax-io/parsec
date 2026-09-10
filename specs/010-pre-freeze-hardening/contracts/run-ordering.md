# Contract 3 — The run ordering

**Applies to**: `run.Find` when it selects among candidates — the newest run in a results root, or
the newest among those `lastRun.txt` names — and `run.FoundByNewest`'s documentation.
**Status**: the signals and their precedence are those documented since v0.0.9; this feature makes
the comparison a total order. Issue #88.

## The rule

Candidates are compared by **one key**, lexicographically:

1. the modification time of the run's `simulation.log` — the later wins;
2. the 17-digit UTC stamp (`yyyyMMddHHmmssSSS`) the run id ends with, compared as text, or the
   empty string for a name that carries none — the greater wins, so **a stamped name outranks an
   unstamped one**;
3. the directory name — the greater wins.

The result does not depend on the order the directory was read in, on which other directories are
present, or on the platform.

## Examples, all at one modification time

| Root | Returns | Today |
|---|---|---|
| `simA-20990101000000000`, `simB-20200101000000000` | `simA-2099…` | same |
| `simA-20990101000000000`, `simB-20200101000000000`, `simAA` | `simA-2099…` | `simB-2020…` (#88) |
| `simAA`, `simA-20990101000000000` | `simA-2099…` | `simAA` |
| `archived-run`, `another-run` | `another-run` | same |
| `zzzsimulation-20260101…`, `aaasimulation-20260909…` | `aaasimulation-2026…` | same |

## What changes for a consumer, in `CHANGELOG.md` terms

**Changed**

- `run.Find` orders candidates by one key — the log's modification time, then the run id's UTC
  stamp, then the name — so the run it returns no longer depends on the order the results root was
  listed in. A name without a stamp now ranks below every name with one at the same modification
  time, where the pair was previously compared by whole name; only a root mixing stamped and
  unstamped directories can see a different answer, and on such a root the previous answer could
  change when an unrelated directory was added. (#88)

## Verified by

`TestFindMixedStampedAndUnstampedNames` (`gatling/run`, external) and the permutation and shuffle
properties in `order_test.go` (`gatling/run`, internal). See [quickstart.md](../quickstart.md) §5.
