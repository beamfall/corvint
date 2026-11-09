package main

// Helpers kept from the retired prove and witness command files (decision
// 0594): the affected, test, docs and query paths still share them.

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// kindAffectedTest names the row kind of one selected test (FPK-V0-017).
const kindAffectedTest = "affected-test"

// commandPositionAfterRoots returns the index of the first argument after any
// leading --root pairs, or -1 when only roots were given.
func commandPositionAfterRoots(arguments []string) int {
	index := 0
	for index < len(arguments) && (arguments[index] == "--root" && rootPreambleValue(arguments, index+1) || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			index += 2
		} else {
			index++
		}
	}
	if index >= len(arguments) {
		return -1
	}
	return index
}

func readBoundedFile(path string, bound int) ([]byte, error) {
	file, err := openBoundedRegularFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, int64(bound)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > bound {
		return nil, errors.New("map exceeds bound")
	}
	return data, nil
}

// hermeticGitCommand is the one Git read shape in cmd/corvint (FPK-V0-052):
// the -c set of internal/contextindex gitRaw plus core.hooksPath, so no
// repository-configured program runs; replace refs ignored, so cited bytes are
// the named objects. Discovery stops at root's parent only when root holds its own
// .git entry; a subdirectory root still reaches its enclosing worktree.
func hermeticGitCommand(ctx context.Context, gitExecutable, root string, arguments ...string) *exec.Cmd {
	prefix := []string{"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false",
		"-c", "core.excludesFile=", "-c", "credential.helper=", "-c", "submodule.recurse=false",
		"-c", "core.hooksPath=/dev/null", "-C", root}
	command := exec.CommandContext(ctx, gitExecutable, append(prefix, arguments...)...)
	command.Env = []string{
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1", "GIT_ALLOW_PROTOCOL=", "LANG=C", "LC_ALL=C",
	}
	if _, err := os.Lstat(filepath.Join(root, ".git")); err == nil {
		command.Env = append(command.Env, "GIT_CEILING_DIRECTORIES="+filepath.Dir(root))
	}
	return command
}

func mapsFromAny(value any) []map[string]any {
	items, _ := value.([]any)
	maps := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if entry, ok := item.(map[string]any); ok {
			maps = append(maps, entry)
		}
	}
	return maps
}

func stringAt(entry map[string]any, key string) string {
	value, _ := entry[key].(string)
	return value
}

// splitWitnessFlag separates `--name=value` from `--name value`. It reports
// whether the value was inline so a valueless flag can refuse one.
func splitWitnessFlag(argument string) (string, string, bool) {
	if position := strings.Index(argument, "="); position > 0 && strings.HasPrefix(argument, "--") {
		return argument[:position], argument[position+1:], true
	}
	return argument, "", false
}
