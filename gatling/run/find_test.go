package run_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/galax-io/parsec/gatling/run"
)

// mustContain fails unless every want appears in msg. The gatling package's tests
// have their own copy; a test helper is not worth an export.
func mustContain(t *testing.T, msg string, wants ...string) {
	t.Helper()

	for _, want := range wants {
		if !strings.Contains(msg, want) {
			t.Fatalf("message %q does not contain %q", msg, want)
		}
	}
}

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

// touchLog sets a run's *log* modification time, which is what the ordering rule
// compares. A fixture that set the directory's time instead would pass whether
// the code read the log or the directory, and so would pin neither.
func touchLog(t *testing.T, dir string, mod time.Time) {
	t.Helper()

	if err := os.Chtimes(filepath.Join(dir, "simulation.log"), mod, mod); err != nil {
		t.Fatalf("chtimes %s: %v", dir, err)
	}
}

// touchDir sets a run directory's modification time. It exists to build the case
// the ordering must ignore: a report regenerated into an old run moves the
// directory and leaves the log alone.
func touchDir(t *testing.T, dir string, mod time.Time) {
	t.Helper()

	if err := os.Chtimes(dir, mod, mod); err != nil {
		t.Fatalf("chtimes %s: %v", dir, err)
	}
}

// requireUnixPermissions skips a test that needs a mode bit to actually deny
// access. os.Geteuid returns -1 on Windows rather than 0, so a uid check does not
// skip there, and os.Chmod only toggles the read-only attribute — a directory
// stays traversable and the test would fail for the platform rather than the code.
func requireUnixPermissions(t *testing.T) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits do not deny directory traversal on Windows")
	}

	if os.Geteuid() == 0 {
		t.Skip("running as root: the permission cannot be enforced")
	}
}

// rootWithThreeRuns builds a results root holding runNames, oldest first, with
// modification times a minute apart so that the middle one is neither newest nor
// oldest — the shape issue #11's acceptance cases are written against.
func rootWithThreeRuns(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	base := time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC)

	// Built newest-first, so that "the newest run" cannot be satisfied by
	// creation order, directory order or os.ReadDir's name sort by accident.
	for i := len(runNames) - 1; i >= 0; i-- {
		touchLog(t, mkRun(t, root, runNames[i]), base.Add(time.Duration(i)*time.Minute))
	}

	return root
}

func TestFoundByString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  run.FoundBy
		want string
	}{
		{name: "zero value", got: run.FoundByUnknown, want: "unknown"},
		{name: "named by the caller", got: run.FoundByPath, want: "path"},
		{name: "named by lastRun.txt", got: run.FoundByLastRun, want: "lastRun.txt"},
		{name: "newest in the results root", got: run.FoundByNewest, want: "newest"},
		{name: "out of range", got: run.FoundBy(9), want: "FoundBy(9)"},
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

// The zero value must be the sentinel and never a real rule: a Location that
// was never filled in has to be distinguishable from one found by a path, or a
// caller reporting how it chose a run reports the first constant by accident.
func TestFoundByZeroValueIsUnknown(t *testing.T) {
	t.Parallel()

	var zero run.FoundBy
	if zero != run.FoundByUnknown {
		t.Fatalf("zero FoundBy = %v, want FoundByUnknown", zero)
	}

	var loc run.Location
	if loc.Found != run.FoundByUnknown || loc.Dir != "" || loc.Log != "" {
		t.Fatalf("zero Location = %+v, want every field zero", loc)
	}
}

// T009 — the ordinary case, and the one every Gradle and sbt user gets: there is
// no lastRun.txt to prefer, so the newest run in the root is the answer.
func TestFindNewest(t *testing.T) {
	t.Parallel()

	root := rootWithThreeRuns(t)

	loc, err := run.Find(root)
	if err != nil {
		t.Fatalf("Find(%s): %v", root, err)
	}

	if want := filepath.Join(root, runNames[2]); loc.Dir != want {
		t.Errorf("Dir = %s, want %s", loc.Dir, want)
	}

	if want := filepath.Join(loc.Dir, "simulation.log"); loc.Log != want {
		t.Errorf("Log = %s, want %s", loc.Log, want)
	}

	if loc.Found != run.FoundByNewest {
		t.Errorf("Found = %v, want %v", loc.Found, run.FoundByNewest)
	}
}

// T010 — the test that earns the ordering rule. A git clone, an rsync without
// -t, a CI cache restore or a container image build gives every run in a root
// one modification time, and those are the same users who never have a
// lastRun.txt. An ordering that is merely "newest" picks arbitrarily here; a
// total one picks the highest name, which for a Gatling run id is the latest
// run start.
func TestFindDeterministicUnderSharedModTime(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	same := time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC)

	for _, name := range runNames {
		touchLog(t, mkRun(t, root, name), same)
	}

	want := filepath.Join(root, runNames[2])

	for i := range 20 {
		loc, err := run.Find(root)
		if err != nil {
			t.Fatalf("Find attempt %d: %v", i, err)
		}

		if loc.Dir != want {
			t.Fatalf("attempt %d: Dir = %s, want %s — the ordering is not total", i, loc.Dir, want)
		}
	}
}

