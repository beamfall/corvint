// Command corvint-playwright-minimize is an explicitly authorized experimental
// companion. Planning reads pinned evidence; execution launches bounded trials.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Beamfall/corvint/internal/playwrightminimize"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: corvint-playwright-minimize <plan|execute> --experimental [--approve-plan sha256:...] < input.json")
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
