package binary_test

import (
	"path/filepath"
	"testing"

	"github.com/galax-io/parsec/gatling"
)

// The counts folded from the decoded records must equal what that run's own
// Gatling reported for itself, exactly. The tolerance is zero: these are counts
// of discrete events, not measurements, and a decoder that loses or invents one
// is wrong rather than imprecise.
//
// The folding happens here, in the test. This module computes no count, mean,
// percentile, range or series — it owns what a failure is and where a run begins,
// and a consumer owns the arithmetic.
func TestDecodedCountsMatchWhatTheRunReported(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			var got counts

			for _, rec := range records(t, openCorpus(t, dir)) {
				if rec.Kind != gatling.KindRequest {
					continue
				}

				got.total++

				switch rec.Status {
				case gatling.StatusOK:
					got.ok++
				case gatling.StatusKO:
					got.ko++
				case gatling.StatusUnknown:
					t.Fatalf("a request decoded with no outcome: the format writes a boolean, "+
						"so there is no third state to reach — %+v", rec)
				}
			}

			accounts := runAccounts(t, dir)
			if len(accounts) == 0 {
				t.Fatal("the recording carries no account of its own numbers, so it proves nothing")
			}

			for source, want := range accounts {
				if got != want {
					t.Errorf("decoded %d requests (%d ok, %d ko); %s says %d (%d ok, %d ko)",
						got.total, got.ok, got.ko, source, want.total, want.ok, want.ko)
				}
			}
		})
	}
}

// Neither the report nor the console carries a count of virtual users or of
// error records, so those cannot be checked against Gatling's own account. They
// are pinned against the recorded record stream instead, and this says so out
// loud rather than leaving the gap unremarked.
func TestCountsGatlingDoesNotReportArePinnedHere(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			byKind := map[gatling.Kind]int{}
			for _, rec := range records(t, openCorpus(t, dir)) {
				byKind[rec.Kind]++
			}

			// Six users, each starting and ending one scenario; one crash per
			// user from the request whose URL cannot be built; two groups closed
			// per user.
			want := map[gatling.Kind]int{
				gatling.KindUser:    12,
				gatling.KindError:   6,
				gatling.KindGroup:   12,
				gatling.KindRequest: 102,
			}

			for kind, n := range want {
				if byKind[kind] != n {
					t.Errorf("%s records: %d; want %d", kind, byKind[kind], n)
				}
			}

			if byKind[gatling.KindRun] != 0 {
				t.Errorf("%d run records reached the event stream; the header is not an event",
					byKind[gatling.KindRun])
			}
		})
	}
}