// T011 — the results root Maven and sbt share is a constant a caller passes, not
// a guess this package makes. Asking for it by name is the whole feature for a
// CLI standing in a project.
//
//nolint:paralleltest // t.Chdir cannot be used from a parallel test.
func TestFindDefaultResultsRoot(t *testing.T) {
	project := t.TempDir()
	dir := mkRun(t, filepath.Join(project, "target", "gatling"), runNames[0])
	touchLog(t, dir, time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC))

	t.Chdir(project)

	loc, err := run.Find(run.DefaultResultsRoot)
	if err != nil {
		t.Fatalf("Find(DefaultResultsRoot): %v", err)
	}

	if want := filepath.Join("target", "gatling", runNames[0]); loc.Dir != want {
		t.Errorf("Dir = %s, want %s", loc.Dir, want)
	}

	if loc.Found != run.FoundByNewest {
		t.Errorf("Found = %v, want %v", loc.Found, run.FoundByNewest)
	}
}

// An empty path is the zero value of every unset flag, config field and omitted
// JSON member. Guessing a root for it would turn missing input into a confident
// report about an unrelated run, so it is refused — and refused before any
// filesystem access, so a server cannot be steered by its own working directory.
//
//nolint:paralleltest // t.Chdir cannot be used from a parallel test.
func TestFindEmptyPathIsRefused(t *testing.T) {
	project := t.TempDir()
	touchLog(t, mkRun(t, filepath.Join(project, "target", "gatling"), runNames[0]),
		time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC))

	// A run really is sitting in the default root: the refusal must not depend
	// on there being nothing to find.
	t.Chdir(project)

	loc, err := run.Find("")
	if !errors.Is(err, run.ErrNoPath) {
		t.Fatalf(`Find("") error = %v, want ErrNoPath`, err)
	}

	if loc != (run.Location{}) {
		t.Errorf("location = %+v, want the zero value beside an error", loc)
	}
}

// T012 — Gradle's layout is not a second thing to search for. It is passed, and
// then it is just a results root like any other.
func TestFindGradleLayout(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	root := filepath.Join(project, "build", "reports", "gatling")
	touchLog(t, mkRun(t, root, runNames[1]), time.Date(2026, time.September, 6, 4, 48, 0, 0, time.UTC))

	// A decoy under the Maven layout in the same project: if the default were
	// consulted at all, this is what would come back.
	touchLog(t, mkRun(t, filepath.Join(project, "target", "gatling"), runNames[2]),
		time.Date(2026, time.September, 6, 4, 49, 0, 0, time.UTC))

	loc, err := run.Find(root)
	if err != nil {
		t.Fatalf("Find(%s): %v", root, err)
	}

	if want := filepath.Join(root, runNames[1]); loc.Dir != want {
		t.Errorf("Dir = %s, want %s — the default root was consulted", loc.Dir, want)
	}
}

