// Command corvint-ruby-test-provider is the experimental RSpec provider
// (docs/specs/live-proof-carrying-verification-v0.md LPCV-V0-090..095,
// LPCV-V0-131..138).
// `rspec` runs RSpec once with its JSON formatter (or, with --formatter
// corvint-rspec-json/0, the Corvint-shipped one), optionally through an
// --exec-prefix such as `docker compose run --rm app`, and prints the
// corvint-ruby-test-provider/0 document on stdout. Git identity is always
// observed on the host before and after the run. Exit is nonzero only for a
// refusal (2), an infrastructure failure or a retention failure (1); a failed
// example is itself an observation and exits zero. With --retain the same
// document bytes are retained under .corvint/test-evidence of the enclosing
// Git worktree (LPCV-V0-055) by testevidence.Retain, which keeps it out of
// `git status` (LPCV-V0-084).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/rubytestprovider"
	"github.com/Beamfall/corvint/internal/testevidence"
)

const usage = `usage: corvint-ruby-test-provider rspec --dir <project-directory> [flags]

Experimental (LPCV-V0-090..095, LPCV-V0-131..138). Runs RSpec once with the
JSON formatter and prints the corvint-ruby-test-provider/0 document. Git is
observed on the host; under --exec-prefix, Ruby and Bundler versions are
observed inside the prefix by a separate probe invocation. --formatter
corvint-rspec-json/0 runs the Corvint-shipped formatter, which records
shared-example inclusion lines and, under a docker prefix, the run container
whose image the host docker then inspects.
`

var (
	errRefused   = errors.New("refused")
	errRetention = errors.New("retention failed")
)

type stringList []string

func (s *stringList) String() string { return fmt.Sprint([]string(*s)) }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	cancel()
	os.Exit(code)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if code, handled := companionHelp().Serve(args, stdout, stderr); handled {
		return code
	}
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usage)
		flags, _ := newFlags(stdout)
		flags.PrintDefaults()
		return 0
	case "rspec":
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q\n", args[0])
		return 2
	}
	err := runRSpec(ctx, args[1:], stdout, stderr)
	switch {
	case err == nil:
		return 0
	case errors.Is(err, flag.ErrHelp):
		return 0
	case errors.Is(err, errRefused):
		return 2
	case errors.Is(err, errRetention):
		return 1
	}
	fmt.Fprintln(stderr, "infrastructure failure:", err)
	return 1
}

type options struct {
	rspec, execPrefix, specs *stringList
	directory, formatter     *string
	junit, retain            *bool
	timeout                  *time.Duration
}

func newFlags(output io.Writer) (*flag.FlagSet, options) {
	flags := flag.NewFlagSet("rspec", flag.ContinueOnError)
	flags.SetOutput(output)
	opts := options{rspec: &stringList{}, execPrefix: &stringList{}, specs: &stringList{}}
	opts.directory = flags.String("dir", ".", "project directory RSpec runs in (inside a Git worktree)")
	flags.Var(opts.execPrefix, "exec-prefix", "one argument of a command prefix such as docker compose run --rm app (repeatable, in order)")
	flags.Var(opts.rspec, "rspec", "one argument of the RSpec command (repeatable; default bundle exec rspec)")
	flags.Var(opts.specs, "spec", "project-relative spec file or directory, path:line[:line...] or path[id,...] to run and record as the selection (repeatable)")
	opts.formatter = flags.String("formatter", rubytestprovider.FormatterStock, "stock (RSpec's json formatter) or "+rubytestprovider.FormatterCorvint+" (the Corvint-shipped formatter)")
	opts.junit = flags.Bool("junit", false, "also request RspecJunitFormatter and retain its digest")
	opts.retain = flags.Bool("retain", false, "retain the document under .corvint/test-evidence of the enclosing worktree")
	opts.timeout = flags.Duration("timeout", rubytestprovider.DefaultTimeout, "bound on the RSpec run")
	return flags, opts
}

func runRSpec(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags, opts := newFlags(stderr)
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return errRefused
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected arguments:", flags.Args())
		return errRefused
	}
	dir, err := filepath.Abs(*opts.directory)
	if err != nil {
		fmt.Fprintln(stderr, "refused:", err)
		return errRefused
	}
	document, err := rubytestprovider.Run(ctx, rubytestprovider.Config{Dir: dir, ExecPrefix: *opts.execPrefix, RSpec: *opts.rspec, Specs: *opts.specs, JUnit: *opts.junit, Timeout: *opts.timeout, Formatter: *opts.formatter})
	if err != nil {
		fmt.Fprintln(stderr, "refused:", err)
		return errRefused
	}
	data, err := json.Marshal(document)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if _, err := stdout.Write(data); err != nil {
		return err
	}
	retained := retain(stderr, *opts.retain, dir, data)
	if document.Infrastructure != nil {
		return fmt.Errorf("%s: %s", document.Infrastructure.Reason, document.Infrastructure.Detail)
	}
	return retained
}

func retain(stderr io.Writer, enabled bool, dir string, data []byte) error {
	if !enabled {
		return nil
	}
	worktree, err := rubytestprovider.EnclosingWorktree(dir)
	if err == nil {
		_, err = testevidence.Retain(worktree, rubytestprovider.Producer, data)
	}
	if err != nil {
		fmt.Fprintln(stderr, "retention failure:", err)
		return errRetention
	}
	return nil
}
