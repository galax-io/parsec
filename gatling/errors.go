package gatling

import (
	"fmt"
	"strconv"
)

// SyntaxError ends a read: the line it names could not be decoded, and no line
// after it was read. There is no partial result beside it — records delivered
// before it are not a result, and no total may be derived from them.
type SyntaxError struct {
	// Line is the 1-based number of the line that could not be decoded. It is 0
	// when the input was empty.
	Line int
	// Expected says what the reader needed at that line.
	Expected string
	// Found says what was there instead.
	Found string
}

// Error names the line, what was expected there and what was found.
func (e *SyntaxError) Error() string {
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