// T013 — only a simulation.log makes a directory a run. Gatling stopped
// producing reports in 3.13.5, so a run without one is ordinary; a report
// without a log is not a run at all.
func TestFindCandidates(t *testing.T) {
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

	runDir := mkRun(t, root, runNames[0])
	touchLog(t, runDir, time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC))

	loc, err := run.Find(root)
	if err != nil {
		t.Fatalf("Find(%s): %v", root, err)
	}

	if loc.Dir != runDir {
		t.Errorf("Dir = %s, want %s — a directory without a simulation.log is not a run", loc.Dir, runDir)
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
func TestFindLastRunBeatsModTime(t *testing.T) {
	t.Parallel()

	root := rootWithThreeRuns(t)
	writeLastRun(t, root, runNames[1]+"\n")

	withPointer, err := run.Find(root)
	if err != nil {
		t.Fatalf("Find with lastRun.txt: %v", err)
	}

	if want := filepath.Join(root, runNames[1]); withPointer.Dir != want {
		t.Errorf("Dir = %s, want %s", withPointer.Dir, want)
	}

	if withPointer.Found != run.FoundByLastRun {
		t.Errorf("Found = %v, want %v", withPointer.Found, run.FoundByLastRun)
	}

	if err := os.Remove(filepath.Join(root, "lastRun.txt")); err != nil {
		t.Fatalf("remove lastRun.txt: %v", err)
	}

	withoutPointer, err := run.Find(root)
	if err != nil {
		t.Fatalf("Find without lastRun.txt: %v", err)
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
func TestFindLastRunPointingAtNothing(t *testing.T) {
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

			loc, err := run.Find(root)
			if err != nil {
				t.Fatalf("Find: %v", err)
			}

			if want := filepath.Join(root, runNames[2]); loc.Dir != want {
				t.Errorf("Dir = %s, want %s", loc.Dir, want)
			}

			if loc.Found != run.FoundByNewest {
				t.Errorf("Found = %v, want %v", loc.Found, run.FoundByNewest)
			}
		})
	}
}

