package binary_test

import (
	"encoding/binary"
	"unicode/utf16"
)

// builder writes the binary format by hand, so a test can produce an input no
// recording contains: a version outside the supported range, a truncation at a
// chosen byte, a cache reference that was never introduced.
//
// It is a fixture generator, not a corpus. Nothing decoded from it is compared
// against a report, and no claim about what Gatling writes rests on it — the
// recordings are what say that. This exists for the cases a real run cannot
// produce on demand.
type builder struct {
	b     []byte
	cache int32
}

// The encoding markers, repeated here rather than shared: this file writes what
// the format says, and a test that imported the decoder's own constant would
// agree with it by construction.
const (
	latin1 = 0
	utf16b = 1
)

func (w *builder) u8(v byte) *builder { w.b = append(w.b, v); return w }

func (w *builder) i32(v int32) *builder {
	w.b = binary.BigEndian.AppendUint32(w.b, uint32(v)) //nolint:gosec // the format's own signed field

	return w
}

func (w *builder) i64(v int64) *builder {
	w.b = binary.BigEndian.AppendUint64(w.b, uint64(v)) //nolint:gosec // the format's own signed field

	return w
}

// str writes a string the way the writer does: a length, the bytes, and a coder
// byte — except for the empty string, which carries no coder.
//
// The coder is chosen the way the JVM chooses it: Latin-1 whenever every
// character is below U+0100, not merely when every character is ASCII. A builder
// that reached for UTF-16 at the first byte above 0x7f could never produce the
// Latin-1 high-byte path, which is the one a run named `GET /café` takes.
func (w *builder) str(s string) *builder {
	if s == "" {
		return w.i32(0)
	}

	if latin1able(s) {
		b := make([]byte, 0, len(s))
		for _, r := range s {
			b = append(b, byte(r&0xff))
		}

		w.i32(length(len(b)))
		w.b = append(w.b, b...)

		return w.u8(latin1)
	}

	units := utf16.Encode([]rune(s))
	w.i32(length(len(units) * 2))

	for _, u := range units {
		w.b = append(w.b, byte(u&0xff), byte(u>>8))
	}

	return w.u8(utf16b)
}

// newString introduces a cache entry and writes it.
func (w *builder) newString(s string) *builder {
	w.cache++

	return w.i32(w.cache).str(s)
}

// ref refers back to an entry already introduced.
func (w *builder) ref(index int32) *builder { return w.i32(-index) }

// latin1able reports whether the JVM would store s as one byte per character.
func latin1able(s string) bool {
	for _, r := range s {
		if r > 0xff {
			return false
		}
	}

	return true
}

// runRecord writes a complete run record naming the given version.
func (w *builder) runRecord(version string, scenarios, assertions []string) *builder {
	w.u8(0).str(version).str("io.example.Sim").i64(runStart).str("")

	w.i32(length(len(scenarios)))

	for _, s := range scenarios {
		w.str(s)
	}

	w.i32(length(len(assertions)))

	for _, a := range assertions {
		w.i32(length(len(a)))
		w.b = append(w.b, a...)
	}

	return w
}

// runStart is the epoch millisecond every built log claims, so a test comparing
// timestamps has a fixed anchor.
const runStart int64 = 1788670094356

// request writes a REQUEST record with no groups, introducing its name and
// message the first time and referring back afterwards.
func (w *builder) request(name string, ok bool) *builder {
	w.u8(1).i32(0).newString(name).i32(10).i32(20)

	if ok {
		w.u8(1)
	} else {
		w.u8(0)
	}

	return w.newString("")
}

func (w *builder) bytes() []byte { return w.b }

// minimal is the smallest log that reaches the version gate: a run record and
// nothing after it.
func minimal(version string) []byte {
	return (&builder{}).runRecord(version, []string{"scenario"}, nil).bytes()
}

// length narrows a fixture's own size to the format's signed 32-bit field. Every
// caller passes the length of something this file just built, so the conversion
// cannot overflow; it is in one place so the reason is written once.
func length(n int) int32 { return int32(n) } //nolint:gosec // a fixture's own size
