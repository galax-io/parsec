# Contract: the fold primitives in `model`, and the shared sentinel in `gatling`

**Feature**: `006-fold-primitives` | **Issue**: [#8](https://github.com/galax-io/parsec/issues/8) (and the model half of [#56](https://github.com/galax-io/parsec/issues/56)) | **Spec**: [spec.md](../spec.md)

Every identifier below is additive except where §4 says otherwise. Doc comments are the ones the
code will carry; a change to them is a change to this contract. Pre-v0.1.0, so nothing here needs a
deprecation window, and every addition is recorded under Added.

## 1. `model.Position`

```go
// PositionKind says what a Position addresses.
type PositionKind uint8

const (
	// PositionUnknown is the zero value. The zero Position addresses nothing,
	// and no constructor produces it.
	PositionUnknown PositionKind = iota
	// PositionSample addresses a sample: a group path and a name.
	PositionSample
	// PositionGroup addresses a group traversal: a group path alone, whose
	// last element is the group's own name.
	PositionGroup
)

// String returns "sample", "group" or "unknown".
func (k PositionKind) String() string

// Position is where in a run something was recorded: the ordered path of
// enclosing groups and, for a sample, its name. It is the definition every
// consumer buckets by, so that two consumers agree on which rows a run has
// without agreeing on a spelling.
//
// It is one comparable value: use it directly as a map key. Two positions are
// equal exactly when they address the same kind of thing at the same path with
// the same name — a group traversal never equals a sample, even where the
// group's path reads as the sample's path plus its name — and distinct paths
// never collide, whatever characters the names contain.
//
// A Position taken from an item stays valid after the reader advances. The
// Groups slice on a Sample or GroupSample is backed by storage the reader
// reuses; a Position is not, so it may be kept for the whole run without
// copying anything.
//
// The zero Position addresses nothing and equals no position a constructor
// returns.
type Position struct {
	// key encodes the kind, every group name and the sample name, each length-
	// prefixed. It is the identity and it is internal: nothing exported reads
	// it, so the encoding may change.
	key string
}

// NewSamplePosition returns the position of a sample recorded under the given
// groups, outermost first, with the given name. groups may be empty or nil for
// a sample outside any group; the slice is not retained.
func NewSamplePosition(groups []string, name string) Position

// NewGroupPosition returns the position of a group traversal at the given path,
// outermost first, the group's own name last. The slice is not retained.
func NewGroupPosition(groups []string) Position

// Kind reports whether the position addresses a sample or a group traversal.
func (p Position) Kind() PositionKind

// Groups returns the ordered path of enclosing groups the position was made
// from — for a group traversal, ending with the group's own name. The slice is
// the caller's own: empty and non-nil for a position with no groups, nil only
// for the zero Position.
func (p Position) Groups() []string

// Name returns the sample's name. It is empty for a group traversal and for the
// zero Position; a sample may also legitimately have an empty name, and Kind
// tells the two apart.
func (p Position) Name() string

// String renders the position for display, as the groups and the name joined
// by " / ". It is not the identity: a name containing the separator renders
// ambiguously and still compares correctly. The zero Position renders empty.
func (p Position) String() string
```

On the existing types:

```go
// Position returns where the sample was recorded: its groups and its name, as
// one value a consumer buckets by. Unlike Groups, the result stays valid after
// the next call to Next.
func (s Sample) Position() Position

// Position returns where the traversal was recorded: its path, the group's own
// name last, as one value a consumer buckets by. Unlike Groups, the result
// stays valid after the next call to Next.
func (g GroupSample) Position() Position
```

## 2. `model.Bounds`

```go
// Bounds is where a run begins and where it ends, as the tool's own report
// bounds it: the earliest of every sample start, group start and virtual-user
// START, and the latest of every sample end, group end and virtual-user event.
// Every rate a report prints divides by this span, and a virtual user can set
// either end of it, which is why it is a definition this package owns rather
// than something each consumer derives.
//
// A consumer folds the stream once and calls Extend on every item; nothing is
// retained but the two instants. The zero Bounds has counted nothing and is
// ready to use. The fold is a minimum and a maximum, so the order of items does
// not matter.
//
// What does not count: the run's recorded start (Run.Start), run-level errors,
// assertion payloads, an instant the source could not resolve (the zero
// time.Time), and a sample or group end that is absent — such an item still
// contributes its start. A group's end is its start plus its wall-clock
// Duration; CumulatedDuration answers a different question and never moves a
// bound.
type Bounds struct {
	start, end time.Time
}

// Extend widens the bounds to cover it, if it is something that counts. It
// takes a pointer because an Item carries a slot for every kind and Extend is
// called once per item of a run.
func (b *Bounds) Extend(it *Item)

// Start returns the instant the run began and true, or the zero time and false
// when nothing that starts a run has been folded.
func (b *Bounds) Start() (time.Time, bool)

// End returns the instant the run ended and true, or the zero time and false
// when nothing that ends a run has been folded. A run can have a start and no
// end: samples whose ends the source did not record, and no virtual-user event.
func (b *Bounds) End() (time.Time, bool)
```

## 3. `gatling.AbsentTimestamp`

```go
// AbsentTimestamp is the value a Record carries in Start, End or Timestamp for a
// time the log could not resolve: a negative offset in a binary log, a negative
// value in a text one, and the sentinel Gatling itself writes for a request
// that never completed, which is this very value. The canonical model reports
// such a time as the zero time.Time.
const AbsentTimestamp int64 = math.MinInt64
```

`Record`'s field documentation changes to name it:

```go
	// Start is a request's or group's start timestamp in milliseconds since the
	// Unix epoch, or AbsentTimestamp for one the log could not resolve.
	Start int64
	// End is a request's or group's end timestamp in milliseconds since the Unix
	// epoch, or AbsentTimestamp: Gatling writes that value for a request that
	// never completed, and a codec reports it for an end it could not resolve.
	// Nothing may assume End >= Start.
	End int64
	// Timestamp is a user event's or error's time in milliseconds since the Unix
	// epoch, or AbsentTimestamp for one the log could not resolve.
	Timestamp int64
	// CumulatedResponseTime is the sum of the response times of the requests
	// inside a group, in milliseconds, exactly as written. A negative value is
	// carried through and reported unset by the canonical model.
	CumulatedResponseTime int64
```

## 4. The zero-time convention — an observable change, ask first

No signature changes. The four fields keep `time.Time` and gain one sentence each:

```go
	// Start is when the operation began, in UTC, exactly as the source recorded
	// it: not rounded, and not re-based against the run's start. It is the zero
	// Time when the source could not resolve it; a recorded instant is never
	// the zero Time.
	Start time.Time                       // Sample

	// Start is when the traversal began, in UTC, exactly as recorded, or the
	// zero Time when the source could not resolve it.
	Start time.Time                       // GroupSample

	// At is when it happened, in UTC, exactly as recorded, or the zero Time
	// when the source could not resolve it.
	At time.Time                          // UserEvent, RunError
```

**Behaviour before**: an unresolvable time arrived as `time.UnixMilli(math.MinInt64)`, the year
−292275055. **After**: the zero time. Only a malformed record can observe the difference; every
corpus run is unchanged. Recorded under Changed. **Approved by the maintainer on 2026-09-06**
(constitution, ask-first); the rejected alternative, `Opt[time.Time]` on the same four fields, is
recorded in [research.md R3](../research.md).

`internal/wire` (not public) changes accordingly: `Millis` maps a negative count to the zero time;
`span` returns an unset duration when either end is `AbsentTimestamp`.

## 5. What is deliberately not added

- No `Span()`, no rounding helper, no rate: the consumer subtracts and rounds, and the example shows
  it.
- No `Item.Position()`: a consumer switches on `Kind` to fold a sample and a group differently, and
  each has its own method.
- No interface: there is one implementation of each type.
- No statistic. FR-017; enforced by the exported-surface golden
  ([data-model.md §6](../data-model.md)).

## 6. Documentation and example

- `model/doc.go` gains a section "The primitives a consumer folds" naming `Position` and `Bounds`
  beside the outcome predicate and the stream, and stating the zero-time convention once.
- `Example_fold` (`model/example_fold_test.go`): one loop over `simlog.NewRunReader` on the 3.15.1
  recording; per-position success and failure counts; a `Bounds`; the span rounded up to whole
  seconds; three rates. Its `// Output:` carries 102 requests, 84 ok, 18 ko, 4 s, 25.5 / 21 / 4.5.

## 7. `CHANGELOG.md` — drafted entries

```markdown
### Added

- `model.Position`, with `NewSamplePosition`, `NewGroupPosition`, `Sample.Position` and
  `GroupSample.Position`: where in a run something was recorded, as one comparable value a consumer
  buckets by. Two consumers keying a map by it produce the same keys without agreeing on a
  spelling; a group traversal and a sample never share one; and it stays valid after the reader
  advances, unlike the `Groups` slice it is made from. (#8)
- `model.Bounds`: where a run begins and ends, exactly as Gatling's own report bounds it — by
  sample, group and virtual-user events, never by the run's recorded start or its errors — folded
  one item at a time with `Extend`. It is the span every rate divides by, and the one definition a
  consumer was most likely to get subtly wrong. (#8)
- `gatling.AbsentTimestamp`: the one value both codecs write on a wire record for a time the log
  could not resolve. `gatling/binary` used it privately; it is now named, shared and documented on
  `Record`. (#56)
- `Example_fold` in `model`: the consumer's loop, over a real recording, checked against that run's
  own console summary. This library still computes nothing; the example shows what a consumer
  computes from. (#8)

### Changed

- An instant a codec could not resolve reaches the canonical model as the zero `time.Time` rather
  than as a date 292 million years in the past. `Sample.Start`, `GroupSample.Start`,
  `UserEvent.At` and `RunError.At` say so; a recorded instant is never the zero time. (#56)
```
