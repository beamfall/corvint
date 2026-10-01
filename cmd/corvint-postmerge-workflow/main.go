// Command corvint-postmerge-workflow is an experimental local replay companion, not a live writer.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Beamfall/corvint/internal/postmergeworkflow"
)

func main() { os.Exit(run()) }
func run() int {
	if len(os.Args) < 2 || os.Args[1] != "replay" {
		fmt.Fprintln(os.Stderr, "usage: corvint-postmerge-workflow replay --change ID --dry-run --fixture FILE --policy FILE --root REPO")
		return 2
	}
	f := flag.NewFlagSet("replay", flag.ContinueOnError)
	change := f.String("change", "", "immutable change selector")
	dry := f.Bool("dry-run", false, "required: recording connectors only")
	fixture := f.String("fixture", "", "historical fixture JSON")
	policy := f.String("policy", "", "separate host-owned policy JSON")
	root := f.String("root", ".", "product Git repository")
	if f.Parse(os.Args[2:]) != nil || f.NArg() != 0 || !*dry || *change == "" || *fixture == "" || *policy == "" {
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	report, err := postmergeworkflow.Replay(ctx, *root, *fixture, *policy, *change)
	if e := json.NewEncoder(os.Stdout).Encode(report); e != nil {
		return 2
	}
	if err != nil {
		return 2
	}
	if report.Status == "MISMATCH" {
		return 1
	}
	return 0
}
