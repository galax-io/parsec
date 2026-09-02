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

func ver(major, minor, patch int) gatling.Version {
	return gatling.Version{Major: major, Minor: minor, Patch: patch}
}

func fixturePath(name string) string {
	return filepath.Join("testdata", "fixtures", name+".fixture.log")
}

func openFixture(t *testing.T, name string) *os.File {
	t.Helper()

	f, err := os.Open(fixturePath(name))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = f.Close() })

	return f
}

func TestGateRefusedBelowRange(t *testing.T) {
	t.Parallel()

	r, err := text.NewReader(openFixture(t, "version-3.9.0"))

	var ve *gatling.VersionError
	if !errors.As(err, &ve) {
		t.Fatalf("got %v, want *gatling.VersionError", err)
	}

	if r != nil {
		t.Fatal("a refused log must not yield a Reader")
	}

	oldest, newest := text.SupportedVersions()
	if ve.Found != "3.9.0" || ve.Version != ver(3, 9, 0) || ve.Min != oldest || ve.Max != newest {
		t.Fatalf("got %+v, want Found=3.9.0 Min=%v Max=%v", ve, oldest, newest)
	}
}

func TestGateAccepted(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"version-3.11.5", "version-3.12.0"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r, err := text.NewReader(openFixture(t, name))
			if err != nil {
				t.Fatalf("NewReader: %v", err)
			}

			if w := r.Warnings(); len(w) != 0 {
				t.Fatalf("Warnings() = %v for a covered version", w)
			}

			if recs := readAll(t, r); len(recs) != 3 {
				t.Fatalf("got %d records, want 3", len(recs))
			}
		})
	}
}

func TestGateUnverifiedAboveRange(t *testing.T) {
	t.Parallel()

	r, err := text.NewReader(openFixture(t, "version-3.13.0"))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	oldest, newest := text.SupportedVersions()

	w := r.Warnings()
	if len(w) != 1 || w[0].Version != ver(3, 13, 0) || w[0].Min != oldest || w[0].Max != newest {
		t.Fatalf("Warnings() = %+v, want one naming 3.13.0 and the range", w)
	}

	if !strings.Contains(w[0].String(), "3.13.0") {
		t.Fatalf("warning %q does not name the version", w[0].String())
	}

	if recs := readAll(t, r); len(recs) != 3 {
		t.Fatalf("got %d records, want 3 — an unverified version still decodes", len(recs))
	}
}

func TestGateNotARelease(t *testing.T) {
	t.Parallel()

	tests := []struct{ fixture, found string }{
		{fixture: "version-3.13.0-snapshot", found: "3.13.0-SNAPSHOT"},
		{fixture: "version-3.12.0-m1", found: "3.12.0-M1"},
		{fixture: "version-garbage", found: "garbage"},
	}

	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			t.Parallel()

			_, err := text.NewReader(openFixture(t, tt.fixture))

			var ve *gatling.VersionError
			if !errors.As(err, &ve) {
				t.Fatalf("got %v, want *gatling.VersionError", err)
			}

			if ve.Found != tt.found || ve.Version != (gatling.Version{}) {
				t.Fatalf("got %+v, want Found=%q and a zero Version", ve, tt.found)
			}

			if !strings.Contains(err.Error(), `"`+tt.found+`"`) {
				t.Fatalf("error %q does not quote %q", err, tt.found)
			}
		})
	}
}

func TestGateNoHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		fixture   string
		wantLine  int
		wantWords string
	}{
		{fixture: "no-header", wantLine: 2, wantWords: "run header"},
		{fixture: "event-before-header", wantLine: 1, wantWords: "USER"},
		{fixture: "empty", wantLine: 0, wantWords: "run header"},
	}

	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			t.Parallel()

			_, err := text.NewReader(openFixture(t, tt.fixture))

			var se *gatling.SyntaxError
			if !errors.As(err, &se) {
				t.Fatalf("got %v, want *gatling.SyntaxError", err)
			}

			if se.Line != tt.wantLine || !strings.Contains(err.Error(), tt.wantWords) {
				t.Fatalf("got %v, want line %d mentioning %q", err, tt.wantLine, tt.wantWords)
			}
		})
	}
}

func TestGateSurplusField(t *testing.T) {
	t.Parallel()

	t.Run("above range decodes with the surplus ignored", func(t *testing.T) {
		t.Parallel()

		r, err := text.NewReader(openFixture(t, "surplus-field-3.13.0"))
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}

		recs := readAll(t, r)
		if len(recs) != 4 || recs[0].Kind != gatling.KindUser || recs[0].Timestamp != 1788379356165 {
			t.Fatalf("got %+v, want 4 records starting with the 5-field USER", recs)
		}
	})

	t.Run("inside range fails at that line", func(t *testing.T) {
		t.Parallel()

		r, err := text.NewReader(openFixture(t, "surplus-field-3.11.5"))
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}

		_, err = r.Next()

		var se *gatling.SyntaxError
		if !errors.As(err, &se) || se.Line != 3 || !strings.Contains(se.Found, "5 fields") {
			t.Fatalf("got %v, want a SyntaxError at line 3 finding 5 fields", err)
		}
	})
}
