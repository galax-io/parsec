package model_test

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/galax-io/parsec/gatling/simlog"
	"github.com/galax-io/parsec/model"
)

// split is a consumer's own accumulator. Nothing in this package counts.
type split struct{ ok, ko int }

func (s *split) add(o model.Outcome) {
	switch o {
	case model.OutcomeSuccess:
		s.ok++
	case model.OutcomeFailure:
		s.ko++
	case model.OutcomeUnknown:
		// An adapter never produces this; a lost outcome is left visible
		// rather than counted as either.
	}
}

// A consumer folds a run in one pass over the stream: it buckets by Position,
// extends one Bounds, and does its own arithmetic — here the counts and the mean
// request rate Gatling's console prints. This package hands over the two
// definitions and computes nothing. The figures printed below are the 3.15.1
// recording's own console summary: 102 requests, 84 OK, 18 KO, and 25.5, 21 and
// 4.5 requests per second over a span Gatling rounds up to 4 seconds.
func Example_fold() {
	f, err := os.Open("../testdata/corpus/gatling/3.15.1/simulation.log")
	if err != nil {
		fmt.Println(err)

		return
	}
	defer func() { _ = f.Close() }()

	// simlog identifies the log format from its first bytes; a text log and a
	// binary log take the same path from here on.
	rd, err := simlog.NewRunReader(f)
	if err != nil {
		fmt.Println(err)

		return
	}

	var (
		requests = map[model.Position]*split{}
		groups   = map[model.Position]*split{}
		total    split
		bounds   model.Bounds
	)

	for {
		it, err := rd.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			fmt.Println(err)

			return
		}

		// Every item may move a bound; only samples and groups have a position.
		bounds.Extend(&it)

		switch it.Kind {
		case model.ItemSample:
			pos := it.Sample.Position()
			if requests[pos] == nil {
				requests[pos] = &split{}
			}

			requests[pos].add(it.Sample.Outcome)
			total.add(it.Sample.Outcome)
		case model.ItemGroup:
			pos := it.Group.Position()
			if groups[pos] == nil {
				groups[pos] = &split{}
			}

			groups[pos].add(it.Group.Outcome)
		case model.ItemUser, model.ItemError, model.ItemAssertion, model.ItemUnknown:
			// Nothing to bucket.
		}
	}

	fmt.Printf("%d requests: %d ok, %d ko, at %d positions; %d group positions\n",
		total.ok+total.ko, total.ok, total.ko, len(requests), len(groups))

	// Both flags are read before anything divides by the span. They are the
	// only thing that distinguishes a run this fold could measure from one it
	// could not, and a rate computed without them is a number with nothing
	// behind it.
	start, haveStart := bounds.Start()

	end, haveEnd := bounds.End()
	if !haveStart || !haveEnd {
		fmt.Println("this run cannot be timed, so no rate is printed")

		return
	}

	// The rounding is the consumer's: Gatling divides by the span in whole
	// seconds, rounded up.
	seconds := math.Ceil(end.Sub(start).Seconds())

	fmt.Printf("run span %.0f s: %.4g rps (%.4g ok, %.4g ko)\n",
		seconds, float64(total.ok+total.ko)/seconds, float64(total.ok)/seconds, float64(total.ko)/seconds)

	// Output:
	// 102 requests: 84 ok, 18 ko, at 7 positions; 2 group positions
	// run span 4 s: 25.5 rps (21 ok, 4.5 ko)
}
