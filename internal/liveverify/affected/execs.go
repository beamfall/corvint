package affected

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// BinaryExecsPath is the repository's declaration of the commands each
// declared Go package's code or tests run as a built binary when no literal in
// the package names the command's directory (AFP-V0-037).
const BinaryExecsPath = ".corvint/test-binary-execs.json"

// BinaryConsumersPath is the repository's declaration of the files, other than
// Go packages, that run a command's built binary (AFP-V0-762).
const BinaryConsumersPath = ".corvint/test-binary-consumers.json"

// UnknownE2EUnmappedChange names a changed path that reaches the commands a
// flow-bound declared consumer runs while no reviewed flow link explains it,
// so the consumer runs and the plan is UNKNOWN (AFU-V1-1652). Detail is the
// path.
const UnknownE2EUnmappedChange = "E2E_UNMAPPED_CHANGE"

// UnknownE2EFlowsInvalid names a flow binding declaration that is present but
// unreadable, invalid, or binds a flow or consumer that does not exist; no
// consumer is narrowed by it (AFU-V1-1651). Detail is the declaration path.
const UnknownE2EFlowsInvalid = "E2E_FLOWS_INVALID"

// E2EOmission is one flow-bound declared consumer a plan leaves out
// (AFU-V1-1653): the change reaches the commands it runs, no link of its
// bound flows is reached, and a reviewed link of some flow explains every
// changed path that reaches those commands.
type E2EOmission struct {
	File     string       `json:"file"`
	Flows    []string     `json:"flows"`
	Commands []string     `json:"commands"`
	Basis    string       `json:"basis"`
	Mapped   []E2EMapping `json:"mapped"`
}

// E2EMapping explains one changed path by the reviewed link target it
// reaches and that link's flow.
type E2EMapping struct {
	Path   string `json:"path"`
	Flow   string `json:"flow"`
	Target string `json:"target"`
}

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
// (AFP-V0-037). The runner is not traversed from here: a unit whose code may
// run a command the dependency traversal reaches is already reached and
// walked on by it (AFP-V0-2993), so what this adds is a unit whose tests
// alone run the binary, whose importers compile none of those tests, and a
// runner of a command only an enclosed path reaches, whose own importers the
// plan does not walk either. Commands are visited by witness length, then id,
// so each runner takes the shortest chain.
func (graph *Graph) execUsersOf(reached, built map[string]Witness) {
	commands := make([]string, 0, len(built))
	for command := range built {
		commands = append(commands, command)
	}
	sort.Slice(commands, func(i, j int) bool {
		left, right := built[commands[i]].ViaLength(), built[commands[j]].ViaLength()
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
				link:      parent.extended(user),
			}
		}
	}
}

// unselectedConsumers names, once each in path order, every declared consumer
// file (an ExecConsumers entry, AFP-V0-762) of a Go command whose build the change
// reaches, by a direct source change, a dependency path or an enclosed path,
// that no selection carries as a test: a Node script that runs the binary is
// no unit's test, so the plan states it rather than leaving the run out
// silently. A command reached only for its own test files, or as a reader,
// was not rebuilt and names no consumer. The hint names the nearest command.
func (graph *Graph) unselectedConsumers(reached, rebuilt map[string]Witness, selected []Selection) []Unknown {
	tests := map[string]bool{}
	for _, selection := range selected {
		for _, test := range selection.Tests {
			tests[test] = true
		}
	}
	commands := map[string]string{}
	for _, id := range graph.order {
		witness, hit := reached[id]
		_, built := rebuilt[id]
		if !hit || !strings.HasPrefix(id, "go:") || !built && !rebuilds(witness.Kind) {
			continue
		}
		for _, file := range graph.units[id].ExecConsumers {
			if previous := commands[file]; !tests[file] && (previous == "" || witness.ViaLength() < reached[previous].ViaLength()) {
				commands[file] = id
			}
		}
	}
	unknown := make([]Unknown, 0, len(commands))
	for _, file := range sortedKeysOf(commands) {
		unknown = append(unknown, Unknown{
			Reason: UnknownUnselectedBinaryConsumer,
			Detail: file,
			Hint:   "declared in " + BinaryConsumersPath + " as running the built binary of " + commands[file] + ", whose build this change reaches; no selected unit runs it as a test, so run it by its own driver",
		})
	}
	return unknown
}

// rebuilds reports whether a witness kind means the unit's build changed.
func rebuilds(kind string) bool {
	return kind == WitnessDirectSource || kind == WitnessDependency || kind == WitnessEnclosingPackage
}

func sortedKeysOf(set map[string]string) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// ValidConsumerRun bounds project-owned argv without interpreting it as shell text.
func ValidConsumerRun(run []string) bool {
	if len(run) == 0 || len(run) > 256 || run[0] == "" {
		return false
	}
	bytes := 0
	for _, arg := range run {
		bytes += len(arg)
		if !utf8.ValidString(arg) || strings.ContainsAny(arg, "\x00\r\n") {
			return false
		}
	}
	return bytes <= 65536
}

// selectDeclaredConsumers adds one runnable selection per declared consumer,
// even if another plugin has already selected that file. The declaration's
// runner is the authority for executing the consumed binary (AZU-V0-007).
func (graph *Graph) selectDeclaredConsumers(rebuilt map[string]Witness, selected []Selection) []Selection {
	commands := map[string]string{}
	for _, id := range graph.order {
		witness, hit := rebuilt[id]
		if !hit || !strings.HasPrefix(id, "go:") {
			continue
		}
		for file := range graph.units[id].ExecConsumerRuns {
			previous := commands[file]
			if previous == "" || witness.ViaLength() < rebuilt[previous].ViaLength() {
				commands[file] = id
			}
		}
	}
	for _, file := range sortedKeysOf(commands) {
		id := commands[file]
		parent := rebuilt[id]
		consumer := "declared-consumer:" + file
		selected = append(selected, Selection{UnitID: consumer, Tests: []string{file}, Kind: "DECLARED_CONSUMER", Run: append([]string(nil), graph.units[id].ExecConsumerRuns[file]...), Witness: Witness{Kind: "DECLARED_CONSUMER", DirtyPath: parent.DirtyPath, link: parent.extended(consumer)}})
	}
	return selected
}
