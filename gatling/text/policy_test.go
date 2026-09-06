package text_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/text"
)

// detectFormat reads just enough of a corpus log to say which Gatling format
// wrote it.
func detectFormat(path string) (gatling.Format, error) {
	f, err := os.Open(path) //nolint:gosec // a corpus path this test spells out itself
	if err != nil {
		return gatling.FormatUnknown, err
	}

	defer f.Close() //nolint:errcheck // read-only

	buf := make([]byte, gatling.DetectSize)

	n, err := f.Read(buf)
	if n == 0 && err != nil {
		return gatling.FormatUnknown, err
	}

	return gatling.Detect(buf[:n])
}

// Principle II binds the gate to the evidence: "a codec's supported range MUST
// equal the range covered by its golden corpus". This is that rule as a test,
// and it is the one the other tests in this file rely on — they name versions
// relative to the range, so something has to pin the range to the recordings
// rather than to itself.
func TestSupportedRangeEqualsTheCorpus(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(filepath.Join("..", "..", "testdata", "corpus", "gatling"))
	if err != nil {
		t.Fatalf("reading the corpus: %v", err)
	}

	var recorded []gatling.Version

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		v, err := gatling.ParseVersion(e.Name())
		if err != nil {
			// The probe project lives here too; only version directories count.
			continue
		}

		log := filepath.Join("..", "..", "testdata", "corpus", "gatling", e.Name(), "simulation.log")
		if _, err := os.Stat(log); err != nil {
			t.Fatalf("%s has no simulation.log: a corpus entry without its log proves nothing", e.Name())
		}

		// The corpus spans both Gatling log formats. This codec's range is bound
		// to the runs *it* can read, so an entry is folded in only when its log
		// is the text one — asked of the recording itself rather than assumed
		// from the version, so that a mis-recorded entry fails here loudly
		// instead of silently widening or narrowing the range.
		format, err := detectFormat(log)
		if err != nil {
			t.Fatalf("detecting the format of %s: %v", log, err)
		}

		if format != gatling.FormatText {
			continue
		}

		recorded = append(recorded, v)
	}

	if len(recorded) == 0 {
		t.Fatal("no recorded run found: the corpus is committed, so this is a broken checkout")
	}

	sort.Slice(recorded, func(i, j int) bool { return recorded[i].Compare(recorded[j]) < 0 })

	oldest, newest := text.SupportedVersions()

	if oldest != recorded[0] || newest != recorded[len(recorded)-1] {
		t.Fatalf("the codec accepts %s through %s; the corpus covers %s through %s. "+
			"Widening the gate means recording a new corpus entry first (Principle II)",
			oldest, newest, recorded[0], recorded[len(recorded)-1])
	}
}

// The codec must not decide versions for itself: what a caller sees has to be
// what the shared policy returns, or a second codec can disagree with this one
// while both look correct in isolation.
//
// The expectations are stated here rather than read back from the codec. A test
// that asks text.SupportedVersions() what to expect passes whatever the gate
// says, including a gate that has been widened by mistake — the case
// TestSupportedRangeEqualsTheCorpus exists to catch and this one cannot.
func TestCodecDefersToThePolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		fixture     string
		wantVerdict gatling.Verdict
		wantWarning bool
		wantErrAs   func(error) bool
	}{
		{
			name:        "below the range",
			fixture:     "version-3.9.0",
			wantVerdict: gatling.VerdictRefused,
			wantErrAs:   func(err error) bool { return errors.As(err, new(*gatling.VersionError)) },
		},
		{name: "the oldest covered version", fixture: "version-3.11.5", wantVerdict: gatling.VerdictAccepted},
		{name: "the newest covered version", fixture: "version-3.12.0", wantVerdict: gatling.VerdictAccepted},
		{
			name:        "above the range",
			fixture:     "version-3.13.0",
			wantVerdict: gatling.VerdictUnverified,
			wantWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, err := text.NewReader(openFixture(t, tt.fixture))

			if tt.wantVerdict == gatling.VerdictRefused {
				if err == nil {
					t.Fatalf("NewReader accepted %s; want a refusal", tt.fixture)
				}

				if !tt.wantErrAs(err) {
					t.Fatalf("NewReader = _, %v (%T); not the error type this case is about", err, err)
				}

				return
			}

			if err != nil {
				t.Fatalf("NewReader(%s) = _, %v; want it accepted", tt.fixture, err)
			}

			got := r.Warnings()

			if !tt.wantWarning {
				if len(got) != 0 {
					t.Fatalf("Warnings() = %+v; a covered version raises none", got)
				}

				return
			}

			if len(got) != 1 {
				t.Fatalf("Warnings() = %+v (%d); want exactly 1", got, len(got))
			}

			// The warning has to name the version and the range, or a report
			// cannot tell a user which release is unverified.
			if got[0].Version == (gatling.Version{}) || got[0].String() == "" {
				t.Fatalf("Warnings()[0] = %+v; want it to name the version and render", got[0])
			}
		})
	}
}

// One log above the range, one warning — counted, not "at least one". Every
// path a caller can take goes through the same single application of the
// policy, so none of them may add a second.
func TestExactlyOneWarningPerRead(t *testing.T) {
	t.Parallel()

	t.Run("NewReader", func(t *testing.T) {
		t.Parallel()

		r, err := text.NewReader(openFixture(t, "version-3.13.0"))
		if err != nil {
			t.Fatalf("NewReader = _, %v", err)
		}

		if got := r.Warnings(); len(got) != 1 {
			t.Fatalf("Warnings() = %+v (%d); want exactly 1", got, len(got))
		}

		records := 0

		for {
			if _, err := r.Next(); err != nil {
				if !errors.Is(err, io.EOF) {
					t.Fatalf("Next() = _, %v", err)
				}

				break
			}

			records++
		}

		if records == 0 {
			t.Fatal("the fixture yielded no records, so reading it proved nothing")
		}

		// Reading to the end must not have added another.
		if got := r.Warnings(); len(got) != 1 {
			t.Fatalf("Warnings() after reading = %+v (%d); want exactly 1", got, len(got))
		}
	})

	t.Run("NewRunReader", func(t *testing.T) {
		t.Parallel()

		x, err := text.NewRunReader(openFixture(t, "version-3.13.0"))
		if err != nil {
			t.Fatalf("NewRunReader = _, %v", err)
		}

		if got := x.Run().Warnings; len(got) != 1 {
			t.Fatalf("Run().Warnings = %+v (%d); want exactly 1", got, len(got))
		}
	})
}
