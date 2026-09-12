package binary_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
	"github.com/galax-io/parsec/gatling/text"
	"github.com/galax-io/parsec/model"
)

// textPreamble is a text run header for the same simulation the binary builder
// writes, so the two logs below differ only in their format.
const textPreamble = "RUN\tio.example.Sim\tsim\t1788670094356\t \t3.12.0\n"

// The two codecs promise the same canonical results for an equivalent run, and
// that promise has to hold for the inputs a well-formed log never carries as
// much as for the ones it does. Each row is one input both formats can express,
// written once for each, read through both RunReaders, and required to come out
// identical — the same items, or the same kind of failure. A fourth divergence
// cannot land without changing this table.
//
// The third row already agreed before this test existed; it is here so that it
// keeps agreeing.
func TestCodecsAgreeOnMalformedInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		bin  []byte
		// wantRefused says both codecs must refuse the input rather than agree
		// on a record, so that a row exercising the error path cannot be added
		// without saying so.
		wantRefused bool
	}{
		{
			name: "a negative time on a request",
			text: textPreamble + "REQUEST\t\tGET /x\t-5\t1788670094366\tOK\t \n",
			bin: (&builder{}).runRecord("3.15.1", []string{"s"}, nil).
				u8(1).i32(0).newString("GET /x").i32(-5).i32(10).u8(1).newString("").
				bytes(),
		},
		{
			name: "a negative cumulated response time on a group",
			text: textPreamble + "GROUP\tg\t1788670094356\t1788670094366\t-5\tOK\n",
			bin: (&builder{}).runRecord("3.15.1", []string{"s"}, nil).
				u8(3).i32(1).newString("g").i32(0).i32(10).i32(-5).u8(1).
				bytes(),
		},
		{
			// Every other row decodes through both codecs, so without this one
			// the table compares successful streams only and could not express
			// a divergence in which one codec refuses and the other does not —
			// which is the shape the run start had.
			name: "a run start past the ceiling both codecs share",
			text: "RUN\tio.example.Sim\tsim\t9223372036854775807\t \t3.12.0\n",
			bin: (&builder{}).u8(0).str("3.15.1").str("io.example.Sim").
				i64(math.MaxInt64).str("").i32(0).i32(0).
				bytes(),
			wantRefused: true,
		},
		{
			name: "a first record with no groups",
			text: textPreamble + "REQUEST\t\tGET /y\t1788670094356\t1788670094366\tOK\t \n",
			bin: (&builder{}).runRecord("3.15.1", []string{"s"}, nil).
				u8(1).i32(0).newString("GET /y").i32(0).i32(10).u8(1).newString("").
				bytes(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fromText, textErr := describeRun(func() (runReader, error) {
				return text.NewRunReader(strings.NewReader(tt.text))
			})
			fromBinary, binErr := describeRun(func() (runReader, error) {
				return binary.NewRunReader(bytes.NewReader(tt.bin))
			})

			if failuresDiffer(textErr, binErr) {
				t.Fatalf("text failed with %v and binary with %v; the codecs must fail alike or not at all",
					textErr, binErr)
			}

			if (textErr != nil) != tt.wantRefused {
				t.Fatalf("wantRefused = %t, but the text codec returned %v", tt.wantRefused, textErr)
			}

			if textErr != nil {
				return
			}

			if fromText != fromBinary {
				t.Errorf("the codecs disagree:\n text:   %s\n binary: %s", fromText, fromBinary)
			}
		})
	}
}

// failuresDiffer reports whether exactly one of the two reads failed, or both
// failed with a different kind of error — the two ways the codecs can disagree
// short of disagreeing about the records.
//
// It is named for what it returns rather than for the property being checked,
// so that the guard at the call site reads the way it behaves: a reader who
// negates it to match a name would silently invert the whole table.
func failuresDiffer(a, b error) bool {
	if (a == nil) != (b == nil) {
		return true
	}

	var sa, sb *gatling.SyntaxError

	return a != nil && errors.As(a, &sa) != errors.As(b, &sb)
}

// describeRun reads a run to its end and renders every item into one line, so
// two runs compare as strings and a difference names the field.
func describeRun(open func() (runReader, error)) (string, error) {
	rd, err := open()
	if err != nil {
		return "", err
	}

	var b strings.Builder

	for {
		it, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return b.String(), nil
		}

		if err != nil {
			return "", err
		}

		b.WriteString(describe(it))
	}
}

func describe(it model.Item) string {
	switch it.Kind {
	case model.ItemSample:
		s := it.Sample

		return fmt.Sprintf("sample groups=%q nilgroups=%t name=%q start=%d zerostart=%t duration=%v outcome=%s failure=%v; ",
			s.Groups, s.Groups == nil, s.Name, s.Start.UnixMilli(), s.Start.IsZero(), s.Duration, s.Outcome, s.Failure)
	case model.ItemGroup:
		g := it.Group

		return fmt.Sprintf("group groups=%q nilgroups=%t start=%d zerostart=%t duration=%v cumulated=%v outcome=%s; ",
			g.Groups, g.Groups == nil, g.Start.UnixMilli(), g.Start.IsZero(), g.Duration, g.CumulatedDuration, g.Outcome)
	case model.ItemUser:
		return fmt.Sprintf("user scenario=%q kind=%s at=%d; ", it.User.Scenario, it.User.Kind, it.User.At.UnixMilli())
	case model.ItemError:
		return fmt.Sprintf("error message=%q at=%d; ", it.Error.Message, it.Error.At.UnixMilli())
	case model.ItemAssertion:
		return fmt.Sprintf("assertion %q; ", it.Assertion)
	case model.ItemUnknown:
		return "unknown; "
	}

	return fmt.Sprintf("%s; ", it.Kind)
}

