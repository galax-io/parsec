package model_test

import (
	"fmt"
	"strconv"
	"time"

	"github.com/galax-io/parsec/model"
)

// report is the shape of consumer code this package exists for: it folds a
// run's items into the numbers a report prints, and nothing in it names a tool.
// The same function serves a Gatling run, and a JMeter or k6 one when those
// adapters land.
func report(run model.Run, items []model.Item) (total, ok, ko int, slowest time.Duration) {
	for _, it := range items {
		if it.Kind != model.ItemSample {
			continue
		}

		total++

		switch it.Sample.Outcome {
		case model.OutcomeSuccess:
			ok++
		case model.OutcomeFailure:
			ko++
		case model.OutcomeUnknown:
			// An adapter never produces this; counting it as neither keeps a
			// lost outcome visible instead of inflating the successes.
		}

		if d, has := it.Sample.Duration.Get(); has {
			slowest = max(slowest, d)
		}
	}

	_ = run

	return total, ok, ko, slowest
}

// A consumer reads a complete run — its totals, its success and failure split,
// and what the source could not measure — through the canonical types alone.
func Example() {
	run := model.Run{
		ID:           "corpussimulation",
		Name:         "io.galaxio.parsec.corpus.CorpusSimulation",
		Tool:         "gatling",
		ToolVersion:  "3.11.5",
		Capabilities: model.NewCapabilities(model.FieldSampleDuration),
	}

	items := []model.Item{
		{Kind: model.ItemUser, User: model.UserEvent{Scenario: "Corpus recording", Kind: model.UserStart}},
		{Kind: model.ItemSample, Sample: model.Sample{
			Name:     "GET /ok",
			Outcome:  model.OutcomeSuccess,
			Duration: model.Some(11 * time.Millisecond),
		}},
		{Kind: model.ItemSample, Sample: model.Sample{
			Groups:   []string{"outer"},
			Name:     "GET /slow",
			Outcome:  model.OutcomeSuccess,
			Duration: model.Some(1503 * time.Millisecond),
		}},
		{Kind: model.ItemSample, Sample: model.Sample{
			Groups:   []string{"outer"},
			Name:     "GET /fail",
			Outcome:  model.OutcomeFailure,
			Duration: model.Some(4 * time.Millisecond),
			Failure:  model.Some(model.Failure{Message: "status.find.is(200), but actually found 500"}),
		}},
		{Kind: model.ItemError, Error: model.RunError{Message: "unresolvable url"}},
	}

	total, ok, ko, slowest := report(run, items)

	fmt.Printf("%s %s: %d requests, %d ok, %d ko, slowest %v\n",
		run.Tool, run.ToolVersion, total, ok, ko, slowest)

	// What the source could not measure is asked once, before anything is
	// rendered — not discovered by finding a whole column empty.
	fmt.Println("no response code recorded:", !run.Capabilities.Provides(model.FieldSampleResponseCode))

	// Output:
	// gatling 3.11.5: 3 requests, 2 ok, 1 ko, slowest 1.503s
	// no response code recorded: true
}

// A failure carries what the source recorded; a success carries none at all.
func ExampleSample_failure() {
	failed := model.Sample{
		Name:    "GET /fail",
		Outcome: model.OutcomeFailure,
		Failure: model.Some(model.Failure{Message: "status.find.is(200), but actually found 500"}),
	}

	if f, ok := failed.Failure.Get(); ok {
		fmt.Println(f.Message)
	}

	succeeded := model.Sample{Name: "GET /ok", Outcome: model.OutcomeSuccess}
	fmt.Println("carries a failure:", succeeded.Failure.IsSet())

	// Output:
	// status.find.is(200), but actually found 500
	// carries a failure: false
}

func show(v model.Opt[int64]) string {
	n, ok := v.Get()
	if !ok {
		return "—"
	}

	return strconv.FormatInt(n, 10)
}

// An absent value is not a zero. A source that records no byte count and one
// that measured zero bytes must not read alike.
func ExampleOpt() {
	var absent model.Opt[int64]

	measured := model.Some(int64(0))

	fmt.Println("absent is set:", absent.IsSet())
	fmt.Println("a measured zero is set:", measured.IsSet())

	// A report decides for itself how to show an absence; this package never
	// decides by substituting a value.
	fmt.Println("rendered:", show(absent), show(measured))

	// Output:
	// absent is set: false
	// a measured zero is set: true
	// rendered: — 0
}
