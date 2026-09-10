package binary

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/galax-io/parsec/gatling"
)

// MaxStringLen is the ceiling on one string or assertion payload, in bytes. A
// length past it fails the read rather than being allocated: the length is read
// straight from the file, so a single corrupt byte would otherwise ask for
// gigabytes.
//
// No real log approaches it. The longest field is an assertion payload, which
// runs to tens of kilobytes — the ten in the 3.14.9 recording are between 15 and
// 51 bytes; the longest string is a failure message, which Gatling truncates
// long before this.
//
// It bounds the bytes read from the file, not the peak while one field is
// decoded. Converting a string allocates the read buffer and then the result, a
// Latin-1 field above ASCII doubles on the way out, and a UTF-16 field of
// three-byte code points grows by half, so one field at the ceiling costs two to
// three times it and a log carrying one of each costs the sum — measured at
// about five and a half times the ceiling, because the garbage from one field is
// not collected before the next is built.
//
// That measurement is what fixes the value. At 8 MiB, which this was until
// v0.0.7, three such fields peaked at 44 MiB and made the 32 MiB budget
// [Reader] documents false for a log of 24 MiB. At 1 MiB the same shape peaks
// under 6 MiB. Refusing a corrupt length prefix — the failure this constant
// exists for — is unaffected: a prefix claiming gigabytes is refused as flatly
// by one ceiling as by the other.
const MaxStringLen = 1 << 20

// readBufferSize is the fixed size of the buffer between the caller's reader and
// this one. It never grows, which is what keeps peak memory independent of the
// length of the log.
const readBufferSize = 64 << 10

// reader reads the format's primitives from a stream and tracks where it is.
//
// A malformed value fails with a *gatling.SyntaxError carrying the offset at
// which that value started. A stream that simply ran out inside a record fails
// with a *gatling.TruncationError instead, described from the record's start —
// see truncated. A failure of the source is neither, and keeps its own cause.
type reader struct {
	src *bufio.Reader
	// off is the number of bytes consumed, and so the offset of the next one.
	off int64
	// recordAt is the offset the record being decoded began at, set by the
	// record loop before each record. A truncation is described from here: it is
	// where a reader would open the file to see what was lost, and the distance
	// from here to where the stream stopped is what was dropped.
	//
	// It stays 0 until the loop sets it, which is right for the one record read
	// before the loop exists: the run record, which begins at byte 0.
	recordAt int64
	// scratch holds the bytes of the value being read. It is reused between
	// values and grows only to what a value needs, never past MaxStringLen.
	scratch []byte
}

func newReader(r io.Reader) *reader {
	// r is wrapped in a plain io.Reader first. bufio.NewReaderSize hands back
	// its argument when that is already a *bufio.Reader of at least the size
	// asked for, so a caller who buffers its own file would otherwise raise
	// the ceiling to its own buffer size without knowing it — the same reason
	// gatling/text/scan.go wraps before it buffers.
	return &reader{src: bufio.NewReaderSize(struct{ io.Reader }{r}, readBufferSize)}
}

// maxEmptyReads is how many times a stalled source is given the benefit of the
// doubt before the read ends. It is bufio's own figure, and simlog.readHead's,
// for the same reason: a Read returning (0, nil) is legal and must not be taken
// for the end of the stream, but a source that only ever does that has wedged
// the caller with nothing to cancel.
const maxEmptyReads = 100

// errCutShort is what readFull returns when the stream ended part-way through a
// value. It exists so that this package's own conversion of an end of stream can
// be told from io.ErrUnexpectedEOF, which compress/gzip, flate and zlib all
// return by identity when their *compressed* input was cut: those are failures
// of the source, and reporting one as a Gatling log cut short would name offsets
// in decompressed coordinates that match no byte of the file on disk.
var errCutShort = errors.New("the stream ended inside a value")

