# Contract 2 — how an exported enum renders

**Applies to**: `gatling.Kind`, `gatling.Format`, `gatling.Verdict`, `gatling.Status`,
`gatling.Event`, `gatling/run.FoundBy`, `model.Outcome`, `model.ItemKind`, `model.PositionKind`,
`model.UserEventKind`, `model.Field` — the eleven exported types with a closed value set and a
`String()`.
**Status**: observable behaviour that freezes at v0.1.0. Five renderings change. Issue
[#78](https://github.com/galax-io/parsec/issues/78); recorded under **Changed**.

## The rule

One rule, on all eleven:

| Value | Renders as |
|---|---|
| the zero value | its own documented name — `"unknown"` on all eleven |
| any value in the known set | its own documented name, unchanged |
| **any other value** | **the type's name and the number**: `Outcome(99)`, `Kind(99)`, `Field(9999)` |

This takes the `gatling` convention everywhere. A value the module cannot name stops rendering
identically to a value that means the source lost this — `model.Outcome`'s own doc comment says the
zero value marks a sample that "lost its outcome on the way rather than succeeding quietly", and the
old rendering discarded that distinction for anything out of range. The number survives a `%v` into a
log, and for a value that can only arrive by a consumer casting an integer, the number is the one
useful thing to print.

## The one idiom

A package-level table indexed by the enum, with a bounds check:

```go
var outcomeNames = [...]string{unknownName, "success", "failure"}

// String returns "success", "failure", or "unknown" for the zero value. A value
// outside the known set renders as the type name and the number, as every
// exported enum in this module does.
func (o Outcome) String() string {
	if int(o) < len(outcomeNames) {
		return outcomeNames[o]
	}

	return "Outcome(" + strconv.Itoa(int(o)) + ")"
}
```

Four of the eleven already do this — `Kind`, `Format`, `FoundBy` and `Field`. The other seven are a
`switch` with a `default` (three) or a `switch` falling through to `unknownName` (four), which is
three idioms for one job. Principle VI asks that the convention already in the codebase be followed;
this is it, and `model.Field`'s table already carries the argument for a table — a value added
without a name there is the empty string and fails an existing test rather than printing as a number
in a report.

`model.Field` keeps its `>= fieldCount` form, which is the same check written against its sentinel.

## What does not change

- `unknownName` stays, in all three packages that declare it (`model/capability.go:8`,
  `gatling/record.go:25`, `gatling/run/doc.go:22`). Each is unexported and package-local.
- `gatling.Version`, `gatling.Warning`, `model.Warning` and `model.Position` have `String()` methods
  and are not enums. Untouched.
- No in-range rendering changes. No `stringer`: a build-time dependency Principle IV does not admit
  for this (#78's own non-goal).

## Acceptance

- A table walking all eleven at a value outside the known set shows the type name and the number.
- A table walking all eleven at the zero value shows each one's own documented name.
- Every affected `String()` doc comment states the rule.
