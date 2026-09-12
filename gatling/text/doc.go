// Package text decodes the tab-separated simulation.log written by Gatling
// 3.11.5 through 3.12.0.
//
// There are two entry points. [RunReader] is the model-facing one: it yields the
// canonical results of
// [github.com/galax-io/parsec/model], the same values the binary codec produces
// for an equivalent run, and it is what a report is written against. [Reader] is
// the wire-facing one: it yields the log's own records, for a consumer that
// needs to see what the file actually held.
//
// Either takes an io.Reader, walks the preamble to the run header, gates on the
// version it names, then yields one value at a time in file order with memory
// that does not grow with the log. The first line that cannot be decoded ends
// the read with an error naming the line number, and nothing delivered before it
// is a result. A log merely cut short — a run killed mid-flight, whose last line
// was never terminated — is a different ending: it yields a
// [github.com/galax-io/parsec/gatling.TruncationError], and the records before
// the cut are what the run recorded. See [Reader] for the three endings a read
// may have.
//
// This package is the shortcut for a log already known to be 3.11.5 through
// 3.12.0. When the version is not known,
// [github.com/galax-io/parsec/gatling/simlog] picks the codec from the file's
// leading bytes and yields the same values.
package text
