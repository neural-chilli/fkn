package initcmd

import "path/filepath"

func findJavaTasks(repoRoot string) []inferredTask {
	if hasFile(repoRoot, "pom.xml") {
		return []inferredTask{
			{Name: "test", Desc: "Run the Maven test suite", Cmd: "mvn test", Safety: "idempotent"},
			{Name: "build", Desc: "Build the Maven project", Cmd: "mvn package", Safety: "idempotent"},
		}
	}

	if hasFile(repoRoot, "build.gradle") || hasFile(repoRoot, "build.gradle.kts") {
		gradleCmd := "./gradlew"
		if !hasFile(repoRoot, "gradlew") {
			gradleCmd = "gradle"
		}
		return []inferredTask{
			{Name: "test", Desc: "Run the Gradle test suite", Cmd: gradleCmd + " test", Safety: "idempotent"},
			{Name: "build", Desc: "Build the Gradle project", Cmd: gradleCmd + " build", Safety: "idempotent"},
		}
	}

	return nil
}

func javaWatchPaths(repoRoot string) []string {
	paths := []string{}
	for _, name := range []string{"pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts", "gradlew", "gradlew.bat"} {
		if hasFile(repoRoot, name) {
			paths = append(paths, name)
		}
	}
	for _, dir := range []string{"src", "app"} {
		if hasDir(repoRoot, dir) {
			paths = append(paths, filepath.ToSlash(dir)+"/")
		}
	}
	return paths
}
