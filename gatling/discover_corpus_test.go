//go:build integration

package gatling_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/galax-io/parsec/gatling"
)

// recordedRoot is the results root recorded from three real Maven runs. See
// testdata/corpus/gatling/lastrun/RECORDING.md for how it was made and why it
// took three of them.
const recordedRoot = "../testdata/corpus/gatling/lastrun/results"

// namedByRecording reads the recorded lastRun.txt the way a person would, so the
// assertions below compare against the file rather than against a constant that
// could drift away from it.
func namedByRecording(t *testing.T) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(recordedRoot, "lastRun.txt"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			t.Skip("no lastRun.txt recording: see testdata/corpus/gatling/lastrun/RECORDING.md")
		}

		t.Fatalf("read recorded lastRun.txt: %v", err)
	}

	return strings.TrimSpace(string(raw))
}

// The only place this package's reader meets a lastRun.txt a build tool actually
// wrote. Everything else about the file is held by unit tests written against
// gatling-maven-plugin's bytecode; this is the check that the bytecode was read
// correctly.
func TestFindRunOverRecordedResultsRoot(t *testing.T) {
	t.Parallel()

	named := namedByRecording(t)

	loc, err := gatling.FindRun(recordedRoot)
	if err != nil {
		t.Fatalf("FindRun(%s): %v", recordedRoot, err)
	}

	if want := filepath.Join(recordedRoot, named); loc.Dir != want {
		t.Errorf("Dir = %s, want %s", loc.Dir, want)
	}

	if loc.Found != gatling.FoundByLastRun {
		t.Errorf("Found = %v, want %v", loc.Found, gatling.FoundByLastRun)
	}

	if _, err := os.Stat(loc.Log); err != nil {
		t.Errorf("the located log is not there: %v", err)
	}
}

// The recording is #11's acceptance case, produced rather than constructed: the
// run lastRun.txt names is the middle of three, so a reader that ignored the
// file would return a different directory and this would catch it.
func TestRecordedRootNamesTheMiddleRun(t *testing.T) {
	t.Parallel()

	named := namedByRecording(t)

	entries, err := os.ReadDir(recordedRoot)
	if err != nil {
		t.Fatalf("read %s: %v", recordedRoot, err)
	}

	runs := make([]string, 0, len(entries))

	for _, entry := range entries {
		if entry.IsDir() {
			runs = append(runs, entry.Name())
		}
	}

	if len(runs) != 3 {
		t.Fatalf("recording holds %d run directories, want 3: %v", len(runs), runs)
	}

	// os.ReadDir sorts by name, and a Gatling run id ends in its UTC start
	// formatted yyyyMMddHHmmssSSS, so name order is run order.
	if runs[1] != named {
		t.Errorf("lastRun.txt names %s, which is not the middle run %s; the recording no longer "+
			"exercises the case it was made for", named, runs[1])
	}
}

// What the recorded file actually contains, asserted against the shape the
// reader assumes: one bare name, no separator, nothing absolute. If
// gatling-maven-plugin ever changes this, the recording is where it shows up.
func TestRecordedLastRunShape(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join(recordedRoot, "lastRun.txt"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			t.Skip("no lastRun.txt recording: see testdata/corpus/gatling/lastrun/RECORDING.md")
		}

		t.Fatalf("read: %v", err)
	}

	text := string(raw)

	if !strings.HasSuffix(text, "\n") {
		t.Errorf("recorded file does not end with a line separator: %q", text)
	}

	lines := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' })
	if len(lines) != 1 {
		t.Fatalf("recorded file holds %d lines, want 1: %q", len(lines), text)
	}

	name := lines[0]
	if name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		t.Errorf("recorded line %q is not a bare directory name", name)
	}

	//nolint:gosec // name comes from the committed recording, and the line above just proved it is a bare name
	if _, err := os.Stat(filepath.Join(recordedRoot, name, "simulation.log")); err != nil {
		t.Errorf("the recorded line does not name a run: %v", err)
	}
}

// A clone flattens modification times, so the recording arrives with every run
// stamped at checkout. That is precisely the case the tie-break exists for, and
// here it is real rather than simulated: with the pointer removed, the newest
// rule still has to choose, and it must choose the same run every time.
func TestRecordedRootWithoutPointerIsDeterministic(t *testing.T) {
	t.Parallel()

	// Copy the recording so the committed one is never touched.
	root := t.TempDir()

	entries, err := os.ReadDir(recordedRoot)
	if err != nil {
		t.Fatalf("read %s: %v", recordedRoot, err)
	}

	same := time.Date(2026, time.September, 9, 3, 0, 0, 0, time.UTC)
	copied := make([]string, 0, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		copied = append(copied, entry.Name())

		dir := filepath.Join(root, entry.Name())
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		src, err := os.ReadFile(filepath.Join(recordedRoot, entry.Name(), "simulation.log"))
		if err != nil {
			t.Fatalf("read log: %v", err)
		}

		//nolint:gosec // dir is this test's own t.TempDir() joined with a name from the committed recording
		if err := os.WriteFile(filepath.Join(dir, "simulation.log"), src, 0o600); err != nil {
			t.Fatalf("write log: %v", err)
		}

		if err := os.Chtimes(dir, same, same); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
	}

	var first gatling.RunLocation

	for i := range 10 {
		loc, err := gatling.FindRun(root)
		if err != nil {
			t.Fatalf("FindRun attempt %d: %v", i, err)
		}

		if loc.Found != gatling.FoundByNewest {
			t.Fatalf("Found = %v, want %v", loc.Found, gatling.FoundByNewest)
		}

		if i == 0 {
			first = loc

			continue
		}

		if loc.Dir != first.Dir {
			t.Fatalf("attempt %d chose %s, attempt 0 chose %s — the ordering is not total",
				i, loc.Dir, first.Dir)
		}
	}

	// The latest run start wins, which for these names is the last in sort order:
	// os.ReadDir sorts by name and a run id ends in its UTC start. Only the run
	// directories count — lastRun.txt is in the listing too, and sorts after them.
	if len(copied) != 3 {
		t.Fatalf("copied %d run directories, want 3: %v", len(copied), copied)
	}

	if want := filepath.Join(root, copied[len(copied)-1]); first.Dir != want {
		t.Errorf("chose %s, want the latest run %s", first.Dir, want)
	}
}
