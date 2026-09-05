# Phase 0 — Research: A Canonical Model, and Requirements Stated Once

**Feature**: `003-canonical-model` · **Date**: 2026-09-04 · **Spec**: [spec.md](spec.md)

Every finding here is either read out of a primary source with the file named, or produced by running
something and keeping the output. Where a claim is neither, it says so.

---

## R1. Does the OpenNFR renderer run under Gatling 3.11.5 and 3.12.0?

**Decision**: Yes. `gatling-picatinny` 1.27.0's `OpenNfrAssertions.fromYaml` renders the probe's
document under Gatling 3.11.5 and 3.12.0 alike. User Story 4 is delivered in this milestone and the
fallback of moving it to v0.0.5 is not taken.

**Why it was in doubt**: the renderer landed in that library's v1.26.0, and its published
compatibility table lists only two rows — the 1.12.0-and-later line against Gatling 3.13.x, and an
archived 0.16.0–0.18.2 line against 3.11.x. The probe must run 3.11.5 and 3.12.0, which is the range
whose text logs this project decodes. The clarification session recorded the further worry that the
renderer's `gatling-shared-model` dependency belonged to the 3.13 line only.

**That worry was wrong.** `io.gatling.commons.stats.assertion.Assertion` — the type the renderer
returns — lives in `gatling-shared-model` for every version in play, not in `gatling-core`:

| Gatling | Artefact carrying the assertion model |
|---|---|
| 3.11.5 | `gatling-shared-model_2.13-0.0.6` |
| 3.12.0 | `gatling-shared-model_2.13-0.0.7` |
| 3.13.5 | `gatling-shared-model_2.13-0.0.11` |

The `io.gatling.commons.stats.assertion` package has an **identical class inventory** in all three
(47 classes: `Assertion`, `AssertionPath` and its three cases, `Condition` and its seven, `Target`
and its four, `CountMetric` and its three, `Stat`, `TimeMetric`). `javap` on `Assertion`,
`Target$Count` and `Condition$Is` shows identical signatures in 0.0.6 and 0.0.11. Gatling is a
`Provided` dependency of the library, so the host project supplies it and no version is bundled.

**Evidence — the run itself.** A throwaway sbt project pinning Gatling 3.11.5 and
`org.galaxio %% gatling-picatinny % 1.27.0`, running the corpus probe unchanged except that its
hand-written `.assertions(...)` block was replaced by
`.assertions(OpenNfrAssertions.fromYaml("src/test/resources/nfr.yaml"))` against the document in
[contracts/nfr.yaml](contracts/nfr.yaml), and pointed at the corpus stub:

```text
> request count                                         36 (OK=18     KO=18    )
Global: count of all events is 36.0 : true (actual : 36.0)
Global: count of failed events is 18.0 : true (actual : 18.0)
Global: max of response time is less than 60000.0 : true (actual : 1503.0)
GET /ok: percentage of failed events is 0.0 : true (actual : 0.0)
outer / GET /ok: percentage of failed events is 0.0 : true (actual : 0.0)
outer / inner  with comma / GET /slow: percentage of failed events is 0.0 : true (actual : 0.0)
outer / inner  with comma / GET /fail: percentage of failed events is 100.0 : true (actual : 100.0)
connect refused: percentage of failed events is 100.0 : true (actual : 100.0)
unknown host: percentage of failed events is 100.0 : true (actual : 100.0)
[info] Simulation R1Simulation successful.
```

Re-run with `-Dgatling.version=3.12.0`: the same nine lines, the same verdicts, the same 36/18/18.

**Three further behaviours were checked in the same harness, because they are requirements rather
than conveniences:**

- **FR-027 / SC-008** — a requirement deliberately made false. Changing the failed-request threshold
  from 18 to 17 produced `Global: count of failed events is 17.0 : false (actual : 18.0)` and
  `[error] Simulation R1Simulation failed.` The run fails and the failure names the requirement.
- **FR-024 / SC-009** — total-and-loud refusal. A document carrying a `good` predicate and a `sum`
  aggregation produced **no assertions at all** and named both reasons:
  `unrenderable/good-share: `good` has no expressible numerator: a selector matches presence and
  never absence` and ``unrenderable/summed: aggregation `sum` over a metric has no equivalent:
  responseTime offers no sum``. The refusal is an `OpenNfrException`, so it fails the build.
- **FR-029** — the recorded-name rule. The assertion path Gatling printed is
  `outer / inner  with comma / GET /slow` — two spaces, the recorded spelling, exactly as written in
  the document. The renderer applied no substitution, which is what FR-029 records.

