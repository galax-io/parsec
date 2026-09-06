package binary_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
)

// The name the simulation declares. It is Cyrillic on purpose: those characters
// cannot be encoded in Latin-1, so Gatling stores the string as UTF-16 with the
// other coder byte, and a decoder that ignores the marker returns plausible
// mojibake rather than an error. A wrong name groups two requests together or
// splits one in two, and the report is then confidently wrong.
const cyrillicName = "Проверка /ok"

// Byte for byte against what the simulation declared, in every recording. This
// is the only check that would catch a decoder that read the coder byte, chose
// the right branch, and still got the bytes out in the wrong order.
func TestTheCyrillicNameSurvivesEveryRecording(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			var seen int

			for _, rec := range records(t, openCorpus(t, dir)) {
				if rec.Kind != gatling.KindRequest || rec.Name != cyrillicName {
					continue
				}

				seen++
			}

			if seen != 6 {
				t.Fatalf("%d requests named %q; the probe makes six, one per user. "+
					"A different count means the name decoded to something else",
					seen, cyrillicName)
			}
		})
	}
}

// Both encodings in one log, because the coder is a property of each string and
// not of the file. A decoder that picked one encoding per log would pass a
// single-alphabet corpus and fail every real run in a non-English team.
func TestOneLogHoldsBothEncodings(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			var ascii, wide int

			for _, rec := range records(t, openCorpus(t, dir)) {
				if rec.Kind != gatling.KindRequest {
					continue
				}

				if len(rec.Name) == utf8.RuneCountInString(rec.Name) {
					ascii++
				} else {
					wide++
				}
			}

			if ascii == 0 || wide == 0 {
				t.Fatalf("the log holds %d one-byte-per-character names and %d wider ones; "+
					"both encodings must appear or this proves nothing", ascii, wide)
			}
		})
	}
}

// Every name in every recording must be valid UTF-8. A decoder that mishandled
// a surrogate pair, or read UTF-16 in the wrong byte order, would produce
// something that is not.
func TestEveryDecodedStringIsValidUTF8(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			for i, rec := range records(t, openCorpus(t, dir)) {
				for _, s := range append([]string{rec.Name, rec.Message, rec.Scenario}, rec.Groups...) {
					if !utf8.ValidString(s) {
						t.Fatalf("record %d holds %q, which is not valid UTF-8", i, s)
					}
				}
			}
		})
	}
}

// The string cache is what makes a soak log small, and the mechanism that makes
// a wrong decoder wrong everywhere rather than once. This pins the ratio the
// recordings actually have: a handful of distinct strings carrying a hundred
// records, so the table is exercised as a table and not as a list.
func TestNamesAreRepeatedFarMoreOftenThanTheyAreIntroduced(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			distinct := map[string]int{}

			var total int

			for _, rec := range records(t, openCorpus(t, dir)) {
				if rec.Kind != gatling.KindRequest {
					continue
				}

				distinct[rec.Name]++
				total++
			}

			// One name carries most of the log: 12 of a user's 17 requests —
			// one at the root and eleven inside the outer group — and it is
			// written into the file once.
			if distinct["GET /ok"] != 72 {
				t.Errorf("%q appears %d times; the probe makes it 72", "GET /ok", distinct["GET /ok"])
			}

			if len(distinct) > 8 || total < 100 {
				t.Fatalf("%d requests across %d distinct names; the ratio is what exercises the cache",
					total, len(distinct))
			}
		})
	}
}

// The JVM stores every character below U+0100 in one byte, so a name like
// `GET /café` reaches the decoder as Latin-1 with a byte above 0x7f — a path no
// recording in this corpus takes, because the only non-ASCII name it holds is
// Cyrillic and therefore UTF-16. Without this, the branch ships unexecuted in a
// package with a 90% coverage floor.
func TestLatin1AboveASCIIRoundTrips(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"GET /café", "requête", "ÿ", "àÿé"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			log := (&builder{}).
				runRecord("3.15.1", []string{"s"}, nil).
				request(name, true).
				bytes()

			recs := records(t, bytes.NewReader(log))
			if len(recs) != 1 {
				t.Fatalf("%d records; want 1", len(recs))
			}

			if recs[0].Name != name {
				t.Fatalf("decoded %q; want %q", recs[0].Name, name)
			}
		})
	}
}

// The assertion payloads are the one part of the grammar the fixture builder can
// write and nothing round-trips: every other test passes nil assertions, so a
// disagreement between the builder and readBlobs would go unnoticed until a
// recording disagreed with both.
func TestAssertionPayloadsRoundTripThroughTheBuilder(t *testing.T) {
	t.Parallel()

	want := []string{"", "\x00\x01\x02", strings.Repeat("payload", 100)}

	log := (&builder{}).runRecord("3.15.1", []string{"s"}, want).bytes()

	rd, err := binary.NewReader(bytes.NewReader(log))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	got := rd.Assertions()
	if len(got) != len(want) {
		t.Fatalf("%d payloads; want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("payload %d = %q; want %q", i, got[i], want[i])
		}
	}
}