// readFull fills buf, and differs from io.ReadFull in the two ways this package
// needs. An end of stream becomes errCutShort rather than io.EOF or
// io.ErrUnexpectedEOF, so only this loop can claim the log ran out; and a source
// that keeps returning (0, nil) ends the read with io.ErrNoProgress rather than
// spinning. io.ReadFull loops on an empty read forever, and bufio's own guard
// lives in fill(), which its Read does not use — so nothing below this line
// would have caught it.
func readFull(r io.Reader, buf []byte) (int, error) {
	n, empty := 0, 0

	for n < len(buf) {
		read, err := r.Read(buf[n:])
		n += read

		switch {
		// Identity: only the stream itself ending, never a source wrapping io.EOF.
		case err == io.EOF:
			return n, errCutShort

		case err != nil:
			return n, err

		case read > 0:
			empty = 0

		default:
			if empty++; empty >= maxEmptyReads {
				return n, io.ErrNoProgress
			}
		}
	}

	return n, nil
}

// syntax builds the error for a value that started at the given offset.
func (r *reader) syntax(at int64, expected, found string) error {
	return &gatling.SyntaxError{
		Offset:   at,
		Format:   gatling.FormatBinary,
		Expected: expected,
		Found:    found,
	}
}

// truncated describes a read that ran off the end of the stream: the log was cut
// short inside a record, which is how every killed run ends, and not damaged.
// at is where the stream stopped.
//
// It is described from the record's start rather than from where the stream
// stopped, because that is the byte a reader would open the file at to see what
// was lost; the stop offset is Offset+Dropped and is not thrown away.
//
// A stream that stops without a byte of the record arriving is not a cut record.
// Inside the record loop it cannot happen — atEnd looks for the end of the log
// before a record is asked for — so this is the empty stream handed straight to
// NewReader, which has no partial record to report and stays a syntax error.
// expected is what the record still needed, not what sits at the offset the
// error names: the offset is pinned to the record's start while the value being
// read moves through it. gatling.TruncationError.Error says it that way round.
func (r *reader) truncated(at int64, expected string) error {
	dropped := at - r.recordAt
	if dropped <= 0 {
		return r.syntax(at, expected, "end of input")
	}

	return &gatling.TruncationError{
		Format:   gatling.FormatBinary,
		Offset:   r.recordAt,
		Expected: expected,
		Dropped:  dropped,
	}
}

// sourceFailed reports a stream that broke rather than ended. The cause is kept,
// because a caller told its log is truncated will re-record a run that was never
// corrupt: a reset connection and a short file are different problems and only
// one of them is about the file.
//
// Two endings alone become a truncation, and both are this package's own: io.EOF
// straight from bufio at the top of a value, and errCutShort, which readFull
// raises when the stream ran out inside one. Compared with == and not errors.Is.
// An error that merely *wraps* io.EOF is the source reporting a failure of its
// own, and bufio hands it through unchanged; io.ErrUnexpectedEOF is worse still,
// because a truncated gzip, flate or zlib stream returns that sentinel by
// identity. Neither says anything about the Gatling log.
// reader.atEnd, scanner.next, simlog.identify and simlog.readHead hold the same
// rule.
func (r *reader) sourceFailed(at int64, expected string, err error) error {
	if err == io.EOF || errors.Is(err, errCutShort) { //nolint:errorlint // deliberate: identity, not wrapping; see above
		return r.truncated(at, expected)
	}

	// A cause that itself ends in io.EOF cannot be wrapped. errors.Is would then
	// match io.EOF on this failure, and every caller whose loop breaks on the
	// clean end of a log — including this module's own — would read a broken
	// transport as a complete run. The text is kept; the chain is not.
	if errors.Is(err, io.EOF) {
		return fmt.Errorf("gatling: byte %d: reading %s: %s", at, expected, err.Error())
	}

	return fmt.Errorf("gatling: byte %d: reading %s: %w", at, expected, err)
}

// u8 reads one byte.
func (r *reader) u8(expected string) (byte, error) {
	at := r.off

	b, err := r.src.ReadByte()
	if err != nil {
		return 0, r.sourceFailed(at, expected, err)
	}

	r.off++

	return b, nil
}

// boolean reads the format's one-byte boolean. Any value but 0 or 1 is
// malformed: the writer emits only those two, so a third means the stream
// desynchronised earlier and every later record would be read from the wrong
// place.
func (r *reader) boolean(expected string) (bool, error) {
	at := r.off

	b, err := r.u8(expected)
	if err != nil {
		return false, err
	}

	switch b {
	case 0:
		return false, nil
	case 1:
		return true, nil
	}

	return false, r.syntax(at, expected, describeByte(b))
}

