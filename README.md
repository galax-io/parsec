# parsec

Load-test result primitives for Go.

One canonical model for the results of Gatling, JMeter, k6, Locust and Yandex.Tank, with decoders that read each tool's own artefacts and a statistics engine that answers the questions a report asks. It is the shared foundation under [galaxio-cli](https://github.com/galax-io/galaxio-cli), the Galaxio backend, and the live-metrics sidecar.

## Why it exists

Gatling stopped generating `stats.json` and `global_stats.json` in 3.13.5, and since 3.13.0 it writes `simulation.log` in an undocumented binary format that only the exact same Gatling version can read. A run archived last year cannot be reported on today, and no tool outside Gatling can read a run at all.

parsec reads those files, and the equivalent artefacts of the other tools, without needing the tool that produced them.

## Supported sources

| Source | Formats | Status |
|---|---|---|
| Gatling 3.11.5, 3.12.x | text `simulation.log` | planned |
| Gatling 3.13.0 … 3.15.x | binary `simulation.log` | planned |
| JMeter | JTL, CSV and XML | planned |
| k6 | metric output and end-of-test summary | planned |
| Locust | CSV statistics | planned |
| Yandex.Tank, Pandora | phout | planned |

The Gatling log format is internal to Gatling, undocumented, and has already changed once. Every read is version-gated: a version below the supported range is refused, and an unknown newer one decodes with a warning.

## Layout

    model/    canonical result types shared by every source
    gatling/  text and binary codecs, version gate, run discovery
    stats/    counts, timings, percentiles, ranges, per-interval series

`model` and `gatling` depend on the standard library only.

## Status

Early. The public API becomes stable at v0.1.0; until then it may change between releases. Work is tracked in milestones, smallest first.

## Licence

MIT.
