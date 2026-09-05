// Package simlog opens a Gatling simulation.log without being told which
// Gatling wrote it.
//
// A simulation.log carries no magic number and no format version, and the two
// formats Gatling has written — the tab-separated text log through 3.12.0 and
// the binary stream from 3.13.0 — share a filename. This package identifies the
// format from the file's leading bytes and hands the stream to the codec that
// reads it, so a caller holding an archived run does not have to know in
// advance which Gatling produced it.
//
// Reach for it where the version is unknown. Where it is known, the codec
// package is one call shorter and one interface plainer.
package simlog
