package config

import "github.com/neural-chilli/fkn/internal/ordered"

func GroupNamesForTask(groups map[string]Group, taskName string) []string {
	names := []string{}
	for _, groupName := range ordered.Keys(groups) {
		for _, member := range groups[groupName].Tasks {
			if member == taskName {
				names = append(names, groupName)
				break
			}
		}
	}
	return names
}
