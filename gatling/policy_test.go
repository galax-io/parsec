package gatling_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
)

// corpusPolicy is the range the text codec's golden corpus covers. The policy
// is a value, so a test can state one without a codec.
var corpusPolicy = gatling.Policy{Min: v(3, 11, 5), Max: v(3, 12, 0)}

func TestPolicyApplyRefusesBelowTheRange(t *testing.T) {
	t.Parallel()

	for _, found := range []gatling.Version{v(3, 10, 5), v(3, 11, 4), v(0, 0, 0)} {
		t.Run(found.String(), func(t *testing.T) {
			t.Parallel()

			verdict, warning, err := corpusPolicy.Apply(found)

			if verdict != gatling.VerdictRefused {
				t.Fatalf("verdict = %v; a refusal is always VerdictRefused", verdict)
			}

			if warning != (gatling.Warning{}) {
				t.Fatalf("warning = %+v; a refused read raises none", warning)
			}

			var versionErr *gatling.VersionError
			if !errors.As(err, &versionErr) {
				t.Fatalf("err = %v; want a *gatling.VersionError", err)
			}

			if !versionErr.Parsed {
				t.Fatal("Parsed = false; the version parsed, it is only too old")
			}

			if versionErr.Version != found || versionErr.Min != corpusPolicy.Min || versionErr.Max != corpusPolicy.Max {
				t.Fatalf("err = %+v; want the version found and both bounds of the range", versionErr)
			}
		})
	}
}

func TestPolicyApplyAcceptsInsideTheRange(t *testing.T) {
	t.Parallel()

	for _, found := range []gatling.Version{v(3, 11, 5), v(3, 11, 9), v(3, 12, 0)} {
		t.Run(found.String(), func(t *testing.T) {
			t.Parallel()

			verdict, warning, err := corpusPolicy.Apply(found)
			if err != nil {
				t.Fatalf("err = %v; a covered version is accepted", err)
			}

			if verdict != gatling.VerdictAccepted {
				t.Fatalf("verdict = %v; want VerdictAccepted", verdict)
			}

			if warning != (gatling.Warning{}) {
				t.Fatalf("warning = %+v; a covered version raises none", warning)
			}
		})
	}
}

func TestPolicyApplyWarnsAboveTheRange(t *testing.T) {
	t.Parallel()

	for _, found := range []gatling.Version{v(3, 12, 1), v(3, 13, 0), v(3, 99, 0)} {
		t.Run(found.String(), func(t *testing.T) {
			t.Parallel()

			verdict, warning, err := corpusPolicy.Apply(found)
			if err != nil {
				t.Fatalf("err = %v; an unverified version still decodes", err)
			}

			if verdict != gatling.VerdictUnverified {
				t.Fatalf("verdict = %v; want VerdictUnverified", verdict)
			}

			if warning.Version != found || warning.Min != corpusPolicy.Min || warning.Max != corpusPolicy.Max {
				t.Fatalf("warning = %+v; want it to name the version and the range", warning)
			}
		})
	}
}

// The zero Warning is how "no warning" travels, so it must not render as one.
// Apply returns it for every accepted version, and a caller logging the result
// would otherwise report a false alarm about version 0.0.0 on every healthy run.
func TestZeroWarningRendersEmpty(t *testing.T) {
	t.Parallel()

	if got := (gatling.Warning{}).String(); got != "" {
		t.Fatalf("Warning{}.String() = %q; the no-warning sentinel must render as empty", got)
	}

	_, warning, err := corpusPolicy.Apply(v(3, 11, 5))
	if err != nil {
		t.Fatalf("Apply(3.11.5) = _, _, %v", err)
	}

	if got := warning.String(); got != "" {
		t.Fatalf("an accepted version produced %q", got)
	}
}

