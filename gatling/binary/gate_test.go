package binary_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
)

// The format is undocumented and has already changed once. Every outcome the
// gate can reach has a check here, so a codec cannot quietly start accepting a
// version nothing was ever recorded for.
func TestEveryVersionOutcome(t *testing.T) {
	t.Parallel()

	oldest, newest := binary.SupportedVersions()

	tests := []struct {
		name    string
		version string
		wantErr func(error) bool
		warns   int
		says    []string
	}{
		{
			name:    "below the range is refused, naming what it found and what is covered",
			version: "3.12.0",
			wantErr: func(err error) bool { return errors.As(err, new(*gatling.VersionError)) },
			says:    []string{"3.12.0", oldest.String(), newest.String()},
		},
		{
			name:    "the floor itself is accepted",
			version: oldest.String(),
		},
		{
			name:    "inside the range is accepted with nothing to say",
			version: "3.14.9",
		},
		{
			name:    "the ceiling itself is accepted",
			version: newest.String(),
		},
		{
			name:    "above the range decodes, and says so exactly once",
			version: "3.16.0",
			warns:   1,
		},
		{
			name:    "a version that is not a plain release is refused, quoting it",
			version: "3.14.0-SNAPSHOT",
			wantErr: func(err error) bool { return errors.As(err, new(*gatling.VersionError)) },
			says:    []string{"3.14.0-SNAPSHOT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rd, err := binary.NewReader(bytes.NewReader(minimal(tt.version)))

			if tt.wantErr != nil {
				if !tt.wantErr(err) {
					t.Fatalf("NewReader(%s) = _, %v; want the refusal this case names", tt.version, err)
				}

				if rd != nil {
					t.Error("a refused read handed back a reader")
				}

				for _, want := range tt.says {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("the error says %q; it must mention %q", err, want)
					}
				}

				return
			}

			if err != nil {
				t.Fatalf("NewReader(%s) = _, %v; want it accepted", tt.version, err)
			}

			if got := len(rd.Warnings()); got != tt.warns {
				t.Fatalf("%d warnings; want %d", got, tt.warns)
			}

			for _, w := range rd.Warnings() {
				if !strings.Contains(w.String(), tt.version) {
					t.Errorf("the warning says %q; it must name the version found", w.String())
				}
			}
		})
	}
}

// A caller that cannot use an unverified number asks for strictness and gets a
// refusal instead of a warning. Nothing else about the read changes.
func TestStrictTurnsTheWarningIntoARefusal(t *testing.T) {
	t.Parallel()

	log := minimal("3.16.0")

	lenient, err := binary.NewReader(bytes.NewReader(log))
	if err != nil {
		t.Fatalf("without strictness: %v", err)
	}

	if len(lenient.Warnings()) != 1 {
		t.Fatalf("%d warnings without strictness; want 1", len(lenient.Warnings()))
	}

	rd, err := binary.NewReader(bytes.NewReader(log), gatling.WithStrict())
	if rd != nil {
		t.Error("a refused read handed back a reader")
	}

	var unverified *gatling.UnverifiedError
	if !errors.As(err, &unverified) {
		t.Fatalf("with strictness = %v; want a *gatling.UnverifiedError", err)
	}

	// Strictness must not touch a version the corpus does cover.
	if _, err := binary.NewReader(bytes.NewReader(minimal("3.14.9")), gatling.WithStrict()); err != nil {
		t.Fatalf("a covered version under strictness: %v", err)
	}
}

// Principle II binds the gate to the evidence: a codec's supported range MUST
// equal the range covered by its golden corpus. This is that rule as a test, and
// it is what makes every other version test in this file mean something — they
// name versions relative to the range, so something has to pin the range to the
// recordings rather than to itself.
func TestSupportedRangeEqualsTheCorpus(t *testing.T) {
	t.Parallel()

	var recorded []gatling.Version

	for _, dir := range corpusDirs(t) {
		v, err := gatling.ParseVersion(filepath.Base(dir))
		if err != nil {
			t.Fatalf("corpus entry %q is not a version directory: %v", dir, err)
		}

		if _, err := os.Stat(filepath.Join(dir, "records.golden")); err != nil {
			t.Fatalf("%s has no records.golden: an entry nothing is compared against proves nothing", dir)
		}

		recorded = append(recorded, v)
	}

	sort.Slice(recorded, func(i, j int) bool { return recorded[i].Compare(recorded[j]) < 0 })

	oldest, newest := binary.SupportedVersions()

	if oldest != recorded[0] || newest != recorded[len(recorded)-1] {
		t.Fatalf("the codec accepts %s through %s; the corpus covers %s through %s. "+
			"Widening the gate means recording a new corpus entry first (Principle II)",
			oldest, newest, recorded[0], recorded[len(recorded)-1])
	}
}

// The gate runs before any record is decoded, so a refusal never arrives after
// records have been handed out. A caller that got three records and then a
// version error would have no way to know the three were meaningless.
func TestARefusedVersionYieldsNoRecords(t *testing.T) {
	t.Parallel()

	log := (&builder{}).runRecord("3.12.0", []string{"scenario"}, nil).request("GET /ok", true).bytes()

	rd, err := binary.NewReader(bytes.NewReader(log))
	if err == nil {
		t.Fatal("a version below the range was accepted")
	}

	if rd != nil {
		t.Fatal("a refused read handed back a reader, which could then yield records")
	}
}
