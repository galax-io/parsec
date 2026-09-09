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

// Three real run ids, from the console output the 3.13.1, 3.14.9 and 3.15.1
// recordings were captured with. Gatling names a run directory
// <simulationId>-<yyyyMMddHHmmssSSS>, so these sort in run order, and using the
// real shape keeps the fixtures honest about what the ordering rule faces.
var runNames = [...]string{
	"corpussimulation-20260906044741110",
	"corpussimulation-20260906044803343",
	"corpussimulation-20260906044814356",
}

// mkRun creates a run directory: one holding a simulation.log. The log is empty
// because nothing here opens it, and a test that put bytes in it would be
// asserting something this feature promises never to look at.
func mkRun(t *testing.T, root, name string) string {
	t.Helper()

	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}

	if err := os.WriteFile(filepath.Join(dir, "simulation.log"), nil, 0o600); err != nil {
		t.Fatalf("write log in %s: %v", dir, err)
	}

	return dir
}

// touch sets a directory's modification time, so a fixture can say which run is
// newest instead of depending on how fast the test created them.
func touch(t *testing.T, dir string, mod time.Time) {
	t.Helper()

	if err := os.Chtimes(dir, mod, mod); err != nil {
		t.Fatalf("chtimes %s: %v", dir, err)
	}
}

// rootWithThreeRuns builds a results root holding runNames, oldest first, with
// modification times a minute apart so that the middle one is neither newest nor
// oldest — the shape issue #11's acceptance cases are written against.
func rootWithThreeRuns(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	base := time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC)

	for i, name := range runNames {
		touch(t, mkRun(t, root, name), base.Add(time.Duration(i)*time.Minute))
	}

	return root
}

func TestFoundByString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  gatling.FoundBy
		want string
	}{
		{name: "zero value", got: gatling.FoundByUnknown, want: "unknown"},
		{name: "named by the caller", got: gatling.FoundByPath, want: "path"},
		{name: "named by lastRun.txt", got: gatling.FoundByLastRun, want: "lastRun.txt"},
		{name: "newest in the results root", got: gatling.FoundByNewest, want: "newest"},
		{name: "out of range", got: gatling.FoundBy(9), want: "FoundBy(9)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.got.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// The zero value must be the sentinel and never a real rule: a RunLocation that
// was never filled in has to be distinguishable from one found by a path, or a
// caller reporting how it chose a run reports the first constant by accident.
func TestFoundByZeroValueIsUnknown(t *testing.T) {
	t.Parallel()

	var zero gatling.FoundBy
	if zero != gatling.FoundByUnknown {
		t.Fatalf("zero FoundBy = %v, want FoundByUnknown", zero)
	}

	var loc gatling.RunLocation
	if loc.Found != gatling.FoundByUnknown || loc.Dir != "" || loc.Log != "" {
		t.Fatalf("zero RunLocation = %+v, want every field zero", loc)
	}
}

// T009 — the ordinary case, and the one every Gradle and sbt user gets: there is
// no lastRun.txt to prefer, so the newest run in the root is the answer.
func TestFindRunNewest(t *testing.T) {
	t.Parallel()

	root := rootWithThreeRuns(t)

	loc, err := gatling.FindRun(root)
	if err != nil {
		t.Fatalf("FindRun(%s): %v", root, err)
	}

	if want := filepath.Join(root, runNames[2]); loc.Dir != want {
		t.Errorf("Dir = %s, want %s", loc.Dir, want)
	}

	if want := filepath.Join(loc.Dir, "simulation.log"); loc.Log != want {
		t.Errorf("Log = %s, want %s", loc.Log, want)
	}

	if loc.Found != gatling.FoundByNewest {
		t.Errorf("Found = %v, want %v", loc.Found, gatling.FoundByNewest)
	}
}

// T010 — the test that earns the ordering rule. A git clone, an rsync without
// -t, a CI cache restore or a container image build gives every run in a root
// one modification time, and those are the same users who never have a
// lastRun.txt. An ordering that is merely "newest" picks arbitrarily here; a
// total one picks the highest name, which for a Gatling run id is the latest
// run start.
func TestFindRunDeterministicUnderSharedModTime(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	same := time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC)

	for _, name := range runNames {
		touch(t, mkRun(t, root, name), same)
	}

	want := filepath.Join(root, runNames[2])

	for i := range 20 {
		loc, err := gatling.FindRun(root)
		if err != nil {
			t.Fatalf("FindRun attempt %d: %v", i, err)
		}

		if loc.Dir != want {
			t.Fatalf("attempt %d: Dir = %s, want %s — the ordering is not total", i, loc.Dir, want)
		}
	}
}

// T011 — no path at all means the results root Maven and sbt share. The caller
// can tell it got a default because it passed nothing and the returned Dir sits
// under target/gatling; nothing else needs to say so.
//
//nolint:paralleltest // t.Chdir cannot be used from a parallel test.
func TestFindRunDefaultResultsRoot(t *testing.T) {
	project := t.TempDir()
	dir := mkRun(t, filepath.Join(project, "target", "gatling"), runNames[0])
	touch(t, dir, time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC))

	t.Chdir(project)

	loc, err := gatling.FindRun("")
	if err != nil {
		t.Fatalf(`FindRun(""): %v`, err)
	}

	if want := filepath.Join("target", "gatling", runNames[0]); loc.Dir != want {
		t.Errorf("Dir = %s, want %s", loc.Dir, want)
	}

	if loc.Found != gatling.FoundByNewest {
		t.Errorf("Found = %v, want %v", loc.Found, gatling.FoundByNewest)
	}
}

