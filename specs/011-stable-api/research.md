# Research: A stable API

**Feature**: 011-stable-api | **Date**: 2026-09-12 | **Constitution**: v2.4.0

Every finding below was taken from the working tree at `0a55f09` (`docs(changelog): stamp 0.0.10`),
not from the issue text. Where the two disagree, the tree wins and the difference is recorded.

---

## R1 — How the contract table is produced

**Decision**: generate the promised surface from the source with `go/parser`, and hold it in a
golden file that a module-root test compares against. `testdata/api/surface.txt`, one line per
exported identifier, sorted, with a package header carrying the count.

**Rationale**: #107 asks that the freeze be "a list and not a description", and FR-001 requires the
spec's table and the tool to agree. A table typed by hand agrees on the day it is written and drifts
on the first merge. A golden file makes any change to the surface — an addition as much as a
removal — a red test whose diff is the changelog line the author then has to write.

The prototype is written and run. Parsing the six packages with `go/parser` (test files excluded)
and collecting exported funcs, methods, types, struct fields, interface methods, consts and vars
yields:

```
model 111   gatling 107   gatling/text 14   gatling/binary 14   gatling/simlog 16   gatling/run 16
TOTAL 278
```

which is #107's inventory to the identifier, per package. The issue's number is therefore verified,
not inherited.

**Alternatives rejected**:

- *Parse `go doc -all`*. Its grouping of const and var blocks, and its rendering of struct fields,
  are presentation and change with the toolchain; the count would be an artefact of formatting.
- *`golang.org/x/exp/apidiff` or `golang.org/x/tools/go/packages`*. A dependency, and Principle IV
  admits none — least of all one that would appear in three consumers' module graphs.
- *Let `gorelease` be the list*. It is not one. `gorelease` reports what **changed** against the last
  tag; it says nothing about what the surface **is**, and an addition passes it silently. The two
  are complementary and both are kept: `gorelease` gates compatibility, the golden file gates the
  inventory.

## R2 — The gate a surface change already has to pass

