package gatling_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
)

func v(major, minor, patch int) gatling.Version {
	return gatling.Version{Major: major, Minor: minor, Patch: patch}
}

func TestParseVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want gatling.Version
		ok   bool
	}{
		{in: "3.11.5", want: v(3, 11, 5), ok: true},
		{in: "3.12.0", want: v(3, 12, 0), ok: true},
		{in: "3.13.0", want: v(3, 13, 0), ok: true},
		{in: "10.0.0", want: v(10, 0, 0), ok: true},
		{in: "3.13.0-SNAPSHOT"},
		{in: "3.12.0-M1"},
		{in: "3.12"},
		{in: "v3.12.0"},
		{in: "3.12.0 "},
		{in: ""},
		{in: "garbage"},
		{in: "99999999999999999999.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got, err := gatling.ParseVersion(tt.in)
			if !tt.ok {
				if err == nil {
					t.Fatalf("ParseVersion(%q) = %v, want error", tt.in, got)
				}

				if quoted := fmt.Sprintf("%q", tt.in); !strings.Contains(err.Error(), quoted) {
					t.Fatalf("error %q does not quote the input %s", err, quoted)
				}

				return
			}

			if err != nil {
				t.Fatalf("ParseVersion(%q): %v", tt.in, err)
			}

			if got != tt.want {
				t.Fatalf("ParseVersion(%q) = %v, want %v", tt.in, got, tt.want)
			}

			if got.String() != tt.in {
				t.Fatalf("String() = %q, want round-trip of %q", got.String(), tt.in)
			}
		})
	}
}

func TestVersionCompare(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a, b gatling.Version
		want int
	}{
		{a: v(3, 11, 5), b: v(3, 11, 5), want: 0},
		{a: v(3, 11, 5), b: v(3, 12, 0), want: -1},
		{a: v(3, 12, 0), b: v(3, 11, 5), want: 1},
		{a: v(3, 11, 5), b: v(3, 11, 6), want: -1},
		{a: v(4, 0, 0), b: v(3, 99, 99), want: 1},
		{a: v(3, 9, 0), b: v(3, 11, 5), want: -1},
	}

	for _, tt := range tests {
		t.Run(tt.a.String()+" vs "+tt.b.String(), func(t *testing.T) {
			t.Parallel()

			if got := tt.a.Compare(tt.b); got != tt.want {
				t.Fatalf("Compare(%v, %v) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestGate(t *testing.T) {
	t.Parallel()

	minV, maxV := v(3, 11, 5), v(3, 12, 0)

	tests := []struct {
		found gatling.Version
		want  gatling.Verdict
	}{
		{found: v(3, 9, 0), want: gatling.VerdictRefused},
		{found: v(3, 11, 4), want: gatling.VerdictRefused},
		{found: v(3, 11, 5), want: gatling.VerdictAccepted},
		{found: v(3, 11, 9), want: gatling.VerdictAccepted},
		{found: v(3, 12, 0), want: gatling.VerdictAccepted},
		{found: v(3, 12, 1), want: gatling.VerdictUnverified},
		{found: v(3, 13, 0), want: gatling.VerdictUnverified},
		{found: v(4, 0, 0), want: gatling.VerdictUnverified},
	}

	for _, tt := range tests {
		t.Run(tt.found.String(), func(t *testing.T) {
			t.Parallel()

			if got := gatling.Gate(tt.found, minV, maxV); got != tt.want {
				t.Fatalf("Gate(%v) = %v, want %v", tt.found, got, tt.want)
			}
		})
	}
}