// i32 reads a big-endian signed 32-bit integer: every count, offset, length and
// cache index in the format is one.
func (r *reader) i32(expected string) (int32, error) {
	buf, err := r.fixed(4, expected)
	if err != nil {
		return 0, err
	}

	return int32(binary.BigEndian.Uint32(buf)), nil //nolint:gosec // the format's own signed 32-bit field
}

// i64 reads a big-endian signed 64-bit integer. The run's start is the only one.
func (r *reader) i64(expected string) (int64, error) {
	buf, err := r.fixed(8, expected)
	if err != nil {
		return 0, err
	}

	return int64(binary.BigEndian.Uint64(buf)), nil //nolint:gosec // the format's own signed 64-bit field
}

// fixed reads exactly n bytes, where n is either fixed by the format or a
// length sized has already capped. The bytes are valid until the next read.
//
// The offset it hands sourceFailed is where the stream actually stopped. A
// truncation is not reported from there: it names the record's start, and keeps
// this position as the far end of what was dropped (see truncated).
func (r *reader) fixed(n int, expected string) ([]byte, error) {
	at := r.off

	r.grow(n)

	buf := r.scratch[:n]

	// readFull's count is what says where the stream actually stopped, which is
	// Offset+Dropped on the truncation this produces.
	read, err := readFull(r.src, buf)
	if err != nil {
		return nil, r.sourceFailed(at+int64(read), expected, err)
	}

	r.off += int64(n)

	return buf, nil
}

// sized reads a length-prefixed run of bytes: a string's characters, or an
// assertion payload. The bytes are valid until the next read. at is the offset
// of the length prefix, because that is where a complaint about the length
// belongs; a truncation names the record's start instead.
//
// The length comes from the file and is therefore untrusted. It is checked
// against MaxStringLen before anything is allocated, so a corrupt prefix fails
// the read instead of asking the allocator for what it claims.
func (r *reader) sized(n int32, at int64, expected string) ([]byte, error) {
	switch {
	case n < 0:
		return nil, r.syntax(at, expected, "a negative length")
	case n > MaxStringLen:
		return nil, r.syntax(at, expected, "a length past the maximum this codec will allocate")
	case n == 0:
		return nil, nil
	}

	return r.fixed(int(n), expected)
}

// grow makes scratch able to hold n bytes. It is only ever called with a length
// that sized has already capped, or with one the format fixes.
func (r *reader) grow(n int) {
	if cap(r.scratch) < n {
		r.scratch = make([]byte, n)
	}

	r.scratch = r.scratch[:cap(r.scratch)]
}

// atEnd reports whether the stream is exhausted, without consuming anything. It
// is how the record loop tells a clean end from a truncated record: at the top
// of a record there is nothing to read, and anywhere inside one there must be.
func (r *reader) atEnd() (bool, error) {
	_, err := r.src.Peek(1)

	switch {
	case err == nil:
		return false, nil
	case errors.Is(err, io.EOF) && err == io.EOF: //nolint:err113 // identity is the point; see below
		return true, nil
	default:
		// Compared with == and not errors.Is. A source that fails with an error
		// merely *wrapping* io.EOF — a truncated decompressor, a closed
		// transport — would otherwise be read as the clean end of the log, and
		// a partial run reported as complete. gatling/simlog holds the same
		// rule for the same reason.
		//
		// bufio also reports a reader that keeps returning (0, nil) as
		// io.ErrNoProgress rather than spinning, and it surfaces here.
		//
		// Handed to sourceFailed rather than returned raw, so that a cause
		// ending in io.EOF is stripped of its chain here too: returning it
		// unchanged would leave errors.Is(err, io.EOF) true and put the caller
		// back where this switch started.
		return false, r.sourceFailed(r.off, "the next record", err)
	}
}

// describeByte names a byte the way an error message should: printable ASCII as
// itself, anything else in hex.
func describeByte(b byte) string {
	if b >= 0x20 && b < 0x7f {
		return "byte " + string(rune(b))
	}

	return "byte " + hex(b)
}

func hex(b byte) string {
	const digits = "0123456789abcdef"

	return "0x" + string([]byte{digits[b>>4], digits[b&0xf]})
}
