# Research: The Primitives a Consumer Folds

**Feature**: `006-fold-primitives` | **Date**: 2026-09-06 | **Spec**: [spec.md](./spec.md)

Every decision below was checked against the code on `main` at f454217, and every number was
measured or folded from a recording rather than estimated. Probes were run out of tree and are not
committed; what they showed is written down here so the plan does not rest on reading alone.

## R1 — What a position is, and why it is one string underneath

**Decision**: `model.Position` is a struct with one unexported field, an encoded key: a kind byte,
then every group name and — for a sample — the name, each as a length prefix followed by its bytes.
Two constructors, `NewSamplePosition(groups, name)` and `NewGroupPosition(groups)`, and two methods
on the model types that call them, `Sample.Position()` and `GroupSample.Position()`. Accessors
`Kind()`, `Groups()`, `Name()` and `String()`.

**Rationale**: FR-002 needs `==` and map-key use, which rules out anything holding a slice. FR-003
needs distinct paths to stay distinct for any names, which rules out joining with a separator — the
corpus itself proves the hazard: spec 002 put a comma in a group name on purpose, and the
verification suite's own `statsKey` joins with a comma and lives with the ambiguity. FR-005 needs
the path and the name back, which rules out a hash. FR-006 needs the value to outlive the reader's
reused `Groups` slice, which a string satisfies by construction because the bytes are copied when
the key is built. A length-prefixed encoding is prefix-free, so it is unambiguous for any byte
sequence including NUL, and it decodes back exactly. The kind byte is what keeps a group traversal
at `a` → `b` apart from a request named `b` inside `a` (FR-004): the same segments, a different
first byte.

The encoding is internal. Nothing exported reads the key, so it can change without touching the
API. `String()` renders for display only, as FR-007 requires, and is not the identity.

**Alternatives considered**:

- A struct of exported `Groups []string` and `Name string` — not comparable; rejected by FR-002.
- A joined string with a separator and escaping — comparable, but the escaping is a second grammar
  every renderer has to know; length prefixes need none.
- A 64-bit hash — comparable and cheap, but collisions are possible and the path is not
  recoverable; rejected by FR-003 and FR-005.
- Interned pointers — identity would depend on which reader produced the value, and two consumers
  would never bucket alike; rejected by FR-002.
- A field on `Sample` filled by the adapter — every item would pay for the key whether or not the
  consumer buckets, and `Item` would stop being allocation-free (`model`'s documentation promises
  that streaming a run allocates nothing per item). A method computes it on demand, so only a
  consumer that asks pays.

**Naming, per `golang-naming`**: `New*` constructors, because the package has many constructible
types; a `PositionKind` enum with `PositionUnknown` at zero, mirroring `ItemKind`, rather than an
`IsGroup` boolean; accessors without a `Get` prefix; `Groups()` rather than `Path()`, because every
model type already calls the path `Groups`.

## R2 — Bounds are an accumulator the consumer extends, not a function over the stream

**Decision**: `model.Bounds` is a small struct — an unset start and an unset end — with
`Extend(*Item)`, `Start() (time.Time, bool)` and `End() (time.Time, bool)`. The zero value means
"nothing counted yet" and is ready to use. The rules are FR-009 through FR-013 verbatim: sample and
group starts and virtual-user STARTs move the start; sample and group ends (start plus wall-clock
duration) and every virtual-user event move the end; an absent end contributes nothing; an absent
start contributes nothing; errors, assertion payloads and the run's own recorded start are ignored;
the result is order-independent because it is a minimum and a maximum.

**Rationale**: the consumer folds everything in one pass over `Next()` (FR-016). A function that
walked the stream itself would force a second read, and a bounds value computed inside each codec's
reader would be two implementations of one definition — the divergence this feature exists to
remove. An accumulator with one method is the smallest thing that serves the single pass. `Extend`
takes `*Item` for the reason `internal/wire.Item` does: an `Item` carries a slot for every kind and
is passed once per record of a multi-gigabyte log.

Folding the 3.15.1 recording through exactly these rules gives a span of 3,232 ms — 4 s after
Gatling's round-up — and 102/4 = 25.5, 84/4 = 21 and 18/4 = 4.5 requests per second, the three
figures its console summary prints. The first bounding event of that run is a virtual-user start
529 ms before the first request, and the run's recorded start is 529 ms earlier still and is
correctly not a bound. This is the rule the text verification suite has implemented by hand since
spec 002 (FR-021c) and holds to `stats.json`.

**Alternatives considered**:

