package run

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// DefaultResultsRoot is where Maven and sbt both write Gatling's output.
//
// It is exported rather than applied automatically: resolved against a process
// working directory it means one thing for a CLI run inside a project and
// something else entirely for a server, and Find cannot tell the two apart.
// A caller that wants it says so — run.Find(run.DefaultResultsRoot)
// — and one that has its own path is never second-guessed.
//
// Gradle writes to "build/reports/gatling", and a run configured by hand writes
// wherever it was told to.
const DefaultResultsRoot = "target/gatling"

// The names this package looks for inside a results root.
const (
	// logName is the file whose presence makes a directory a run. Both formats
	// Gatling has written share it, which is why gatling/simlog exists.
	logName = "simulation.log"
	// lastRunFile is what gatling-maven-plugin leaves in the results root naming
	// the run directories its execution created. Nothing else writes one: not
	// Gatling, not the Gradle plugin, and not even that plugin unless its
	// failOnError parameter is turned off, which is not its default. See Find
	// for what is assumed of it.
	lastRunFile = "lastRun.txt"
	// maxLastRunSize caps the read of lastRunFile. The real file holds one short
	// line per run of a single build; anything past this is not that file, and is
	// treated as no pointer at all rather than loaded into memory.
	maxLastRunSize = 64 << 10
	// runIDTimeLen is the length of the yyyyMMddHHmmssSSS stamp Gatling ends a
	// run id with. See compareRuns for what it is used for and what it is not.
	runIDTimeLen = 17
)

// ErrNoPath is returned by Find when it is given an empty path.
//
// It exists because "" is the zero value of every configuration field, every
// command-line flag left unset and every omitted JSON member. Guessing a results
// root for it would turn missing input into a successful read of some unrelated
// run, which is worse than any error: the caller gets a plausible report about a
// test nobody asked for. Pass DefaultResultsRoot to ask for the usual layout.
var ErrNoPath = errors.New("gatling: no path given; pass a results root, a run directory, or run.DefaultResultsRoot")

// Location is where a Gatling run's artefacts sit, and how they were found.
//
// It is not a result. A run's records are read by opening Log through one of the
// codecs; this type is discarded the moment that happens, and the canonical
// result of a run is model.Run.
type Location struct {
	// Dir is the run directory, cleaned: absolute if the caller's path was,
	// relative if it was not. Two spellings of one run yield one Dir, so a
	// Location is safe to compare and to use as a map key.
	Dir string
	// Log is the simulation.log inside Dir. It is always Dir joined with
	// "simulation.log": a directory is a run because it holds one, so the two
	// are never independently true.
	Log string
	// Found is the rule that selected this run.
	Found FoundBy
}

// FoundBy is how a run was chosen.
//
// A caller that reports which run it read should report this too, because
// FoundByNewest is a guess from modification times — and it is the ordinary
// outcome rather than the exceptional one. A lastRun.txt is written only by
// gatling-maven-plugin, only when its failOnError parameter is set to false, and
// its own gatling:verify goal deletes the file again; neither the Gradle plugin
// nor Gatling itself writes one at all.
type FoundBy uint8

// The three rules that can select a run, behind the unknown sentinel.
const (
	// FoundByUnknown is the zero value: no run was selected. Find never
	// returns it with a nil error.
	FoundByUnknown FoundBy = iota
	// FoundByPath means the path named the run itself, as a directory or as the
	// simulation.log inside it. Nothing was searched.
	FoundByPath
	// FoundByLastRun means lastRun.txt named it and it was still there. When the
	// file names several runs that all still exist — a Maven build running
	// multiple simulations — the newest of those is taken, by the same ordering
	// FoundByNewest uses.
	FoundByLastRun
	// FoundByNewest means it was the most recently modified run in the results
	// root, ties broken by the UTC stamp the run id ends with and then by name,
	// a name without a stamp ranking below every name with one. This is a
	// guess: see compareRuns for what the ordering can and cannot say.
	FoundByNewest
)

var foundByNames = [...]string{unknownName, "path", "lastRun.txt", "newest"}

