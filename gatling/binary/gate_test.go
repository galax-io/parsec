package binary_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
)

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

// The version is the first field of the run record and the gate is the
// cheapest decision the codec makes, so it is taken before anything after the
// version is decoded: a log naming a version below the range, or no release at
// all, is refused as such whatever its scenario or assertion tables hold.
// Before this the whole run record was read first, so a corrupt count after an
// out-of-range version was reported as damage by this codec and as a version by
// the text one — and simlog exists so a consumer cannot tell the two apart.
func TestAVersionBelowTheRangeIsRefusedBeforeTheScenarioCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		parsed  bool
	}{
		{"below the range", "1.0.0", true},
		{"not a release", "3.x", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// A scenario count of -1: the first field after the version that
			// cannot be right.
			log := (&builder{}).u8(0).str(tt.version).str("io.example.Sim").i64(runStart).str("").i32(-1).bytes()

			_, err := binary.NewReader(bytes.NewReader(log))

			var ve *gatling.VersionError
			if !errors.As(err, &ve) {
				t.Fatalf("NewReader = _, %v; want a *gatling.VersionError whatever follows the version", err)
			}

			if ve.Found != tt.version || ve.Parsed != tt.parsed {
				t.Fatalf("the refusal names %q (parsed %t); want %q (parsed %t)", ve.Found, ve.Parsed, tt.version, tt.parsed)
			}
		})
	}
}

// refusedLog is a run record naming a version below the range, with the given
// number of scenario names of about fifty bytes each, followed by enough bytes
// that the first buffer fill is a full one from either log.
func refusedLog(names int) []byte {
	scenarios := make([]string, names)
	for i := range scenarios {
		scenarios[i] = fmt.Sprintf("scenario-%040d", i)
	}

	return append((&builder{}).runRecord("1.0.0", scenarios, nil).bytes(), make([]byte, 2*binary.ReadBufferSize)...)
}

// A refusal is the cheapest decision the codec makes, and it costs one read:
// the bytes pulled from the source before a version below the range is refused
// do not depend on what the log's scenario or assertion tables claim. Before
// this the tables were read, and retained, up to their ceilings first.
func TestARefusedVersionPullsAtMostOneBufferFill(t *testing.T) {
	t.Parallel()

	pulled := func(names int) int64 {
		src := bytes.NewReader(refusedLog(names))

		_, err := binary.NewReader(src)
		if !errors.As(err, new(*gatling.VersionError)) {
			t.Fatalf("NewReader over a log with %d names = _, %v; want a *gatling.VersionError", names, err)
		}

		return src.Size() - int64(src.Len())
	}

	few, many := pulled(3), pulled(4096)

	if few != many || few > binary.ReadBufferSize {
		t.Fatalf("a refused log with three scenario names pulled %d bytes and one with 4096 pulled %d; "+
			"want the same figure, at most the %d-byte read buffer", few, many, binary.ReadBufferSize)
	}
}

// The version is the one field read before the gate rules, so its length is
// the one thing that could make a refusal cost more than a buffer fill. A
// release is a handful of characters: a length at the ceiling is still read
// and judged, and one past it is refused before the bytes behind it are read,
// with a message that quotes no megabyte.
func TestAnOverlongVersionPullsAtMostOneBufferFill(t *testing.T) {
	t.Parallel()

	atCeiling := (&builder{}).runRecord(strings.Repeat("9", 64), []string{"s"}, nil).bytes()
	if _, err := binary.NewReader(bytes.NewReader(atCeiling)); !errors.As(err, new(*gatling.VersionError)) {
		t.Fatalf("a 64-byte version = %v; want it read and refused as not a release", err)
	}

	w := (&builder{}).u8(0).i32(1 << 20)
	w.b = append(w.b, make([]byte, 1<<20+1+2*binary.ReadBufferSize)...)

	src := bytes.NewReader(w.bytes())

	_, err := binary.NewReader(src)

	var se *gatling.SyntaxError
	if !errors.As(err, &se) || se.Offset != 1 {
		t.Fatalf("a version length of 1 MiB = %v; want a *gatling.SyntaxError at byte 1", err)
	}

	if pulled := src.Size() - int64(src.Len()); pulled > binary.ReadBufferSize || len(err.Error()) > 200 {
		t.Fatalf("the refusal pulled %d bytes and runs to %d bytes of message; want at most one %d-byte fill and a short message",
			pulled, len(err.Error()), binary.ReadBufferSize)
	}
}
