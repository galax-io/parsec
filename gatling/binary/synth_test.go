//go:build integration

package binary_test

import "io"

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
