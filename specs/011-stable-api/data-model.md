# Data Model: A stable API

**Feature**: 011-stable-api | **Date**: 2026-09-12

This feature adds no type and no field. What follows is what changes about types that already exist,
and the one artefact that is new — the surface inventory, which is data about the module rather than
data the module carries.

---

## 1. The surface inventory (new artefact)

**Where**: `testdata/api/surface.txt`, generated and asserted by `api_test.go` at the module root.

**Shape**: one line per exported identifier, grouped by package, sorted within a package, each line a
kind and a name:

```
## model  (111)
    const FieldConnectTiming
    field Bounds.…            (none — Bounds has no exported field)
    func Some
    imethod …
    method Bounds.End
    type Bounds
```

Kinds: `const`, `var`, `type`, `field <Type>.<Name>`, `method <Type>.<Name>`,
`imethod <Interface>.<Name>`, `func <Name>`.

**Rules**

| Rule | Why |
|---|---|
| Test files are excluded | `_test.go` declares nothing a consumer can import |
| A method on an unexported receiver is excluded | it is unreachable from outside |
| Struct fields and interface methods are counted | they are the surface a consumer reads and, for the interfaces, implements |
| The count per package is part of the file | a diff that changes only a count is still a diff |

**Counts**: 278 before this feature (`model` 111, `gatling` 107, `text` 14, `binary` 14, `simlog` 16,
`run` 16), verified against the tree; 275 at the tag (research [R1](./research.md#r1--how-the-contract-table-is-produced)).

**Relationship to `gorelease`**: complementary, not overlapping. `gorelease` decides whether a change
is *compatible*; this file decides whether the surface is *what the contract says*. An addition
passes the first and fails the second, which is the point.

---

## 2. Enums with a `String()` (eleven, changed rendering)

**Entity**: an exported integer type with a closed set of named values, a zero value meaning
"unknown", and a `String()`.

**Invariants after this feature**

| Invariant | Before | After |
|---|---|---|
| Zero value renders as its own name | `unknown` on all eleven | unchanged |
| In-range value renders as its documented name | yes | unchanged |
| Out-of-range value renders | `Type(N)` on six, `unknown` on five | `Type(N)` on all eleven |
| Implementation | table lookup (4), switch with `default` (3), switch falling through (4) | table lookup (11) |

**The eleven**: `gatling.Kind`, `gatling.Format`, `gatling.Verdict`, `gatling.Status`,
`gatling.Event`, `run.FoundBy`, `model.Outcome`, `model.ItemKind`, `model.PositionKind`,
`model.UserEventKind`, `model.Field`.

**Not enums, not touched**: `gatling.Version`, `gatling.Warning`, `model.Warning`, `model.Position` —
each has a `String()` and none has a closed value set.

---

## 3. `model.Bounds` (changed definition)

**Entity**: the span of a run, folded from items; two instants and an `unplaced` flag.

**State transitions** — what each folded item does:

| Item | `Start()` | `End()` before | `End()` after |
|---|---|---|---|
| sample or group with start and duration | extends to min | extends to start+duration | unchanged |
| sample or group with start, **no recorded end** | extends to min | **nothing** | extends to its own start |
| sample or group with a zero start | sets `unplaced` | sets `unplaced` | unchanged |
| virtual-user START | extends to min | extends to its instant | unchanged |
| virtual-user END | — | extends to its instant | unchanged |
| error, assertion, unknown | — | — | unchanged |

**Invariant gained**: `End()`, when it reports a value, is never earlier than the start of any item
the fold counted. Every path that calls `begin(x)` now also calls `finish(≥x)`, and `finish` only
raises — so `end ≥ max(begun) ≥ min(begun) = start`. `coverUser`'s `UserStart` already did exactly
this (`begin(u.At); finish(u.At)`), so the change makes `cover` consistent with its neighbour rather
than inventing a rule.

**Consequence**: the `b.end.Before(b.start)` guard in `End()` becomes unreachable and is removed,
together with the doc paragraph that explains it.

**Worked example**, verified against the tree today and asserted by a test after:

| | today | after |
|---|---|---|
| sample at 00:00:10, duration 5 s | | |
| sample at 00:00:20, no end | | |
| `Start()` | 00:00:10, true | 00:00:10, true |
| `End()` | **00:00:15**, true | **00:00:20**, true |

---

## 4. The error types (one changed, the rest frozen as they are)

| Type | Means | Change |
|---|---|---|
| `FormatError` | these bytes are not a Gatling simulation.log | none |
| `UnsupportedFormatError` | a Gatling log this reader does not decode | **message reworded; gains producers in both codecs** |
| `SyntaxError` | a log of this format, damaged at the position named | none — `Line`, `Offset`, `Format` confirmed as three fields |
| `TruncationError` | a log of this format, cut short | none — one duplicated doc paragraph removed |
| `VersionError` | a version below the supported range | none |
| `UnverifiedError` | a version above the range, under strict mode | none |

**The discrimination a consumer writes**, after this feature:

```
*FormatError             -> not a Gatling log at all
*UnsupportedFormatError  -> a Gatling log; this reader does not decode it (Format says which)
*SyntaxError             -> a Gatling log of this reader's format, damaged
*TruncationError         -> a Gatling log of this reader's format, cut short
*VersionError            -> refused by the gate
*UnverifiedError         -> above the range under WithStrict
```

Today the second row is empty for every real input and the third row absorbs it, which is the defect
#84 names.

---

## 5. The two `Warning` types (unchanged shape, linked)

| | `gatling.Warning` | `model.Warning` |
|---|---|---|
| Fields | `Version Version`, `Min, Max Version` | `Version string`, `Reason string` |
| Ordering | `Version.Compare` | none — text, deliberately |
| Reached through | `simlog.NewReader` → `Warnings()` | `simlog.NewRunReader` → `Run().Warnings` |
| Change | doc comment names the model type as the canonical form | doc comment names the Gatling type and says why `Version` is text |

Neither shape changes; neither is renamed. The crossing is one `simlog` call, permanently.

---

## 6. `simlog.RecordReader` and `simlog.RunReader` (frozen two-sided)

| Interface | Methods | Freeze |
|---|---|---|
| `RecordReader` | `Header`, `Assertions`, `Warnings`, `Next` | final at the tag |
| `RunReader` | `Run`, `Next` | final at the tag |

Consumers' test doubles implement these, so an added method after v0.1.0 breaks the implementer, not
only the caller. Each gains the single-goroutine sentence beside the aliasing rule it already carries.

---

## 7. Identifiers withdrawn

| Identifier | Where it goes | Only callers today |
|---|---|---|
| `gatling.Gate` | unexported `gate` | `gatling/policy.go:41`, `gatling/version_test.go:118` |
| `gatling.MaxRunStart` | `internal/wire.MaxRunStart` | `gatling/binary/record.go:147`, `gatling/text/parse.go:483,487` |
| `text.Tool` | replaced by `gatling.Tool` | `gatling/text/model.go` itself |
| `binary.Tool` | replaced by `gatling.Tool` | `gatling/binary/model.go` itself |

**The arithmetic**: 278 today, minus `Gate`, `MaxRunStart` and the two codec `Tool`s, plus one
`gatling.Tool` = **275**. `internal/wire.MaxRunStart` is not counted: `internal/` is not public
surface. 275 is the figure the generator must report at the tag, and the golden file
`testdata/api/surface.txt` is what binds — if the generator says otherwise, the generator is right
and this number is corrected.
