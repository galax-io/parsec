// Package parsec is the root of the Galaxio load-test result primitives.
//
// The work is split across packages:
//
//   - [github.com/galax-io/parsec/model]: the canonical result types every
//     source is decoded into, and the Capabilities a source declares about what
//     it cannot provide. This is what a consumer builds on.
//   - [github.com/galax-io/parsec/gatling/run]: finding which Gatling run to
//     read — a log, the directory holding one, or a results root to search. It
//     runs before everything below it and depends on none of it.
//   - [github.com/galax-io/parsec/gatling/simlog]: opens a simulation.log
//     without being told which Gatling wrote it, by identifying the format from
//     the file's leading bytes. This is the entry point.
//   - [github.com/galax-io/parsec/gatling]: the wire records a Gatling
//     simulation.log carries, the version type and the version gate every
//     Gatling codec shares.
//   - [github.com/galax-io/parsec/gatling/text]: the codec for the
//     tab-separated simulation.log written by Gatling 3.11.5 through 3.12.0.
//   - [github.com/galax-io/parsec/gatling/binary]: the codec for the binary
//     simulation.log Gatling writes from 3.13.0, accepting 3.13.1 through
//     3.15.1 — the range its golden corpus covers.
//
// # Where to start
//
// Three calls read a Gatling run:
//
//	loc, err := run.Find("target/gatling")     // which run
//	rd, err := simlog.NewRunReader(f)          // open its log, whatever wrote it
//	for { item, err := rd.Next(); ... }        // fold the items
//
// Nothing in that path names a log format, and nothing in the items names a
// tool. Reach for a codec package directly only when the version is already
// known, and for the records in gatling only when you need to see what the file
// actually held rather than what it means.
//
// This module computes no statistic. It decodes artefacts and offers the
// primitives a consumer computes from; counts, percentiles and series belong to
// the consumer, and galaxio-cli is where they are computed.
//
// From v0.1.0 the exported surface of these packages is stable: see CHANGELOG.md
// for the compatibility promise and the Gatling versions it covers.
package parsec
