// SPDX-License-Identifier: Apache-2.0
// Command fault-harness measures whether a selected test run executes every
// test that kills a fault put into a line a PR changed (FIH-V1). It generalises
// tools/affected-edit-oracle: pre-registered, hashed fault sets, a three-run
// baseline with quarantine, three-run kill confirmation and a miss verdict per
// runner. It never changes the selector or writes a source repository.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"
)

const usage = `Usage: fault-harness COMMAND [FLAGS]

Commands:
  check           verify a pre-registration against its committed hash (FIH-V1-001)
  select-reverts  apply the reverted-fix rule to observed PR facts (FIH-V1-002)
  generate-go     generate 3 to 5 Go mutants on one PR's changed lines (FIH-V1-002, 003)
  seal            check a fault set against its pre-registration and seal it (FIH-V1-002, 014)
  parity          run affected-edit-oracle mutation cases through the Go adapter (FIH-V1-011)

Run "fault-harness COMMAND --help" for a command's flags and an example.
Exit 0 means the measurement passed; 1 a measured failure or a refusal; 2 an invocation error.
`

type subcommand struct {
	name string
	run  func(ctx context.Context, args []string, w io.Writer) int
}

func commands() []subcommand {
	return []subcommand{
		{"check", runCheckCommand},
		{"select-reverts", runSelectRevertsCommand},
		{"generate-go", runGenerateGoCommand},
		{"seal", runSealCommand},
		{"parity", runParityCommand},
	}
}

func run(ctx context.Context, args []string, w io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(w, usage)
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	for _, c := range commands() {
		if c.name == args[0] {
			return c.run(ctx, args[1:], w)
		}
	}
	fmt.Fprintf(w, "fault-harness: unknown command %q\n%s", args[0], usage)
	return 2
}

// flags builds a subcommand flag set whose usage prints a line and an example.
func flags(name, line, example string, w io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("fault-harness "+name, flag.ContinueOnError)
	fs.SetOutput(w)
	fs.Usage = func() {
		fmt.Fprintf(w, "Usage: fault-harness %s %s\nExample: fault-harness %s %s\n", name, line, name, example)
		fs.PrintDefaults()
	}
	return fs
}

// parse returns -1 to continue, else the exit code.
func parse(fs *flag.FlagSet, args []string, required ...*string) int {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 || slices.ContainsFunc(required, func(s *string) bool { return *s == "" }) {
		fs.Usage()
		return 2
	}
	return -1
}

// report writes v as indented JSON to out, or to w when out is empty.
func report(w io.Writer, out string, v any) error {
	body, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if out == "" {
		_, err = w.Write(body)
		return err
	}
	return os.WriteFile(out, body, 0o600)
}

// resolveRun checks the source, clone parent and planner binary.
func resolveRun(source, clones, binary string) (runConfig, error) {
	var cfg runConfig
	var err error
	if cfg.Source, err = filepath.Abs(source); err != nil {
		return cfg, err
	}
	if cfg.Clones, err = filepath.Abs(clones); err != nil {
		return cfg, err
	}
	if st, err := os.Stat(cfg.Clones); err != nil || !st.IsDir() {
		return cfg, errors.New("--clones must be an existing private directory")
	}
	if cfg.Binary, err = exec.LookPath(binary); err != nil {
		return cfg, err
	}
	cfg.Binary, err = filepath.Abs(cfg.Binary)
	return cfg, err
}

func runParityCommand(ctx context.Context, args []string, w io.Writer) int {
	fs := flags("parity", "--corpus FILE --root DIR --clones DIR [--workspace DIR] [--corvint BIN] [--case ID]... [--out FILE]",
		"--corpus tools/affected-edit-oracle/testdata/corpus.json --root . --clones /tmp/fh-clones --out /tmp/parity.json", w)
	corpusFile := fs.String("corpus", "tools/affected-edit-oracle/testdata/corpus.json", "affected-edit-oracle/0 corpus")
	root := fs.String("root", ".", "Corvint source checkout (read only)")
	workspace := fs.String("workspace", "", "Beamfall source parent (read only), for beamfall-workspace cases")
	clones := fs.String("clones", "", "required existing private clone parent; each case is removed after execution")
	binary := fs.String("corvint", "corvint", "planner binary path or PATH name")
	timeout := fs.Duration("timeout", 10*time.Minute, "wall bound per adapter command")
	out := fs.String("out", "", "write the JSON parity report (default stdout)")
	var only multiFlag
	fs.Var(&only, "case", "run only this case ID (repeatable)")
	if code := parse(fs, args, clones); code >= 0 {
		return code
	}
	body, err := boundedRead(*corpusFile, 4<<20)
	if err != nil {
		fmt.Fprintln(w, err)
		return 2
	}
	var c aeoCorpus
	if err := decodeStrict(body, &c); err != nil || c.Profile != aeoProfile {
		fmt.Fprintf(w, "corpus: want %s: %v\n", aeoProfile, err)
		return 2
	}
	rows := []parityRow{}
	code := 0
	for _, tc := range c.Cases {
		if tc.Mutation == nil || (len(only) > 0 && !slices.Contains(only, tc.ID)) {
			continue
		}
		source := *root
		if name, ok := strings.CutPrefix(tc.Repo, "beamfall-workspace/"); ok {
			if *workspace == "" {
				rows = append(rows, parityRow{ID: tc.ID, Safety: "SKIPPED", Detail: "no --workspace for " + tc.Repo})
				continue
			}
			source = filepath.Join(*workspace, name)
		}
		cfg, err := resolveRun(source, *clones, *binary)
		if err != nil {
			fmt.Fprintln(w, err)
			return 2
		}
		row := runParity(ctx, cfg, adapterOptions{Timeout: *timeout}, tc)
		if row.Safety != "PASS" && row.Safety != "SKIPPED" {
			code = 1
		}
		rows = append(rows, row)
	}
	if err := report(w, *out, map[string]any{"profile": "fault-harness-parity/1", "corpusSha256": sha(body), "cases": rows}); err != nil {
		fmt.Fprintln(w, err)
		return 2
	}
	return code
}

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout)
	stop()
	os.Exit(code)
}
