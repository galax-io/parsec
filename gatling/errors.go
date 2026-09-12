package gatling

import (
	"fmt"
	"strconv"
)

// SyntaxError ends a read: the position it names could not be decoded, and
// nothing after it was read. There is no partial result beside it — records
// delivered before it are not a result, and no total may be derived from them.
//
// One type serves both Gatling log formats. A caller catching a malformed log
// wants one errors.As for either, and a decoder that gives up is the same event
// whichever grammar it was reading; two types would make every consumer branch
// on the format to ask the same question. Which position field is meaningful
// depends on the format the error came from, and Error renders whichever is set.
type SyntaxError struct {
	// Line is the 1-based number of the line that could not be decoded, for a
	// text log. It is 0 when the input was empty, and 0 for a binary log, where
	// Offset carries the position instead.
	Line int
	// Offset is the 0-based byte offset at which decoding stopped, for a binary
	// log. It is 0 for a text log.
	//
	// Exactly one of Line and Offset is meaningful, and Format says which.
	// Principle II asks a decoder for "the byte offset (line number for text
	// formats)", so both positions are real and this type carries both rather
	// than pretending one is the other.
	Offset int64
	// Format is the log format the failing decoder was reading. It says which of
	// Line and Offset to read, and it is not redundant with them: a binary log
	// can fail at byte 0 and a text log can fail before it has a line, so both
	// positions are legitimately zero and neither can discriminate on its own.
	//
	// These three fields are the v0.1.0 contract. Folding the two positions into
	// one was considered and rejected: it loses the type-level distinction
	// between a line and a byte, and leaves nothing to discriminate on.
	//
	// Both codecs in this module set it. The zero value is FormatUnknown, which
	// renders as a line: that is what an error built by something other than a
	// codec — a test, or a consumer constructing one by hand — reads as.
	Format Format
	// Expected says what the reader needed at that position.
	Expected string
	// Found says what was there instead.
	Found string
}

// Error names the line, what was expected there and what was found.
func (e *SyntaxError) Error() string {
	if e.Format == FormatBinary {
		return fmt.Sprintf("gatling: byte %d: expected %s, found %s", e.Offset, e.Expected, e.Found)
	}

	return fmt.Sprintf("gatling: line %d: expected %s, found %s", e.Line, e.Expected, e.Found)
}

// TruncationError ends a read whose bytes ran out inside a record: the log was
// cut short rather than damaged. Every record decoded before that point has
// already been delivered, and each is exactly what a whole-file read of the same
// prefix yields.
//
// It is what a run killed mid-flight leaves behind. Gatling writes the log
// through a buffer flushed in blocks, so a test stopped by a signal, an OOM kill,
// a CI timeout or a full disk ends inside a record: the shape is the ordinary
// ending of an aborted run, and usually not evidence of a corrupt file.
//
// Usually, because two shapes are indistinguishable in the artefact and this
// type cannot separate them. A length prefix corrupted to claim more bytes than
// the file holds is byte for byte what a file cut mid-value looks like, so a
// damaged log can arrive here — with a Dropped as large as the file — rather
// than as a *SyntaxError, which is what a defect the grammar *can* see still
// yields. And a binary log cut exactly on a record boundary is a shorter valid
// log, so it ends cleanly with no truncation at all. A caller that must not act
// on a damaged run needs a check this module cannot give it, such as the run's
// own report or a writer it can see.
//
// It is deliberately neither io.EOF nor a wrapper around one, and it unwraps to
// nothing. A caller written before this type existed breaks its loop on io.EOF
// and treats every other error as a failed read, so it goes on doing exactly
// what it does today and cannot mistake a killed run for a complete one. A
// caller that wants the partial run reaches for this type with errors.As.
//
// Whether a run whose log was cut short may be used at all is the caller's to
// decide. This module states the fact and derives nothing from it.
//
// A binary log cut exactly on a record boundary carries no evidence of the cut:
// the format has no end marker, so such a file is a shorter valid log and ends
// the read with io.EOF like any complete one. Only something that can see the
// writer can tell those two apart.
type TruncationError struct {
	// Line is the 1-based number of the line the incomplete record began on, for
	// a text log. It is 0 for a binary log, where Offset carries the position.
	Line int
	// Offset is the 0-based byte offset the incomplete record began at, for a
	// binary log. It is 0 for a text log.
	//
	// It names where the incomplete record began, not where the stream stopped.
	// That is the position a reader would open the file at to see what was lost;
	// the stream stopped at Offset+Dropped.
	Offset int64
	// Format is the log format the reader was reading. It says which of Line and
	// Offset to read, for the reason SyntaxError.Format gives.
	Format Format
	// Expected says what the reader still needed when the bytes ran out. It is
	// not what sits at the position this error names: Line and Offset are pinned
	// to the start of the incomplete record, while the value being read moves
	// through it, so the two are rarely the same place. Error words it that way
	// round.
	Expected string
	// Dropped is how many trailing bytes could not be decoded. It is always
	// positive: bytes that run out at a record boundary are the clean end of a
	// log and are reported as io.EOF, not as a truncation.
	Dropped int64
}

// Error names where the incomplete record began, how many bytes were left
// undecoded, and what was still to come. The position and the expectation are
// stated apart, because Expected is rarely at the position named.
func (e *TruncationError) Error() string {
	if e.Format == FormatBinary {
		return fmt.Sprintf("gatling: byte %d: the log is cut short: %d trailing bytes could not be decoded, and %s was still to come",
			e.Offset, e.Dropped, e.Expected)
	}

	return fmt.Sprintf("gatling: line %d: the log is cut short: %d trailing bytes could not be decoded, and %s was still to come",
		e.Line, e.Dropped, e.Expected)
}