// A nil Option is what an ordinary helper returns for "no policy configured".
// Calling it would panic, and because options are resolved on every path the
// crash would once have depended on the log's own version.
func TestNilOptionIsSkipped(t *testing.T) {
	t.Parallel()

	strictIf := func(on bool) gatling.Option {
		if on {
			return gatling.WithStrict()
		}

		return nil
	}

	for _, found := range []gatling.Version{v(3, 10, 5), v(3, 11, 5), v(3, 12, 0), v(3, 13, 0)} {
		t.Run(found.String(), func(t *testing.T) {
			t.Parallel()

			// A panic here fails the test; that is the assertion.
			_, _, _ = corpusPolicy.Apply(found, strictIf(false))
			_, _, _ = corpusPolicy.Apply(found, nil, gatling.WithStrict(), nil)
		})
	}
}

// Strictness may only ever turn the unverified case into a refusal. Every other
// cell of this table is identical with and without it, which is the requirement
// asserted rather than assumed.
func TestPolicyApplyStrictness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		found         gatling.Version
		strictDiffers bool
	}{
		{name: "below the range is refused either way", found: v(3, 10, 5)},
		{name: "covered reads the same either way", found: v(3, 11, 5)},
		{name: "the newest covered reads the same either way", found: v(3, 12, 0)},
		{name: "above the range is where strictness bites", found: v(3, 13, 0), strictDiffers: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, lenientWarning, lenientErr := corpusPolicy.Apply(tt.found)
			_, strictWarning, strictErr := corpusPolicy.Apply(tt.found, gatling.WithStrict())

			if !tt.strictDiffers {
				if lenientWarning != strictWarning {
					t.Fatalf("warning differs under strictness: %+v vs %+v", lenientWarning, strictWarning)
				}

				if (lenientErr == nil) != (strictErr == nil) {
					t.Fatalf("error differs under strictness: %v vs %v", lenientErr, strictErr)
				}

				if lenientErr != nil && lenientErr.Error() != strictErr.Error() {
					t.Fatalf("error text differs under strictness: %q vs %q", lenientErr, strictErr)
				}

				return
			}

			if lenientErr != nil || lenientWarning == (gatling.Warning{}) {
				t.Fatalf("lenient Apply(%s) = %+v, %v; want a warning and no error", tt.found, lenientWarning, lenientErr)
			}

			var unverified *gatling.UnverifiedError
			if !errors.As(strictErr, &unverified) {
				t.Fatalf("strict Apply(%s) = _, %v; want a *gatling.UnverifiedError", tt.found, strictErr)
			}

			if unverified.Version != tt.found || unverified.Min != corpusPolicy.Min || unverified.Max != corpusPolicy.Max {
				t.Fatalf("strict Apply(%s) = %+v; want the version and both bounds", tt.found, unverified)
			}

			if strictWarning != (gatling.Warning{}) {
				t.Fatalf("strict Apply(%s) = %+v, _; a refused read raises no warning", tt.found, strictWarning)
			}
		})
	}
}

// Too old and too new are opposite evidence gaps, so a caller has to be able to
// tell them apart without reading a message.
func TestRefusalsAreDistinctTypes(t *testing.T) {
	t.Parallel()

	_, _, tooOld := corpusPolicy.Apply(v(3, 10, 5))
	_, _, tooNew := corpusPolicy.Apply(v(3, 13, 0), gatling.WithStrict())

	var versionErr *gatling.VersionError
	if !errors.As(tooOld, &versionErr) {
		t.Fatalf("too old = %v; want a *gatling.VersionError", tooOld)
	}

	if errors.As(tooOld, new(*gatling.UnverifiedError)) {
		t.Fatal("a version below the range must not read as unverified")
	}

	var unverified *gatling.UnverifiedError
	if !errors.As(tooNew, &unverified) {
		t.Fatalf("too new = %v; want a *gatling.UnverifiedError", tooNew)
	}

	if errors.As(tooNew, new(*gatling.VersionError)) {
		t.Fatal("a strict refusal above the range must not read as a version-below-range error")
	}

	if !strings.Contains(unverified.Error(), "strict") {
		t.Fatalf("UnverifiedError = %q; it must say the read was strict, or it is indistinguishable "+
			"from the warning a lenient read raises", unverified)
	}
}