// T012 — Gradle's layout is not a second thing to search for. It is passed, and
// then it is just a results root like any other.
func TestFindRunGradleLayout(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	root := filepath.Join(project, "build", "reports", "gatling")
	touch(t, mkRun(t, root, runNames[1]), time.Date(2026, time.September, 6, 4, 48, 0, 0, time.UTC))

	// A decoy under the Maven layout in the same project: if the default were
	// consulted at all, this is what would come back.
	touch(t, mkRun(t, filepath.Join(project, "target", "gatling"), runNames[2]),
		time.Date(2026, time.September, 6, 4, 49, 0, 0, time.UTC))

	loc, err := gatling.FindRun(root)
	if err != nil {
		t.Fatalf("FindRun(%s): %v", root, err)
	}

	if want := filepath.Join(root, runNames[1]); loc.Dir != want {
		t.Errorf("Dir = %s, want %s — the default root was consulted", loc.Dir, want)
	}
}

// T013 — only a simulation.log makes a directory a run. Gatling stopped
// producing reports in 3.13.5, so a run without one is ordinary; a report
// without a log is not a run at all.
func TestFindRunCandidates(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// A report-only directory, the reverse of a report-less run.
	reportOnly := filepath.Join(root, "corpussimulation-20260906044900000")
	if err := os.MkdirAll(reportOnly, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(reportOnly, "index.html"), nil, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	// A plain file sitting in the root, which is not a candidate at all.
	if err := os.WriteFile(filepath.Join(root, "simulation.log.bak"), nil, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	run := mkRun(t, root, runNames[0])
	touch(t, run, time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC))

	loc, err := gatling.FindRun(root)
	if err != nil {
		t.Fatalf("FindRun(%s): %v", root, err)
	}

	if loc.Dir != run {
		t.Errorf("Dir = %s, want %s — a directory without a simulation.log is not a run", loc.Dir, run)
	}
}

// writeLastRun puts a lastRun.txt in a results root, exactly as
// gatling-maven-plugin would: whatever bytes the plugin wrote, verbatim.
func writeLastRun(t *testing.T, root, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(root, "lastRun.txt"), []byte(content), 0o600); err != nil {
		t.Fatalf("write lastRun.txt: %v", err)
	}
}

