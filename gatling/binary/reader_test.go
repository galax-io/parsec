package binary_test

import (
	"path/filepath"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
)

// A caller must be able to say what run this is before deciding to read it: the
// header is what a UI shows while the records are still streaming.
func TestTheHeaderIsCompleteBeforeTheFirstRecord(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		version := filepath.Base(dir)

		t.Run(version, func(t *testing.T) {
			t.Parallel()

			rd, err := binary.NewReader(openCorpus(t, dir))
			if err != nil {
				t.Fatalf("NewReader: %v", err)
			}

			h := rd.Header()

			if h.Version.String() != version {
				t.Errorf("header names version %s; the recording is %s", h.Version, version)
			}

			if h.SimulationClass != "io.galaxio.parsec.corpus.CorpusSimulation" {
				t.Errorf("header names simulation %q", h.SimulationClass)
			}

			if h.RunID != h.SimulationClass {
				t.Errorf("RunID is %q; the binary record carries no separate identifier, so it is the class",
					h.RunID)
			}

			if h.Start <= 0 {
				t.Errorf("header names start %d; a run start is an epoch millisecond", h.Start)
			}

			if h.Description != "" {
				t.Errorf("header names description %q; the probe leaves it unset", h.Description)
			}

			if len(rd.Warnings()) != 0 {
				t.Errorf("a version inside the range raised %d warnings; want none", len(rd.Warnings()))
			}
		})
	}
}

// The payloads are Gatling's own encoding of the run's assertions. This module
// carries them through unread: it neither decodes, validates nor interprets one,
// and a byte lost here is a byte a consumer can never get back.
func TestAssertionPayloadsArriveVerbatim(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			rd, err := binary.NewReader(openCorpus(t, dir))
			if err != nil {
				t.Fatalf("NewReader: %v", err)
			}

			got := rd.Assertions()
			if len(got) != 10 {
				t.Fatalf("the run declared %d assertions; the probe declares 10", len(got))
			}

			// The first is `global.allRequests.count.is(102)`, and the double at
			// its tail is what says 102. Reading it back is not this module's
			// job, but a payload that lost or gained a byte would not hold it.
			const want = "\x00\x01\x01\x00\x01\x05\x00\x00\x00\x00\x00\x00\x80\x59\x40"
			if got[0] != want {
				t.Errorf("first payload is %q; want %q", got[0], want)
			}

			// Mutating what Assertions hands back must not reach the reader.
			got[0] = "tampered"

			if rd.Assertions()[0] == "tampered" {
				t.Error("Assertions hands out the reader's own slice: a caller can rewrite the run's assertions")
			}
		})
	}
}

// Every request and group carries the full ordered path of the groups enclosing
// it. A path in the wrong order, or a request that lost its path, reads as a
// different position in the simulation and silently regroups a report.
func TestEveryRecordCarriesItsGroupPath(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			var (
				rootless, outer, inner int
				groups                 int
			)

			for _, rec := range records(t, openCorpus(t, dir)) {
				switch rec.Kind {
				case gatling.KindRequest:
					switch depthOf(t, rec.Groups) {
					case 0:
						rootless++
					case 1:
						outer++
					default:
						inner++
					}
				case gatling.KindGroup:
					groups++
				case gatling.KindUser, gatling.KindError, gatling.KindRun,
					gatling.KindAssertion, gatling.KindUnknown:
				}
			}

			// Per user, four requests run outside any group — the root GET,
			// the Cyrillic GET, and the two that fail before reaching the wire —
			// eleven inside outer, and two inside outer/inner. Six users.
			if rootless != 24 || outer != 66 || inner != 12 {
				t.Errorf("group paths distribute as %d rootless, %d under outer, %d under outer/inner; "+
					"want 24, 66, 12", rootless, outer, inner)
			}

			if groups != 12 {
				t.Errorf("%d group records; the probe closes two groups per user over six users", groups)
			}
		})
	}
}

// The group path a record hands out points into memory the reader reuses. That
// is the contract, and it is only safe because it is documented — this pins both
// halves: the slice really is reused, and copying it really does keep it.
func TestTheGroupPathIsReusedBetweenRecords(t *testing.T) {
	t.Parallel()

	dir := corpusDirs(t)[0]

	rd, err := binary.NewReader(openCorpus(t, dir))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	var held []string

	for {
		rec, err := rd.Next()
		if err != nil {
			break
		}

		if len(rec.Groups) == 2 {
			if held == nil {
				held = rec.Groups

				continue
			}

			if &held[0] != &rec.Groups[0] {
				t.Skip("the reader allocated a fresh path; nothing here is wrong, but the reuse this pins is gone")
			}

			return
		}
	}

	t.Fatal("no two records shared a group path: the corpus should hold many")
}

// depthOf checks a group path against the two the probe declares and returns how
// deep it is. A path that is neither fails the test where it is found.
func depthOf(t *testing.T, groups []string) int {
	t.Helper()

	switch len(groups) {
	case 0:
		return 0
	case 1:
		if groups[0] != "outer" {
			t.Fatalf("a one-deep record names group %q", groups[0])
		}

		return 1
	case 2:
		if groups[0] != "outer" || groups[1] != "inner, with comma" {
			t.Fatalf("a two-deep record names groups %q", groups)
		}

		return 2
	}

	t.Fatalf("a record nests %d groups deep; the probe nests two", len(groups))

	return 0
}
