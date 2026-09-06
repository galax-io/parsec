package model

import "time"

// Bounds is where a run begins and where it ends, as the tool's own report
// bounds it: the earliest of every sample start, group start and virtual-user
// START, and the latest of every sample end, group end and virtual-user event.
// Every rate a report prints divides by this span, and a virtual user can set
// either end of it, which is why it is a definition this package owns rather
// than something each consumer derives.
//
// A consumer folds the stream once and calls Extend on every item; nothing is
// retained but the two instants. The zero Bounds has counted nothing and is
// ready to use. The fold is a minimum and a maximum, so the order of items does
// not matter.
//
// What does not count: the run's recorded start ([Run.Start]), run-level
// errors, assertion payloads, and a sample or group end that is absent — such
// an item still contributes its start. A group's end is its start plus its
// wall-clock Duration; CumulatedDuration answers a different question and never
// moves a bound.
//
// An item the source recorded but could not place in time — one whose start is
// the zero [time.Time] — is different, and it makes the bounds unusable rather
// than being skipped. Such an item still belongs to the run, and the model
// carries no end without a start to measure from, so its end is unreachable
// here; reporting a span that silently excludes it would be a span too short and
// a rate too high, with nothing to say so. Start and End report nothing instead.
// For the same reason they report nothing when the end would precede the start.
type Bounds struct {
	start, end time.Time
	// unplaced records that an item counted towards the run and could not be
	// placed in it, which is what makes the two instants an incomplete account
	// of the run rather than a wrong one.
	unplaced bool
}

// Extend widens the bounds to cover it, if it is something that counts. It
// takes a pointer because an Item carries a slot for every kind and Extend is
// called once per item of a run.
func (b *Bounds) Extend(it *Item) {
	switch it.Kind {
	case ItemSample:
		b.cover(it.Sample.Start, it.Sample.Duration)
	case ItemGroup:
		b.cover(it.Group.Start, it.Group.Duration)
	case ItemUser:
		b.coverUser(it.User)
	case ItemError, ItemAssertion, ItemUnknown:
		// Not events that bound a run: the report counts no error towards the
		// span, and a payload has no time at all.
	}
}

// Start returns the instant the run began and true, or the zero time and false
// when nothing that starts a run has been folded or an item could not be placed
// in time.
//
// The receiver is a value, so bounds held in a map or any other unaddressable
// place can still be read.
func (b Bounds) Start() (time.Time, bool) {
	if b.unplaced || b.start.IsZero() {
		return time.Time{}, false
	}

	return b.start, true
}

// End returns the instant the run ended and true, or the zero time and false
// when nothing that ends a run has been folded, an item could not be placed in
// time, or the end folded so far precedes the start. A run can have a start and
// no end: samples whose ends the source did not record, and no virtual-user
// event.
//
// The end is never reported before the start. A virtual-user END extends only
// the end and a sample with no recorded end extends only the start, so the two
// can cross; a consumer dividing a count by a negative span would print a
// negative rate for every row, and no span at all is the honest answer.
func (b Bounds) End() (time.Time, bool) {
	if b.unplaced || b.end.IsZero() || b.end.Before(b.start) {
		return time.Time{}, false
	}

	return b.end, true
}

// cover extends the bounds by an operation that began at start and, when its
// end was recorded, lasted d. An absent start leaves the bounds unusable: there
// is no instant to begin at and none to add a duration to, so the operation's
// own end — which the source may well have recorded — cannot be reached from
// here, and skipping it would shorten the run without saying so.
//
// A duration that is not positive time cannot give an end either. Every source
// this package documents promises a non-negative Duration, and the Gatling path
// keeps that promise, but Bounds is exported and an adapter that broke it would
// otherwise drag the end behind the start.
func (b *Bounds) cover(start time.Time, d Opt[time.Duration]) {
	if start.IsZero() {
		b.unplaced = true

		return
	}

	b.begin(start)

	if v, ok := d.Get(); ok && v >= 0 {
		b.finish(start.Add(v))
	}
}

// coverUser extends the bounds by a virtual-user event: a START can open the
// run, and either kind can close it. An event the source could not place in
// time leaves the bounds unusable, for the reason cover gives.
func (b *Bounds) coverUser(u UserEvent) {
	if u.At.IsZero() {
		b.unplaced = true

		return
	}

	switch u.Kind {
	case UserStart:
		b.begin(u.At)
		b.finish(u.At)
	case UserEnd:
		b.finish(u.At)
	case UserEventUnknown:
		// Never produced by an adapter; an event with no direction counts nothing.
	}
}

func (b *Bounds) begin(at time.Time) {
	if b.start.IsZero() || at.Before(b.start) {
		b.start = at
	}
}

func (b *Bounds) finish(at time.Time) {
	if b.end.IsZero() || at.After(b.end) {
		b.end = at
	}
}