**Alternatives considered**: writing a renderer in this repository (rejected — one already exists in
the organisation, maintained beside the format, and ours would be the poorer of two); using the
deprecated `assertionFromYaml` (rejected — it is not OpenNFR, its keys are a closed list of six
Russian strings, it cannot express a count, and it is slated for removal in that library's 2.0.0);
deferring the story to v0.0.5 (not needed, since the renderer runs).

**Residual risk, named rather than dismissed**: the library documents this surface as *experimental*
and outside its binary-compatibility guarantee, and it tracks OpenNFR upstream `v0.8.0`. Pinning the
exact version in the probe's build is therefore part of the work, not an optimisation.

---

## R2. Which OpenNFR spellings the probe's expectations translate to

**Decision**: all nine of the probe's current assertions are expressible, with one restatement.

Read out of `assertions/opennfr/Reach.scala` in the library and confirmed by the R1 run:

| Probe's Gatling DSL today | OpenNFR predicate | Renders to |
|---|---|---|
| `global.successfulRequests.count.is(18)` | — restated, see below | — |
| `global.failedRequests.count.is(18)` | `{bad: {error.type: "*"}, aggregation: count, op: eq, threshold: 18, unit: "{request}"}` | `failedRequests.count` |
| — (new, replaces the successful count) | `{aggregation: count, op: eq, threshold: 36, unit: "{request}"}` under `selector: {}` | `allRequests.count` |
| `details(…).successfulRequests.percent.is(100)` | `{bad: {error.type: "*"}, aggregation: rate, op: eq, threshold: 0, unit: "%"}` | `failedRequests.percent` |
| `details(…).failedRequests.percent.is(100)` | the same with `threshold: 100` | `failedRequests.percent` |
| `global.responseTime.max.lt(60000)` | `{metric: loadtest.request.duration, aggregation: max, op: lt, threshold: 60000, unit: ms}` | `responseTime.max` |

**The restatement**: `good` is refused by the renderer — "a selector matches presence and never
absence" — so *successful* is not directly writable. Eighteen successes out of thirty-six recorded
requests is the same statement as thirty-six recorded with eighteen failed, and that is what the
document says. Per request, "100% successful" becomes "0% failed". Nothing is weakened: the R1 run
shows the restated document producing the same verdicts on the same run.

**`op: eq` renders**: `Reach.scala` maps `eq` to Gatling's `.is(v)`; only `neq` is refused, because
Gatling has no negating condition.

**Alternatives considered**: keeping the counts in Scala and moving only the percentiles into the
document — rejected, that is the two-sources-of-truth end state issue #30 names and rejects.

---

## R3. What the run carries, and what streams

**Decision**: `Run` carries only what does not grow with the length of the run — identity, tool and
version, capabilities, version warnings, and the opaque assertion payloads. Samples, group
traversals, user events and run-level errors all arrive as items of one stream.

**Rationale**: the clarification session settled that a run does not hold its samples. Planning
extends the same test to the other three and finds that only the payloads survive it. A run with a
million virtual users writes two million user events; a crashing simulation writes an error record
per crash. Neither is bounded, so neither can sit on a value a consumer holds while reading a 1 GB
log under the 32 MiB ceiling of SC-004. The opaque assertion payloads are bounded — Gatling writes
one per declared requirement, a handful — so they stay on the run, where FR-014 puts them.

**This corrects the spec's own first draft of FR-011a**, which listed user events as something the
run carries. FR-011a and FR-014 and the `Run` entity were amended; the recorded clarification answer
is untouched, since its substance — a run does not hold what grows — is what this extends.

**Shape**: one cursor per run, mirroring the decoder that already exists
(`Header()` + `Next() (Record, error)`), so the module has one convention rather than two:
`Run()` returns the O(1) header, `Next()` yields a `model.Item` discriminated by a `Kind`.

**The convention claim in this document's first draft was wrong, and is corrected here.** It said
`Item` matches "the flat `gatling.Record` the codebase already uses". It does not. `gatling.Record`
is a *flattened* union — 152 bytes for five kinds, because `Groups`, `Start`, `End`, `Timestamp`,
`Status` and `Message` are reused across them. `Item` is a *concatenated* union — 392 bytes for four
kinds, storing `Groups` twice and a timestamp four times and sharing nothing. Measured with
`unsafe.Sizeof`: Item 392, Sample 208, GroupSample 88, UserEvent 48, RunError 40.

The nested shape is kept, and now on its real premise rather than a borrowed one: this is the
canonical, consumer-facing model, and `it.Sample.Name` reading as a sample's name is worth more here
than in a wire record whose per-kind field table a decoder author reads once. Flattening would buy
throughput by making every field's meaning depend on `Kind`, which is exactly what `model` exists
not to do.

**What that costs, measured rather than assumed.** Against the raw decoder over a 64 MiB synthetic
log, the conversion loses about 41.6 ms/op. Attribution: the returned copy ~28%, the `Record`
argument copy ~10%, and the zero-and-fill of the oversized union ~61%. `plan.md`'s first draft blamed
the returned copy alone, which explains under a third of it. The two copies are removed — `convert`
now takes and fills pointers — and the remaining majority is the price of the shape, paid knowingly.

**Alternatives considered**: an interface per item kind (rejected — one allocation per item in the
hot path, and SC-004 is a memory criterion); four separate typed streams (rejected — a single
`io.Reader` cannot be read four times, so three of them would have to buffer, which is the thing
being avoided); `iter.Seq2` (rejected for the same single-pass reason, and it hides the error until
the range ends); flattening `Item` to `gatling.Record`'s shape (rejected above, with the measurement
that prices it).