// VersionError ends a read before any record is delivered: the log names a
// version below the supported range, or names no release version at all.
type VersionError struct {
	// Found is the version string exactly as the log wrote it, so a string that
	// did not parse can be quoted back.
	Found string
	// Version is the release Found parsed as, meaningful only when Parsed is
	// true. It is not a discriminator: 0.0.0 is a version string that parses.
	Version Version
	// Parsed says which fault this is: true when Found is a release that lies
	// below the supported range, false when Found is not a release at all.
	Parsed bool
	// Min and Max bound the supported range.
	Min, Max Version
}

// Error names the version found and the range supported.
func (e *VersionError) Error() string {
	if !e.Parsed {
		return fmt.Sprintf("gatling: version %q is not a release version; supported range is %s through %s",
			e.Found, e.Min, e.Max)
	}

	return fmt.Sprintf("gatling: version %s is below the supported range %s through %s", e.Found, e.Min, e.Max)
}

// Warning is raised for a log written by a version above the range any
// recording covers. The log decodes, and the warning travels in the result;
// it is never only logged.
type Warning struct {
	// Version is the release that wrote the log.
	Version Version
	// Min and Max bound the range recordings cover.
	Min, Max Version
}

// String names the version and the range it lies outside, and is empty for the
// zero Warning.
//
// The zero value is how "no warning" travels — Policy.Apply returns it for
// every accepted version — so rendering it as a warning about version 0.0.0
// would put a false alarm in the log of every healthy run.
func (w Warning) String() string {
	if w == (Warning{}) {
		return ""
	}

	return fmt.Sprintf("gatling: version %s is above the verified range %s through %s: "+
		"no recording covers it, so the records decode unverified", w.Version, w.Min, w.Max)
}

// FormatError ends a read before anything is decoded: the bytes at the start of
// the stream are not a Gatling simulation.log.
//
// It is not a damaged log. A damaged log is one whose format was recognised and
// whose contents then failed, which is a *SyntaxError naming a line.
type FormatError struct {
	// Head is the leading bytes that were examined, at most DetectSize of them.
	Head []byte
	// Short says the bytes ran out while they were still a possible opening,
	// rather than being long enough and matching neither format. The two ask
	// different things of a caller: one may have more bytes to offer, the other
	// has nothing to gain by fetching them.
	Short bool
}

// Error names what was found at the start of the stream, or how far the input
// got before it ran out.
func (e *FormatError) Error() string {
	if e.Short {
		// Deliberately not "the input ended": Detect is documented to accept a
		// head shorter than DetectSize, so all it knows is that the bytes it was
		// given ran out mid-opening. Whether the stream has more is the caller's
		// to say.
		return fmt.Sprintf("gatling: not a Gatling simulation.log: %d bytes is too few to tell, "+
			"and they are still a possible opening", len(e.Head))
	}

	// Quoted rather than raw: a gzip stream or a truncated archive would
	// otherwise spray unprintable bytes through the caller's log.
	return fmt.Sprintf("gatling: not a Gatling simulation.log: found %s at the start of the stream",
		strconv.Quote(string(e.Head)))
}

// UnverifiedError ends a read that asked for strictness: the log names a
// version above the range any recording covers, so nothing proves its records
// would be decoded correctly.
//
// It is the opposite gap to a VersionError. That one refuses a version older
// than any evidence; this one refuses a version newer than any evidence, on a
// caller's instruction rather than as a rule. Without WithStrict the same log
// decodes and raises a Warning instead.
type UnverifiedError struct {
	// Version is the release that wrote the log.
	Version Version
	// Min and Max bound the range recordings cover.
	Min, Max Version
}

// Error names the version, the range no recording covers it in, and that the
// read asked to be strict — without which the same version would have decoded.
func (e *UnverifiedError) Error() string {
	return fmt.Sprintf("gatling: version %s is above the verified range %s through %s: "+
		"no recording covers it, and this read is strict", e.Version, e.Min, e.Max)
}

// UnsupportedFormatError ends a read before anything is decoded: the stream is
// a Gatling simulation.log in a format the reader that was asked does not
// decode.
//
// It is neither of the two failures it would otherwise be mistaken for. The
// bytes were recognised, so this is not a *FormatError; nothing was decoded, so
// it is not a *SyntaxError. A caller can tell a user that the file is fine and
// this reader is the wrong one, which is a different message from either.
//
// Two things produce it. A codec handed the other format's log returns one
// naming the format it found, so a consumer with a mixed archive does not
// quarantine every log of the other format as damaged; use
// [github.com/galax-io/parsec/gatling/simlog] to open a log without being told
// which Gatling wrote it. And a format this module knows of but has no codec for
// would return one from simlog itself — no input reaches that today, since both
// formats Gatling writes have a codec.
//
// The type is part of the v0.1.0 contract. It was a candidate for removal while
// nothing produced one; the wrong-codec case is what decides it, because
// overloading *FormatError there would make a consumer's "not a Gatling file"
// branch wrong for every log of the other format.
type UnsupportedFormatError struct {
	// Format is the format that was detected.
	Format Format
	// Head is the leading bytes identification read from the stream before it
	// decided, at most DetectSize of them.
	//
	// They are returned because reading them consumed them: a caller holding a
	// stream it cannot rewind — a pipe, a response body, an archive entry —
	// needs them back to spool the log aside for a later version of this module.
	// Without them the file it writes is missing its own header.
	Head []byte
}

// Error names the format found and says plainly that the reader in hand does
// not decode it. It is worded for both producers: the codec handed the other
// format's log, and a format this module has no codec for at all.
func (e *UnsupportedFormatError) Error() string {
	return fmt.Sprintf("gatling: %s simulation.log: this reader does not decode it", e.Format)
}
