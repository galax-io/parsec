// Package text decodes the tab-separated simulation.log written by Gatling
// 3.11.5 through 3.12.0.
//
// A Reader takes an io.Reader, walks the preamble to the run header, gates on
// the version it names, then yields one record at a time in file order with
// memory that does not grow with the log. The first line it cannot decode ends
// the read with an error naming the line number, and nothing delivered before it
// is a result. A log merely cut short — a run killed mid-flight, whose last line
// was never terminated — is a different ending: it yields a
// *gatling.TruncationError, and the records before the cut are what the run
// recorded. See Reader for the three endings a read may have.
package text