---

## R4. How absence is represented

**Decision**: two mechanisms, at two granularities, neither redundant.

- `Capabilities` answers *what this source can never record*. A Gatling request has no response
  code, for any run, ever. A consumer reads it before rendering anything (FR-007).
- `Opt[T]`, a value type carrying a `T` and whether it is set, answers *what this record does not
  carry*. A source that usually records a response code did not record one here.

**Rationale**: with only the first, a partially populated field is unrepresentable. With only the
second, every consumer must scan every sample to learn what the source could never provide — which
is the "discover every value is empty" failure FR-007 exists to prevent.

**Why a value type and not a pointer**: a pointer per optional field is an allocation per sample in
a streaming path measured against a 32 MiB ceiling. `Opt[T]` is a struct field, allocates nothing,
and is distinguishable from a recorded zero, which FR-006 requires. The value-plus-flag shape is
the one the standard library itself uses for this (`sql.NullString` and its siblings), so it is not
a new convention.

**Capabilities is a set of what the source *provides*, queried by `Provides(Field) bool`.** This
inverts issue #4's wording ("declares per source what is absent") deliberately, and the reason is
the direction the mistake falls in. If the stored set is the absences, adding a field to the model
later leaves every existing adapter not listing it — and a consumer is told it is present when it
is not. If the stored set is what is provided, the same addition leaves it absent everywhere until
an adapter claims it, which is the conservative answer and the honest one. The query FR-007 asks
for is answered either way.

---

## R5. Times and durations

**Decision**: `time.Time` for instants, normalised to UTC; `time.Duration` for spans; `Opt[…]` where
the source may not have recorded one.

**Rationale**: FR-012 requires every recorded time to be preserved exactly, without rounding,
re-basing or timezone conversion. Gatling writes epoch milliseconds; `time.UnixMilli` converts
without loss, and normalising to UTC makes the value deterministic to render rather than dependent
on the reading machine's zone. The instant is unchanged either way — normalising fixes only how it
prints.

**Why not keep epoch milliseconds in the model**: the wire records already carry them and stay
exported (FR-014a), so a caller who wants the raw integer has it. The canonical model is the
consumer-facing one, and three downstream builds would each convert it themselves — which is where
two implementations disagree.

**The sentinel**: Gatling's own reader branches on an end timestamp equal to the minimum signed
64-bit integer. Whether a 3.11.5 or 3.12.0 run can produce one is still unconfirmed (spec 002 left
it open). The conversion must not assume the end is at or after the start: a sample whose end is the
sentinel, or is before its start, yields `Duration` unset rather than a negative or enormous span
(FR-020). This is cheap to honour and impossible to retrofit once a consumer has divided by it.

---

## R6. Which `loadtest.*` names bind, and what carries a failure

**Decision**: bind to the three names OpenNFR has settled and mint none.

| Quantity | Name | Source |
|---|---|---|
| The name of one recorded operation | `loadtest.request.name` | OpenNFR selector attribute |
| The enclosing groups, ordered, outermost first | `loadtest.group.name` | OpenNFR selector attribute |
| The duration of one recorded operation | `loadtest.request.duration` | OpenNFR metric |
| The cumulated duration of one group traversal | `loadtest.group.duration` | OpenNFR metric, minted upstream in v0.8.0 |

