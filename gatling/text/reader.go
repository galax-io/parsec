package text

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"

	"github.com/galax-io/parsec/gatling"
)

// Reader decodes a Gatling 3.11.5 or 3.12.0 text simulation.log from a stream.
//
// NewReader consumes the preamble and the run header and applies the version
// gate, so Header, Assertions and Warnings are available before the first Next.
// Records then arrive one at a time in file order. Peak memory does not grow
// with the log.
//
// The first line that cannot be decoded ends the read with a *gatling.SyntaxError
// naming its line number, and every later Next returns that same error. Records
// delivered before that point are not a result: no total may be derived from
// them.
type Reader struct {
	sc       *scanner
	p        *parser
	header   gatling.Header
	asserts  []string
	assertsN int
	warnings []gatling.Warning
	err      error
}

// maxAssertionBytes caps what the preamble may retain. One assertion payload
// runs to tens of kilobytes and a simulation declares a handful, so a real log
// stays far below this; a truncated or damaged file whose header never arrives
// would otherwise be held whole, which is the one place the reader's bounded
// memory depended on the input being well formed.
const maxAssertionBytes = 8 << 20

// The constructor is NewReader rather than New: it constructs an io-style
// reader, and the standard library names those NewReader everywhere
// (bufio, csv, gzip), so callers find it where they expect it.

// NewReader reads the preamble and the run header and gates on the version it
// names. It fails when no header can be found, when the version is below the
// supported range, and when the version is not a plain release. A version above
// the range succeeds and records a warning.
//
// It allocates its line buffer up front, once, and refuses any line past
// MaxLineLen, so no line can grow past the ceiling. Beyond that buffer, a
// bounded table of the names the log repeats and a bounded preamble, the
// reader holds only the records it hands out.
func NewReader(r io.Reader) (*Reader, error) {
	rd := &Reader{sc: newScanner(r)}

	// Assertion records precede the header, one per declared assertion. Their
	// field count cannot be judged until the header names the version, so a
	// surplus is remembered — with the count that made it one — and ruled on
	// afterwards.
	surplusLine, surplusFields := 0, 0

	for {
		line, isTerminated, err := rd.sc.next()
		if err != nil {
			return nil, rd.preambleError(err)
		}

		if !isTerminated {
			return nil, unterminated(rd.sc.lineNo)
		}

		kind := kindOf(line)

		switch string(kind) {
		case kindAssertion:
			fields, n := split(nil, line, assertionFields)
			if n < assertionFields {
				return nil, fieldCountError(rd.sc.lineNo, kindAssertion, assertionFields, n)
			}

			if n > assertionFields && surplusLine == 0 {
				surplusLine, surplusFields = rd.sc.lineNo, n
			}

			if rd.assertsN += len(fields[1]); rd.assertsN > maxAssertionBytes {
				return nil, &gatling.SyntaxError{
					Line:     rd.sc.lineNo,
					Expected: "a run header within " + strconv.Itoa(maxAssertionBytes) + " bytes of assertions",
					Found:    "assertions still",
				}
			}

			rd.asserts = append(rd.asserts, string(fields[1]))

		case kindRun:
			return rd.finishPreamble(line, surplusLine, surplusFields)

		default:
			return nil, &gatling.SyntaxError{
				Line:     rd.sc.lineNo,
				Expected: "ASSERTION or RUN before the run header",
				Found:    quote(kind),
			}
		}
	}
}

// preambleError turns the end of input before a header into a syntax error and
// gives any other read failure its line.
func (r *Reader) preambleError(err error) error {
	if errors.Is(err, io.EOF) {
		return &gatling.SyntaxError{Line: r.sc.lineNo, Expected: "a run header", Found: "end of input"}
	}

	return readError(r.sc.lineNo+1, err)
}

// readError adds the line being read to an error from the underlying stream.
// The scanner's own errors already carry their line and pass through untouched.
func readError(lineNo int, err error) error {
	var syntaxErr *gatling.SyntaxError
	if errors.As(err, &syntaxErr) {
		return err
	}

	return fmt.Errorf("gatling: reading line %d: %w", lineNo, err)
}

func unterminated(lineNo int) error {
	return &gatling.SyntaxError{Line: lineNo, Expected: "a line terminator", Found: "end of input"}
}

// finishPreamble decodes the header, applies the gate and settles the field
// count rule for everything read so far.
func (r *Reader) finishPreamble(line []byte, surplusLine, surplusFields int) (*Reader, error) {
	hdr, n, err := parseHeader(line, r.sc.lineNo)
	if err != nil {
		return nil, err
	}

	isLenient := false

	switch gatling.Gate(hdr.Version, minVersion, maxVersion) {
	case gatling.VerdictRefused:
		return nil, &gatling.VersionError{
			Found:   hdr.Version.String(),
			Version: hdr.Version,
			Parsed:  true,
			Min:     minVersion,
			Max:     maxVersion,
		}
	case gatling.VerdictUnverified:
		r.warnings = append(r.warnings, gatling.Warning{Version: hdr.Version, Min: minVersion, Max: maxVersion})
		isLenient = true
	case gatling.VerdictAccepted, gatling.VerdictUnknown:
	}

	if !isLenient {
		if n != runFields {
			return nil, fieldCountError(r.sc.lineNo, kindRun, runFields, n)
		}

		if surplusLine != 0 {
			return nil, fieldCountError(surplusLine, kindAssertion, assertionFields, surplusFields)
		}
	}

	r.header = hdr
	r.p = newParser(isLenient)

	return r, nil
}

// Header returns the run header. Valid as soon as NewReader returns.
func (r *Reader) Header() gatling.Header { return r.header }

// Assertions returns the payloads written ahead of the header, in file order,
// verbatim and uninterpreted. Their number is a property of the simulation
// rather than of the log, so a real run holds a handful; the preamble is
// capped regardless, because a damaged file is not bound by that.
func (r *Reader) Assertions() []string { return slices.Clone(r.asserts) }

// Warnings returns what the version gate raised: empty for a covered version,
// one warning for a version above the range.
func (r *Reader) Warnings() []gatling.Warning { return slices.Clone(r.warnings) }

// Next returns the next record. It returns io.EOF at the end of the log. Any
// other error ends the read — there is no next record after it, and the same
// error is returned on every later call.
//
// The returned record's Groups slice is valid until the next call to Next;
// copy it to keep it.
func (r *Reader) Next() (gatling.Record, error) {
	if r.err != nil {
		return gatling.Record{}, r.err
	}

	line, isTerminated, err := r.sc.next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return gatling.Record{}, err
		}

		r.err = readError(r.sc.lineNo+1, err)

		return gatling.Record{}, r.err
	}

	if !isTerminated {
		r.err = unterminated(r.sc.lineNo)

		return gatling.Record{}, r.err
	}

	rec, err := r.p.parse(line, r.sc.lineNo)
	if err != nil {
		r.err = err

		return gatling.Record{}, err
	}

	return rec, nil
}
