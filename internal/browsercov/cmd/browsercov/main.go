// SPDX-License-Identifier: Apache-2.0

// Command browsercov collects a per-test line-coverage index from a browser
// end-to-end suite (Playwright or Cypress), joining the browser's V8 coverage
// with the coverage of the server under test (LPCV-V0-970..979). The index is
// the internal/linecov format, bound to the checkout's HEAD.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/browsercov"
	"github.com/Beamfall/corvint/internal/linecov"
)

const usage = `usage: browsercov collect -root R -runner playwright|cypress -out INDEX [flags]

Collect runs each spec file once in order, then each test alone, with every
Node process and the browser instrumented, and writes a linecov index of
which tracked lines (browser code and server code) each test executed. A test
that fails alone, runs with others, or leaves coverage that cannot be bound to
a tracked file gets no record, so selection keeps it.

Playwright starts the server under test from its webServer config. Cypress
does not: pass -server and -server-url, and a Chromium-family -browser.
Protractor, WebdriverIO, TestCafe, Nightwatch, Storybook test-runner, Detox
and Windows are refused.

Flags:
  -root DIR         clean checkout at the index commit (default .)
  -runner NAME      playwright or cypress (required)
  -out FILE         index file to write (required)
  -outcomes FILE    also write each test's outcome as JSON
  -config FILE      the runner's configuration, root-relative
  -server CMD       shell command that starts the application under test
  -server-url URL   URL that answers once the server is ready
  -browser B        Cypress browser name or path (Chromium family)
  -module M         record path prefix (default browser)
  -bin DIR          runner executables (default ROOT/node_modules/.bin)
  -timeout D        per runner invocation (default 10m)
  -work DIR         scratch directory parent (default the system temp directory)

Example:
  browsercov collect -root . -runner playwright -out e2e.linecov
  browsercov collect -root . -runner cypress -browser chrome \
    -server 'node server.js' -server-url http://localhost:3000 -out e2e.linecov
`

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help" {
		fmt.Fprint(os.Stdout, usage)
		if len(os.Args) < 2 {
			os.Exit(2)
		}
		return
	}
	if os.Args[1] != "collect" {
		fmt.Fprintf(os.Stderr, "browsercov: unknown command %q\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err := collect(os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "browsercov:", err)
		os.Exit(1)
	}
}

func collect(args []string) error {
	fs := flag.NewFlagSet("collect", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	var o browsercov.Options
	fs.StringVar(&o.Root, "root", ".", "")
	fs.StringVar(&o.Runner, "runner", "", "")
	out := fs.String("out", "", "")
	outcomes := fs.String("outcomes", "", "")
	fs.StringVar(&o.Config, "config", "", "")
	fs.StringVar(&o.Server, "server", "", "")
	fs.StringVar(&o.ServerURL, "server-url", "", "")
	fs.StringVar(&o.Browser, "browser", "", "")
	fs.StringVar(&o.Module, "module", "browser", "")
	fs.StringVar(&o.Bin, "bin", "", "")
	fs.DurationVar(&o.Timeout, "timeout", 10*time.Minute, "")
	scratch := fs.String("work", "", "")
	fs.Parse(args)
	if *out == "" || o.Runner == "" {
		return errors.New("collect: -runner and -out are required")
	}
	commit, err := git(o.Root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if dirty, err := git(o.Root, "status", "--porcelain", "--untracked-files=no"); err != nil {
		return err
	} else if dirty != "" {
		return errors.New("collect: checkout has uncommitted changes")
	}
	work, err := os.MkdirTemp(*scratch, "browsercov-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	o.Work = work
	o.Log = func(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...) }
	ctx := context.Background()
	start := time.Now()
	res, runner, err := browsercov.Collect(ctx, o)
	if err != nil {
		return err
	}
	var records []linecov.Record
	var unordered []string
	refused := 0
	for _, r := range res {
		if r.Record == nil {
			refused++
			fmt.Fprintf(os.Stderr, "browsercov: no record for %s %s: %s\n", r.Package, r.Name, r.Reason)
			if r.Name == linecov.InOrder {
				unordered = append(unordered, r.Package)
			}
			continue
		}
		records = append(records, *r.Record)
	}
	oids, err := linecov.OIDs(ctx, o.Root, commit)
	if err != nil {
		return err
	}
	ix, err := linecov.Build(o.Module, commit, runner, oids, records)
	if err != nil {
		return err
	}
	ix.Unordered = unordered
	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	if err := ix.Encode(f); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if *outcomes != "" {
		type row struct{ Package, Name, Reason string }
		rows := make([]row, len(res))
		for i, r := range res {
			rows[i] = row{r.Package, r.Name, r.Reason}
		}
		body, _ := json.MarshalIndent(rows, "", "  ")
		if err := os.WriteFile(*outcomes, append(body, '\n'), 0o644); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "browsercov: %d records (%d without), %d files, %s at %s in %s\n", len(records), refused, len(ix.Files), runner, commit, time.Since(start).Round(time.Second))
	return nil
}

func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
