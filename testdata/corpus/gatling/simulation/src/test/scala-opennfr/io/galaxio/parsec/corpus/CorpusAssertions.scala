package io.galaxio.parsec.corpus

import io.gatling.core.Predef._
import org.galaxio.gatling.assertions.opennfr.OpenNfrAssertions

/** What the run must produce, rendered from the probe's OpenNFR document.
  *
  * This is the flavour used up to Gatling 3.12.x, the last line gatling-picatinny can record on. Everything
  * the run is held to lives in src/test/resources/nfr.yaml and nothing else: no number is written
  * twice and no Scala file carries one. The document names no tool, so the same expectations can be
  * held against a JMeter or k6 run of the same probe when those adapters land.
  *
  * A requirement the renderer cannot express refuses the whole document, naming every reason, so a
  * run can never quietly check fewer things than the document states.
  *
  * From 3.13.0 picatinny is absent from the classpath and src/test/scala-plain is compiled instead
  * — the same expectations, written in Gatling's own DSL. build.sbt chooses between the two.
  */
object CorpusAssertions {

  /** Evaluated inside the running simulation: Gatling's implicit configuration does not exist until
    * it has instantiated one, so this cannot be a `val`.
    */
  def all: Iterable[Assertion] = OpenNfrAssertions.fromYaml("src/test/resources/nfr.yaml")
}
