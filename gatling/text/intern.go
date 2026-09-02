package text

// Bounds on the name table. A log repeats a small vocabulary — scenario,
// request and group names, and usually the same failure texts — many
// thousands of times, so one shared string per distinct value replaces one
// allocation per record. The table is capped so that memory stays independent
// of the log's size even when every line carries a value never seen before:
// at most internMaxEntries values of at most internMaxLen bytes are kept,
// about 1 MiB in the worst case, and everything beyond that is allocated
// as it was before.
const (
	internMaxEntries = 4096
	internMaxLen     = 256
)

// interner hands out one shared string per distinct byte sequence.
type interner struct {
	seen map[string]string
}

func newInterner() *interner {
	return &interner{seen: make(map[string]string, 64)}
}

// intern returns the shared string for b, allocating only the first time a
// value is seen. The map lookup with a converted key does not allocate.
//
// Measured on the 64 MiB synthetic log (benchstat, n=6): allocations per
// read fell from 2,067,796 to 28 and bytes allocated from 28.3 MiB to 1.0 MiB
// (-96%), while throughput stayed within noise (405.6 vs 395.1 MiB/s,
// p=0.093). The gain is garbage-collector pressure in a consumer that
// embeds this reader alongside a large heap, not single-threaded speed.
func (in *interner) intern(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	if len(b) > internMaxLen {
		return string(b)
	}

	if s, ok := in.seen[string(b)]; ok {
		return s
	}

	s := string(b)

	if len(in.seen) < internMaxEntries {
		in.seen[s] = s
	}

	return s
}
