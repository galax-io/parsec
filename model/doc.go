// Package model is the canonical form of a load-test result.
//
// Every source — Gatling today, JMeter, k6, Locust and Yandex.Tank as their
// adapters land — is decoded into these types, and these are what a consumer
// depends on. Reading a run through this package, nothing names the tool that
// produced it except [Run.Tool], so a report written once works for every
// source and two tools can be compared on equal terms.
//
// # What a run holds, and what streams
//
// A [Run] carries only what does not grow with the length of the run: its
// identity, the tool and version, what the source can and cannot record, any
// warning its version gate raised, and the opaque payloads the source wrote
// once per declared requirement. Everything that grows — samples, group
// traversals, virtual-user events and run-level errors alike — arrives as an
// [Item] through the stream a source package hands back, one at a time. A run
// large enough to matter is larger than the memory available to hold it, and a
// consumer that needs all of one kind at once collects it and owns that memory.
//
// # Absence
//
// Two questions about a missing value have two different answers, and neither
// implies the other. [Capabilities] says what the source can *never* record: a
// Gatling request has no response code, for any run, ever, and a consumer reads
// that before rendering anything rather than discovering that a whole column is
// empty. [Opt] says what *this* record does not carry: a source that usually
// records a response code did not record one here.
//
// What a source cannot provide is reported as absent. It is never filled with a
// zero, an average or a guess, and how to show an absence is the consumer's
// decision rather than this package's.
//
// A time is the one value that says so without [Opt]: a start or an event time
// the source could not resolve is the zero [time.Time], the standard library's
// own convention for a time that was never set, and a recorded instant is never
// the zero Time — nothing recorded is before 1970, and the zero Time is the year
// 1. Ask [time.Time.IsZero] before treating one as a measurement.
//
// # The primitives a consumer folds
//
// A consumer that wants a number folds the stream once, and this package hands
// it the definitions the fold rests on rather than the arithmetic. [Position] is
// where a sample or a group traversal was recorded — its groups and its name as
// one comparable value — so two consumers bucket a run by the same key without
// agreeing on a spelling. [Bounds] is where the run begins and where it ends, as
// the tool's own report bounds it, extended one item at a time; every rate
// divides by that span, and a virtual-user event can set either end of it.
// [Outcome] is what a failure is, and the stream of [Item] values is what is
// walked. What a consumer computes from these — counts, means, percentiles,
// ranges, series — is its own, and the package example shows the loop.
//
// # What this package does not do
//
// It computes nothing. There is no count, mean, percentile, range or series
// here, and so no entry point can return a statistic that has quietly pooled a
// failed sample into a successful one. The distinction is structural rather
// than conventional: [Sample.Outcome] is read from what the source recorded and
// never inferred, [Sample.Failure] is set if and only if the outcome is
// [OutcomeFailure], and a group carries its own outcome rather than the
// conjunction of what it enclosed. The arithmetic is the consumer's: counts,
// means, percentiles, ranges and series are computed in galaxio-cli, from these
// types. What this package owns is the definitions they are computed from.
//
// It holds no tool-specific type. The records a decoder reads off the wire stay
// in that decoder's own package, where they are the log's events rather than a
// result; nothing here mirrors them field for field.
package model
