//go:build integration

package binary_test

import (
	"errors"
	"io"
	"runtime"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
	"github.com/galax-io/parsec/model"
)

const mib = 1 << 20

// samples is how many times a run looks at the heap, whatever its size. A peak
// is a max over the draws taken, so more draws find a higher one: sampling at a
// fixed byte interval would give a ten-times-larger log ten times the chances to
// catch a collector trough, and the two peaks would not be comparable — which is
// the comparison the test exists to make.
const samples = 64

// budget is the peak heap this package documents for a decode of any size.
const budget = 32 * mib

// drift is how far the live heap may move between a run and one ten times
// longer. SC-004 says the figure does not change; for a sampled measurement
// this is what "does not change" has to mean, and it is set well above the
// spread the sampler actually shows.
//
// Sampling after a collection leaves almost no spread to allow for. Eighteen
// pairs were measured for this change — three idle and fifteen under sixteen
// spinning goroutines at GOMAXPROCS=2 — and every pair came back equal: 0.5
// against 0.5 MiB idle here, 0.3 against 0.3 loaded, and 2.5 and 2.3 in
// gatling/text. The unswept figure this replaced moved 3.8 to 23.0 MiB under
// that same load, which is why it could not be compared against itself.
//
// A leak that grew with the log would show as a multiple of the figure, not as
// a fraction of a MiB, so a megabyte is loose enough never to fire on noise and
// tight enough that nothing real hides under it.
const drift = 1 * mib

// sampler records the highest live heap seen while the input flows through it.
type sampler struct {
	r     io.Reader
	every int64
	since int64
	peak  uint64
}

func (s *sampler) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	s.since += int64(n)

	if s.since >= s.every {
		s.since = 0
		s.peak = max(s.peak, liveHeap())
	}

	return n, err
}

// liveHeap collects, then reports what is still held.
//
// The collection is the measurement. HeapAlloc on its own counts what has been
// allocated and not yet swept, so it reads the collector's backlog as if it
// were retention: against a fixed live set it moves 2.0x when the machine is
// idle and 8.4x under contention, while the same set sampled after a
// collection moves 1.03x and 1.05x. A ten-times-longer run gives the collector
// ten times as many chances to fall behind, which is why the unswept figure
// grows with the log while the live heap does not — the growth a bare peak
// comparison then reports as a leak.
//
// It costs about 280us against the 29us of the read alone, taken `samples`
// times a run: under 20ms, against a decode measured in seconds.
func liveHeap() uint64 {
	var m runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&m)

	return m.HeapAlloc
}

func decodeSized(t *testing.T, size int64) (records int64, peak uint64) {
	t.Helper()

	n, peak, err := decodeStream(t, newSynthLog(size), max(size/samples, 1))
	if err != nil {
		t.Fatalf("after %d records: %v", n, err)
	}

	return int64(n), peak
}

// The reader's memory must not grow with the log. A 256 MiB log and one ten
// times larger both stay inside the budget, and the live heap of the larger is
// the live heap of the smaller, within drift. Growth proportional to the input
// would show as ten times the figure and fails both bounds outright.
//
// This is the format's own wrinkle: the string table is held for the whole read,
// so "bounded" has to mean bounded by the *simulation* and not by the *run*. The
// synthetic log introduces its names once and refers back for the rest, which is
// exactly the shape a soak run has.
//
//nolint:paralleltest // measures peak heap and must run alone
func TestPeakMemory(t *testing.T) {
	const small = 256 * mib

	n1, peak1 := decodeSized(t, small)
	t.Logf("%d MiB: %d records, peak heap %.1f MiB", small/mib, n1, float64(peak1)/mib)

	if peak1 >= budget {
		t.Fatalf("peak heap %.1f MiB for a %d MiB log, want under %d MiB", float64(peak1)/mib, small/mib, budget/mib)
	}

	if testing.Short() {
		t.Skip("the ten-times-larger run is skipped under -short")
	}

	n2, peak2 := decodeSized(t, 10*small)
	t.Logf("%d MiB: %d records, peak heap %.1f MiB", 10*small/mib, n2, float64(peak2)/mib)

	if peak2 >= budget || peak2 > peak1+drift {
		t.Fatalf("peak heap grew from %.1f MiB to %.1f MiB when the log grew ten times",
			float64(peak1)/mib, float64(peak2)/mib)
	}
}

