package model_test

import (
	"reflect"
	"testing"

	"github.com/galax-io/parsec/model"
)

// Two positions are equal exactly when they address the same kind of thing at
// the same path with the same name, and nothing else: not a request outside a
// group and the same name inside one, not a group traversal and a request that
// reads the same, and not two paths a separator would have merged.
func TestPositionEquality(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		a, b      model.Position
		wantEqual bool
	}{
		{
			name: "the same sample twice", wantEqual: true,
			a: model.NewSamplePosition([]string{"outer", "inner"}, "GET /ok"),
			b: model.NewSamplePosition([]string{"outer", "inner"}, "GET /ok"),
		},
		{
			name: "nil and empty groups are the same absence of a path", wantEqual: true,
			a: model.NewSamplePosition(nil, "GET /ok"),
			b: model.NewSamplePosition([]string{}, "GET /ok"),
		},
		{
			name: "a request outside any group and the same name inside one",
			a:    model.NewSamplePosition(nil, "GET /ok"),
			b:    model.NewSamplePosition([]string{"outer"}, "GET /ok"),
		},
		{
			name: "a group traversal and a request that reads as the same path",
			a:    model.NewGroupPosition([]string{"a", "b"}),
			b:    model.NewSamplePosition([]string{"a"}, "b"),
		},
		{
			name: "one group named with a comma and two groups a comma would join",
			a:    model.NewGroupPosition([]string{"a,b"}),
			b:    model.NewGroupPosition([]string{"a", "b"}),
		},
		{
			name: "a slash inside a name is not a path separator",
			a:    model.NewSamplePosition([]string{"a/b"}, "c"),
			b:    model.NewSamplePosition([]string{"a"}, "b/c"),
		},
		{
			name: "a tab inside a name is not a separator either",
			a:    model.NewGroupPosition([]string{"a\tb"}),
			b:    model.NewGroupPosition([]string{"a", "b"}),
		},
		{
			name: "nor is a NUL",
			a:    model.NewSamplePosition([]string{"a\x00b"}, "r"),
			b:    model.NewSamplePosition([]string{"a", "b"}, "r"),
		},
		{
			name: "an empty name outside any group and an empty name inside an unnamed group",
			a:    model.NewSamplePosition(nil, ""),
			b:    model.NewSamplePosition([]string{""}, ""),
		},
		{
			name: "an empty request name and a group traversal at the same path",
			a:    model.NewSamplePosition([]string{"g"}, ""),
			b:    model.NewGroupPosition([]string{"g"}),
		},
		{
			name: "the zero value and the smallest constructed position",
			a:    model.Position{},
			b:    model.NewSamplePosition(nil, ""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.a == tt.b; got != tt.wantEqual {
				t.Errorf("(%q == %q) = %t, want %t", tt.a, tt.b, got, tt.wantEqual)
			}

			// A map keyed by position must agree with ==, which is what lets two
			// consumers bucket alike.
			seen := map[model.Position]int{tt.a: 1}
			if _, bucketed := seen[tt.b]; bucketed != tt.wantEqual {
				t.Errorf("map lookup of %q after storing %q = %t, want %t", tt.b, tt.a, bucketed, tt.wantEqual)
			}
		})
	}
}

// What a position was made from comes back exactly, so a report can print a
// row without keeping the sample it came from.
func TestPositionRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pos        model.Position
		wantKind   model.PositionKind
		wantGroups []string
		wantName   string
		wantString string
	}{
		{
			name: "a nested sample", pos: model.NewSamplePosition([]string{"outer", "inner, with comma"}, "GET /ok"),
			wantKind: model.PositionSample, wantGroups: []string{"outer", "inner, with comma"}, wantName: "GET /ok",
			wantString: "outer / inner, with comma / GET /ok",
		},
		{
			name: "a sample outside any group", pos: model.NewSamplePosition(nil, "GET /ok"),
			wantKind: model.PositionSample, wantGroups: []string{}, wantName: "GET /ok", wantString: "GET /ok",
		},
		{
			name: "a sample with an empty name", pos: model.NewSamplePosition([]string{"g"}, ""),
			wantKind: model.PositionSample, wantGroups: []string{"g"}, wantName: "", wantString: "g / ",
		},
		{
			name: "a group traversal", pos: model.NewGroupPosition([]string{"outer", "inner"}),
			wantKind: model.PositionGroup, wantGroups: []string{"outer", "inner"}, wantName: "", wantString: "outer / inner [group]",
		},
		{
			name: "a name outside Latin-1", pos: model.NewSamplePosition([]string{"группа"}, "Проверка /ok"),
			wantKind: model.PositionSample, wantGroups: []string{"группа"}, wantName: "Проверка /ok",
			wantString: "группа / Проверка /ok",
		},
		{
			name: "the zero value", pos: model.Position{},
			wantKind: model.PositionUnknown, wantGroups: nil, wantName: "", wantString: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.pos.Kind(); got != tt.wantKind {
				t.Errorf("Kind() = %v, want %v", got, tt.wantKind)
			}

			got := tt.pos.Groups()
			if !reflect.DeepEqual(got, tt.wantGroups) {
				t.Errorf("Groups() = %#v, want %#v", got, tt.wantGroups)
			}

			if (got == nil) != (tt.wantGroups == nil) {
				t.Errorf("Groups() nil-ness = %t, want %t: empty and non-nil for a constructed position, nil only for the zero value",
					got == nil, tt.wantGroups == nil)
			}

			if name := tt.pos.Name(); name != tt.wantName {
				t.Errorf("Name() = %q, want %q", name, tt.wantName)
			}

			if s := tt.pos.String(); s != tt.wantString {
				t.Errorf("String() = %q, want %q", s, tt.wantString)
			}
		})
	}
}

