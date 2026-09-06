// Package parsec is the root of the Galaxio load-test result primitives.
//
// The work is split across packages:
//
//   - gatling: the wire records a Gatling simulation.log carries, the
//     version type and the version gate every Gatling codec shares.
//   - gatling/text: the codec for the tab-separated simulation.log written
//     by Gatling 3.11.5 through 3.12.0.
//   - gatling/binary: the codec for the binary simulation.log Gatling writes
//     from 3.13.0, accepting 3.13.1 through 3.15.1 — the range its golden
//     corpus covers.
//   - gatling/simlog: opens a simulation.log without being told which Gatling
//     wrote it, by identifying the format from the file's leading bytes.
//   - model: the canonical result types every source is decoded into, and the
//     Capabilities a source declares about what it cannot provide. This is what
//     a consumer builds on; the records in gatling are the log's own events.
//
// This module computes no statistic. It decodes artefacts and offers the
// primitives a consumer computes from; counts, percentiles and series belong to
// the consumer, and galaxio-cli is where they are computed.
//
// Each package arrives in its own milestone. Until v0.1.0 the API may change
// between releases; see the README for what becomes stable and when.
package parsec