// T017 — issue #11's acceptance case. The middle run is neither newest nor
// oldest, so a pass proves the pointer beat the clock rather than agreeing with
// it by luck.
func TestFindRunLastRunBeatsModTime(t *testing.T) {
	t.Parallel()

	root := rootWithThreeRuns(t)
	writeLastRun(t, root, runNames[1]+"\n")

	withPointer, err := gatling.FindRun(root)
	if err != nil {
		t.Fatalf("FindRun with lastRun.txt: %v", err)
	}

	if want := filepath.Join(root, runNames[1]); withPointer.Dir != want {
		t.Errorf("Dir = %s, want %s", withPointer.Dir, want)
	}

	if withPointer.Found != gatling.FoundByLastRun {
		t.Errorf("Found = %v, want %v", withPointer.Found, gatling.FoundByLastRun)
	}

	if err := os.Remove(filepath.Join(root, "lastRun.txt")); err != nil {
		t.Fatalf("remove lastRun.txt: %v", err)
	}

	withoutPointer, err := gatling.FindRun(root)
	if err != nil {
		t.Fatalf("FindRun without lastRun.txt: %v", err)
	}

	if want := filepath.Join(root, runNames[2]); withoutPointer.Dir != want {
		t.Errorf("Dir = %s, want %s", withoutPointer.Dir, want)
	}

	if withPointer.Dir == withoutPointer.Dir {
		t.Fatal("both reads chose the same run; the fixture proves nothing")
	}
}

// T018 — a pointer to nothing costs a stat and nothing else. None of these is a
// failure: the file is a hint, and a wrong hint falls back to the clock.
//
// The two error shapes are what gatling-maven-plugin appends when the run
// failed. Neither is matched as text — they name no directory, so they fail the
// same existence check every other non-name fails, and the plugin can reword
// them freely.
func TestFindRunLastRunPointingAtNothing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{name: "a run that was deleted", content: "corpussimulation-20250101000000000\n"},
		{name: "a directory with no simulation.log", content: "reports-only\n"},
		{name: "an execution error", content: "ExecutionError: java.lang.RuntimeException: boom | cause\n"},
		{name: "an assertion failure", content: "Gatling simulation assertions failed!\n"},
		{name: "a blank line", content: "\n"},
		{name: "an empty file", content: ""},
		{name: "whitespace only", content: "   \n\t\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := rootWithThreeRuns(t)

			reportOnly := filepath.Join(root, "reports-only")
			if err := os.MkdirAll(reportOnly, 0o750); err != nil {
				t.Fatalf("mkdir: %v", err)
			}

			writeLastRun(t, root, tt.content)

			loc, err := gatling.FindRun(root)
			if err != nil {
				t.Fatalf("FindRun: %v", err)
			}

			if want := filepath.Join(root, runNames[2]); loc.Dir != want {
				t.Errorf("Dir = %s, want %s", loc.Dir, want)
			}

			if loc.Found != gatling.FoundByNewest {
				t.Errorf("Found = %v, want %v", loc.Found, gatling.FoundByNewest)
			}
		})
	}
}

// T019 — nothing outside the results root is followed. Each case is asserted
// against a tree where the escape target really is a run, so a passing test
// means the rule bit rather than that the path happened to miss.
func TestFindRunLastRunContainment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line func(escape string) string
	}{
		{name: "parent traversal", line: func(string) string { return "../escape" }},
		{name: "deep traversal", line: func(string) string { return "../../escape" }},
		{name: "absolute path", line: func(escape string) string { return escape }},
		{name: "nested path", line: func(string) string { return "nested/run" }},
		{name: "the root itself", line: func(string) string { return "." }},
		{name: "the parent itself", line: func(string) string { return ".." }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := t.TempDir()
			root := filepath.Join(parent, "results")

			base := time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC)
			for i, name := range runNames {
				touch(t, mkRun(t, root, name), base.Add(time.Duration(i)*time.Minute))
			}

			// A real run outside the root, and a real run nested one level too
			// deep inside it. Both are valid runs; neither is a direct child.
			escape := mkRun(t, parent, "escape")
			touch(t, escape, base.Add(time.Hour))
			touch(t, mkRun(t, filepath.Join(root, "nested"), "run"), base.Add(time.Hour))

			writeLastRun(t, root, tt.line(escape)+"\n")

			loc, err := gatling.FindRun(root)
			if err != nil {
				t.Fatalf("FindRun: %v", err)
			}

			if want := filepath.Join(root, runNames[2]); loc.Dir != want {
				t.Fatalf("Dir = %s, want %s — a lastRun.txt line escaped the results root", loc.Dir, want)
			}
		})
	}
}

