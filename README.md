# parsec

Load-test result primitives for Go.

**The Gatling text decoder is the first piece to land.** Everything below it is still a backlog; this file says what is being built and why, and marks what works today. Progress is visible in the [milestones](https://github.com/galax-io/parsec/milestones), each small and ending in a tag.

## The problem

Gatling stopped generating `stats.json` and `global_stats.json` in 3.13.5, and since 3.13.0 it writes `simulation.log` in an undocumented binary format that only the exact same Gatling version can read. A run archived last year cannot be reported on today, and no tool outside Gatling can read a run at all. Every other load-testing tool leaves its results in a shape of its own, so anything that wants to compare two of them starts by writing a parser.

## What it will be

One canonical model for the results of a load test, and a decoder per tool that produces it. Three consumers are waiting for it: the `galaxio report` commands in [galaxio-cli](https://github.com/galax-io/galaxio-cli), result ingestion in the Galaxio platform, and the live-metrics sidecar.

**This library computes no statistic.** No count, no mean, no percentile, no range, no per-interval series. What it owns is the part two implementations diverge on — the definitions: what counts as a failure, what a request position is, where a run begins and ends — and the primitives a consumer computes from.

The arithmetic belongs to the consumers, and there is deliberately more than one of it. `galaxio-cli` summarises a finished run in one pass over an archived log. The `comet` sidecar aggregates a run while it is still being written, in windows, and carries its own arithmetic for it. Those are different computations, and one shared accumulator would have been shaped by neither.

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

### Packages

    model/          canonical result types shared by every source                       (v0.0.3)
    gatling/        version type, version policy, format detection and the errors        (v0.0.2)
                    every Gatling codec shares                                           (v0.0.4)
    gatling/text/   the text simulation.log codec for 3.11.5 and 3.12.0, and the         (v0.0.2)
                    conversion of a log into model types                                 (v0.0.3)
    gatling/simlog/ opens a simulation.log without being told which Gatling wrote it     (v0.0.4)

`model` and `gatling` depend on the standard library only, and CI checks that. No module is pre-approved anywhere: `go.mod` naming no requirement is the intended steady state, not a property of a young project.

## Status

`gatling/` and `gatling/text/` decode a Gatling 3.11.5 or 3.12.0 text `simulation.log` from a
stream, in bounded memory, gated on the version the log names.

`model/` is the canonical form every source is decoded into, and `gatling/text.NewRunReader` reads a
log straight into it — so a report can be written once and work for every tool. What a source cannot
measure is declared through `Capabilities` and never filled in with a zero. A run carries only what
does not grow with its length; its samples, groups, virtual-user events and errors stream beside it,
so a log larger than memory still reads.

`gatling/simlog` opens a log without being told which Gatling wrote it: it reads the first ten bytes,
names the format, and hands the stream to the codec that reads it. A binary `simulation.log` — every
Gatling from 3.13.0 — is refused with an error naming the format and saying no codec reads it yet,
rather than failing as a syntax error on the first line. What the module accepts is readable
programmatically, so a tool reports it instead of hard-coding a version range.

A caller that cannot use a number nothing has verified can say so: `gatling.WithStrict` refuses a
version above the recorded range instead of decoding it with a warning.

The log's own wire records are still there and still exported: they are the format's events rather
than a result, and the binary codec will share them. Build on `model/`.

Every source in the table above except the Gatling text log is unimplemented.

## What this library will not do

Compute statistics. Counts, means, percentiles, response-time ranges and per-interval series are the
consumer's: [galaxio-cli#51](https://github.com/galax-io/galaxio-cli/issues/51) for a finished run,
[galaxio-cli#61](https://github.com/galax-io/galaxio-cli/issues/61) for its series, and `comet` for
the live case with its own arithmetic. This library hands over decoded results and the primitives to
fold them — the position a sample was recorded at, the bounds of the run, the outcome of every
sample — so that consumers computing differently still compute the same thing.

The public API becomes stable at v0.1.0; until then it may change between releases.

## Licence

MIT.
