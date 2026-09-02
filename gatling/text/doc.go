// Package text decodes the tab-separated simulation.log written by Gatling
// 3.11.5 through 3.12.0.
//
// A Reader takes an io.Reader, walks the preamble to the run header, gates on
// the version it names, then yields one record at a time in file order with
// memory that does not grow with the log. The first line it cannot decode ends
// the read with an error naming the line number; a partial read is never
// presented as a result.
package text
