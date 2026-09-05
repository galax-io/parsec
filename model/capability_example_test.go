package model_test

import (
	"fmt"

	"github.com/galax-io/parsec/model"
)

// A report asks what a source cannot measure once, before it renders anything.
// The alternative — rendering a column and discovering every value is empty —
// cannot tell "the source never records this" from "this run was quiet".
func ExampleCapabilities() {
	// What a source declares. A Gatling text log records a request's duration
	// and a group's timings and status, and nothing else on this list.
	caps := model.NewCapabilities(
		model.FieldSampleDuration,
		model.FieldGroupDuration,
		model.FieldGroupCumulatedDuration,
		model.FieldGroupOutcome,
	)

	columns := []struct {
		heading string
		field   model.Field
	}{
		{heading: "duration", field: model.FieldSampleDuration},
		{heading: "response code", field: model.FieldSampleResponseCode},
		{heading: "bytes received", field: model.FieldSampleBytesReceived},
	}

	for _, c := range columns {
		if caps.Provides(c.field) {
			fmt.Printf("%-14s render it\n", c.heading)

			continue
		}

		fmt.Printf("%-14s this source does not record it\n", c.heading)
	}

	fmt.Println("fields this source cannot measure:", len(caps.Absent()))

	// Output:
	// duration       render it
	// response code  this source does not record it
	// bytes received this source does not record it
	// fields this source cannot measure: 11
}
