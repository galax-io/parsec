import Dependencies._

enablePlugins(GatlingPlugin)

lazy val root = (project in file("."))
  .settings(
    inThisBuild(
      List(
        organization := "io.galaxio.parsec.corpus",
        scalaVersion := "2.13.18",
        version      := "0.1.0",
      ),
    ),
    name := "corpus",
    libraryDependencies ++= gatling ++ picatinny,
    scalacOptions ++= Seq("-encoding", "UTF-8", "-deprecation", "-feature", "-unchecked"),
    // The simulation is one file for every version. Only how it states its expectations differs,
    // and that is a whole source directory rather than a conditional import: picatinny is simply
    // absent from the classpath from 3.13.0 on, so a file mentioning it could not compile there.
    Test / unmanagedSourceDirectories += {
      val flavour = if (picatinnySupported) "scala-opennfr" else "scala-plain"
      (Test / sourceDirectory).value / flavour
    },
  )
