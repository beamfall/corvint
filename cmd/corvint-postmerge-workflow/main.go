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

// encodeIDUsage is the encode-id help (PMR-V0-960).
const encodeIDUsage = `usage: corvint-postmerge-workflow encode-id VALUE

Prints VALUE, such as a branch name, encoded as a workflow identifier:
every byte outside [A-Za-z0-9._-] becomes ':' and two uppercase hex digits.
VALUE must begin with a letter or digit and encode to at most 128 bytes.
Example: corvint-postmerge-workflow encode-id claude/lane-a   (prints claude:2Flane-a)`

func run() int {
	if len(os.Args) >= 2 && os.Args[1] == "encode-id" {
		if len(os.Args) == 3 && (os.Args[2] == "--help" || os.Args[2] == "-h") {
			fmt.Println(encodeIDUsage)
			return 0
		}
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, encodeIDUsage)
			return 2
		}
		id, err := postmergeworkflow.EncodeID(os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, "encode-id: "+err.Error())
			return 2
		}
		fmt.Println(id)
		return 0
	}
	if len(os.Args) < 2 || os.Args[1] != "replay" {
		fmt.Fprintln(os.Stderr, "usage: corvint-postmerge-workflow replay --change ID --dry-run --fixture FILE --policy FILE --root REPO\n       corvint-postmerge-workflow encode-id VALUE")
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
	if postmergeworkflow.FixtureProfile(*fixture) == postmergeworkflow.NativeProfile {
		report, _ := postmergeworkflow.ReplayNative(ctx, *root, *fixture, *policy, *change)
		if e := json.NewEncoder(os.Stdout).Encode(report); e != nil {
			return 2
		}
		return 2 // Every /1 outcome in this bounded slice is BLOCKED.
	}
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