// Memory must follow the number of distinct names, not the number of records.
// That is the format's own wrinkle: the string table is held for the whole read,
// so a decoder could be bounded in every other way and still grow with a run
// that keeps inventing names.
//
// The synthetic log introduces four names and then refers back for millions of
// records, which is the shape a soak run has. Ten times the records must cost
// the same table.
//
// The name ends in PeakMemory for the reason given on TestStringCeilingPeakMemory:
// CI selects this class with an anchored `-run 'PeakMemory$'` and runs it without
// the race detector or coverage, both of which move the figure being asserted.
// Under the older name this test missed that pattern and was measured instrumented,
// where a raced run takes thirteen times as long and so gives the collector
// thirteen times as many chances to be caught mid-sweep.
//
//nolint:paralleltest // measures peak heap and must run alone
func TestNamesNotRecordsPeakMemory(t *testing.T) {
	n1, peak1 := decodeSized(t, 16*mib)
	n2, peak2 := decodeSized(t, 160*mib)

	if n2 < 9*n1 {
		t.Fatalf("the larger log yielded %d records and the smaller %d; the comparison needs ten times", n2, n1)
	}

	t.Logf("%d records: %.1f MiB; %d records: %.1f MiB — the same four names throughout",
		n1, float64(peak1)/mib, n2, float64(peak2)/mib)

	// Two separate readings. This one is the budget the package documents, and
	// it applies to both figures; the one below is about the table's shape. A
	// breach of either deserves its own message rather than the other's.
	if peak1 >= budget || peak2 >= budget {
		t.Fatalf("peak heap %.1f MiB and %.1f MiB for %d and %d records, want under the %d MiB this package documents",
			float64(peak1)/mib, float64(peak2)/mib, n1, n2, budget/mib)
	}

	if peak2 > peak1+drift {
		t.Fatalf("ten times the records cost %.1f MiB against %.1f MiB; the table is growing with the run",
			float64(peak2)/mib, float64(peak1)/mib)
	}
}

// foldSized reads a synthetic log through the model-facing reader the way a
// consumer's pass does: a position taken for every sample and group, one Bounds
// extended for every item. Positions are dropped as they are taken — a consumer
// keeps one per distinct place, and the synthetic log has four.
func foldSized(t *testing.T, size int64) (items int64, peak uint64) {
	t.Helper()

	runtime.GC()

	src := &sampler{r: newSynthLog(size), every: max(size/samples, 1)}

	rd, err := binary.NewRunReader(src)
	if err != nil {
		t.Fatalf("NewRunReader: %v", err)
	}

	var (
		bounds model.Bounds
		last   model.Position
	)

	for {
		it, err := rd.Next()
		if errors.Is(err, io.EOF) {
			if _, ok := bounds.End(); !ok || last == (model.Position{}) {
				t.Fatal("the fold bounded nothing or took no position; the log is not the shape this test assumes")
			}

			return items, max(src.peak, liveHeap())
		}

		if err != nil {
			t.Fatalf("Next after %d items: %v", items, err)
		}

		bounds.Extend(&it)

		switch it.Kind {
		case model.ItemSample:
			last = it.Sample.Position()
		case model.ItemGroup:
			last = it.Group.Position()
		case model.ItemUser, model.ItemError, model.ItemAssertion, model.ItemUnknown:
		}

		items++
	}
}

