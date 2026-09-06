//go:build integration

package binary_test

import (
	"io"

	"github.com/galax-io/parsec/gatling/binary"
)

// synthLog streams a valid binary log of roughly the requested size without ever
// holding it in memory: a run record, then a repeating block of events that
// refer back to the handful of names introduced once at the start.
//
// The repetition is the point. A run that makes millions of requests against a
// few names is what the format's string cache exists for, and it is the shape
// that proves memory tracks distinct names rather than records.
type synthLog struct {
	target   int64
	produced int64
	buf      []byte
	offset   int32
	isDone   bool
}

func newSynthLog(size int64) *synthLog {
	s := &synthLog{target: size}

	w := &builder{}
	w.runRecord("3.15.1", []string{"synthetic"}, nil)
	// Introduce every name once, in one request, so every later block is
	// nothing but back-references.
	w.u8(1).i32(1).newString("outer").newString("GET /ok").i32(1).i32(2).u8(1).newString("")
	s.buf = w.bytes()

	return s
}

// block writes one round of events, all by reference.
func (s *synthLog) block() {
	s.offset += 10

	w := &builder{}
	// USER start, one request under outer, a group closing, an error, USER end.
	w.u8(2).i32(0).u8(1).i32(s.offset)
	w.u8(1).i32(1).ref(1).ref(2).i32(s.offset).i32(s.offset + 9).u8(1).ref(3)
	w.u8(3).i32(1).ref(1).i32(s.offset).i32(s.offset + 9).i32(9).u8(0)
	w.u8(4).ref(3).i32(s.offset + 9)
	w.u8(2).i32(0).u8(0).i32(s.offset + 9)

	s.buf = append(s.buf, w.bytes()...)
}

func (s *synthLog) Read(p []byte) (int, error) {
	for len(s.buf) < len(p) && !s.isDone {
		if s.produced+int64(len(s.buf)) >= s.target {
			s.isDone = true

			break
		}

		s.block()
	}

	if len(s.buf) == 0 {
		return 0, io.EOF
	}

	n := copy(p, s.buf)
	s.buf = s.buf[n:]
	s.produced += int64(n)

	return n, nil
}

// segment is a run of bytes to hand out: either a literal, or a pattern
// repeated. The pattern form is what lets a field at the string ceiling be
// written without ever holding one in memory.
type segment struct {
	lit     []byte
	pattern []byte
	times   int
}

// ceilingLog streams a log carrying one field at MaxStringLen in each encoding
// the format can store a string in.
//
// It is streamed for the same reason synthLog is, and the reason bites harder
// here: a log materialised in a []byte sits on the heap for the whole decode and
// is counted in every sample, so a measurement made that way reports the input's
// own size back as the peak. At this size that is most of the answer.
type ceilingLog struct {
	segments []segment
	si       int
	done     int
	buf      []byte
}

// The three paths through strings.go, and what each costs on the way out.
//
//	Latin-1, all ASCII        one wire byte per character, copied once
//	Latin-1, bytes >= 0x80    one wire byte per character, two UTF-8 bytes out
//	UTF-16                    two wire bytes per code unit; U+4E2D is the widest
//	                          single-unit case at three UTF-8 bytes out
//
// A field at the ceiling in each is what the peak-memory claim has to hold for,
// and what the synthetic log above never contained: its longest name is seven
// ASCII bytes.
var ceilingFields = []struct {
	name    string
	pattern []byte
	coder   byte
}{
	{"latin-1 ASCII", []byte{'a'}, latin1},
	{"latin-1 above ASCII", []byte{0xE9}, latin1},
	{"utf-16", []byte{0x2D, 0x4E}, utf16b},
}

func newCeilingLog() *ceilingLog {
	s := &ceilingLog{}

	w := &builder{}
	w.runRecord("3.15.1", []string{"ceiling"}, nil)
	s.lit(w.bytes())

	// One request per encoding. Its name is introduced into the cache, and its
	// failure message is the field at the ceiling — which is what that field is
	// for in a real log.
	for i, f := range ceilingFields {
		h := &builder{}
		h.u8(1).i32(0)
		h.i32(length(1 + 2*i)).str(f.name)
		h.i32(10).i32(20).u8(0)
		h.i32(length(2 + 2*i))
		h.i32(binary.MaxStringLen)
		s.lit(h.bytes())

		s.repeat(f.pattern, binary.MaxStringLen/len(f.pattern))
		s.lit([]byte{f.coder})
	}

	return s
}

func (s *ceilingLog) lit(b []byte) { s.segments = append(s.segments, segment{lit: b}) }

func (s *ceilingLog) repeat(pattern []byte, times int) {
	s.segments = append(s.segments, segment{pattern: pattern, times: times})
}

func (s *ceilingLog) Read(p []byte) (int, error) {
	for len(s.buf) == 0 {
		if s.si >= len(s.segments) {
			return 0, io.EOF
		}

		seg := s.segments[s.si]

		switch {
		case seg.lit != nil:
			s.buf, s.si, s.done = seg.lit, s.si+1, 0
		case s.done < seg.times:
			s.buf = seg.pattern
			s.done++
		default:
			s.si++
			s.done = 0
		}
	}

	n := copy(p, s.buf)
	s.buf = s.buf[n:]

	return n, nil
}

// assertionLog streams a run record whose assertion payloads fill
// maxAssertionBytes, each one at the string ceiling.
//
// This is the other ceiling, and it is unbounded in a different way: a payload
// is retained for the whole read and copied again by Assertions, where a string
// field is transient. The synthetic log above declares no assertions at all, so
// this path was as unmeasured as the string one.
func newAssertionLog(payloads, size int) *ceilingLog {
	s := &ceilingLog{}

	w := &builder{}
	w.u8(0).str("3.15.1").str("io.example.Sim").i64(runStart).str("")
	w.i32(1)
	w.str("scenario")
	w.i32(length(payloads))
	s.lit(w.bytes())

	for range payloads {
		h := &builder{}
		h.i32(length(size))
		s.lit(h.bytes())
		s.repeat([]byte{'a'}, size)
	}

	return s
}
