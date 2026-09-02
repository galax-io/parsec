// Package gatling holds what every Gatling codec shares: the wire records a
// simulation.log carries, the version type, the version gate and the errors a
// read can end with.
//
// These are the log's own records, not the canonical result model. Nothing here
// derives a count, a timing or a percentile; the canonical model in model/ and
// the conversion into it arrive in a later milestone. Until v0.1.0 the exported
// identifiers may change between releases.
package gatling
