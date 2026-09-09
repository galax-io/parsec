// Package gatling holds what every Gatling codec shares: the wire records a
// simulation.log carries, the version type, the version gate and the errors a
// read can end with.
//
// It also holds the step before all of them. FindRun turns a path that may be a
// run, a results root, or nothing at all into the run directory and the
// simulation.log inside it, so that the three consumers of this module do not
// each work out where a build tool put its results.
//
// These are the log's own records, not the canonical result model. Nothing here
// derives a count, a timing or a percentile; the canonical model in model/ and
// the conversion into it arrive in a later milestone. Until v0.1.0 the exported
// identifiers may change between releases.
package gatling
