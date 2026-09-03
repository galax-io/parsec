package io.galaxio.parsec.corpus

import io.gatling.core.Predef._
import io.gatling.http.Predef._

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
    .assertions(
      // Exact on purpose: six users, each making three requests that must succeed and
      // three that must fail. A broken environment — the stub down, an "invalid" domain
      // that resolves — then fails this run itself, instead of producing a log that is
      // merely consistent with its own report.
      global.successfulRequests.count.is(18),
      global.failedRequests.count.is(18),
      details("GET /ok").successfulRequests.percent.is(100),
      details("outer" / "GET /ok").successfulRequests.percent.is(100),
      // The group is declared with a comma, but Gatling writes the comma as a space and
      // builds its statistics from what it wrote, so an assertion path must use the name
      // as recorded — two spaces where the comma was.
      details("outer" / "inner  with comma" / "GET /slow").successfulRequests.percent.is(100),
      details("outer" / "inner  with comma" / "GET /fail").failedRequests.percent.is(100),
      details("connect refused").failedRequests.percent.is(100),
      details("unknown host").failedRequests.percent.is(100),
      global.responseTime.max.lt(60000),
    )
}
