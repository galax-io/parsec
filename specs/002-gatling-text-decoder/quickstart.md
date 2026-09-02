# Quickstart: validating the Gatling text decoder

**Feature**: `002-gatling-text-decoder` | **Date**: 2026-09-02 | **Plan**: [plan.md](plan.md)

How to prove this feature works, in the order the evidence has to be built. Steps 1 and 2 come
first because the corpus cannot be recorded after the fact — Principle III makes the moment of the
run the only moment its report can be captured.

API shapes referenced below are in [contracts/gatling-text.md](contracts/gatling-text.md); field
meanings are in [data-model.md](data-model.md).

---

## Prerequisites

- Go 1.25, matching the `go` directive in `go.mod`
- Gatling 3.11.5 and 3.12.0, both pinned, for recording the corpus once
- An endpoint the sample simulation can drive into both a failed check and a connection-level
  exception — a local stub is enough and is preferable to a shared service

---

## Step 1 — build the sample simulation

One simulation, run unchanged under both versions. It must exercise, in a single run:

- every record kind, including at least one declared assertion, so assertion records appear
- nested groups, and at least one request outside any group
- a **group name containing a comma**, to prove the path split is lossless (FR-005)
- a request that fails a **check**, and a request that fails with an **exception** — the second
  produces an error record as well as a failed request (FR-020)
- an error whose message spans **more than one line**, if the endpoint can be made to produce one.
  This is the confirmed-unescaped case in [research.md](research.md) R5. Whether it occurs decides
  whether a real Gatling log can defeat the fail-fast rule, and it cannot be established later.

## Step 2 — record the corpus

For each of 3.11.5 and 3.12.0, run the simulation and capture, at that moment:

```text
testdata/corpus/gatling/<version>/
├── simulation.log        # exactly as written, not reformatted
├── global_stats.json     # js/global_stats.json from the report that run generated
└── stats.json            # js/stats.json from the same report
```

Before archiving, confirm by inspection that the two JSON files carry request counts split
total/ok/ko and a mean requests-per-second figure, and note whether they carry anything for virtual
users or error records. If a file does not carry what FR-021 and FR-021b compare against, capture
whatever does — after the run is archived nothing can be added.

Record the Gatling version, the machine's line separator and the JVM charset alongside the entry.
The line separator matters: a run on Windows produces carriage returns natively
([research.md](research.md) R6).

## Step 3 — generate the golden record streams

```bash
go test -tags=integration -run TestGolden -update ./gatling/text/
```

Review `records.golden` as a diff before committing it. Regeneration is behind an explicit flag so
a golden file can never update itself as a side effect of a passing run.

---

## Running the suites

Unit tests, race detector on, order shuffled — the local verify step and the CI command:

```bash
go test -race -shuffle=on ./...
```

End-to-end over the recorded runs:

```bash
go test -tags=integration -race ./...
```

Coverage against the floors — 90% for `gatling/` and `gatling/text/`, 80% overall:

```bash
go test -tags=integration -cover ./...
```

Throughput and allocation against the recorded baseline:

```bash
go test -tags=integration -bench=BenchmarkReader -benchmem ./gatling/text/
```

---

## What each check proves

| Check | Proves | Requirement |
|---|---|---|
| Decoded stream equals `records.golden`, field for field | the decoder reproduces the recording exactly | SC-001 |
| Request counts total/ok/ko equal `global_stats.json`, per request and group equal `stats.json` | the numbers are Gatling's own, to the unit | SC-002, FR-021 |
| Mean request rate equals the report's figure | span and counts are both right; see the formula in [research.md](research.md) R7 | FR-021b, FR-021c |
| User starts, user ends and error records equal the golden stream | the counts the report cannot prove are still pinned | FR-021a |
| 3.11.5 and 3.12.0 streams match once timestamps, run id and version are set aside | the format did not move between the versions | SC-003 |
| Whole-file read equals reads chunked at arbitrary boundaries | streaming has no seam | SC-005, FR-018 |
| One corrupted line, at first, middle and last position, fails naming that line | fail-fast lands on the right line every time | SC-006, FR-013 |
| Mutated and truncated inputs, ≥ 10,000 of them, produce errors and no panic | no input can crash the reader | SC-007, FR-015 |
| Below range refused, in range clean, above range warned, non-release refused | all four gate outcomes are real | SC-008, FR-009a–FR-011 |
| 1 GB read under 32 MiB peak, and the same peak ten times larger | memory is independent of log size | SC-004, FR-017 |

---

## Reading the evidence by hand

No example binary is added — everything below is visible from the verbose end-to-end run, and a
command-line tool is milestone v0.0.10's concern, not this one.

```bash
go test -tags=integration -race -run 'TestGolden|TestReport|TestCrossVersion' -v ./gatling/text/
```

Expected in the output, per recorded version: `TestReport` logs the request, group, user and error
counts and the run span it compared, and any mismatch names the request or group and both numbers;
`TestCrossVersion` logs that the two recordings are identical as a multiset. Both versions report
36 requests (18 ok, 18 ko), 12 groups, 6 user starts, 6 user ends and 6 errors.

For the failure path:

```bash
go test -race -run 'TestSyntax|TestGate' -v ./gatling/text/
```

Expected: for a corruption at the first event line, in the middle and at the last line, one error
each naming exactly that line number — and in no case a record stream returned as a success.

---

## Definition of done

- [ ] Both corpus entries recorded, each with its own two report files, and the file contents
      confirmed at capture time
- [ ] Golden record streams committed and reviewed as diffs
- [ ] `go test -race -shuffle=on ./...` green
- [ ] `go test -tags=integration -race ./...` green, and not green by skipping
- [ ] Coverage at or above 90% for the decoder packages, 80% overall, with the numbers in the PR
- [ ] Benchmark recorded as the baseline: ≥ 100 MB/s on one core, peak under 32 MiB
- [ ] `CHANGELOG.md` entry under Added
- [X] Issue #3's acceptance criterion corrected to match the fail-fast decision
- [ ] PR carries milestone v0.0.2 and closes issue #3
