package text_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/text"
)

// The most likely first failure a new consumer meets is opening an archived run
// with the wrong codec, and it used to say the log was damaged. A caller with a
// mixed archive that catches *gatling.SyntaxError and quarantines the file as
// corrupt would have quarantined every binary log it held.
//
// The answer is in bytes the reader has already consumed: the preamble scanner
// has read a whole line by the time it gives up, far more than the ten
// gatling.Detect needs.
func TestTextReaderRefusesABinaryLog(t *testing.T) {
	t.Parallel()

	log := filepath.Join("..", "..", "testdata", "corpus", "gatling", "3.14.9", "simulation.log")

	raw, err := os.ReadFile(log) //nolint:gosec // a fixed path inside the module's own corpus
	if err != nil {
		t.Fatalf("reading the binary corpus log: %v", err)
	}

	opens := map[string]func() error{
		"NewReader": func() error {
			_, err := text.NewReader(strings.NewReader(string(raw)))

			return err
		},
		"NewRunReader": func() error {
			_, err := text.NewRunReader(strings.NewReader(string(raw)))

			return err
		},
	}

	for name, open := range opens {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := open()
			if err == nil {
				t.Fatal("a binary log opened through the text codec without an error")
			}

			var unsupported *gatling.UnsupportedFormatError
			if !errors.As(err, &unsupported) {
				t.Fatalf("error is %T (%v), want *gatling.UnsupportedFormatError", err, err)
			}

			if unsupported.Format != gatling.FormatBinary {
				t.Errorf("Format = %v, want %v", unsupported.Format, gatling.FormatBinary)
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

// A log of the format this codec does take, damaged, is still a damaged log. The
// wrong-format answer must not widen into the case it was carved out of.
func TestADamagedTextLogIsStillASyntaxError(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"an unknown record kind on line 1": "NOPE\ta\tb\n",
		"a damaged record after the header": "RUN\tio.example.Sim\tsim\t1788670094356\t \t3.12.0\n" +
			"NOPE\tx\n",
	}

	for name, log := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r, err := text.NewReader(strings.NewReader(log))
			if err == nil {
				_, err = r.Next()
			}

			var syntax *gatling.SyntaxError
			if !errors.As(err, &syntax) {
				t.Fatalf("error is %T (%v), want *gatling.SyntaxError", err, err)
			}
		})
	}
}
