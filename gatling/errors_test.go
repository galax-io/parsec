package gatling_test

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
)

func mustContain(t *testing.T, msg string, wants ...string) {
	t.Helper()

	for _, want := range wants {
		if !strings.Contains(msg, want) {
			t.Fatalf("message %q does not contain %q", msg, want)
		}
	}
}

func TestSyntaxError(t *testing.T) {
	t.Parallel()

	err := &gatling.SyntaxError{Line: 42, Expected: "REQUEST with 7 fields", Found: "5 fields"}
	mustContain(t, err.Error(), "42", "REQUEST with 7 fields", "5 fields")

	var target *gatling.SyntaxError
	if wrapped := fmt.Errorf("read: %w", err); !errors.As(wrapped, &target) || target.Line != 42 {
		t.Fatalf("errors.As does not recover the SyntaxError from %v", wrapped)
	}
}

func TestVersionError(t *testing.T) {
	t.Parallel()

	t.Run("below range", func(t *testing.T) {
		t.Parallel()

		err := &gatling.VersionError{
			Found: "3.9.0", Version: v(3, 9, 0), Parsed: true, Min: v(3, 11, 5), Max: v(3, 12, 0),
		}
		mustContain(t, err.Error(), "3.9.0", "3.11.5", "3.12.0", "below the supported range")
	})

	// Parsed, not the zero Version, is what separates the two faults: 0.0.0 is a
	// version string that parses, so reading the zero value as "did not parse"
	// would report a refused release as a malformed one and send a caller
	// branching on it down the wrong path.
	t.Run("below range at the zero version", func(t *testing.T) {
		t.Parallel()

		err := &gatling.VersionError{
			Found: "0.0.0", Version: v(0, 0, 0), Parsed: true, Min: v(3, 11, 5), Max: v(3, 12, 0),
		}
		mustContain(t, err.Error(), "0.0.0", "below the supported range")

		if strings.Contains(err.Error(), "not a release") {
			t.Fatalf("a version that parses is reported as unparseable: %v", err)
		}
	})

	t.Run("not a release", func(t *testing.T) {
		t.Parallel()

		err := &gatling.VersionError{Found: "3.13.0-SNAPSHOT", Min: v(3, 11, 5), Max: v(3, 12, 0)}
		mustContain(t, err.Error(), `"3.13.0-SNAPSHOT"`, "3.11.5", "3.12.0", "not a release")
	})

	t.Run("errors.As", func(t *testing.T) {
		t.Parallel()

		err := &gatling.VersionError{Found: "3.9.0", Version: v(3, 9, 0)}

		var target *gatling.VersionError
		if wrapped := fmt.Errorf("open: %w", err); !errors.As(wrapped, &target) || target.Found != "3.9.0" {
			t.Fatalf("errors.As does not recover the VersionError from %v", wrapped)
		}
	})
}

func TestWarning(t *testing.T) {
	t.Parallel()

	w := gatling.Warning{Version: v(3, 13, 0), Min: v(3, 11, 5), Max: v(3, 12, 0)}
	mustContain(t, w.String(), "3.13.0", "3.11.5", "3.12.0")
	mustContain(t, strings.ToLower(w.String()), "no recording")
}

func TestTruncationError(t *testing.T) {
	t.Parallel()

	t.Run("a text log names the line", func(t *testing.T) {
		t.Parallel()

		err := &gatling.TruncationError{
			Format: gatling.FormatText, Line: 42, Dropped: 33, Expected: "the rest of the line",
		}
		mustContain(t, err.Error(), "cut short", "33", "the rest of the line")

		if !strings.HasPrefix(err.Error(), "gatling: line 42:") {
			t.Fatalf("a text truncation does not open with its line: %v", err)
		}
	})

	t.Run("a binary log names the offset", func(t *testing.T) {
		t.Parallel()

		err := &gatling.TruncationError{
			Format: gatling.FormatBinary, Offset: 1234, Dropped: 57, Expected: "a request record's name",
		}
		mustContain(t, err.Error(), "cut short", "57", "a request record's name")

		if !strings.HasPrefix(err.Error(), "gatling: byte 1234:") {
			t.Fatalf("a binary truncation does not open with its offset: %v", err)
		}

		if strings.Contains(err.Error(), "line") {
			t.Fatalf("a binary truncation reports a line number: %v", err)
		}
	})

	// The whole point of a separate type. A consumer written before this type
	// existed breaks its loop on io.EOF and treats everything else as a failed
	// read; if a truncation were io.EOF, or wrapped one, that consumer would
	// silently report a run killed mid-flight as a complete one.
	t.Run("is not the end of the log", func(t *testing.T) {
		t.Parallel()

		err := error(&gatling.TruncationError{Format: gatling.FormatBinary, Offset: 8, Dropped: 4})

		if errors.Is(err, io.EOF) {
			t.Fatal("a truncation is io.EOF, so a caller written before v0.0.8 reads a cut log as complete")
		}

		if errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatal("a truncation is io.ErrUnexpectedEOF, which is how a failing source reports itself")
		}

		var syntaxErr *gatling.SyntaxError
		if errors.As(err, &syntaxErr) {
			t.Fatal("a truncation unwraps to a SyntaxError, so a cut log still reads as a damaged one")
		}
	})

	t.Run("errors.As", func(t *testing.T) {
		t.Parallel()

		err := &gatling.TruncationError{Format: gatling.FormatText, Line: 7, Dropped: 12}

		var target *gatling.TruncationError
		if wrapped := fmt.Errorf("read: %w", err); !errors.As(wrapped, &target) || target.Dropped != 12 {
			t.Fatalf("errors.As does not recover the TruncationError from %v", wrapped)
		}
	})
}

func TestRunNotFoundError(t *testing.T) {
	t.Parallel()

	t.Run("names the directory the caller gave", func(t *testing.T) {
		t.Parallel()

		err := &gatling.RunNotFoundError{Dir: "/srv/results"}
		mustContain(t, err.Error(), "/srv/results", "no Gatling run")

		var target *gatling.RunNotFoundError
		if wrapped := fmt.Errorf("find: %w", err); !errors.As(wrapped, &target) || target.Dir != "/srv/results" {
			t.Fatalf("errors.As does not recover the RunNotFoundError from %v", wrapped)
		}
	})

	// A consumer with no meaningful working directory — a server — is otherwise
	// shown a relative path it never chose, with nothing saying where it came
	// from. The flag is the difference between a puzzle and an instruction.
	t.Run("says when the directory was a default", func(t *testing.T) {
		t.Parallel()

		err := &gatling.RunNotFoundError{Dir: "target/gatling", Default: true}
		mustContain(t, err.Error(), "target/gatling", "default")
	})

	t.Run("does not say default when the caller chose the directory", func(t *testing.T) {
		t.Parallel()

		err := &gatling.RunNotFoundError{Dir: "build/reports/gatling"}
		if strings.Contains(err.Error(), "default") {
			t.Fatalf("message %q calls a caller-supplied directory a default", err.Error())
		}
	})
}
