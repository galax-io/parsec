package gatling_test

import (
	"errors"
	"fmt"
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
