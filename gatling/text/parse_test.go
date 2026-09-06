package text

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
)

func v(major, minor, patch int) gatling.Version {
	return gatling.Version{Major: major, Minor: minor, Patch: patch}
}

func TestParseHeader(t *testing.T) {
	t.Parallel()

	const class = "io.galaxio.parsec.corpus.CorpusSimulation"

	tests := []struct {
		name       string
		line       string
		want       gatling.Header
		wantFields int
		wantSyntax bool
		wantVerErr string // Found of the expected *gatling.VersionError
	}{
		{
			name:       "full",
			line:       "RUN\t" + class + "\tcorpussimulation\t1788379354534\tnightly soak\t3.11.5",
			want:       gatling.Header{SimulationClass: class, RunID: "corpussimulation", Start: 1788379354534, Description: "nightly soak", Version: v(3, 11, 5)},
			wantFields: 6,
		},
		{
			name:       "lone space is an empty description",
			line:       "RUN\t" + class + "\tcorpussimulation\t1788379354534\t \t3.12.0",
			want:       gatling.Header{SimulationClass: class, RunID: "corpussimulation", Start: 1788379354534, Version: v(3, 12, 0)},
			wantFields: 6,
		},
		{
			name:       "surplus field is reported, not judged here",
			line:       "RUN\t" + class + "\tid\t1\t \t3.13.0\textra",
			want:       gatling.Header{SimulationClass: class, RunID: "id", Start: 1, Version: v(3, 13, 0)},
			wantFields: 7,
		},
		{name: "five fields", line: "RUN\ta\tb\t1\t3.11.5", wantSyntax: true},
		{name: "start is not a number", line: "RUN\ta\tb\tnope\t \t3.11.5", wantSyntax: true},
		{name: "start is negative", line: "RUN\ta\tb\t-1\t \t3.11.5", wantSyntax: true},
		// Both codecs bound the run start by gatling.MaxRunStart, because every
		// later instant is resolved against it and the binary format adds a
		// 32-bit offset to it. A start one past the ceiling was readable here
		// and refused by the binary codec.
		{
			name: "start at the ceiling both codecs share",
			line: "RUN\t" + class + "\tid\t9223372034707292160\t \t3.11.5",
			want: gatling.Header{SimulationClass: class, RunID: "id", Start: 9223372034707292160, Version: v(3, 11, 5)}, wantFields: 6,
		},
		{name: "start one past the ceiling", line: "RUN\ta\tb\t9223372034707292161\t \t3.11.5", wantSyntax: true},
		{name: "start at the largest int64", line: "RUN\ta\tb\t9223372036854775807\t \t3.11.5", wantSyntax: true},
		{name: "snapshot version", line: "RUN\ta\tb\t1\t \t3.13.0-SNAPSHOT", wantVerErr: "3.13.0-SNAPSHOT"},
		{name: "milestone version", line: "RUN\ta\tb\t1\t \t3.12.0-M1", wantVerErr: "3.12.0-M1"},
		{name: "garbage version", line: "RUN\ta\tb\t1\t \tgarbage", wantVerErr: "garbage"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, n, err := parseHeader([]byte(tt.line), 3)

			switch {
			case tt.wantSyntax:
				var se *gatling.SyntaxError
				if !errors.As(err, &se) || se.Line != 3 {
					t.Fatalf("got %v, want *gatling.SyntaxError at line 3", err)
				}
			case tt.wantVerErr != "":
				var ve *gatling.VersionError
				if !errors.As(err, &ve) {
					t.Fatalf("got %v, want *gatling.VersionError", err)
				}

				if ve.Found != tt.wantVerErr || ve.Min != minVersion || ve.Max != maxVersion {
					t.Fatalf("got %+v, want Found=%q Min=%v Max=%v", ve, tt.wantVerErr, minVersion, maxVersion)
				}
			default:
				if err != nil {
					t.Fatalf("parseHeader: %v", err)
				}

				if got != tt.want || n != tt.wantFields {
					t.Fatalf("got %+v with %d fields, want %+v with %d", got, n, tt.want, tt.wantFields)
				}
			}
		})
	}
}

