// Package gatling holds what every Gatling codec shares: the wire records a
// simulation.log carries, the version type, the version gate and the errors a
// read can end with.
//
// Finding the run to read comes before any of that and lives in gatling/run,
// which imports nothing from here: it is not something the codecs share.
//
// These are the log's own records, not the canonical result model. Nothing here
// derives a count, a timing or a percentile; the canonical model in model/ and
// the conversion into it arrive in a later milestone. Until v0.1.0 the exported
// identifiers may change between releases.
package gatling
