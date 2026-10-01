// corvint-cem-experiments is an opt-in experimental local companion, excluded
// from the Core release. Tests are trusted executable code, not sandboxed code.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	ce "github.com/Beamfall/corvint/internal/criterionexperiment"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(command(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
func command(ctx context.Context, args []string, out, errout io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errout, "experimental companion: plan | run | verify | gate")
		return 2
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(errout)
	tasksExe := flags.String("tasks-executable", "", "independently trusted absolute Tasks executable")
	tasksSHA := flags.String("tasks-sha256", "", "independently pinned Tasks executable SHA256")
	repo := flags.String("repo", ".", "repository")
	request := flags.String("request", "", "request JSON")
	plan := flags.String("plan", "", "plan JSON")
	receipt := flags.String("receipt", "", "receipt JSON")
	output := flags.String("out", "", "NEW absolute disposable output directory")
	approve := flags.String("approve", "", "exact canonical plan SHA256")
	experimental := flags.Bool("experimental", false, "admit experimental profile")
	trusted := flags.Bool("trusted-local", false, "admit trusted local test execution")
	seen := map[string]bool{}
	for _, arg := range args[1:] {
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		name := strings.SplitN(strings.TrimLeft(arg, "-"), "=", 2)[0]
		if seen[name] {
			fmt.Fprintln(errout, "repeated flag:", name)
			return 2
		}
		seen[name] = true
	}
	if flags.Parse(args[1:]) != nil || flags.NArg() != 0 {
		return 2
	}
	tasks := ce.TasksVerifierConfig{Executable: *tasksExe, SHA256: *tasksSHA}
	var result any
	var err error
	switch args[0] {
	case "plan":
		var b []byte
		b, err = ce.Read(*request)
		if err == nil {
			var r ce.Request
			err = ce.Decode(b, &r)
			if err == nil {
				var p ce.Plan
				p, err = ce.PlanExperiment(ctx, *repo, r, *output, tasks)
				if err == nil {
					result = map[string]string{"schema": ce.Profile, "state": "PLANNED", "planSha256": ce.CanonicalDigest(p)}
				}
			}
		}
	case "run":
		var r ce.Receipt
		r, err = ce.Run(ctx, *repo, *plan, *approve, *output, *experimental, *trusted, tasks)
		if err == nil {
			result = map[string]string{"schema": ce.Profile, "state": "RECORDED", "receiptSha256": ce.CanonicalDigest(r)}
		}
	case "verify", "gate":
		result, err = ce.Verify(ctx, *repo, *plan, *receipt, args[0] == "gate", tasks)
	default:
		err = fmt.Errorf("unknown command")
	}
	if err != nil {
		fmt.Fprintln(errout, "experiment refused:", err)
		return 1
	}
	fmt.Fprintln(out, string(ce.Encode(result)))
	return 0
}
