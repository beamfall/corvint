// Package cli is the minimize companion's command entry, shared by the `corvint-playwright-minimize`
// shim and the `corvint-test minimize` group (CTF-V0-004). It is explicitly authorized and
// experimental: planning reads pinned evidence; execution launches bounded trials.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/Beamfall/corvint/internal/playwrightminimize"
)

// Run runs the minimize companion under the invocation name prog: `corvint-playwright-minimize` for
// the old companion shim, `corvint-test minimize` for the group (CTF-V0-004). Only the usage
// diagnostic names prog.
func Run(ctx context.Context, prog string, args []string, input io.Reader, output io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s <plan|execute> --experimental [--approve-plan sha256:...] < input.json", prog)
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	experimental := flags.Bool("experimental", false, "admit closed experimental profile")
	approval := flags.String("approve-plan", "", "authorize execution of exactly this live plan digest")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if !*experimental || flags.NArg() != 0 {
		return fmt.Errorf("explicit --experimental required; positional arguments forbidden")
	}
	data, err := io.ReadAll(io.LimitReader(input, (32<<20)+1))
	if err != nil {
		return err
	}
	var result any
	switch args[0] {
	case "plan":
		if *approval != "" {
			return fmt.Errorf("planning does not accept execution approval")
		}
		var request playwrightminimize.LiveRequest
		if err := playwrightminimize.DecodeLiveRequest(data, &request); err != nil {
			return err
		}
		result, err = playwrightminimize.LivePlanRequest(ctx, request)
	case "execute":
		if *approval == "" {
			return fmt.Errorf("explicit --approve-plan required")
		}
		var plan playwrightminimize.LivePlan
		if err := playwrightminimize.DecodeLiveRequest(data, &plan); err != nil {
			return err
		}
		result, err = playwrightminimize.ExecuteLive(ctx, plan, *approval)
	default:
		return fmt.Errorf("unknown operation %q", args[0])
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(result)
}
