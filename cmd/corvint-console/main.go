// Command corvint-console builds the Corvint Console executable described by
// `docs/specs/local-admin-console-v0.md` (decision 0081).
//
// It is a separate, opt-in executable and is never linked into `corvint`, so
// an operator who does not start it has the invariant-7 local product
// unchanged. It binds loopback only, runs for the lifetime of this foreground
// invocation, installs no service, holds no database, and opens no outbound
// connection. It renders what `corvint-tasks`, `corvint-dashboard-snapshot` and `git`
// report, and performs a change only by invoking the owning tool's own verb.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/console"
)

type options struct {
	addr, repo, specs, tasks, snapshot string
	timeout                            time.Duration
}

func main() {
	parsed, err := parseOptions(os.Args[1:], os.Stderr)
	if err == flag.ErrHelp {
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "corvint-console: %v\n", err)
		os.Exit(2)
	}

	root, err := filepath.Abs(parsed.repo)
	if err != nil {
		fail("could not resolve --repo: %v", err)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		fail("--repo %s is not a directory", root)
	}

	specsRoot := root
	if parsed.specs != "" {
		if specsRoot, err = filepath.Abs(parsed.specs); err != nil {
			fail("could not resolve --specs: %v", err)
		}
	}
	// A Git read below the toplevel would render the enclosing repository (decision 0255).
	for _, candidate := range []string{root, specsRoot} {
		if err := console.RequireToplevel(context.Background(), candidate, parsed.timeout); err != nil {
			fail("%v", err)
		}
	}
	server, err := console.New(console.Options{
		Addr: parsed.addr, Repo: root, Binary: parsed.tasks, Specs: specsRoot,
		Snapshot: parsed.snapshot, Timeout: parsed.timeout,
	})
	if err != nil {
		fail("%v", err)
	}
	listener, err := net.Listen("tcp", parsed.addr)
	if err != nil {
		fail("could not bind %s: %v", parsed.addr, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(os.Stderr, "Corvint Console on http://%s administering %s\n", listener.Addr(), root)
	fmt.Fprintf(os.Stderr, "loopback only, no account, no database, no outbound connection; ^C to stop\n")
	if err := server.Serve(ctx, listener); err != nil {
		fail("%v", err)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "corvint-console: "+format+"\n", args...)
	os.Exit(1)
}

func parseOptions(arguments []string, output io.Writer) (options, error) {
	defaults := options{
		addr: "127.0.0.1:7777", repo: ".",
		tasks:    companion("corvint-tasks", "atm"),
		snapshot: companion("corvint-dashboard-snapshot"),
		timeout:  30 * time.Second,
	}
	parsed := defaults
	var tasks, legacyTasks string
	flags := flag.NewFlagSet("corvint-console", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&parsed.addr, "addr", defaults.addr, "loopback address to bind; a non-loopback address is refused")
	flags.StringVar(&parsed.repo, "repo", defaults.repo, "repository to administer")
	flags.StringVar(&tasks, "tasks", "", "the Corvint Tasks executable to invoke")
	flags.StringVar(&legacyTasks, "atm", "", "legacy alias for -tasks")
	flags.StringVar(&parsed.specs, "specs", "", "repository holding docs/specs and the tree the code pane reads (default: -repo)")
	flags.StringVar(&parsed.snapshot, "snapshot", defaults.snapshot, "the corvint-dashboard-snapshot executable the evidence pane invokes")
	flags.DurationVar(&parsed.timeout, "timeout", defaults.timeout,
		"per-invocation timeout for corvint-tasks, git and the snapshot compiler; the snapshot is the slowest source and may need longer on a loaded host")
	if err := flags.Parse(arguments); err != nil {
		return options{}, err
	}
	seen := make(map[string]bool)
	flags.Visit(func(current *flag.Flag) { seen[current.Name] = true })
	selected, explicit, err := selectTasks(tasks, seen["tasks"], legacyTasks, seen["atm"])
	if err != nil {
		return options{}, err
	}
	if explicit {
		parsed.tasks = selected
	}
	return parsed, nil
}

func selectTasks(tasks string, tasksSet bool, legacy string, legacySet bool) (string, bool, error) {
	if tasksSet && tasks == "" {
		return "", false, fmt.Errorf("-tasks must not be empty")
	}
	if legacySet && legacy == "" {
		return "", false, fmt.Errorf("-atm must not be empty")
	}
	if tasksSet && legacySet && tasks != legacy {
		return "", false, fmt.Errorf("-tasks and -atm conflict")
	}
	if tasksSet {
		return tasks, true, nil
	}
	if legacySet {
		return legacy, true, nil
	}
	return "", false, nil
}

// Prefer adjacent current and legacy companions over unrelated PATH installs.
func companion(names ...string) string {
	executable, err := os.Executable()
	if err != nil {
		executable = ""
	}
	return findCompanion(executable, names, exec.LookPath)
}

func findCompanion(executable string, names []string, lookPath func(string) (string, error)) string {
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		for _, name := range names {
			candidate := filepath.Join(filepath.Dir(resolved), name)
			if info, statErr := os.Stat(candidate); statErr == nil && info.Mode().IsRegular() {
				return candidate
			}
		}
	}
	for _, name := range names {
		if candidate, lookErr := lookPath(name); lookErr == nil {
			return candidate
		}
	}
	return names[0]
}
