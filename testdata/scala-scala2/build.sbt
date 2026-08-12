ThisBuild / version := "0.1.0"
ThisBuild / scalaVersion := "2.13.14"

lazy val root = (project in file("."))
  .settings(
    name := "my-akka-app"
  )
