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

// The version is judged before the rest of the RUN line, so a log naming a
// version below the range is refused as such even when its run start is out of
// bounds. The binary codec does the same, and simlog exists so a consumer
// cannot tell the two apart; before this the start was validated first and the
// same line was reported as damage.
func TestAVersionBelowTheRangeIsJudgedBeforeTheRunStart(t *testing.T) {
	t.Parallel()

	line := func(version string) string {
		return "RUN\tio.example.Sim\tsim\t9223372036854775807\t \t" + version + "\n"
	}

	_, err := text.NewReader(strings.NewReader(line("1.0.0")))
	if !errors.As(err, new(*gatling.VersionError)) {
		t.Fatalf("NewReader over 1.0.0 with a run start past the ceiling = _, %v; want a *gatling.VersionError", err)
	}

	_, err = text.NewReader(strings.NewReader(line("3.12.0")))
	if !errors.As(err, new(*gatling.SyntaxError)) {
		t.Fatalf("NewReader over 3.12.0 with a run start past the ceiling = _, %v; want the start's *gatling.SyntaxError", err)
	}
}

// An ASSERTION line with too few fields waits for the version, as a surplus
// field does, so a version the gate refuses outranks it — the answer the binary
// codec gives for a damaged assertion table, whose version comes first. With no
// version to rule on, the earliest fault in the file is the one reported.
func TestAShortAssertionLineIsJudgedAfterTheVersion(t *testing.T) {
	t.Parallel()

	const short = "ASSERTION\n"

	header := func(version string) string { return "RUN\tio.example.Sim\tsim\t1788670094356\t \t" + version + "\n" }
	refused := func(err error) bool {
		return errors.As(err, new(*gatling.VersionError)) || errors.As(err, new(*gatling.UnverifiedError))
	}
	lineOne := func(err error) bool {
		var se *gatling.SyntaxError

		return errors.As(err, &se) && se.Line == 1
	}

	tests := []struct {
		name string
		log  string
		opts []gatling.Option
		want func(error) bool
		says string
	}{
		{name: "beside a version below the range", log: short + header("1.0.0"), want: refused, says: "the version refused"},
		{
			name: "beside a version above the range, strict", log: short + header("3.99.0"),
			opts: []gatling.Option{gatling.WithStrict()}, want: refused, says: "the version refused",
		},
		{name: "beside a version in the range", log: short + header("3.12.0"), want: lineOne, says: "a *gatling.SyntaxError at line 1"},
		{name: "before a line that is neither", log: short + "garbage\n", want: lineOne, says: "a *gatling.SyntaxError at line 1"},
		{name: "before a run header with no version", log: short + "RUN\tio.example.Sim\n", want: lineOne, says: "a *gatling.SyntaxError at line 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := text.NewReader(strings.NewReader(tt.log), tt.opts...)
			if !tt.want(err) {
				t.Fatalf("NewReader = _, %v; want %s", err, tt.says)
			}
		})
	}
}
