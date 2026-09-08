# Contracts: Finding the run

**Feature**: 009-gatling-run-discovery | **Date**: 2026-09-08

Two contracts. The first is what this module adds to its public surface; the second is what it
assumes about a file another project writes, which is the part most likely to be wrong later.

| # | Contract | Audience | Kind of change |
|---|---|---|---|
| 1 | [public API](public-api.md) | every consumer of the module | **four additions, nothing changed** — no existing identifier moves or alters behaviour |
| 2 | [the `lastRun.txt` contract](lastrun-file.md) | maintainers of this module | **an assumption about `gatling-maven-plugin`**, recorded with the version it was read from |

Neither adds a package or a dependency. `gatling/` stays standard-library only, and `model/` is
untouched: a located run is not a result (see [data-model.md](../data-model.md)).
