package simlog_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/galax-io/parsec/gatling/simlog"
	"github.com/galax-io/parsec/model"
)

// Open an archived run without knowing which Gatling wrote it.
func ExampleNewRunReader() {
	f, err := os.Open(filepath.Join("..", "..", "testdata", "corpus", "gatling", "3.11.5", "simulation.log"))
	if err != nil {
		panic(err)
	}

	defer f.Close() //nolint:errcheck // read-only

	rd, err := simlog.NewRunReader(f)
	if err != nil {
		// A binary log lands here today: the format is recognised and named,
		// rather than failing as a syntax error on line 1.
		panic(err)
	}

	run := rd.Run()
	fmt.Println("tool:", run.Tool, run.ToolVersion)
	fmt.Println("warnings:", len(run.Warnings))

	samples := 0

	for {
		item, err := rd.Next()
		if err != nil {
			// Only io.EOF is the end of the log. Any other error means the read
			// failed, and the records already delivered are not a result — no
			// total may be derived from them.
			if !errors.Is(err, io.EOF) {
				panic(err)
			}

			break
		}

		if item.Kind == model.ItemSample {
			samples++
		}
	}

	fmt.Println("samples:", samples)

	// Output:
	// tool: gatling 3.11.5
	// warnings: 0
	// samples: 36
}

// Report what this module reads, without hard-coding a version anywhere.
func ExampleSupported() {
	for _, s := range simlog.Supported() {
		if !s.Readable {
			fmt.Printf("%s: known, no codec yet\n", s.Format)

			continue
		}

		fmt.Printf("%s: %s through %s\n", s.Format, s.Oldest, s.Newest)
	}

	// Output:
	// text: 3.11.5 through 3.12.0
	// binary: 3.13.1 through 3.15.1
}
