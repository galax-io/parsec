package io.galaxio.parsec.corpus

import io.gatling.core.Predef._

/** What the run must produce, in Gatling's own assertion DSL.
  *
  * This is the flavour used by the versions that write a binary simulation.log — from 3.13.0 on. The split is the log
  * format, not the tool version: an OpenNFR `loadtest.group.name` is the literal recorded name, and the two formats record
  * `inner, with comma` differently (see the note below), so src/test/resources/nfr.yaml cannot address the group in both.
  * gatling-picatinny is not the obstacle — it resolves and runs correctly on 3.13.1 — but it has no release above 3.13.x
  * and so could not have covered this range anyway.
  *
  * This file states exactly what nfr.yaml states for the versions that render it, and the two must be kept in step by
  * hand — that is the cost specs/005-gatling-binary-decoder/research.md R8 accepted, and it is why every assertion below
  * carries the name of the requirement it mirrors.
  *
  * The numbers are exact on purpose. Six virtual users each make fourteen requests that must succeed and three that must fail.
  * A broken environment — the stub down, an "invalid" domain that resolves — then fails this run itself, instead of producing a
  * log that is merely consistent with its own report. The corpus is recorded from this run, so a log that passed a vaguer bar
  * would be evidence of nothing.
  *
  * Two things that look wrong here are not:
  *
  *   - `inner, with comma` keeps its comma here, where src/test/resources/nfr.yaml spells the same group `inner  with
  *     comma` with two spaces. Neither is a typo and they are not drift: the two formats record the name differently.
  *     A text simulation.log separates a group path with commas, so Gatling replaces the comma in the name with a space
  *     before writing it, and resolves assertion paths from what it wrote. The binary format length-prefixes each group
  *     name, so nothing has to be substituted and the declared name survives intact. The group keeps its comma
  *     deliberately: in the text corpus it is what proves the decoder's split on comma is lossless, and here it is what
  *     proves the binary format loses nothing to begin with.
  *   - The successes are stated as a total and a failure count rather than as a success count. It is the same statement about
  *     the same run, and it is how nfr.yaml has to say it, so the two flavours stay comparable line for line.
  */
object CorpusAssertions {

  /** Evaluated inside the running simulation: Gatling's implicit configuration does not exist until it has instantiated one, so
    * this cannot be a `val`.
    */
  def all: Iterable[Assertion] = List(
    // whole-run / recorded-requests
    global.allRequests.count.is(102),
    // whole-run / failed-requests
    global.failedRequests.count.is(18),
    // whole-run / slowest — a ceiling, not a performance target: it exists so a hung environment
    // fails the recording rather than producing a log with a plausible shape.
    global.responseTime.max.lt(60000),
    // root-ok
    details("GET /ok").failedRequests.percent.is(0),
    // root-cyrillic — the name that forces a UTF-16 string into the log
    details("Проверка /ok").failedRequests.percent.is(0),
    // outer-ok — the same request name inside a group is a different position
    details("outer" / "GET /ok").failedRequests.percent.is(0),
    // inner-slow
    details("outer" / "inner, with comma" / "GET /slow").failedRequests.percent.is(0),
    // inner-fail
    details("outer" / "inner, with comma" / "GET /fail").failedRequests.percent.is(100),
    // connect-refused
    details("connect refused").failedRequests.percent.is(100),
    // unknown-host
    details("unknown host").failedRequests.percent.is(100),
  )
}
