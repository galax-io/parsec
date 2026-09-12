# Contract 3 — what an item with no recorded end does to the span

**Applies to**: `model.Bounds`, `Bounds.Extend`, `Bounds.Start`, `Bounds.End`.
**Status**: a frozen primitive's definition, and the one number this feature moves. Issue
[#103](https://github.com/galax-io/parsec/issues/103); recorded under **Changed**.

## The rule

An item the fold counted, whose end the source did not record, is known to have been running at its
own start instant. It extends the run's end to that instant.

Therefore: **`End()`, when it reports a value, is never earlier than the start of any item the fold
counted.**

## Why

`model.Bounds` is what galaxio-cli#61 makes the source of every run's bounds, and every rate a report
prints divides by this span. Today a fold of

| item | start | end |
|---|---|---|
| sample A | 00:00:10 | 00:00:15 |
| sample B | 00:00:20 | not recorded |

reports `Start() = 00:00:10` and `End() = 00:00:15` — a span that ends before an item the same fold
counted began. A consumer divides a count including sample B by an interval that excludes it. The
type's own `unplaced` rationale rejects exactly this shape: "a span too short and a rate too high,
with nothing to say so."

Gatling's own arithmetic treats a request that never completed as occupying its start instant, and
`coverUser` already does the same for a virtual-user START (`begin(u.At); finish(u.At)`). The change
makes `cover` consistent with its neighbour rather than inventing a rule.

## What changes in the code

`model/bounds.go`:

- `cover` calls `finish(start)` unconditionally, then extends by the duration when one was recorded
  and is not negative.
- `End()` loses the `b.end.Before(b.start)` guard, which becomes unreachable: every path that calls
  `begin(x)` now also calls `finish(≥x)`, and `finish` only raises, so `end ≥ max(begun) ≥ start`.
- The type's doc paragraph "a sample or group end that is absent — such an item still contributes its
  start" is replaced by the rule above. The paragraph explaining the crossing goes with the guard.

## What does not change

- An item whose start is the zero time still sets `unplaced`, and `Start()` and `End()` still report
  nothing. Its end is unreachable from here, and a span that silently excluded it would be the same
  dishonesty one step further on.
- A run with zero items still reports nothing from both.
- `UserStart` and `UserEnd` move the bounds exactly as they do today (#103's non-goal).
- A negative duration still contributes no end past the start.

## Acceptance

| Fold | `Start()` | `End()` |
|---|---|---|
| sample at 00:00:10 lasting 5 s; sample at 00:00:20 with no end | 00:00:10, true | **00:00:20**, true |
| any fold | — | never earlier than the start of any item it counted |
| an item with a zero start | nothing, false | nothing, false |
| nothing folded | nothing, false | nothing, false |

Pinned by a test with a start-only item later than every recorded end, and recorded under **Changed**
in `CHANGELOG.md`.
