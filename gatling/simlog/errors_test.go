package simlog_test

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
	"github.com/galax-io/parsec/gatling/simlog"
	"github.com/galax-io/parsec/gatling/text"
)

// textLog builds the smallest complete text log: a run header naming a version,
// and nothing after it. Enough to reach the version gate, which is all these
// tests need.
func textLog(version string) string {
	return "RUN\tio.example.Sim\tsim\t1788379664977\t \t" + version + "\n"
}

var errStream = errors.New("the stream broke")

// The whole milestone is here. A binary log handed to the text codec is refused;
// handed to this package it is read.
//
// The text half is kept: it records what a consumer gets without this package,
// and it is what makes the other half mean something. What it records changed
// with #84 — the refusal used to be a *gatling.SyntaxError, which says the log
// is damaged and sent a user looking for corruption that is not there. It now
// names the format found and the package that reads it, which is a better
// failure and still a failure: the one-call answer is still this package.
func TestBinaryIsReadWhereTheTextCodecOnlyFails(t *testing.T) {
	t.Parallel()

	t.Run("through the text codec, the wrong format", func(t *testing.T) {
		t.Parallel()

		_, err := text.NewReader(open(t, binaryLog()))

		var unsupported *gatling.UnsupportedFormatError
		if !errors.As(err, &unsupported) {
			t.Fatalf("text.NewReader on a binary log = %v; want a *gatling.UnsupportedFormatError — "+
				"this half of the test records the outcome the milestone exists to replace", err)
		}

		if unsupported.Format != gatling.FormatBinary {
			t.Errorf("Format = %v, want %v", unsupported.Format, gatling.FormatBinary)
		}

		if errors.As(err, new(*gatling.SyntaxError)) {
			t.Error("the refusal still reads as a damaged log")
		}
	})

	t.Run("through simlog, records", func(t *testing.T) {
		t.Parallel()

		rd, err := simlog.NewReader(open(t, binaryLog()))
		if err != nil {
			t.Fatalf("simlog.NewReader on a binary log = %v; a codec reads one now", err)
		}

		if rd == nil {
			t.Fatal("a successful read must hand back a reader")
		}

		if got := rd.Header().Version.String(); got != "3.15.1" {
			t.Fatalf("the header names version %s; the recording is 3.15.1", got)
		}

		rec, err := rd.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}

		if rec.Kind == gatling.KindUnknown {
			t.Fatal("the first record decoded to no kind at all")
		}
	})
}

// A binary log opened through the detecting entry point must be the same log
// opened through the codec: same records, field for field. A dispatch that
// quietly passed different options, or wrapped the stream, would show up here
// and nowhere else.
func TestDispatchIsTheCodecItself(t *testing.T) {
	t.Parallel()

	direct, err := binary.NewReader(open(t, binaryLog()))
	if err != nil {
		t.Fatalf("binary.NewReader: %v", err)
	}

	through, err := simlog.NewReader(open(t, binaryLog()))
	if err != nil {
		t.Fatalf("simlog.NewReader: %v", err)
	}

	if direct.Header() != through.Header() {
		t.Fatalf("headers differ:\n direct  %+v\n through %+v", direct.Header(), through.Header())
	}

	for n := 1; ; n++ {
		a, aErr := direct.Next()
		b, bErr := through.Next()

		if (aErr == nil) != (bErr == nil) {
			t.Fatalf("record %d: direct %v, through %v", n, aErr, bErr)
		}

		if aErr != nil {
			if !errors.Is(aErr, io.EOF) {
				t.Fatalf("record %d: %v", n, aErr)
			}

			if n < 100 {
				t.Fatalf("the stream ended after %d records; the recording holds 132", n-1)
			}

			return
		}

		if !slices.Equal(a.Groups, b.Groups) {
			t.Fatalf("record %d: group paths differ: %q and %q", n, a.Groups, b.Groups)
		}

		a.Groups, b.Groups = nil, nil
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("record %d differs:\n direct  %+v\n through %+v", n, a, b)
		}
	}
}

