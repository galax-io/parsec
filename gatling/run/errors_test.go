package run_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/galax-io/parsec/gatling/run"
)

func TestNotFoundError(t *testing.T) {
	t.Parallel()

	err := &run.NotFoundError{Dir: "/srv/results"}
	mustContain(t, err.Error(), "/srv/results", "no Gatling run")

	// The directory is always one the caller gave — Find substitutes none — so
	// there is nothing here to say where the path came from.
	var target *run.NotFoundError
	if wrapped := fmt.Errorf("find: %w", err); !errors.As(wrapped, &target) || target.Dir != "/srv/results" {
		t.Fatalf("errors.As does not recover the NotFoundError from %v", wrapped)
	}
}
