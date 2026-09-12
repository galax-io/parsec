// Package run finds the Gatling run to read.
//
// A simulation.log sits in a directory a build tool named, under a results root
// that build tool chose, and until now every consumer of this module worked that
// out for itself. Find answers it once: give it the log, the directory holding
// one, or a results root to search, and it returns where the run's artefacts sit
// and which rule chose them.
//
// It stops exactly where reading begins. No log is opened here and no version
// gate is applied, so a run whose log is truncated, damaged or outside the
// supported range still resolves and fails only when
// [github.com/galax-io/parsec/gatling/text],
// [github.com/galax-io/parsec/gatling/binary] or
// [github.com/galax-io/parsec/gatling/simlog] is handed the file. The usual next
// call is simlog.NewRunReader on [Location].Log.
//
// The package deliberately depends on nothing else in this module. Locating a
// run is not something the codecs share — it happens strictly before them — and
// keeping it separate is what lets the codecs stay pure vocabulary over bytes.
package run

// unknownName renders the zero value of this package's one enum. It is spelled
// here rather than borrowed from gatling so that this package imports nothing
// from the rest of the module.
const unknownName = "unknown"
