package simlog_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/simlog"
)

// tail is the source a sidecar following a live run hands the reader: bytes
// appear over time, a read with nothing to hand back waits instead of reporting
// an end, and io.EOF arrives only when the follower says the run is over.
//
// It is deliberately not a chunked reader over a complete buffer — that is
// chunk_test.go's subject. What this one adds is *time*: the reader must treat a
// pause as a wait rather than as the end of the log.
type tail struct {
	mu     sync.Mutex
	arrive *sync.Cond
	// drained lets a writer wait for the reader to take what it appended, so a
	// test can hold the log's bytes at a bounded size and measure what the
	// *reader* keeps rather than what the test is still holding.
	drained *sync.Cond
	buf     []byte
	done    bool
}

func newTail() *tail {
	t := &tail{}
	t.arrive = sync.NewCond(&t.mu)
	t.drained = sync.NewCond(&t.mu)

	return t
}

// appendWhenDrained waits until the reader has taken everything appended so far,
// then appends more. It is what a writer flushing into a file the reader keeps
// up with looks like, and it makes the reader wait at every piece boundary.
func (t *tail) appendWhenDrained(b []byte) {
	t.mu.Lock()

	for len(t.buf) > 0 {
		t.drained.Wait()
	}

	t.buf = append(t.buf, b...)
	t.mu.Unlock()
	t.arrive.Broadcast()
}

func (t *tail) close() {
	t.mu.Lock()
	t.done = true
	t.mu.Unlock()
	t.arrive.Broadcast()
}

func (t *tail) Read(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for len(t.buf) == 0 && !t.done {
		t.arrive.Wait()
	}

	if len(t.buf) == 0 {
		return 0, io.EOF
	}

	n := copy(p, t.buf)
	t.buf = t.buf[n:]

	if len(t.buf) == 0 {
		t.drained.Broadcast()
	}

	return n, nil
}

// follow reads a log through the given source and returns the records it
// yielded and the error that ended the read.
func follow(r io.Reader) ([]gatling.Record, error) {
	rd, err := simlog.NewReader(r)
	if err != nil {
		return nil, err
	}

	var recs []gatling.Record

	for {
		rec, err := rd.Next()
		if err != nil {
			return recs, err
		}

		if rec.Groups != nil {
			rec.Groups = append(make([]string, 0, len(rec.Groups)), rec.Groups...)
		}

		recs = append(recs, rec)
	}
}

// writeOverTime appends the log in pieces of the given size from another
// goroutine, then reports the end. Nothing sleeps: the reader blocks in Read
// until the writer appends, which is the ordering under test and is faster and
// steadier than waiting on a clock.
//
// It waits for each piece to be taken before appending the next. Without that
// the goroutine runs free and drains its whole loop before the reader's first
// read, so the reader gets the log in one piece and nothing is ever split — at
// 300 bytes that was every run, which made the tests below assert nothing about
// the property they are named for.
func writeOverTime(src *tail, log []byte, piece int) {
	go func() {
		for off := 0; off < len(log); off += piece {
			src.appendWhenDrained(log[off:min(off+piece, len(log))])
		}

		src.close()
	}()
}

// A log still being written decodes to exactly what the finished file decodes
// to. This is the promise the comet sidecar follows a live run on
// (specs/008-incomplete-record/contracts/blocking-source.md), and it holds
// because the state a follower needs — the binary codec's string cache, the
// group path, the version gate's verdict — already lives across records.
func TestFollowingAGrowingLogMatchesTheFinishedFile(t *testing.T) {
	t.Parallel()

	for _, log := range corpusLogs(t) {
		t.Run(filepath.Base(filepath.Dir(log)), func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(log) //nolint:gosec // a corpus path from the test's own glob
			if err != nil {
				t.Fatal(err)
			}

			want, err := follow(bytes.NewReader(raw))
			if !errors.Is(err, io.EOF) {
				t.Fatalf("the finished file ended with %v, want io.EOF", err)
			}

			// Both sides of the comparison below are the same codec, so a
			// regression that made it yield nothing would satisfy every check
			// that follows. The recording holds records; require them.
			if len(want) == 0 {
				t.Fatal("the finished file yielded no records, so nothing below compares anything")
			}

			// 300 bytes is the sidecar's own figure (comet#3). One byte is the
			// worst case: every record arrives in pieces.
			for _, piece := range []int{300, 1} {
				src := newTail()
				writeOverTime(src, raw, piece)

				got, err := follow(src)
				if !errors.Is(err, io.EOF) {
					t.Fatalf("%d-byte appends ended with %v, want io.EOF", piece, err)
				}

				if len(got) != len(want) {
					t.Fatalf("%d-byte appends yield %d records; the finished file yields %d",
						piece, len(got), len(want))
				}

				for i := range got {
					if !reflect.DeepEqual(got[i], want[i]) {
						t.Fatalf("%d-byte appends, record %d differs:\n got  %+v\n want %+v",
							piece, i, got[i], want[i])
					}
				}
			}
		})
	}
}

