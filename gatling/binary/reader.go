package binary

import (
	"io"
	"slices"

	"github.com/galax-io/parsec/gatling"
)

// Reader decodes a Gatling 3.13.1 through 3.15.1 binary simulation.log from a
// stream.
//
// NewReader consumes the run record and applies the version gate, so Header,
// Assertions and Warnings are available before the first Next. Records then
// arrive one at a time in file order. Peak memory does not grow with the log: it
// is bounded by a fixed read buffer, the deepest group nesting, and the number
// of distinct strings the simulation declares — not by the number of records.
//
// The first byte that cannot be decoded ends the read with a
// *gatling.SyntaxError naming its offset, and every later Next returns that same
// error. Records delivered before that point are not a result: no total may be
// derived from them.
//
// The stream must begin at the first byte of the file. See the package
// documentation for why the format allows nothing else.
type Reader struct {
	rd    reader
	cache cache
	run   runHeader

	warnings []gatling.Warning
	// path is the group scratch every record's Groups slice points into. It is
	// reused between records, which is what a caller must copy to keep.
	path []string
	// err is terminal: once set, every later Next returns it unchanged.
	err error
}

// The constructor is NewReader rather than New, matching gatling/text and the
// standard library's io-style readers.

// NewReader reads the run record and gates on the version it names. It fails
// when the record cannot be read, when the version is below the supported range,
// and when the version is not a plain release. A version above the range
// succeeds and records a warning — or, under [gatling.WithStrict], fails with a
// *gatling.UnverifiedError instead.
func NewReader(r io.Reader, opts ...gatling.Option) (*Reader, error) {
	rd := &Reader{rd: *newReader(r)}

	kind, err := rd.rd.u8("the run record")
	if err != nil {
		return nil, err
	}

	if kind != kindRun {
		return nil, rd.rd.syntax(0, "the run record", describeByte(kind))
	}

	if rd.run, err = readRun(&rd.rd); err != nil {
		return nil, err
	}

	_, warning, err := versionPolicy.Apply(rd.run.header.Version, opts...)
	if err != nil {
		return nil, err
	}

	if warning != (gatling.Warning{}) {
		rd.warnings = append(rd.warnings, warning)
	}

	return rd, nil
}

// Header is the run header, decoded before the first record.
func (r *Reader) Header() gatling.Header { return r.run.header }

// Assertions is the opaque payloads the run record carried, in file order and
// exactly as written. Nothing here decodes or validates one.
func (r *Reader) Assertions() []string { return slices.Clone(r.run.assertions) }

// Warnings is what the version gate raised: one warning for a version above the
// range the corpus covers, and nothing otherwise.
func (r *Reader) Warnings() []gatling.Warning { return slices.Clone(r.warnings) }

// Next returns the next record, or [io.EOF] at the end of the log.
//
// Any other error ends the read: there is no next record after it, the same
// error is returned on every later call, and the records already delivered are
// not a result.
//
// The returned record's Groups slice is backed by memory the reader reuses. It
// is valid until the next call; copy it to keep it.
func (r *Reader) Next() (gatling.Record, error) {
	if r.err != nil {
		return gatling.Record{}, r.err
	}

	end, err := r.rd.atEnd()
	if err != nil {
		r.err = err

		return gatling.Record{}, r.err
	}

	if end {
		r.err = io.EOF

		return gatling.Record{}, io.EOF
	}

	rec, err := r.record()
	if err != nil {
		r.err = err

		return gatling.Record{}, err
	}

	return rec, nil
}

// record decodes one record, dispatching on its kind byte.
func (r *Reader) record() (gatling.Record, error) {
	at := r.rd.off

	kind, err := r.rd.u8("a record kind")
	if err != nil {
		return gatling.Record{}, err
	}

	var rec gatling.Record

	switch kind {
	case kindRequest:
		err = r.readRequest(&rec)
	case kindUser:
		err = r.readUser(&rec)
	case kindGroup:
		err = r.readGroup(&rec)
	case kindError:
		err = r.readError(&rec)
	case kindRun:
		// The run record occurs exactly once and opens the log. A second one
		// means the stream desynchronised, and continuing would read every
		// later record against the wrong run start.
		err = r.rd.syntax(at, "a record kind", "a second run record")
	default:
		err = r.rd.syntax(at, "a record kind", describeByte(kind))
	}

	if err != nil {
		return gatling.Record{}, err
	}

	return rec, nil
}