// Every way a read can be refused, and the type that says which. No test here
// matches on message text: a caller branches on the type or it branches on
// prose that is free to change.
func TestRefusalsAreTellableApart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		opts []gatling.Option
		want func(error) bool
	}{
		{
			name: "not a Gatling log",
			body: "just some notes about the run\n",
			want: func(err error) bool { return errors.As(err, new(*gatling.FormatError)) },
		},
		{
			name: "too short to tell",
			body: "RU",
			want: func(err error) bool {
				var e *gatling.FormatError

				return errors.As(err, &e) && e.Short
			},
		},
		{
			name: "a version older than any recording",
			body: textLog("3.9.0"),
			want: func(err error) bool { return errors.As(err, new(*gatling.VersionError)) },
		},
		{
			name: "a version newer than any recording, read strictly",
			body: textLog("3.13.0"),
			opts: []gatling.Option{gatling.WithStrict()},
			want: func(err error) bool { return errors.As(err, new(*gatling.UnverifiedError)) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := simlog.NewReader(strings.NewReader(tt.body), tt.opts...)
			if err == nil {
				t.Fatal("want a refusal, got none")
			}

			if !tt.want(err) {
				t.Fatalf("err = %v (%T); not the type this case is about", err, err)
			}
		})
	}
}

// An interface holding a nil pointer is not nil. Returning one would make every
// caller's `if rd != nil` pass and the next call panic, which is the single
// most likely way this package could go wrong.
func TestNoTypedNilOnAnyErrorPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		reader func() io.Reader
		opts   []gatling.Option
	}{
		{name: "unknown format", reader: func() io.Reader { return strings.NewReader("not a log\n") }},
		{name: "too short", reader: func() io.Reader { return strings.NewReader("RU") }},
		{name: "empty", reader: func() io.Reader { return strings.NewReader("") }},
		{name: "version below the range", reader: func() io.Reader { return strings.NewReader(textLog("3.9.0")) }},
		{
			name:   "strict refusal above the range",
			reader: func() io.Reader { return strings.NewReader(textLog("3.13.0")) },
			opts:   []gatling.Option{gatling.WithStrict()},
		},
		{name: "a stream that fails", reader: func() io.Reader { return iotest.ErrReader(errStream) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rd, err := simlog.NewReader(tc.reader(), tc.opts...)
			if err == nil {
				t.Fatal("want a refusal, got none")
			}

			if rd != nil {
				t.Fatalf("NewReader returned a non-nil reader (%T) beside an error; "+
					"a typed nil in an interface defeats every nil check a caller writes", rd)
			}

			run, err := simlog.NewRunReader(tc.reader(), tc.opts...)
			if err == nil {
				t.Fatal("want a refusal from NewRunReader, got none")
			}

			if run != nil {
				t.Fatalf("NewRunReader returned a non-nil reader (%T) beside an error", run)
			}
		})
	}
}

// A stream that breaks while the format is being read is a broken stream, not
// an unrecognised format. Reporting it as the latter would send a user looking
// at the file instead of at the pipe.
func TestFailingStreamIsNotAMisclassification(t *testing.T) {
	t.Parallel()

	_, err := simlog.NewReader(iotest.ErrReader(errStream))
	if !errors.Is(err, errStream) {
		t.Fatalf("NewReader = _, %v; want the stream's own error wrapped", err)
	}

	if errors.As(err, new(*gatling.FormatError)) {
		t.Fatal("a failing stream must not be reported as bytes that are not a Gatling log")
	}
}

