package model

import "time"

// Warning is something a source's version gate raised about a run that was read
// anyway — typically that no recording covers the version that wrote it.
//
// It travels on the [Run] so that a result decoded from an unverified version
// stays identifiable as one. A conversion that dropped it would launder an
// unverified result into one that looks verified.
type Warning struct {
	// Version is the version the source named.
	Version string
	// Reason says why the run is unverified, in a sentence a report can print.
	Reason string
}

// String returns the version and why it is unverified.
func (w Warning) String() string { return w.Version + ": " + w.Reason }

// Run is one execution, described by everything about it that does not grow
// with its length.
//
// It holds no samples, group traversals, virtual-user events or errors: all
// four arrive through the stream a source package hands back beside it, because
// a run large enough to matter is larger than the memory available to hold it.
// A consumer that needs all of one kind at once collects it and owns that
// memory.
//
// Assertions stays here because a source writes one per declared requirement —
// a handful, however long the run.
type Run struct {
	// ID is the identifier the tool assigned this run.
	ID string
	// Name is the simulation or scenario the run executed.
	Name string
	// Description is free text the run carried, empty where it recorded none.
	Description string
	// Start is the wall-clock start of the run, in UTC, exactly as recorded.
	Start time.Time
	// Tool is what produced the run, such as "gatling".
	Tool string
	// ToolVersion is the version that tool stated, such as "3.11.5".
	ToolVersion string
	// Capabilities is what this source can and cannot record. Read it before
	// rendering anything.
	Capabilities Capabilities
	// Warnings is empty for a version the project has a recording for.
	Warnings []Warning
	// Assertions is the opaque payloads the source wrote, one per declared
	// requirement, verbatim. This module does not decode or interpret them.
	Assertions []string
}

// ItemKind says which field of an [Item] carries the value.
type ItemKind uint8

const (
	// ItemUnknown is the zero value and is never produced by an adapter.
	ItemUnknown ItemKind = iota
	// ItemSample selects Item.Sample.
	ItemSample
	// ItemGroup selects Item.Group.
	ItemGroup
	// ItemUser selects Item.User.
	ItemUser
	// ItemError selects Item.Error.
	ItemError
)

// String returns "sample", "group", "user", "error" or "unknown".
func (k ItemKind) String() string {
	switch k {
	case ItemSample:
		return "sample"
	case ItemGroup:
		return "group"
	case ItemUser:
		return "user"
	case ItemError:
		return "error"
	case ItemUnknown:
		return unknownName
	default:
		return unknownName
	}
}

// Item is one thing a run's stream yields.
//
// Kind selects the field that carries the value; every other field holds its
// zero value, so a consumer that switches on Kind can never read a stale value
// from the item before it.
//
// A discriminated struct rather than an interface, so that streaming a run
// allocates nothing per item.
type Item struct {
	// Kind says which of the fields below is meaningful.
	Kind ItemKind
	// Sample is meaningful when Kind is ItemSample.
	Sample Sample
	// Group is meaningful when Kind is ItemGroup.
	Group GroupSample
	// User is meaningful when Kind is ItemUser.
	User UserEvent
	// Error is meaningful when Kind is ItemError.
	Error RunError
}
