package binary

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"

	"github.com/galax-io/parsec/gatling"
)

// MaxStringLen is the ceiling on one string or assertion payload, in bytes. A
// length past it fails the read rather than being allocated: the length is read
// straight from the file, so a single corrupt byte would otherwise ask for
// gigabytes.
//
// No real log approaches it. The longest field is an assertion payload, which
// runs to tens of kilobytes; the longest string is a failure message, which
// Gatling truncates long before this.
const MaxStringLen = 8 << 20

// readBufferSize is the fixed size of the buffer between the caller's reader and
// this one. It never grows, which is what keeps peak memory independent of the
// length of the log.
const readBufferSize = 64 << 10

// reader reads the format's primitives from a stream and tracks where it is.
//
// Every method reports a failure as a *gatling.SyntaxError carrying the offset
// at which the failing value started, so an error names the byte a reader can
// open the file at rather than the byte the decoder happened to stop on.
type reader struct {
	src *bufio.Reader
	// off is the number of bytes consumed, and so the offset of the next one.
	off int64
	// scratch holds the bytes of the value being read. It is reused between
	// values and grows only to what a value needs, never past MaxStringLen.
	scratch []byte
}

func newReader(r io.Reader) *reader {
	return &reader{src: bufio.NewReaderSize(r, readBufferSize)}
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

// truncated describes a read that ran off the end of the stream. io.EOF at the
// very start of a value is still a truncation to every caller but the one
// looking for the end of the log, which checks for it before asking for a value.
func (r *reader) truncated(at int64, expected string) error {
	return r.syntax(at, expected, "end of input")
}

// u8 reads one byte.
func (r *reader) u8(expected string) (byte, error) {
	at := r.off

	b, err := r.src.ReadByte()
	if err != nil {
		return 0, r.truncated(at, expected)
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
// A truncation names the offset the missing bytes should have started at, which
// is where a reader would open the file to see the end of it — not the offset of
// the enclosing value, which may be far behind.
func (r *reader) fixed(n int, expected string) ([]byte, error) {
	at := r.off

	r.grow(n)

	buf := r.scratch[:n]

	if _, err := io.ReadFull(r.src, buf); err != nil {
		return nil, r.truncated(at, expected)
	}

	r.off += int64(n)

	return buf, nil
}

// sized reads a length-prefixed run of bytes: a string's characters, or an
// assertion payload. The bytes are valid until the next read. at is the offset
// of the length prefix, because that is where a complaint about the length
// belongs; a truncation names its own position instead.
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
	case errors.Is(err, io.EOF):
		return true, nil
	default:
		// bufio reports a reader that keeps returning (0, nil) as
		// io.ErrNoProgress rather than spinning, and it surfaces here.
		return false, err
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
