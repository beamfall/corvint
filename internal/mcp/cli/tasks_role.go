package cli

import (
	"slices"

	"github.com/Beamfall/corvint/internal/mcp/tasksbridge"
)

// toolProfileTasks is the tasks tool group (TAW-V0-015): the corvint-tasks
// work verbs, run in process against the store of each declared root.
const toolProfileTasks = "tasks"

// The refusal reasons of --tasks-role (TAW-V0-015).
const (
	tasksRoleReasonRepeated = "--tasks-role must be given once with a value"
	tasksRoleReasonValue    = "--tasks-role must be AGENT, OPERATOR or OWNER"
	tasksRoleReasonUnpaired = "--tasks-role requires --tool-profile with the tasks group"
)

// extractTasksRole removes the optional --tasks-role CLASS: given at most
// once, with one of the closed class values.
func extractTasksRole(arguments []string) (remaining []string, class, reason string) {
	remaining = make([]string, 0, len(arguments))
	for index := 0; index < len(arguments); index++ {
		if arguments[index] != "--tasks-role" {
			remaining = append(remaining, arguments[index])
			continue
		}
		if class != "" || index+1 == len(arguments) || arguments[index+1] == "" {
			return nil, "", tasksRoleReasonRepeated
		}
		class = arguments[index+1]
		index++
	}
	if class != "" && !slices.Contains(tasksbridge.Classes, class) {
		return nil, "", tasksRoleReasonValue
	}
	return remaining, class, ""
}

// bindTasksRole pairs --tasks-role with the tasks group; the class defaults
// to AGENT, the least privileged one.
func bindTasksRole(groups []string, class string) (string, string) {
	if !slices.Contains(groups, toolProfileTasks) {
		if class != "" {
			return "", tasksRoleReasonUnpaired
		}
		return "", ""
	}
	if class == "" {
		class = tasksbridge.Classes[0]
	}
	return class, ""
}
