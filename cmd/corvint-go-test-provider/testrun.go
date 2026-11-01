package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/Beamfall/corvint/internal/gotestrun"
)

const testRunUsage = `usage: corvint-go-test-provider test-run [--dir DIR] [--timeout DURATION] [PACKAGE...]

Runs go test -json -count=1 once in DIR (default .) for the package patterns (default
./...) and prints one corvint-go-test-run/0 document: the repository identity
before and after the run, the module path that maps import paths to tree
directories, and each test's outcome with the repository-relative file that
declares it and that file's Git blob ID. corvint witness --evidence binds it
to the head only when that identity and every blob ID match (AGW-V0-012).
Exit 0 prints a document, including one that records infrastructure failure;
exit 2 is a refusal before the run.
`

// runOrTestRun dispatches the test-run subcommand and otherwise runs the
// existing provider entry point unchanged.
func runOrTestRun(arguments []string, stdout, stderr io.Writer) int {
	if code, handled := serveHelp(arguments, stdout, stderr); handled {
		return code
	}
	if len(arguments) == 0 || arguments[0] != "test-run" {
		return run(arguments, stdout, stderr)
	}
	ctx, cancel := providerContext()
	defer cancel()
	return runTestRunCommand(ctx, arguments[1:], stdout, stderr)
}

// runTestRunCommand is the test-run subcommand (AGW-V0-012). It is separate
// from the closed corvint-go-live-session-event/0 session stream.
func runTestRunCommand(ctx context.Context, arguments []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("corvint-go-test-provider test-run", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	directory := flags.String("dir", ".", "directory inside the module to run go test in")
	timeout := flags.Duration("timeout", gotestrun.DefaultTimeout, "bound on the go test run")
	if err := flags.Parse(arguments); err != nil {
		fmt.Fprint(stderr, testRunUsage)
		return 2
	}
	dir, err := filepath.Abs(*directory)
	if err != nil || *timeout <= 0 || *timeout > 24*time.Hour {
		fmt.Fprint(stderr, testRunUsage)
		return 2
	}
	document, err := gotestrun.Run(ctx, gotestrun.Config{Dir: dir, Packages: flags.Args(), Timeout: *timeout})
	if err != nil {
		fmt.Fprintln(stderr, "corvint-go-test-provider test-run: refused:", err)
		return 2
	}
	data, err := json.Marshal(document)
	if err != nil {
		fmt.Fprintln(stderr, "corvint-go-test-provider test-run:", err)
		return 1
	}
	if _, err := stdout.Write(append(data, '\n')); err != nil {
		return 1
	}
	return 0
}
