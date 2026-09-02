package text

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
)

type scannedLine struct {
	no           int
	text         string
	isTerminated bool
}

func scanAll(t *testing.T, r io.Reader) ([]scannedLine, error) {
	t.Helper()

	sc := newScanner(r)

	var lines []scannedLine

	for {
		data, isTerminated, err := sc.next()
		if errors.Is(err, io.EOF) {
			return lines, nil
		}

		if err != nil {
			return lines, err
		}

		lines = append(lines, scannedLine{no: sc.lineNo, text: string(data), isTerminated: isTerminated})
	}
}

func TestScannerLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want []scannedLine
	}{
		{name: "empty", in: "", want: nil},
		{name: "lf", in: "a\nb\n", want: []scannedLine{{no: 1, text: "a", isTerminated: true}, {no: 2, text: "b", isTerminated: true}}},
		{name: "crlf", in: "a\r\nb\r\n", want: []scannedLine{{no: 1, text: "a", isTerminated: true}, {no: 2, text: "b", isTerminated: true}}},
		{name: "mixed", in: "a\r\nb\n", want: []scannedLine{{no: 1, text: "a", isTerminated: true}, {no: 2, text: "b", isTerminated: true}}},
		{name: "unterminated last line", in: "a\nb", want: []scannedLine{{no: 1, text: "a", isTerminated: true}, {no: 2, text: "b", isTerminated: false}}},
		{name: "lone cr inside a line is kept", in: "a\rb\n", want: []scannedLine{{no: 1, text: "a\rb", isTerminated: true}}},
		{name: "empty lines are lines", in: "\n\nx\n", want: []scannedLine{{no: 1, text: "", isTerminated: true}, {no: 2, text: "", isTerminated: true}, {no: 3, text: "x", isTerminated: true}}},
		{name: "tabs untouched", in: "a\t \tb\n", want: []scannedLine{{no: 1, text: "a\t \tb", isTerminated: true}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := scanAll(t, strings.NewReader(tt.in))
			if err != nil {
				t.Fatalf("scan: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("got %d lines %v, want %d lines %v", len(got), got, len(tt.want), tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("line %d: got %+v, want %+v", i+1, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestScannerLineAtLimit(t *testing.T) {
	t.Parallel()

	line := bytes.Repeat([]byte("x"), MaxLineLen)
	in := append(append([]byte{}, line...), '\n')

	got, err := scanAll(t, bytes.NewReader(in))
	if err != nil {
		t.Fatalf("a line of exactly MaxLineLen bytes must scan: %v", err)
	}

	if len(got) != 1 || len(got[0].text) != MaxLineLen || !got[0].isTerminated {
		t.Fatalf("got %d lines, first of %d bytes terminated=%v", len(got), len(got[0].text), got[0].isTerminated)
	}
}

// repeatReader yields n copies of a byte with no newline, without materialising them.
type repeatReader struct {
	b    byte
	left int
}

func (r *repeatReader) Read(p []byte) (int, error) {
	if r.left == 0 {
		return 0, io.EOF
	}

	n := min(len(p), r.left)
	for i := range n {
		p[i] = r.b
	}

	r.left -= n

	return n, nil
}

//nolint:paralleltest // measures heap allocation and must not share the process with other tests
func TestScannerLineOverLimit(t *testing.T) {
	const input = 8 * MaxLineLen

	var before, after runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&before)

	_, err := scanAll(t, &repeatReader{b: 'x', left: input})

	runtime.ReadMemStats(&after)

	var syntaxErr *gatling.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("got %v, want *gatling.SyntaxError", err)
	}

	if syntaxErr.Line != 1 {
		t.Fatalf("error names line %d, want 1", syntaxErr.Line)
	}

	// The scanner may own one buffer of MaxLineLen plus change; it must not have
	// grown with the 8 MiB line it refused.
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 2*MaxLineLen {
		t.Fatalf("scanning an over-long line allocated %d bytes, want at most %d", allocated, 2*MaxLineLen)
	}
}

// TestScannerLineAtLimitCRLF holds the ceiling to the line rather than to the
// line plus whatever terminator wrote it. Gatling ends a record with
// System.lineSeparator, so the same simulation recorded on Windows carries two
// terminator bytes, and a log that reads on one platform must read on the other.
func TestScannerLineAtLimitCRLF(t *testing.T) {
	t.Parallel()

	line := bytes.Repeat([]byte("x"), MaxLineLen)
	in := append(append([]byte{}, line...), '\r', '\n')

	got, err := scanAll(t, bytes.NewReader(in))
	if err != nil {
		t.Fatalf("a line of exactly MaxLineLen bytes must scan with CRLF too: %v", err)
	}

	if len(got) != 1 || len(got[0].text) != MaxLineLen || !got[0].isTerminated {
		t.Fatalf("got %d lines, first of %d bytes terminated=%v", len(got), len(got[0].text), got[0].isTerminated)
	}
}

// TestScannerLineOverLimitByOne fixes the ceiling exactly: one byte past it
// fails, whichever terminator follows.
func TestScannerLineOverLimitByOne(t *testing.T) {
	t.Parallel()

	for _, term := range []struct {
		name string
		b    []byte
	}{{"LF", []byte("\n")}, {"CRLF", []byte("\r\n")}} {
		t.Run(term.name, func(t *testing.T) {
			t.Parallel()

			in := append(bytes.Repeat([]byte("x"), MaxLineLen+1), term.b...)

			_, err := scanAll(t, bytes.NewReader(in))

			var syntaxErr *gatling.SyntaxError
			if !errors.As(err, &syntaxErr) || syntaxErr.Line != 1 {
				t.Fatalf("got %v, want a *gatling.SyntaxError at line 1", err)
			}
		})
	}
}

// TestScannerCallerBufferDoesNotRaiseTheCeiling covers the one way the ceiling
// could be lifted from outside: bufio.NewReaderSize hands back its argument
// when that is already a *bufio.Reader of at least the size asked for, so a
// caller who buffers its own file would set the ceiling to its own buffer.
func TestScannerCallerBufferDoesNotRaiseTheCeiling(t *testing.T) {
	t.Parallel()

	in := append(bytes.Repeat([]byte("x"), 2*MaxLineLen), '\n')

	_, err := scanAll(t, bufio.NewReaderSize(bytes.NewReader(in), 4*MaxLineLen))

	var syntaxErr *gatling.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("a caller's larger buffer raised the ceiling: got %v, want *gatling.SyntaxError", err)
	}
}

// TestScannerUnterminatedLineOverLimit closes the last way past the ceiling: a
// final line with no terminator does not reach the buffer-full path, so it is
// the measurement after the terminator is off that has to catch it.
func TestScannerUnterminatedLineOverLimit(t *testing.T) {
	t.Parallel()

	_, err := scanAll(t, bytes.NewReader(bytes.Repeat([]byte("x"), MaxLineLen+1)))

	var syntaxErr *gatling.SyntaxError
	if !errors.As(err, &syntaxErr) || syntaxErr.Line != 1 {
		t.Fatalf("got %v, want a *gatling.SyntaxError at line 1", err)
	}
}