// T020 — the file's shape, as gatling-maven-plugin writes it. Lines are
// File.getName() joined by System.lineSeparator(), so CRLF is what a Windows
// build produces; the cap is this package's own, not the plugin's.
func TestFindRunLastRunShape(t *testing.T) {
	t.Parallel()

	// runNames[0] is the oldest, so naming it proves the file was read: the
	// clock alone would return runNames[2].
	oldest, newestRun := runNames[0], runNames[2]

	tests := []struct {
		name    string
		content string
		want    string
		found   gatling.FoundBy
	}{
		{name: "bare name, no trailing newline", content: oldest, want: oldest, found: gatling.FoundByLastRun},
		{name: "trailing LF", content: oldest + "\n", want: oldest, found: gatling.FoundByLastRun},
		{name: "CRLF, as a Windows build writes", content: oldest + "\r\n", want: oldest, found: gatling.FoundByLastRun},
		{name: "surrounding whitespace", content: "  " + oldest + "\t \n", want: oldest, found: gatling.FoundByLastRun},
		{name: "blank lines around the name", content: "\n\n" + oldest + "\n\n", want: oldest, found: gatling.FoundByLastRun},
		{
			name:    "a name and a trailing error line",
			content: oldest + "\nExecutionError: boom\n",
			want:    oldest, found: gatling.FoundByLastRun,
		},
		{
			name:    "invalid UTF-8 names nothing",
			content: "\xff\xfe\x00bad\n",
			want:    newestRun, found: gatling.FoundByNewest,
		},
		{
			name:    "past the cap is treated as absent",
			content: oldest + "\n" + strings.Repeat("x", 64<<10),
			want:    newestRun, found: gatling.FoundByNewest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := rootWithThreeRuns(t)
			writeLastRun(t, root, tt.content)

			loc, err := gatling.FindRun(root)
			if err != nil {
				t.Fatalf("FindRun: %v", err)
			}

			if want := filepath.Join(root, tt.want); loc.Dir != want {
				t.Errorf("Dir = %s, want %s", loc.Dir, want)
			}

			if loc.Found != tt.found {
				t.Errorf("Found = %v, want %v", loc.Found, tt.found)
			}
		})
	}
}

// T021 — Maven's runMultipleSimulations writes one line per run the build
// created, so several lines can name real runs at once. The newest of *those*
// wins, which is what "the last run" means; it is not the newest in the root,
// and it is not simply the last line, because the last line is the error message
// when the run failed.
func TestFindRunLastRunMultipleNames(t *testing.T) {
	t.Parallel()

	root := rootWithThreeRuns(t)

	// A fourth run, newer than everything, that this build did not produce: it
	// must not win, or the lines were ignored in favour of the clock.
	stranger := "othersimulation-20260906050000000"
	touch(t, mkRun(t, root, stranger), time.Date(2026, time.September, 6, 5, 0, 0, 0, time.UTC))

	writeLastRun(t, root, runNames[0]+"\n"+runNames[1]+"\nExecutionError: boom\n")

	loc, err := gatling.FindRun(root)
	if err != nil {
		t.Fatalf("FindRun: %v", err)
	}

	if want := filepath.Join(root, runNames[1]); loc.Dir != want {
		t.Errorf("Dir = %s, want %s", loc.Dir, want)
	}

	if loc.Found != gatling.FoundByLastRun {
		t.Errorf("Found = %v, want %v", loc.Found, gatling.FoundByLastRun)
	}
}

// T026 — what every caller does today keeps working, and the log itself is
// accepted as well as its directory: a script that has the one path and a CI job
// that has the other reach the same run through the same call.
func TestFindRunNamedPath(t *testing.T) {
	t.Parallel()

	root := rootWithThreeRuns(t)
	run := filepath.Join(root, runNames[0])

	byDir, err := gatling.FindRun(run)
	if err != nil {
		t.Fatalf("FindRun(dir): %v", err)
	}

	byLog, err := gatling.FindRun(filepath.Join(run, "simulation.log"))
	if err != nil {
		t.Fatalf("FindRun(log): %v", err)
	}

	if byDir != byLog {
		t.Errorf("FindRun(dir) = %+v, FindRun(log) = %+v; want identical", byDir, byLog)
	}

	if byDir.Dir != run || byDir.Found != gatling.FoundByPath {
		t.Errorf("got %+v, want Dir %s found by path", byDir, run)
	}
}

