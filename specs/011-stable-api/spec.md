# Feature Specification: A stable API

**Feature Branch**: `011-stable-api`

**Created**: 2026-09-12

**Status**: Draft

**Input**: User description: "https://github.com/galax-io/parsec/milestone/11" — the whole milestone,
with no subset named.

**Milestone**: v0.1.0 — A stable API (parsec milestone #11). Sixteen open issues: #13, #58, #77, #78,
#79, #84, #85, #86, #93, #95, #96, #97, #99, #101, #103, #107. Its seventeenth, #12, closed on
2026-09-05 when the statistics moved whole to galaxio-cli#61.

---

## Context

v0.1.0 is the tag at which this module stops describing itself and starts promising. Principle V
draws the line: before it, an exported identifier MAY change between releases at the cost of a
`CHANGELOG.md` entry; from it, changing a signature or an observable behaviour is a breaking change
that needs a spec, an approval, a `// Deprecated:` window of at least one MINOR release, and a MINOR
release of its own. Three codebases — galaxio-cli, the comet sidecar and the Galaxio backend — will
pin the tag, and none of them can express "no breaking changes" against a v0.0.x constraint.

So the freeze is not a ceremony. It converts every current defect on the surface into a permanent
one, and this milestone is the last commit-priced opportunity to fix each. The sixteen issues fall
into five groups.

**There is no contract, and the surface is larger than the contract would be** (#13, #107). Nothing
today states which identifiers a consumer may depend on or what a version bump means. #107 inventoried
the surface identifier by identifier — 278 exported names across `model`, `gatling`, `gatling/text`,
`gatling/binary`, `gatling/simlog` and `gatling/run` — against what the three consumers plan to call
and against every recorded contract under `specs/*/contracts/`. Two identifiers should not be frozen:
`gatling.Gate`, whose only production caller is `Policy.Apply`, itself documented as "the single place
the outcomes are decided"; and `gatling.MaxRunStart`, read by the two codecs and computed with by no
consumer. #107 also answers the three questions #13 left open — keep `UnsupportedFormatError` and give
it the producers #84 needs, confirm `SyntaxError`'s three position fields, and make `Record.Line` = 0
for a binary log the contract rather than an omission. What remains is to write the list down, because
a freeze that is a description rather than a list cannot be checked.

**One thing has two names, one rule has two spellings, one sentence has two copies** (#77, #78, #58).
`Tool = "gatling"` is an exported constant in both codec packages, so a consumer branching on the tool
picks between two names that are equal only because someone typed the same string twice. Eleven
exported enums answer an out-of-range value in two conventions and three idioms: every `gatling`-side
enum renders `Type(99)`, every `model`-side enum renders `unknown` — the same string the zero value
renders, which in `model.Outcome` means "the source lost this". And the `NewRunReader` warning loop,
including the user-visible sentence "no recording covers it — the verified range is … through …, so
the records decode unverified", is byte-identical in both codecs, one function short of where the
v0.0.5 `internal/wire` extraction stopped: the same condition can begin printing two different
sentences depending on which format was opened, in a module whose stated goal is that a consumer
cannot tell the two formats apart.

**What pkg.go.dev will publish is not always true** (#79, #85, #86, #96). `SyntaxError.Error`'s
comment says it names a line; for a binary log it renders a byte offset, and a consumer that believes
the comment reports byte 42 as line 42. `binary.NewReader`'s comment carries a stray `//` that
collapses a paragraph and publishes a literal `//` on the page a consumer evaluating this module reads
first. `TruncationError` states one fact twice inside one comment. No exported reader says it is
unsafe for concurrent use, although every `Next` mutates unsynchronised state and the text interner's
map turns a shared reader into `fatal error: concurrent map read and map write` — a runtime throw
`recover` cannot catch — and the consumer most likely to reach for a second goroutine, the comet
sidecar, holds only the `simlog` interface, where the concrete type carrying the map is invisible.
`Warning` is an exported type in two packages with incompatible shapes one call apart, and code
written against `gatling.Warning.Version.Compare` does not survive the move `simlog` recommends. And
the package overviews — the first page pkg.go.dev shows — route a newcomer away: `gatling/doc.go` says
the canonical model "arrive[s] in a later milestone" when it arrived in v0.0.3 and v0.0.5, the root
`doc.go` omits `gatling/run` entirely, `gatling/text/doc.go` never mentions its own `RunReader`, and
`simlog/doc.go` points readers at the codec packages, whose overviews never point back.

**Two definitions on the frozen surface are wrong, and are decided here** (#84, #103). A codec handed
the other format's log returns a `*gatling.SyntaxError`, documented as a damaged log — so a consumer
with a mixed archive quarantines as corrupt every file of the other format, and the most likely first
failure a newcomer meets says nothing about formats, nothing about the other codec, and nothing about
`simlog`, the package that exists to remove the choice. The answer is already in the bytes: `Detect`
needs ten and both codecs have consumed more before they fail. And `model.Bounds` can report an end
earlier than the start of an item it counted, because an item with a start and no recorded end extends
only the start — the shape the type's own `unplaced` rationale calls dishonest, in the primitive
galaxio-cli#61 says the run's bounds must come from. This feature settles the second: an end-less item
extends the end to its own start, so the run is known to have been running at least until that
instant, which is what Gatling's own arithmetic does with a request that never completed.

**Tests that cannot fail, and a release path with no barrier at either end** (#95, #93, #97, #99,
#101). Principle III is NON-NEGOTIABLE and the corpus is the specification, so six tests that cannot
go red — including the only test of the `Groups` reuse guarantee, which skips when the guarantee
breaks — are gaps in it. Four third-party actions are pinned to movable tags, one of them inside the
job that holds `contents: write` and cuts releases; two workflows interpolate dispatch inputs straight
into shell, although `record-corpus.yml` already carries the guard and states the reason. The
`CHANGELOG.md` 0.0.9 entry states the run-ordering rule twice, contradicting the code the first time
and itself the second. The `README.md` is a project narrative with no code fence, no `go get`, a
compatibility table wrong at both ends, and — as the one read call it names — the codec that cannot
read the format the README opens by promising to rescue. And a module whose whole job is decoding
untrusted binary input, with four fuzz targets and a nightly fuzz workflow, has no `SECURITY.md` and
no private reporting channel.

**The two decisions this spec takes, beyond what the issues had already settled.** #78 leaves the
choice of rendering rule open and #103 leaves the bounds definition open; both freeze permanently at
the tag. Taken here: an out-of-range enum renders as the type name and the number everywhere, which
takes the `gatling` convention into `model` and stops a value the module cannot name from rendering
identically to a value that means the source lost it; and an end-less item extends the end, which
moves a number every consumer divides by and is recorded under Changed. Both are FR-014 and FR-021.

**Every story below blocks the tag.** The priorities order the work; they do not make anything
optional. A v0.1.0 cut with any of these unresolved freezes the defect.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - The promise is a list, and nothing outside it is reachable (Priority: P1)

A consumer pins `v0.1.0` and wants to know, before writing an import, exactly which identifiers it may
depend on and what a future version number will tell it. It reads one table, imports only what the
table names, and compiles.

**Why this priority**: without it the other fifteen issues are polish on an unbounded surface. Every
removal in this milestone is priced in commits only while the module is below v0.1.0, and the list is
what makes the freeze checkable rather than describable.

**Independent Test**: a program importing only the identifiers the contract table names builds against
the tag with no reference to any `internal/` package; the table and `go doc -all` over the six packages
agree name for name.

**Acceptance Scenarios**:

1. **Given** the spec's contract table and `go doc -all` over `model`, `gatling`, `gatling/text`,
   `gatling/binary`, `gatling/simlog` and `gatling/run`, **When** the two are compared, **Then** every
   exported identifier appears in both and the count is recorded.
2. **Given** `go doc -all ./gatling`, **When** it is searched for `Gate` and `MaxRunStart`, **Then**
   neither is present, and both removals are recorded under Removed in `CHANGELOG.md`.
3. **Given** a consumer importing only the documented surface, **When** it is built against the tag,
   **Then** it compiles without naming any internal package.
4. **Given** the released contract, **When** a reader asks which Gatling versions are supported,
   **Then** the answer is stated as part of the compatibility promise: 3.11.5 through 3.12.0 for the
   text format, 3.13.1 through 3.15.1 for the binary one.
5. **Given** `simlog.RecordReader` and `simlog.RunReader`, **When** the tag is cut, **Then** their
   method sets are final, because a consumer's test double implements them and an added method breaks
   the implementer.
6. **Given** the three decisions #13 left open, **When** each is looked up, **Then** it is stated in
   the doc comment of the identifier it governs: `UnsupportedFormatError` is kept and produced,
   `SyntaxError` keeps `Line`, `Offset` and `Format`, and `Record.Line` = 0 for a binary log is the
   contract.

---

### User Story 2 - One name, one rule, one copy (Priority: P1)

A consumer branches on the tool name, renders an enum it received as an integer, and reads a warning
about an unverified version. Each of the three has exactly one answer, whichever package it arrived
through.

**Why this priority**: all three are surface changes. After the tag, removing the duplicate `Tool`
costs a deprecation window and a MINOR release; changing what `String()` prints is a breaking change
to observable behaviour; and the duplicated warning prose is the one copy that a user actually reads.

**Independent Test**: `grep -rn 'Tool = "gatling"' --include='*.go'` finds one declaration; a table
walking all eleven exported enums at an out-of-range value shows one rule; a text log and a binary log
naming the same above-range version produce identical `Run().Warnings`.

**Acceptance Scenarios**:

1. **Given** a consumer importing only `gatling`, **When** it names the tool, **Then** it can do so
   without importing either codec, and the two codec-local constants are gone and recorded under
   Removed.
2. **Given** each of the eleven exported enums at a value outside its known set, **When** `String()`
   is called, **Then** every rendering names the type and the number — `Outcome(99)` as well as
   `Kind(99)` — produced by one idiom.
3. **Given** the zero value of each of those eleven enums, **When** `String()` is called, **Then** it
   still renders its own documented name, unchanged.
4. **Given** a text log and a binary log naming the same above-range version, **When** each run's
   `Warnings` are compared, **Then** they are identical, including the reason text.
5. **Given** the two `NewRunReader` bodies, **When** they are diffed, **Then** nothing differs but the
   constructor call and the capability set.
6. **Given** `gatling/text`'s existing tests, **When** the extraction lands, **Then** they pass
   unedited.

---

### User Story 3 - A log handed to the wrong reader is told so, not called damaged (Priority: P1)

An engineer with a mixed archive opens a 3.14 binary log with the text codec. The error names the
binary format and points at the package that reads it. Nothing in the archive is quarantined as
corrupt for being the other format.

**Why this priority**: this is the most likely first failure a new consumer meets, and it is sorted
into the wrong error type — `*gatling.SyntaxError`, documented as "the position it names could not be
decoded". A caller branching on error type mis-handles every file of the other format.

**Independent Test**: `text.NewReader` on a binary corpus log and `binary.NewReader` on a text corpus
log each return an error naming the format found; a genuinely damaged log of the right format still
returns a `*gatling.SyntaxError`.

**Acceptance Scenarios**:

1. **Given** `text.NewReader` on `testdata/corpus/gatling/3.14.9/simulation.log`, **When** the reader
   is constructed, **Then** the error names the binary format, names the package that reads it, and is
   not a `*gatling.SyntaxError`.
2. **Given** `binary.NewReader` on `testdata/corpus/gatling/3.11.5/simulation.log`, **When** the reader
   is constructed, **Then** the same holds in reverse.
3. **Given** a truncated or corrupt log of the format the reader does take, **When** it is read,
   **Then** a `*gatling.SyntaxError` is still returned.
4. **Given** either wrong-format construction, **When** the source is counted, **Then** the head bytes
   are not read twice: both codecs decide from bytes they have already consumed.
5. **Given** a file that is not a Gatling simulation.log at all, **When** it reaches `simlog`,
   **Then** its existing behaviour is unchanged.

---

### User Story 4 - A span never ends before something it counted began (Priority: P2)

A consumer folds a run in which the last sample has no recorded end, divides the count by the span,
and gets a rate it can defend. The span does not exclude an item the same fold counted.

**Why this priority**: `Bounds` is a frozen primitive and galaxio-cli#61 makes it the source of the
run's bounds for every report. The decision taken here — an end-less item extends the end to its own
start — moves a number every consumer divides by, so it has to land before the tag and be recorded
under Changed.

**Independent Test**: the two-item fold — a sample at 00:00:10 lasting 5 s, and a sample at 00:00:20
with no end — returns `End() = 00:00:20`, asserted by a test, and `Bounds`' doc comment states that
rule.

**Acceptance Scenarios**:

1. **Given** a fold over a 5 s sample starting at 00:00:10 and an end-less sample starting at
   00:00:20, **When** `Start()` and `End()` are read, **Then** they report 00:00:10 and 00:00:20, and
   a test asserts it.
2. **Given** any fold, **When** `End()` is compared against the start of every item the fold counted,
   **Then** it is never earlier than any of them.
3. **Given** `Bounds`' doc comment, **When** a consumer reads it, **Then** it states that an item
   whose end the source did not record is known to have been running at its own start instant, and
   the old paragraph saying such an item contributes only its start is gone.
4. **Given** the `end.Before(start)` guard, **When** the change lands, **Then** it is unreachable and
   is removed together with its doc paragraph.
5. **Given** the release, **When** the changelog is read, **Then** the moved end is recorded under
   Changed.
6. **Given** a run in which `UserStart`/`UserEnd` move the bounds, **When** it is folded, **Then**
   their effect is unchanged.

---

### User Story 5 - What the module publishes about itself is true, and it routes to the entry point (Priority: P2)

A newcomer lands on any of the six package pages, believes what it says, and arrives at the intended
three-call path — `run.Find`, then `simlog.NewRunReader`, then a fold over `Next` — rather than at the
wire records or at nothing.

**Why this priority**: Principle V freezes a doc comment with the identifier it documents. These are
the pages three consumers read for the life of the v0.1.x line, and four of them currently misroute or
misstate.

**Independent Test**: `go doc` over every exported identifier in the six packages contains no literal
`//` and no collapsed paragraph; each package overview names where to start; the two `Warning` types
link to each other.

**Acceptance Scenarios**:

1. **Given** `go doc` over every exported identifier in `model`, `gatling`, `gatling/text`,
   `gatling/binary`, `gatling/simlog` and `gatling/run`, **When** the output is inspected, **Then** no
   rendering contains a literal `//` or a collapsed paragraph.
2. **Given** `SyntaxError.Error`'s doc comment read alone, **When** a consumer writes a message for a
   binary failure, **Then** it renders a byte offset rather than a line number.
3. **Given** `TruncationError`'s doc comment, **When** it is read, **Then** the boundary-cut fact is
   stated once in it.
4. **Given** `go doc` for `text.Reader`, `text.RunReader`, `binary.Reader`, `binary.RunReader`,
   `simlog.RecordReader` and `simlog.RunReader`, **When** each is read, **Then** each states that a
   value may be used by one goroutine at a time and says what a concurrent call can cost — the string
   table is a map, so it can end the process rather than return a wrong result.
5. **Given** a consumer reading only the `simlog` interface documentation, **When** it plans a
   follower, **Then** it learns the constraint without ever seeing the concrete type.
6. **Given** `go doc github.com/galax-io/parsec/gatling Warning` and the same for `model`, **When**
   each is read, **Then** each names the other and says how a value crosses, and
   `model.Warning.Version` explains why it is a string.
7. **Given** `go doc` for each package, **When** it is read, **Then** none claims a shipped package or
   type is future work.
8. **Given** `go doc github.com/galax-io/parsec`, **When** it is read, **Then** every package in the
   module is listed, `gatling/run` included, and the three-call entry path is named.
9. **Given** each codec's overview, **When** it is read, **Then** it names both entry points, and
   identifiers named in package prose are doc-linked consistently across the six overview files.

---

### User Story 6 - Every test that pins the frozen surface can go red (Priority: P2)

A maintainer breaks a documented guarantee on purpose. The suite fails. It does not skip, and it does
not pass because the assertion restates the fixture.

**Why this priority**: Principle III is NON-NEGOTIABLE, and two of the six tests cover public
guarantees that freeze at the tag. The `Groups` reuse guarantee is stated on six surfaces and its only
test turns green when the guarantee breaks.

**Independent Test**: each of the six is demonstrated by breaking the production code and watching the
test go red.

**Acceptance Scenarios**:

1. **Given** the binary reader changed to return a fresh group path each call, **When** the suite runs,
   **Then** the reuse test fails rather than skipping, and it copies before it compares.
2. **Given** the meaning of `model.Sample.Outcome` changed, **When** the suite runs, **Then** a test in
   `model` fails rather than asserting an identity of its own filter.
3. **Given** the corpus-backed selection test, **When** it runs, **Then** it asserts the module's
   claim about failure selection rather than a sequence true of any decoder output.
4. **Given** the `model.Item` test, **When** it runs, **Then** it asserts something other than Go
   zeroing the fields a literal did not set.
5. **Given** the `Opt` accessor test, **When** it runs, **Then** it carries no assertion that cannot be
   reached, and the danger it names is either tested where it is testable or dropped.
6. **Given** the suite run as root — an ordinary Docker CI container — **When** the run-discovery
   permission test executes, **Then** it either skips through `requireUnixPermissions` or still tests
   both fixtures, and a regression in which `Find` opens each log is caught.

---

### User Story 7 - A stranger installs it, calls it, and is told the truth about versions (Priority: P3)

Someone with a `simulation.log` and no prior contact with this project reads `README.md`, runs
`go get`, pastes the first program, and reads their run. The compatibility table answers "does this
read my version?" correctly, including why 3.13.0 is refused.

**Why this priority**: the README is the landing page of a library about to be pinned by three
codebases, and it currently contains no code, no install line, a table wrong at both ends, and — as
its one named read call — the codec that cannot read the format it opens by promising to rescue.

**Independent Test**: a Go engineer with a `simulation.log`, given only `README.md`, writes a working
program without opening another file.

**Acceptance Scenarios**:

1. **Given** only `README.md`, **When** an engineer follows it, **Then** they install the module, see
   the minimum Go version, and compile a copy-pasteable first program that reads a run.
2. **Given** the README's named default read call, **When** a consumer follows it for an archived run
   of unknown format, **Then** it is `simlog.NewRunReader`, with the codec packages presented as the
   shortcut for a known version.
3. **Given** the compatibility table, **When** a 3.13.0 user reads it, **Then** they are told the log
   is refused and why, the range reads 3.13.1 through 3.15.1, and the reader is pointed at
   `simlog.Supported()` rather than asked to trust prose.
4. **Given** the README, **When** it is searched for per-package version stamps and the "Status"
   narrative, **Then** they are gone, and the compatibility promise from User Story 1 is stated where
   `doc.go` already points.
5. **Given** the 0.0.9 changelog entry, **When** the run-ordering rule is read, **Then** it is stated
   once and states modification time, then the run id's own UTC stamp, then the directory name.
6. **Given** a reader following that entry for a root of mixed simulation ids with equal modification
   times, **When** they predict what `Find` returns, **Then** they are right.
7. **Given** `gh api repos/galax-io/parsec`, **When** the description is read, **Then** it does not
   promise statistics, and the homepage points at pkg.go.dev.
8. **Given** the repository, **When** a researcher who has fuzzed a hostile log into a crash looks for
   somewhere to report it, **Then** `SECURITY.md` names a private channel, says what to expect, and
   states which versions receive fixes in terms of the release policy.

---

### User Story 8 - The release path cannot be moved under it (Priority: P3)

The job that cuts releases runs only code the repository chose. A dispatch input containing a quote
never reaches a shell.

**Why this priority**: the moved-tag half needs no access to this repository at all, and it lands in
the job that holds `contents: write` and a token that can publish. The repository already pins its
non-action tooling by exact version, so the convention exists and the actions are the gap.

**Independent Test**: `grep -rn 'uses: ' .github/workflows/` shows a 40-character SHA for every
third-party entry; a dispatch input carrying a quote is refused before any shell runs.

**Acceptance Scenarios**:

1. **Given** every workflow, **When** its `uses:` lines are listed, **Then** each third-party action
   names a full commit SHA with the version in a trailing comment.
2. **Given** `.github/dependabot.yml`, **When** an action update is proposed, **Then** it updates the
   SHA rather than the tag.
3. **Given** a `workflow_dispatch` input containing a quote, **When** the canary or the nightly fuzz
   workflow runs, **Then** it is refused before any shell executes, using the validation
   `record-corpus.yml` already carries and not a second pattern.
4. **Given** the workflows after the change, **When** they run, **Then** they do what they did before.

---

### Edge Cases

- **A version below the supported range.** Unchanged: refused with an error naming the version found
  and the range supported, on both codecs, before the rest of the run record is judged.
- **A version above the supported range.** Unchanged: decodes with a warning, or fails under strict
  mode. The warning's reason text becomes identical across the two codecs (User Story 2), which is the
  only observable change.
- **A malformed log of the right format.** Unchanged: a `*gatling.SyntaxError`. User Story 3 must not
  widen the wrong-format answer into this case.
- **A file that is not a Gatling simulation.log at all.** Unchanged: `simlog` already returns "not a
  Gatling simulation.log" naming the head bytes; nothing in this feature touches it.
- **An out-of-range enum value a consumer produced by casting an integer.** Renders as the type name
  and the number, on all eleven exported enums; `model.Outcome(99)` stops rendering as `unknown`.
- **A run whose last item has a start and no recorded end.** The item extends the end to its own
  start, so `End()` no longer precedes a start the fold counted; this is the one number this feature
  moves, and it is recorded under Changed.
- **A run with zero items.** Unchanged: `Bounds.Start()` and `End()` report absent; the User Story 4
  change does not reach this case.
- **A reader shared between goroutines.** Not made safe; documented as unsupported, with what it costs
  stated (User Story 5). The text interner's map means the process can die rather than return a wrong
  number.
- **A binary log's `Record.Line`.** Zero, and that is the contract; a failure's position is carried by
  `SyntaxError.Offset`.
- **The suite run as root, or on a platform without POSIX mode bits.** The permission fixture either
  denies or the test skips; it never passes vacuously (User Story 6).

## Requirements *(mandatory)*

### Functional Requirements

**The contract (#13, #107)**

- **FR-001**: The stable surface MUST be listed identifier by identifier in a contract artefact under
  this feature's directory, and that list MUST match `go doc -all` over `model`, `gatling`,
  `gatling/text`, `gatling/binary`, `gatling/simlog` and `gatling/run`, with the count recorded.
- **FR-002**: Everything outside the listed surface MUST be unexported or moved under `internal/`.
- **FR-003**: No exported identifier MAY remain whose only callers are inside this module, unless a
  consumer's recorded plan names it.
- **FR-004**: `gatling.Gate` MUST be unexported; its behaviour MUST remain tested through
  `Policy.Apply`, which the code already documents as the single place the outcomes are decided.
- **FR-005**: `gatling.MaxRunStart` MUST move to `internal/wire`, which both codecs already import;
  the refusal it implements MUST stay documented on the readers.
- **FR-006**: The compatibility promise MUST state that from v0.1.0 a breaking change to the listed
  surface requires a MINOR bump while the module is below v1, a `CHANGELOG.md` entry, and a
  `// Deprecated:` window of at least one MINOR release before a removal.
- **FR-007**: The supported Gatling range MUST be documented as part of the contract — 3.11.5 through
  3.12.0 for the text format, 3.13.1 through 3.15.1 for the binary one — and MUST be derivable from
  `simlog.Supported()` rather than only from prose.
- **FR-008**: `simlog.RecordReader` and `simlog.RunReader` MUST be frozen two-sided: their method sets
  are final at the tag, because consumers' test doubles implement them.
- **FR-009**: `gatling.UnsupportedFormatError` MUST be kept and MUST have producers (FR-017, FR-018); its
  message MUST NOT claim the module has no codec for the format.
- **FR-010**: `gatling.SyntaxError` MUST keep `Line`, `Offset` and `Format` as three fields, and the
  reason MUST be stated in its doc comment: `Format` is the discriminator two legitimately-zero
  positions need.
- **FR-011**: `gatling.Record.Line` MUST be documented as 0 for every record of a binary log, and the
  reason MUST be stated: a binary record's offset serves no seek, and a failure's position is carried
  by `SyntaxError.Offset`.
- **FR-012**: Every removal in this feature MUST be recorded under Removed in `CHANGELOG.md`, and
  every behavioural change under Changed.

**One name, one rule, one copy (#77, #78, #58)**

- **FR-013**: Exactly one exported constant MUST carry the value `"gatling"`, in `gatling`, reachable
  without importing either codec; both codec-local copies MUST go.
- **FR-014**: An out-of-range value of an exported enum MUST render as the type name and the number —
  `Outcome(99)`, `Kind(99)` — across `model`, `gatling` and `gatling/run`, implemented by one idiom,
  stated in every affected `String()` doc comment, and applied to all eleven enums. This takes the
  `gatling` convention everywhere and changes the five `model` methods: a value the module cannot
  name stops rendering identically to a value that means the source lost it, the number survives a
  `%v` into a log, and the three current idioms — bounds-checked array lookup, `switch` with a
  `default`, `switch` falling through to `unknownName` — collapse to one.
- **FR-015**: What an in-range value renders as MUST NOT change, and the zero value of each enum MUST
  keep rendering its own documented name.
- **FR-016**: The `NewRunReader` warning text MUST exist in exactly one place, with the warnings loop
  and the run-header construction shared by both codecs; `gatling/text`'s existing tests MUST pass
  unedited.

**The wrong-codec failure (#84)**

- **FR-017**: A codec handed a log of the other format MUST NOT return an error that says the log is
  damaged.
- **FR-018**: That error MUST name the format found and SHOULD name the package that reads it.
- **FR-019**: The head bytes MUST NOT be read twice; both codecs decide from bytes already consumed.
- **FR-020**: A genuinely damaged log of the format the reader does take MUST still return a
  `*gatling.SyntaxError`, and `simlog`'s behaviour MUST NOT change.

**The bounds definition (#103)**

- **FR-021**: An item with a start and no recorded end MUST extend the run's end to its own start, so
  `Bounds.End()` is never earlier than the start of an item the fold counted. The rule MUST be
  recorded in `Bounds`' doc comment, replacing the paragraph that says such an item contributes only
  its start, and MUST be pinned by a test covering a start-only item later than every recorded end.
- **FR-022**: The change MUST be recorded under Changed in `CHANGELOG.md` — it moves a number every
  consumer divides by — and the `end.Before(start)` guard that becomes unreachable, together with its
  doc paragraph, MUST be removed with it.
- **FR-023**: How `UserStart` and `UserEnd` move the bounds MUST NOT change.

**Documentation that freezes with the identifiers (#79, #85, #86, #96)**

- **FR-024**: `go doc` output for every exported identifier in the six packages MUST contain no
  literal `//` and no collapsed paragraph.
- **FR-025**: `SyntaxError.Error`'s doc comment MUST describe both renderings or defer to the type's.
- **FR-026**: A fact stated in a doc comment MUST be stated once in that comment.
- **FR-027**: Every exported reader type and both `simlog` interfaces MUST state that a value may be
  used by one goroutine at a time, beside the aliasing rule they already carry, and MUST say what a
  concurrent call costs rather than only that it is unsupported.
- **FR-028**: `gatling/simlog/doc.go`'s follower contract SHOULD state the same constraint.
- **FR-029**: Each `Warning` doc comment MUST name the other and say how a value crosses between them;
  `model.Warning.Version`'s doc MUST say why it is a string, so a consumer does not order two of them
  as text.
- **FR-030**: No package overview MAY state that a shipped package or type is future work.
- **FR-031**: The root `doc.go` MUST list every package in the module, and the root `doc.go` and
  `model/doc.go` MUST name the intended path: `run.Find` → `simlog.NewRunReader` → a fold over `Next`.
- **FR-032**: Each codec overview MUST name both entry points, and identifiers named in package prose
  MUST be doc-linked consistently across the six overview files.

**Tests that can fail (#95)**

- **FR-033**: A test MUST be able to fail; where a property is environment-dependent, the test MUST
  assert the property it can see rather than skipping out of the assertion.
- **FR-034**: A test that names a public guarantee MUST fail when that guarantee breaks — specifically
  the `Groups` reuse guarantee, and the failure-selection claim in `model`.
- **FR-035**: A test whose fixture depends on POSIX mode bits MUST call `requireUnixPermissions`.
- **FR-036**: Each of the six fixes MUST be demonstrated by breaking the production code and watching
  the test go red.

**The landing pages and the release path (#99, #97, #101, #93)**

- **FR-037**: `README.md` MUST open with what the library is, `go get`, the minimum Go version, and a
  copy-pasteable first program; the program SHOULD be inlined from `gatling/simlog/example_test.go`,
  which `go test` already compiles and output-checks, so it cannot rot.
- **FR-038**: `simlog.NewRunReader` MUST be the named default in `README.md`; the codec packages are
  presented as the shortcut when the version is already known.
- **FR-039**: The compatibility table MUST state 3.13.1 through 3.15.1, carry the reason 3.13.0 is
  refused, and point at `simlog.Supported()`.
- **FR-040**: Per-package version stamps and the "Status" narrative MUST go from `README.md`; the
  compatibility section MUST exist where `doc.go` already points at it.
- **FR-041**: The GitHub repository description MUST NOT promise statistics, and `homepage` SHOULD be
  the pkg.go.dev URL.
- **FR-042**: `CHANGELOG.md`'s 0.0.9 entry MUST state the run-ordering rule the code implements —
  modification time, then the run id's own UTC stamp, then the directory name — once.
- **FR-043**: `SECURITY.md` MUST exist, naming how to report privately and what to expect, and MUST
  state which versions receive fixes consistently with the release policy; GitHub's private
  vulnerability reporting MUST be enabled, and the community profile MUST report the file.
- **FR-044**: `SECURITY.md` SHOULD name the trust boundary plainly: this module decodes files it does
  not trust, in a process it does not own.
- **FR-045**: Every third-party action MUST be pinned to a full commit SHA with the version in a
  trailing comment; `actions/*` SHOULD be too; `.github/dependabot.yml` MUST update SHAs. A job's
  `container:` and every `services.<id>.image:` MUST be pinned the same way, by `@sha256:` digest: a
  step's action is one step's code, but the job's container is the filesystem and the interpreter every
  `run:` in that job executes inside, so a movable tag there is worth more than a movable action.
- **FR-046**: No `${{ … }}` substitution of any context MAY reach a `run:` line or a `run:` body. The
  value MUST be bound in `env:` and read back as a quoted `$VAR`, or taken from a variable the runner
  already exports, and where it needs validating the validation MUST be the one `record-corpus.yml`
  already uses.

  *Amended after the gate shipped.* This requirement first named only `${{ inputs.* }}` and matrix
  values derived from one, and `scripts/check-pins.sh` implemented exactly that — so `github.event.*`,
  `github.head_ref` and `steps.*.outputs.*` passed a gate written to stop textual substitution into a
  shell. The hazard the rationale states is a property of the *value*, which no gate can see, never of
  the context's name, and a context that is safe today stops being safe in a later commit that the
  `run:` line's own diff does not touch. There is no allowlist: every context can be bound in `env:`,
  and the ones worth exempting (`$GITHUB_EVENT_NAME`, `$RUNNER_OS`, `$RUNNER_TEMP`, `$GITHUB_SHA`) are
  already exported by the runner, so an entry would only ever permit a spelling strictly worse than one
  that always exists.
- **FR-047**: No workflow's behaviour MAY change as a result of FR-045 and FR-046. Four substitutions in
  three `run:` bodies were rewritten under the amended FR-046; each substitutes a value byte-identical
  to what GitHub would have inlined, so this holds.
- **FR-048**: `scripts/check-pins.sh` MUST decide FR-045 and FR-046 from the parsed YAML, not from the
  text of the lines, and MUST refuse a file it cannot parse rather than reporting success on it.

  A line matcher cannot tell a `uses:` key from the same text inside a scalar, and both halves of that
  failed: it refused a workflow because a step was *named* `"assert that nothing uses: a floating tag"`,
  reporting the English phrase as an unpinned action, and it passed `? uses` / `: actions/evil@main` —
  YAML's explicit-key spelling of an ordinary mapping, which PyYAML, Psych, go-yaml and actionlint all
  resolve identically, and whose parser event stream is byte-identical to the ordinary form, so no
  event-driven reader can distinguish them. Lone `\r` line endings, a `\uXXXX`-escaped key and a
  flow-style `steps:` failed the same way. Widening the patterns cannot fix this: refusing the text
  inside a scalar and reading the key in a flow mapping are contradictory requirements for a regular
  expression, and the same requirement for a parser.

  Attacking the parser in turn found what parsing does not fix by itself: the *walk* is the rule's real
  scope, and it read only `jobs.<id>.{uses, steps}`. A job `container:`, a `services.<id>.image:`, a
  `docker://` step's `args:`, and — because GitHub repository names are case-insensitive while the
  comparison was not — `actions/GitHub-Script`'s `script:` all passed. So this requirement covers the
  walk as well as the parse: every place a workflow names code to fetch, or hands a value to an
  interpreter, MUST be visited, and a mapping whose keys the gate will not guess about — a duplicate
  key, a `<<` merge — MUST be refused rather than resolved by guess.

### Key Entities

- **The contract table**: the per-identifier list of the promised surface, held under this feature's
  `contracts/`, against which `go doc -all` is compared. It is the freeze; a description is not.
- **Exported reader**: `text.Reader`, `text.RunReader`, `binary.Reader`, `binary.RunReader` and the
  two `simlog` interfaces they satisfy — the six surfaces that carry the aliasing rule and must carry
  the concurrency rule.
- **`gatling.Warning` and `model.Warning`**: the wire-side warning, carrying an ordered `Version` and
  the range it fell outside, and its canonical form, carrying the release as text and a `Reason`. One
  `simlog` call apart, permanently, cross-linked rather than unified.
- **`model.Bounds`**: the run's span, folded from items; the primitive galaxio-cli divides by.
- **Enum with a `String()`**: eleven exported types — six in `gatling` and `gatling/run`, five in
  `model` — whose out-of-range rendering is observable behaviour that freezes at the tag.
- **Error types on the frozen surface**: `FormatError`, `UnsupportedFormatError`, `SyntaxError`,
  `TruncationError`, `VersionError`, `UnverifiedError` — what a consumer branches on, and what decides
  whether a file is quarantined as corrupt.

### Source Coverage

- **Tool and versions**: Gatling 3.11.5 through 3.12.0 (text), 3.13.1 through 3.15.1 (binary).
  Unchanged by this feature; stated here because the contract must name it.
- **Artefact formats**: text `simulation.log`; binary `simulation.log` from 3.13.1.
- **Version gate**: unchanged. Below the range is refused with a `*gatling.VersionError`; above it
  decodes with a warning, or fails under strict mode. This feature makes the warning's reason text
  identical across the two codecs and adds no new verdict.
- **Not provided by this source**: unchanged; `Capabilities` is not touched, and the two codecs' sets
  stay separate and asserted equal, as #58 and #77 both leave deliberately.
- **Golden corpus**: unchanged — `testdata/corpus/gatling/{3.11.5,3.12.0,3.13.1,3.14.9,3.15.1}`. This
  feature records no new run. It uses the existing corpus in two new ways: the 3.14.9 binary log and
  the 3.11.5 text log become the wrong-codec fixtures for User Story 3, and the corpus-backed
  selection test in `gatling/text` is repaired rather than replaced.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A program importing only the identifiers the contract table names compiles against the
  v0.1.0 tag with zero references to any `internal/` package.
- **SC-002**: The contract table and `go doc -all` over the six packages agree on every name, in both
  directions, and the count is recorded in the release.
- **SC-003**: Zero exported identifiers remain whose only callers are inside this module and which no
  consumer's recorded plan names.
- **SC-004**: All eleven exported enums render an out-of-range value as the type name and the number,
  produced by one idiom, and all eleven still render their zero value as before.
- **SC-005**: Exactly one declaration of the tool name exists in the module.
- **SC-006**: For a text log and a binary log naming the same above-range version, the two runs'
  warnings are identical, and the two `NewRunReader` bodies differ only in the constructor call and
  the capability set.
- **SC-007**: Each codec, handed a corpus log of the other format, returns an error naming the format
  found and not a `*gatling.SyntaxError`; a damaged log of the right format still returns one.
- **SC-008**: `Bounds.End()` is never earlier than the start of any item the fold counted, including
  a start-only item later than every recorded end, and a test fails if it stops holding.
- **SC-009**: `go doc` over every exported identifier in the six packages produces no literal `//` and
  no collapsed paragraph, and every reader surface states the one-goroutine constraint.
- **SC-010**: A reader landing on any of the six package pages is told where to start, and no page
  claims a shipped thing is future work.
- **SC-011**: Each of the six tests in #95 goes red when the production code it names is broken, and
  the run-discovery permission test does not pass vacuously as root.
- **SC-012**: A Go engineer with a `simulation.log` and only `README.md` writes a working program
  without opening another file, and a 3.13.0 user learns from the table that their log is refused and
  why.
- **SC-013**: A reader following the 0.0.9 changelog entry predicts what `Find` returns for a root of
  mixed simulation ids with equal modification times.
- **SC-014**: The repository reports a `SECURITY.md` in its community profile, accepts private reports,
  and its description no longer promises statistics.
- **SC-015**: Every third-party action in every workflow names a 40-character SHA, and a dispatch
  input containing a quote is refused before any shell runs.
- **SC-016**: `go build ./... && go test ./...` is green with the race detector on, coverage stays at
  or above 90 percent for the decoder packages and 80 percent overall, and `go mod tidy` leaves the
  tree unchanged.

## Assumptions

- **The whole milestone is in scope.** The user gave the milestone URL with no subset, unlike the
  previous feature, which named nine issues. All sixteen open issues are specified here. #12 is
  closed and moved to galaxio-cli#61; it is not revisited.
- **v0.0.10 has shipped**, so everything spec 010 covered is on `main` and is not re-specified. This
  feature starts from that state.
- **The tag is cut after every story lands.** Nothing here is deferrable past v0.1.0 without freezing
  the defect it fixes, which is why all eight stories are release-blocking regardless of priority.
- **The release mechanics are the ones in `AGENTS.md`**: a `release/0.1.0` branch cut from `main`, the
  tag on that branch, every merged PR assigned to the milestone and every fixed issue closed before
  the tag.
- **No new dependency.** Principle IV admits none, and nothing in these sixteen issues needs one —
  `stringer` is explicitly ruled out by #78 as a build-time dependency.
- **The contract table is generated from `go doc -all`, not hand-typed.** A hand-typed list drifts on
  the first merge; the check that the table and the tool agree is what makes the freeze real.
- **The three consumers' plans are the authority on what must stay exported** — galaxio-cli#50 and
  #61, comet#3, backend#230 — as #107's inventory used them. Where a plan does not name an identifier
  and no contract under `specs/*/contracts/` does either, it is a candidate for unexporting.
- **The wire path stays public.** `simlog.NewReader`, `gatling.Header`, `gatling.Record`, `Kind`,
  `Status`, `Event` and `AbsentTimestamp` are promised, because galaxio-cli's `report dump` emits
  them as JSONL and the sidecar will emit the same schema.
- **`v0.1.0` is not `v1.0.0`.** The promise is "no breaking change without a MINOR bump and a
  deprecation window", not a commitment to the surface for all time.

## Dependencies

- **#107 is #13's concrete scope**, and #13 is the release note #107 produces. They land together or
  #13 has nothing to list.
- **#84 depends on the `UnsupportedFormatError` decision in #107**, which keeps the type precisely so
  that #84 has an error meaning "a Gatling log this reader does not decode" that is not
  `FormatError`'s "not a Gatling file at all".
- **#77 and #58 both touch both `NewRunReader` bodies.** Landing #58 first leaves #77 a one-line
  change in the shared builder; landing #77 first leaves #58's diff smaller. Either order works, but
  they should not be in flight simultaneously.
- **#78 and #107 both change `gatling` and `model` broadly**; the enum rule should be settled before
  the contract table is generated, or the table is regenerated twice.
- **#99 depends on #13**: the README's compatibility section is #13's consumer-facing half, and
  `doc.go:22` already points at a section that does not exist.
- **#101 depends on the release policy** in `AGENTS.md` for the supported-versions statement: patch
  releases are cherry-picked onto `release/X.Y.0`, so the supported set is expressible.
- **#95's `Groups` reuse test depends on nothing else**, but it guards a guarantee #13 freezes, so it
  should land before the tag rather than after.
- **#59 is not a dependency.** The corpus test-helper duplication is milestone v0.6.0; #95 explicitly
  leaves it alone.

## Out of Scope

- **Making the readers safe for concurrent use.** #85's non-goal: a decoder over a single stream has
  no reason to be, and a mutex on `Next` costs every consumer for a case none of them wants. The
  constraint is documented, not removed.
- **Renaming or unifying either `Warning`.** #86's non-goal; a sentence and two doc links solve what a
  rename would cost a deprecation window.
- **Changing the shape of `SyntaxError`, or what any in-range enum value renders as.** #107 confirms
  the first; #78 excludes the second.
- **Moving `Policy`, `Verdict` or `Version` behind `internal/`.** #107's non-goal: a future codec must
  call the one gate, and hiding it drags `Option` resolution with it.
- **Moving `Capabilities`.** #58 and #77 both leave it: the two sets are asserted equal by a test and
  may legitimately diverge later.
- **Making either codec read the other format, or changing `simlog`'s behaviour.** #84's non-goals;
  `simlog` is already right.
- **Changing how `UserStart`/`UserEnd` move the bounds.** #103's non-goal.
- **Rewriting the corpus, or the test-helper duplication of #59.** #95's non-goals; #59 is v0.6.0.
- **Rewriting the type-level documentation**, which #96 calls good; only the overviews misroute.
- **Restructuring `CHANGELOG.md`**, which #97 and #99 both call good.
- **`CODE_OF_CONDUCT.md`, a PR template, an issue template.** #101's non-goal; nothing here turns on
  them. `CONTRIBUTING.md` is marginal — `AGENTS.md` and the constitution carry the rules, and a
  one-line README pointer costs nothing.
- **Changing what any workflow does.** #93's non-goal; pinning and validation only.
- **A v1.0.0 commitment.** #13's non-goal.
- **Any new statistic, model field, option or exported identifier that a consumer has not asked for.**
  Principle I forbids the arithmetic; this feature reduces the surface and makes what remains honest.
