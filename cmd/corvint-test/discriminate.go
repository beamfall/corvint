package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/extmutate"
)

const discriminateUsage = "usage: corvint-test e2e discriminate --base REV --head REV --app-dir DIR --test-command JSON [--root DIR] [--tests-dir DIR] [--rebuild-command JSON] [--verdict exit|provider] [--max-mutants N] [--wall-time D] [--run-timeout D] [--rebuild-timeout D] [--dir-match any|leading] [--exclude-dir NAME]... [--keep-dir NAME]... [--map FILE] [--out FILE]"

// discriminateHelp is `corvint-test e2e discriminate --help` (CCE-V0-176); the options follow it.
const discriminateHelp = `Mutate the changed Ruby and TS/JS application lines of base..head and record which external
end-to-end tests kill each mutant (CCE-V0-170..176, 236..238).

Usage:
  corvint-test e2e discriminate --base REV --head REV --app-dir DIR --test-command JSON [--root DIR]
    [--tests-dir DIR] [--rebuild-command JSON] [--verdict exit|provider] [--max-mutants N]
    [--wall-time D] [--run-timeout D] [--rebuild-timeout D] [--dir-match any|leading]
    [--exclude-dir NAME]... [--keep-dir NAME]... [--map FILE] [--out FILE]

Inputs:
  --app-dir is a Git worktree checked out at --head with no modified tracked files; each mutant
  is written there, the --rebuild-command (a JSON argv, run in --app-dir) rebuilds or restarts the
  application, the --test-command (a JSON argv, run in --tests-dir, default --app-dir) runs the
  suite, and the original bytes are restored and verified before the next mutant. Both commands
  see CORVINT_EXTMUTATE_RUN=baseline, m1, m2, ... or restore. --verdict exit counts a nonzero exit
  as a kill of the one test "command"; --verdict provider reads a corvint-js-test-provider or
  corvint-ruby-test-provider document from stdout, decides per test, and records a provider
  infrastructure failure (a preflight included) as not-run. The operator set is
  corvint-extmutate-operators/0; at most --max-mutants (default 20, at most 500) mutants run
  inside --wall-time (default 30m), and every bound leaves named not-run records. Files under a
  test, vendored or generated directory name (spec, test, e2e, vendor, ...) are not mutated:
  --dir-match leading matches those names against a path's first segment only, --exclude-dir
  adds a name and --keep-dir removes one. --map joins each hunk to the containing hunk ID of a
  cem/1.0 map prepared for the same base and head.

Produces:
  An e2e-external-discrimination/0 document on stdout (or --out): the base, app and test
  revisions, every mutant with its rebuild, exit status and receipt digest, killed, survived
  or not-run per (hunk, test), and every changed file left unmutated with its rule. Exit 1 after printing the document when --app-dir could not be
  restored.

Example:
  corvint-test e2e discriminate --base origin/main --head HEAD --app-dir /tmp/app-wt --test-command '["bundle","exec","rspec","spec/system"]' --rebuild-command '["bin/restart"]'
`

func discriminate(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("corvint-test e2e discriminate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", ".", "repository the base..head diff is read from")
	base := fs.String("base", "", "base revision")
	head := fs.String("head", "", "head revision; --app-dir must be checked out at it")
	appDir := fs.String("app-dir", "", "clean Git worktree at --head that each mutant is written to and restored in")
	testsDir := fs.String("tests-dir", "", "directory the test command runs in and whose Git revision pins the tests; default --app-dir")
	testCommand := fs.String("test-command", "", "JSON argv of the external test command or a Corvint JS/Ruby provider")
	rebuildCommand := fs.String("rebuild-command", "", "JSON argv run in --app-dir after each mutant is applied and after the last restore")
	verdict := fs.String("verdict", extmutate.VerdictExit, "exit (nonzero exit kills the one test \"command\") or provider (per-test from a provider document on stdout)")
	maxMutants := fs.Int("max-mutants", extmutate.DefaultMaxMutants, "mutants to run, at most 500; the rest are recorded as omitted")
	wallTime := fs.Duration("wall-time", extmutate.DefaultWallTime, "bound on the whole run; later mutants are not-run")
	runTimeout := fs.Duration("run-timeout", extmutate.DefaultRunTimeout, "bound on one test command run")
	rebuildTimeout := fs.Duration("rebuild-timeout", extmutate.DefaultRebuildTimeout, "bound on one rebuild command run")
	dirMatch := fs.String("dir-match", extmutate.DirectoryMatchAny, "any (an excluded directory name anywhere in the path) or leading (the first path segment only)")
	var excludeDirs, keepDirs nameList
	fs.Var(&excludeDirs, "exclude-dir", "repeatable: also exclude files under this directory name")
	fs.Var(&keepDirs, "keep-dir", "repeatable: stop excluding this default directory name")
	mapPath := fs.String("map", "", "cem/1.0 map of the same base and head whose hunk IDs the records carry")
	out := fs.String("out", "", "write the document to this file instead of stdout")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return writeCommandHelp(stdout, "discriminate", discriminateHelp, fs)
		}
		return fmt.Errorf("%v (%s)", err, discriminateUsage)
	}
	if fs.NArg() != 0 {
		return errors.New(discriminateUsage)
	}
	opts := extmutate.Options{Root: *root, Base: *base, Head: *head, AppDir: *appDir, TestsDir: *testsDir, Verdict: *verdict,
		MaxMutants: *maxMutants, WallTime: *wallTime, RunTimeout: *runTimeout, RebuildTimeout: *rebuildTimeout,
		DirectoryMatch: *dirMatch, ExcludeDirectories: excludeDirs, KeepDirectories: keepDirs, MapPath: *mapPath}
	for _, argv := range []struct {
		raw    string
		target *[]string
		name   string
	}{{*testCommand, &opts.TestCommand, "test-command"}, {*rebuildCommand, &opts.RebuildCommand, "rebuild-command"}} {
		if argv.raw != "" && json.Unmarshal([]byte(argv.raw), argv.target) != nil {
			return &extmutate.Refusal{Code: "discriminate-command-invalid", Detail: "--" + argv.name + " must be a JSON array of strings"}
		}
	}
	for _, d := range []time.Duration{*wallTime, *runTimeout, *rebuildTimeout} {
		if d <= 0 {
			return &extmutate.Refusal{Code: "discriminate-bound-invalid", Detail: "durations must be positive"}
		}
	}
	if *maxMutants <= 0 {
		return &extmutate.Refusal{Code: "discriminate-bound-invalid", Detail: "max-mutants must be in 1..500"}
	}
	doc, runErr := extmutate.Discriminate(ctx, opts)
	if doc.Schema == "" {
		return runErr
	}
	if *out != "" {
		if err := os.WriteFile(*out, doc.Encode(), 0o644); err != nil {
			return errors.Join(runErr, err)
		}
	} else if _, err := stdout.Write(doc.Encode()); err != nil {
		return errors.Join(runErr, err)
	}
	return runErr
}

// nameList is a repeatable string flag.
type nameList []string

func (l *nameList) String() string { return strings.Join(*l, ",") }

func (l *nameList) Set(value string) error {
	*l = append(*l, value)
	return nil
}
