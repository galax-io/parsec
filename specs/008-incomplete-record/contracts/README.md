# Contracts: An incomplete record

**Feature**: 008-incomplete-record | **Date**: 2026-09-07

Two contracts. The first is public API and changes observable behaviour; the second is a promise to
a named consumer that this module has been keeping by accident and now states on purpose.

| # | Contract | Audience | Kind of change |
|---|---|---|---|
| 1 | [public API](public-api.md) | every consumer of the module | **one addition, one observable change** — permitted before v0.1.0 (Principle V), recorded under Changed in `CHANGELOG.md` |
| 2 | [the blocking-source contract](blocking-source.md) | the comet sidecar ([comet#3](https://github.com/galax-io/comet/issues/3)), and any follower | **new statement of existing behaviour** — no code change (research R6) |

Neither contract adds a package, an interface or a dependency. `gatling/` and `model/` stay
standard-library only.
