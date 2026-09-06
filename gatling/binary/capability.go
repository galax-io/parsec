package binary

import (
	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/model"
)

// Tool is what this source is called in [model.Run].Tool.
const Tool = "gatling"

// The range this codec accepts without a warning. It equals the versions the
// golden corpus covers, and widening it means recording a new entry first.
//
// The floor is 3.13.1 rather than 3.13.0, where the format begins, because
// 3.13.0 cannot be recorded: it fails to read back the assertion records it
// writes, so no run of it produces a report. See
// testdata/corpus/gatling/3.13.1/RECORDING.md.
var (
	minVersion = gatling.Version{Major: 3, Minor: 13, Patch: 1}
	maxVersion = gatling.Version{Major: 3, Minor: 15, Patch: 1}
)

// versionPolicy is this codec's gate, stated once. The decision it applies lives
// in gatling, so that both codecs share one rule rather than a copy each.
var versionPolicy = gatling.Policy{Min: minVersion, Max: maxVersion}

// SupportedVersions returns the oldest and newest Gatling release this codec
// accepts without a warning: 3.13.1 through 3.15.1, the range the golden corpus
// covers. Widening it means recording a new corpus entry first.
//
// A 3.13.0 log is refused although this codec could read it. That version writes
// the format but cannot generate a report, so no run of it can carry the second
// account of its own numbers a corpus entry needs, and the range follows the
// corpus rather than what the decoder believes it could manage.
func SupportedVersions() (oldest, newest gatling.Version) {
	return minVersion, maxVersion
}

// Capabilities returns what a Gatling binary simulation.log records, and by
// omission what it never does.
//
// Provided: the duration of a request; a group traversal's wall-clock duration,
// its cumulated response time and its own status. The two group durations are
// different quantities — wall clock includes any pause inside the traversal, the
// cumulated figure is the sum of the durations of the requests it enclosed — and
// the record carries both.
//
// Absent, and reported as absent rather than filled in:
//
//   - the scenario a request ran under. A user record names a scenario and a
//     request record does not, so a request cannot be attributed to one.
//   - the response code, the request and response body sizes, and the connect,
//     DNS and TLS timings. The format records none of them.
//   - the identity of the virtual user that made a request. No version in the
//     supported range records one.
//   - a classified failure type. Gatling writes free text, and this package does
//     not invent a taxonomy for it.
//   - what the assertion payload encodes. It is carried through unread.
//   - per-interval series. The log is a stream of events, not buckets.
//
// It is the same set the text codec reports, which is asserted by a test rather
// than assumed: the two formats record the same things and omit the same things,
// so a report written against the model cannot tell which log it came from.
func Capabilities() model.Capabilities {
	return model.NewCapabilities(
		model.FieldSampleDuration,
		model.FieldGroupDuration,
		model.FieldGroupCumulatedDuration,
		model.FieldGroupOutcome,
	)
}
