package io.galaxio.parsec.corpus

import io.gatling.core.Predef._
import io.gatling.http.Predef._
import org.galaxio.gatling.assertions.opennfr.OpenNfrAssertions

import scala.concurrent.duration._

/** The one simulation the golden corpus is recorded from.
  *
  * In a single run it produces every record kind the text simulation.log has, and every case the
  * decoder spec needs to prove: declared assertions (written ahead of the run header), a request
  * outside any group, groups nested two deep, a group name containing a comma, a request that fails a
  * check, requests that fail with a connection-level exception, and a request whose build fails —
  * the one case that produces an ERROR record. It runs
  * unchanged under Gatling 3.11.5 and 3.12.0; the version is chosen by the build, not here.
  */
class CorpusSimulation extends Simulation {

  private val baseUrl = sys.props.getOrElse("baseUrl", "http://localhost:8089")

  private val httpProtocol = http
    .baseUrl(baseUrl)
    .acceptHeader("application/json")
    .disableFollowRedirect

  private val ok           = http("GET /ok").get("/ok").check(status.is(200))
  private val slow         = http("GET /slow").get("/slow").check(status.is(200))
  private val failingCheck = http("GET /fail").get("/fail").check(status.is(200))
  // Absolute URLs bypass baseUrl: a closed port and an unresolvable host each crash the request,
  // which is the path that writes an ERROR record with the raw exception text.
  private val connectRefused = http("connect refused").get("http://127.0.0.1:1/")
  private val unknownHost    = http("unknown host").get("http://nonexistent.invalid/")
  // A request whose URL cannot be built, because the session attribute is never set, never
  // reaches the wire. Gatling records that as an ERROR line — a crash — rather than as a KO
  // request; a connection failure only yields a KO. This is what puts an ERROR record in the log.
  private val buildCrash = http("unresolvable url").get("/ok/#{undefinedAttribute}")

  private val scn = scenario("Corpus recording")
    .exec(ok)
    .pause(100.millis)
    .group("outer") {
      exec(ok)
        .group("inner, with comma") {
          exec(slow).exec(failingCheck)
        }
    }
    .pause(100.millis)
    .exec(connectRefused)
    .exec(unknownHost)
    .exec(buildCrash)

  setUp(
    scn.inject(atOnceUsers(2), rampUsers(4).during(2.seconds)),
  ).protocols(httpProtocol)
    // What this run must produce is stated once, in src/test/resources/nfr.yaml,
    // and rendered into Gatling assertions from there. Changing an expectation
    // is an edit to that file and to nothing else — no number lives here.
    //
    // The document names no tool, so the same expectations can be held against
    // a JMeter or k6 run of the same probe when those adapters land. A
    // requirement it cannot render refuses the whole document loudly rather
    // than quietly checking fewer things than the document states.
    .assertions(OpenNfrAssertions.fromYaml("src/test/resources/nfr.yaml"))
}
