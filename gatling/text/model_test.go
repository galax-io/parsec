package text_test

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/text"
	"github.com/galax-io/parsec/model"
)

// modelPreamble is the two lines every synthetic log here starts with: one assertion
// payload, then the run header naming a covered version.
const modelPreamble = "ASSERTION\tAAEBAAICAAAAAAAAAPA/\n" +
	"RUN\tio.galaxio.parsec.corpus.CorpusSimulation\tcorpussimulation\t1788379354534\t \t3.11.5\n"

// modelItems reads a whole log into a slice. Test-only: a real consumer streams.
func modelItems(t *testing.T, log string) []model.Item {
	t.Helper()

	rd, err := text.NewRunReader(strings.NewReader(log))
	if err != nil {
		t.Fatalf("NewRunReader: %v", err)
	}

	var got []model.Item

	for {
		it, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return got
		}

		if err != nil {
			t.Fatalf("Next: %v", err)
		}

		// Groups is valid only until the next call, so a test that keeps modelItems
		// must copy it — the same rule a consumer follows.
		it.Sample.Groups = slices.Clone(it.Sample.Groups)
		it.Group.Groups = slices.Clone(it.Group.Groups)
		got = append(got, it)
	}
}

func TestRunHeaderBecomesTheRun(t *testing.T) {
	t.Parallel()

	rd, err := text.NewRunReader(strings.NewReader(modelPreamble))
	if err != nil {
		t.Fatalf("NewRunReader: %v", err)
	}

	run := rd.Run()

	if got, want := run.ID, "corpussimulation"; got != want {
		t.Errorf("ID = %q, want %q", got, want)
	}

	if got, want := run.Name, "io.galaxio.parsec.corpus.CorpusSimulation"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}

	// The header wrote a lone space, which is how Gatling records an absent
	// description; the decoder already reads that as empty.
	if run.Description != "" {
		t.Errorf("Description = %q, want empty", run.Description)
	}

	if got, want := run.Tool, "gatling"; got != want {
		t.Errorf("Tool = %q, want %q", got, want)
	}

	if got, want := run.ToolVersion, "3.11.5"; got != want {
		t.Errorf("ToolVersion = %q, want %q", got, want)
	}

	if got, want := run.Start.UTC(), time.UnixMilli(1788379354534).UTC(); !got.Equal(want) {
		t.Errorf("Start = %v, want %v", got, want)
	}

	if got, want := run.Assertions, []string{"AAEBAAICAAAAAAAAAPA/"}; !slices.Equal(got, want) {
		t.Errorf("Assertions = %v, want %v", got, want)
	}

	if len(run.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none for a covered version", run.Warnings)
	}
}

// oneSample reads a log expected to hold exactly one request and returns it.
func oneSample(t *testing.T, line string) model.Sample {
	t.Helper()

	got := modelItems(t, modelPreamble+line)
	if len(got) != 1 {
		t.Fatalf("got %d items, want 1", len(got))
	}

	if got[0].Kind != model.ItemSample {
		t.Fatalf("Kind = %v, want %v", got[0].Kind, model.ItemSample)
	}

	return got[0].Sample
}

// checkUnrecorded asserts that nothing the source never records was filled in.
func checkUnrecorded(t *testing.T, s model.Sample) {
	t.Helper()

	if s.Scenario.IsSet() || s.ResponseCode.IsSet() || s.BytesSent.IsSet() || s.BytesReceived.IsSet() {
		t.Error("a sample carries a value the source never records")
	}
}

func TestRequestBecomesASample(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		line       string
		wantGroups []string
		wantName   string
		wantStatus model.Outcome
		wantDur    time.Duration
	}{
		{
			name:       "top level, ok",
			line:       "REQUEST\t\tGET /ok\t1788379356162\t1788379356173\tOK\t \n",
			wantGroups: nil,
			wantName:   "GET /ok",
			wantStatus: model.OutcomeSuccess,
			wantDur:    11 * time.Millisecond,
		},
		{
			name:       "nested two deep, ko",
			line:       "REQUEST\touter,inner  with comma\tGET /fail\t1788379356162\t1788379356200\tKO\tstatus.find.is(200), but actually found 500\n",
			wantGroups: []string{"outer", "inner  with comma"},
			wantName:   "GET /fail",
			wantStatus: model.OutcomeFailure,
			wantDur:    38 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := oneSample(t, tt.line)

			if !slices.Equal(s.Groups, tt.wantGroups) {
				t.Errorf("Groups = %q, want %q", s.Groups, tt.wantGroups)
			}

			if s.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", s.Name, tt.wantName)
			}

			if s.Outcome != tt.wantStatus {
				t.Errorf("Outcome = %v, want %v", s.Outcome, tt.wantStatus)
			}

			if d, ok := s.Duration.Get(); !ok || d != tt.wantDur {
				t.Errorf("Duration = %v (set %t), want %v", d, ok, tt.wantDur)
			}

			checkUnrecorded(t, s)
		})
	}
}