// T027 — a caller that named a place is never quietly answered about a different
// one. runNames[0] is the oldest, so a scan would have returned its sibling.
func TestFindRunNamedPathIsNeverSearchedPast(t *testing.T) {
	t.Parallel()

	root := rootWithThreeRuns(t)
	run := filepath.Join(root, runNames[0])

	// A pointer naming a different run, to prove neither rule outranks a path.
	writeLastRun(t, root, runNames[2]+"\n")

	loc, err := gatling.FindRun(run)
	if err != nil {
		t.Fatalf("FindRun: %v", err)
	}

	if loc.Dir != run {
		t.Errorf("Dir = %s, want %s — a named run was displaced", loc.Dir, run)
	}
}

// T028 — a directory holding a log is a run, not a results root, even when runs
// sit beneath it. Anything else would make an archive of archives ambiguous.
func TestFindRunDirectoryHoldingLogIsTheRun(t *testing.T) {
	t.Parallel()

	outer := t.TempDir()
	if err := os.WriteFile(filepath.Join(outer, "simulation.log"), nil, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	touch(t, mkRun(t, outer, runNames[2]), time.Date(2026, time.September, 6, 5, 0, 0, 0, time.UTC))

	loc, err := gatling.FindRun(outer)
	if err != nil {
		t.Fatalf("FindRun: %v", err)
	}

	if loc.Dir != outer || loc.Found != gatling.FoundByPath {
		t.Errorf("got %+v, want the directory itself (%s) found by path", loc, outer)
	}
}

// T029 — an archive that stores runs elsewhere and links them in still resolves.
// This is the case os.Root would have refused, and the reason containment here
// is a rule about the text of a lastRun.txt line rather than about the
// filesystem.
func TestFindRunSymlinkedRun(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	root := filepath.Join(parent, "results")

	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	elsewhere := mkRun(t, filepath.Join(parent, "archive"), runNames[1])
	link := filepath.Join(root, runNames[1])

	if err := os.Symlink(elsewhere, link); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}

	// A dangling link beside it, which is not a run at all.
	if err := os.Symlink(filepath.Join(parent, "gone"), filepath.Join(root, "corpussimulation-20260906050000000")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	loc, err := gatling.FindRun(root)
	if err != nil {
		t.Fatalf("FindRun: %v", err)
	}

	if loc.Dir != link {
		t.Errorf("Dir = %s, want %s — a symlinked run was not followed", loc.Dir, link)
	}
}

// T022 — US1 scenario 5, completed. A consumer that reports which run it read
// can say how it was chosen, and the three rules are distinguishable.
func TestFindRunReportsWhichRuleChose(t *testing.T) {
	t.Parallel()

	root := rootWithThreeRuns(t)

	byNewest, err := gatling.FindRun(root)
	if err != nil {
		t.Fatalf("FindRun(root): %v", err)
	}

	writeLastRun(t, root, runNames[1]+"\n")

	byPointer, err := gatling.FindRun(root)
	if err != nil {
		t.Fatalf("FindRun(root) with pointer: %v", err)
	}

	byPath, err := gatling.FindRun(filepath.Join(root, runNames[0]))
	if err != nil {
		t.Fatalf("FindRun(run): %v", err)
	}

	got := [...]gatling.FoundBy{byNewest.Found, byPointer.Found, byPath.Found}
	want := [...]gatling.FoundBy{gatling.FoundByNewest, gatling.FoundByLastRun, gatling.FoundByPath}

	if got != want {
		t.Errorf("Found = %v, want %v", got, want)
	}
}

// T031 — the whole cost of pointing a consumer at the wrong place is how long it
// takes to learn where the right one is, so every failure names the directory it
// read and says where that directory came from.
//
//nolint:paralleltest // t.Chdir cannot be used from a parallel test.
func TestFindRunNotFound(t *testing.T) {
	empty := t.TempDir()
	missing := filepath.Join(t.TempDir(), "no", "such", "place")
	project := t.TempDir()

	tests := []struct {
		name        string
		path        string
		wantDir     string
		wantDefault bool
	}{
		{name: "a results root holding no run", path: empty, wantDir: empty},
		{name: "a path that is not there", path: missing, wantDir: missing},
		{name: "the default root, absent", path: "", wantDir: "target/gatling", wantDefault: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.path == "" {
				t.Chdir(project)
			}

			loc, err := gatling.FindRun(tt.path)

			var notFound *gatling.RunNotFoundError
			if !errors.As(err, &notFound) {
				t.Fatalf("FindRun(%q) error = %v, want a *RunNotFoundError", tt.path, err)
			}

			if notFound.Dir != tt.wantDir {
				t.Errorf("Dir = %s, want %s", notFound.Dir, tt.wantDir)
			}

			if notFound.Default != tt.wantDefault {
				t.Errorf("Default = %v, want %v", notFound.Default, tt.wantDefault)
			}

			if loc != (gatling.RunLocation{}) {
				t.Errorf("location = %+v, want the zero value beside an error", loc)
			}

			mustContain(t, err.Error(), tt.wantDir)
		})
	}
}

