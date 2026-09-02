package gatling

import "fmt"

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

// String names the version and the range it lies outside.
func (w Warning) String() string {
	return fmt.Sprintf("gatling: version %s is above the verified range %s through %s: "+
		"no recording covers it, so the records decode unverified", w.Version, w.Min, w.Max)
}
