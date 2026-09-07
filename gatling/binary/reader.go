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
// of distinct strings the log introduces — not by the number of records. The
// string table is capped, so a log whose every failure message differs cannot
// make that table follow the record count.
//
// # The budget
//
// Peak heap stays under 32 MiB for any log this codec accepts, and that figure
// is what the package's own tests assert. It holds for a multi-gigabyte log — a
// 2.5 GB log of 174 million records decodes with a peak under 4 MiB — and it
// holds for the shape that costs most per record: a field at [MaxStringLen] in
// each of the three encodings the format can store a string in, which measures
// at roughly five and a half times the ceiling because the garbage from one
// field is not collected before the next is built.
//
// The two halves of that sentence are why [MaxStringLen] is what it is. A
// ceiling that let one field cost more than the budget would make the budget
// false for a log no larger than a few megabytes, which is the failure this
// documentation exists to rule out.
//
// A read ends in one of three ways, and each ending is terminal — every later
// Next returns the same value.
//
//   - io.EOF is a clean end: the log ended at a record boundary and every record
//     it held was delivered.
//   - A *gatling.TruncationError is a log cut short: the bytes ran out inside a
//     record, which is how a run killed mid-flight ends. Every record before the
//     cut was delivered and is exactly what an intact log would have given.
//   - Any other error is a failed read — a *gatling.SyntaxError naming the byte
//     that could not be decoded, or a failure of the source. Nothing may be
//     derived from what was delivered.
//
// The format carries no end marker, so a log cut exactly on a record boundary is
// a shorter valid log and ends cleanly. Only something that can see the writer
// can tell that from a complete one.
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
// *gatling.UnverifiedError instead.//
// The source may still be being written. A Read that blocks is a wait, not an
// end, and a record split across reads is delivered once, when its last byte
// arrives; the [github.com/galax-io/parsec/gatling/simlog] package documentation
// states the contract in full, and it holds for this constructor too.
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
// Any other error ends the read: there is no next record after it, and the same
// error is returned on every later call. A *gatling.TruncationError says the log
// was cut short and that the records already delivered are what it recorded;
// anything else says the read failed and that they are not a result. See
// [Reader] for the three endings in full.
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

	// Where this record begins, so a read that runs out inside it can say where
	// to open the file and how much was lost.
	r.rd.recordAt = r.rd.off

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