// The primitives retain nothing: folding a log through them peaks where decoding
// it does, and a tenfold longer log does not move the figure.
//
//nolint:paralleltest // measures peak heap and must run alone
func TestFoldPeakMemory(t *testing.T) {
	const small = 256 * mib

	n1, peak1 := foldSized(t, small)
	t.Logf("%d MiB: %d items, peak heap %.1f MiB", small/mib, n1, float64(peak1)/mib)

	if peak1 >= budget {
		t.Fatalf("peak heap %.1f MiB for a %d MiB log, want under %d MiB", float64(peak1)/mib, small/mib, budget/mib)
	}

	if testing.Short() {
		t.Skip("the ten-times-larger run is skipped under -short")
	}

	n2, peak2 := foldSized(t, 10*small)
	t.Logf("%d MiB: %d items, peak heap %.1f MiB", 10*small/mib, n2, float64(peak2)/mib)

	if peak2 >= budget || peak2 > peak1+drift {
		t.Fatalf("peak heap grew from %.1f MiB to %.1f MiB when the log grew ten times",
			float64(peak1)/mib, float64(peak2)/mib)
	}
}

// peakOf decodes a whole log and reports the highest heap seen, failing the
// test if the log does not decode.
func peakOf(t *testing.T, src io.Reader, every int64) (records int, peak uint64) {
	t.Helper()

	records, peak, err := decodeStream(t, src, every)
	if err != nil {
		t.Fatalf("after %d records: %v", records, err)
	}

	return records, peak
}

// The budget Reader documents must hold for the field the ceiling is written
// for.
//
// TestPeakMemory measures a log whose longest name is seven ASCII bytes, so the
// claim it proves is about that run's shape and not about the format's. A single
// field at MaxStringLen costs a multiple of the ceiling — the read buffer, then
// the result, and for Latin-1 above ASCII twice the result — and a log carrying
// one of each encoding costs the sum, because the garbage from one is not
// collected before the next is built.
//
// A bound with an unstated exclusion is not a bound, and the excluded shape here
// is the one a long failure message takes.
//
// The name ends in PeakMemory because CI selects this class of test by
// `-run 'PeakMemory$'` and runs it without the race detector, which moves the
// very HeapAlloc figure being asserted. A name this pattern misses lands in the
// raced step instead and measures the detector.
//
//nolint:paralleltest // measures peak heap and must run alone
func TestStringCeilingPeakMemory(t *testing.T) {
	records, peak := peakOf(t, newCeilingLog(), int64(binary.MaxStringLen)/8)

	t.Logf("%d fields at the %d MiB ceiling, one per encoding: peak heap %.1f MiB (%.1f x the ceiling)",
		records, binary.MaxStringLen/mib, float64(peak)/mib, float64(peak)/float64(binary.MaxStringLen))

	if records != len(ceilingFields) {
		t.Fatalf("decoded %d records, want one per encoding (%d)", records, len(ceilingFields))
	}

	if peak >= budget {
		t.Fatalf("peak heap %.1f MiB decoding one field at the %d MiB ceiling in each encoding, "+
			"want under the %d MiB this package documents",
			float64(peak)/mib, binary.MaxStringLen/mib, budget/mib)
	}
}

// The other ceiling: what a run record's assertion payloads come to in total.
//
// They are held for the whole read and copied again by Assertions, so this is
// live memory where a string field is transient. The synthetic log declares no
// assertions, so nothing measured this until now.
//
//nolint:paralleltest // measures peak heap and must run alone
func TestAssertionCeilingPeakMemory(t *testing.T) {
	// Filling the assertion ceiling with payloads at the string ceiling is the
	// worst shape that stays inside both limits — each a header short of
	// MaxStringLen, since the ceiling counts what the heap holds.
	payloads := max(binary.MaxAssertionBytes/binary.MaxStringLen, 1)
	size := binary.MaxStringLen - binary.StringHeader

	_, peak := peakOf(t, newAssertionLog(payloads, size), int64(binary.MaxStringLen)/8)

	t.Logf("%d assertion payloads of %d MiB, %d MiB retained: peak heap %.1f MiB",
		payloads, binary.MaxStringLen/mib, payloads*binary.MaxStringLen/mib, float64(peak)/mib)

	if peak >= budget {
		t.Fatalf("peak heap %.1f MiB reading a run record's assertion payloads, "+
			"want under the %d MiB this package documents", float64(peak)/mib, budget/mib)
	}
}

