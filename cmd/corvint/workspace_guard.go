// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/workspace"
)

// workspaceRemedies names, per verb that reads one repository, how to cover the other declared
// repositories instead (WSD-V0-146).
var workspaceRemedies = map[string]string{
	"witness": "pass --range app=BASE..HEAD --range tests=BASE..HEAD with --range-root NAME=ABSOLUTE_ROOT for a two-repository witness",
	"review":  "review one repository per --root; cross-repository review is not delivered (V1-2910)",
	"cem":     "a Change Evidence Map cites one repository; run cem per --root; cross-repository citation is not delivered (V1-2910)",
}

// stripWorkspaceFlags removes --workspace FILE (or --workspace=FILE) and --no-workspace from args.
func stripWorkspaceFlags(args []string) (rest []string, file string, named, disabled bool, err error) {
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--no-workspace":
			if disabled {
				return nil, "", false, false, argumentError("--no-workspace may be given once")
			}
			disabled = true
		case a == "--workspace" || strings.HasPrefix(a, "--workspace="):
			value, ok := strings.CutPrefix(a, "--workspace=")
			if !ok {
				if i+1 >= len(args) {
					return nil, "", false, false, argumentError("--workspace requires FILE")
				}
				i++
				value = args[i]
			}
			if named || value == "" {
				return nil, "", false, false, argumentError("invalid --workspace")
			}
			file, named = value, true
		default:
			rest = append(rest, a)
		}
	}
	if named && disabled {
		return nil, "", false, false, argumentError("--workspace does not combine with --no-workspace")
	}
	return rest, file, named, disabled, nil
}

// guardWorkspace refuses a verb that reads one repository when a workspace is named, or, with
// discover, found under root (checkWorkspace). It returns args without the workspace flags.
func guardWorkspace(verb, root string, args []string, discover bool) ([]string, error) {
	rest, file, named, disabled, err := stripWorkspaceFlags(args)
	if err != nil {
		return nil, err
	}
	return rest, checkWorkspace(verb, root, file, named, disabled, discover)
}

// checkWorkspace refuses as workspace-unsupported, naming the verb, the aliases it would ignore
// and the remedy, when a workspace is named or, with discover and without --no-workspace, found
// under root; an invalid one refuses as workspace-invalid (WSD-V0-146).
func checkWorkspace(verb, root, file string, named, disabled, discover bool) error {
	if disabled || !named && !discover {
		return nil
	}
	var w *workspace.Workspace
	var err error
	if named {
		w, err = workspace.Load(file)
	} else {
		w, err = workspace.Discover(root)
	}
	if err != nil || w == nil {
		return err
	}
	how := "named by --workspace"
	if !named {
		how = "discovered at " + workspace.DiscoveryPath + "; --no-workspace runs " + verb + " on --root alone"
	}
	return &gokernel.Error{Code: "workspace-unsupported", Message: fmt.Sprintf("%s reads only --root and would ignore workspace aliases %s (%s); %s", verb, strings.Join(w.Aliases(), ", "), how, workspaceRemedies[verb])}
}
