package simlog_test

import (
	"errors"
	"fmt"
	"io"
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

// The whole milestone is here. A binary log handed to the text codec fails on
// its first line with a message about a missing separator, which sends a user
// looking for corruption that is not there. Handed to this package it is read.
//
// The text half is kept: it records what a consumer gets without this package,
// and it is what makes the other half mean something.
func TestBinaryIsReadWhereTheTextCodecOnlyFails(t *testing.T) {
	t.Parallel()

	t.Run("through the text codec, a syntax error", func(t *testing.T) {
		t.Parallel()

		_, err := text.NewReader(open(t, binaryLog()))

		var syntaxErr *gatling.SyntaxError
		if !errors.As(err, &syntaxErr) {
			t.Fatalf("text.NewReader on a binary log = %v; want a *gatling.SyntaxError — "+
				"this half of the test records the outcome the milestone exists to replace", err)
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

// io.ReadFull converts a source EOF with ==, so an error that merely wraps
// io.EOF reaches the caller unconverted. Matching it with errors.Is would
// swallow a truncated decompressor or a closed transport and report it as bytes
// that are not a Gatling log, sending a user to inspect a file that is fine.
func TestWrappedEOFIsAStreamFailure(t *testing.T) {
	t.Parallel()

	broken := errors.New("transport closed")

	_, err := simlog.NewReader(iotest.ErrReader(fmt.Errorf("%w: %w", broken, io.EOF)))

	if !errors.Is(err, broken) {
		t.Fatalf("NewReader = _, %v; want the source's own failure", err)
	}

	if errors.As(err, new(*gatling.FormatError)) {
		t.Fatal("a wrapped io.EOF from the source must not read as a bad format")
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
