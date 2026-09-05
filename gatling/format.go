package gatling

import (
	"bytes"
	"strconv"
)

// Format is the on-disk shape of a Gatling simulation.log. It is a property of
// the bytes in the file and never of its name: the two formats are written to
// the same filename by every Gatling that has ever produced one.
type Format uint8

// The two shapes Gatling has written, behind the unknown sentinel.
const (
	// FormatUnknown is the zero value: nothing has been detected. Detect never
	// returns it with a nil error.
	FormatUnknown Format = iota
	// FormatText is the tab-separated log written through Gatling 3.12.0.
	FormatText
	// FormatBinary is the binary stream written from Gatling 3.13.0.
	FormatBinary
)

var formatNames = [...]string{unknownName, "text", "binary"}

// String names the format: "text", "binary", or "unknown" for the zero value.
func (f Format) String() string {
	if int(f) < len(formatNames) {
		return formatNames[f]
	}

	return "Format(" + strconv.Itoa(int(f)) + ")"
}

// DetectSize is the number of leading bytes Detect examines. It is the length
// of the longest opening a log may have, so a caller doing its own buffering
// can size it from here. Detect never looks at more, whatever the size of the
// log.
const DetectSize = 10

// binaryRunRecord is the kind byte a binary log's first record carries. The
// run record is necessarily first, and its kind is zero.
const binaryRunRecord = 0x00

// binaryOpeningLen is how much of the binary opening is checked, and
// maxReleaseStringLen bounds the length prefix inside it.
//
// The kind byte alone is one bit of evidence, and a NUL is the most common
// first byte a file can have: every sparse, zero-padded, or UTF-16BE-without-a-
// BOM artefact starts with one. Answering "binary simulation.log, no codec yet"
// for such a file promises that a later release will read something that is not
// a Gatling log at all — the same confident misdiagnosis this package exists to
// remove. The recording under testdata/samples shows what a real one opens with:
//
//	00 | 00 00 00 06 | "3.15.1" | ...
//
// so the kind byte, a four-byte big-endian length, and the first digit of the
// release string it introduces are all inside the window at no cost.
const (
	binaryOpeningLen    = 6
	maxReleaseStringLen = 64
)

// binaryOpeningByte reports whether b is what a binary log carries at offset i.
func binaryOpeningByte(i int, b byte) bool {
	switch i {
	case 0:
		return b == binaryRunRecord
	case 1, 2, 3:
		// The top three bytes of the release string's length. A release string
		// is a handful of bytes, so they are zero for any real log.
		return b == 0
	case 4:
		return b > 0 && b <= maxReleaseStringLen
	default:
		// The release string opens with its major version.
		return b >= '0' && b <= '9'
	}
}

// binaryOpening reports whether head opens like a binary log, and — when it
// does not — whether it is a prefix that more bytes could still complete.
func binaryOpening(head []byte) (match, couldMatch bool) {
	for i, b := range head {
		if i >= binaryOpeningLen {
			break
		}

		if !binaryOpeningByte(i, b) {
			return false, false
		}
	}

	if len(head) >= binaryOpeningLen {
		return true, false
	}

	return false, true
}

// textOpenings are the only two records that may legally open a text log. A
// simulation that declares assertions writes one ASSERTION record per assertion
// ahead of the run header, so both are ordinary — and the corpus contains only
// the second kind, which is why a rule keyed on RUN alone would be wrong.
//
// The tab is part of each: without it a plain note beginning "Ran the suite"
// would pass for a log.
var textOpenings = [...]string{"RUN\t", "ASSERTION\t"}

// Detect names the format a simulation.log is written in, from its leading
// bytes alone. head may be shorter than DetectSize; Detect decides as soon as
// the bytes are conclusive, and ignores anything past the window.
//
// A binary log opens with the run record's kind byte and the length-prefixed
// release string that follows it. A text log opens with RUN or ASSERTION
// followed by a tab.
//
// It returns a *FormatError when the bytes are not a Gatling simulation.log,
// and one whose Short field is set when they ran out while still a possible
// opening. It never consults a file name, and it never guesses: a format that
// is neither of the two is refused rather than tried.
func Detect(head []byte) (Format, error) {
	if len(head) > DetectSize {
		head = head[:DetectSize]
	}

	if len(head) == 0 {
		return FormatUnknown, &FormatError{Short: true}
	}

	// Set by any rule the bytes could still satisfy, so a caller is told to
	// fetch more only when more would settle it.
	short := false

	match, couldMatch := binaryOpening(head)
	if match {
		return FormatBinary, nil
	}

	short = couldMatch

	for _, opening := range textOpenings {
		if hasPrefix(head, opening) {
			return FormatText, nil
		}

		if isProperPrefixOf(head, opening) {
			short = true
		}
	}

	return FormatUnknown, &FormatError{Head: bytes.Clone(head), Short: short}
}

// hasPrefix reports whether b starts with s. The comparison converts nothing:
// the compiler recognises string(b[:n]) == literal and compares in place.
func hasPrefix(b []byte, s string) bool {
	return len(b) >= len(s) && string(b[:len(s)]) == s
}

// isProperPrefixOf reports whether b is a prefix of s and shorter than it, so
// that more bytes could still make it match.
func isProperPrefixOf(b []byte, s string) bool {
	return len(b) < len(s) && string(b) == s[:len(b)]
}
