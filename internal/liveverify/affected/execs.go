package affected

import "sort"

// BinaryExecsPath is the repository's declaration of the commands each
// declared Go package's code or tests run as a built binary when no literal in
// the package names the command's directory (AFP-V0-037).
const BinaryExecsPath = ".corvint/test-binary-execs.json"

// WitnessBinaryExec names a unit that runs the built binary of a command
// whose build a dirty path reaches (AFP-V0-037).
const WitnessBinaryExec = "BINARY_EXEC"

// builtCommands returns, with its witness, every command some unit runs whose
// build a dirty path reaches: by a dependency path (reached before test users
// are added, since a binary never compiles a test) or by an enclosed path,
// which may be embedded into it (AFP-V0-037).
func (graph *Graph) builtCommands(reached, enclosing map[string]Witness) map[string]Witness {
	built := make(map[string]Witness)
	for command := range graph.execUsers {
		if witness, hit := reached[command]; hit {
			built[command] = witness
		} else if witness, hit := enclosing[command]; hit {
			built[command] = witness
		}
	}
	return built
}

// execUsersOf selects each unit not already reached that runs the built
// binary of a command in built, one edge past it with a BINARY_EXEC witness
// (AFP-V0-037). Like a reader the runner is not traversed. Commands are
// visited by witness length, then id, so each runner takes the shortest chain.
func (graph *Graph) execUsersOf(reached, built map[string]Witness) {
	commands := make([]string, 0, len(built))
	for command := range built {
		commands = append(commands, command)
	}
	sort.Slice(commands, func(i, j int) bool {
		left, right := len(built[commands[i]].Via), len(built[commands[j]].Via)
		if left != right {
			return left < right
		}
		return commands[i] < commands[j]
	})
	for _, command := range commands {
		parent := built[command]
		for _, user := range graph.execUsers[command] {
			if _, seen := reached[user]; seen {
				continue
			}
			reached[user] = Witness{
				Kind:      WitnessBinaryExec,
				DirtyPath: parent.DirtyPath,
				Via:       append(append([]string(nil), parent.Via...), user),
			}
		}
	}
}
