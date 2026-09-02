package text

import (
	"bufio"
	"errors"
	"io"
	"strconv"

	"github.com/galax-io/parsec/gatling"
)

// MaxLineLen is the ceiling on one line, in bytes. A longer line fails the read
// rather than being held in memory. No valid log approaches it: the longest
// realistic line is an assertion payload, which runs to tens of kilobytes.
const MaxLineLen = 1 << 20

// scanner yields one line at a time from a stream through a buffer of fixed
// size, so peak memory does not grow with the log. The slice it returns is
// valid only until the next call.
type scanner struct {
	r *bufio.Reader
	// lineNo is the 1-based number of the line most recently returned.
	lineNo int
}

func newScanner(r io.Reader) *scanner {
	// Two bytes more than the ceiling, so a line of exactly MaxLineLen bytes
	// fits with either terminator: "\n", or the "\r\n" a run on Windows writes.
	// The buffer alone cannot state the rule, because it bounds the line and
	// its terminator together, so next also measures the line once the
	// terminator is off.
	//
	// r is wrapped in a plain io.Reader first. bufio.NewReaderSize hands back
	// its argument when that is already a *bufio.Reader of at least the size
	// asked for, so a caller who buffers its own file would otherwise raise
	// the ceiling to its own buffer size without knowing it.
	return &scanner{r: bufio.NewReaderSize(struct{ io.Reader }{r}, MaxLineLen+2)}
}

// tooLong is the one error for a line past the ceiling, wherever it is caught.
func (s *scanner) tooLong() error {
	return &gatling.SyntaxError{
		Line:     s.lineNo,
		Expected: "a line of at most " + strconv.Itoa(MaxLineLen) + " bytes",
		Found:    "a longer line",
	}
}

// next returns the next line without its terminator and whether it had one. A
// single '\r' before the '\n' is stripped. It returns io.EOF once the input is
// exhausted and a *gatling.SyntaxError for a line longer than MaxLineLen.
func (s *scanner) next() ([]byte, bool, error) {
	data, err := s.r.ReadSlice('\n')

	switch {
	case err == nil:
		s.lineNo++

		data = data[:len(data)-1]
		if n := len(data); n > 0 && data[n-1] == '\r' {
			data = data[:n-1]
		}

		if len(data) > MaxLineLen {
			return nil, false, s.tooLong()
		}

		return data, true, nil

	case errors.Is(err, bufio.ErrBufferFull):
		s.lineNo++

		return nil, false, s.tooLong()

	case errors.Is(err, io.EOF):
		if len(data) == 0 {
			return nil, false, io.EOF
		}

		s.lineNo++

		if len(data) > MaxLineLen {
			return nil, false, s.tooLong()
		}

		return data, false, nil

	default:
		return nil, false, err
	}
}
