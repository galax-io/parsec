package gatling_test

import (
	"errors"
	"testing"

	"github.com/galax-io/parsec/gatling"
)

// A simulation.log is untrusted input, and so is anything a caller mistakes for
// one. Detect must return an answer or an error for every possible input and
// must never panic — Principle II allows a decoder no other outcome.
func FuzzDetect(f *testing.F) {
	f.Add([]byte(nil))
	f.Add([]byte("RUN\t"))
	f.Add([]byte("ASSERTION\t"))
	f.Add([]byte{0x00})
	f.Add([]byte("ASSERT"))
	f.Add([]byte("<html>"))
	f.Add([]byte{0x1f, 0x8b, 0x08, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, head []byte) {
		format, err := gatling.Detect(head)
		if err == nil {
			if format != gatling.FormatText && format != gatling.FormatBinary {
				t.Fatalf("Detect(%q) = %v, <nil>; a nil error must name a real format", head, format)
			}

			return
		}

		if format != gatling.FormatUnknown {
			t.Fatalf("Detect(%q) = %v, %v; an error must come with the unknown format", head, format, err)
		}

		var formatErr *gatling.FormatError
		if !errors.As(err, &formatErr) {
			t.Fatalf("Detect(%q) = _, %v; every refusal is a *gatling.FormatError", head, err)
		}

		// Whatever it reports back is bounded by the window, so an error
		// message cannot grow with the input.
		if len(formatErr.Head) > gatling.DetectSize {
			t.Fatalf("Detect(%q): FormatError.Head is %d bytes; the window is %d",
				head, len(formatErr.Head), gatling.DetectSize)
		}

		// The message must be renderable whatever the bytes were.
		_ = formatErr.Error()
	})
}
