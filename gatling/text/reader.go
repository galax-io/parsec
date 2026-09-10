package text

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/internal/source"
)

// Reader decodes a Gatling 3.11.5 or 3.12.0 text simulation.log from a stream.
//
// NewReader consumes the preamble and the run header and applies the version
// gate, so Header, Assertions and Warnings are available before the first Next.
// Records then arrive one at a time in file order. Peak memory does not grow
// with the log.
//
// A read ends in one of three ways, and each ending is terminal — every later
// Next returns the same value.
//
//   - io.EOF is a clean end: the log ended at a line boundary and every record
//     it held was delivered.
//   - A *gatling.TruncationError is a log cut short: the final line was never
//     terminated, which is how a run killed mid-flight ends. Every record before
//     the cut was delivered and is exactly what an intact log would have given.
//   - Any other error is a failed read — a *gatling.SyntaxError naming the line
//     that could not be decoded, or a failure of the source. Nothing may be
//     derived from what was delivered.
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
// the range succeeds and records a warning — or, under [gatling.WithStrict],
// fails with a *gatling.UnverifiedError instead.
//
// The source may still be being written. A Read that blocks is a wait, not an
// end, and a record split across reads is delivered once, when its last byte
// arrives; the [github.com/galax-io/parsec/gatling/simlog] package documentation
// states the contract in full, and it holds for this constructor too.
//
// It allocates its line buffer up front, once, and refuses any line past
// MaxLineLen, so no line can grow past the ceiling. Beyond that buffer, a
// bounded table of the names the log repeats and a bounded preamble, the
// reader holds only the records it hands out.
func NewReader(r io.Reader, opts ...gatling.Option) (*Reader, error) {
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
			return nil, unterminated(rd.sc.lineNo, len(line))
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
					Format:   gatling.FormatText,
					Line:     rd.sc.lineNo,
					Expected: "a run header within " + strconv.Itoa(maxAssertionBytes) + " bytes of assertions",
					Found:    "assertions still",
				}
			}

			rd.asserts = append(rd.asserts, string(fields[1]))

		case kindRun:
			return rd.finishPreamble(line, surplusLine, surplusFields, opts)

		default:
			return nil, &gatling.SyntaxError{
				Format:   gatling.FormatText,
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
	// Identity, for the reason scanner.next gives: a source failing with an
	// error that wraps io.EOF has not reached the end of anything, and blaming
	// the file for a transport fault sends a caller to inspect a log that is
	// fine.
	if err == io.EOF { //nolint:errorlint // deliberate: identity, not wrapping; see above
		return &gatling.SyntaxError{Format: gatling.FormatText, Line: r.sc.lineNo, Expected: "a run header", Found: "end of input"}
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

	// A cause whose chain holds io.EOF must not satisfy errors.Is(err, io.EOF):
	// a caller whose loop breaks on the clean end of a log would read a broken
	// source as a complete run. source.Failed hides io.EOF and keeps every other
	// cause reachable; gatling/binary's sourceFailed does the same.
	return source.Failed(fmt.Sprintf("gatling: reading line %d", lineNo), err)
}

// unterminated reports a line the writer never finished. That is not a damaged
// log: Gatling writes through a buffer flushed in blocks, so a run stopped by a
// signal, an OOM kill or a full disk ends exactly here, and the lines before it
// are as true as any other. dropped is the tail that was never terminated.
//
// A prefix that ends on a line boundary is a different thing and is not this
// error. In the record stream it is a shorter valid log; in the preamble it is a
// log with no run header, which preambleError reports, because a cut on a
// boundary leaves no evidence of itself either way.
func unterminated(lineNo, dropped int) error {
	return &gatling.TruncationError{
		Format:   gatling.FormatText,
		Line:     lineNo,
		Expected: "the rest of the line",
		Dropped:  int64(dropped),
	}
}

// finishPreamble decodes the header, applies the gate and settles the field
// count rule for everything read so far.
func (r *Reader) finishPreamble(line []byte, surplusLine, surplusFields int, opts []gatling.Option) (*Reader, error) {
	hdr, n, err := parseHeader(line, r.sc.lineNo)
	if err != nil {
		return nil, err
	}

	// The decision is not made here. versionPolicy holds this codec's range and
	// gatling.Policy holds the rule, so a second codec cannot disagree with this
	// one about what a version means.
	verdict, warning, err := versionPolicy.Apply(hdr.Version, opts...)
	if err != nil {
		return nil, err
	}

	// Read from the verdict, not from whether a warning came back. The two
	// coincide today, but leniency relaxes real checks — a surplus field is
	// accepted, an error record's timestamp is searched for rather than counted
	// to — and tying that to the presence of a struct would let any warning
	// raised for some later reason silently relax the parser for a version the
	// corpus fully covers.
	isLenient := verdict == gatling.VerdictUnverified
	if isLenient {
		r.warnings = append(r.warnings, warning)
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
// error is returned on every later call. A *gatling.TruncationError says the log
// was cut short and that the records already delivered are what it recorded;
// anything else says the read failed. See [Reader] for the three endings in
// full.
//
// The returned record's Groups slice is valid until the next call to Next;
// copy it to keep it.
func (r *Reader) Next() (gatling.Record, error) {
	if r.err != nil {
		return gatling.Record{}, r.err
	}

	line, isTerminated, err := r.sc.next()
	if err != nil {
		// Latched, like every other ending. bufio's readErr clears its own
		// stored error, so a source that returns io.EOF and then more bytes — a
		// file still being appended to, read without a blocking wrapper — would
		// otherwise deliver records after the end this call declared, and the
		// two codecs behind simlog.RecordReader would disagree about a contract
		// both of them state.
		//
		// Identity, for the reason scanner.next gives.
		if err == io.EOF { //nolint:errorlint // deliberate: identity, not wrapping; see scanner.next
			r.err = err

			return gatling.Record{}, err
		}

		r.err = readError(r.sc.lineNo+1, err)

		return gatling.Record{}, r.err
	}

	if !isTerminated {
		r.err = unterminated(r.sc.lineNo, len(line))

		return gatling.Record{}, r.err
	}

	rec, err := r.p.parse(line, r.sc.lineNo)
	if err != nil {
		r.err = err

		return gatling.Record{}, err
	}

	return rec, nil
}
