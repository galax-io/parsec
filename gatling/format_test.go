package gatling_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
)

func TestDetect(t *testing.T) {
	t.Parallel()

	// Every row of the contract's behaviour table. The two text openings are
	// the only ones a log may legally start with: Gatling writes one ASSERTION
	// record per declared assertion ahead of the RUN header, and refuses any
	// other kind there.
	tests := []struct {
		name     string
		head     []byte
		want     gatling.Format
		wantErr  bool
		wantShrt bool
	}{
		// 00 | 00 00 00 06 | '3' — the kind byte, the release string's length,
		// and its first digit. Taken from a real 3.15.1 log; see
		// testdata/corpus/gatling/3.15.1/RECORDING.md.
		{name: "binary run record", head: []byte{0x00, 0x00, 0x00, 0x00, 0x06, '3'}, want: gatling.FormatBinary},
		{name: "binary, whole window", head: []byte{0x00, 0x00, 0x00, 0x00, 0x06, '3', '.', '1', '5', '.'}, want: gatling.FormatBinary},
		{name: "text opening with RUN", head: []byte("RUN\tsim\trun\t1"), want: gatling.FormatText},
		{name: "text opening with RUN, exactly", head: []byte("RUN\t"), want: gatling.FormatText},
		{name: "text opening with ASSERTION", head: []byte("ASSERTION\tAAEBAAIC"), want: gatling.FormatText},
		{name: "text opening with ASSERTION, exactly", head: []byte("ASSERTION\t"), want: gatling.FormatText},

		{name: "empty", head: nil, wantErr: true, wantShrt: true},
		{name: "prefix of ASSERTION", head: []byte("ASSERT"), wantErr: true, wantShrt: true},
		{name: "prefix of RUN", head: []byte("RU"), wantErr: true, wantShrt: true},
		{name: "one NUL byte alone", head: []byte{0x00}, wantErr: true, wantShrt: true},
		{name: "prefix of the binary opening", head: []byte{0x00, 0x00, 0x00, 0x00, 0x06}, wantErr: true, wantShrt: true},

		{name: "RUN without its tab", head: []byte("RUN "), wantErr: true},
		{name: "a word starting with R", head: []byte("Ran the suite"), wantErr: true},
		{name: "a word starting with A", head: []byte("Also ran it"), wantErr: true},
		{name: "ASSERTION without its tab", head: []byte("ASSERTIONS "), wantErr: true},
		{name: "html", head: []byte("<html><head>"), wantErr: true},
		{name: "gzip", head: []byte{0x1f, 0x8b, 0x08, 0x00}, wantErr: true},
		{name: "json", head: []byte(`{"runs": []}`), wantErr: true},

		// A leading NUL is the most common first byte a file can have, so it is
		// not on its own evidence of anything. Answering "binary simulation.log,
		// no codec yet" for one of these would promise a later release will read
		// a file that is not a Gatling log at all.
		{name: "a text log re-encoded UTF-16BE", head: []byte{0x00, 'R', 0x00, 'U', 0x00, 'N', 0x00, '\t'}, wantErr: true},
		{name: "an all-NUL block", head: make([]byte, gatling.DetectSize), wantErr: true},
		{name: "a NUL-led length that is really data", head: []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, wantErr: true},
		{name: "a release length of zero", head: []byte{0x00, 0x00, 0x00, 0x00, 0x00, '3'}, wantErr: true},
		{name: "a release string not starting with a digit", head: []byte{0x00, 0x00, 0x00, 0x00, 0x06, 'v'}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := gatling.Detect(tt.head)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("Detect(%q) = _, %v; want %v, <nil>", tt.head, err, tt.want)
				}

				if got != tt.want {
					t.Fatalf("Detect(%q) = %v; want %v", tt.head, got, tt.want)
				}

				return
			}

			if got != gatling.FormatUnknown {
				t.Fatalf("Detect(%q) = %v; want %v on error", tt.head, got, gatling.FormatUnknown)
			}

			var formatErr *gatling.FormatError
			if !errors.As(err, &formatErr) {
				t.Fatalf("Detect(%q) = _, %v; want a *gatling.FormatError", tt.head, err)
			}

			if formatErr.Short != tt.wantShrt {
				t.Fatalf("Detect(%q): Short = %v; want %v", tt.head, formatErr.Short, tt.wantShrt)
			}
		})
	}
}

