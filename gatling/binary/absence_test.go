package binary_test

import (
	"bytes"
	"testing"

	"github.com/galax-io/parsec/gatling/binary"
	"github.com/galax-io/parsec/model"
)

// Through the model, an offset the log could not resolve is the zero time — the
// standard library's own "never set" — rather than a plausible date 292 million
// years in the past. A consumer bucketing by start would otherwise file the
// event there and nothing would look wrong until someone read the axis.
func TestAnUnresolvableOffsetIsTheZeroTimeInTheModel(t *testing.T) {
	t.Parallel()

	log := (&builder{}).runRecord("3.15.1", []string{"s"}, nil).
		u8(2).i32(0).u8(1).i32(-5).
		bytes()

	rd, err := binary.NewRunReader(bytes.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}

	it, err := rd.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}

	if it.Kind != model.ItemUser {
		t.Fatalf("Kind = %v, want a user event", it.Kind)
	}

	if !it.User.At.IsZero() {
		t.Errorf("At = %v; want the zero time for a time the log could not resolve", it.User.At)
	}
}