// A record whose bytes arrive in two pieces is delivered once, when its last
// byte lands — never twice, and never lost. The reader is asked for a record
// after every single append, so a record that was going to be duplicated or
// dropped has every chance to be.
func TestARecordSplitAcrossAppendsArrivesOnce(t *testing.T) {
	t.Parallel()

	for _, log := range corpusLogs(t) {
		t.Run(filepath.Base(filepath.Dir(log)), func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(log) //nolint:gosec // a corpus path from the test's own glob
			if err != nil {
				t.Fatal(err)
			}

			want, _ := follow(bytes.NewReader(raw))
			if len(want) == 0 {
				t.Fatal("the finished file yielded no records, so the count below compares nothing")
			}

			src := newTail()
			writeOverTime(src, raw, 7)

			got, err := follow(src)
			if !errors.Is(err, io.EOF) {
				t.Fatalf("seven-byte appends ended with %v, want io.EOF", err)
			}

			if len(got) != len(want) {
				t.Fatalf("%d records arrived from seven-byte appends, %d from the finished file: "+
					"a record split across appends was duplicated or lost", len(got), len(want))
			}
		})
	}
}

// What a follower holds does not grow with the records it has already
// delivered. The heap is sampled early in a long follow and repeatedly after
// that: a reader accumulating records, or re-buffering the log it has read,
// would show it here.
//
// Not parallel, and deliberately so: the figure is process-wide, so another test
// allocating beside this one would be read as growth in the reader. Nor is the
// long log materialised — the writer appends the same recorded lines again and
// again — so what the test itself holds is a few kilobytes and every megabyte
// measured belongs to the reader.
//
//nolint:paralleltest // measures process-wide heap and must run alone
func TestAWaitingFollowerHoldsBoundedMemory(t *testing.T) {
	raw, err := os.ReadFile(textCorpusLog(t))
	if err != nil {
		t.Fatal(err)
	}

	// A text log's records are lines, so a longer valid log is its preamble
	// followed by more of them. The names repeat, which is the point: memory
	// follows the distinct strings a log introduces, not its record count.
	header := bytes.Index(raw, []byte("USER"))
	if header < 0 {
		t.Fatal("the text recording holds no USER record, so no longer log can be built from it")
	}

	const repeats = 2048

	src := newTail()

	go func() {
		src.appendWhenDrained(raw[:header])

		for range repeats {
			src.appendWhenDrained(raw[header:])
		}

		src.close()
	}()

	rd, err := simlog.NewReader(src)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	const (
		firstSample = 1_000
		every       = 1_000
	)

	var early, latest uint64

	delivered := 0

	for {
		if _, err := rd.Next(); err != nil {
			if !errors.Is(err, io.EOF) {
				t.Fatalf("the follow ended with %v, want io.EOF", err)
			}

			break
		}

		delivered++

		switch {
		case delivered == firstSample:
			early = heapNow()
		case delivered > firstSample && delivered%every == 0:
			latest = max(latest, heapNow())
		}
	}

	if delivered < 10*firstSample || early == 0 || latest == 0 {
		t.Fatalf("the follow delivered %d records and sampled the heap at %d and %d bytes; "+
			"the test measured nothing", delivered, early, latest)
	}

	// The reader's own fixed buffers dominate the figure and do not move. A
	// reader holding the records it had delivered would be tens of megabytes
	// above this by the end, not a margin above it.
	if latest > early+(2<<20) {
		t.Fatalf("a follower held %d bytes after %d records and %d bytes after %d in the same log: "+
			"what it holds grows with what it has delivered", early, firstSample, latest, delivered)
	}
}

// heapNow is what is live right now, with the collector given its chance first.
func heapNow() uint64 {
	runtime.GC()

	var m runtime.MemStats

	runtime.ReadMemStats(&m)

	return m.HeapAlloc
}

// textCorpusLog is a recorded text log: the format whose records are lines, so a
// test can build a longer valid log by repeating them.
func textCorpusLog(t *testing.T) string {
	t.Helper()

	for _, log := range corpusLogs(t) {
		raw, err := os.ReadFile(log) //nolint:gosec // a corpus path from the test's own glob
		if err != nil {
			t.Fatal(err)
		}

		if format, err := gatling.Detect(raw); err == nil && format == gatling.FormatText {
			return log
		}
	}

	t.Fatal("no text recording in the corpus")

	return ""
}