// T019 — nothing outside the results root is followed. Each case is asserted
// against a tree where the escape target really is a run, so a passing test
// means the rule bit rather than that the path happened to miss.
func TestFindLastRunContainment(t *testing.T) {
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
				touchLog(t, mkRun(t, root, name), base.Add(time.Duration(i)*time.Minute))
			}

			// A real run outside the root, and a real run nested one level too
			// deep inside it. Both are valid runs; neither is a direct child.
			escape := mkRun(t, parent, "escape")
			touchLog(t, escape, base.Add(time.Hour))
			touchLog(t, mkRun(t, filepath.Join(root, "nested"), "run"), base.Add(time.Hour))

			writeLastRun(t, root, tt.line(escape)+"\n")

			loc, err := run.Find(root)
			if err != nil {
				t.Fatalf("Find: %v", err)
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
func TestFindLastRunShape(t *testing.T) {
	t.Parallel()

	// runNames[0] is the oldest, so naming it proves the file was read: the
	// clock alone would return runNames[2].
	oldest, newestRun := runNames[0], runNames[2]

	tests := []struct {
		name    string
		content string
		want    string
		found   run.FoundBy
	}{
		{name: "bare name, no trailing newline", content: oldest, want: oldest, found: run.FoundByLastRun},
		{name: "trailing LF", content: oldest + "\n", want: oldest, found: run.FoundByLastRun},
		{name: "CRLF, as a Windows build writes", content: oldest + "\r\n", want: oldest, found: run.FoundByLastRun},
		{name: "surrounding whitespace", content: "  " + oldest + "\t \n", want: oldest, found: run.FoundByLastRun},
		{name: "blank lines around the name", content: "\n\n" + oldest + "\n\n", want: oldest, found: run.FoundByLastRun},
		{
			name:    "a name and a trailing error line",
			content: oldest + "\nExecutionError: boom\n",
			want:    oldest, found: run.FoundByLastRun,
		},
		{
			name:    "invalid UTF-8 names nothing",
			content: "\xff\xfe\x00bad\n",
			want:    newestRun, found: run.FoundByNewest,
		},
		{
			name:    "past the cap is treated as absent",
			content: oldest + "\n" + strings.Repeat("x", 64<<10),
			want:    newestRun, found: run.FoundByNewest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := rootWithThreeRuns(t)
			writeLastRun(t, root, tt.content)

			loc, err := run.Find(root)
			if err != nil {
				t.Fatalf("Find: %v", err)
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
func TestFindLastRunMultipleNames(t *testing.T) {
	t.Parallel()

	root := rootWithThreeRuns(t)

	// A fourth run, newer than everything, that this build did not produce: it
	// must not win, or the lines were ignored in favour of the clock.
	stranger := "othersimulation-20260906050000000"
	touchLog(t, mkRun(t, root, stranger), time.Date(2026, time.September, 6, 5, 0, 0, 0, time.UTC))

	writeLastRun(t, root, runNames[0]+"\n"+runNames[1]+"\nExecutionError: boom\n")

	loc, err := run.Find(root)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if want := filepath.Join(root, runNames[1]); loc.Dir != want {
		t.Errorf("Dir = %s, want %s", loc.Dir, want)
	}

	if loc.Found != run.FoundByLastRun {
		t.Errorf("Found = %v, want %v", loc.Found, run.FoundByLastRun)
	}
}

// T026 — what every caller does today keeps working, and the log itself is
// accepted as well as its directory: a script that has the one path and a CI job
// that has the other reach the same run through the same call.
func TestFindNamedPath(t *testing.T) {
	t.Parallel()

	root := rootWithThreeRuns(t)
	runDir := filepath.Join(root, runNames[0])

	byDir, err := run.Find(runDir)
	if err != nil {
		t.Fatalf("Find(dir): %v", err)
	}

	byLog, err := run.Find(filepath.Join(runDir, "simulation.log"))
	if err != nil {
		t.Fatalf("Find(log): %v", err)
	}

	if byDir != byLog {
		t.Errorf("Find(dir) = %+v, Find(log) = %+v; want identical", byDir, byLog)
	}

	if byDir.Dir != runDir || byDir.Found != run.FoundByPath {
		t.Errorf("got %+v, want Dir %s found by path", byDir, runDir)
	}
}

// T027 — a caller that named a place is never quietly answered about a different
// one. runNames[0] is the oldest, so a scan would have returned its sibling.
func TestFindNamedPathIsNeverSearchedPast(t *testing.T) {
	t.Parallel()

	root := rootWithThreeRuns(t)
	runDir := filepath.Join(root, runNames[0])

	// A pointer naming a different run, to prove neither rule outranks a path.
	writeLastRun(t, root, runNames[2]+"\n")

	loc, err := run.Find(runDir)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if loc.Dir != runDir {
		t.Errorf("Dir = %s, want %s — a named run was displaced", loc.Dir, runDir)
	}
}

// T028 — a directory holding a log is a run, not a results root, even when runs
// sit beneath it. Anything else would make an archive of archives ambiguous.
func TestFindDirectoryHoldingLogIsTheRun(t *testing.T) {
	t.Parallel()

	outer := t.TempDir()
	if err := os.WriteFile(filepath.Join(outer, "simulation.log"), nil, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	touchLog(t, mkRun(t, outer, runNames[2]), time.Date(2026, time.September, 6, 5, 0, 0, 0, time.UTC))

	loc, err := run.Find(outer)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if loc.Dir != outer || loc.Found != run.FoundByPath {
		t.Errorf("got %+v, want the directory itself (%s) found by path", loc, outer)
	}
}

// T029 — an archive that stores runs elsewhere and links them in still resolves.
// This is the case os.Root would have refused, and the reason containment here
// is a rule about the text of a lastRun.txt line rather than about the
// filesystem.
func TestFindSymlinkedRun(t *testing.T) {
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

	loc, err := run.Find(root)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if loc.Dir != link {
		t.Errorf("Dir = %s, want %s — a symlinked run was not followed", loc.Dir, link)
	}
}

// T022 — US1 scenario 5, completed. A consumer that reports which run it read
// can say how it was chosen, and the three rules are distinguishable.
func TestFindReportsWhichRuleChose(t *testing.T) {
	t.Parallel()

	root := rootWithThreeRuns(t)

	byNewest, err := run.Find(root)
	if err != nil {
		t.Fatalf("Find(root): %v", err)
	}

	writeLastRun(t, root, runNames[1]+"\n")

	byPointer, err := run.Find(root)
	if err != nil {
		t.Fatalf("Find(root) with pointer: %v", err)
	}

	byPath, err := run.Find(filepath.Join(root, runNames[0]))
	if err != nil {
		t.Fatalf("Find(runDir): %v", err)
	}

	got := [...]run.FoundBy{byNewest.Found, byPointer.Found, byPath.Found}
	want := [...]run.FoundBy{run.FoundByNewest, run.FoundByLastRun, run.FoundByPath}

	if got != want {
		t.Errorf("Found = %v, want %v", got, want)
	}
}

// T031 — the whole cost of pointing a consumer at the wrong place is how long it
// takes to learn where the right one is, so every failure names the directory it
// read. The path is always the caller's own: nothing is ever substituted.
func TestFindNotFound(t *testing.T) {
	t.Parallel()

	empty := t.TempDir()
	missing := filepath.Join(t.TempDir(), "no", "such", "place")

	notLogFile := filepath.Join(t.TempDir(), "results.zip")
	if err := os.WriteFile(notLogFile, []byte("PK"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{name: "a results root holding no run", path: empty},
		{name: "a path that is not there", path: missing},
		// A file that exists but is not a simulation.log is the commonest typo
		// in this feature. It must reach the same error type as every other
		// "no run here", not a raw ENOTDIR from the syscall layer.
		{name: "a regular file that is not a log", path: notLogFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			loc, err := run.Find(tt.path)

			var notFound *run.NotFoundError
			if !errors.As(err, &notFound) {
				t.Fatalf("Find(%q) error = %v, want a *NotFoundError", tt.path, err)
			}

			if notFound.Dir != tt.path {
				t.Errorf("Dir = %s, want %s", notFound.Dir, tt.path)
			}

			if loc != (run.Location{}) {
				t.Errorf("location = %+v, want the zero value beside an error", loc)
			}

			mustContain(t, err.Error(), tt.path)
		})
	}
}

// T032 — the test that rots. A version treating every read error as "no runs
// here" passes every other case in this file and turns a broken mount, a bad
// permission or a dead network share into a clean, wrong answer.
func TestFindUnreadableDirectory(t *testing.T) {
	t.Parallel()

	requireUnixPermissions(t)

	root := t.TempDir()
	touchLog(t, mkRun(t, root, runNames[0]), time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC))

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

	_, err := run.Find(root)
	if err == nil {
		t.Fatal("Find on an unreadable directory returned no error")
	}

	var notFound *run.NotFoundError
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
func TestFindNamedPathDoesNotFallBackToDefault(t *testing.T) {
	project := t.TempDir()
	touchLog(t, mkRun(t, filepath.Join(project, "target", "gatling"), runNames[2]),
		time.Date(2026, time.September, 6, 5, 0, 0, 0, time.UTC))

	elsewhere := filepath.Join(project, "somewhere-else")
	if err := os.MkdirAll(elsewhere, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	t.Chdir(project)

	_, err := run.Find(elsewhere)

	var notFound *run.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("error = %v, want a *NotFoundError", err)
	}

	if notFound.Dir != elsewhere {
		t.Errorf("Dir = %s, want %s — a path the caller never gave was searched", notFound.Dir, elsewhere)
	}
}

// T037 — FR-013, asserted positively rather than by absence. Discovery opens no
// simulation.log, so a root full of logs that are not Gatling logs, or that
// cannot be opened at all, still resolves; the version gate and the codec are
// the reader's job and fire later.
func TestFindNeverOpensTheLog(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	base := time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC)

	// A log that is not a Gatling log at all.
	garbage := mkRun(t, root, runNames[0])
	if err := os.WriteFile(filepath.Join(garbage, "simulation.log"), []byte("not a gatling log"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	touchLog(t, garbage, base)

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

	touchLog(t, unreadable, base.Add(time.Minute))

	loc, err := run.Find(root)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if loc.Dir != unreadable {
		t.Errorf("Dir = %s, want %s — discovery is not affected by what a log contains", loc.Dir, unreadable)
	}
}

// The tie-break has to survive a root holding two simulations, which is what
// Maven's runMultipleSimulations produces. Whole-name order is alphabetical by
// simulation id first, so it would hand back a run that started months earlier;
// the run id's own UTC stamp is the only thing in the name that is about time.
func TestFindTieBreakAcrossSimulationIds(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	same := time.Date(2026, time.September, 9, 0, 0, 0, 0, time.UTC)

	older := "zzzsimulation-20260101000000000" // alphabetically last, ran in January
	newer := "aaasimulation-20260909000000000" // alphabetically first, ran in September

	for _, name := range []string{older, newer} {
		touchLog(t, mkRun(t, root, name), same)
	}

	loc, err := run.Find(root)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if want := filepath.Join(root, newer); loc.Dir != want {
		t.Errorf("Dir = %s, want %s — whole-name order picked the older run", loc.Dir, want)
	}
}

// A name without a run-id stamp still has to resolve, and resolve the same way
// every time: an archive is free to rename its directories.
func TestFindTieBreakWithoutRunIDStamp(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	same := time.Date(2026, time.September, 9, 0, 0, 0, 0, time.UTC)

	for _, name := range []string{"archived-run", "another-run", "sim-2026090900000000"} { // 16 digits, not 17
		touchLog(t, mkRun(t, root, name), same)
	}

	first, err := run.Find(root)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	for i := range 10 {
		again, err := run.Find(root)
		if err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}

		if again.Dir != first.Dir {
			t.Fatalf("attempt %d chose %s, attempt 0 chose %s — the ordering is not total",
				i, again.Dir, first.Dir)
		}
	}
}

// The ordering reads the log's time, not the run directory's. Regenerating a
// report writes an index.html and a js tree into an old run and moves that
// directory's mtime; the run itself did not happen again. This is the scenario
// US2 exists to prevent, and the pointer file that would otherwise defend
// against it is absent for almost every caller.
func TestFindIgnoresDirectoryMtime(t *testing.T) {
	t.Parallel()

	root := rootWithThreeRuns(t)
	oldest := filepath.Join(root, runNames[0])

	// A report regenerated into the oldest run, long after every other run.
	if err := os.WriteFile(filepath.Join(oldest, "index.html"), nil, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	touchDir(t, oldest, time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC))

	loc, err := run.Find(root)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if want := filepath.Join(root, runNames[2]); loc.Dir != want {
		t.Errorf("Dir = %s, want %s — a regenerated report made an old run look newest", loc.Dir, want)
	}
}

// One run, however it is spelled, is one Location. Location is comparable
// and consumers key caches on Dir, so a path with a trailing separator or a "."
// segment must not become a second run.
func TestFindCanonicalisesPaths(t *testing.T) {
	t.Parallel()

	root := rootWithThreeRuns(t)
	name := runNames[0]
	runDir := filepath.Join(root, name)

	spellings := []string{
		runDir,
		runDir + string(filepath.Separator),
		filepath.Join(root, ".", name),
		root + string(filepath.Separator) + "." + string(filepath.Separator) + name,
		filepath.Join(runDir, "simulation.log"),
	}

	seen := map[run.Location]bool{}

	for _, spelling := range spellings {
		loc, err := run.Find(spelling)
		if err != nil {
			t.Fatalf("Find(%q): %v", spelling, err)
		}

		if loc.Log != filepath.Join(loc.Dir, "simulation.log") {
			t.Errorf("Find(%q): Log = %s, want it to be Dir joined with the log name", spelling, loc.Log)
		}

		seen[loc] = true
	}

	if len(seen) != 1 {
		t.Errorf("%d distinct Location values for one run: %v", len(seen), seen)
	}
}

// A failure to look is not an absence of runs. This is the case the root-level
// permission test cannot reach: the root reads fine and the run inside it does
// not.
func TestFindUnreadableRunDirectory(t *testing.T) {
	t.Parallel()

	requireUnixPermissions(t)

	root := t.TempDir()
	only := mkRun(t, root, runNames[0])
	touchLog(t, only, time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC))

	if err := os.Chmod(only, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	t.Cleanup(func() {
		if err := os.Chmod(only, 0o700); err != nil { //nolint:gosec // a directory this test must leave traversable for its own cleanup
			t.Errorf("restoring permissions: %v", err)
		}
	})

	_, err := run.Find(root)

	var notFound *run.NotFoundError
	if errors.As(err, &notFound) {
		t.Fatalf("error = %v; an unreadable run directory was reported as an absence of runs", err)
	}

	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("error = %v, want it to carry the permission failure", err)
	}
}

// A pointer that cannot be read is not the same as no pointer. Falling through
// to the clock would hand back a different run with nothing said about it.
func TestFindUnreadableLastRun(t *testing.T) {
	t.Parallel()

	requireUnixPermissions(t)

	root := rootWithThreeRuns(t)
	writeLastRun(t, root, runNames[0]+"\n")

	pointer := filepath.Join(root, "lastRun.txt")
	if err := os.Chmod(pointer, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	t.Cleanup(func() {
		if err := os.Chmod(pointer, 0o600); err != nil {
			t.Errorf("restoring permissions: %v", err)
		}
	})

	loc, err := run.Find(root)
	if err == nil {
		t.Fatalf("Find returned %s with no error; an unreadable pointer was silently ignored",
			filepath.Base(loc.Dir))
	}

	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("error = %v, want it to carry the permission failure", err)
	}
}

// What a run directory is called is Gatling's to choose and an archive's to
// rewrite, so nothing may turn on the name — including the name of the log.
func TestFindDirectoryNamedLikeTheLog(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	odd := mkRun(t, root, "simulation.log")
	touchLog(t, odd, time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC))

	loc, err := run.Find(odd)
	if err != nil {
		t.Fatalf("Find(%s): %v", odd, err)
	}

	if loc.Dir != odd || loc.Found != run.FoundByPath {
		t.Errorf("got %+v, want the directory itself (%s) found by path", loc, odd)
	}
}

// A root that mixes Gatling-stamped names with a renamed directory is where a
// pairwise tie-break broke. It compared stamps when both names carried one and
// whole names otherwise, and switching rule per pair made the order cyclic —
// A beats B by stamp, B beats C by name, C beats A by name — so the run
// returned depended on which candidate the scan reached last, which
// os.ReadDir's name order decided: adding the renamed directory changed the
// answer from the run stamped 2099 to the run stamped 2020, while Found still
// said the ordinary rule had applied.
func TestFindMixedStampedAndUnstampedNames(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	same := time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC)

	for _, name := range []string{"simA-20990101000000000", "simB-20200101000000000", "simAA"} {
		touchLog(t, mkRun(t, root, name), same)
	}

	loc, err := run.Find(root)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if want := filepath.Join(root, "simA-20990101000000000"); loc.Dir != want {
		t.Errorf("Dir = %s, want %s — the run with the latest stamp", loc.Dir, want)
	}

	if loc.Found != run.FoundByNewest {
		t.Errorf("Found = %s, want %s", loc.Found, run.FoundByNewest)
	}
}