// T032 — the test that rots. A version treating every read error as "no runs
// here" passes every other case in this file and turns a broken mount, a bad
// permission or a dead network share into a clean, wrong answer.
func TestFindRunUnreadableDirectory(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("running as root: the permission cannot be enforced")
	}

	root := t.TempDir()
	touch(t, mkRun(t, root, runNames[0]), time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC))

	if err := os.Chmod(root, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	t.Cleanup(func() {
		// A directory, not a file: without its execute bit the test framework
		// cannot traverse it to remove it, so G302's 0600 ceiling does not apply.
		if err := os.Chmod(root, 0o700); err != nil { //nolint:gosec // a directory this test must leave traversable for its own cleanup
			t.Errorf("restoring permissions: %v", err)
		}
	})

	_, err := gatling.FindRun(root)
	if err == nil {
		t.Fatal("FindRun on an unreadable directory returned no error")
	}

	var notFound *gatling.RunNotFoundError
	if errors.As(err, &notFound) {
		t.Fatalf("error = %v; a directory that could not be read was reported as an absence of runs", err)
	}

	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("error = %v, want it to wrap an *fs.PathError", err)
	}

	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("errors.Is(err, fs.ErrPermission) = false for %v", err)
	}
}

// T033 — a path the caller named is answered about, or it fails. It never
// silently becomes a question about target/gatling instead.
//
//nolint:paralleltest // t.Chdir cannot be used from a parallel test.
func TestFindRunNamedPathDoesNotFallBackToDefault(t *testing.T) {
	project := t.TempDir()
	touch(t, mkRun(t, filepath.Join(project, "target", "gatling"), runNames[2]),
		time.Date(2026, time.September, 6, 5, 0, 0, 0, time.UTC))

	elsewhere := filepath.Join(project, "somewhere-else")
	if err := os.MkdirAll(elsewhere, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	t.Chdir(project)

	_, err := gatling.FindRun(elsewhere)

	var notFound *gatling.RunNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("error = %v, want a *RunNotFoundError", err)
	}

	if notFound.Dir != elsewhere || notFound.Default {
		t.Errorf("got Dir %s default %v, want %s and false — the default root was consulted",
			notFound.Dir, notFound.Default, elsewhere)
	}
}

// T037 — FR-013, asserted positively rather than by absence. Discovery opens no
// simulation.log, so a root full of logs that are not Gatling logs, or that
// cannot be opened at all, still resolves; the version gate and the codec are
// the reader's job and fire later.
func TestFindRunNeverOpensTheLog(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	base := time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC)

	// A log that is not a Gatling log at all.
	garbage := mkRun(t, root, runNames[0])
	if err := os.WriteFile(filepath.Join(garbage, "simulation.log"), []byte("not a gatling log"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	touch(t, garbage, base)

	// A log that cannot be opened, in a directory that can still be listed.
	unreadable := mkRun(t, root, runNames[1])
	if err := os.Chmod(filepath.Join(unreadable, "simulation.log"), 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	t.Cleanup(func() {
		if err := os.Chmod(filepath.Join(unreadable, "simulation.log"), 0o600); err != nil {
			t.Errorf("restoring permissions: %v", err)
		}
	})

	touch(t, unreadable, base.Add(time.Minute))

	loc, err := gatling.FindRun(root)
	if err != nil {
		t.Fatalf("FindRun: %v", err)
	}

	if loc.Dir != unreadable {
		t.Errorf("Dir = %s, want %s — discovery is not affected by what a log contains", loc.Dir, unreadable)
	}
}