**Finding**: `scripts/check-compat.sh` (landed with #106, PR #109) reads a `gorelease` report and
fails any incompatible change unless the pull request carries the `breaking` label **and**
`CHANGELOG.md` records the change under `[Unreleased]` as `### Changed` or `### Removed`. The label
exists on the repository (`breaking` — "A deliberate incompatible change to the public API: MINOR
bump, changelog entry required"). `[Unreleased]` is currently empty.

**Consequence for the plan**: every PR in this feature that unexports, moves or re-renders an
exported identifier is pre-priced — it carries the label and an `[Unreleased]` bullet, or it does not
merge. This is not extra process to design; it is the mechanism 010 built, and the plan reuses it.
Nothing here needs a second gate for compatibility. The golden file of R1 is the gate for the
*inventory*, which `gorelease` does not cover.

## R3 — Unexporting `gatling.Gate`

**Callers**: `gatling/policy.go:41` (`switch verdict := Gate(found, p.Min, p.Max); verdict {`) in
production, and `gatling/version_test.go:118` in tests. Nothing else in the module, and no consumer
plan names it.

**Decision**: rename to `gate`. The table-driven case in `version_test.go` drives it through
`Policy.Apply`, which is what `policy.go`'s own doc comment calls "the single place the outcomes are
decided" — the test then asserts the documented entry point rather than a second one.

**Note**: `Policy`, `Verdict` and `Version` stay exported. #107 rules the alternative out: a future
codec must call the one gate, and hiding `Policy` drags `Option` resolution with it.

## R4 — Moving `gatling.MaxRunStart`

**Callers**: `gatling/binary/record.go:147`, `gatling/text/parse.go:483` and `:487`. Both codecs
already import `internal/wire`. One test comment refers to it (`gatling/text/parse_test.go:51`).

**Decision**: `wire.MaxRunStart`, same value (`math.MaxInt64 - math.MaxInt32`), same doc comment. The
refusal it implements stays documented on the two readers, so a consumer still learns the ceiling
from the API it calls.

## R5 — One `Tool`

**Finding**: `gatling/text/model.go:83` and `gatling/binary/capability.go:9`, both
`const Tool = "gatling"`, both with the same doc comment, in files that are not the same kind of
file — so neither placement is the convention.

**Decision**: one `gatling.Tool`, beside `Header` and `Version`, which both codecs already import.

**Ordering**: land #58 first. Once the run-header construction is shared (R6), the two `Tool` reads
collapse into one line of the shared builder and #77 is a two-line removal. Landing #77 first leaves
#58 a slightly smaller diff but leaves `Tool` threaded through both copies in the meantime. Either
order works and they must not be in flight at once, since both rewrite both `NewRunReader` bodies.

## R6 — What #58 extracts, and where

**Finding**: `gatling/text/model.go:39-79` and `gatling/binary/model.go:30-71` are byte-identical
apart from one comment above `ID:` — the text copy explains that Gatling's run identifier is the
simulation's, the binary copy that the binary log records none of its own. Everything else, including
the reason string `"no recording covers it — the verified range is … through …, so the records
decode unverified"`, is duplicated.

**Decision**: two functions in `internal/wire`:

```go
func Warnings(ws []gatling.Warning, oldest, newest gatling.Version) []model.Warning
func Run(h gatling.Header, caps model.Capabilities, ws []model.Warning, assertions [][]byte) model.Run
```

`Run` reads `gatling.Tool` itself (R5), so neither codec names the tool. `Capabilities` is **not**
moved: #58 and #77 both leave it, because the two sets are asserted equal by a test and may
legitimately diverge.

`Warnings` keeps the nil-when-empty behaviour both copies document ("a caller reading
`len(run.Warnings) == 0` and one reading `run.Warnings == nil` must agree").

**Acceptance already stated**: `gatling/text`'s tests pass unedited, as they did for the v0.0.5
extraction.

## R7 — The enum rule, and which of the three idioms survives

**The rule** (spec FR-014, decided by the user on 2026-09-12): an out-of-range value renders as the
type name and the number. This takes the `gatling` convention into `model`.

**The eleven**, with what each does today:

| Enum | File | Today, out of range | Idiom |
|---|---|---|---|
| `gatling.Kind` | `gatling/record.go:30` | `Kind(99)` | table + bounds check |
| `gatling.Format` | `gatling/format.go:27` | `Format(99)` | table + bounds check |
| `gatling.Verdict` | `gatling/version.go:85` | `Verdict(99)` | switch + `default` |
| `gatling.Status` | `gatling/record.go:51` | `Status(99)` | switch + `default` |
| `gatling.Event` | `gatling/record.go:76` | `Event(99)` | switch + `default` |
| `run.FoundBy` | `gatling/run/find.go:109` | `FoundBy(99)` | table + bounds check |
| `model.Outcome` | `model/sample.go:24` | `unknown` | switch, falls through |
| `model.ItemKind` | `model/run.go:81` | `unknown` | switch, falls through |
| `model.PositionKind` | `model/position.go:23` | `unknown` | switch, falls through |
| `model.UserEventKind` | `model/sample.go:126` | `unknown` | switch, falls through |
| `model.Field` | `model/capability.go:109` | `unknown` | table + `>= fieldCount` |

Verified live: `model.Outcome(99)` → `"unknown"`, `model.Field(9999)` → `"unknown"`.

**Decision on the idiom**: the bounds-checked table lookup. It is already what four of the eleven do
(`Kind`, `Format`, `FoundBy`, `Field`), it is the only one of the three that does not grow a line per
value, and `model.Field`'s table already carries the comment explaining why a table is right — "a
field added without a name here is the empty string and fails `TestFieldStringNamesEveryField`
rather than printing as a number in a report". The seven switches become tables.

**What does not change**: `unknownName` stays, as the zero value's rendering, in all three packages
that declare it (`model/capability.go:8`, `gatling/record.go:25`, `gatling/run/doc.go:22`). Each is
unexported and package-local; three copies of an unexported constant is not the duplication #77
objects to.

**Scope note**: `gatling.Version`, `gatling.Warning`, `model.Warning` and `model.Position` also have
`String()` methods and are **not** enums. They are untouched.

## R8 — How a codec detects the other format, and what it returns

**Verified live**, against the corpus:

```
text.NewReader(3.14.9 binary log)  -> *gatling.SyntaxError
    gatling: line 1: expected ASSERTION or RUN before the run header, found "\x00…" (98 bytes)
binary.NewReader(3.11.5 text log)  -> *gatling.SyntaxError
    gatling: byte 0: expected the run record, found byte A
```

**Where the head bytes are.** The text codec has read a whole line — 98 bytes in the reproduction,
far past `gatling.DetectSize` (10). The binary codec fails at **byte 0**, having consumed exactly one
byte through `reader.u8`; but `reader.src` is a `*bufio.Reader` sized `readBufferSize`, so the bytes
are in hand. `src.Peek(gatling.DetectSize)` taken **before** the `u8` call costs no extra read and
consumes nothing. Peeking after the `u8` would return bytes 1..10 and misidentify the log, so the
order matters and is stated here so it is not rediscovered.

**The error type.** #107 decided: keep `UnsupportedFormatError` and give it producers. Its current
message is `"gatling: %s simulation.log: this module has no codec for it yet"`, which is false for
this case — the module reads it, just not in this package.

**Decision**: reword the type's own message to say what is true of both its uses — the reader in hand
does not decode this format — and have each codec wrap it with `%w`, naming `gatling/simlog` as the
entry point that does. The routing datum a consumer branches on is `Format`, which the type already
carries; the package name is a hint for a human, and a hint belongs in a message.

**Alternative rejected**: adding a `Reader string` field naming the package. It is one more
identifier on a surface this feature is shrinking, for information `Format` already carries in a form
a program can use.

**Not changed**: `simlog`'s own behaviour, and the genuinely-damaged path. `simlog.NewReader` and
`NewRunReader` already construct `UnsupportedFormatError` (`gatling/simlog/simlog.go:168`, `:184`)
for a known format with no reader — a branch nothing reaches today because both formats have codecs,
which is what "no input produces one" in the type's comment means. The reworded message must stay
true there.

## R9 — #79 is two defects, not three

**Defect 2 is already fixed.** `binary/reader.go`'s stray `//` is gone: `go doc ./gatling/binary
NewReader` renders two clean paragraphs, and `grep` finds no `instead.//` anywhere in the module. It
was rewritten while #76 moved the version gate ahead of the run record (`daed9e0`, in v0.0.10). The
issue is older than the fix.

**Still real**, both verified:

- `gatling/errors.go:45` — "Error names the line, what was expected there and what was found", above
  a method that branches on `Format` and renders `byte %d` for a binary log.
- `gatling/errors.go:69` and `:83` — the boundary-cut fact stated twice inside one doc comment.

**Decision**: fix the two, and add the guard the acceptance criterion implies rather than treating
the already-fixed one as done and unprotected. The guard reuses R1's parser: the same module-root
test walks every exported identifier's doc comment and fails on a line whose text, after the comment
marker is stripped, still contains `//`. That is what produced the published `instead.//`, and it is
cheaper to assert than to re-review.

## R10 — The concurrency sentence, and where it goes

**Finding**: `concurrent` appears once in non-test code, at `gatling/run/find.go:345`, about a
concurrent build appending to a file. Nothing says a reader is single-goroutine.

**Decision**: one sentence on each of the six surfaces that already carry the aliasing rule —
`text.Reader`, `text.RunReader`, `binary.Reader`, `binary.RunReader`, `simlog.RecordReader`,
`simlog.RunReader` — plus `gatling/simlog/doc.go`'s follower contract. It states the consequence, not
only the prohibition: the text codec's interner is a map (`gatling/text/intern.go`), so a concurrent
`Next` can end the process with `fatal error: concurrent map read and map write`, which `recover`
cannot catch.

**Not done**: making the readers safe. #85's non-goal, and a mutex on `Next` would cost every
consumer for a case none of them wants.

## R11 — The two `Warning`s

**Finding**: neither doc comment mentions the other. `grep` for `model.Warning` in `gatling/errors.go`
and for `gatling.Warning` in `model/run.go` finds nothing.

**Decision**: direction 1 of #86 — cross-link, do not rename. `gatling.Warning` names
`[github.com/galax-io/parsec/model.Warning]` as the canonical form whose `Reason` is this type's
`String()`; `model.Warning` names the Gatling side and explains that `Version` is text because the
model is tool-agnostic and other tools do not number like Gatling. A rename would cost a
`// Deprecated:` window from v0.1.0 for a problem two sentences solve.

## R12 — The four overview defects

All four verified on the tree:

1. `gatling/doc.go:10` — "the canonical model in model/ and the conversion into it arrive in a later
   milestone". They arrived in v0.0.3 and v0.0.5.
2. Root `doc.go` lists five packages: `gatling`, `gatling/text`, `gatling/binary`, `gatling/simlog`,
   `model`. `gatling/run` is absent.
3. `gatling/text/doc.go` mentions `RunReader` zero times, while `gatling/binary/doc.go` asserts the
   text counterpart exists.
4. No overview names the three-call path, and `simlog/doc.go` points at the codec packages, whose
   overviews do not point back.

**Decision**: the root `doc.go` and `model/doc.go` both name `run.Find` → `simlog.NewRunReader` → a
fold over `Next`. Every identifier named in package prose becomes a doc link, in all six overview
files, so the two codec overviews stop being near-parallel prose with only one of them linked.

## R13 — The six tests

Each verified at the site #95 names, with one line number moved:

| # | Site | What is wrong |
|---|---|---|
| 1 | `gatling/binary/reader_test.go:146`, skip at `:172` | skips when the documented `Groups` reuse breaks; never copies before comparing |
| 2 | `model/outcome_test.go:114` | asserts an identity of its own filter (`successes`, `:99`) |
| 3 | `gatling/text/model_selection_test.go:143` | the corpus-backed twin, same shape |
| 4 | `model/item_test.go:12` | asserts Go zeroes the fields a literal did not set |
| 5 | `gatling/text/model_review_test.go:65` | `d < 0` unreachable: outside `model`, an `Opt` is `Some(v)` or zero |
| 6 | `gatling/run/find_test.go:829` | `TestFindNeverOpensTheLog` does not call `requireUnixPermissions` (`:82`), which its siblings at `:760`, `:996` and `:1029` do |

**Decision**: each fix is demonstrated by breaking the production code and watching the test go red,
as the issue requires, and the break is recorded in the PR body. That is the only evidence that a
test which could not fail now can.

## R14 — The README, and what it can safely inline

**Verified**: `grep -c '```' README.md` → 0. No `go get`. The compatibility table at `:24` reads
`Gatling 3.13.0 … 3.15.x`; `:50` reads "over 3.13.1 through 3.15.1"; `binary.SupportedVersions`'s
doc says a 3.13.0 log is refused and why. `:53` names `gatling/text.NewRunReader`. Per-package
version stamps at `:23`–`:39`.

**Decision**: the first program is the body of `gatling/simlog/example_test.go` (82 lines, compiled
and output-checked by `go test`), inlined. A README program that `go test` does not compile is a
README program that rots; this one cannot.

The table reads 3.11.5–3.12.0 (text) and 3.13.1–3.15.1 (binary), carries the reason 3.13.0 is
refused — that version writes the format but cannot generate a report, so no run of it can carry the
second account of its own numbers a corpus entry needs — and points at `simlog.Supported()`.

**Repository metadata**: `gh api repos/galax-io/parsec` returns a description ending "with decoders
and statistics" and `homepage: null`. Both are changed through GitHub settings, not a file, so they
are their own task with no diff to review.

## R15 — Where the changelog contradiction now lives

The 0.0.9 entry moved down when 0.0.10 was stamped. The wrong statement is at `CHANGELOG.md:145-149`
("modification time first, then the directory name … descending name is descending run start"); the
correct one is at `:157-161` ("Ties break on the run id's own UTC stamp before the directory name").
`gatling/run/find.go:438-450` compares modification time, then `runStart(name)`, then the whole name —
three levels.

**Decision**: correct the first statement to the three-level rule and leave the second, rather than
deleting the first. The paragraph it sits in is about why the tie-break is not a formality, and that
argument still needs the rule stated.

## R16 — `SECURITY.md`

**Verified**: `gh api repos/galax-io/parsec/community/profile` lists `code_of_conduct`,
`code_of_conduct_file`, `contributing`, `issue_template`, `license`, `pull_request_template`,
`readme` — no `security`. The file does not exist.

**Decision**: report privately through GitHub's private vulnerability reporting, enabled in
repository settings. Supported versions are expressible from the release policy in `AGENTS.md`: fixes
land on `main` and are cherry-picked onto the current `release/X.Y.0` branch, so the supported set is
"the latest `X.Y` line", which is `0.1` at this tag. The trust boundary is stated plainly: this
module decodes files it does not trust, in a process it does not own.

## R17 — The workflow pins and the two unguarded inputs

**Every `uses:` resolved to a commit SHA** on 2026-09-12:

| Action | SHA | Latest release |
|---|---|---|
| `orhun/git-cliff-action@v4` † | `3d96a18cc4ec17e9dc69ddcc424ccafaf1f78ce2` | v4.9.0 |
| `golangci/golangci-lint-action@v9` † | `ba0d7d2ec06a0ea1cb5fa41b2e4a3ab91d21278a` | v9.3.0 |
| `sbt/setup-sbt@v1` † | `82da71df4e122282484a99a8d70096bc2369dbd8` | v1.5.9 |
| `actions/checkout@v7` | `3d3c42e5aac5ba805825da76410c181273ba90b1` | v7.0.1 |
| `actions/setup-go@v7` | `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` | v7.0.0 |
| `actions/setup-java@v6` | `de7274f081f381c8f8158605e0321c36c376e2e6` | v6.0.1 |
| `actions/cache@v6` | `55cc8345863c7cc4c66a329aec7e433d2d1c52a9` | v6.1.0 |
| `actions/upload-artifact@v7` | `043fb46d1a93c77aae656e7c1c64a875d1fc6a0a` | v7.0.1 |
| `actions/download-artifact@v8` | `3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c` | v8.0.1 |
| `actions/setup-python@v7` | `5fda3b95a4ea91299a34e894583c3862153e4b97` | v7.0.0 |

† third-party, where #93 says MUST. The `actions/*` rows are the SHOULD, and are pinned in the same
pass because a half-pinned workflow file invites the next contributor to copy the unpinned line.

**These SHAs are resolved at implementation time, not taken from here.** They are recorded to prove
each tag resolves and to give the task a starting point; a month-old SHA in a plan is a stale pin.

**Dependabot** needs no configuration change. Its `github-actions` ecosystem updates a SHA pin and
keeps the trailing version comment in step, which is what #93 asks for; the existing
`.github/dependabot.yml` already groups actions weekly.

**The two unguarded interpolations**:

- `.github/workflows/gatling-canary.yml:85` — `-Dgatling.version="${{ matrix.gatling }}"`.
- `.github/workflows/fuzz-nightly.yml:70-71` — `-fuzz '^${{ matrix.case.target }}$'`,
  `-fuzztime '${{ inputs.fuzztime || '10m' }}'`, and `'${{ matrix.case.package }}'`.

**Decision**: the guard `record-corpus.yml:74-87` already uses — route the value through `env:`, then
validate with a regex in a `set -euo pipefail` step that exits 1 with a message naming the value.
Three shapes are needed: a Gatling version (`^[0-9]+\.[0-9]+\.[0-9]+$`, the existing one), a fuzz
target and package (Go identifier and import path), and a duration (`^[0-9]+[smh]$`). No second
pattern is introduced.

## R18 — Skills read, and the testify question

Constitution v2.4.0, *Engineering Guidance (Skills)*. Every required-reading row is triggered by this
feature: exported identifiers change (`golang-naming`), an error type and its producers change
(`golang-error-handling`), tests change — six of them structurally (`golang-testing`), doc comments
change across six packages (`golang-documentation`), and interface method sets are frozen
(`golang-structs-interfaces`). Of the consult rows, `golang-safety` / `golang-security` applies twice
over: the module decodes untrusted input, and #93 is a supply-chain change to the job that publishes.
`golang-benchmark` is not triggered — this feature states no new throughput or memory figure.

**No disagreement to record.** `golang-testing` and `golang-stretchr-testify` recommend `testify`
for assertions and suites. Until 2026-09-12 that was a standing prohibition — a *Must not be
followed* row in the constitution and a *Never* in `AGENTS.md` — and this section recorded it as a
conflict. The maintainer has lifted it (constitution **v2.4.0**): the ban was never their position,
and the standing instruction is to use what is warranted, judged case by case, which is what
Principle IV's ask-first already is. The whole *Must not be followed* table went with it — the
`samber/*` family and the dependency-injection skills too — and each is now a *consult* entry with a
stated occasion.

**The judgement for this feature: no.** Not because a rule forbids it, but because nothing here asks
for it. The six repairs in #95 are structural — a test that skips out of its assertion, a filter
asserting its own identity, a fixture that does not deny as root — and none of them is an assertion
that reads badly in stdlib. Against that, `parsec` is pinned by three repositories and this is the
feature that freezes its surface; adding a module inside it would be exactly the opportunistic
change `AGENTS.md` sends to its own PR. Principle IV's ask-first stands, `model/` and `gatling/`
still ship stdlib-only, and the `deps` job still proves it.

The question is now open rather than closed, which is the point of the amendment. A later feature
whose tests genuinely read better with `testify` makes the case in its own `research.md` and asks —
and `.golangci.yml` already enables `testifylint`, so the lint is waiting for it.

**Read on 2026-09-12, before the first line of code.** All five required-reading skills, against
constitution v2.4.0. Four things came out of it.

**One disagreement, and the constitution wins.** `golang-testing` requires a test file to be named
after the source file it tests (`foo.go` → `foo_test.go`). This repository names test files by
concern: `model/outcome_test.go`, `model/item_test.go`, `model/unknown_test.go` and
`model/exports_test.go` map to no source file of that name, and neither do
`gatling/binary/agreement_test.go` or `gatling/text/model_selection_test.go`. Principle VI says to
follow the convention already in the codebase before adding a new one, and adopting the skill's rule
would rename five existing files as an opportunistic refactor `AGENTS.md` forbids. The concern-named
convention stands: `api_test.go` at the module root (which tests six packages and belongs to none),
`wrongformat_test.go` in each codec.

**One improvement adopted.** `golang-naming` on constructors and stuttering: `wire.Run(...)` reads
as "execute" for a function that builds a `model.Run`. It is named **`wire.NewRun`**, which is the
constructor convention for a package with several constructible things. The plan's `wire.Warnings`
stands — it returns a slice, not a constructed value.

**Two recommendations not adopted, deliberately.** `golang-structs-interfaces` recommends
compile-time interface checks (`var _ I = (*T)(nil)`); the module has none anywhere, so adding them
is a new convention, which Principle VI says is named in the plan and applied consistently — this
plan does not name it, and the golden inventory already fails when an interface method set moves.
`golang-documentation` says a README SHOULD carry badges; #99 asks for install, a program and a
truthful table, and CI or coverage badges would need wiring this feature does not touch.

**What reading the code changed about #78.** The six `gatling`-side enums already render `Type(N)`
*and already have out-of-range assertions*: `gatling/format_test.go:153`, `gatling/record_test.go:24`
(`Kind(42)`), `:48`, `:72`, `:97`, and `gatling/run/find_test.go:124`. So #78 is a `model`-only
change. And the test to rewrite is not a new `enum_test.go` but the existing
`model/unknown_test.go`, whose `TestOutOfRangeValuesReadAsUnknown` asserts the behaviour being
replaced — for four of the five `model` enums, `PositionKind` missing. It gains that fifth case.

## R19 — Ordering, and what must not be in flight together

- **#58 → #77.** Both rewrite both `NewRunReader` bodies; #58 first makes #77 a two-line removal.
- **#78 is free of the golden file.** The inventory lists identifiers, and the enum change adds,
  removes and renames none — it rewrites eleven `String()` bodies and their doc comments. It does
  share `gatling/record.go`, `gatling/format.go`, `gatling/version.go` and `model/run.go` with #107
  and #86, so it is sequenced against those rather than against R1.
- **#107 and #13 land together.** #107 is the scope, #13 is the release note it produces; #13 has
  nothing to list without it.
- **#84 after #107's `UnsupportedFormatError` decision is recorded in the type's doc comment**, since
  the reword is what makes the type honest for both uses.
- **#95 anywhere**, but before the tag: two of the six pin guarantees the tag freezes.
- **#93, #99, #97, #101 are independent of every Go change** and of each other, except that #99's
  compatibility section is #13's consumer-facing half and follows it.

## R20 — What this feature does not need to research

The supported Gatling ranges, the version gate, `Capabilities`, format detection, the memory budget
and the endings contract are all v0.0.10 behaviour and are untouched. No corpus entry is recorded:
this feature uses `testdata/corpus/gatling/3.14.9` and `3.11.5` as the wrong-codec fixtures for #84
and repairs the corpus-backed selection test for #95, and records nothing new.
