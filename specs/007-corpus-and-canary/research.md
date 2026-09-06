# Research: The corpus and the canary

**Feature**: 007-corpus-and-canary | **Date**: 2026-09-06 | **Spec**: [spec.md](spec.md)

Every finding below was checked against this repository or measured, not recalled. Where a figure
appears, the command that produced it is named.

---

## R1 — Where each version's account of itself lives, and the one shape they all reduce to

**Decision**: Model every run's own report as **one tree** — a node carrying a name, a parent, and a
`total/ok/ko` triple plus rate — and give it three readers, one per artefact shape. The verification
walks the tree, never the artefact.

**Evidence** (read from the committed corpus):

| Version | Format | Machine-readable per-request tree | Read from |
|---|---|---|---|
| 3.11.5 | text | `stats.json` | JSON, already walked by `gatling/text/helpers_test.go:walk` |
| 3.12.0 | text | `stats.json` | same |
| 3.13.1 | binary | `js/stats.json` | same JSON shape — verified identical node structure |
| 3.14.9 | binary | **none** | `index.html`, tables baked into the markup |
| 3.15.1 | binary | **none** | `index.html`, same markup |

The 3.13.1 `js/stats.json` walked to exactly the tree the report shows:

```
All Requests -> 102/84/18
  outer -> 6/0/6
    inner, with comma -> 6/0/6
      GET /slow -> 6/6/0
      GET /fail -> 6/0/6
    GET /ok -> 66/66/0
  GET /ok -> 6/6/0
  Проверка /ok -> 6/6/0
  connect refused -> 6/0/6
  unknown host -> 6/0/6
```

Extracting the same tree from 3.14.9's and 3.15.1's `index.html` yields ten rows with identical
names and identical figures. So the tree is the real contract; the artefact is an encoding of it.

**Rationale**: `gatling/text` already compares per-request figures this way and finds real defects.
The binary side compares three numbers (`gatling/binary/tolerance_test.go`) because nothing could
read the rest. One tree with three readers closes that gap without inventing a second comparison.

**Alternatives rejected**:

- *Compare only the versions that ship JSON, document 3.14.9 and 3.15.1 as excluded.* Rejected on
  the same ground issue #61 rejects an unstated exclusion: two of five versions, and the two newest,
  would be verified less than the rest, while the spec claims all five.
- *Extract the figures at recording time into a normalised file.* Rejected by Principle III and by
  the recording notes, which keep artefacts as the tool wrote them precisely so a later reader can
  check what the run said rather than what was recorded about it.

---

## R2 — Reading the per-request tree out of the HTML report with the standard library

**Decision**: Extract with `regexp` plus `html.UnescapeString`, both standard library. Identity comes
from the row's `id` and `data-parent`, never from the display name.

**Evidence** — the markup is machine-shaped, not prose. One row of 3.14.9's
`container_statistics_body`:

```html
<tr id="req_outer---get--ok-2130245131" data-parent="group_outer-106111099">
  <td class="total col-1"> … <span … class="ellipsed-name">GET /ok</span> … </td>
  <td class="value total col-2">66</td>
  <td class="value ok col-3">66</td>
  <td class="value ko col-4">0</td>
  <td class="value ko col-5">0.0</td>
  <td class="value total col-6">16.5</td>
  …  <!-- col-7 … col-14: min, 50th, 75th, 95th, 99th, max, mean, std dev -->
</tr>
```

Every figure has its own class (`col-N`), the header row names each column, and the parent link is an
attribute. Names repeat — `GET /ok` appears twice, once under `outer` and once at the root — so the
tree, not the name, is what identifies a row.

**Why not an HTML parser**: `golang.org/x/net/html` is a third-party module. Principle IV admits none
without approval, `go.mod` is dependency truth, and the module is imported by three downstream
builds. The `deps` job happens not to inspect test-only imports, which makes this a rule to keep
deliberately rather than one the machine would catch.