- `Span()` returning end minus start — a derived duration a consumer computes in one line, and the
  rounding Gatling applies is the consumer's (FR-014). Left out under Principle VI; it can be added
  if two consumers ever write the subtraction differently, which subtraction cannot.
- Bounds on `Run` — `Run` is available before the first item and the bounds are known only after
  the last; it would also be one implementation per codec.
- Forbidding the half-set state — a run whose only samples lack ends and which has no user events
  has a start and no end. Representing that honestly is cheaper than inventing an end.

**Receivers, per `golang-structs-interfaces`**: `Extend` mutates, so `Bounds` uses pointer
receivers throughout; an accumulator lives in a variable, where that costs nothing. `Position` is
immutable and small and uses value receivers.

## R3 — An absent instant is the zero `time.Time`, and that is not a shape change

**Decision**: `Sample.Start`, `GroupSample.Start`, `UserEvent.At` and `RunError.At` keep their
type. An instant the source could not resolve is the zero `time.Time`, documented on each field.
`internal/wire.Millis` maps every negative millisecond count — after R5, only ever the wire
sentinel — to the zero time, and `span` reports no duration when either end is absent.
`Bounds.Extend` skips a zero start.

**Rationale**: today an unresolvable time reaches the model as `time.UnixMilli(math.MinInt64)`,
which prints as the year −292275055 and answers `IsZero()` with false — a probe confirmed both.
That is the "plausible-looking instant" FR-023 forbids. The zero `time.Time` is the standard
library's own convention for a time that was never set, `IsZero` is its documented test, and no
recorded instant can ever equal it: both codecs report every negative millisecond value absent (R5),
and zero milliseconds is 1970, not the year 1. So absence is distinguishable from every recorded
value, which is what spec 003's FR-006 asks, without changing a public field's type for the three
builds that import this module while galaxio-cli#51 waits on it. It is also one rule in one place:
`Millis` is already the single conversion both codecs share.

**Alternatives considered**:

- `Opt[time.Time]` on the four fields — structural rather than conventional, which is how the model
  treats outcomes. Rejected for this release: it changes the shape of the most-read field on four
  types for every consumer, and `Opt`'s equality is `==` on `time.Time`, which compares wall clock,
  monotonic reading and location rather than the instant, so two equal times can compare unequal
  inside it — a wart the `Opt` documentation would then have to carry. If the maintainer prefers the
  structural form the change is mechanical, and the contract lists the four fields.
- Keeping the sentinel time and documenting it — a consumer bucketing by start would file it 292
  million years ago and nothing would look wrong until someone read the axis.

**Ask first — approved 2026-09-06**: this changes what a consumer observes for a malformed record,
not a signature. The maintainer approved the zero-time convention over `Opt[time.Time]` in the
`/speckit-tasks` session; it is recorded under Changed.

## R4 — One sentinel, exported, shared by both codecs

**Decision**: `gatling.AbsentTimestamp`, a constant equal to `math.MinInt64`, documented as the
value a wire record carries for a time the log could not resolve. It is the value `gatling/binary`
already uses privately as `absentTime`, and the value Gatling's own reader treats as a request that
never completed, which `gatling/text` already parses out of the log. Both codecs use the one
constant, and `Record`'s documentation names it.

**Rationale**: FR-022 requires the two codecs to produce identical records for an equivalent
input, and a sentinel that exists as two private copies is a sentinel that can drift. The wire
records are exported (spec 003, FR-014a), so a consumer reading `Record.End` needs the value named
rather than described as "a sentinel".

**Alternatives considered**: two private constants pinned equal by a test — works until someone
changes one, and leaves `Record` documented in prose.

## R5 — What changes in the text codec, and what turned out not to need changing

**Decision**: in `gatling/text`, a timestamp field — request start and end, group start and end,
user event time, error time — that reads as a minus sign followed by digits is `AbsentTimestamp`,
not an error; a cumulated response time that reads as a minus sign followed by digits is kept as the
negative number it is, exactly as the binary codec keeps a negative 32-bit field, and
`internal/wire.millisDuration` already reports such a value unset. Anything that is not digits, and
a positive value that overflows, is refused as today: those are inputs the binary format cannot
express, and the spec's non-goal keeps them out.

**What a probe showed**: all four refusals reproduce on `main` — `expected a non-negative integer
start`, `… end, or the never-completed sentinel`, `… cumulated response time` and `… timestamp`.
The third divergence in issue #56 does **not** reproduce: `newParser` allocates the groups scratch,
`parseGroups` always returns `sealGroups()` over it, and both the first and the second group-less
request record come back with a non-nil, empty path from `NewReader` and from `NewRunReader` alike.
The comment at `parse.go:377` is correct. The agreement test (R6) still pins the case — that is what
FR-024 is for — and no code changes for it unless the test proves the probe wrong: Principle VI
allows no fix without a failing test.