// A caller that buffers for itself needs to know how much to read, so the
// window is exported and is the length of the longest opening a log may have.
func TestDetectSize(t *testing.T) {
	t.Parallel()

	if gatling.DetectSize != len("ASSERTION\t") {
		t.Fatalf("DetectSize = %d; want %d, the length of the longest opening", gatling.DetectSize, len("ASSERTION\t"))
	}
}

// Detect must never read past the window, whatever it is handed. Asserting the
// returned Format proves nothing — the first ten bytes decide that either way —
// so this pins what the window actually bounds: the bytes an error carries
// back, which is the only place a missing clamp would show.
func TestDetectReadsBoundedInput(t *testing.T) {
	t.Parallel()

	const huge = 1 << 20

	t.Run("a classified head is not retained", func(t *testing.T) {
		t.Parallel()

		head := append([]byte("ASSERTION\t"), bytes.Repeat([]byte("x"), huge)...)

		got, err := gatling.Detect(head)
		if err != nil || got != gatling.FormatText {
			t.Fatalf("Detect(long) = %v, %v; want %v, <nil>", got, err, gatling.FormatText)
		}
	})

	t.Run("a refused head is clamped to the window", func(t *testing.T) {
		t.Parallel()

		head := append([]byte("<html>"), bytes.Repeat([]byte("x"), huge)...)

		_, err := gatling.Detect(head)

		var formatErr *gatling.FormatError
		if !errors.As(err, &formatErr) {
			t.Fatalf("Detect(long) = _, %v; want a *gatling.FormatError", err)
		}

		// Without the clamp this is a megabyte cloned into every error.
		if len(formatErr.Head) != gatling.DetectSize {
			t.Fatalf("FormatError.Head is %d bytes; the window is %d", len(formatErr.Head), gatling.DetectSize)
		}
	})
}

func TestFormatString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		format gatling.Format
		want   string
	}{
		{gatling.FormatUnknown, "unknown"},
		{gatling.FormatText, "text"},
		{gatling.FormatBinary, "binary"},
		{gatling.Format(9), "Format(9)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			if got := tt.format.String(); got != tt.want {
				t.Fatalf("Format(%d).String() = %q; want %q", tt.format, got, tt.want)
			}
		})
	}
}

func TestFormatErrorMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		err   *gatling.FormatError
		wants []string
	}{
		{
			name:  "long enough, and printable",
			err:   &gatling.FormatError{Head: []byte("<html><hea")},
			wants: []string{"not a Gatling simulation.log", `"<html><hea"`},
		},
		{
			// A gzip stream must read as text, not as a spray of bytes.
			name:  "unprintable bytes are quoted",
			err:   &gatling.FormatError{Head: []byte{0x1f, 0x8b, 0x08}},
			wants: []string{"not a Gatling simulation.log", `\x1f`},
		},
		{
			name:  "short says so, and how short",
			err:   &gatling.FormatError{Head: []byte("ASSERT"), Short: true},
			wants: []string{"6 bytes is too few to tell", "still a possible opening"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.err.Error()
			if !strings.HasPrefix(got, "gatling: ") {
				t.Fatalf("Error() = %q; want the gatling: prefix every error in this package carries", got)
			}

			for _, want := range tt.wants {
				if !strings.Contains(got, want) {
					t.Fatalf("Error() = %q; want it to contain %q", got, want)
				}
			}
		})
	}
}
