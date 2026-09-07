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
//
// # Following a log that is still being written
//
// The readers here, and the codecs behind them, accept a source that is still
// growing. A follower — the comet sidecar is the one this was written for —
// wraps its own polling in an [io.Reader] and hands it over; what this package
// promises in return is:
//
//   - A source whose Read blocks is supported. A read that returns fewer bytes
//     than asked for is a wait, not an end.
//   - A record whose bytes arrive across separate reads is delivered exactly
//     once, when its last byte lands, and is identical to what a whole-file read
//     of the same log yields.
//   - The end of input is the caller's statement and is taken as given. Nothing
//     here polls, reopens or stats a file to guess whether the writer is still
//     alive: it has no way to know, and for a binary log cut on a record
//     boundary no reader could.
//   - Memory held while waiting stays bounded and does not grow with the records
//     already delivered.
//   - A failure of the source is reported as a failure, including one that
//     merely wraps io.EOF, and never as the end of the log.
//
// What the follower must do in return:
//
//   - Read from the first byte of the file. The binary codec rebuilds its string
//     cache as it goes, and a record can name a string introduced megabytes
//     earlier, so starting anywhere else is silently wrong.
//   - Block when there are no new bytes rather than returning (0, nil) in a
//     loop. That return is legal and must not be read as an end, so a spinning
//     source would wedge a reader; it is caught and ended with io.ErrNoProgress
//     rather than hung, but that is a backstop, not a supported mode.
//   - Return io.EOF only when the follow is over. An io.EOF inside a record is a
//     log cut short, and is reported as a *gatling.TruncationError.
//   - Make a blocked read returnable. The reader is inside Read; only the source
//     can end that, so cancellation belongs to the follower.
//   - Report a file that shrank or was replaced as an error. This package cannot
//     see it, and an io.EOF would be read as the log ending normally.
package simlog
