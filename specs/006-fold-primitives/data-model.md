# Data Model: The Primitives a Consumer Folds

**Feature**: `006-fold-primitives` | **Date**: 2026-09-06 | **Spec**: [spec.md](./spec.md)

What the two new values hold, what the fixes change in the values that already exist, and the
invariants a test can pin. The API surface with its doc comments is in
[contracts/model-fold.md](./contracts/model-fold.md) and
[contracts/gatling-fixes.md](./contracts/gatling-fixes.md); the reasons are in
[research.md](./research.md).

## 1. Position

A position addresses either a sample or a group traversal. It is one comparable value.

```text
Position
  key  string   (unexported; the identity)

key   := kind ‖ seg(groups[0]) ‖ … ‖ seg(groups[n-1]) ‖ [ seg(name) ]   — name only for a sample
kind  := 0x01 for a sample | 0x02 for a group traversal
seg(s):= uvarint(len(s)) ‖ bytes(s)
```

| Property | Holds because |
|---|---|
| `p == q` exactly when same kind, same path, same name (FR-002) | the key is a deterministic function of the three and nothing else |
| distinct paths never collide, whatever the names contain (FR-003) | every segment is length-prefixed, so the encoding is prefix-free and needs no reserved byte |
| a sample and a group traversal never collide (FR-004) | the kind byte differs even when the segments are identical |
| the path and the name decode back exactly (FR-005) | lengths are stored, so decoding walks them; nothing is escaped or lost |
| valid after the reader advances (FR-006) | building the key copies the bytes; the reader's reused `Groups` array is never referenced |
| a map key, allocation-free to look up | a struct of one string compares and hashes as the string |

**Zero value**: `Position{}` has an empty key. `Kind()` is `PositionUnknown`, `Groups()` is nil,
`Name()` is empty, `String()` is empty. No constructor produces it — even a sample with no groups
and an empty name encodes to two bytes — so it never equals a real position and can serve as
"no position".

**`Groups()`** returns a fresh slice the caller owns: empty and non-nil for a constructed position
with no groups, nil only for the zero value — the same rule the codecs apply to `Sample.Groups`.

**`String()`** renders the segments and the name joined by ` / ` for display. A name containing
that separator renders ambiguously; that is acceptable, because the rendering is not the identity
(FR-007).

**Cost**: one allocation per constructed position, proportional to the total length of its names.
Measured against the goal in [research.md R12](./research.md).

## 2. Bounds

```text
Bounds
  start  time.Time   (zero = unset)
  end    time.Time   (zero = unset)
```

The zero value is "nothing counted yet". `Extend` folds one item; `Start` and `End` report
`(instant, true)` or `(zero, false)`.

| Item kind | Moves the start with | Moves the end with | Ignored when |
|---|---|---|---|
| `ItemSample` | `Sample.Start` | `Sample.Start + Sample.Duration` | start is zero (whole item); duration unset (end only) |
| `ItemGroup` | `Group.Start` | `Group.Start + Group.Duration` — the wall clock, never `CumulatedDuration` | same |
| `ItemUser`, `UserStart` | `User.At` | `User.At` | `At` is zero |
| `ItemUser`, `UserEnd` | — | `User.At` | `At` is zero |
| `ItemUser`, `UserEventUnknown` | — | — | always: no adapter produces it |
| `ItemError` | — | — | always (spec 002 FR-021c: errors do not bound the run) |
| `ItemAssertion`, `ItemUnknown` | — | — | always |

"Moves" means minimum for the start and maximum for the end, so the fold is order-independent
(FR-012). The run's recorded start (`Run.Start`) is never offered to `Extend` and never counts
(FR-009).

**States**: unset → start only → both, or unset → end only (a run of user ENDs alone) → both. A
half-set state is legal and reported honestly; nothing invents the missing end.

**Cost**: no allocation; a handful of comparisons per item.

## 3. Absent instants

Three layers, one rule at each.

