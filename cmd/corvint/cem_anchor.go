package main

import (
	"context"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/gitnotes"
)

// init installs `cem anchor` and `cem provenance` (FPK-V0-037..040) into the
// CEM dispatch without adding internal/gitnotes to the CEM seams' closure.
func init() { cemcli.GitNotes = runCEMGitNotes }

// An anchor given --artifacts also records the native stable verdict in the
// verdict ledger (CEM-V1-273); provenance --verdicts lists it read-only
// (CEM-V1-277).
func runCEMGitNotes(ctx context.Context, root, action string, values map[string]string, booleans map[string]bool) (map[string]any, error) {
	if action == "provenance" {
		return gitnotes.ProvenanceWithVerdictsTip(ctx, root, values["--commit"], values["--notes-tip"], booleans["--verdicts"], values["--verdicts-tip"])
	}
	commit := values["--commit"]
	if commit == "" {
		commit = "HEAD"
	}
	artifacts, record := values["--artifacts"]
	return gitnotes.AnchorWithVerdict(ctx, root, values["--map"], commit, artifacts, record)
}
