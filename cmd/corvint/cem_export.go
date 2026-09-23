package main

import (
	"context"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/receiptbundle"
)

// init installs `cem export` (RCB-V0-001) into the CEM dispatch without adding
// internal/receiptbundle to the CEM seams' closure.
func init() { cemcli.Export = runCEMExport }

func runCEMExport(ctx context.Context, root string, values map[string]string) (map[string]any, error) {
	return receiptbundle.Export(ctx, root, receiptbundle.Options{
		MapPath: values["--map"], Target: values["--target"],
		Output: values["--output"], Witness: values["--witness"],
	})
}
