package model_test

import (
	"testing"
	"time"

	"github.com/galax-io/parsec/model"
)

// Kind decides which field is read, and nothing else does. Asserting that the
// other fields of a literal hold their zero value asserts that Go zeroes what a
// literal omits; Item has no constructor, so nothing in this package could make
// that fail. What can fail is dispatch: a consumer switching on Kind must read
// the field the kind selects even when another field happens to be populated,
// and this package's own consumer of Kind is [model.Bounds.Extend].
//
// Each item below carries a decoy — a plausible value in a field its Kind does
// not select, set an hour away from the real one — so a fold that read the wrong
// field would place the run an hour out.
func TestKindDecidesWhichFieldIsRead(t *testing.T) {
	t.Parallel()

	var (
		selected = time.Unix(0, 0).UTC()
		decoy    = selected.Add(time.Hour)
	)

	tests := []struct {
		name string
		item model.Item
		want time.Time // where the fold must place the run
	}{
		{
			name: "a group is read from Group, not from the Sample beside it",
			item: model.Item{
				Kind:   model.ItemGroup,
				Group:  model.GroupSample{Groups: []string{"outer"}, Start: selected},
				Sample: model.Sample{Name: "decoy", Start: decoy},
			},
			want: selected,
		},
		{
			name: "a sample is read from Sample, not from the Group beside it",
			item: model.Item{
				Kind:   model.ItemSample,
				Sample: model.Sample{Name: "GET /ok", Start: selected},
				Group:  model.GroupSample{Groups: []string{"decoy"}, Start: decoy},
			},
			want: selected,
		},
		{
			name: "a user event is read from User, not from the Sample beside it",
			item: model.Item{
				Kind:   model.ItemUser,
				User:   model.UserEvent{Scenario: "s", Kind: model.UserStart, At: selected},
				Sample: model.Sample{Name: "decoy", Start: decoy},
			},
			want: selected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			item := tt.item

			var b model.Bounds

			b.Extend(&item)

			start, ok := b.Start()
			if !ok || !start.Equal(tt.want) {
				t.Errorf("Start() = %v, %t; want %v — the fold read a field this Kind does not select",
					start, ok, tt.want)
			}
		})
	}
}

// An error counts towards no bound however it is filled in: the report counts no
// error towards the span, and a decoy in every other field must not change that.
func TestAnErrorItemBoundsNothing(t *testing.T) {
	t.Parallel()

	at := time.Unix(0, 0).UTC()

	item := model.Item{
		Kind:   model.ItemError,
		Error:  model.RunError{Message: "boom", At: at},
		Sample: model.Sample{Name: "decoy", Start: at},
		Group:  model.GroupSample{Groups: []string{"decoy"}, Start: at},
		User:   model.UserEvent{Scenario: "s", Kind: model.UserStart, At: at},
	}

	var b model.Bounds

	b.Extend(&item)

	if _, ok := b.Start(); ok {
		t.Error("an error item moved the start; the report counts no error towards the span")
	}

	if _, ok := b.End(); ok {
		t.Error("an error item moved the end; the report counts no error towards the span")
	}
}

func TestZeroItemCarriesNothing(t *testing.T) {
	t.Parallel()

	var it model.Item

	if it.Kind != model.ItemUnknown {
		t.Errorf("the zero Item has kind %v, want %v", it.Kind, model.ItemUnknown)
	}

	if it.Sample.Outcome != model.OutcomeUnknown {
		t.Error("the zero Item carries an outcome")
	}

	if it.Sample.Duration.IsSet() {
		t.Error("the zero Item carries a duration")
	}
}

func TestEnumStringsCoverEveryConstant(t *testing.T) {
	t.Parallel()

	t.Run("ItemKind", func(t *testing.T) {
		t.Parallel()

		want := map[model.ItemKind]string{
			model.ItemUnknown: "unknown",
			model.ItemSample:  "sample",
			model.ItemGroup:   "group",
			model.ItemUser:    "user",
			model.ItemError:   "error",
		}
		for k, s := range want {
			if got := k.String(); got != s {
				t.Errorf("ItemKind(%d).String() = %q, want %q", int(k), got, s)
			}
		}
	})

	t.Run("Outcome", func(t *testing.T) {
		t.Parallel()

		want := map[model.Outcome]string{
			model.OutcomeUnknown: "unknown",
			model.OutcomeSuccess: "success",
			model.OutcomeFailure: "failure",
		}
		for o, s := range want {
			if got := o.String(); got != s {
				t.Errorf("Outcome(%d).String() = %q, want %q", int(o), got, s)
			}
		}
	})

	t.Run("UserEventKind", func(t *testing.T) {
		t.Parallel()

		want := map[model.UserEventKind]string{
			model.UserEventUnknown: "unknown",
			model.UserStart:        "start",
			model.UserEnd:          "end",
		}
		for k, s := range want {
			if got := k.String(); got != s {
				t.Errorf("UserEventKind(%d).String() = %q, want %q", int(k), got, s)
			}
		}
	})

	t.Run("PositionKind", func(t *testing.T) {
		t.Parallel()

		want := map[model.PositionKind]string{
			model.PositionUnknown: "unknown",
			model.PositionSample:  "sample",
			model.PositionGroup:   "group",
			// Out of range names the type and the number, as every exported
			// enum in this module does; TestOutOfRangeValuesNameTheTypeAndTheNumber
			// walks all five of this package's.
			model.PositionKind(9): "PositionKind(9)",
		}
		for k, s := range want {
			if got := k.String(); got != s {
				t.Errorf("PositionKind(%d).String() = %q, want %q", int(k), got, s)
			}
		}
	})
}

func TestWarningStringNamesTheVersion(t *testing.T) {
	t.Parallel()

	w := model.Warning{Version: "3.99.0", Reason: "no recording covers it"}

	if got, want := w.String(), "3.99.0: no recording covers it"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