| Layer | Representation of "the source could not resolve this time" | Set by |
|---|---|---|
| wire, `gatling.Record` (`Start`, `End`, `Timestamp`) | `gatling.AbsentTimestamp` (= `math.MinInt64`) | both codecs: a negative offset in binary; a negative value in text; Gatling's own never-completed end, which is this very value |
| model (`Sample.Start`, `GroupSample.Start`, `UserEvent.At`, `RunError.At`) | the zero `time.Time`; `IsZero()` is true | `internal/wire.Millis`, which maps every negative millisecond count to the zero time |
| model (`Sample.Duration`, `GroupSample.Duration`) | unset `Opt` | `internal/wire.span`, which reports no duration when either end is the sentinel, or the end precedes the start, or the span overflows |

**Invariant**: a recorded instant is never the zero time. Every recorded value is at or after
1970-01-01 (a non-negative millisecond count), and the zero time is the year 1. Tests pin it from
both sides: a negative wire value becomes the zero time, and zero milliseconds becomes 1970.

## 4. What the fixes change in the wire records (#56)

| Field, text codec | Today | After |
|---|---|---|
| request `Start`, group `Start`, user `Timestamp`, error `Timestamp`: `-` then digits | `*SyntaxError` "expected a non-negative integer …" | `AbsentTimestamp`, no error |
| request `End`, group `End`: `-` then digits | error unless exactly `-9223372036854775808` | `AbsentTimestamp`, no error — the sentinel is one case of the rule |
| group `CumulatedResponseTime`: `-` then digits | error | the negative value, verbatim (the model reports it unset) |
| any of the above: not digits, or digits that overflow `int64` | error | error, unchanged — inputs the binary format cannot express |
| `Groups` of the first group-less record | empty, non-nil (verified by probe) | unchanged; pinned by the agreement test |

The binary codec changes only in naming `gatling.AbsentTimestamp` where it had a private constant.
Every golden record stream stays byte-identical: no corpus run carries a negative value.

## 5. Ceilings (#57)

| Constant | Bounds | Value | Justification |
|---|---|---|---|
| `maxGroupDepth` | the nesting depth a request or group record claims | 1024 (unchanged) | Gatling's DSL nests far below it |
| `maxScenarios` | the scenario names a run record declares | 1 << 16 | a simulation declares its scenarios in code, a few at most; the ceiling caps a `[]string` header allocation at 1 MiB |
| `maxAssertions` | the assertion payloads a run record carries | 1 << 16 | a generated suite may run to thousands; the spec's floor is 2,000; same 1 MiB cap |
| `initialPathCap` | the first capacity of the reused group-path scratch | 8 | a sizing hint, not a ceiling; today spelled `maxGroupDepth/128` |

A count above its ceiling is refused at its own offset with the message the reader uses today,
`expected <the count>, found a count of N`. `MaxStringLen` (8 MiB per string or payload) is not
touched.

## 6. The exported-surface golden (FR-018)

`model/testdata/exports.golden`: one identifier per line, sorted, in the forms

```text
const FieldSampleDuration
func NewSamplePosition
method Bounds.Extend
type Position
field Sample.Start
```

Built by `go/parser` over `model/*.go` excluding `_test.go`. The test fails on any difference;
`go test ./model/ -run TestExportedSurfaceIsGolden -update` rewrites the file, and the diff is what a
reviewer approves.

## 7. The verification tallies (FR-032, FR-033)

| Tally | Package | Keyed by | Bounded by | Held to |
|---|---|---|---|---|
| `decodeTally` (exists) | `gatling/text` | `statsKey{path joined by ",", name}` from wire records | `injectStart`/`injectEnd` by hand | `global_stats.json`, `stats.json` |
| `modelTally` (exists) | `gatling/text` | same, from model items | same, from model items | same |
| primitive tally (new) | `gatling/text` | `Position` | `Bounds` | equality with `modelTally`, then the same report checks |
| `counts` (exists) | `gatling/binary` | request name and group path | — | `index.html` rows, `console.txt` request count |
| primitive tally (new) | `gatling/binary` | `Position` | `Bounds` | the same rows, plus `console.txt` `mean throughput (rps)` from the bounds rounded up to whole seconds |

Mapping the new tally onto the old one for the equality check: `statsKey{path: strings.Join(p.Groups(), ","), name: p.Name()}` — the join is lossy in general and exact on this corpus, which is precisely why the primitive exists.
