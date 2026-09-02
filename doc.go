// Package parsec is the root of the Galaxio load-test result primitives.
//
// The work is split across packages, none of which exists yet:
//
//   - model: the canonical result types every source is decoded into,
//     and the Capabilities a source declares about what it cannot provide.
//   - gatling: text and binary simulation.log codecs for Gatling 3.11.5
//     onwards, the version gate, and run-directory discovery.
//   - stats: counts, timings, percentiles, response-time ranges and
//     per-interval series computed from decoded samples.
//
// Each arrives in its own milestone. Until v0.1.0 the API may change
// between releases; see the README for what becomes stable and when.
package parsec
