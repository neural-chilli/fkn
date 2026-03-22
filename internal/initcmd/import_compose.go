package initcmd

func findComposeTasks(repoRoot string) []inferredTask {
	composeFile := findComposeFile(repoRoot)
	if composeFile == "" {
		return nil
	}

	agent := false
	return []inferredTask{
		{Name: "compose-up", Desc: "Start the Docker Compose services", Cmd: "docker compose up -d", Agent: &agent, Safety: "external"},
		{Name: "compose-down", Desc: "Stop the Docker Compose services", Cmd: "docker compose down", Agent: &agent, Safety: "external"},
		{Name: "compose-logs", Desc: "Stream Docker Compose service logs", Cmd: "docker compose logs -f", Agent: &agent, Safety: "external"},
	}
}

func composeWatchPaths(repoRoot string) []string {
	composeFile := findComposeFile(repoRoot)
	if composeFile == "" {
		return nil
	}
	return []string{composeFile}
}

func findComposeFile(repoRoot string) string {
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		if hasFile(repoRoot, name) {
			return name
		}
	}
	return ""
}