// A failure carries what the source recorded and a success carries none at all.
// Gatling records free text rather than a classification, so Type stays empty
// and Capabilities says so — inventing a taxonomy would be faking.
func TestSampleFailureFollowsTheOutcome(t *testing.T) {
	t.Parallel()

	const message = "status.find.is(200), but actually found 500"

	failed := oneSample(t, "REQUEST\t\tGET /fail\t1788379356162\t1788379356200\tKO\t"+message+"\n")

	f, ok := failed.Failure.Get()
	if !ok {
		t.Fatal("a failed sample carries no Failure")
	}

	if f.Message != message {
		t.Errorf("Failure.Message = %q, want %q", f.Message, message)
	}

	if f.Type != "" {
		t.Errorf("Failure.Type = %q, want empty — the source classifies nothing", f.Type)
	}

	succeeded := oneSample(t, "REQUEST\t\tGET /ok\t1788379356162\t1788379356173\tOK\t \n")
	if succeeded.Failure.IsSet() {
		t.Error("a successful sample carries a Failure")
	}
}

func TestGroupBecomesAGroupSample(t *testing.T) {
	t.Parallel()

	got := modelItems(t, modelPreamble+"GROUP\touter,inner\t1788379356162\t1788379356200\t9\tKO\n")
	if len(got) != 1 {
		t.Fatalf("got %d modelItems, want 1", len(got))
	}

	if got[0].Kind != model.ItemGroup {
		t.Fatalf("Kind = %v, want %v", got[0].Kind, model.ItemGroup)
	}

	g := got[0].Group

	if want := []string{"outer", "inner"}; !slices.Equal(g.Groups, want) {
		t.Errorf("Groups = %q, want %q", g.Groups, want)
	}

	if g.Outcome != model.OutcomeFailure {
		t.Errorf("Outcome = %v, want %v", g.Outcome, model.OutcomeFailure)
	}

	if d, ok := g.CumulatedDuration.Get(); !ok || d != 9*time.Millisecond {
		t.Errorf("CumulatedDuration = %v (set %t), want 9ms", d, ok)
	}

	// Wall clock across the traversal is a different quantity from the
	// cumulated response time, and the record carries both.
	if d, ok := g.Duration.Get(); !ok || d != 38*time.Millisecond {
		t.Errorf("Duration = %v (set %t), want 38ms of wall clock", d, ok)
	}
}

// A group can fail while every request inside it succeeded, so its outcome is
// its own and never the conjunction of what it enclosed.
func TestGroupOutcomeIsItsOwn(t *testing.T) {
	t.Parallel()

	log := modelPreamble +
		"REQUEST\touter\tGET /ok\t1788379356162\t1788379356173\tOK\t \n" +
		"REQUEST\touter\tGET /ok2\t1788379356174\t1788379356180\tOK\t \n" +
		"GROUP\touter\t1788379356162\t1788379356200\t17\tKO\n"

	got := modelItems(t, log)
	if len(got) != 3 {
		t.Fatalf("got %d modelItems, want 3", len(got))
	}

	for _, it := range got[:2] {
		if it.Sample.Outcome != model.OutcomeSuccess {
			t.Errorf("enclosed sample %q = %v, want success", it.Sample.Name, it.Sample.Outcome)
		}
	}

	if got[2].Group.Outcome != model.OutcomeFailure {
		t.Errorf("group outcome = %v, want failure despite every enclosed sample succeeding", got[2].Group.Outcome)
	}
}

func TestUserBecomesAUserEvent(t *testing.T) {
	t.Parallel()

	log := modelPreamble +
		"USER\tCorpus recording\tSTART\t1788379356165\n" +
		"USER\tCorpus recording\tEND\t1788379356180\n"

	got := modelItems(t, log)
	if len(got) != 2 {
		t.Fatalf("got %d modelItems, want 2", len(got))
	}

	for i, want := range []model.UserEventKind{model.UserStart, model.UserEnd} {
		if got[i].Kind != model.ItemUser {
			t.Fatalf("item %d Kind = %v, want %v", i, got[i].Kind, model.ItemUser)
		}

		if got[i].User.Kind != want {
			t.Errorf("item %d event = %v, want %v", i, got[i].User.Kind, want)
		}

		if got[i].User.Scenario != "Corpus recording" {
			t.Errorf("item %d Scenario = %q", i, got[i].User.Scenario)
		}
	}

	if got, want := got[0].User.At.UTC(), time.UnixMilli(1788379356165).UTC(); !got.Equal(want) {
		t.Errorf("At = %v, want %v", got, want)
	}
}

