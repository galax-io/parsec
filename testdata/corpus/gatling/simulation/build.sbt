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
    libraryDependencies ++= gatling,
    scalacOptions ++= Seq("-encoding", "UTF-8", "-deprecation", "-feature", "-unchecked"),
  )
