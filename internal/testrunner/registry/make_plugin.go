// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"fmt"
	"regexp"
	"sort"

	"github.com/Beamfall/corvint/internal/sessiontest"
	"github.com/Beamfall/corvint/internal/testrunner"
)

// MakePlugins contains only targets explicitly declared by the tracked CI
// command manifest. It never reads or interprets a Makefile (decision 0526).
// Each target is one instance of T13's script kind, not another runner kind.
type MakePlugins struct{ scripts *ScriptPlugins }

// ReadMakePlugins uses the same immutable, bounded declaration reader as T13.
// A declaration needing make options, variable assignments or multiple goals
// cannot be split safely into independent targets; it refuses with a reason.
func ReadMakePlugins(ctx context.Context, root, checkout string) (*MakePlugins, error) {
	scripts, err := ReadScriptPlugins(ctx, root, checkout, sessiontest.DeclarationPath)
	if err != nil {
		return nil, err
	}
	return makePlugins(scripts)
}

var literalMakeTarget = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)

func makePlugins(scripts *ScriptPlugins) (*MakePlugins, error) {
	if scripts == nil || scripts.source != sessiontest.DeclarationPath {
		return nil, fmt.Errorf("make-declaration-unavailable")
	}
	commands := []sessiontest.LanguageCommand{}
	seen := map[string]bool{}
	for _, command := range scripts.commands {
		if len(command.Argv) == 0 || command.Argv[0] != "make" {
			continue
		}
		if len(command.Argv) != 2 || !literalMakeTarget.MatchString(command.Argv[1]) {
			return nil, fmt.Errorf("make-command-unsupported:%s", command.Language)
		}
		target := command.Argv[1]
		if seen[target] {
			continue
		}
		seen[target] = true
		commands = append(commands, sessiontest.LanguageCommand{Language: "make:" + target, Argv: append([]string(nil), command.Argv...)})
	}
	if len(commands) == 0 {
		return nil, fmt.Errorf("make-targets-undeclared")
	}
	sort.Slice(commands, func(i, j int) bool { return commands[i].Language < commands[j].Language })
	return &MakePlugins{scripts: &ScriptPlugins{checkout: scripts.checkout, source: scripts.source, digest: scripts.digest, commands: commands}}, nil
}
func (p *MakePlugins) IDs() []string {
	if p == nil {
		return nil
	}
	return p.scripts.IDs()
}
func (p *MakePlugins) Plugin(id, os, arch string) (RunnerPlugin, testrunner.InventoryKey) {
	var scripts *ScriptPlugins
	if p != nil {
		scripts = p.scripts
	}
	plugin, key := scripts.Plugin(id, os, arch)
	if _, ok := plugin.(*scriptPlugin); ok {
		return makePlugin{plugin}, key
	}
	return plugin, key
}

// The command, reconciliation and evidence implementation are T13's exactly.
// RSP-V0-001 / RSP-V0-040: even claimed qualification cannot enable this slice.
type makePlugin struct{ RunnerPlugin }

func (p makePlugin) Pieces() testrunner.RunnerPieces { return shellPieces() }