func isVersionError(err error) bool { return errors.As(err, new(*gatling.VersionError)) }

func isUnparsedVersionError(err error) bool {
	var ve *gatling.VersionError

	return errors.As(err, &ve) && !ve.Parsed
}

func isUnverifiedError(err error) bool { return errors.As(err, new(*gatling.UnverifiedError)) }

func isSyntaxError(err error) bool { return errors.As(err, new(*gatling.SyntaxError)) }

// The gate rules on the version before anything after it is judged, in both
// codecs, so a log naming a version outside the range is refused as such
// whatever else is wrong with it — with one exception the text format forces:
// its assertions come before its version, so an assertion table past its 8 MiB
// ceiling is refused there as damage before the gate can rule. Each row puts a
// fault both formats can express beside a version the gate must rule on first,
// once for each fault; the last row is the control, where the gate passes and
// the fault is judged next. failuresDiffer would call two different refusals
// agreement, so this table asserts the type.
func TestCodecsGateBeforeTheRestOfTheRunRecord(t *testing.T) {
	t.Parallel()

	faults := []struct {
		name     string
		inText   func(version string) string
		inBinary func(version string) []byte
	}{
		{
			name:   "a run start past the ceiling",
			inText: func(v string) string { return "RUN\tio.example.Sim\tsim\t9223372036854775807\t \t" + v + "\n" },
			inBinary: func(v string) []byte {
				return (&builder{}).u8(0).str(v).str("io.example.Sim").i64(math.MaxInt64).str("").i32(0).i32(0).bytes()
			},
		},
		{
			name: "an assertion that cannot be read",
			inText: func(v string) string {
				return "ASSERTION\n" + "RUN\tio.example.Sim\tsim\t1788670094356\t \t" + v + "\n"
			},
			inBinary: func(v string) []byte {
				return (&builder{}).u8(0).str(v).str("io.example.Sim").i64(runStart).str("").i32(0).i32(-1).bytes()
			},
		},
	}

	tests := []struct {
		name    string
		version string
		opts    []gatling.Option
		want    func(error) bool
		wants   string
	}{
		{name: "below the range", version: "1.0.0", want: isVersionError, wants: "a *gatling.VersionError"},
		{name: "not a release", version: "3.x", want: isUnparsedVersionError, wants: "a *gatling.VersionError with Parsed false"},
		{
			name:    "above the range, strict",
			version: "3.99.0",
			opts:    []gatling.Option{gatling.WithStrict()},
			want:    isUnverifiedError,
			wants:   "a *gatling.UnverifiedError",
		},
		{
			name:    "above the range, lenient: the gate passes and the fault is judged",
			version: "3.99.0",
			want:    isSyntaxError,
			wants:   "the fault's *gatling.SyntaxError",
		},
	}

	for _, f := range faults {
		for _, tt := range tests {
			t.Run(f.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				_, fromText := text.NewReader(strings.NewReader(f.inText(tt.version)), tt.opts...)
				_, fromBinary := binary.NewReader(bytes.NewReader(f.inBinary(tt.version)), tt.opts...)

				for codec, err := range map[string]error{"text": fromText, "binary": fromBinary} {
					if !tt.want(err) {
						t.Errorf("%s: NewReader = _, %v; want %s", codec, err, tt.wants)
					}
				}
			})
		}
	}
}

// The version warning a user reads is prose, and it was written out twice — once
// in each codec's NewRunReader, byte for byte the same, one function short of
// where the v0.0.5 extraction stopped. Two copies of a sentence can drift, and
// then the same condition prints differently depending on which format was
// opened, in a module whose stated goal is that a consumer cannot tell the two
// formats apart.
//
// The two ranges differ, so the sentences are not equal; what must be equal is
// everything around the two versions. The test builds each codec's expected
// reason from one template and its own SupportedVersions, so a codec that
// reworded its half fails here.
func TestBothCodecsWordTheSameWarningIdentically(t *testing.T) {
	t.Parallel()

	const template = "no recording covers it — the verified range is %s through %s, " +
		"so the records decode unverified"

	// One release above either codec's range, so both gate it as unverified.
	const above = "3.16.0"

	textOldest, textNewest := text.SupportedVersions()
	binOldest, binNewest := binary.SupportedVersions()

	textRun, err := text.NewRunReader(strings.NewReader(
		"RUN\tio.example.Sim\tsim\t1788670094356\t \t" + above + "\n",
	))
	if err != nil {
		t.Fatalf("text.NewRunReader: %v", err)
	}

	binRun, err := binary.NewRunReader(bytes.NewReader(
		(&builder{}).runRecord(above, []string{"s"}, nil).bytes(),
	))
	if err != nil {
		t.Fatalf("binary.NewRunReader: %v", err)
	}

	cases := []struct {
		name           string
		got            []model.Warning
		oldest, newest gatling.Version
	}{
		{name: "text", got: textRun.Run().Warnings, oldest: textOldest, newest: textNewest},
		{name: "binary", got: binRun.Run().Warnings, oldest: binOldest, newest: binNewest},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			want := model.Warning{
				Version: above,
				Reason:  fmt.Sprintf(template, tt.oldest, tt.newest),
			}

			if len(tt.got) != 1 {
				t.Fatalf("Warnings = %+v, want exactly one", tt.got)
			}

			if tt.got[0] != want {
				t.Errorf("Warnings[0] =\n  %+v\nwant\n  %+v", tt.got[0], want)
			}
		})
	}
}
