package gatling

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The names this package looks for, and the root it looks in when told nothing.
const (
	// logName is the file whose presence makes a directory a run. Both formats
	// Gatling has written share it, which is why gatling/simlog exists.
	logName = "simulation.log"
	// lastRunFile is what gatling-maven-plugin leaves in the results root naming
	// the run directories its execution created. Nothing else writes one: not
	// Gatling, not the Gradle plugin, and not even that plugin unless its
	// failOnError parameter is turned off, which is not its default. See FindRun
	// for what is assumed of it.
	lastRunFile = "lastRun.txt"
	// defaultResultsRoot is where Maven and sbt both put Gatling's output, and
	// what FindRun searches when it is given no path. Gradle uses
	// "build/reports/gatling" instead, which is passed rather than guessed.
	defaultResultsRoot = "target/gatling"
	// maxLastRunSize caps the read of lastRunFile. The real file holds one short
	// line per run of a single build; anything past this is not that file, and is
	// treated as no pointer at all rather than loaded into memory.
	maxLastRunSize = 64 << 10
)

// RunLocation is where a Gatling run's artefacts sit, and how they were found.
//
// It is not a result. A run's records are read by opening Log through one of the
// codecs; this type is discarded the moment that happens, and the canonical
// result of a run is model.Run.
type RunLocation struct {
	// Dir is the run directory.
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
	// FoundByUnknown is the zero value: no run was selected. FindRun never
	// returns it with a nil error.
	FoundByUnknown FoundBy = iota
	// FoundByPath means the caller named the run, as a directory or as the
	// simulation.log inside it. Nothing was searched.
	FoundByPath
	// FoundByLastRun means lastRun.txt named it and it was still there.
	FoundByLastRun
	// FoundByNewest means it was the most recently modified run in the results
	// root. Ties break on the directory name, descending, which is run-start
	// order for the names Gatling generates.
	FoundByNewest
)

var foundByNames = [...]string{unknownName, "path", lastRunFile, "newest"}

// String names the rule: "path", "lastRun.txt", "newest", or "unknown" for the
// zero value.
func (f FoundBy) String() string {
	if int(f) < len(foundByNames) {
		return foundByNames[f]
	}

	return "FoundBy(" + strconv.Itoa(int(f)) + ")"
}

// FindRun locates a Gatling run. It returns where the run's artefacts sit and
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
// An empty path means the results root Maven and sbt write to, target/gatling,
// relative to the working directory. Gradle writes to build/reports/gatling and
// a run configured by hand writes wherever it was told to; pass either as path.
//
// When no run is found it returns a *RunNotFoundError naming the directory that
// was searched and saying whether that directory was the default. A directory
// that cannot be read is reported as that failure, wrapping the *fs.PathError,
// and never as an absence of runs.
func FindRun(path string) (RunLocation, error) {
	root, byDefault := path, false
	if root == "" {
		root, byDefault = defaultResultsRoot, true
	}

	if loc, ok := asRun(root); ok {
		return loc, nil
	}

	return findInRoot(root, byDefault)
}

// asRun reports whether the path names a run outright, and returns it if so.
//
// Two shapes count: the simulation.log itself, which a script may be holding
// alone, and a directory that directly contains one. A directory holding a log
// is a run and never a results root, even when runs sit beneath it too —
// otherwise an archive of archives would be ambiguous, and a caller that named a
// place could be answered about a different one.
func asRun(path string) (RunLocation, bool) {
	if filepath.Base(path) == logName {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return RunLocation{Dir: filepath.Dir(path), Log: path, Found: FoundByPath}, true
		}

		return RunLocation{}, false
	}

	if info, err := os.Stat(filepath.Join(path, logName)); err == nil && info.Mode().IsRegular() {
		return RunLocation{Dir: path, Log: filepath.Join(path, logName), Found: FoundByPath}, true
	}

	return RunLocation{}, false
}

