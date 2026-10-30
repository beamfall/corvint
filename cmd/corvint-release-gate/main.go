// corvint-release-gate is an experimental offline evidence gate. It never builds
// or publishes artifacts.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Beamfall/corvint/internal/releasegate"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(arguments []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	options, err := parse(arguments)
	if err == nil {
		var report releasegate.Report
		report, err = releasegate.Scan(ctx, options)
		if err == nil {
			err = json.NewEncoder(stdout).Encode(report)
			if err == nil {
				return reportOutcome(report)
			}
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "corvint-release-gate:", err)
		return 2
	}
	return 0
}

func reportOutcome(report releasegate.Report) int {
	if report.Blocking() {
		return 1
	}
	return 0
}

func parse(arguments []string) (releasegate.Options, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return releasegate.Options{}, err
	}
	result := releasegate.Options{Root: workingDirectory}
	for index := 0; index < len(arguments); index++ {
		switch arguments[index] {
		case "--root", "--commit", "--manifest", "--git", "--git-sha256", "--policy", "--policy-sha256":
			if index+1 == len(arguments) {
				return result, fmt.Errorf("%s requires a value", arguments[index])
			}
			value := arguments[index+1]
			index++
			switch arguments[index-1] {
			case "--root":
				result.Root = value
			case "--commit":
				result.Commit = value
			case "--manifest":
				result.ManifestPath = value
			case "--git":
				result.GitExecutable = value
			case "--git-sha256":
				result.GitSHA256 = value
			case "--policy":
				result.PolicyPath = value
			case "--policy-sha256":
				result.PolicySHA256 = value
			}
			if err != nil {
				return result, err
			}
		default:
			return result, fmt.Errorf("unknown argument %s", arguments[index])
		}
	}
	if result.Commit == "" {
		return result, fmt.Errorf("--commit is required")
	}
	if result.ManifestPath == "" {
		return result, fmt.Errorf("--manifest is required and must name a path in --commit")
	}
	if result.GitExecutable == "" || result.GitSHA256 == "" || result.PolicyPath == "" || result.PolicySHA256 == "" {
		return result, fmt.Errorf("--git, --git-sha256, --policy, and --policy-sha256 are required external pins")
	}
	return result, nil
}
