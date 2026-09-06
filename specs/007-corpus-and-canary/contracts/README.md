# Contracts: The corpus and the canary

**Feature**: 007-corpus-and-canary | **Date**: 2026-09-06

Four interfaces change or appear. Only the first is public API; the rest are operator-facing and are
recorded here because a maintainer depends on them the way a consumer depends on a signature.

| # | Contract | Audience | Kind of change |
|---|---|---|---|
| 1 | [`gatling/binary.MaxStringLen`](public-api.md) | consumers of the module | **value change, observable** — approved 2026-09-06 |
| 2 | [`PARSEC_CANARY_RUNS`](canary-env.md) | the canary workflow, and a maintainer running it locally | extended, backward compatible |
| 3 | [`record-corpus` workflow](record-corpus-workflow.md) | a maintainer recording a version | new |
| 4 | [fuzz legs](fuzz-ci.md) | CI | new |

`internal/corpus` is deliberately **not** a contract: it is internal by construction, invisible to
consumers, and free to change. Research [R3](../research.md) explains why it exists and why it is
named for the package [parsec#59](https://github.com/galax-io/parsec/issues/59) will grow.