**Alternatives considered**: making the binary codec refuse, as the issue weighed and rejected —
spec 005 FR-009 is explicit, and the review of #53 already rejected one bad field ending a
ten-million-record read.

## R6 — Where the agreement test lives

**Decision**: in `gatling/binary`'s external test package. It already imports `gatling/text` —
`TestOneReportRendersBothFormats` renders a text run and a binary run through one function — and it
owns `builder`, the hand-written binary log writer that can produce a negative offset, a negative
cumulated time and a record without groups on demand. The test is table-driven over the three
inputs, reads each through both `NewRunReader`s, and requires either identical items or identical
error kinds.

**Rationale**: the fixture generator cannot be imported by another package's tests without moving
it, and moving it is a refactor outside the issue. `gatling/simlog` also imports both codecs but has
no builder. The constitution's rule that tool packages do not import each other is about production
imports; `binary_test` importing `text` is the precedent the v0.0.5 review accepted.

**Regression tests that fail without the fix**: the agreement test itself fails on `main` for the
first two inputs, because text refuses and binary accepts. `gatling/text` additionally gains a table
test over the negative-field cases so a reader of that package's tests sees the rule where the
parser is.

## R7 — One ceiling per count

**Decision**: `gatling/binary` keeps `maxGroupDepth = 1024` for nesting and adds `maxScenarios` and
`maxAssertions`, both 1 << 16; `readStrings` takes its ceiling as a parameter and `readBlobs` uses
`maxAssertions`. Each constant carries its own justification. The group-path scratch capacity, today
written as `maxGroupDepth/128`, becomes its own small constant so that the nesting ceiling bounds
nothing but nesting.

**Rationale**: the ceilings exist to stop a corrupt count from sizing an allocation, and the
allocation they size is a slice of string headers at 16 bytes each. At 65,536 entries the worst a
corrupt count can force is 1 MiB, against a 32 MiB budget and before any payload is read; no
simulation author declares sixty-five thousand assertions or scenarios, and two thousand assertions —
the spec's floor — is already a large generated suite. The values sit far above use and far below
harm, which is the whole job of a ceiling.

**Alternatives considered**: growing the slice as payloads are read, so the count sizes nothing at
all — robust, but it changes the allocation strategy of a reviewed reader to solve a problem the
ceiling already solves.

## R8 — The documentation fix and its provenance

**Decision**: README's package list gains the `gatling/binary/` row and its Status section says
what v0.0.5 shipped — a binary log is read, over 3.13.1 through 3.15.1, through `gatling/simlog` as
before; the "no codec reads it yet" sentence and the "except the Gatling text log" sentence go. The
two comments cite `testdata/corpus/gatling/3.15.1/RECORDING.md`, and that file gains one line
stating what the log opens with, so the provenance is readable without a hex dump. Confirmed against
the recording: its first eleven bytes are `00 00 00 00 06 33 2e 31 35 2e 31` — kind 0, a length of
6, `3.15.1`.

## R9 — The exported surface is a golden file

**Decision**: a test in `model` parses the package's non-test sources with `go/parser`, lists every
exported type, function, method, constant, variable and struct field, and compares the sorted list
to `model/testdata/exports.golden`; `-update` rewrites it, as the corpus goldens do.

**Rationale**: FR-018 asks that adding a statistic be a deliberate act that fails a check. Whether
an identifier *is* a statistic is a judgment; what a test can do is make every addition visible as a
diff a reviewer must approve. Struct fields are included because a statistic could hide as a field
as easily as a method. Standard library only, and no tool is invoked at test time.

**Alternatives considered**: diffing `go doc -all` — shells out to the toolchain from a test;
reflection — cannot enumerate package-level functions.

## R10 — The documented example is the consumer

**Decision**: `Example_fold` in `model`'s external test package opens the 3.15.1 recording through
`gatling/simlog`, folds success and failure counts per `Position` and one `Bounds` in a single
loop, and prints the totals, the span in whole seconds as Gatling rounds it, and the three rates.
Its `// Output:` is the console summary's own figures, so the example is also a check against the
recording.

**Rationale**: FR-021 wants the loop shown, and an example is documentation the test suite
executes. `model_test` importing `gatling/simlog` is an external test package importing a package
that imports `model`, which Go permits and which keeps `model` itself stdlib-only.

