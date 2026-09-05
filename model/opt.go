package model

// Opt is a value the source may not have recorded.
//
// The zero Opt is unset, so a field nobody filled reads as absent rather than
// as zero — which is the whole point: a source that measured zero bytes and a
// source that records no byte count must not read alike.
//
// Opt says this record does not carry the value. [Capabilities] says the source
// never records it. Neither implies the other, which is why both exist.
//
// It is a value rather than a pointer because a run is read as a stream under a
// fixed memory budget, and a pointer per optional field would be an allocation
// per field per sample. T is constrained to a comparable type so that two Opt
// values compare equal exactly when both are unset or both hold equal values,
// which is what lets a test assert absence with ==.
type Opt[T comparable] struct {
	value T
	set   bool
}

// Some returns an Opt holding v.
func Some[T comparable](v T) Opt[T] {
	return Opt[T]{value: v, set: true}
}

// Get returns the recorded value and whether there was one. When there was not,
// the value is the zero of its type and must not be read as a measurement.
func (o Opt[T]) Get() (T, bool) {
	return o.value, o.set
}

// IsSet reports whether the source recorded a value.
func (o Opt[T]) IsSet() bool { return o.set }
