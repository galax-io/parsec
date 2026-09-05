package simlog_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/simlog"
	"github.com/galax-io/parsec/gatling/text"
)

// A caller states its policy once and it has to survive every layer between
// there and the gate. Four constructors, one option, one outcome.
func TestStrictReachesTheGateThroughEveryConstructor(t *testing.T) {
	t.Parallel()

	const aboveRange = "3.13.0"

	t.Run("refused above the range", func(t *testing.T) {
		t.Parallel()

		opens := map[string]func() error{
			"text.NewReader": func() error {
				_, err := text.NewReader(strings.NewReader(textLog(aboveRange)), gatling.WithStrict())

				return err
			},
			"text.NewRunReader": func() error {
				_, err := text.NewRunReader(strings.NewReader(textLog(aboveRange)), gatling.WithStrict())

				return err
			},
			"simlog.NewReader": func() error {
				_, err := simlog.NewReader(strings.NewReader(textLog(aboveRange)), gatling.WithStrict())

				return err
			},
			"simlog.NewRunReader": func() error {
				_, err := simlog.NewRunReader(strings.NewReader(textLog(aboveRange)), gatling.WithStrict())

				return err
			},
		}

		for name, open := range opens {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				err := open()

				var unverified *gatling.UnverifiedError
				if !errors.As(err, &unverified) {
					t.Fatalf("%s with WithStrict = %v; want a *gatling.UnverifiedError", name, err)
				}

				if unverified.Version.String() != aboveRange {
					t.Fatalf("%s: UnverifiedError names %s; want %s", name, unverified.Version, aboveRange)
				}
			})
		}
	})

	t.Run("the same log decodes with one warning when not strict", func(t *testing.T) {
		t.Parallel()

		rd, err := simlog.NewReader(strings.NewReader(textLog(aboveRange)))
		if err != nil {
			t.Fatalf("lenient NewReader = _, %v; want it to decode", err)
		}

		if got := rd.Warnings(); len(got) != 1 {
			t.Fatalf("Warnings() = %+v (%d); want exactly 1", got, len(got))
		}
	})
}

// Strictness may only tighten the gate. A covered version has to read
// identically with it and without it, down to the records.
func TestStrictChangesNothingInsideTheRange(t *testing.T) {
	t.Parallel()

	for _, log := range corpusLogs(t) {
		t.Run(filepath.Base(filepath.Dir(log)), func(t *testing.T) {
			t.Parallel()

			whole, err := os.ReadFile(log) //nolint:gosec // a corpus path from the test's own glob
			if err != nil {
				t.Fatalf("reading %s: %v", log, err)
			}

			lenient, err := simlog.NewReader(strings.NewReader(string(whole)))
			if err != nil {
				t.Fatalf("lenient NewReader: %v", err)
			}

			strict, err := simlog.NewReader(strings.NewReader(string(whole)), gatling.WithStrict())
			if err != nil {
				t.Fatalf("strict NewReader on a covered version: %v; strictness must not refuse it", err)
			}

			if got, want := strict.Header(), lenient.Header(); got != want {
				t.Fatalf("Header() differs under strictness: %+v vs %+v", got, want)
			}

			if got, want := strict.Warnings(), lenient.Warnings(); len(got) != 0 || len(want) != 0 {
				t.Fatalf("a covered version raised warnings: strict %+v, lenient %+v", got, want)
			}

			got, want := records(t, strict), records(t, lenient)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("records differ under strictness: %d vs %d", len(got), len(want))
			}
		})
	}
}

// Strictness must not turn a too-old version into a different kind of refusal.
func TestStrictDoesNotChangeTheBelowRangeRefusal(t *testing.T) {
	t.Parallel()

	_, lenientErr := simlog.NewReader(strings.NewReader(textLog("3.9.0")))
	_, strictErr := simlog.NewReader(strings.NewReader(textLog("3.9.0")), gatling.WithStrict())

	if lenientErr == nil || strictErr == nil {
		t.Fatalf("want both refused; lenient = %v, strict = %v", lenientErr, strictErr)
	}

	if lenientErr.Error() != strictErr.Error() {
		t.Fatalf("the refusal changed under strictness:\n lenient %v\n strict  %v", lenientErr, strictErr)
	}
}
