import sbt._

object Dependencies {
  // One project, two recordings: the Gatling version comes from a system property so the
  // simulation runs unchanged under 3.11.5 and 3.12.0. Example: sbt -Dgatling.version=3.12.0 ...
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
  // Gatling is a Provided dependency there, so this one version serves both
  // Gatling versions the probe runs under; verified against 3.11.5 and 3.12.0
  // in specs/003-canonical-model/research.md R1.
  val picatinnyVersion: String = "1.27.0"

  lazy val picatinny: Seq[ModuleID] = Seq(
    "org.galaxio" %% "gatling-picatinny" % picatinnyVersion % Test,
  )
}