// String names the rule: "path", "lastRun.txt", "newest", or "unknown" for the
// zero value.
func (f FoundBy) String() string {
	if int(f) < len(foundByNames) {
		return foundByNames[f]
	}

	return "FoundBy(" + strconv.Itoa(int(f)) + ")"
}

// Find locates a Gatling run. It returns where the run's artefacts sit and
// which rule chose them; it opens no simulation.log and applies no version gate,
// so a run whose log is truncated, damaged or out of the supported range still
// resolves and fails only when the log is read.
//
// path may name the run itself — a simulation.log, or a directory holding one —
// in which case it is returned as given and nothing else is searched. Otherwise
// path is a results root and its immediate children are searched: the run named
// by lastRun.txt if that file names one that is still there, and failing that
// the most recently modified run in the root. That file is rarer than it looks —
// only a Maven build with failOnError turned off leaves one — so the second rule
// is the ordinary one; see FoundBy.
//
// path is required. An empty path returns ErrNoPath rather than a guess; pass
// DefaultResultsRoot for the results root Maven and sbt write to.
//
// When no run is found it returns a *NotFoundError naming the directory that
// was searched. A directory that cannot be read is reported as that failure,
// wrapping the *fs.PathError, and never as an absence of runs.
func Find(path string) (Location, error) {
	if path == "" {
		return Location{}, ErrNoPath
	}

	// Cleaned once, here, so that every path this function reports is canonical
	// however the caller spelled it: "x/", "./x" and "a/../x" are one run and
	// must produce one Location.
	root := filepath.Clean(path)

	if loc, ok := asRun(root); ok {
		return loc, nil
	}

	return findInRoot(root)
}

// asRun reports whether the path names a run outright, and returns it if so.
//
// Two shapes count: the simulation.log itself, which a script may be holding
// alone, and a directory that directly contains one. A directory holding a log
// is a run and never a results root, even when runs sit beneath it too —
// otherwise an archive of archives would be ambiguous, and a caller that named a
// place could be answered about a different one.
//
// A directory *named* simulation.log is still tested for a log inside it: what a
// run directory is called is Gatling's to choose and an archive's to rewrite, so
// nothing here may turn on it.
func asRun(path string) (Location, bool) {
	if filepath.Base(path) == logName && isRegular(path) {
		return Location{Dir: filepath.Dir(path), Log: path, Found: FoundByPath}, true
	}

	if log := filepath.Join(path, logName); isRegular(log) {
		return Location{Dir: path, Log: log, Found: FoundByPath}, true
	}

	return Location{}, false
}

// isRegular reports whether path is a regular file that can be stat-ed.
func isRegular(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.Mode().IsRegular()
}

// notADirectory reports whether path exists and is something other than a
// directory — an archive, a log, a typo that landed on a file.
//
// It is asked only after a look has already failed, so it costs nothing on the
// path that works, and it is how "you pointed at a file" is told from "the
// directory is broken" without naming a platform's errno.
func notADirectory(path string) bool {
	info, err := os.Stat(path)

	return err == nil && !info.IsDir()
}

// findInRoot searches a results root: the run lastRun.txt names, else the newest.
func findInRoot(root string) (Location, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		// Nothing there, or there but not a directory, means the caller's
		// question — "which run is here?" — has the answer "none". A directory
		// that could not be *read* is a different answer and keeps its cause:
		// reporting a bad permission, a broken mount or a dead network share as
		// a clean "no runs here" sends a caller looking for a run that was never
		// the problem.
		if errors.Is(err, fs.ErrNotExist) || notADirectory(root) {
			return Location{}, &NotFoundError{Dir: root}
		}

		return Location{}, fmt.Errorf("gatling: reading results root %s: %w", root, err)
	}

	runs, err := runsIn(root, entries)
	if err != nil {
		return Location{}, err
	}

	if len(runs) == 0 {
		return Location{}, &NotFoundError{Dir: root}
	}

	named, err := namedByLastRun(root, runs)
	if err != nil {
		return Location{}, err
	}

	if len(named) > 0 {
		return located(root, newest(named), FoundByLastRun), nil
	}

	return located(root, newest(runs), FoundByNewest), nil
}

