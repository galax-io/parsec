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
// a Gatling simulation.log in a format this module cannot read yet.
//
// It is neither of the two failures it would otherwise be mistaken for. The
// bytes were recognised, so this is not a *FormatError; nothing was decoded, so
// it is not a *SyntaxError. A caller can tell a user that the file is fine and
// the reader is not yet, which is a different message from either.
//
// No input produces one today: this module has a codec for both formats Gatling
// writes. It is kept because the answer it gives — "this is a Gatling log, of a
// format nothing here reads" — is the one a third format would need, and a
// consumer's errors.As branch for it costs nothing while it cannot fire.
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

// Error names the format and says plainly that nothing here reads it yet.
func (e *UnsupportedFormatError) Error() string {
	return fmt.Sprintf("gatling: %s simulation.log: this module has no codec for it yet", e.Format)
}