func TestParseRecord(t *testing.T) {
	t.Parallel()

	const koMessage = "status.find.is(200), but actually found 500"

	tests := []struct {
		name string
		line string
		want gatling.Record
	}{
		{
			name: "user start",
			line: "USER\tCorpus recording\tSTART\t1788379356165",
			want: gatling.Record{Kind: gatling.KindUser, Scenario: "Corpus recording", Event: gatling.EventStart, Timestamp: 1788379356165},
		},
		{
			name: "user end",
			line: "USER\tCorpus recording\tEND\t1788379357702",
			want: gatling.Record{Kind: gatling.KindUser, Scenario: "Corpus recording", Event: gatling.EventEnd, Timestamp: 1788379357702},
		},
		{
			name: "top-level ok request with a lone-space message",
			line: "REQUEST\t\tGET /ok\t1788379356162\t1788379356173\tOK\t ",
			want: gatling.Record{Kind: gatling.KindRequest, Groups: []string{}, Name: "GET /ok", Start: 1788379356162, End: 1788379356173, Status: gatling.StatusOK},
		},
		{
			name: "nested ko request with a message",
			line: "REQUEST\touter,inner  with comma\tGET /fail\t1788379357690\t1788379357699\tKO\t" + koMessage,
			want: gatling.Record{Kind: gatling.KindRequest, Groups: []string{"outer", "inner  with comma"}, Name: "GET /fail", Start: 1788379357690, End: 1788379357699, Status: gatling.StatusKO, Message: koMessage},
		},
		{
			name: "three-level path",
			line: "REQUEST\ta,b,c\tr\t1\t2\tOK\t ",
			want: gatling.Record{Kind: gatling.KindRequest, Groups: []string{"a", "b", "c"}, Name: "r", Start: 1, End: 2, Status: gatling.StatusOK},
		},
		{
			name: "request end sentinel for an event that never completed",
			line: "REQUEST\t\tr\t1788379356162\t-9223372036854775808\tKO\t ",
			want: gatling.Record{Kind: gatling.KindRequest, Groups: []string{}, Name: "r", Start: 1788379356162, End: math.MinInt64, Status: gatling.StatusKO},
		},
		// A negative time is reported absent, not refused — the answer the binary
		// codec already gives to a negative offset — and a negative cumulated
		// response time is carried as written, as the binary codec carries a
		// negative 32-bit field. Neither ends the read.
		{
			name: "negative user timestamp is absent, not refused",
			line: "USER\tscn\tSTART\t-5",
			want: gatling.Record{Kind: gatling.KindUser, Scenario: "scn", Event: gatling.EventStart, Timestamp: gatling.AbsentTimestamp},
		},
		{
			name: "negative request start and end are absent",
			line: "REQUEST\t\tr\t-5\t-7\tOK\t ",
			want: gatling.Record{Kind: gatling.KindRequest, Groups: []string{}, Name: "r", Start: gatling.AbsentTimestamp, End: gatling.AbsentTimestamp, Status: gatling.StatusOK},
		},
		{
			name: "negative group times are absent and a negative cumulated time is kept",
			line: "GROUP\tg\t-1\t-2\t-5\tKO",
			want: gatling.Record{Kind: gatling.KindGroup, Groups: []string{"g"}, Start: gatling.AbsentTimestamp, End: gatling.AbsentTimestamp, CumulatedResponseTime: -5, Status: gatling.StatusKO},
		},
		{
			name: "negative error timestamp is absent",
			line: "ERROR\tboom\t-9",
			want: gatling.Record{Kind: gatling.KindError, Message: "boom", Timestamp: gatling.AbsentTimestamp},
		},
		// A negative zero is the instant it spells. Reading it as an absence
		// would make the epoch unrepresentable in one spelling and not the
		// other, and the two parsers would disagree about identical bytes.
		{
			name: "negative zero is the epoch instant, not an absence",
			line: "USER\tscn\tSTART\t-0",
			want: gatling.Record{Kind: gatling.KindUser, Scenario: "scn", Event: gatling.EventStart, Timestamp: 0},
		},
		// The most negative int64 is the one negative number Gatling writes. It
		// is an absence in a time field and a value in the cumulated field, and
		// neither may refuse it: stripping the sign to parse the magnitude used
		// to, because that magnitude has no positive counterpart.
		{
			name: "the most negative cumulated response time is kept as written",
			line: "GROUP\tg\t1\t2\t-9223372036854775808\tKO",
			want: gatling.Record{Kind: gatling.KindGroup, Groups: []string{"g"}, Start: 1, End: 2, CumulatedResponseTime: math.MinInt64, Status: gatling.StatusKO},
		},
		{
			name: "group",
			line: "GROUP\touter\t1788379356180\t1788379357700\t1520\tKO",
			want: gatling.Record{Kind: gatling.KindGroup, Groups: []string{"outer"}, Start: 1788379356180, End: 1788379357700, CumulatedResponseTime: 1520, Status: gatling.StatusKO},
		},
		{
			name: "error",
			line: "ERROR\tunresolvable url: Failed to build request \t1788379357701",
			want: gatling.Record{Kind: gatling.KindError, Message: "unresolvable url: Failed to build request ", Timestamp: 1788379357701},
		},
		{
			name: "error whose message contains separators",
			line: "ERROR\tfirst\tsecond\tthird\t1788379357701",
			want: gatling.Record{Kind: gatling.KindError, Message: "first\tsecond\tthird", Timestamp: 1788379357701},
		},
		{
			name: "error with a lone-space message",
			line: "ERROR\t \t5",
			want: gatling.Record{Kind: gatling.KindError, Timestamp: 5},
		},
		{
			name: "assertion after the header",
			line: "ASSERTION\tAAEBAAICAAAAAAAAAPA/",
			want: gatling.Record{Kind: gatling.KindAssertion, Payload: "AAEBAAICAAAAAAAAAPA/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := newParser(false)

			got, err := p.parse([]byte(tt.line), 42)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			tt.want.Line = 42
			if tt.want.Groups == nil {
				got.Groups = nil // kinds without a path leave the slice untouched
			}

			if !equalRecords(got, tt.want) {
				t.Fatalf("got  %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func equalRecords(a, b gatling.Record) bool {
	if len(a.Groups) != len(b.Groups) {
		return false
	}

	for i := range a.Groups {
		if a.Groups[i] != b.Groups[i] {
			return false
		}
	}

	a.Groups, b.Groups = nil, nil

	return reflect.DeepEqual(a, b)
}

func TestParseRecordErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		line      string
		isLenient bool
		wantOK    bool
		wantFound string
	}{
		{name: "empty line", line: ""},
		{name: "unknown kind", line: "BOGUS\ta\tb", wantFound: "BOGUS"},
		{name: "run after the header", line: "RUN\ta\tb\t1\t \t3.11.5", wantFound: "RUN"},
		{name: "user with three fields", line: "USER\tscn\tSTART"},
		{name: "user with five fields, strict", line: "USER\tscn\tSTART\t1\textra"},
		{name: "user with five fields, lenient", line: "USER\tscn\tSTART\t1\textra", isLenient: true, wantOK: true},
		{name: "request with eight fields, strict", line: "REQUEST\t\tr\t1\t2\tOK\t \textra"},
		{name: "request with eight fields, lenient", line: "REQUEST\t\tr\t1\t2\tOK\t \textra", isLenient: true, wantOK: true},
		{name: "request with six fields", line: "REQUEST\t\tr\t1\t2\tOK"},
		{name: "group with five fields", line: "GROUP\tg\t1\t2\t3"},
		{name: "error with two fields", line: "ERROR\tmessage only"},
		{name: "assertion with one field", line: "ASSERTION"},
		{name: "assertion with three fields, strict", line: "ASSERTION\tabc\textra"},
		{name: "bad timestamp", line: "USER\tscn\tSTART\tsoon"},
		{name: "timestamp that overflows", line: "USER\tscn\tSTART\t99999999999999999999"},
		{name: "negative timestamp that is not digits", line: "USER\tscn\tSTART\t-1x"},
		// The magnitude is bounded for both signs alike. A negative too wide for
		// an int64 used to pass as an absence while its positive twin was
		// refused, so a line whose fields had shifted could clear the only
		// structural check a timestamp gets.
		{name: "negative timestamp too wide for an int64", line: "USER\tscn\tSTART\t-99999999999999999999999"},
		{name: "negative request start too wide for an int64", line: "REQUEST\t\tr\t-00000000000000000000001\t2\tOK\t "},
		{name: "negative cumulated response time too wide for an int64", line: "GROUP\tg\t1\t2\t-99999999999999999999999\tOK"},
		{name: "a lone minus sign", line: "USER\tscn\tSTART\t-"},
		// Above the covered range a newer Gatling may append a field behind the
		// timestamp, so the reader scans back for the field that reads as one.
		// The scan probed with a predicate that refused negatives while the
		// parser accepted them, so it walked past a real timestamp and refused
		// the read on the field behind it.
		{name: "lenient error record with an absent timestamp and an appended field", line: "ERROR\tboom\t-1\textra", isLenient: true, wantOK: true},
		{name: "cumulated response time that overflows", line: "GROUP\tg\t1\t2\t-99999999999999999999\tOK"},
		{name: "bad status", line: "REQUEST\t\tr\t1\t2\tok\t "},
		{name: "bad event", line: "USER\tscn\tBEGIN\t1"},
		{name: "bad cumulated response time", line: "GROUP\tg\t1\t2\tx\tOK"},
		{name: "error with a bad timestamp", line: "ERROR\tm\tsoon"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := newParser(tt.isLenient)

			_, err := p.parse([]byte(tt.line), 7)
			if tt.wantOK {
				if err != nil {
					t.Fatalf("parse: %v", err)
				}

				return
			}

			var se *gatling.SyntaxError
			if !errors.As(err, &se) {
				t.Fatalf("got %v, want *gatling.SyntaxError", err)
			}

			if se.Line != 7 || se.Expected == "" || se.Found == "" {
				t.Fatalf("incomplete error %+v", se)
			}

			if tt.wantFound != "" && !strings.Contains(se.Found, tt.wantFound) {
				t.Fatalf("Found %q does not name %q", se.Found, tt.wantFound)
			}
		})
	}
}
