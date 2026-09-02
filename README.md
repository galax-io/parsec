# parsec

Load-test result primitives for Go.

**Nothing is implemented yet.** The repository holds a scaffold and a backlog; this file says what is being built and why, not what works today. Progress is visible in the [milestones](https://github.com/galax-io/parsec/milestones), each small and ending in a tag.

## The problem

Gatling stopped generating `stats.json` and `global_stats.json` in 3.13.5, and since 3.13.0 it writes `simulation.log` in an undocumented binary format that only the exact same Gatling version can read. A run archived last year cannot be reported on today, and no tool outside Gatling can read a run at all. Every other load-testing tool leaves its results in a shape of its own, so anything that wants to compare two of them starts by writing a parser.

## What it will be

One canonical model for the results of a load test, a decoder per tool that produces it, and a statistics engine that answers the questions a report asks. Three consumers are waiting for it: the `galaxio report` commands in [galaxio-cli](https://github.com/galax-io/galaxio-cli), result ingestion in the Galaxio platform, and the live-metrics sidecar.

### Sources, in the order they are planned

| Source | Formats | Milestone |
|---|---|---|
| Gatling 3.11.5, 3.12.x | text `simulation.log` | v0.0.2 |
| Gatling 3.13.0 … 3.15.x | binary `simulation.log` | v0.0.5 |
| JMeter | JTL, CSV and XML | v0.3.0 |
| k6 | metric output and end-of-test summary | v0.4.0 |
| Locust | CSV statistics | v0.5.0 |
| Yandex.Tank, Pandora | phout | v0.6.0 |

The Gatling log format is internal to Gatling, undocumented, and has already changed once. Every read will be version-gated: a version below the supported range is refused, and an unknown newer one decodes with a warning.

### Packages, once they exist

    model/    canonical result types shared by every source
    gatling/  text and binary codecs, version gate, run discovery
    stats/    counts, timings, percentiles, ranges, per-interval series

`model` and `gatling` will depend on the standard library only, and CI already checks that.

## Status

Empty by design. The public API becomes stable at v0.1.0; until then it may change between releases.

## Licence

MIT.
