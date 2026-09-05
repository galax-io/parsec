package text

import "github.com/galax-io/parsec/model"

// Capabilities returns what a Gatling text simulation.log records, and by
// omission what it never does.
//
// Provided: the duration of a request; a group traversal's wall-clock duration,
// its cumulated response time and its own status. The two group durations are
// different quantities — wall clock includes any pause inside the traversal,
// the cumulated figure is the sum of the durations of the requests it enclosed
// — and the record carries both.
//
// Absent, and reported as absent rather than filled in:
//
//   - the scenario a request ran under. The log names a scenario on a USER
//     record and not on a REQUEST, so a request cannot be attributed to one.
//   - the response code, the request and response body sizes, and the connect,
//     DNS and TLS timings. The format records none of them.
//   - the identity of the virtual user that made a request. Neither 3.11.5 nor
//     3.12.0 records one, so a request cannot be attributed to a user.
//   - a classified failure type. Gatling writes free text, and this package
//     does not invent a taxonomy for it.
//   - what the assertion payload encodes. It is carried through unread.
//   - per-interval series. The log is a stream of events, not buckets.
//
// It is the same value as Run.Capabilities and is exported so a caller can ask
// before opening anything.
func Capabilities() model.Capabilities {
	return model.NewCapabilities(
		model.FieldSampleDuration,
		model.FieldGroupDuration,
		model.FieldGroupCumulatedDuration,
		model.FieldGroupOutcome,
	)
}
