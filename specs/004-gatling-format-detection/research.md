# Phase 0 Research: Telling Which Gatling Wrote a simulation.log

**Feature**: `004-gatling-format-detection` | **Date**: 2026-09-05 | **Spec**: [spec.md](./spec.md)

Constitution in force: **v2.0.0**. Every decision below is checked against it, and the two that
touch the public API are marked for approval before implementation.

---

## R1 — Where dispatch lives, given that `gatling/` cannot import a codec

**Decision**: a new package **`gatling/simlog`**. It imports `gatling`, `gatling/text` and `model`,
detects the format, and returns a reader for it. Detection itself and the version policy stay in
`gatling/`, where issue #5 puts them and where they carry no codec dependency.

**Rationale**: `gatling/text` imports `gatling`, so `gatling` importing a codec back is a cycle the
compiler rejects. Issue #5's "new file in `gatling`: format detection and the version policy" is
right for the parts that are pure — classifying bytes, deciding a verdict, defining errors — and
those stay there. Only the step that *chooses a codec* has to live above both, and a sibling package
is the smallest thing that can. `simlog` names the artefact it opens (`simulation.log`), reads
without stutter at the call site (`simlog.NewReader`), and sits naturally beside `text` and the
`binary` package v0.0.5 will add.

**Alternatives considered**:

- **Registration by import side effect, the way `image.Decode` works.** The standard library solves
  exactly this problem this way: `image.RegisterFormat` plus a blank `import _ "image/png"`.
  Rejected. It buys decoupling this module does not need — both Gatling codecs live in this
  repository and are always linkable — and it costs the two things the spec requires most: FR-025's
  "known format, no codec yet" degrades to "unknown format" when a caller forgets the blank import,
  and FR-026's enumeration becomes a function of link-time imports rather than of what the module
  supports. Init-order magic also sits badly with Principle VI.
- **A `parsec.Open` at the module root.** Rejected: the root would then have to know about every
  tool, and JMeter, k6, Locust and phout are still ahead. A per-tool entry point keeps each tool's
  dispatch beside that tool.
- **Dispatch inside `gatling/text`.** Rejected: it would make the text codec import the binary one,
  inverting the dependency and putting "which codec" inside one of the codecs.

---

## R2 — Keeping the stream readable from byte 0 after detection

**Decision**: read up to `DetectSize` bytes into a fixed-size array with `io.ReadFull`, classify
them, then hand the codec `io.MultiReader(bytes.NewReader(head[:n]), r)`.

**Rationale**: FR-004 makes this correctness, not ergonomics — issue #6 records that the binary codec
reconstructs its string cache from the start of the file, so a detector that swallowed the bytes it
looked at would silently corrupt every binary read that ever follows. `io.MultiReader` states
plainly that the bytes are still there, costs one small allocation per opened log, and works on a
pipe, a network body or a decompressor's output, none of which can be rewound.

`io.ReadFull` also gives the error discipline FR-028 needs: `io.EOF` and `io.ErrUnexpectedEOF` mean
the input ended early and become the "too short" refusal; any other error is a failing stream and is
returned as it is, wrapped, never mistaken for a classification.

**Alternatives considered**:

- **`bufio.Reader.Peek`.** The obvious answer, and rejected on a detail of this codebase:
  `text.newScanner` deliberately wraps its argument in `struct{ io.Reader }{r}` so that a caller's
  own `*bufio.Reader` cannot raise the 1 MiB line ceiling (`gatling/text/scan.go`). A peeked buffer
  handed to the codec is therefore re-buffered anyway, so `Peek` buys nothing over `MultiReader` and
  hides a second buffer behind an interface.
- **Requiring `io.Seeker` or `*os.File`.** Rejected: it would exclude every non-seekable source, and
  the module's whole read path is `io.Reader` by Principle II.
- **Buffering the whole file to be safe.** Rejected outright: unbounded memory, Principle II.

---

## R3 — What actually identifies a text log (issue #5 is wrong here)

**Decision**: a text log is one that begins with `RUN\t` or `ASSERTION\t`. Nothing else.

**Rationale**: issue #5 proposes `'R'` for the text `RUN` line. The project's own corpus falsifies
it — both `testdata/corpus/gatling/3.11.5/simulation.log` and `3.12.0/simulation.log` begin with the
byte `A`, because Gatling writes one `ASSERTION` record per declared assertion *ahead of* the run
header. `gatling/text/reader.go` encodes the same rule from the other side: its preamble loop
accepts `ASSERTION` and `RUN` and refuses every other kind, so those two literals are exactly the
set that may legally open a text log. Requiring the tab as well as the literal is what keeps a plain
text file beginning "Ran the suite again" from being classified as a Gatling log.