**Why this is safe**: FR-011 requires a failed extraction to fail loudly. The extractor asserts it
found a `ROOT` row and at least one child, and that every row it found carries all of `col-2`,
`col-3`, `col-4`. A future report shape that breaks the pattern yields zero rows and fails, rather
than an empty comparison that passes.

**Alternatives rejected**: parsing `js/stats.js` (a JavaScript assignment wrapping the same JSON) —
present on 3.13.x, absent from 3.14.0 on, so it solves nothing the JSON does not already solve.

---

## R3 — Where the shared extractor lives

**Decision**: a new package `internal/corpus`, holding only what this feature adds — the report tree
and its three readers. Both `gatling/text` and `gatling/binary` external test packages import it.

**Rationale**: the extractor is needed by two test packages, and a `_test.go` file cannot be imported
across packages. `internal/` is already established (`internal/wire`), is invisible to consumers, and
is exactly where Principle I sends shared helpers.

**Why that name**: [parsec#59](https://github.com/galax-io/parsec/issues/59) — *The corpus test
helpers are copied into every package that reads the corpus* — already proposes `internal/corpus`
with `Format`, `Logs`, `Dirs`, `WriteRecord`, `Canonical` and the memory sampler. That issue is **not
in this milestone**. This feature therefore:

- **adds** `internal/corpus` containing only the report tree, which is new code with no duplicate;
- **does not move** any existing helper into it. Moving `canonical`, `writeRecord`, `sampler` and
  friends is #59's change, and AGENTS.md sends an out-of-scope refactor to its own PR.

The result is that #59 later grows the package rather than creating a second one.

**Alternative rejected**: duplicating the extractor into both test packages. It would be the sixth
instance of exactly the duplication #59 was filed about, and Principle VI forbids duplicated code.

---

## R4 — The binary canary

**Decision**: widen the existing `gatling-canary.yml` matrix from two versions to five, and add a
canary test file to `gatling/binary` mirroring `gatling/text/canary_test.go`, reading its runs
through the R1 tree.

**Evidence that the probe already runs under every version** — `project/Dependencies.scala` selects
the assertion flavour from the version itself:

```scala
val picatinnySupported: Boolean = … major == 3 && minor <= 12
```

and `build.sbt` adds `scala-opennfr` or `scala-plain` to the source directories accordingly. The
recordings for 3.13.1, 3.14.9 and 3.15.1 were produced this way. So running a binary version in the
canary is a matrix entry, not a build change.

**What changes in the workflow**:

- default version list `["3.11.5","3.12.0"]` → all five;
- the decode step runs `./gatling/text/` or `./gatling/binary/` according to the version's format;
- the `compare` job holds every fresh run to every other, across the format boundary (R5).

**`TestCanaryCoversSupportedRange` already exists** in `gatling/text` and fails when a supported
bound was not exercised. The binary codec needs its own, reading `binary.SupportedVersions()`
(3.13.1 – 3.15.1); `gatling/text` covers 3.11.5 – 3.12.0. Together they satisfy FR-002.

**Cost**: the matrix goes from two jobs to five, running in parallel. Each job is one sbt resolve
(cached, keyed on the Gatling version) plus a three-second simulation.

---

## R5 — Cross-format comparison, and the one difference that is not a defect

**Decision**: compare fresh runs across the format boundary, with the group-name spelling normalised
and the reason stated at the assertion.

**The difference**: the probe declares a group `inner, with comma`. A text `simulation.log` separates
a group path with commas, so Gatling replaces the comma with a space before writing —
`inner  with comma`, two spaces. The binary format length-prefixes each name, so the comma survives.
`Dependencies.scala` documents this as the reason the two assertion flavours exist at all. **Both
spellings are correct for their format**; a comparison that did not normalise would fail on a
correctness the decoder is responsible for preserving.

Already set aside by the existing comparisons, and set aside here too: timing, run identity
(id and version), record order, the version-gate warning, and — from
`gatling/binary/crossversion_test.go` — the check-failure message text, which Gatling reworded at
3.14.0 from `status.find.is(200), but actually found 500` to `status.find.is(200), found 500`.

**Where it lives**: `gatling/binary`'s external test package already imports `gatling/text`
(`gatling/binary/agreement_test.go` does exactly this, to hold both codecs to the same canonical
results). Principle I's "tool packages MUST NOT import each other" governs the packages themselves,
not an external `_test` package written to compare them; the precedent is in the tree and is the
right home.

---

## R6 — Fuzzing in CI

**Decision**: one CI job per fuzz target, discovered rather than listed, on every pull request; a
longer scheduled run beside it.

**The three targets** (`go test -list '^Fuzz' ./...`):

| Package | Target |
|---|---|
| `github.com/galax-io/parsec/gatling` | `FuzzDetect` |
| `github.com/galax-io/parsec/gatling/binary` | `FuzzDecode` |
| `github.com/galax-io/parsec/gatling/text` | `FuzzReader` |

**The invocation**: `go test -run '^$' -fuzz '^FuzzDecode$' -fuzztime 90s ./gatling/binary/`. The
`-run '^$'` matters: without it the ordinary tests run first and eat the budget. `-fuzz` accepts one
target in one package per invocation, which is what makes this a matrix rather than a loop.

**Discovery, not a list**: FR-013 says *every* target. A hard-coded list silently stops covering a
target added later. The matrix is generated by parsing `go test -list '^Fuzz' ./...`, whose output
interleaves package lines with target names — so a target added tomorrow is fuzzed tomorrow.

**Measured here** (`go test -run '^$' -fuzz '^FuzzDecode$' -fuzztime 30s ./gatling/binary/`):
30 s produced 4,972,101 executions at 116k–256k exec/s, using 862% CPU — the fuzzer saturates every
core it is given. A GitHub `ubuntu-latest` runner has four cores against this machine's ten, so a
runner will do meaningfully fewer executions per second. **Two consequences**: the budget must be
measured on the runner, not inferred from here (FR-014), and the three targets must be three jobs —
three fuzzers sharing four cores would each get a third of the budget they appear to be given.

**Starting budget**: 90 s per target, from #60's observation that `FuzzDecode` finds the
`math.MinInt32` crasher in about ninety seconds. Confirmed against the runner by the acceptance test
in FR-014: revert the fix, the leg must fail.

**Crashers**: `go test` writes a failing input to `testdata/fuzz/<Target>/<name>` in the package
directory and fails. That path is uploaded as an artefact and the job fails (FR-015). Nothing is
committed (FR-016) — the job has no write permission and the workflow adds none. The *generated*
corpus lives in `$GOCACHE/fuzz` and never touches the tree.

**Alternative rejected**: nightly only. #60 rejects it explicitly — the defect it exists to catch
would have merged and been found the next morning.

---

## R7 — One dispatch records a version

**Decision**: a `workflow_dispatch` job that reuses the canary's run steps, then assembles the entry
and uploads it. The maintainer downloads, writes `RECORDING.md`, and commits.

**Why the run steps are already written**: `gatling-canary.yml` starts the stub, waits for its port,
runs `sbt -Dgatling.version=… "Gatling/testOnly …"`, and uploads the run directory as an artefact —
every mechanical step of the documented procedure except selecting the files and generating the
golden stream.

**Selecting the files from the run, not from a declared shape** (FR-021): keep, if present,
`simulation.log`, `index.html`, `js/global_stats.json`, `js/stats.json`, `js/stats.js`,
`js/all_sessions.js`, `js/assertions.xml`, plus the redirected `console.txt`. Drop the render-only
assets — `js/highstock.js`, `js/jquery-*.js`, `js/bootstrap.min.js`, `js/highcharts-more.js`, the
rest of `js/`, and all of `style/`. This is presence-driven, so 3.12.0 keeps its JSON, 3.13.1 keeps
both JSON and HTML, and 3.15.1 keeps HTML and console, with no version table in the script.

**The golden stream**: both codecs already carry `var update = flag.Bool("update", …)` in
`golden_test.go` and generate `records.golden` from the decoder's own output. The job calls that.

**Failing rather than publishing a half-entry** (FR-022): the sbt run fails when the probe misses its
declared expectations, which is what already makes a merely self-consistent log impossible to record.
Gatling 3.13.0 additionally fails to read back its own assertion records and produces no report — the
job's "a run with no report is not an entry" check catches that class in general.

**Size**: the corpus is 440 KB today across five entries, against the spec's 5 MB ceiling. Dropping
render-only assets is what keeps it there — they are about a megabyte per entry.

**Platform**: entries produced this way are recorded on `ubuntu-latest`, where the five existing
entries record macOS/arm64. The scaffolded note states the platform (FR-023) so provenance is never
inferred.

---

## R8 — Peak memory at the string ceiling, measured

**Decision**: **reduce `MaxStringLen` from 8 MiB to 1 MiB.** The documented 32 MiB budget does not
hold at 8 MiB, and 1 MiB restores it with a 4.7× margin. This changes an exported identifier's
observable behaviour, so FR-028 and AGENTS.md (*Ask first*) required approval before it is made —
**given 2026-09-06**, on the measurements below and with the alternative stated alongside them.

**How it was measured**: a probe outside the repository (scratchpad, not committed) built binary logs
carrying one field of exactly the ceiling size in each encoding, **streamed** — a materialised log
would sit on the heap and be counted in every sample, which inflates the figure by the log's own
size — and sampled `runtime.MemStats.HeapAlloc` the way `gatling/binary/memory_test.go` does.

**At today's ceiling of 8 MiB**:

| Field at the ceiling | Peak heap | × ceiling |
|---|---:|---:|
| Latin-1, ASCII | 16.3 MiB | 2.0× |
| **Latin-1, above ASCII (0xE9)** | **24.3 MiB** | **3.0×** |
| UTF-16 → 2-byte UTF-8 (U+0444) | 16.3 MiB | 2.0× |
| UTF-16 → 3-byte UTF-8 (U+4E2D) | 20.3 MiB | 2.5× |
| **one field of each, one log** | **52.3 MiB** | **6.5×** |

The multipliers are exactly what the code predicts: an 8 MiB read buffer, plus a result that is 1×
the ceiling for ASCII, 2× for Latin-1 above ASCII (`latin1` grows a builder to `len(b)+high`), and
1×–1.5× for UTF-16 depending on the UTF-8 width of the code points.

**A single ceiling-sized field is therefore under 32 MiB; a log carrying one of each is not.** The
6.5× figure is garbage that has not been collected between records, which is a real cost the
assertion measures and the existing test already absorbs with slack.

**At candidate ceilings**, same probe:

| Ceiling | Worst single field | One of each, one log | 32 MiB budget |
|---:|---:|---:|---|
| 8 MiB (today) | 24.3 MiB | 52.3 MiB | **fails** |
| 4 MiB | 12.3 MiB | 26.3 MiB | holds, 18% margin |
| 2 MiB | 6.3 MiB | 13.3 MiB | holds, 58% margin |
| **1 MiB** | **3.3 MiB** | **6.8 MiB** | **holds, 79% margin** |

**Why 1 MiB and not 4**: 4 MiB passes on this machine with 18% to spare, which is not enough for a
figure that moves with GC scheduling across runners and Go releases. 1 MiB is also what #61 argued
for on its own grounds: the constant exists to stop a corrupt length prefix asking the allocator for
gigabytes, and 1 MiB does that as well as 8 MiB does.

**Does 1 MiB fit real logs?** The longest field the format carries is an assertion payload; the
3.14.9 recording's ten payloads run 15–51 bytes. The longest string is a failure message, which
Gatling truncates. 1 MiB is roughly twenty thousand times the largest field in the corpus.

### The other ceiling, checked before assuming it is safe

`MaxStringLen` is not the only cap. `maxAssertionBytes` (8 MiB, `record.go:43`) bounds what a run
record's assertion payloads come to **in total**, and unlike a string field those payloads are
*retained for the whole read* and copied again by `Assertions()`. Each payload also passes through
`sized()`, so it is gated by `MaxStringLen` first and by the running total second.

Measured with a second streamed probe, filling `maxAssertionBytes` with payloads at the string
ceiling:

| String ceiling | Payloads × size | Peak heap | 32 MiB budget |
|---:|---|---:|---|
| 8 MiB | 1 × 8 MiB | 16.3 MiB | holds |
| 1 MiB | 8 × 1 MiB | **9.3 MiB** | holds |

Lowering the ceiling **improves** this path rather than threatening it: the retained total is
unchanged at 8 MiB, while the transient scratch beside it drops from 8 MiB to 1 MiB. No second
change is needed here.

### Consequences to carry

- a `CHANGELOG.md` *Changed* entry (Principle V: pre-v0.1.0 exported identifiers may change, but
  every such change is recorded);
- the doc comment on `MaxStringLen` and the memory paragraph on `Reader` restated to the same
  number, so FR-026's "documented budget = asserted budget" is visibly true;
- **`TestAssertionPayloadsPastTheByteCeilingAreRefused` must be resized with the constant.** It
  builds two payloads of 4608 KiB each — deliberately *inside* the old 8 MiB string ceiling so that
  what refuses them is `maxAssertionBytes`. At a 1 MiB ceiling each payload is past `MaxStringLen`
  on its own, so `sized()` refuses the first one with `"a length past the maximum this codec will
  allocate"`; the test's `strings.Contains(se.Found, "past the ceiling")` then fails. It fails
  loudly rather than passing wrongly, which is the right failure — but it stops exercising
  `maxAssertionBytes` at all. Reaching that ceiling at a 1 MiB string cap needs **nine payloads of
  1 MiB**, not two of 4.5 MiB.

**Where the new assertion runs**: with `TestPeakMemory`, in the e2e job's separate
`-run 'PeakMemory$'` step **without `-race`** — the detector moves the very `HeapAlloc` figure being
asserted, which is why that step is already split out.

---

## R9 — Engineering guidance: skills triggered

Per the constitution's *Engineering Guidance (Skills)*, the rows this change triggers:

**Required reading, before the code is written** (`/speckit-tasks` sequences these with their tasks):

| Trigger in this feature | Skill |
|---|---|
| every task is a test change | `golang-testing` — testify sections excluded (Principle IV) |
| `MaxStringLen`'s value and doc comment change | `golang-documentation`, `golang-naming` |
| the extractor returns errors on a report it cannot read | `golang-error-handling` |
| `internal/corpus` introduces types | `golang-structs-interfaces` |

**Consulted, occasion arisen**:

| Occasion | Skill |
|---|---|
| the plan states a peak-memory figure | `golang-benchmark` |
| untrusted input — the fuzz leg is exactly this | `golang-safety`, `golang-security` |
| corpus recording and Gatling version questions | `galaxio-gatling-pro`, `gatling-versions`, `gatling-build` |

**Disagreements with the constitution**: none found. The one place a skill could have led elsewhere
is R2 — a general-purpose Go answer to "parse HTML" is `golang.org/x/net/html`, which Principle IV
forbids here. The constitution wins, and the extractor is written against the standard library.

**Forbidden skills not followed**: `golang-stretchr-testify` and the testify sections of
`golang-testing`; `golang-samber-*`; the dependency-injection family. Nothing in this feature reaches
for them.

---

## Open items carried into implementation

1. ~~`MaxStringLen` 8 MiB → 1 MiB needs sign-off before the constant moves (FR-028).~~
   **Resolved 2026-09-06: approved.** Everything else in R8 follows from it — see R8's
   *Consequences to carry*, including the one test that must be resized with the constant.
2. **The pull-request fuzz budget is confirmed on the runner**, not assumed from the 90 s starting
   point (FR-014); the acceptance test is the reverted `MinInt32` fix.