**Failure carries `error.type`.** OpenNFR's only expressible numerator is `{error.type: "*"}`, which
tests *presence*. So a failed sample carries an error whose presence is what distinguishes it, and a
successful sample carries none — FR-009. The value is what the source recorded about the failure;
this module classifies nothing, because Gatling records a free-text message and not a type, and
inventing a taxonomy here would be exactly the faking Principle I forbids.

**Alternatives considered**: minting `loadtest.*` names for what Gatling text does not record
(rejected — OpenNFR's own bar is that a name is minted only where something outside that repository
already records the quantity, and a name nothing emits is a vocabulary of one).

---

## R7. What Gatling text cannot provide

**Decision**: the capabilities a Gatling text run declares name these as absent, and the list is
closed by what spec 002 established about the format.

Response code; the scenario a request ran under (the log records a scenario on a `USER` record and
not on a `REQUEST`, so a request cannot be attributed to one); request and response byte counts;
connection, DNS and TLS timings; per-request throughput; the requirements the assertion payload
encodes; per-interval series; and the identity of the virtual user that made a request (spec 002's
data-model records that 3.11.5 and 3.12.0 record none, so a request cannot be attributed to a user).

**Group outcome is provided**, and is not the conjunction of the outcomes inside it: Gatling writes a
status on the `GROUP` record, and a group can fail while every request in it succeeded (FR-003).

**Both group durations are provided, and this corrects an earlier reading of this document.** A
`GROUP` record carries a start, an end and a cumulated response time, so wall clock across the
traversal is recorded as well as the sum of the requests it enclosed, and the two differ: a corpus
record reads `GROUP outer,inner  with comma 1788379665736 1788379667251 1505 KO` — 1515 ms of wall
clock beside 1505 ms cumulated. The draft listed wall clock as absent, confusing what the *log
records* with what Gatling's *assertion interface reaches*, which is only the cumulated figure;
`Capabilities` answers the first question. This matters beyond tidiness: the run span every derived
rate divides by is bounded by group ends, so a model without wall clock could not reproduce
Gatling's own rates and v0.0.7 would have inherited the gap. Found while writing the integration
test, which needed a stub helper to supply the end the model had dropped.

---

## R8. Where the conversion lives, and what it may depend on

**Decision**: `model/` holds the canonical types; `gatling/text/` gains the conversion. Both stay
standard-library only.

**Rationale**: Principle I puts the conversion in the tool package — "every tool package MUST convert
that tool's artefacts into `model` types" — and Principle IV keeps both packages stdlib-only, which
the `deps` CI job enforces. Nothing here needs a third-party module: the conversion reads the wire
records the decoder already produces.

**The probe's document needs no Go dependency either.** It is read by the Scala side, by the
renderer already in the DSL library. What this repository does with it is validate it against the
published OpenNFR schema on every change (FR-026), and that is a CI step rather than library code.

**Alternatives considered**: a `convert/` package of its own (rejected — Principle I names the tool
package, and a third package between them earns nothing); adding a YAML module to `go.mod` so Go
could read the document (rejected — nothing in Go needs to read it, and it would be the module's
first dependency, inherited by three downstream builds).

---

## R9. How the document is validated without adding a dependency

**Decision**: validate in CI against the schema file the OpenNFR repository publishes, using the
same tooling that repository uses, pinned.

**Rationale**: FR-026 requires the document to validate on every change and an unknown field to be
rejected naming the field — the OpenNFR schema sets `additionalProperties: false` throughout, so the
schema does that work. Validating here rather than trusting upstream means a schema change this
document violates is caught by this project's pipeline instead of by nobody.

**Open, and cheap to settle in implementation**: which validator, and whether the schema is vendored
or fetched. Vendoring pins the check and makes CI hermetic; fetching keeps it current and can break
on an upstream move. The renderer tracks upstream `v0.8.0` while the published schema has moved on,
so the document must satisfy **the renderer** first — R1 proves it does — and the schema check is
the second gate, not the first. A divergence between the two is a finding to report upstream, not a
reason to change the document.

---

## R10. What the existing verification keeps doing

**Decision**: the corpus and canary checks that read wire records stay exactly as they are, and the
conversion adds a second layer held to the same numbers.

**Rationale**: the existing checks prove the decoder. A check that ran only through the model could
not tell a decoder bug from a conversion bug. Keeping both means a failure localises itself, and
the cost is one more assertion over data already in memory.

**The two recordings are not re-recorded.** They were captured with the reports their own Gatling
generated, which is the half that cannot be recreated, and their logs already carry the assertions
those runs evaluated. The document changes what the canary holds a *fresh* run to, and how a future
recording derives its assertions — nothing about the two entries already committed.
