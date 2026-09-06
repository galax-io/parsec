package binary

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
)

// syntaxError unwraps the error every malformed input must produce, and fails
// the test when it is anything else. A decoder that returns a bare error, or an
// error without a position, is not usable: a caller cannot say where the log
// went wrong.
func syntaxError(t *testing.T, err error) *gatling.SyntaxError {
	t.Helper()

	if err == nil {
		t.Fatal("want a *gatling.SyntaxError, got no error at all")
	}

	var se *gatling.SyntaxError
	if !errors.As(err, &se) {
		t.Fatalf("want a *gatling.SyntaxError, got %T: %v", err, err)
	}

	if se.Format != gatling.FormatBinary {
		t.Errorf("error names format %v; a binary decoder must say so, or Error renders a line number "+
			"for a log that has none", se.Format)
	}

	return se
}

func TestPrimitivesReadWhatTheWriterWrote(t *testing.T) {
	t.Parallel()

	in := []byte{
		0x2a,                   // u8
		0x00,                   // false
		0x01,                   // true
		0x00, 0x00, 0x01, 0x00, // i32 256
		0xff, 0xff, 0xff, 0xff, // i32 -1
		0x00, 0x00, 0x01, 0xa0, 0x8c, 0x63, 0x74, 0x00, // i64, an epoch millisecond value
	}

	r := newReader(bytes.NewReader(in))

	if b, err := r.u8("a byte"); err != nil || b != 0x2a {
		t.Fatalf("u8 = %#x, %v; want 0x2a, <nil>", b, err)
	}

	for _, want := range []bool{false, true} {
		got, err := r.boolean("a flag")
		if err != nil || got != want {
			t.Fatalf("boolean = %v, %v; want %v, <nil>", got, err, want)
		}
	}

	for _, want := range []int32{256, -1} {
		got, err := r.i32("a count")
		if err != nil || got != want {
			t.Fatalf("i32 = %d, %v; want %d, <nil>", got, err, want)
		}
	}

	got, err := r.i64("the run start")
	if err != nil || got != 1789061723136 {
		t.Fatalf("i64 = %d, %v; want 1789061723136, <nil>", got, err)
	}

	if r.off != int64(len(in)) {
		t.Fatalf("consumed %d bytes of %d; the offset an error carries is only as good as this count",
			r.off, len(in))
	}
}

func TestPrimitivesRefuseMalformedInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		in         []byte
		read       func(*reader) error
		wantOffset int64
		wantFound  string
	}{
		{
			name: "a bool that is neither 0 nor 1",
			in:   []byte{0x02},
			read: func(r *reader) error { _, err := r.boolean("ok"); return err },
			// The offset names the byte itself, not the byte after it.
			wantOffset: 0,
			wantFound:  "0x02",
		},
		{
			name:       "a bool past the end",
			in:         nil,
			read:       func(r *reader) error { _, err := r.boolean("ok"); return err },
			wantOffset: 0,
			wantFound:  "end of input",
		},
		{
			name: "an int32 cut short",
			in:   []byte{0x00, 0x00, 0x01},
			read: func(r *reader) error { _, err := r.i32("a count"); return err },
			// A truncation names the first byte that was not there, which is
			// where a reader would open the file to see the end of it.
			wantOffset: 3,
			wantFound:  "end of input",
		},
		{
			name:       "an int64 cut short",
			in:         []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
			read:       func(r *reader) error { _, err := r.i64("the run start"); return err },
			wantOffset: 7,
			wantFound:  "end of input",
		},
		{
			name: "a negative length",
			in:   []byte{0xff, 0xff, 0xff, 0xff},
			read: func(r *reader) error { _, err := r.str("a name"); return err },
			// The offset names where the value started, which is where a reader
			// would open the file, not where the reader gave up.
			wantOffset: 0,
			wantFound:  "a negative length",
		},
		{
			name:       "a length past the allocation cap",
			in:         []byte{0x7f, 0xff, 0xff, 0xff},
			read:       func(r *reader) error { _, err := r.str("a name"); return err },
			wantOffset: 0,
			wantFound:  "past the maximum",
		},
		{
			name: "a length past the end of the file",
			in:   []byte{0x00, 0x00, 0x10, 0x00, 'a', 'b'},
			read: func(r *reader) error { _, err := r.str("a name"); return err },
			// A truncation names the first byte that was not there: two of the
			// four thousand it claimed did arrive.
			wantOffset: 6,
			wantFound:  "end of input",
		},
		{
			name:       "a string cut short of its coder byte",
			in:         []byte{0x00, 0x00, 0x00, 0x02, 'o', 'k'},
			read:       func(r *reader) error { _, err := r.str("a name"); return err },
			wantOffset: 6,
			wantFound:  "end of input",
		},
		{
			name:       "an encoding marker that is neither coder",
			in:         []byte{0x00, 0x00, 0x00, 0x02, 'o', 'k', 0x07},
			read:       func(r *reader) error { _, err := r.str("a name"); return err },
			wantOffset: 0,
			wantFound:  "an encoding marker of 7",
		},
		{
			name:       "a UTF-16 string of an odd number of bytes",
			in:         []byte{0x00, 0x00, 0x00, 0x03, 'a', 0x00, 'b', coderUTF16},
			read:       func(r *reader) error { _, err := r.str("a name"); return err },
			wantOffset: 0,
			wantFound:  "odd number of bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			se := syntaxError(t, tt.read(newReader(bytes.NewReader(tt.in))))

			if se.Offset != tt.wantOffset {
				t.Errorf("error names byte %d; want %d", se.Offset, tt.wantOffset)
			}

			if !strings.Contains(se.Found, tt.wantFound) {
				t.Errorf("error says found %q; want it to mention %q", se.Found, tt.wantFound)
			}

			if se.Line != 0 {
				t.Errorf("error names line %d; a binary log has no lines", se.Line)
			}
		})
	}
}

// A cap that is only checked after allocating is not a cap. This asks for a
// gigabyte from a reader that holds four bytes and requires the refusal to come
// before any of it is reserved.
func TestALengthPastTheCapIsRefusedBeforeAllocating(t *testing.T) {
	t.Parallel()

	r := newReader(bytes.NewReader([]byte{0x3f, 0xff, 0xff, 0xff}))

	_, err := r.str("a name")
	_ = syntaxError(t, err)

	if cap(r.scratch) > MaxStringLen {
		t.Fatalf("the reader reserved %d bytes for a length it refused", cap(r.scratch))
	}
}

func TestStringsDecodeInBothEncodings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{
			name: "the empty string carries no coder byte",
			in:   []byte{0x00, 0x00, 0x00, 0x00},
			want: "",
		},
		{
			name: "Latin-1 ASCII",
			in:   append([]byte{0x00, 0x00, 0x00, 0x07}, append([]byte("GET /ok"), coderLatin1)...),
			want: "GET /ok",
		},
		{
			name: "Latin-1 above ASCII",
			in:   []byte{0x00, 0x00, 0x00, 0x02, 0xe9, 0xff, coderLatin1},
			want: "éÿ",
		},
		{
			name: "UTF-16 Cyrillic",
			in: []byte{
				0x00, 0x00, 0x00, 0x06,
				0x1f, 0x04, 0x40, 0x04, 0x3e, 0x04, // П р о, little-endian
				coderUTF16,
			},
			want: "Про",
		},
		{
			name: "UTF-16 outside the basic multilingual plane",
			in: []byte{
				0x00, 0x00, 0x00, 0x04,
				0x3d, 0xd8, 0x00, 0xde, // U+1F600, as a surrogate pair
				coderUTF16,
			},
			want: "\U0001F600",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newReader(bytes.NewReader(tt.in)).str("a name")
			if err != nil {
				t.Fatalf("str: %v", err)
			}

			if got != tt.want {
				t.Fatalf("str = %q; want %q", got, tt.want)
			}
		})
	}
}

