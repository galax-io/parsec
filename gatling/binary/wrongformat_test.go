package binary_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
)

// The reverse of the text codec's case, and the tighter one: this constructor
// fails at byte 0, having consumed exactly one byte. The bytes are in the
// buffered reader all the same, so the head is peeked before that byte is read —
// peeking after it would hand Detect bytes 1..10 and misidentify the log.
func TestBinaryReaderRefusesATextLog(t *testing.T) {
	t.Parallel()

	log := filepath.Join("..", "..", "testdata", "corpus", "gatling", "3.11.5", "simulation.log")

	raw, err := os.ReadFile(log) //nolint:gosec // a fixed path inside the module's own corpus
	if err != nil {
		t.Fatalf("reading the text corpus log: %v", err)
	}

	opens := map[string]func() error{
		"NewReader": func() error {
			_, err := binary.NewReader(bytes.NewReader(raw))

			return err
		},
		"NewRunReader": func() error {
			_, err := binary.NewRunReader(bytes.NewReader(raw))

			return err
		},
	}

	for name, open := range opens {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := open()
			if err == nil {
				t.Fatal("a text log opened through the binary codec without an error")
			}

			var unsupported *gatling.UnsupportedFormatError
			if !errors.As(err, &unsupported) {
				t.Fatalf("error is %T (%v), want *gatling.UnsupportedFormatError", err, err)
			}

			if unsupported.Format != gatling.FormatText {
				t.Errorf("Format = %v, want %v", unsupported.Format, gatling.FormatText)
			}

			if len(unsupported.Head) == 0 {
				t.Error("Head is empty; a caller holding an unrewindable stream needs the bytes back")
			}

			var syntax *gatling.SyntaxError
			if errors.As(err, &syntax) {
				t.Error("the error still unwraps to *gatling.SyntaxError, which means a damaged log")
			}

			if !strings.Contains(err.Error(), "gatling/simlog") {
				t.Errorf("message %q names no package that reads this format", err)
			}
		})
	}
}

// A binary log of this codec's own format, damaged, is still a damaged log; and
// a file that is not a Gatling simulation.log at all is still neither.
func TestBinaryReaderStillReportsDamageAndNonGatlingFiles(t *testing.T) {
	t.Parallel()

	t.Run("a damaged binary log", func(t *testing.T) {
		t.Parallel()

		// A well-formed run record, then a record kind no version writes.
		raw := (&builder{}).runRecord("3.15.1", []string{"s"}, nil).u8(200).bytes()

		r, err := binary.NewReader(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("the run record itself must still open: %v", err)
		}

		_, err = r.Next()

		var syntax *gatling.SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("error is %T (%v), want *gatling.SyntaxError", err, err)
		}
	})

	t.Run("a gzip header is not a Gatling log", func(t *testing.T) {
		t.Parallel()

		_, err := binary.NewReader(bytes.NewReader([]byte{
			0x1f, 0x8b, 0x08, 0x08, 0x56, 0x66, 0x9d, 0x6a, 0x00, 0x03,
		}))

		var unsupported *gatling.UnsupportedFormatError
		if errors.As(err, &unsupported) {
			t.Fatalf("a non-Gatling file reported as a known format: %v", err)
		}

		if err == nil {
			t.Fatal("a gzip header opened without an error")
		}
	})
}