// A Read returning (0, nil) is legal — the io.Reader contract says so, and says
// it must not be taken for EOF. io.ReadFull loops on it forever, so a reader
// that stalls once used to wedge the constructor with no error and nothing to
// cancel, where the codec's own path returns promptly.
func TestStalledReaderDoesNotWedgeTheConstructor(t *testing.T) {
	t.Parallel()

	done := make(chan error, 1)

	go func() {
		_, err := simlog.NewReader(stalledReader{})
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a reader that never delivers a byte must not succeed")
		}

		if errors.As(err, new(*gatling.FormatError)) {
			t.Fatalf("a stalled stream is not a bad format: %v", err)
		}

	case <-time.After(5 * time.Second):
		t.Fatal("NewReader did not return: io.ReadFull spins on a reader that makes no progress")
	}
}

type stalledReader struct{}

func (stalledReader) Read([]byte) (int, error) { return 0, nil }

// Identification consumes bytes it cannot put back, so a refusal has to hand
// them over. A caller holding a pipe, a response body or an archive entry
// cannot rewind, and spooling the log aside for a later version of this module
// would otherwise write a file missing its own header.
func TestRefusalHandsBackTheBytesItRead(t *testing.T) {
	t.Parallel()

	// Neither format: not tab-separated text, and not a run record.
	body := "PK\x03\x04 this is a zip file, and the rest of whatever it is"

	r := strings.NewReader(body)

	_, err := simlog.NewReader(r)

	var format *gatling.FormatError
	if !errors.As(err, &format) {
		t.Fatalf("NewReader = _, %v; want a *gatling.FormatError", err)
	}

	rest, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("draining the reader: %v", err)
	}

	if got := string(format.Head) + string(rest); got != body {
		t.Fatalf("the refusal lost bytes: %d of %d recovered", len(got), len(body))
	}
}

// A source that fails with an error merely wrapping io.EOF has not reached the
// end of anything. Matching it with errors.Is would swallow a truncated
// decompressor or a closed transport and report it as bytes that are not a
// Gatling log; letting errors.Is(err, io.EOF) through would read the same
// failure as the clean end of the log. So io.EOF alone is hidden, and every
// other cause in the chain stays reachable: the rule all three packages share.
func TestWrappedEOFIsAStreamFailure(t *testing.T) {
	t.Parallel()

	broken := errors.New("transport closed")

	_, err := simlog.NewReader(iotest.ErrReader(fmt.Errorf("%w: %w", broken, io.EOF)))

	if !errors.Is(err, broken) {
		t.Fatalf("NewReader = _, %v; want the source's own failure", err)
	}

	if errors.Is(err, io.EOF) {
		t.Fatalf("a wrapped io.EOF from the source reads as the end of the log: %v", err)
	}

	if errors.As(err, new(*gatling.FormatError)) {
		t.Fatal("a wrapped io.EOF from the source must not read as a bad format")
	}

	var pathErr *fs.PathError

	_, err = simlog.NewReader(iotest.ErrReader(&fs.PathError{Op: "read", Path: "simulation.log", Err: io.EOF}))
	if !errors.As(err, &pathErr) || errors.Is(err, io.EOF) {
		t.Fatalf("NewReader over a failing file = _, %v; want its *fs.PathError reachable and no io.EOF", err)
	}
}