// runDir is a candidate: a directory in the results root that holds a log, and
// the modification time the ordering rule compares.
type runDir struct {
	name string
	mod  time.Time
}

// runsIn keeps the entries that are runs, at one stat each.
//
// The time it keeps is the *log's*, not the run directory's. A directory's
// modification time moves whenever anything is added to it, and regenerating a
// report writes an index.html and a js tree into an old run — which would make
// that run the newest and hand a caller the wrong test, the exact failure US2
// exists to prevent. The log is written by the run and by nothing else.
//
// Nothing checks whether the entry is itself a directory: a name holding a
// simulation.log is one, and asking the question this way follows a symlinked
// run into another tree, which is how an archive that stores runs elsewhere and
// links them in keeps working.
//
// An entry that cannot be inspected is an error, not a skip. Silently dropping a
// candidate whose permissions or mount are broken is how a readable root with an
// unreadable run inside it comes to report "no runs here" — an absence of runs
// that is really a failure to look.
func runsIn(root string, entries []fs.DirEntry) ([]runDir, error) {
	runs := make([]runDir, 0, len(entries))

	for _, entry := range entries {
		name := entry.Name()

		info, err := os.Stat(filepath.Join(root, name, logName))
		if err != nil {
			if skippable(root, name, err) {
				continue
			}

			return nil, fmt.Errorf("gatling: reading run directory %s: %w", filepath.Join(root, name), err)
		}

		if !info.Mode().IsRegular() {
			continue
		}

		runs = append(runs, runDir{name: name, mod: info.ModTime()})
	}

	return runs, nil
}

// skippable reports whether a failed look inside an entry means "not a run"
// rather than "could not tell".
//
// Two shapes are ordinary and carry no information: the entry holds no log, and
// the entry is not a directory at all — lastRun.txt sits in every Maven results
// root and cannot have children. Everything else is a failure to look.
func skippable(root, name string, err error) bool {
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}

	return notADirectory(filepath.Join(root, name))
}

// namedByLastRun keeps the candidates that lastRun.txt names.
//
// The file is a hint and never an instruction. gatling-maven-plugin writes one
// bare directory name per run its execution created, followed by an error
// message when the run failed — and it is the only thing that writes one at all:
// not Gatling, not the Gradle plugin, not that plugin under its default
// configuration, and its own gatling:verify goal deletes the file afterwards. So
// absence is ordinary, and every line is checked against the candidates rather
// than trusted.
//
// That check is what makes the error line harmless: it names no directory, so it
// fails the same membership test a deleted run fails. Nothing here matches the
// plugin's error text, which is free to change.
//
// The names are collected into a set before the candidates are walked, so a
// garbage file cannot turn the lookup into a scan of every run for every line.
func namedByLastRun(root string, runs []runDir) ([]runDir, error) {
	lines, err := readLastRun(root)
	if err != nil {
		return nil, err
	}

	wanted := make(map[string]struct{}, len(lines))

	for _, line := range lines {
		if isBareName(line) {
			wanted[line] = struct{}{}
		}
	}

	if len(wanted) == 0 {
		return nil, nil
	}

	named := make([]runDir, 0, min(len(wanted), len(runs)))

	for _, run := range runs {
		if _, ok := wanted[run.name]; ok {
			named = append(named, run)
		}
	}

	return named, nil
}

// readLastRun reads the pointer file into trimmed, non-empty lines. A file that
// is not there is not an error and yields no lines.
//
// The size is taken from the open descriptor and the read is bounded by it, so
// the cap holds against a file still being appended to by a concurrent build — a
// stat of the path could only ever have been a hint about some earlier moment.
// Java writes the file as UTF-8 with System.lineSeparator(), so a line may carry
// a trailing carriage return.
func readLastRun(root string) ([]string, error) {
	path := filepath.Join(root, lastRunFile)

	f, err := os.Open(path) //nolint:gosec // lastRun.txt inside the caller's own results root.
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("gatling: reading %s: %w", path, err)
	}

	defer f.Close() //nolint:errcheck // read-only

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("gatling: reading %s: %w", path, err)
	}

	if !info.Mode().IsRegular() || info.Size() > maxLastRunSize {
		return nil, nil
	}

	data, err := io.ReadAll(io.LimitReader(f, maxLastRunSize))
	if err != nil {
		return nil, fmt.Errorf("gatling: reading %s: %w", path, err)
	}

	text := string(data)
	lines := make([]string, 0, strings.Count(text, "\n")+1)

	for line := range strings.SplitSeq(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}

	return lines, nil
}

