package simlog_test

import (
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
	"github.com/galax-io/parsec/gatling/simlog"
	"github.com/galax-io/parsec/gatling/text"
)

// The advertised range is derived from the codec, never restated here. That is
// what makes this test fail if the corpus is widened without the gate, and if
// the gate is widened without the corpus.
func TestSupportedTextRangeComesFromTheCodec(t *testing.T) {
	t.Parallel()

	oldest, newest := text.SupportedVersions()

	var found bool

	for _, s := range simlog.Supported() {
		if s.Format != gatling.FormatText {
			continue
		}

		found = true

		if !s.Readable {
			t.Fatal("the text format is readable; this module ships its codec")
		}

		if s.Oldest != oldest || s.Newest != newest {
			t.Fatalf("Supported() advertises %s through %s; the codec accepts %s through %s",
				s.Oldest, s.Newest, oldest, newest)
		}
	}

	if !found {
		t.Fatal("Supported() does not mention the text format")
	}
}

// The reported range must be the codec's own, read from the same table dispatch
// uses. A hard-coded range here goes stale the first time a codec widens one,
// and nobody notices until a user is told their supported log is unsupported.
func TestSupportedReportsBinaryOverTheRangeItsCorpusCovers(t *testing.T) {
	t.Parallel()

	var found bool

	oldest, newest := binary.SupportedVersions()

	for _, s := range simlog.Supported() {
		if s.Format != gatling.FormatBinary {
			continue
		}

		found = true

		if !s.Readable {
			t.Fatal("the binary format is reported unreadable, but this module has a codec for it")
		}

		if s.Oldest != oldest || s.Newest != newest {
			t.Fatalf("Supported() reports %s through %s; the codec accepts %s through %s",
				s.Oldest, s.Newest, oldest, newest)
		}
	}

	if !found {
		t.Fatal("Supported() omits the binary format; a caller handed one would be told nothing")
	}
}

// The order is part of the contract: a consumer rendering a table needs it
// stable across releases.
func TestSupportedOrderIsStable(t *testing.T) {
	t.Parallel()

	got := simlog.Supported()

	want := []gatling.Format{gatling.FormatText, gatling.FormatBinary}
	if len(got) != len(want) {
		t.Fatalf("Supported() has %d entries; want %d", len(got), len(want))
	}

	for i, format := range want {
		if got[i].Format != format {
			t.Fatalf("Supported()[%d] is %v; want %v", i, got[i].Format, format)
		}
	}
}

// Every format this module can name must be reportable, or a caller handed one
// is told "unknown" about a format the module knows perfectly well.
//
// The set is walked out of the enum rather than restated here. Restating it is
// what the previous version of this test did, which made it a comparison of a
// literal against a copy of itself: adding a third format and teaching Detect
// to return it would have left the test green and Supported() silent about it.
func TestSupportedCoversEveryKnownFormat(t *testing.T) {
	t.Parallel()

	var known []gatling.Format

	// Format.String() names every value it knows and falls back to "Format(n)"
	// for the rest, so the first fallback is the end of the enum.
	for f := gatling.FormatUnknown + 1; !strings.HasPrefix(f.String(), "Format("); f++ {
		known = append(known, f)
	}

	if len(known) < 2 {
		t.Fatalf("walked %d formats; the enum has at least text and binary", len(known))
	}

	reported := make(map[gatling.Format]bool, len(known))
	for _, s := range simlog.Supported() {
		reported[s.Format] = true
	}

	for _, f := range known {
		if !reported[f] {
			t.Fatalf("gatling names the format %v but Supported() never mentions it, "+
				"so a caller handed one would be told nothing about it", f)
		}
	}
}

// A caller cannot widen a range by mutating what it was handed.
func TestSupportedIsNotACallersToChange(t *testing.T) {
	t.Parallel()

	first := simlog.Supported()
	first[0].Newest = gatling.Version{Major: 9, Minor: 9, Patch: 9}
	first[1].Oldest = gatling.Version{Major: 1}

	second := simlog.Supported()

	_, newest := text.SupportedVersions()
	if second[0].Newest != newest {
		t.Fatalf("Supported() came back mutated: %s; the codec says %s", second[0].Newest, newest)
	}

	oldest, _ := binary.SupportedVersions()
	if second[1].Oldest != oldest {
		t.Fatalf("Supported() came back mutated: %s; the codec says %s", second[1].Oldest, oldest)
	}
}