// A failure of the source is a failure from every constructor of this module,
// whatever shape it arrives in: never the end of the log, never a log cut
// short, never a stream too short to identify. The package documentation
// promises the first, in the list of what a follower may rely on; the codecs
// keep it; and identify — the entry point a consumer is told to prefer — broke
// it, so the rule is stated once here, over all three packages, rather than in
// a comment per package. In every cell the cause also stays reachable.
func TestASourceFailureIsNeverTheEndOfTheLog(t *testing.T) {
	t.Parallel()

	constructors := []struct {
		name string
		open func(io.Reader) error
	}{
		{"simlog.NewReader", func(r io.Reader) error { _, err := simlog.NewReader(r); return err }},
		{"simlog.NewRunReader", func(r io.Reader) error { _, err := simlog.NewRunReader(r); return err }},
		{"binary.NewReader", func(r io.Reader) error { _, err := binary.NewReader(r); return err }},
		{"binary.NewRunReader", func(r io.Reader) error { _, err := binary.NewRunReader(r); return err }},
		{"text.NewReader", func(r io.Reader) error { _, err := text.NewReader(r); return err }},
		{"text.NewRunReader", func(r io.Reader) error { _, err := text.NewRunReader(r); return err }},
	}

	causes := []struct {
		name string
		err  error
		says string
	}{
		{"a plain failure", errors.New("transport closed"), "transport closed"},
		{"a failure wrapping io.EOF", fmt.Errorf("upload aborted: %w", io.EOF), "upload aborted"},
		{"io.ErrUnexpectedEOF, as a torn gzip returns it", io.ErrUnexpectedEOF, "unexpected EOF"},
		{"a failure wrapping io.ErrUnexpectedEOF", fmt.Errorf("gunzip: %w", io.ErrUnexpectedEOF), "gunzip"},
	}

	for _, c := range constructors {
		for _, cause := range causes {
			t.Run(c.name+"/"+cause.name, func(t *testing.T) {
				t.Parallel()

				err := c.open(iotest.ErrReader(cause.err))
				if err == nil {
					t.Fatal("a failing source was read as a log")
				}

				if errors.Is(err, io.EOF) {
					t.Fatalf("errors.Is(err, io.EOF) is true for a source failure: %v", err)
				}

				if errors.As(err, new(*gatling.TruncationError)) {
					t.Fatalf("a source failure is reported as a log cut short: %v", err)
				}

				if errors.As(err, new(*gatling.FormatError)) {
					t.Fatalf("a source failure is reported as bytes that are not a log: %v", err)
				}

				if !strings.Contains(err.Error(), cause.says) {
					t.Fatalf("the cause's text is lost: %v", err)
				}

				if !errors.Is(err, cause.err) {
					t.Fatalf("the cause is not reachable through the error: %v", err)
				}
			})
		}
	}
}

// Both entry points must reach the binary codec. The refusal this replaces was
// once asserted only on NewReader, and removing the guard from NewRunReader
// alone left every test green while the constructor the README points consumers
// at fell through to a syntax error on line 1. The same asymmetry is possible
// now in reverse, so both are still checked.
func TestBinaryIsReadByBothConstructors(t *testing.T) {
	t.Parallel()

	t.Run("NewReader", func(t *testing.T) {
		t.Parallel()

		rd, err := simlog.NewReader(open(t, binaryLog()))
		if err != nil {
			t.Fatalf("NewReader = _, %v; a codec reads a binary log now", err)
		}

		if rd == nil {
			t.Fatal("a successful read must hand back a reader")
		}
	})

	t.Run("NewRunReader", func(t *testing.T) {
		t.Parallel()

		rd, err := simlog.NewRunReader(open(t, binaryLog()))
		if err != nil {
			t.Fatalf("NewRunReader = _, %v; a codec reads a binary log now", err)
		}

		if rd == nil {
			t.Fatal("a successful read must hand back a reader")
		}
	})
}

// simlogConstructors opens a stream through each of this package's
// constructors, for the tests that hold both to one answer.
func simlogConstructors() map[string]func(io.Reader) error {
	return map[string]func(io.Reader) error{
		"NewReader":    func(r io.Reader) error { _, err := simlog.NewReader(r); return err },
		"NewRunReader": func(r io.Reader) error { _, err := simlog.NewRunReader(r); return err },
	}
}

