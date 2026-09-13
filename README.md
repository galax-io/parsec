# parsec

[![CI](https://github.com/galax-io/parsec/actions/workflows/ci.yml/badge.svg)](https://github.com/galax-io/parsec/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/galax-io/parsec)](https://github.com/galax-io/parsec/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/galax-io/parsec)](https://goreportcard.com/report/github.com/galax-io/parsec)
[![Go Reference](https://pkg.go.dev/badge/github.com/galax-io/parsec.svg)](https://pkg.go.dev/github.com/galax-io/parsec)
[![License](https://img.shields.io/github/license/galax-io/parsec)](https://github.com/galax-io/parsec/blob/main/LICENSE)

Load-test result primitives for Go: one canonical model for a load test's results, and a decoder per
tool that produces it. Gatling is implemented; JMeter, k6, Locust and Yandex.Tank follow.

**This library computes no statistic.** No count, no mean, no percentile, no range, no per-interval
series. It owns the part two implementations diverge on — the definitions: what counts as a failure,
what a request position is, where a run begins and ends — and the primitives a consumer computes
from. The arithmetic belongs to the consumer.

## Install

```bash
go get github.com/galax-io/parsec
```

Go 1.25 or newer.

## Read a run

Give it a results root and it finds the run, opens the log whatever Gatling wrote it, and yields the
canonical items:

```go
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/simlog"
	"github.com/galax-io/parsec/model"
)

func main() {
	f, err := os.Open("target/gatling/mysimulation-20260906044741110/simulation.log")
	if err != nil {
		panic(err)
	}

	defer f.Close()

	rd, err := simlog.NewRunReader(f)
	if err != nil {
		// A log of either format lands here read, not refused: the format is
		// identified from the file's leading bytes.
		panic(err)
	}

	run := rd.Run()
	fmt.Println("tool:", run.Tool, run.ToolVersion)
	fmt.Println("warnings:", len(run.Warnings))

	samples := 0

	for {
		item, err := rd.Next()
		if err != nil {
			// io.EOF is the clean end of the log. A *gatling.TruncationError
			// says the log was cut short — a run killed mid-flight — and the
			// items already delivered are the ones it recorded; whether to use
			// a run that did not finish is the caller's decision. Any other
			// error means the read failed, and nothing delivered may be totalled.
			var cutShort *gatling.TruncationError
			if !errors.Is(err, io.EOF) && !errors.As(err, &cutShort) {
				panic(err)
			}

			break
		}

		if item.Kind == model.ItemSample {
			samples++
		}
	}

	fmt.Println("samples:", samples)
}
```

This is the body of [`gatling/simlog/example_test.go`](gatling/simlog/example_test.go), which
`go test` compiles and checks the output of, so it cannot rot.

Don't know which directory the run is in? `run.Find` answers that first:

```go
loc, err := run.Find(run.DefaultResultsRoot) // "target/gatling", where Maven and sbt both write
```

It returns where the run's artefacts sit and which rule chose them. It opens no log and applies no
version gate, so it fails only about the thing it was asked.

The whole path is three calls: `run.Find`, then `simlog.NewRunReader`, then a fold over `Next`.
Nothing in it names a log format, and nothing in the items names a tool.

## What it reads

| Tool | Format | Versions accepted |
|---|---|---|
| Gatling | text `simulation.log` | 3.11.5 through 3.12.0 |
| Gatling | binary `simulation.log` | 3.13.1 through 3.15.1 |

**3.13.0 is refused**, although the codec could read it. That version writes the binary format and
cannot generate a report, so no run of it can carry the second account of its own numbers that a
golden corpus entry needs — and the accepted range follows the corpus rather than what the decoder
believes it could manage. A 3.13.0 log fails with a `*gatling.VersionError` naming the version found
and the range supported.

A version **below** the range is refused. A version **above** it decodes and records a warning, so a
newer Gatling is readable before anyone has recorded it; `gatling.WithStrict` refuses those instead,
for a caller that cannot use a number nothing has verified.

Take the range from the API rather than from this table — a hard-coded range goes stale the first
time one is recorded:

```go
for _, s := range simlog.Supported() {
	fmt.Printf("%s: %s through %s\n", s.Format, s.Oldest, s.Newest)
}
```

## Stability

From **v0.1.0** the exported surface is a contract. Every exported identifier of `model`, `gatling`,
`gatling/text`, `gatling/binary`, `gatling/simlog` and `gatling/run` is promised; nothing else is,
and `internal/` is not importable. Changing a promised signature or an observable behaviour is a
breaking change: it needs a `CHANGELOG.md` entry and a new MINOR release, and a superseded identifier
keeps working for at least one MINOR release behind a `// Deprecated:` comment.

The full promise, and what it covers, is in [CHANGELOG.md](CHANGELOG.md).

## Packages

- [`model`](model) — the canonical result types every source is decoded into, and the `Capabilities`
  a source declares about what it cannot provide. This is what a consumer builds on.
- [`gatling/run`](gatling/run) — which run to read. Runs before everything below it and depends on
  none of it.
- [`gatling/simlog`](gatling/simlog) — opens a `simulation.log` without being told which Gatling
  wrote it. **The entry point.**
- [`gatling`](gatling) — the wire records a Gatling `simulation.log` carries, the version type and
  the version gate every Gatling codec shares.
- [`gatling/text`](gatling/text), [`gatling/binary`](gatling/binary) — the two codecs. Reach for one
  directly only when the version is already known.

`model` and `gatling` depend on the standard library only, and CI checks it. No module is
pre-approved anywhere: `go.mod` naming no requirement is the intended steady state, not a property of
a young project.

## What it will not do

Compute statistics. Counts, means, percentiles, response-time ranges and per-interval series belong
to the consumer: [galaxio-cli#51](https://github.com/galax-io/galaxio-cli/issues/51) for a finished
run, [galaxio-cli#61](https://github.com/galax-io/galaxio-cli/issues/61) for its series, and `comet`
for the live case with its own arithmetic. Those are different computations over the same decoded
results, and one shared accumulator would have been shaped by neither.

## Contributing

The rules are in [AGENTS.md](AGENTS.md) and
[`.specify/memory/constitution.md`](.specify/memory/constitution.md). Security reports go through
[SECURITY.md](SECURITY.md).

Part of what keeps `main` green is repository configuration rather than a file a clone can show. It
is declared as files anyway, so what is applied can be diffed against what is intended:

- [`.github/ruleset-main.json`](.github/ruleset-main.json) — `main`: no deletion, no force-push,
  changes arrive by pull request, and the `verify` check must pass. No bypass.
- [`.github/ruleset-release.json`](.github/ruleset-release.json) — `release/*`: no deletion, no
  force-push. No required check: a patch is a cherry-pick pushed directly, and the gate for a release
  is the tag.
- [`.github/ruleset-tags.json`](.github/ruleset-tags.json) — `v*`: a release tag is never deleted or
  moved.

How to apply or re-apply them is in
[specs/001-ci-release-automation/quickstart.md](specs/001-ci-release-automation/quickstart.md), under
*Maintainer actions*.

## Licence

MIT.