This narrows nothing the issue promised: every log the issue names still identifies correctly. It is
the mechanism that was wrong, and issue #5 should be amended.

**Alternatives considered**:

- **First byte `'R'` only** (as issue #5 says). Rejected: misclassifies the ordinary case, proven by
  the corpus.
- **Any of the six record-kind literals.** Rejected as looser than the format: `USER`, `REQUEST`,
  `GROUP` and `ERROR` cannot legally open a log, and accepting them would classify a
  mid-file fragment as a whole log.
- **Trying the text codec and treating failure as "not text".** Rejected: it cannot distinguish a
  binary log from a damaged text one, which is the distinction FR-010 exists for, and it consumes
  the stream.

---

## R4 — What identifies a binary log, and why that is still a claim

**Decision**: a binary log is one whose first byte is `0x00`. This is recorded as a **claim to be
settled by evidence**, and FR-031a's sample is the evidence: the implementation task that adds the
rule and the task that captures the sample are the same change, and the rule is not merged without
it.

**Rationale**: the byte comes from issue #6's reading of the layout — records open with a kind byte
and `0` is the run record, which is necessarily the first record. Nothing in this repository has
ever read a binary `simulation.log`, so whether the file truly *begins* with that record — rather
than with a magic number, a length, or a header this project has not heard of — is unknown until
one is looked at. Spec 002 handled its source-derived claims the same way and recorded the rule:
where a recording disagrees with the spec, the recording wins.

The classification order also matters: `0x00` is tested first and is unambiguous against the text
literals, which are ASCII. There is no input for which both rules match.

**Alternatives considered**:

- **Shipping the `0x00` rule on a hand-written fixture.** Rejected by the clarification the spec
  records: a hand-written sample proves only that the code agrees with whoever wrote the sample,
  which is issue #14's complaint about asserted-not-demonstrated support.
- **Reading the binary version string here too.** Rejected in the clarification: it is the binary
  codec's own work and needs the binary codec's own evidence. FR-031b forbids the feature from
  claiming otherwise.

---

## R5 — How many bytes detection looks at

**Decision**: `DetectSize = 10`, a documented exported constant, and detection decides as early as it
can within that window.

**Rationale**: 10 is `len("ASSERTION\t")`, the longest opening this feature must recognise; `RUN\t`
needs 4 and the binary marker needs 1. A fixed window is what FR-005 and SC-008 require — the cost
of identifying a log must not grow with the log — and exporting it lets a caller that does its own
buffering size it correctly. Deciding early means a one-byte file that starts with `0x00` is
classified without waiting for nine more bytes that will never come.

Input shorter than the window is only "too short" when it is also *inconclusive*: bytes that are a
proper prefix of `ASSERTION\t` and then end. Anything else short is a decisive mismatch and is
refused as not a Gatling log, which is a more useful answer than "give me more bytes".

**Alternatives considered**:

- **One byte.** Rejected with R3: one byte cannot tell `ASSERTION\t` from a note beginning `A`.
- **A larger window "to be safe".** Rejected: every byte past the opening literal is the codec's to
  validate, and a bigger window makes the too-short case swallow more valid-but-tiny inputs.

---

## R6 — How a caller asks for strictness

**Decision**: variadic functional options shared by every codec, defined once in `gatling`, over an
**unexported** configuration:

```go
type Option func(*readOptions)
func WithStrict() Option
```

`text.NewReader`, `text.NewRunReader`, `simlog.NewReader` and `simlog.NewRunReader` all take
`opts ...gatling.Option`, `simlog` forwards them untouched, and the codec forwards them again to
`Policy.Apply`.

**Rationale**: strictness has to be expressible where a read begins, and the option has to be the
same one for both codecs or FR-012 is broken by the plumbing rather than by the policy. Defining
`Option` in `gatling` gives one definition that `text`, the coming `binary` and `simlog` all accept.

Two details follow the Go naming convention rather than a first instinct (research R13). The prefix
is **`With`** — `WithStrict()`, not `Strict()` — because `With*` is what Go uses for functional
options everywhere, and a codebase that mixes `With*`, `Set*` and `Use*` makes a reader check each
one. And the configuration the option writes to stays **unexported**: nothing outside `gatling`
needs to read it, a codec only forwards options it never inspects, and an exported config would
invite a consumer to define an option this package then has to honour. The exported surface is two
identifiers, not four.

Crucially this is **source-compatible**: adding a variadic parameter leaves every existing call site
(`text.NewReader(r)`) compiling unchanged, so FR-032's "observable behaviour unchanged" holds
without a deprecation dance. The one caller it could break is one that stored the constructor in a
`func(io.Reader) (*text.Reader, error)` variable, which nothing does and which pre-v0.1.0 permits
anyway (Principle V). **This is the API change `AGENTS.md` requires approval for; it is stated
exactly in [contracts/gatling-detect.md](./contracts/gatling-detect.md) and is not implemented until
approved.**

**Alternatives considered**:

- **A second constructor per reader** (`NewStrictReader`, `NewStrictRunReader`). Rejected: four
  constructors become eight, and eight become sixteen when the binary codec lands.
- **A required parameter.** Rejected: it breaks every call site to express something almost every
  caller leaves at the default, and it is not what Go does for optional behaviour.
- **An exported `ReadOptions` struct plus an `Options(...)` folding helper.** Rejected during the
  skills review: two more exported identifiers for no caller that needs them, and `Policy.Apply`
  taking `opts ...Option` directly removes the need to fold anywhere else.
- **A package-level variable or an environment variable.** Rejected: global state, untestable in
  parallel, and it makes one caller's choice another caller's surprise.

## R7 — Where the version policy lives, and how strict is expressed in the result

**Decision**: keep `gatling.Gate(found, lo, hi) Verdict` exactly as it is — the pure range fact — and
add a small type above it that carries the whole policy:

```go
type Policy struct{ Min, Max Version }
func (p Policy) Apply(found Version, opts ...Option) (Warning, error)
```

`Apply` is the single place the three outcomes are decided. A codec calls it once, before any record
is decoded, and does nothing else about versions. A new `*UnverifiedError` is what strict returns.

**Rationale**: FR-012 asks for one policy, not one function; `Min`, `Max` and the caller's strictness
always travel together and reading them from one place is what stops the coming binary codec from
growing a second copy. Leaving `Gate` untouched keeps the change purely additive — no exported
identifier changes meaning or signature — and `Gate` stays useful and tested as the predicate
`Apply` is built on, rather than becoming a second way to ask the same question: `Apply` answers
"what happens", `Gate` answers "where does this version sit".

Strictness deliberately does **not** become a fourth `Verdict`. The verdict is a fact about the
corpus range and does not change because a caller changed its mind; what changes is the action.
Keeping that split is what lets FR-021 be true by construction — strictness can only turn
`VerdictUnverified` into an error, and cannot reach the other two.

**Alternatives considered**:

- **Replacing `Gate` with `Policy.Verdict`.** Rejected: a breaking change for no gain, and it would
  need the approval R6 already spends.
- **A fifth verdict, `VerdictRefusedUnverified`.** Rejected: it conflates the range fact with the
  caller's policy, and every `switch` over `Verdict` in the module would have to handle a case that
  is not a property of the log.
- **Returning `(Verdict, error)` and letting each codec build its own error.** Rejected: that is
  precisely the duplication FR-012 forbids — two codecs, two error messages, drift nobody sees.

---

## R8 — Making the three refusals distinguishable without matching text

**Decision**: one error type per cause, all in `gatling` so both codecs share them, all inspected
with `errors.As`:

| Cause | Type | Exists |
|---|---|---|
| Not a Gatling `simulation.log`, or too short to tell | `*gatling.FormatError` (field `Short bool`) | new |
| Known format, no codec in this module yet | `*gatling.UnsupportedFormatError` | new |
| Version below the covered range, or not a release string | `*gatling.VersionError` | already exists |
| Version above the covered range, read strictly | `*gatling.UnverifiedError` | new |
| The log is damaged | `*gatling.SyntaxError` | already exists |

**Rationale**: FR-010 and FR-022 both say "distinguishable by a program without matching on message
text", which in Go means distinct types reachable through `errors.As`. `VersionError` already carries
a `Parsed bool` to separate its two faults, so `FormatError.Short` follows a convention the package
already has rather than inventing one. Putting all of them in `gatling` is FR-027: the binary codec
inherits the same words and the same shapes for free.

**Alternatives considered**:

- **Sentinel errors plus `errors.Is`.** Rejected: the callers need the values — which version, which
  range, which format — and a sentinel carries none.
- **One error type with a `Cause` enum.** Rejected: `errors.As` on one type then still requires a
  switch on a field, which is the text-matching problem with extra steps.

---

## R9 — What the dispatcher hands back

**Decision**: two interfaces, both declared in `gatling/simlog`, mirroring the two readers `text`
already offers:

```go
type RecordReader interface { Header() gatling.Header; Assertions() []string; Warnings() []gatling.Warning; Next() (gatling.Record, error) }
type RunReader    interface { Run() model.Run; Next() (model.Item, error) }
```

**Rationale**: `text` deliberately offers both a wire-record reader and a model reader (spec 003),
and a dispatcher that covered only one would leave the other caller writing its own detection —
the duplication this feature exists to delete. The interfaces are exactly the existing methods, so
`*text.Reader` and `*text.RunReader` satisfy them without a line of adapter code, and the binary
codec will too without importing them.

They live in `simlog` rather than `gatling` because `RunReader` names `model` types, and
`gatling/doc.go` states that package's job as the log's own records and not the canonical model.
Putting a model-facing interface there would contradict a doc comment that is currently true.

**Returning an interface from a constructor is the exception, and this is why it holds.** Go's
default is to return a concrete type, and to wait for a second implementation before extracting an
interface at all; today there is exactly one, `*text.Reader`. Both defaults are set aside on
purpose. `NewReader`'s job *is* to pick the codec, so it cannot name one concrete type in its
signature without lying about what it does, and returning `*text.Reader` today would have to become
an interface the moment the binary codec lands — a breaking change scheduled for v0.0.5 rather than
a hypothetical one. The interfaces also sit where they are consumed, which is the part of the
convention that matters most here: `simlog` declares them because `simlog` returns them, and neither
codec imports them to satisfy them.

**Alternatives considered**:

- **Returning the concrete `*text.Reader`.** Rejected: it cannot also return `*binary.Reader`.
- **One interface with both shapes.** Rejected: it would force every codec to implement the model
  conversion even where a caller only wants records.
- **Declaring the interfaces in `gatling`.** Rejected above, on the doc-comment ground.

---

## R10 — Capturing the binary sample FR-031a requires

**Decision**: run the existing probe simulation under a real Gatling 3.15.1 with
`sbt -Dgatling.version=3.15.1 "Gatling/testOnly io.galaxio.parsec.corpus.CorpusSimulation"`, take the
first 256 bytes of the `simulation.log` it writes, and commit them under
`testdata/samples/gatling/binary/` beside a `SAMPLE.md` recording the release, the machine, the JVM
and the command — the same provenance block the corpus `RECORDING.md` files carry.

**Rationale**: the probe project already parameterises the Gatling version through a system property
(`project/Dependencies.scala`), which is how the 3.11.5 and 3.12.0 recordings were made, so no new
tooling is needed. 256 bytes is far more than the 10 detection reads and far less than a run: it
cannot be mistaken for a corpus entry, and a human can see a run header in it. It goes under
`testdata/samples/`, **not** `testdata/corpus/`, because FR-031a forbids it being counted as corpus
— it holds no complete run, no report, and nothing may compare a decoder against it.

**The probe cannot be used, and this is settled rather than guessed.** The first draft of this entry
planned to run the existing probe under 3.15.1 and kept a fallback in reserve. The `gatling-versions`
skill (`galaxio/galaxio-gatling` 2.4.0, `references/galaxio-artifacts.md`) settles it the other way:
`gatling-picatinny` has **no release targeting the 3.14.x or 3.15.x line at all** — the column reads
`none`. The probe pins picatinny because it renders its OpenNFR requirements into Gatling assertions
(spec 003), so under 3.15.1 there is nothing to pin.

**The fallback is therefore the plan, not the reserve**: a throwaway minimal simulation with no
picatinny and a `gatling-sbt` from the 3.15.x column, run once to produce a `simulation.log`. This
is not a compromise — the sample needs a real binary log, not this project's probe in particular,
and FR-031a asks only for the leading bytes with their provenance. Widening the probe itself is
**not** in scope here and may not be possible at all while picatinny has no release on those lines;
that question belongs to v0.0.5 together with the corpus it records.

**Observation, flagged rather than acted on.** The same table places `gatling-picatinny` 1.27.0 on
the 3.13.x line (3.11.x tops out at 1.10.4), while the probe runs it under 3.11.5 and 3.12.0 — a
wrong-column pin by that authority, of the kind the skill says compiles and then fails at run time
only on APIs binding Gatling internals. The recordings exist and the assertions rendered, so
whatever the probe uses is evidently not such an API. This is an observation from a third-party
table, not a defect established against the code, and it is out of scope for this feature; it
belongs to whichever milestone next touches the probe (v0.0.5 or v0.2.0) and should be verified
there rather than assumed either way.

**Alternatives considered**:

- **Capturing through the canary workflow** (`workflow_dispatch` with `["3.15.1"]`). Rejected as the
  primary route: the canary's decode step hands the fresh log to the *text* reader, which refuses a
  binary one, so the job fails before uploading. Reworking the canary for a format it cannot yet
  read is v0.0.5's business.
- **Downloading a `simulation.log` from somewhere.** Rejected: provenance is the whole point.

---

## R11 — Performance, and what the benchmark measures

**Decision**: state the goal as *constant overhead*, and prove it with a `testing.B` that compares
`text.NewReader` against `simlog.NewReader` over the largest corpus log, plus one over `Detect`
alone.

**Rationale**: the constitution requires a decoder plan to state a throughput and peak-memory goal
and to ship a benchmark. This feature adds no decoding, so the honest goal is that it costs a fixed
amount however large the log, and spec 002's figures are unchanged end to end.

**What the benchmark actually said, and what it corrected.** `Detect` allocates nothing on every
path a real log takes and runs in 5.2–10.7 ns; fed 14 bytes and 1 MiB it costs 6.905 ns and
7.030 ns, so the size of the input is genuinely unreachable. Dispatch costs **five** extra
allocations and 138 bytes per opened log, not the one this entry first predicted: `io.MultiReader`,
its slice, the `bytes.Reader` over the replayed head, the head escaping to the heap, and the clone a
refusal returns so a caller on a non-rewindable stream keeps the whole log. The prediction was wrong
and is corrected here rather than explained away — the property that matters, that the cost is
constant and cannot grow with the log, is what the measurement confirms. Throughput is
indistinguishable, the two figures sitting inside run-to-run noise.

**Why not bufio.** Reading the window through a `bufio.Reader` was tried and reverted. Its Read
does one underlying read and returns `(0, nil)` straight through — the no-progress guard lives in
`fill()`, which Read does not use — so it fixed nothing, and it read its whole 16-byte buffer
rather than the ten bytes wanted, putting six bytes beyond the reach of the error a refusal returns.
`readHead` carries the guard explicitly and consumes exactly the window.

**Alternatives considered**: none — a throughput target for a function that reads ten bytes would be
theatre.

---

## R12 — Coverage floors and CI

**Decision**: no CI change. `scripts/check-coverage.sh` maps `*/gatling/*` to the 90% floor, so
`gatling/simlog` inherits it automatically, and `verify.yml`'s `deps` job already excludes this
module's own packages from the stdlib-only check, so `simlog` importing `model` is fine.

**Rationale**: verified by reading both files rather than assumed. The one thing to watch is that
90% applies to `simlog` from its first commit, which is achievable because the package is small and
every branch in it is reachable from a test.

---

## R13 — Which Go skills apply, and which must not

**Decision**: five skills are **mandated** for the implementation, four are situational, and three
are **forbidden** because following them would violate the constitution. The list is recorded here
so it survives into `tasks.md` and into review. Classified against
`samber/cc-skills-golang` 2.0.1 and `galaxio/galaxio-gatling` 2.4.0.

**Mandated** — read before writing the code they govern:

| Skill | Governs | What it already changed |
|---|---|---|
| `golang-naming` | every exported identifier | `Strict()` → `WithStrict()` (R6); confirmed `FormatError`/`UnverifiedError` (`Error` suffix), `FormatUnknown` at iota 0, `NewReader`/`NewRunReader` for a package with two constructible types |
| `golang-error-handling` | the five error types | confirmed custom types over sentinels (they carry data), `%w` wrapping, `errors.As` inspection, the single-handling rule — an error is returned or logged, never both |
| `golang-structs-interfaces` | `RecordReader`, `RunReader` | forced the justification for returning an interface from a constructor, and confirmed the interfaces belong in `simlog` where they are consumed (R9) |
| `golang-testing` | every test | table-driven with named subtests, `t.Parallel()` where independent, `FuzzDetect` for FR-028/SC-007 — **stdlib only**, see forbidden below |
| `golang-documentation` | doc comments and examples | every exported identifier gets one (Principle V already), plus an `ExampleXxx` for `simlog`: the skill rates example tests as expected for a library, and `gatling/text` and `model` both already have one |

**Situational** — reach for them when the moment arrives:

- `golang-benchmark` — `b.Loop()` over `for range b.N`, `b.ReportAllocs()`, and `benchstat` output
  pasted into the PR body. The repository's existing benchmarks already use `b.Loop()`, so this
  confirms rather than changes; what it adds is `benchstat` as the way to satisfy the constitution's
  "a regression against the recorded number MUST be justified in the PR".
- `golang-safety` — the nil-interface trap is a live hazard here: `NewReader` returns an interface,
  and returning a typed nil `*text.Reader` inside a non-nil interface is exactly the bug that skill
  opens with. Every error path must return a literal `nil` reader.
- `golang-lint` — only if `golangci-lint` objects. `errname`, `errorlint`, `wsl_v5` and `godot` are
  already enabled and enforce much of what `golang-naming` and `golang-error-handling` describe.
- `gatling-versions` — for R10, the artefact-per-Gatling-line table. It is what ruled the probe out
  under 3.15.x; `galaxio-gatling-pro` covers the simulation itself.
- `golang-refactoring` — this feature moves the text codec onto the shared policy and turns on an
  import cycle (R1), which is that skill's subject exactly.
- `golang-gopls` — for the rename in R6 and for confirming every call site of `text.NewReader`
  before the signature changes. Inert here: `gopls` is not installed, so the search is manual.
- `golang-project-layout` — **with a carve-out, and not forbidden.** The first draft of this entry
  banned it outright and that was wrong. Its `pkg/`, `cmd/`, architecture and dependency-injection
  questions are settled by Principle I and by published import paths, and are not open to revision —
  a `main` package here lives with the thing it serves, which is why the corpus stub sits under
  `testdata/` and not `cmd/`. What stays usable is its guidance on when a helper belongs in
  `internal/` and where a package boundary falls, and that grows as `jmeter/`, `k6/`, `locust/` and
  `phout/` arrive. This feature adds no `internal/` package and one clearly-placed one under
  `gatling/`, so no occasion arises here — but the general classification now lives in the
  constitution rather than in this file.

**Forbidden** — using these would break a constitution MUST:

| Skill | Why not |
|---|---|
| `golang-stretchr-testify` | Principle IV forbids third-party modules in `model/` and `gatling/`, and the `deps` job in CI enforces it. Principle III fixes the test framework as the standard `testing` package. The testify sections of `golang-testing` are excluded with it. |
| `golang-samber-*`, `golang-popular-libraries` | same — every one of them proposes a dependency this module may not take. `go.mod` naming no requirement is the intended steady state (Principle IV). |
| `golang-dependency-injection`, `golang-google-wire`, `golang-uber-dig`, `golang-uber-fx`, `golang-samber-do` | a container is both a dependency (IV) and an abstraction with no current need (VI). This module is imported, not wired. |

**Rationale**: the skills are guidance, the constitution is law, and where they disagree the
constitution wins — the same rule that governs `AGENTS.md`. Writing the disagreements down is what
stops a later contributor from "fixing" the layout or reaching for testify because a skill said so.

**This review outgrew the feature and was promoted.** The classification above was drafted here, for
spec 004, and it was immediately obvious that none of it is specific to format detection: the same
five skills govern every change to this module, and the same three would break the same MUSTs in any
of them. It is therefore now **constitution v2.1.0, Quality Gates & Tooling → Engineering Guidance
(Skills)**, with the triggers stated generally and a field added to the plan template so every
future plan names the rows its change fires. What survives in this file is the feature-specific
working — which rows spec 004 triggers, and what reading them actually changed in the contract.

**Alternatives considered**: applying every Go skill uniformly. Rejected — three of them would
introduce a dependency or a layout the CI gates reject, so the review has to happen once, here,
rather than once per pull request.

---

## Open questions carried into implementation

| # | Question | Settled by |
|---|---|---|
| 1 | Does a real binary `simulation.log` begin with `0x00`? | R4 — the FR-031a sample, in the same change as the rule |
| 2 | Does the probe build under Gatling 3.15.x? | R10 — attempted first, fallback written into the task |
| 3 | Is the variadic-options API change approved? | R6 — contracts/, approval before implementation |
| 4 | Which Picatinny and `gatling-sbt` pair with Gatling 3.15.x? | R10/R13 — Maven Central metadata lookup, at capture time |

Nothing else in Technical Context is unknown.