// findInRoot searches a results root: the run lastRun.txt names, else the newest.
func findInRoot(root string, byDefault bool) (RunLocation, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		// A directory that is not there held no run, which is the question the
		// caller asked. A directory that could not be *read* is a different
		// answer and keeps its cause: reporting a bad permission, a broken mount
		// or a dead network share as a clean "no runs here" sends a caller
		// looking for a run that was never the problem.
		if errors.Is(err, fs.ErrNotExist) {
			return RunLocation{}, &RunNotFoundError{Dir: root, Default: byDefault}
		}

		return RunLocation{}, fmt.Errorf("gatling: reading results root %s: %w", root, err)
	}

	runs := runsIn(root, entries)
	if len(runs) == 0 {
		return RunLocation{}, &RunNotFoundError{Dir: root, Default: byDefault}
	}

	if named := namedByLastRun(root, runs); len(named) > 0 {
		return located(root, newest(named), FoundByLastRun), nil
	}

	return located(root, newest(runs), FoundByNewest), nil
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
func namedByLastRun(root string, runs []runDir) []runDir {
	lines, ok := readLastRun(root)
	if !ok {
		return nil
	}

	named := make([]runDir, 0, len(lines))

	for _, line := range lines {
		if !isBareName(line) {
			continue
		}

		for _, run := range runs {
			if run.name == line {
				named = append(named, run)

				break
			}
		}
	}

	return named
}

// readLastRun reads the pointer file into trimmed, non-empty lines.
//
// A file past maxLastRunSize is reported as absent rather than read: the real
// one holds a few short names, so anything larger is not this file, and a
// pointer is never worth an unbounded allocation. Java writes it as UTF-8 with
// System.lineSeparator(), so a line may carry a trailing carriage return.
func readLastRun(root string) ([]string, bool) {
	path := filepath.Join(root, lastRunFile)

	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxLastRunSize {
		return nil, false
	}

	data, err := os.ReadFile(path) //nolint:gosec // The path is the caller's own results root, capped above.
	if err != nil {
		return nil, false
	}

	lines := make([]string, 0, bytes.Count(data, []byte{'\n'})+1)

	for line := range strings.SplitSeq(string(data), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}

	return lines, true
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
	if line == "" || line == "." || line == ".." {
		return false
	}

	if strings.ContainsRune(line, '/') || strings.ContainsRune(line, filepath.Separator) {
		return false
	}

	return line == filepath.Base(line)
}

// runDir is a candidate: a directory in the results root that holds a log, and
// the modification time the ordering rule compares.
type runDir struct {
	name string
	mod  time.Time
}

// runsIn keeps the entries that are runs. An entry costs one stat to test for a
// log, and a second only if it turns out to be a run.
//
// Nothing checks whether the entry is itself a directory: a name holding a
// simulation.log is one, and asking the question this way follows a symlinked
// run into another tree, which is how an archive that stores runs elsewhere and
// links them in keeps working.
func runsIn(root string, entries []fs.DirEntry) []runDir {
	runs := make([]runDir, 0, len(entries))

	for _, entry := range entries {
		name := entry.Name()
		if !holdsLog(root, name) {
			continue
		}

		info, err := os.Stat(filepath.Join(root, name))
		if err != nil {
			// Removed between the listing and the stat. A run that is no longer
			// there is not a candidate, and a racing build is not an error here.
			continue
		}

		runs = append(runs, runDir{name: name, mod: info.ModTime()})
	}

	return runs
}

// holdsLog reports whether root/name directly contains a simulation.log.
func holdsLog(root, name string) bool {
	info, err := os.Stat(filepath.Join(root, name, logName))

	return err == nil && info.Mode().IsRegular()
}

// newest names the most recently modified run, breaking ties on the directory
// name, descending.
//
// The tie-break is not a formality. A git clone, an rsync without -t, a CI cache
// restore and a container image build all give every run in a root one
// modification time — and those are the same callers who have no lastRun.txt to
// prefer, because only gatling-maven-plugin writes one. For them this comparison
// is the whole selection, so it has to be total. Descending name is also the
// right answer rather than merely a stable one: Gatling names a run directory
// <simulationId>-<yyyyMMddHHmmssSSS>, the run's start in UTC, which sorts in run
// order — and in UTC it keeps doing so across a daylight-saving change.
func newest(runs []runDir) string {
	best := runs[0]

	for _, run := range runs[1:] {
		if later(run, best) {
			best = run
		}
	}

	return best.name
}

// later orders two candidates: modification time first, then name, descending.
func later(a, b runDir) bool {
	if !a.mod.Equal(b.mod) {
		return a.mod.After(b.mod)
	}

	return a.name > b.name
}

// located builds the result for a run directory named relative to root.
func located(root, name string, found FoundBy) RunLocation {
	dir := filepath.Join(root, name)

	return RunLocation{Dir: dir, Log: filepath.Join(dir, logName), Found: found}
}