## R11 — Two independent folds in the verification suite

**Decision**: the text suite gains a third tally beside `decodeTally` and `modelTally`, keyed by
`Position` and bounded by `Bounds`, and asserts it equal to `modelTally` — the hand-rolled fold
spec 002 wrote — before holding it to the report through the same `checkCounts` and `checkRates`.
The binary suite, which today checks counts only, gains the console summary's `mean throughput
(rps)` line: the folded bounds, rounded up to whole seconds, must reproduce all three figures
exactly at the precision the console printed.

**Rationale**: FR-033 asks for two folds written independently; the hand-rolled one already exists
and predates the primitives, which is the strongest kind of independence. FR-015 asks the bounds to
reproduce the span-derived figure each run's own account carries, and the console summary is the
account the binary runs kept for it (spec 005, FR-029).

## R12 — Performance goal and its baseline

**Measured on `main` at f454217** (Apple M-series, Go 1.25, `-benchtime=2s`, `-tags=integration`):

| benchmark | ns/op | items/op | ns/item | allocs/op |
|---|---|---|---|---|
| `binary BenchmarkDecodeToModel/corpus` | 21,762 | 132 | 165 | 48 |
| `binary BenchmarkDecodeToModel/synthetic-64MiB` | 468,440,292 | 4,357,716 | 107 | 21 |
| `text BenchmarkRunReader/corpus` | 25,537 | 66 | 387 | 44 |
| `text BenchmarkRunReader/synthetic-64MiB` | 183,688,559 | 1,315,860 | 140 | 32 |

**Goal**: taking a position for every sample and group and extending the bounds for every item
costs at most one allocation per position and none per `Extend`, and the consumer's whole pass —
decode to model, position, bounds — stays under twice the decode-to-model time per item on the
synthetic log, so above four million items a second there. Peak memory stays under the 32 MiB budget
and does not move when the log grows tenfold, measured with the existing sampler. `BenchmarkFold` in
`gatling/binary` reports `items/op` beside `ns/op` and `allocs/op`, and the PR records the numbers.

**Fallback, measured before taken**: if the key allocation dominates, both codecs already intern
names, so a path is a sequence of stable string headers and a per-reader cache keyed by those
pointers could hand out one key per distinct place without allocating per record. That is an
optimisation with a benchmark to justify it, and Principle VI keeps it out until the number says
otherwise.

## R13 — Skills required for this change

**Required reading, read for this plan**: `golang-naming` and `golang-structs-interfaces`. Applied:
`New*` constructors; an enum with an unknown zero rather than a boolean; no `Get` prefixes;
consistent receivers per type; a useful zero value for both new types; `fmt.Stringer` honoured by
`String()`; no interface introduced, because there is one implementation of everything here.

**Required reading, read before the code**: `golang-error-handling` (the text codec's refusals
become values; nothing new is refused), `golang-testing` (every task), `golang-documentation` (a doc
comment on each new identifier, and the zero-time convention stated on four existing fields).

**One disagreement, found at implementation and resolved for the codebase**: `golang-testing` asks
for one test file per source file, named after it. This codebase names test files by concern —
`golden_test.go`, `limits_test.go`, `review_test.go`, `agreement_test.go` — and Principle VI says to
follow the conventions already in the codebase before adding a new one. The new tests follow the
codebase. Recorded here rather than argued in the code.

**Consult, occasion arisen**: `golang-benchmark` (R12); `golang-safety` (R7's ceilings, and R1's
copy-out of the reused slice).

**Disagreements with the constitution**: none found. `golang-structs-interfaces` recommends
dependency injection through interfaces; nothing here has a dependency to inject, and the
constitution rules out a container regardless. `golang-naming` prefers `New()` for a package with
one primary type; `model` has many, so `NewSamplePosition` and `NewGroupPosition` follow its
multi-type rule.

**Must not be followed**: testify and the `samber/*` family, as always.

## Open questions carried into implementation

1. ~~**Approval of R3**~~ — **approved 2026-09-06** by the maintainer: the zero-time convention on
   the four fields, not `Opt[time.Time]`.
2. ~~**Approval of R5**~~ — **approved 2026-09-06** by the maintainer: the three refusals in
   `gatling/text` become values, recorded under Changed.
3. **The exact `// Output:` of `Example_fold`** — the totals, span and rates are known from the
   recording (102, 84, 18; 4 s; 25.5, 21, 4.5); the number of distinct positions is read off the run
   when the example is written and reviewed against `index.html`'s per-request rows.
