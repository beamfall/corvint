package main

import (
	"context"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/gitnotes"
)

// init installs `cem anchor` and `cem provenance` (FPK-V0-037..040) into the
// CEM dispatch without adding internal/gitnotes to the CEM seams' closure.
func init() { cemcli.GitNotes = runCEMGitNotes }

func runCEMGitNotes(ctx context.Context, root, action string, values map[string]string) (map[string]any, error) {
	if action == "provenance" {
		return gitnotes.Provenance(ctx, root, values["--commit"])
	}
	commit := values["--commit"]
	if commit == "" {
		commit = "HEAD"
	}
	return gitnotes.Anchor(ctx, root, values["--map"], commit)
}