// The empty string is the one field whose framing differs, and reading it wrong
// costs nothing at the string itself and everything afterwards. This checks that
// the next value is still where the writer put it.
func TestTheEmptyStringConsumesNoCoderByte(t *testing.T) {
	t.Parallel()

	in := []byte{
		0x00, 0x00, 0x00, 0x00, // the empty string
		0x00, 0x00, 0x00, 0x2a, // and the int32 that follows it
	}

	r := newReader(bytes.NewReader(in))

	if s, err := r.str("a message"); err != nil || s != "" {
		t.Fatalf("str = %q, %v; want \"\", <nil>", s, err)
	}

	got, err := r.i32("a count")
	if err != nil || got != 42 {
		t.Fatalf("the value after the empty string reads as %d, %v; want 42, <nil> — "+
			"a coder byte was consumed that the writer never wrote", got, err)
	}
}

func TestCacheIntroducesAndRefersBack(t *testing.T) {
	t.Parallel()

	in := []byte{
		0x00, 0x00, 0x00, 0x01, // introduce entry 1
		0x00, 0x00, 0x00, 0x07, 'G', 'E', 'T', ' ', '/', 'o', 'k', coderLatin1,
		0xff, 0xff, 0xff, 0xff, // refer to entry 1
		0x00, 0x00, 0x00, 0x02, // introduce entry 2
		0x00, 0x00, 0x00, 0x00, // which is empty
		0xff, 0xff, 0xff, 0xfe, // refer to entry 2
	}

	r, c := newReader(bytes.NewReader(in)), &cache{}

	want := []string{"GET /ok", "GET /ok", "", ""}
	for i, w := range want {
		got, err := c.read(r, "a name")
		if err != nil || got != w {
			t.Fatalf("read %d = %q, %v; want %q, <nil>", i, got, err, w)
		}
	}

	if len(c.entries) != 2 {
		t.Fatalf("the table holds %d entries; want 2 — a reference must not grow it", len(c.entries))
	}
}

func TestCacheRefusesEveryBrokenIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        []byte
		wantFound string
	}{
		{
			name:      "a reference to an entry never introduced",
			in:        []byte{0xff, 0xff, 0xff, 0xff},
			wantFound: "only 0 have been introduced",
		},
		{
			name:      "index zero, which the format never writes",
			in:        []byte{0x00, 0x00, 0x00, 0x00},
			wantFound: "cache index 0",
		},
		{
			name: "a new index that skips one",
			in: []byte{
				0x00, 0x00, 0x00, 0x02, // entry 2 before entry 1
				0x00, 0x00, 0x00, 0x01, 'x', coderLatin1,
			},
			wantFound: "entry 1 comes next",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := (&cache{}).read(newReader(bytes.NewReader(tt.in)), "a name")
			se := syntaxError(t, err)

			if se.Offset != 0 {
				t.Errorf("error names byte %d; want 0", se.Offset)
			}

			if !strings.Contains(se.Found, tt.wantFound) {
				t.Errorf("error says found %q; want it to mention %q", se.Found, tt.wantFound)
			}
		})
	}
}

// A reader that returns (0, nil) for ever is a livelock, not an error, and a
// decoder that loops on one hangs a caller with no way to tell what happened.
func TestAReaderThatNeverProgressesIsAnError(t *testing.T) {
	t.Parallel()

	r := newReader(stalledReader{})

	end, err := r.atEnd()
	if err == nil {
		t.Fatalf("atEnd = %v, <nil>; want an error rather than a verdict", end)
	}

	if !errors.Is(err, io.ErrNoProgress) {
		t.Fatalf("atEnd = %v; want io.ErrNoProgress", err)
	}
}

// stalledReader is neither at its end nor able to produce a byte.
type stalledReader struct{}

func (stalledReader) Read([]byte) (int, error) { return 0, nil }
