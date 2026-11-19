// SPDX-License-Identifier: Apache-2.0

package flows

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/gokernel"
)

func runFlowsPropose(ctx context.Context, root string, args []string, out, diag io.Writer) error {
	f := flag.NewFlagSet("flows propose", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	links := f.Bool("links", false, "propose test links")
	all := f.Bool("all", false, "include assigned tests")
	flow := f.String("flow", "", "flow ID")
	if err := f.Parse(args); err != nil || f.NArg() != 0 || !(*links || *all) {
		return argumentError("flows propose requires --links or --all")
	}
	proposals, err := appflows.ProposeLinks(ctx, root, *flow, *all)
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(proposals)
}

func runFlowsAccept(ctx context.Context, root string, args []string, out, diag io.Writer) error {
	if !terminalInput(os.Stdin) {
		return &gokernel.Error{Code: "flow-accept-needs-person", Message: "flow-accept-needs-person"}
	}
	return acceptFlowArgs(ctx, root, args, out)
}

func acceptFlowArgs(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows accept", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	flow := f.String("flow", "", "flow ID")
	by := f.String("by", "", "reviewer name")
	variation := f.String("variation", "", "variation ID")
	var only []string
	f.Func("only", "test ID", func(s string) error { only = append(only, s); return nil })
	if err := f.Parse(args); err != nil || f.NArg() != 0 && len(only) == 0 || *flow == "" || *by == "" {
		return argumentError("flows accept requires --flow ID --by NAME")
	}
	only = append(only, f.Args()...)
	n, err := appflows.AcceptLinks(ctx, root, *flow, *by, *variation, only)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "accepted %d links\n", n)
	return err
}
