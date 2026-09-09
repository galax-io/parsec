package gatling_test

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/simlog"
)

// Read an archived run whose directory is known, without joining "simulation.log"
// by hand. FindRun stops one line short of opening the log, so the version gate
// and the codec still belong to the reader.
func ExampleFindRun() {
	loc, err := gatling.FindRun(filepath.Join("..", "testdata", "corpus", "gatling", "3.11.5"))
	if err != nil {
		panic(err)
	}

	f, err := os.Open(loc.Log)
	if err != nil {
		panic(err)
	}

	defer f.Close() //nolint:errcheck // read-only

	rd, err := simlog.NewRunReader(f)
	if err != nil {
		panic(err)
	}

	fmt.Println("chosen by:", loc.Found)
	fmt.Println("log:", filepath.Base(loc.Log))
	fmt.Println("gatling:", rd.Run().ToolVersion)
	// Output:
	// chosen by: path
	// log: simulation.log
	// gatling: 3.11.5
}

// Point at a results root and take the run it holds. With no lastRun.txt — which
// is the ordinary case, since only gatling-maven-plugin writes one and its
// verify goal deletes it — the most recently modified run wins, and ties break
// on the directory name, which for a Gatling run id is run-start order.
func ExampleFindRun_resultsRoot() {
	root, err := os.MkdirTemp("", "gatling-results")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(root) //nolint:errcheck // temporary directory

	same := time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC)

	for _, name := range []string{
		"corpussimulation-20260906044741110",
		"corpussimulation-20260906044814356",
	} {
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0o750); err != nil {
			panic(err)
		}

		if err := os.WriteFile(filepath.Join(dir, "simulation.log"), nil, 0o600); err != nil {
			panic(err)
		}

		if err := os.Chtimes(filepath.Join(dir, "simulation.log"), same, same); err != nil {
			panic(err)
		}
	}

	loc, err := gatling.FindRun(root)
	if err != nil {
		panic(err)
	}

	fmt.Println("chosen by:", loc.Found)
	fmt.Println("run:", filepath.Base(loc.Dir))
	// Output:
	// chosen by: newest
	// run: corpussimulation-20260906044814356
}
