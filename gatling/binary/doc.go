// Package binary decodes the simulation.log Gatling writes from 3.13.0: an
// undocumented binary stream with a string cache and JVM-compact strings.
//
// A [Reader] takes an [io.Reader], reads the run record, gates on the version it
// names, then yields one record at a time in file order with memory that does
// not grow with the log. [RunReader] is the model-facing counterpart, yielding
// the same canonical results the text codec produces for an equivalent run.
//
// # The stream must begin at the first byte of the file
//
// The format replaces a repeated string with a back-reference into a table the
// reader rebuilds as it goes, and that table cannot be reconstructed from the
// middle. There is no way to resynchronise: a reader starting late has no table,
// so every back-reference it meets names an entry it never saw. This is a
// property of the format rather than a limitation of this package.
//
// # Versions
//
// This codec accepts Gatling 3.13.1 through 3.15.1 — the range its golden corpus
// covers — and applies the shared version policy from the parent package: below
// the range is refused, above it decodes with a warning, and
// [gatling.WithStrict] turns that warning into a refusal.
//
// The format itself begins at 3.13.0, and a 3.13.0 log is nonetheless refused.
// That version cannot read back the assertion records it writes, so no run of it
// produces the report a golden-corpus entry needs, and the constitution binds a
// codec's range to its corpus rather than to what it believes it could read.
//
// # Byte order
//
// Every integer in the format is big-endian. A string is the JVM's internal
// character array plus a marker naming its encoding: Latin-1, or UTF-16 in the
// writing JVM's native byte order. The file records nothing about which byte
// order that was; little-endian is assumed, because every machine this corpus
// could be recorded on is little-endian, and no recording can prove it.
package binary
