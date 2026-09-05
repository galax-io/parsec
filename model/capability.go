package model

import "slices"

// unknownName is what every zero value in this package prints as. Named once
// for the same reason the gatling package names it once: seven copies of a
// string are seven places to mistype it.
const unknownName = "unknown"

// Field names something a source may or may not record.
//
// The set is closed and grows only when the model gains a field. It is what
// [Capabilities] is expressed in, so a report can name what is missing rather
// than printing an empty column and leaving the reader to guess why.
type Field uint16

const (
	// FieldUnknown is the zero value. It names nothing and is neither provided
	// nor reported as absent.
	FieldUnknown Field = iota

	// FieldSampleDuration is how long one recorded operation took. It is the
	// OpenNFR metric loadtest.request.duration.
	FieldSampleDuration
	// FieldSampleScenario is the scenario an operation ran under, recorded on
	// the operation itself.
	FieldSampleScenario
	// FieldSampleResponseCode is the protocol's own result code for one
	// operation.
	FieldSampleResponseCode
	// FieldSampleBytesSent is the request body size of one operation.
	FieldSampleBytesSent
	// FieldSampleBytesReceived is the response body size of one operation.
	FieldSampleBytesReceived
	// FieldSampleFailureType is a classification of a failure, as against the
	// free text a source may record instead.
	FieldSampleFailureType
	// FieldSampleUserIdentity is which virtual user performed an operation.
	FieldSampleUserIdentity

	// FieldGroupDuration is wall clock across one traversal of a group,
	// including any pause inside it.
	FieldGroupDuration
	// FieldGroupCumulatedDuration is the sum of the durations of the operations
	// a group traversal enclosed. It is the OpenNFR metric
	// loadtest.group.duration.
	FieldGroupCumulatedDuration
	// FieldGroupOutcome is a group traversal's own success or failure, which is
	// not the conjunction of what it enclosed.
	FieldGroupOutcome

	// FieldConnectTiming is the time spent establishing a connection.
	FieldConnectTiming
	// FieldDNSTiming is the time spent resolving a name.
	FieldDNSTiming
	// FieldTLSTiming is the time spent on the TLS handshake.
	FieldTLSTiming

	// FieldRequirements is what an opaque payload the source wrote encodes. A
	// source that carries the payload through unread does not provide this.
	FieldRequirements
	// FieldIntervalSeries is per-interval measurements over the run, as against
	// one figure for the whole of it.
	FieldIntervalSeries
)

// fieldNames is indexed by Field. A field added without a name here fails
// TestFieldStringNamesEveryField rather than printing as a number in a report.
var fieldNames = [...]string{
	FieldUnknown:                unknownName,
	FieldSampleDuration:         "sample duration",
	FieldSampleScenario:         "sample scenario",
	FieldSampleResponseCode:     "sample response code",
	FieldSampleBytesSent:        "sample bytes sent",
	FieldSampleBytesReceived:    "sample bytes received",
	FieldSampleFailureType:      "sample failure type",
	FieldSampleUserIdentity:     "sample user identity",
	FieldGroupDuration:          "group duration",
	FieldGroupCumulatedDuration: "group cumulated duration",
	FieldGroupOutcome:           "group outcome",
	FieldConnectTiming:          "connect timing",
	FieldDNSTiming:              "dns timing",
	FieldTLSTiming:              "tls timing",
	FieldRequirements:           "requirements",
	FieldIntervalSeries:         "interval series",
}

// String returns the field's name, for a report that names what is missing.
func (f Field) String() string {
	if int(f) >= len(fieldNames) {
		return fieldNames[FieldUnknown]
	}

	return fieldNames[f]
}

// Capabilities is a source's own statement of what it records.
//
// A consumer reads it before rendering anything, rather than discovering that
// every value of a column is empty and having to guess whether the run was
// quiet or the source is blind.
//
// The set held is what the source *provides*. A field this package gains later
// therefore reads as absent for every adapter written before it, until one
// claims it. That is the conservative direction: the opposite storage — holding
// the absences — would report a new field as present for a source that never
// recorded it, and a consumer would be told a measurement exists when it does
// not.
//
// The zero Capabilities provides nothing, which is the honest reading of a
// source that has said nothing about itself.
type Capabilities struct {
	provided [len(fieldNames)]bool
}

// NewCapabilities returns the capabilities of a source that provides exactly
// the given fields and nothing else.
//
// FieldUnknown and any field outside the known set are ignored: they name
// nothing, so neither providing nor withholding them says anything.
func NewCapabilities(provided ...Field) Capabilities {
	var c Capabilities

	for _, f := range provided {
		if f == FieldUnknown || int(f) >= len(fieldNames) {
			continue
		}

		c.provided[f] = true
	}

	return c
}

// Provides reports whether the source records f.
func (c Capabilities) Provides(f Field) bool {
	if f == FieldUnknown || int(f) >= len(c.provided) {
		return false
	}

	return c.provided[f]
}

// Absent returns every known field the source does not record, in ascending
// field order so a report reads the same way twice. The result is the caller's
// own slice.
//
// FieldUnknown is never named: it is the zero value rather than a measurement
// anything could have made.
func (c Capabilities) Absent() []Field {
	absent := make([]Field, 0, len(c.provided))

	for f := FieldUnknown + 1; int(f) < len(c.provided); f++ {
		if !c.provided[f] {
			absent = append(absent, f)
		}
	}

	return slices.Clip(absent)
}
