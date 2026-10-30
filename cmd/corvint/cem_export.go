package main

import (
	"context"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/receiptbundle"
)

// init installs `cem export` (RCB-V0-001) into the CEM dispatch without adding
// internal/receiptbundle to the CEM seams' closure.
func init() { cemcli.Export = runCEMExport }

// A cem/1.0 map is exported through the stable route, which admits callerRoot
// byte for byte and needs --artifacts (CEM-V1-060).
func runCEMExport(ctx context.Context, root, callerRoot string, values map[string]string) (map[string]any, error) {
	_, artifactsGiven := values["--artifacts"]
	_, witnessGiven := values["--witness"]
	return receiptbundle.Export(ctx, root, receiptbundle.Options{
		MapPath: values["--map"], ExpectedBase: values["--expected-base"], Target: values["--target"],
		Output: values["--output"], Witness: values["--witness"],
		Repository: callerRoot, Artifacts: values["--artifacts"], ArtifactsGiven: artifactsGiven, WitnessGiven: witnessGiven,
	})
}