// Gatling writes an ERROR for a request whose URL could not be built: it never
// reached the wire and produced no request record, so it belongs to no sample.
func TestErrorBecomesARunError(t *testing.T) {
	t.Parallel()

	got := modelItems(t, modelPreamble+"ERROR\tunresolvable url: no attribute\t1788379356190\n")
	if len(got) != 1 {
		t.Fatalf("got %d modelItems, want 1", len(got))
	}

	if got[0].Kind != model.ItemError {
		t.Fatalf("Kind = %v, want %v", got[0].Kind, model.ItemError)
	}

	if want := "unresolvable url: no attribute"; got[0].Error.Message != want {
		t.Errorf("Message = %q, want %q", got[0].Error.Message, want)
	}

	if got, want := got[0].Error.At.UTC(), time.UnixMilli(1788379356190).UTC(); !got.Equal(want) {
		t.Errorf("At = %v, want %v", got, want)
	}

	// It is not attached to any sample.
	if got[0].Sample.Name != "" {
		t.Error("a run error carries a sample")
	}
}

// One item per event record, in file order — the property every count rests on.
func TestItemsArriveOncePerRecordInOrder(t *testing.T) {
	t.Parallel()

	log := modelPreamble +
		"USER\ts\tSTART\t1788379356160\n" +
		"REQUEST\t\tGET /ok\t1788379356162\t1788379356173\tOK\t \n" +
		"GROUP\touter\t1788379356162\t1788379356200\t9\tOK\n" +
		"ERROR\tboom\t1788379356201\n" +
		"USER\ts\tEND\t1788379356210\n"

	want := []model.ItemKind{model.ItemUser, model.ItemSample, model.ItemGroup, model.ItemError, model.ItemUser}

	got := modelItems(t, log)
	if len(got) != len(want) {
		t.Fatalf("got %d modelItems, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i].Kind != want[i] {
			t.Errorf("item %d Kind = %v, want %v", i, got[i].Kind, want[i])
		}
	}
}

// The run's capabilities are the source's, available before the first item.
func TestRunCarriesTheSourceCapabilities(t *testing.T) {
	t.Parallel()

	rd, err := text.NewRunReader(strings.NewReader(modelPreamble))
	if err != nil {
		t.Fatalf("NewRunReader: %v", err)
	}

	if got, want := rd.Run().Capabilities, text.Capabilities(); got != want {
		t.Error("Run.Capabilities is not the source's own")
	}
}

// A version above the covered range decodes, and the warning travels on the run
// so an unverified result cannot pass for a verified one.
func TestVersionWarningTravelsOntoTheRun(t *testing.T) {
	t.Parallel()

	log := "RUN\tio.galaxio.parsec.corpus.CorpusSimulation\tr\t1788379354534\t \t3.99.0\n"

	rd, err := text.NewRunReader(strings.NewReader(log))
	if err != nil {
		t.Fatalf("NewRunReader: %v", err)
	}

	run := rd.Run()
	if len(run.Warnings) != 1 {
		t.Fatalf("Warnings = %v, want exactly one", run.Warnings)
	}

	if !strings.Contains(run.Warnings[0].Version, "3.99.0") {
		t.Errorf("the warning does not name the version found: %+v", run.Warnings[0])
	}

	if run.Warnings[0].Reason == "" {
		t.Error("the warning gives no reason")
	}
}

// The gate belongs to the decoder and is not re-implemented: a refused log never
// reaches the conversion, and the error is the decoder's own.
func TestRefusedLogNeverReachesTheConversion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		log  string
	}{
		{name: "below the range", log: "RUN\tc\tr\t1788379354534\t \t3.10.0\n"},
		{name: "not a release", log: "RUN\tc\tr\t1788379354534\t \t3.12.0-M1\n"},
		{name: "no header", log: "USER\ts\tSTART\t1788379356160\n"},
		{name: "empty", log: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := text.NewRunReader(strings.NewReader(tt.log)); err == nil {
				t.Fatal("NewRunReader accepted a log the decoder refuses")
			}
		})
	}
}

// A read that stopped on an unreadable line is not a shorter successful one.
func TestUnreadableLineEndsTheStream(t *testing.T) {
	t.Parallel()

	log := modelPreamble +
		"REQUEST\t\tGET /ok\t1788379356162\t1788379356173\tOK\t \n" +
		"NONSENSE\twhat\n"

	rd, err := text.NewRunReader(strings.NewReader(log))
	if err != nil {
		t.Fatalf("NewRunReader: %v", err)
	}

	if _, err := rd.Next(); err != nil {
		t.Fatalf("the first item failed: %v", err)
	}

	_, err = rd.Next()
	if err == nil || errors.Is(err, io.EOF) {
		t.Fatalf("the unreadable line ended the stream as %v, want a syntax error", err)
	}

	var se *gatling.SyntaxError
	if !errors.As(err, &se) {
		t.Errorf("error is %T, want the decoder's own *gatling.SyntaxError", err)
	}

	// Every later call returns the same error: there is no next item after it.
	if _, again := rd.Next(); !errors.Is(again, err) {
		t.Errorf("a later Next returned %v, want the same error", again)
	}
}
