package text_test

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/text"
)

func ExampleNewReader() {
	log := "ASSERTION\tAAEBAAICAAAAAAAAAPA/\n" +
		"RUN\tcom.example.PetStore\tpetstore\t1788379354534\t \t3.11.5\n" +
		"USER\tBrowse\tSTART\t1788379356165\n" +
		"REQUEST\t\tGET /pets\t1788379356162\t1788379356173\tOK\t \n" +
		"REQUEST\tcheckout\tPOST /orders\t1788379356180\t1788379356201\tKO\tstatus.find.is(200), but actually found 500\n" +
		"USER\tBrowse\tEND\t1788379356202\n"

	r, err := text.NewReader(strings.NewReader(log))
	if err != nil {
		fmt.Println("open:", err)

		return
	}

	fmt.Println(r.Header().SimulationClass, "under Gatling", r.Header().Version)

	requests := map[gatling.Status]int{}

	for {
		rec, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			fmt.Println("read:", err)

			return
		}

		if rec.Kind == gatling.KindRequest {
			requests[rec.Status]++
		}
	}

	fmt.Println("requests: ok", requests[gatling.StatusOK], "ko", requests[gatling.StatusKO])
	// Output:
	// com.example.PetStore under Gatling 3.11.5
	// requests: ok 1 ko 1
}

func ExampleNewReader_damagedLog() {
	log := "RUN\tcom.example.PetStore\tpetstore\t1788379354534\t \t3.11.5\n" +
		"USER\tBrowse\tSTART\t1788379356165\n" +
		"REQUEST\t\tGET /pets\t1788379356162\n"

	r, err := text.NewReader(strings.NewReader(log))
	if err != nil {
		fmt.Println("open:", err)

		return
	}

	for {
		if _, err := r.Next(); err != nil {
			fmt.Println(err)

			break
		}
	}
	// Output:
	// gatling: line 3: expected REQUEST with 7 fields, found 4 fields
}

func ExampleSupportedVersions() {
	oldest, newest := text.SupportedVersions()
	fmt.Println(oldest, "through", newest)
	// Output:
	// 3.11.5 through 3.12.0
}
