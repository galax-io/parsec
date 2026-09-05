# Quickstart — validating the canonical model and the probe's document

**Feature**: `003-canonical-model` · **Plan**: [plan.md](plan.md)

Every scenario below is runnable and states what proves it. Types and fields are in
[data-model.md](data-model.md) and [contracts/](contracts/); nothing is repeated here.

## Prerequisites

- Go 1.25 (`go.mod` is authoritative)
- For the probe scenarios only: a JDK 17+, `sbt`, and the corpus stub

---

## 1. The gates the whole module is held to

```bash
gofmt -l . && go vet ./... && go build ./... && go test -race -shuffle=on ./...
```

Green is the definition of a green commit. Add the boundary check, which is what keeps `model/` and
`gatling/` importable by three downstream builds without dragging anything in:

```bash
go list -deps ./model/... ./gatling/... | grep -v '^github.com/galax-io/parsec' | grep '\.' || echo "stdlib only"
```

Nothing printed but the last line means no third-party package is reachable from either.

---

## 2. A run read without naming the tool (User Story 1)

The proof that the milestone happened: counts taken through `model` alone, equal to what that run's
own Gatling report states.

```bash
go test -tags=integration -run TestModelAgainstReport ./gatling/text/ -v
```

**What it does**: for each corpus entry under `testdata/corpus/gatling/<version>/`, opens
`simulation.log` through `text.NewRunReader`, folds the item stream into totals, and compares them
against `global_stats.json` and `stats.json` — the run total with its OK/KO split, and the same three
numbers per request name and per group.

**Expected**: exact equality. A difference of one fails (SC-002). The existing wire-record checks
(`TestReport`) keep running beside it, unchanged, so a failure in one and not the other says
immediately whether the decoder or the conversion is wrong.

---

## 3. Absence is declared, not invented (User Story 2)

```bash
go test -run TestCapabilities ./gatling/text/ ./model/ -v
go test -tags=integration -run TestAbsentFieldsAreNeverFilledIn ./gatling/text/ -v
```

**Expected**: `text.Capabilities()` reports every field of research R7 as absent by name, and no
sample of any corpus run carries a set value for one of them (SC-005). A field the run declares
absent reads as unset, distinguishable from a recorded zero.

Quick manual check of the same thing:

```bash
go run ./internal/cmd/... 2>/dev/null || cat <<'EOF'
# In a scratch program:
#   rd, _ := text.NewRunReader(f)
#   for _, f := range rd.Run().Capabilities.Absent() { fmt.Println(f) }
# prints the fields a Gatling text log never records.
EOF
```

---

## 4. A failure is never counted as a success (User Story 3)

```bash
go test -run TestSuccessSelectionIsUnchangedByFailures ./model/ -v
go test -tags=integration -run TestCorpusSuccessSelectionIsUnchangedByFailures ./gatling/text/ -v
```

**What it does**: takes a decoded run, selects the successful samples, then re-runs the selection
with every failed sample of that run added back to the input.

**Expected**: identical multisets, for every corpus run and for generated runs mixing the two in any
proportion (SC-003).

---

## 5. Memory does not grow with the log (SC-004)

```bash
go test -tags=integration -run TestModelPeakMemory ./gatling/text/ -v
go test -bench BenchmarkRunReader -benchmem -run '^$' ./gatling/text/
```

**Expected**: peak memory under 32 MiB on a 256 MiB generated log, unchanged when the log is made ten
times larger. Both `PeakMemory` tests run in the workflow's dedicated no-race step and are skipped
by the race and coverage steps, whose instrumentation moves the very figure they assert on. The benchmark's `B/op` is compared against the decoder's own recorded figure; a
regression is justified in the PR or it is a bug.

Chunked and whole-file agreement is inherited from the decoder and re-asserted through the model, so
a conversion that quietly buffered would fail here:

```bash
go test -run TestModelChunkedFixtures ./gatling/text/ -v
go test -tags=integration -run TestModelChunkedCorpus ./gatling/text/ -v
```

---

## 6. The probe is held to its document (User Story 4)

Start the stub, then run the probe under each supported version:

```bash
go run ./testdata/corpus/gatling/simulation/stub
```

```bash
cd testdata/corpus/gatling/simulation && sbt -Dgatling.version=3.11.5 "Gatling/testOnly io.galaxio.parsec.corpus.CorpusSimulation"
```

**Expected**, and verified during Phase 0 under both 3.11.5 and 3.12.0 ([research.md](research.md)
R1) — nine assertions, rendered from [contracts/nfr.yaml](contracts/nfr.yaml), all true:

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
```

Note the two spaces in `inner  with comma`: Gatling records the group's declared comma as a space and
addresses assertions by recorded names, which is why the document carries the recorded spelling and
says so beside it (FR-029).

### 6a. Changing an expectation touches one file (SC-007)

Edit a threshold in `src/test/resources/nfr.yaml` — say the failed-request count from 18 to 17 — and
re-run. No Scala file is touched.

**Expected**: the run fails, naming the requirement.

```text
Global: count of failed events is 17.0 : false (actual : 18.0)
[error] Simulation CorpusSimulation failed.
```

### 6b. An unrenderable requirement refuses the whole document (SC-009)

Add a predicate the renderer cannot express — a `good` numerator, or `aggregation: sum` — and re-run.

**Expected**: no assertions at all, and every reason listed, so a run can never check fewer
requirements than its document states.

```text
OpenNfrException: src/test/resources/nfr.yaml is not a renderable OpenNFR document:
  - unrenderable/good-share: `good` has no expressible numerator: a selector matches presence and never absence
  - unrenderable/summed: aggregation `sum` over a metric has no equivalent: responseTime offers no sum
```

### 6c. The document validates against the published schema (FR-026)

Runs in CI on every change, against the schema `galax-io/opennfr` publishes. A document carrying an
unknown field is rejected naming the field — the schema sets `additionalProperties: false`
throughout.

---

## 7. The canary, unchanged in shape

```bash
PARSEC_CANARY_RUNS="3.11.5=/path/to/run;3.12.0=/path/to/run" go test -tags=canary ./gatling/text/
```

The canary keeps holding a fresh run to its own fresh report through the wire records, and now also
through the model. Without a Gatling to run it skips with a reason; in the pipeline a canary run in
which no test passed is a failure.

---

## What is deliberately not here

- **No corpus recording step.** The two entries under `testdata/corpus/gatling/` are not re-recorded:
  they were captured with the reports their own Gatling generated, which cannot be recreated. The
  document changes what a *fresh* run is held to, and how a future version is recorded.
- **No statistics.** Counts here are the verification suite's, taken to compare against a report.
  Percentiles, ranges and series are v0.0.7 and v0.0.8.
- **No summary-only run.** `Aggregate` is v0.5.0; see the Complexity Tracking row in
  [plan.md](plan.md).

---

## Note on the commands above

Every `go test -run` here names a test that exists. `go test -run` on a pattern that matches nothing
prints `no tests to run` and **exits 0**, so a stale name in this file would read as a passing proof
of something that never ran. If you rename a test, rename it here in the same change.
