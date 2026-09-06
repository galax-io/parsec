import sbt._

object Dependencies {
  // One project, one recording per version: the Gatling version comes from a system property so the
  // simulation runs unchanged under every version the corpus covers.
  // Example: sbt -Dgatling.version=3.15.1 ...
  val gatlingVersion: String = sys.props.getOrElse("gatling.version", "3.11.5")

  lazy val gatling: Seq[ModuleID] = Seq(
    "io.gatling.highcharts" % "gatling-charts-highcharts",
    "io.gatling"            % "gatling-test-framework",
  ).map(_ % gatlingVersion % Test)

  // Renders the probe's OpenNFR document into Gatling assertions. Pinned
  // exactly: the OpenNFR surface is documented upstream as experimental and
  // outside that library's binary-compatibility guarantee, and it tracks a
  // pre-1.0 format that has changed between consecutive releases.
  //
  // Gatling is a Provided dependency there, so this one version serves every
  // Gatling version it supports; verified against 3.11.5 and 3.12.0 in
  // specs/003-canonical-model/research.md R1.
  val picatinnyVersion: String = "1.27.0"

  /** Whether this Gatling line records its expectations through gatling-picatinny.
    *
    * The split is the **log format**, not the tool version and not picatinny's availability.
    *
    * An OpenNFR `loadtest.group.name` element is the literal *recorded* group name, and the two
    * Gatling log formats record one differently. A text simulation.log separates a group path with
    * commas, so Gatling replaces a comma inside a name with a space before writing it; the binary
    * format length-prefixes each name, so the declared name survives intact. The probe declares a
    * group called `inner, with comma` on purpose, so one document cannot address it in both formats
    * — src/test/resources/nfr.yaml must spell it one way or the other. The versions that write a
    * text log render their assertions from that document; the versions that write a binary one state
    * the same expectations in Gatling's own DSL, from src/test/scala-plain.
    *
    * picatinny is not the obstacle: 1.27.0 resolves and runs correctly on 3.13.1, verified directly.
    * It simply has no release above 3.13.x, so it could not have covered the binary range in any
    * case, and using it for part of that range would record one version through a different
    * mechanism than the rest (spec 005 FR-032 asks for runs that can be compared).
    *
    * A version this cannot parse is treated as writing a binary log: the probe then holds itself to
    * the same numbers through the plain DSL, which is the safe direction for anything newer than
    * what is known here.
    */
  val picatinnySupported: Boolean = {
    // The build definition is compiled with Scala 2.12, where String.toIntOption does not exist.
    def number(s: String): Option[Int] = scala.util.Try(s.toInt).toOption

    gatlingVersion.split("[.\\-]").toList match {
      case major :: minor :: _ => number(major).contains(3) && number(minor).exists(_ <= 12)
      case _                   => false
    }
  }

  lazy val picatinny: Seq[ModuleID] =
    if (picatinnySupported) Seq("org.galaxio" %% "gatling-picatinny" % picatinnyVersion % Test)
    else Seq.empty
}
