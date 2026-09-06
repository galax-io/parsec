# Quickstart: The corpus and the canary

**Feature**: 007-corpus-and-canary | **Date**: 2026-09-06

How to run every check this feature adds, and what each one proves. Run from the repository root.

---

## Prerequisites

| For | Needs |
|---|---|
| everything except the canary and recording | Go 1.25 — `go.mod` is authoritative |
| the canary, recording a version locally | JDK 17, sbt, and a free port 8089 for the stub |

The canary and the recording workflow both run in CI, where nothing is installed by hand. The local
route below exists for iterating on a failure.

---

## 1. Per-request figures against each run's own report

The core of User Story 2 — no Gatling, no network, just the five committed recordings.

```bash
go test -tags=integration -race -count=1 -run 'Report|Tolerance' ./gatling/...
```

**Proves**: every per-request and per-group row the recorded reports state is compared against the
same figure folded from the decoded records — for all five versions, including 3.14.9 and 3.15.1
whose figures live only in `index.html` (spec FR-006, FR-007).

**Should fail when you alter one expected value.** There are **two** expected files per recording and
they are read by **different** checks — altering one does not exercise the other:

| Altered file | Read by | Run |
|---|---|---|
| `records.golden` | `TestGolden` (`golden_test.go`), `gate_test.go` | the golden comparison |
| `stats.json` / `index.html` / `console.txt` | the report/tolerance tests | the report comparison |

The report and tolerance tests decode `simulation.log` and compare against the run's own report —
they never open `records.golden`. Both halves of FR-012 need proving, so run both:

```bash
# (a) the golden comparison — alter the recorded record stream
sed -i.bak 's|GET /slow|GET /sloq|' testdata/corpus/gatling/3.15.1/records.golden
go test -tags=integration -count=1 -run '^TestGolden$' ./gatling/binary/   # expect FAIL
mv testdata/corpus/gatling/3.15.1/records.golden.bak testdata/corpus/gatling/3.15.1/records.golden

# (b) the report comparison — alter a figure in the run's own report
sed -i.bak 's|<td class="value total col-2">66</td>|<td class="value total col-2">65</td>|' \
  testdata/corpus/gatling/3.15.1/index.html
go test -tags=integration -count=1 -run 'Report' ./gatling/binary/   # expect FAIL naming the row
mv testdata/corpus/gatling/3.15.1/index.html.bak testdata/corpus/gatling/3.15.1/index.html
```

**`-tags=integration` is required in both, and its absence is silent.** Without it the corpus-wide
tests are not compiled into the binary at all, so `go test -run 'Report' ./gatling/binary/` reports
`ok` against a corpus you have just deliberately corrupted. Verified by doing exactly that. A
green result from a check that did not run is the failure mode this whole feature exists to remove,
so it is worth seeing once: drop the tag on purpose and watch it pass.

With the tag, the failure names the version, the row and both figures:

```
--- FAIL: TestDecodedPerRequestFiguresMatchTheRunReport/3.15.1
    index.html: request "GET /ok" under "outer" decoded 66 (66 ok, 0 ko); the report says 65 (66 ok, 0 ko)
```

That is spec FR-012 and SC-003.

---

## 2. The peak-memory bound at the string ceiling

```bash
go test -tags=integration -count=1 -run 'PeakMemory$' ./gatling/binary/
```

**No `-race`.** The detector moves the very `HeapAlloc` figure the test asserts on, which is why CI
already runs this step on its own.

**Proves**: the budget `Reader`'s documentation states holds for a field at `MaxStringLen` in all
three encodings — Latin-1 ASCII, Latin-1 above ASCII, UTF-16 (spec FR-025, FR-026).

To see the measurement behind the ceiling change, see [research R8](research.md); the figures there
are reproducible with a streamed synthetic log — a materialised one sits on the heap and reports its
own size back to you.

---

## 3. The fuzzers

One target, the way CI runs it:

```bash
go test -run '^$' -fuzz '^FuzzDecode$' -fuzztime 90s ./gatling/binary/
```

Every target, the way the matrix is built:

```bash
go test -list '^Fuzz' ./...
```

**Proves**: no input reaches a panic (Principle II). A finding lands in
`gatling/binary/testdata/fuzz/FuzzDecode/` and fails the run — **do not commit it**; CI uploads it as
an artefact instead (spec FR-015, FR-016).

**The regression this exists for**: revert the v0.0.5 `math.MinInt32` fix and the leg must fail
within its budget (spec FR-014, SC-006).

---

## 4. The canary, against a Gatling that ran a minute ago

### The short way — let CI do it

```bash
gh workflow run gatling-canary.yml -f versions='["3.13.1","3.15.1"]'
```

### Locally

Terminal 1 — the stub the probe talks to:

```bash
go run ./testdata/corpus/gatling/simulation/stub
```

Terminal 2 — run the probe under the version to try:

```bash
cd testdata/corpus/gatling/simulation
sbt -Dgatling.version=3.15.1 "Gatling/testOnly io.galaxio.parsec.corpus.CorpusSimulation" 2>&1 | tee /tmp/console.txt
```

Terminal 2 — hold the fresh run to its own report:

```bash
cd -
PARSEC_CANARY_RUNS="3.15.1=testdata/corpus/gatling/simulation/target/gatling/corpussimulation-<timestamp>" \
  go test -tags=canary -race -count=1 ./gatling/binary/
```

**Proves**: the binary codec matches a Gatling that exists now, not only the recordings (User Story
1, SC-001). Two or more versions in one value additionally compare the runs against each other,
across the text/binary boundary (spec FR-003).

**Without `PARSEC_CANARY_RUNS` the canary skips with a reason** — it never fakes a run. In CI the job
fails when no canary test passed, so a skip cannot pass for a run.

---

## 5. Recording a version

```bash
gh workflow run record-corpus.yml -f version=3.16.0
gh run download <run-id> -n corpus-entry-3.16.0 -D testdata/corpus/gatling/3.16.0/
```

Then **write `RECORDING.md`** — the scaffold in the download carries the mechanical facts and the
headings; what was checked at capture time, and which absences are Gatling's own, is yours to state.
Then commit.

**Proves**: recording is one dispatch plus the note, with no manual file selection and nothing
installed locally (spec FR-018, SC-004).

**Widening the gate is a separate decision.** A new version decodes with a warning until its entry is
committed *and* `SupportedVersions()` is widened in the same change (Principle II). The canary's
`TestCanaryCoversSupportedRange` fails if the range moves without the canary running the new bound.

---

## Everything, the way CI sees it

```bash
gofmt -l . && go vet ./... && go test -race -shuffle=on ./...
go test -tags=integration -race -shuffle=on -count=1 -skip 'PeakMemory$' ./...
go test -tags=integration -count=1 -run 'PeakMemory$' ./...
bash scripts/check-coverage.sh --enforce
for t in scripts/*_test.sh .claude/hooks/*_test.sh .githooks/*_test.sh; do bash "$t"; done
```
