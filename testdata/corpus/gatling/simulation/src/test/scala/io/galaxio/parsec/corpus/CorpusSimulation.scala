package io.galaxio.parsec.corpus

import io.gatling.core.Predef._
import io.gatling.http.Predef._

import scala.concurrent.duration._

/** The one simulation the golden corpus is recorded from.
  *
  * In a single run it produces every record kind a simulation.log has, and every case the decoder specs need to prove: declared
  * assertions (written ahead of the run header), a request outside any group, groups nested two deep, a group name containing a
  * comma, a request that fails a check, requests that fail with a connection-level exception, and a request whose build fails —
  * the one case that produces an ERROR record.
  *
  * Three of those cases exist for the binary format specifically, and none of them can be added to a run after it is made
  * (specs/005-gatling-binary-decoder/research.md R8):
  *
  *   - a request named in Cyrillic, so at least one string in the log is stored as UTF-16 rather than Latin-1. Without it the
  *     corpus proves nothing about the half of the format that turns a wrong decoder into plausible mojibake instead of an
  *     error.
  *   - a name repeated far more often than it is introduced, so the writer's string cache is exercised as a table and not as a
  *     list.
  *   - a group whose name repeats across users and across those repeats, so a cached string appears inside a group path and not
  *     only in a request name.
  *
  * It runs unchanged under every Gatling version the corpus covers; the version is chosen by the build, not here. How it states
  * its expectations does depend on the version — see [[CorpusAssertions]] — but what it does does not, so runs stay comparable
  * across versions.
  */
class CorpusSimulation extends Simulation {

  private val baseUrl = sys.props.getOrElse("baseUrl", "http://localhost:8089")

  private val httpProtocol = http
    .baseUrl(baseUrl)
    .acceptHeader("application/json")
    .disableFollowRedirect

  private val ok             = http("GET /ok").get("/ok").check(status.is(200))
  private val slow           = http("GET /slow").get("/slow").check(status.is(200))
  // Cyrillic, so the name cannot be encoded in Latin-1 and the writer stores it as UTF-16 with the
  // other coder byte. The request itself is an ordinary success: what is being recorded is the name.
  private val cyrillic       = http("Проверка /ok").get("/ok").check(status.is(200))
  private val failingCheck   = http("GET /fail").get("/fail").check(status.is(200))
  // Absolute URLs bypass baseUrl: a closed port and an unresolvable host each crash the request,
  // which is the path that writes an ERROR record with the raw exception text.
  private val connectRefused = http("connect refused").get("http://127.0.0.1:1/")
  private val unknownHost    = http("unknown host").get("http://nonexistent.invalid/")
  // A request whose URL cannot be built, because the session attribute is never set, never
  // reaches the wire. Gatling records that as an ERROR line — a crash — rather than as a KO
  // request; a connection failure only yields a KO. This is what puts an ERROR record in the log.
  private val buildCrash     = http("unresolvable url").get("/ok/#{undefinedAttribute}")

  /** How many times "GET /ok" is repeated inside the outer group, per user.
    *
    * The point is the ratio, not the number: the name is written into the log once and referred to by index every other time,
    * so a decoder that rebuilds the cache wrongly renames the majority of the log rather than one record. Ten keeps the
    * recording small enough to read by hand.
    */
  private val repeats = 10

  private val scn = scenario("Corpus recording")
    .exec(ok)
    .exec(cyrillic)
    .pause(100.millis)
    .group("outer") {
      exec(ok)
        .repeat(repeats) {
          exec(ok)
        }
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
    // What this run must produce is stated in one place per Gatling version, and never in this file.
    // Up to 3.13.x that place is src/test/resources/nfr.yaml, an OpenNFR document rendered into
    // assertions; above it, where gatling-picatinny has no release, it is the plain Gatling DSL in
    // src/test/scala-plain. Either way no number lives here.
    .assertions(CorpusAssertions.all)
}
