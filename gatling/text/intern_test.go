package text

import (
	"bytes"
	"strconv"
	"testing"
)

func TestInternShares(t *testing.T) {
	t.Parallel()

	in := newInterner()

	first := in.intern([]byte("GET /ok"))
	second := in.intern([]byte("GET /ok"))

	if first != "GET /ok" || second != first {
		t.Fatalf("intern returned %q then %q, want the same value twice", first, second)
	}

	if len(in.seen) != 1 {
		t.Fatalf("table holds %d entries after one distinct value, want 1", len(in.seen))
	}

	if got := in.intern(nil); got != "" {
		t.Fatalf("intern(nil) = %q, want empty", got)
	}
}

// A repeated value must cost no allocation: the whole point of the table.
//
//nolint:paralleltest // testing.AllocsPerRun refuses to run inside a parallel test
func TestInternLookupDoesNotAllocate(t *testing.T) {
	in := newInterner()
	key := []byte("GET /ok")
	in.intern(key)

	if allocs := testing.AllocsPerRun(100, func() { in.intern(key) }); allocs != 0 {
		t.Fatalf("a repeated value allocated %.0f times per call, want 0", allocs)
	}
}

func TestInternBounds(t *testing.T) {
	t.Parallel()

	in := newInterner()

	long := bytes.Repeat([]byte("x"), internMaxLen+1)
	if got := in.intern(long); got != string(long) || len(in.seen) != 0 {
		t.Fatalf("a value over %d bytes must be returned unchanged and not kept; table holds %d", internMaxLen, len(in.seen))
	}

	for i := range internMaxEntries {
		in.intern([]byte("name-" + strconv.Itoa(i)))
	}

	if len(in.seen) != internMaxEntries {
		t.Fatalf("table holds %d entries, want the cap of %d", len(in.seen), internMaxEntries)
	}

	if got := in.intern([]byte("one too many")); got != "one too many" || len(in.seen) != internMaxEntries {
		t.Fatalf("past the cap the value must still decode and the table must not grow; got %q with %d entries", got, len(in.seen))
	}
}
