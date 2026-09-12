// Package gatling holds what every Gatling codec shares: the wire records a
// simulation.log carries, the version type, the version gate and the errors a
// read can end with.
//
// These are the log's own records, not the canonical result model. Nothing here
// derives a count, a timing or a percentile — that arithmetic belongs to the
// consumer. The canonical model is
// [github.com/galax-io/parsec/model], and both codecs convert into it: reach for
// [github.com/galax-io/parsec/gatling/simlog].NewRunReader, which opens a log
// without being told which Gatling wrote it, and fold the items it yields.
// [Record] and the rest of this package are what you read instead when you need
// to see what the file actually held.
//
// Finding the run to read comes before any of that and lives in
// [github.com/galax-io/parsec/gatling/run], which imports nothing from here: it
// is not something the codecs share. The whole path is run.Find, then
// simlog.NewRunReader, then a fold over Next.
package gatling
