// Command corvint-postmerge-connect is a separate experimental companion.
// Its live references are local files; it has no network or agent write tool.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	pm "github.com/Beamfall/corvint/internal/postmergeconnector"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: corvint-postmerge-connect <plan|read-intake|write> --experimental --root REPO --fixture FILE --policy FILE --input FILE [--mode dry-run|recording|live --output FILE --author-root DIR]; live is local-fixture-only")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	experimental := flags.Bool("experimental", false, "admit experimental local fixture profile; no remote qualification")
	root := flags.String("root", "", "product Git checkout")
	fixturePath := flags.String("fixture", "", "absolute raw tracker/forge fixture file; reader access only")
	policyPath := flags.String("policy", "", "absolute trusted independent policy file")
	inputPath := flags.String("input", "", "absolute typed write input file; no prose slots")
	mode := flags.String("mode", "dry-run", "dry-run | recording | live (local fixture only)")
	output := flags.String("output", "", "absolute private file outside authoring worktree")
	author := flags.String("author-root", "", "actual authoring worktree for output exclusion")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if !*experimental || flags.NArg() != 0 || !filepath.IsAbs(*root) {
		return fmt.Errorf("experimental-and-absolute-root-required")
	}
	var fixture pm.Fixture
	var policy pm.Policy
	var input pm.Input
	for _, file := range []struct {
		path  string
		value any
	}{{*fixturePath, &fixture}, {*policyPath, &policy}, {*inputPath, &input}} {
		b, e := pm.ReadFile(file.path)
		if e != nil {
			return e
		}
		if e = pm.Decode(b, file.value); e != nil {
			return e
		}
	}
	inputs := []string{*fixturePath, *policyPath, *inputPath}
	protected, e := pm.ProtectedGitPaths(ctx, *root)
	if e != nil {
		return e
	}
	inputs = append(inputs, protected...)
	if args[0] == "read-intake" {
		if *output == "" || *author == "" || *mode != "dry-run" {
			return fmt.Errorf("intake-file-and-author-root-required")
		}
		if input.Binding != policy.Expected {
			return fmt.Errorf("input-binding-mismatch")
		}
		intake, e := pm.Read(ctx, *root, fixture, policy, input.SourceItem)
		if e != nil {
			return e
		}
		return pm.SaveIntake(*output, *author, inputs, intake)
	}
	plan, e := pm.Build(ctx, *root, fixture, policy, input)
	if e != nil {
		return e
	}
	if args[0] == "plan" {
		if *output != "" || *author != "" || *mode != "dry-run" {
			return fmt.Errorf("plan-does-not-write-files")
		}
		b, e := pm.Encode(plan)
		if e != nil {
			return e
		}
		n, e := out.Write(b)
		if e == nil && n != len(b) {
			e = io.ErrShortWrite
		}
		return e
	}
	if args[0] != "write" {
		return fmt.Errorf("unknown-operation")
	}
	switch *mode {
	case "dry-run":
		if *output != "" || *author != "" {
			return fmt.Errorf("dry-run-does-not-write-files")
		}
		return pm.DryRun(ctx, *root, fixture, policy, plan, out)
	case "recording":
		if *output == "" || *author == "" {
			return fmt.Errorf("recording-destination-required")
		}
		return pm.Record(ctx, *root, fixture, policy, plan, *output, *author, inputs)
	case "live":
		if *output == "" || *author == "" {
			return fmt.Errorf("live-fixture-destination-required")
		}
		if e := pm.CheckDestination(*output, *author, inputs); e != nil {
			return e
		}
		state := pm.NewState()
		if _, e := os.Lstat(*output); e == nil {
			b, e := pm.ReadFile(*output)
			if e != nil {
				return e
			}
			if e = pm.Decode(b, &state); e != nil {
				return e
			}
		} else if !os.IsNotExist(e) {
			return e
		}
		next, e := pm.ApplyLocal(ctx, *root, fixture, policy, plan, state)
		if e != nil {
			return e
		}
		return pm.SaveState(*output, *author, inputs, next)
	default:
		return fmt.Errorf("mode-unsupported")
	}
}
