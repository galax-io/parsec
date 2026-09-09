package gatling_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/galax-io/parsec/gatling"
)

// FuzzLastRun drives the only input this feature parses.
//
// Everything else FindRun reads is filesystem metadata; lastRun.txt is bytes
// written by another project, and it is the one place a malformed input could
// reach. Two properties are asserted: no input panics, and none of them selects
// a run outside the results root.
func FuzzLastRun(f *testing.F) {
	f.Add("")
	f.Add("corpussimulation-20260906044741110")
	f.Add("corpussimulation-20260906044741110\n")
	f.Add("corpussimulation-20260906044741110\r\n")
	f.Add("corpussimulation-20260906044741110\ncorpussimulation-20260906044803343\n")
	f.Add("ExecutionError: java.lang.RuntimeException: boom | cause\n")
	f.Add("Gatling simulation assertions failed!\n")
	f.Add("../escape\n")
	f.Add("/etc\n")
	f.Add("nested/run\n")
	f.Add("..\n")
	f.Add(".\n")
	f.Add("\x00\xff\xfe not utf-8 \n")
	f.Add(strings.Repeat("x", 64<<10))

	parent := f.TempDir()
	root := filepath.Join(parent, "results")
	base := time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC)

	for i, name := range runNames {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			f.Fatalf("mkdir: %v", err)
		}

		if err := os.WriteFile(filepath.Join(dir, "simulation.log"), nil, 0o600); err != nil {
			f.Fatalf("write: %v", err)
		}

		mod := base.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(filepath.Join(dir, "simulation.log"), mod, mod); err != nil {
			f.Fatalf("chtimes: %v", err)
		}
	}

	// A real run outside the root and one nested too deep inside it. Both are
	// valid runs, so a line that reached either would be selected if the
	// bare-name rule ever stopped holding.
	for _, outside := range []string{filepath.Join(parent, "escape"), filepath.Join(root, "nested", "run")} {
		if err := os.MkdirAll(outside, 0o750); err != nil {
			f.Fatalf("mkdir: %v", err)
		}

		if err := os.WriteFile(filepath.Join(outside, "simulation.log"), nil, 0o600); err != nil {
			f.Fatalf("write: %v", err)
		}
	}

	f.Fuzz(func(t *testing.T, content string) {
		if err := os.WriteFile(filepath.Join(root, "lastRun.txt"), []byte(content), 0o600); err != nil {
			t.Fatalf("write lastRun.txt: %v", err)
		}

		loc, err := gatling.FindRun(root)
		if err != nil {
			// The root always holds three runs, so the only legal ending is a
			// location. Anything else is the bug this target looks for.
			t.Fatalf("FindRun: %v", err)
		}

		// The chosen run must be a direct child of the root, whatever the file
		// said. filepath.Dir of the run directory is the root exactly then.
		if got := filepath.Dir(loc.Dir); got != root {
			t.Fatalf("selected %s, whose parent is %s, not the results root %s", loc.Dir, got, root)
		}

		if loc.Log != filepath.Join(loc.Dir, "simulation.log") {
			t.Fatalf("Log = %s, want it inside %s", loc.Log, loc.Dir)
		}

		if loc.Found != gatling.FoundByLastRun && loc.Found != gatling.FoundByNewest {
			t.Fatalf("Found = %v, want a results-root rule", loc.Found)
		}
	})
}
