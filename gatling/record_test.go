package gatling_test

import (
	"fmt"
	"testing"

	"github.com/galax-io/parsec/gatling"
)

func TestKindString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind gatling.Kind
		want string
	}{
		{kind: gatling.KindUnknown, want: "unknown"},
		{kind: gatling.KindRun, want: "RUN"},
		{kind: gatling.KindUser, want: "USER"},
		{kind: gatling.KindRequest, want: "REQUEST"},
		{kind: gatling.KindGroup, want: "GROUP"},
		{kind: gatling.KindError, want: "ERROR"},
		{kind: gatling.KindAssertion, want: "ASSERTION"},
		{kind: gatling.Kind(42), want: "Kind(42)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			if got := tt.kind.String(); got != tt.want {
				t.Fatalf("Kind(%d).String() = %q, want %q", uint8(tt.kind), got, tt.want)
			}
		})
	}
}

func TestStatusString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status gatling.Status
		want   string
	}{
		{status: gatling.StatusUnknown, want: "unknown"},
		{status: gatling.StatusOK, want: "OK"},
		{status: gatling.StatusKO, want: "KO"},
		{status: gatling.Status(9), want: "Status(9)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			if got := tt.status.String(); got != tt.want {
				t.Fatalf("Status(%d).String() = %q, want %q", uint8(tt.status), got, tt.want)
			}
		})
	}
}

func TestEventString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		event gatling.Event
		want  string
	}{
		{event: gatling.EventUnknown, want: "unknown"},
		{event: gatling.EventStart, want: "START"},
		{event: gatling.EventEnd, want: "END"},
		{event: gatling.Event(9), want: "Event(9)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			if got := tt.event.String(); got != tt.want {
				t.Fatalf("Event(%d).String() = %q, want %q", uint8(tt.event), got, tt.want)
			}
		})
	}
}

func TestVerdictString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		verdict gatling.Verdict
		want    string
	}{
		{verdict: gatling.VerdictUnknown, want: "unknown"},
		{verdict: gatling.VerdictRefused, want: "refused"},
		{verdict: gatling.VerdictAccepted, want: "accepted"},
		{verdict: gatling.VerdictUnverified, want: "unverified"},
		{verdict: gatling.Verdict(9), want: "Verdict(9)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			if got := tt.verdict.String(); got != tt.want {
				t.Fatalf("Verdict(%d).String() = %q, want %q", uint8(tt.verdict), got, tt.want)
			}
		})
	}
}

// The zero value of every enum must be the unknown sentinel, so a Record that
// was never decoded cannot pass for one.
func TestZeroValuesAreUnknown(t *testing.T) {
	t.Parallel()

	var rec gatling.Record

	for _, field := range []fmt.Stringer{rec.Kind, rec.Status, rec.Event} {
		if got := field.String(); got != "unknown" {
			t.Fatalf("zero %T renders as %q, want unknown", field, got)
		}
	}
}