// A group traversal and a sample at what reads as the same path are different
// positions, and a report that prints them must be able to tell them apart. The
// rendering carries the kind for that reason: without it the two rows would
// carry one label, and the failure message of the equality test above — which
// formats both operands with %q — could not describe its own failure either.
func TestPositionRenderingDistinguishesAGroupFromASample(t *testing.T) {
	t.Parallel()

	group := model.NewGroupPosition([]string{"a", "b"})
	sample := model.NewSamplePosition([]string{"a"}, "b")

	if group == sample {
		t.Fatal("the two positions are equal; the rest of this test assumes they are not")
	}

	if group.String() == sample.String() {
		t.Errorf("both render as %q, so a report shows two distinct rows under one label", group)
	}
}

// The Groups slice on a sample is backed by storage the reader reuses between
// calls. A position taken from it must not be: it is kept as a map key for the
// whole run, long after the reader has overwritten that storage.
func TestPositionSurvivesTheReaderReusingItsSlice(t *testing.T) {
	t.Parallel()

	scratch := []string{"outer", "inner"}
	sample := model.Sample{Groups: scratch, Name: "GET /ok"}
	pos := sample.Position()

	// The reader moves on and rewrites its scratch for the next record.
	scratch[0], scratch[1] = "other", "path"

	if want := model.NewSamplePosition([]string{"outer", "inner"}, "GET /ok"); pos != want {
		t.Errorf("the position changed when the reader reused its slice: %q, want %q", pos, want)
	}

	if got := pos.Groups(); !reflect.DeepEqual(got, []string{"outer", "inner"}) {
		t.Errorf("Groups() = %q after the reader reused its slice", got)
	}
}

// The slice Groups returns is the caller's own: writing to it cannot change the
// position, and a second call starts fresh.
func TestPositionGroupsIsTheCallersOwn(t *testing.T) {
	t.Parallel()

	pos := model.NewGroupPosition([]string{"outer", "inner"})

	first := pos.Groups()
	first[0] = "overwritten"

	if got := pos.Groups(); !reflect.DeepEqual(got, []string{"outer", "inner"}) {
		t.Errorf("Groups() = %q after a caller wrote into an earlier result", got)
	}
}

// Two consumers that never agreed on a spelling bucket alike: one keys a map by
// the positions it takes from samples, the other by positions it builds from
// names it knows, and the lookups meet.
func TestTwoConsumersBucketAlike(t *testing.T) {
	t.Parallel()

	items := []model.Item{
		{Kind: model.ItemSample, Sample: model.Sample{Name: "GET /ok"}},
		{Kind: model.ItemSample, Sample: model.Sample{Groups: []string{"outer"}, Name: "GET /ok"}},
		{Kind: model.ItemSample, Sample: model.Sample{Groups: []string{"outer"}, Name: "GET /ok"}},
		{Kind: model.ItemGroup, Group: model.GroupSample{Groups: []string{"outer"}}},
	}

	counted := map[model.Position]int{}

	for i := range items {
		it := &items[i]

		if it.Kind == model.ItemGroup {
			counted[it.Group.Position()]++

			continue
		}

		counted[it.Sample.Position()]++
	}

	want := map[model.Position]int{
		model.NewSamplePosition(nil, "GET /ok"):               1,
		model.NewSamplePosition([]string{"outer"}, "GET /ok"): 2,
		model.NewGroupPosition([]string{"outer"}):             1,
	}

	if !reflect.DeepEqual(counted, want) {
		t.Errorf("counted %v, want %v", counted, want)
	}
}
