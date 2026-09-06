package binary

import (
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// The encoding markers the format writes after a string's bytes. They are the
// JVM's own compact-string coders: a String holds either one byte per character
// or two, and says which.
const (
	coderLatin1 = 0
	coderUTF16  = 1
)

// str reads one string: a big-endian length, that many bytes, and — only when
// the length is non-zero — a one-byte encoding marker.
//
// The empty string carries no marker. That asymmetry is the writer's, and a
// reader that expects a marker anyway consumes the first byte of the next field
// and desynchronises from there on without ever failing at the string itself.
func (r *reader) str(expected string) (string, error) {
	at := r.off

	n, err := r.i32(expected)
	if err != nil {
		return "", err
	}

	buf, err := r.sized(n, at, expected)
	if err != nil {
		return "", err
	}

	if n == 0 {
		return "", nil
	}

	coder, err := r.u8(expected)
	if err != nil {
		return "", err
	}

	switch coder {
	case coderLatin1:
		return latin1(buf), nil
	case coderUTF16:
		return utf16le(buf, r, at, expected)
	}

	// Guessing here would return mojibake that looks like data, which is worse
	// than a refusal: a wrong name groups two requests together or splits one
	// in two, and the report is then confidently wrong.
	return "", r.syntax(at, expected, "an encoding marker of "+strconv.Itoa(int(coder)))
}

// latin1 decodes the JVM's one-byte-per-character form. Every byte is the code
// point of the same value, so bytes below 0x80 are already UTF-8 and the common
// case copies rather than converts.
func latin1(b []byte) string {
	ascii := true

	for _, c := range b {
		if c >= utf8.RuneSelf {
			ascii = false

			break
		}
	}

	if ascii {
		return string(b)
	}

	out := make([]rune, len(b))
	for i, c := range b {
		out[i] = rune(c)
	}

	return string(out)
}

// utf16le decodes the JVM's two-bytes-per-character form.
//
// The byte order is the writing JVM's native one and the file records nothing
// about it. Little-endian is assumed and documented; no corpus this project can
// record proves it, because every machine one could be recorded on is
// little-endian.
//
// An unpaired surrogate becomes U+FFFD, which is what utf16.Decode does and what
// the JVM itself permits a String to hold: refusing would reject a name the
// writer legitimately wrote.
func utf16le(b []byte, r *reader, at int64, expected string) (string, error) {
	if len(b)%2 != 0 {
		return "", r.syntax(at, expected, "a UTF-16 string of an odd number of bytes")
	}

	units := make([]uint16, len(b)/2)
	for i := range units {
		units[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}

	return string(utf16.Decode(units)), nil
}

// cache rebuilds the table of strings the writer built, in the writer's order.
//
// The writer stores each distinct string once and refers back to it afterwards,
// which is why memory here is bounded by the number of distinct strings a
// simulation declares rather than by the number of records it writes. It is also
// why the stream must be read from its first byte: the table cannot be rebuilt
// from the middle.
type cache struct {
	entries []string
}

// read reads one cached string: a big-endian index, then — when the index is
// positive — the string being introduced.
//
// A positive index introduces the entry it names; a negative one refers to the
// entry at its absolute value. Indices start at 1, because a hit is written as
// the negation of an index and zero has no negation.
//
// An index that is not the next expected one is malformed. The writer's counter
// is strictly sequential, so a gap means the stream desynchronised earlier, and
// accepting it would rename every record that follows rather than failing here.
func (c *cache) read(r *reader, expected string) (string, error) {
	at := r.off

	i, err := r.i32(expected)
	if err != nil {
		return "", err
	}

	if i > 0 {
		if int(i) != len(c.entries)+1 {
			return "", r.syntax(at, expected, "cache entry "+strconv.Itoa(int(i))+
				", where entry "+strconv.Itoa(len(c.entries)+1)+" comes next")
		}

		s, err := r.str(expected)
		if err != nil {
			return "", err
		}

		c.entries = append(c.entries, s)

		return s, nil
	}

	if i == 0 {
		return "", r.syntax(at, expected, "cache index 0, which the format never writes")
	}

	k := int(-i)
	if k > len(c.entries) {
		return "", r.syntax(at, expected, "a reference to cache entry "+strconv.Itoa(k)+
			", where only "+strconv.Itoa(len(c.entries))+" have been introduced")
	}

	return c.entries[k-1], nil
}
