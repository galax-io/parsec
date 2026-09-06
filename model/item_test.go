package model_test

import (
	"testing"
	"time"

	"github.com/galax-io/parsec/model"
)

// Kind selects one field; the rest hold their zero value. A consumer that
// switches on Kind must never be able to read a value the item does not carry.
func TestItemKindSelectsOneField(t *testing.T) {
	t.Parallel()

	at := time.Unix(0, 0).UTC()

	tests := []struct {
		name string
		item model.Item
	}{
		{
			name: "sample",
			item: model.Item{
				Kind:   model.ItemSample,
				Sample: model.Sample{Name: "GET /ok", Start: at, Outcome: model.OutcomeSuccess},
			},
		},
		{
			name: "group",
			item: model.Item{
				Kind:  model.ItemGroup,
				Group: model.GroupSample{Groups: []string{"outer"}, Start: at, Outcome: model.OutcomeSuccess},
			},
		},
		{
			name: "user",
			item: model.Item{
				Kind: model.ItemUser,
				User: model.UserEvent{Scenario: "s", Kind: model.UserStart, At: at},
			},
		},
		{
			name: "error",
			item: model.Item{
				Kind:  model.ItemError,
				Error: model.RunError{Message: "boom", At: at},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Sample and GroupSample carry a []string and so are not
			// comparable; every other field of each is, and a zero one is what
			// an unselected field must hold.
			if tt.item.Kind != model.ItemSample {
				if tt.item.Sample.Name != "" || tt.item.Sample.Outcome != model.OutcomeUnknown ||
					!tt.item.Sample.Start.IsZero() || tt.item.Sample.Groups != nil {
					t.Error("Sample is set on an item that does not select it")
				}
			}

			if tt.item.Kind != model.ItemGroup {
				if tt.item.Group.Outcome != model.OutcomeUnknown ||
					!tt.item.Group.Start.IsZero() || tt.item.Group.Groups != nil {
					t.Error("Group is set on an item that does not select it")
				}
			}

			var zero model.Item

			if tt.item.Kind != model.ItemUser && tt.item.User != zero.User {
				t.Error("User is set on an item that does not select it")
			}

			if tt.item.Kind != model.ItemError && tt.item.Error != zero.Error {
				t.Error("Error is set on an item that does not select it")
			}
		})
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
			model.PositionKind(9): "unknown",
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