// A source's io.ErrUnexpectedEOF is not this package's to interpret.
// compress/gzip, flate and zlib return it by identity when their compressed
// input was cut, and the head loop used to return the same value for a head
// that merely ran short — so identify could not tell a torn archive from a
// stream that had not arrived yet, and answered "come back with more bytes" to
// both. A follower retries on that answer; a torn archive never gets longer.
func TestASourceUnexpectedEOFIsNotAShortHead(t *testing.T) {
	t.Parallel()

	t.Run("two bytes, then io.ErrUnexpectedEOF", func(t *testing.T) {
		t.Parallel()

		for name, open := range simlogConstructors() {
			err := open(io.MultiReader(strings.NewReader("RU"), iotest.ErrReader(io.ErrUnexpectedEOF)))
			if errors.As(err, new(*gatling.FormatError)) {
				t.Fatalf("%s: a source's io.ErrUnexpectedEOF is reported as a stream too short to identify: %v", name, err)
			}

			if err == nil || errors.Is(err, io.EOF) {
				t.Fatalf("%s = %v; want a failure that is not the end of the log", name, err)
			}

			if !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("%s: the source's own failure is not reachable through the error: %v", name, err)
			}
		}
	})

	t.Run("a gzip of a text log cut inside the detection window", func(t *testing.T) {
		t.Parallel()

		var zipped bytes.Buffer

		zw := gzip.NewWriter(&zipped)
		if _, err := zw.Write([]byte(textLog("3.12.0"))); err != nil {
			t.Fatal(err)
		}

		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}

		// The gzip header is ten bytes and parses whole; the four bytes of
		// deflate stream after it end mid-block, which flate reports as
		// io.ErrUnexpectedEOF by identity.
		cut := zipped.Bytes()[:14]

		for name, open := range simlogConstructors() {
			zr, err := gzip.NewReader(bytes.NewReader(cut))
			if err != nil {
				t.Fatalf("gzip.NewReader over %d bytes: %v", len(cut), err)
			}

			err = open(zr)
			if errors.As(err, new(*gatling.FormatError)) {
				t.Fatalf("%s: a torn gzip is reported as a stream too short to identify: %v", name, err)
			}

			if err == nil || errors.Is(err, io.EOF) {
				t.Fatalf("%s over a torn gzip = %v; want a failure that is not the end of the log", name, err)
			}
		}
	})
}

// A stream that genuinely ends inside the detection window — io.EOF from the
// source itself, after bytes that are still a possible opening — is the one
// case that is a short head, and it stays one: a sidecar attaching in a run's
// first milliseconds relies on that answer to come back with more.
func TestAGenuinelyShortStreamIsStillAShortHead(t *testing.T) {
	t.Parallel()

	for name, open := range simlogConstructors() {
		err := open(strings.NewReader("RU"))

		var formatErr *gatling.FormatError
		if !errors.As(err, &formatErr) || !formatErr.Short {
			t.Fatalf("%s over two bytes then io.EOF = %v; want a *gatling.FormatError with Short set", name, err)
		}
	}
}

// errSpool is the failure of a spool that took the bytes and then failed.
var errSpool = errors.New("the spool failed")

// spoolFailingOnce is the spool of an io.TeeReader that reports a failure on
// its first write, having taken the bytes. The tee hands that failure back
// beside the bytes it read, once, and reads on normally afterwards, so nothing
// downstream sees it again.
type spoolFailingOnce struct{ failed bool }

func (s *spoolFailingOnce) Write(p []byte) (int, error) {
	if s.failed {
		return len(p), nil
	}

	s.failed = true

	return len(p), errSpool
}

// A failure that arrives with the byte that completes the detection window is
// a failure, not a head. The window is read straight from the source, so a
// failure dropped there would be gone: the log behind it would decode, and the
// caller's spool would be short without anything saying so.
func TestAFailureBesideTheTenthByteIsNotAShortHead(t *testing.T) {
	t.Parallel()

	for name, open := range simlogConstructors() {
		err := open(io.TeeReader(strings.NewReader(textLog("3.12.0")), &spoolFailingOnce{}))
		if !errors.Is(err, errSpool) {
			t.Fatalf("%s over a tee whose spool fails beside the tenth byte = %v; want the spool's failure", name, err)
		}
	}
}
