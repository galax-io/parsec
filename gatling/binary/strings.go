package binary

import (
	"strconv"
	"strings"
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
//
// The conversion is sized exactly and built once. Going through a []rune would
// reserve four bytes per input byte and then copy the result again, so a field
// at MaxStringLen would allocate several times the ceiling that constant exists
// to enforce.
func latin1(b []byte) string {
	high := 0

	for _, c := range b {
		if c >= utf8.RuneSelf {
			high++
		}
	}

	if high == 0 {
		return string(b)
	}

	// Every byte above 0x7f is two bytes of UTF-8; every other byte is one.
	var out strings.Builder

	out.Grow(len(b) + high)

	for _, c := range b {
		out.WriteRune(rune(c))
	}

	return out.String()
}

// utf16le decodes the JVM's two-bytes-per-character form.
//
// The byte order is the writing JVM's native one and the file records nothing
// about it. Little-endian is assumed and documented; no corpus this project can
// record proves it, because every machine one could be recorded on is
// little-endian.
//
// An unpaired surrogate is refused rather than replaced. utf16.Decode would
// substitute U+FFFD, and a name that decodes to a different name is worse than a
// refusal: it groups two requests together or splits one in two, and the report
// is then confidently wrong. FR-004 asks for exactly this.
//
// Two passes: the first validates and measures, the second builds. That way the
// result is allocated once, at its exact size.
func utf16le(b []byte, r *reader, at int64, expected string) (string, error) {
	if len(b)%2 != 0 {
		return "", r.syntax(at, expected, "a UTF-16 string of an odd number of bytes")
	}

	size := 0

	for i := 0; i < len(b); i += 2 {
		u, pair := unit(b, i)

		switch {
		case !utf16.IsSurrogate(rune(u)):
			size += utf8.RuneLen(rune(u))
		case pair == 0:
			return "", r.syntax(at, expected, "an unpaired UTF-16 surrogate")
		default:
			size += utf8.RuneLen(utf16.DecodeRune(rune(u), rune(pair)))
			i += 2
		}
	}

	var out strings.Builder

	out.Grow(size)

	for i := 0; i < len(b); i += 2 {
		u, pair := unit(b, i)

		if !utf16.IsSurrogate(rune(u)) {
			out.WriteRune(rune(u))

			continue
		}

		out.WriteRune(utf16.DecodeRune(rune(u), rune(pair)))

		i += 2
	}

	return out.String(), nil
}

// unit reads the code unit at i and, when it is a high surrogate followed by a
// low one, its partner. A surrogate with no valid partner comes back with a zero
// pair, which is not a code unit the format can write beside one.
func unit(b []byte, i int) (u, pair uint16) {
	u = uint16(b[i]) | uint16(b[i+1])<<8
	if !utf16.IsSurrogate(rune(u)) || i+3 >= len(b) {
		return u, 0
	}

	lo := uint16(b[i+2]) | uint16(b[i+3])<<8
	if utf16.DecodeRune(rune(u), rune(lo)) == utf8.RuneError {
		return u, 0
	}

	return u, lo
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

// maxCacheEntries bounds the table. Memory here is bounded by the number of
// *distinct* strings, and for names and group paths a simulation declares a
// fixed handful — but failure messages also go through the table, and Gatling
// builds those from exception text that embeds addresses, ports and status
// lines. A run that fails in a new way on every request would otherwise grow a
// table linear in the record count, which is the one way this reader's memory
// could follow the length of the log.
//
// A simulation with more than this many distinct strings is not something
// Gatling produces; a log that claims one is damaged.
const maxCacheEntries = 1 << 20

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

		if len(c.entries) >= maxCacheEntries {
			return "", r.syntax(at, expected, "more than "+strconv.Itoa(maxCacheEntries)+
				" distinct strings, which no simulation declares")
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

	// Negated in int64. Inside int32, math.MinInt32 negates to itself and stays
	// negative, so a ">" bounds check would pass it straight through to an index.
	k := -int64(i)
	if k > int64(len(c.entries)) {
		return "", r.syntax(at, expected, "a reference to cache entry "+strconv.FormatInt(k, 10)+
			", where only "+strconv.Itoa(len(c.entries))+" have been introduced")
	}

	return c.entries[k-1], nil
}