// decodeStream decodes a log to its end, sampling the live heap every so many
// bytes and once more at the end, and reports how the read ended — a refusal is
// an ending some of these tests expect.
//
// The draw after the loop is not decoration: the largest allocation of a record
// happens while that record is being built, after the reader has stopped pulling
// its bytes, so a sampler wrapped around the input alone can miss it entirely.
func decodeStream(t *testing.T, log io.Reader, every int64) (records int, peak uint64, err error) {
	t.Helper()

	runtime.GC()

	src := &sampler{r: log, every: every}

	rd, err := binary.NewReader(src)
	if err != nil {
		return 0, max(src.peak, liveHeap()), err
	}

	for {
		_, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return records, max(src.peak, liveHeap()), nil
		}

		if err != nil {
			return records, max(src.peak, liveHeap()), err
		}

		records++
	}
}

// refusedWithinBudget requires the read to have ended as damage and the heap
// never to have left the budget on the way.
func refusedWithinBudget(t *testing.T, what string, peak uint64, err error) {
	t.Helper()

	if !errors.As(err, new(*gatling.SyntaxError)) {
		t.Fatalf("%s = %v; want a refusal as damaged", what, err)
	}

	if peak >= budget {
		t.Fatalf("%s: peak heap %.1f MiB, want under the %d MiB this package documents", what, float64(peak)/mib, budget/mib)
	}

	t.Logf("%s: refused — %v; peak heap %.1f MiB", what, err, float64(peak)/mib)
}

// Forty-eight scenario names of MaxStringLen each: 48 MiB that a ceiling on the
// count alone let the reader retain — 49.1 MiB measured — against the 32 MiB it
// documents. Bounded in bytes, the run record is refused as damaged long before
// that, and the heap never leaves the budget.
//
//nolint:paralleltest // measures peak heap and must run alone
func TestDistinctScenarioNamesPeakMemory(t *testing.T) {
	_, peak, err := decodeStream(t, &tablesLog{scenarios: 48, scenarioLen: binary.MaxStringLen}, mib)
	refusedWithinBudget(t, "48 scenario names of MaxStringLen", peak, err)
}

// The same shape built from the string table: forty-eight request records each
// introducing a fresh MaxStringLen name. The table was bounded by entries and
// by nothing else, so a log a little over the budget retained all of it.
//
//nolint:paralleltest // measures peak heap and must run alone
func TestDistinctStringsPeakMemory(t *testing.T) {
	_, peak, err := decodeStream(t, &tablesLog{scenarios: 1, scenarioLen: 1, introduced: 48, introducedLen: binary.MaxStringLen}, mib)
	refusedWithinBudget(t, "48 introduced strings of MaxStringLen", peak, err)
}

// The proof of the figure: every table just under its ceiling at once — 1 MiB of
// scenario names, 8 MiB of assertion payloads, 12 MiB of string table, headers
// counted — followed by a hundred thousand records that refer back to it. The
// log is accepted, and what the reader holds for it stays under the budget,
// which the arithmetic in the plan puts near 26 MiB in the worst case.
//
//nolint:paralleltest // measures peak heap and must run alone
func TestTablesAtTheirCeilingsPeakMemory(t *testing.T) {
	log := &tablesLog{
		scenarios: 16_384, scenarioLen: 48, // 64 bytes each with the header: 1 MiB
		payloads: 8, payloadLen: binary.MaxStringLen - binary.StringHeader, // 1 MiB each with the header: 8 MiB
		introduced: 98_300, introducedLen: 112, // 128 bytes each with the header: just under 12 MiB
		referring: 100_000,
	}

	records, peak, err := decodeStream(t, log, mib)
	if err != nil {
		t.Fatalf("a log with every table just under its ceiling is refused after %d records: %v", records, err)
	}

	t.Logf("%d records: peak live heap %.1f MiB with every table just under its ceiling", records, float64(peak)/mib)

	if peak >= budget {
		t.Fatalf("peak heap %.1f MiB with the tables at their ceilings, want under the %d MiB this package documents",
			float64(peak)/mib, budget/mib)
	}
}
