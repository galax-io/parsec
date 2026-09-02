import sbt._

object Dependencies {
  // One project, two recordings: the Gatling version comes from a system property so the
  // simulation runs unchanged under 3.11.5 and 3.12.0. Example: sbt -Dgatling.version=3.12.0 ...
  val gatlingVersion: String = sys.props.getOrElse("gatling.version", "3.11.5")

  lazy val gatling: Seq[ModuleID] = Seq(
    "io.gatling.highcharts" % "gatling-charts-highcharts",
    "io.gatling"            % "gatling-test-framework",
  ).map(_ % gatlingVersion % Test)
}