// isBareName reports whether a line could name a direct child of the results
// root: no separator, no traversal, nothing absolute.
//
// This is a guard against a stale or garbage file sending discovery somewhere
// surprising, and not a security boundary — the file is written by the caller's
// own build into the caller's own results root, and whoever can write it can
// write the run directories beside it. os.Root would enforce containment by
// construction, and is deliberately not used: it refuses a symlink leaving the
// root, and an archived run linked into a tree has to keep resolving.
func isBareName(line string) bool {
	if line == "." || line == ".." {
		return false
	}

	if strings.ContainsRune(line, '/') || strings.ContainsRune(line, filepath.Separator) {
		return false
	}

	return line == filepath.Base(line)
}

// newest names the most recently modified run, breaking ties on the run's own
// recorded start and then on the directory name, all descending: the maximum
// under compareRuns.
func newest(runs []runDir) string {
	return slices.MaxFunc(runs, compareRuns).name
}

// compareRuns orders two candidates by one key, and the order it imposes is
// worth stating exactly.
//
// The modification time of the log comes first, which is #11's rule. It is a
// proxy for when the run happened and not a record of it: a git clone, an rsync
// without -t, a CI cache restore and a container image build each give every run
// in a root one timestamp — and those are the same callers who have no
// lastRun.txt to prefer, because only gatling-maven-plugin writes one.
//
// For them the tie-break is the whole selection, so it has to be total, and it
// is also the one place a run's real start is recoverable: Gatling names a run
// directory <simulationId>-<yyyyMMddHHmmssSSS>, in UTC. Comparing that stamp
// rather than the whole name matters as soon as a root holds two simulations —
// which is what Maven's runMultipleSimulations produces — because whole-name
// order is alphabetical by simulation id first, and would hand back a run that
// started months earlier.
//
// A name without the stamp compares as an empty stamp, so it ranks below every
// name that carries one, and among its own kind by name. That is what makes
// the order total: a rule that compared stamps for one pair and whole names for
// the next was cyclic on a root mixing the two, and a linear maximum over a
// cycle returns whichever candidate the scan reached last — os.ReadDir's name
// order, in practice. It is also the right answer: the stamp is the only
// evidence in a name about when a run started, and a name without one says
// nothing about time. An archive that renamed every directory still resolves
// deterministically, by name.
func compareRuns(a, b runDir) int {
	if c := a.mod.Compare(b.mod); c != 0 {
		return c
	}

	as, _ := runStart(a.name)
	bs, _ := runStart(b.name)

	if c := strings.Compare(as, bs); c != 0 {
		return c
	}

	return strings.Compare(a.name, b.name)
}

// runStart returns the yyyyMMddHHmmssSSS stamp a Gatling run id ends with, and
// whether the name carries one. It is compared as text, never parsed: the stamp
// is fixed-width and zero-padded, so it sorts in time order as it stands.
func runStart(name string) (string, bool) {
	hyphen := strings.LastIndexByte(name, '-')
	if hyphen < 0 {
		return "", false
	}

	stamp := name[hyphen+1:]
	if len(stamp) != runIDTimeLen {
		return "", false
	}

	for i := range len(stamp) {
		if stamp[i] < '0' || stamp[i] > '9' {
			return "", false
		}
	}

	return stamp, true
}

// located builds the result for a run directory named relative to root.
func located(root, name string, found FoundBy) Location {
	dir := filepath.Join(root, name)

	return Location{Dir: dir, Log: filepath.Join(dir, logName), Found: found}
}
