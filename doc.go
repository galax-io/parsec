// Package parsec is the root of the Galaxio load-test result primitives.
//
// The work is split across packages:
//
//   - gatling: the wire records a Gatling simulation.log carries, the
//     version type and the version gate every Gatling codec shares.
//   - gatling/text: the codec for the tab-separated simulation.log written
//     by Gatling 3.11.5 through 3.12.0.
//   - model (planned): the canonical result types every source is decoded
//     into, and the Capabilities a source declares about what it cannot
//     provide.
//   - stats (planned): counts, timings, percentiles, response-time ranges
//     and per-interval series computed from decoded records.
//
// Each arrives in its own milestone. Until v0.1.0 the API may change
// between releases; see the README for what becomes stable and when.
package parsec
