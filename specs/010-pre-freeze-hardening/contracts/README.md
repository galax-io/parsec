# Contracts: Pre-freeze hardening

**Feature**: 010-pre-freeze-hardening | **Date**: 2026-09-10

No exported identifier is added, renamed or re-typed, so there is no public-API contract to add.
What this feature changes is **observable behaviour of existing exported identifiers** — permitted
before v0.1.0 (Principle V), recorded under Changed or Fixed in `CHANGELOG.md` in the same commit as
each fix — and each contract below states the behaviour as it will be frozen.

| # | Contract | Audience | Kind of change |
|---|---|---|---|
| 1 | [Endings and source failures](endings.md) | every consumer; the comet sidecar as a follower; the Galaxio backend as an ingester | **three fixes** to what `simlog` and both codecs return (#82, #83, #102) — Fixed |
| 2 | [The binary reader's budget and buffer](binary-budget.md) | anyone sizing a process around `binary.Reader`; the sidecar | **two ceilings added, one count removed, the gate moved first, the buffer made the codec's** (#75, #76, #87) — Changed and Fixed |
| 3 | [The run ordering](run-ordering.md) | every caller of `run.Find` without a `lastRun.txt` | **one ordering rule made total** (#88) — Changed |

The two CI issues in this feature are governed by spec 001's contracts, not by new ones here:
[workflows.md](../../001-ci-release-automation/contracts/workflows.md) for the `compat` gate (#106,
landed in PR #109) and [dependency-ownership.md](../../001-ci-release-automation/contracts/dependency-ownership.md)
for the toolchain line (#104, PR #110).
