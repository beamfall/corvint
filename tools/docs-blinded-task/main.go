// SPDX-License-Identifier: Apache-2.0

// Command docs-blinded-task is the operator harness for SDD-V0-014 and SDD-V0-015. It is never a
// read command: `examples` runs a page's declared Go examples through `go test`, and `score`
// scores one blinded maintainer run and writes the result record, whether it passes or fails.
//
//	go run ./tools/docs-blinded-task examples --root . --page docs/x.md
//	go run ./tools/docs-blinded-task score --tasks T --key K --responses R --out O
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/docmaintain"
)

func main() { os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr)) }

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: docs-blinded-task examples|score [flags]")
		return 2
	}
	var err error
	code := 0
	switch args[0] {
	case "examples":
		code, err = examples(ctx, args[1:], stdout)
	case "score":
		code, err = score(args[1:], stdout)
	default:
		err = fmt.Errorf("unknown subcommand %q", args[0])
	}
	if err != nil {
		fmt.Fprintln(stderr, "docs-blinded-task:", err)
		return 2
	}
	return code
}

func examples(ctx context.Context, args []string, stdout io.Writer) (int, error) {
	fs := flag.NewFlagSet("examples", flag.ContinueOnError)
	root := fs.String("root", ".", "repository root")
	page := fs.String("page", "", "Markdown page holding generated blocks and example declarations")
	goBin := fs.String("go", "go", "go executable")
	if err := fs.Parse(args); err != nil {
		return 0, err
	}
	if *page == "" || filepath.IsAbs(*page) {
		return 0, errors.New("--page must name a relative page under --root")
	}
	data, err := os.ReadFile(filepath.Join(*root, *page))
	if err != nil {
		return 0, err
	}
	report, err := docmaintain.CheckExamples(ctx, *root, data, docmaintain.GoTestRunner(*goBin))
	if err != nil {
		return 0, err
	}
	if err := writeJSON(stdout, report); err != nil {
		return 0, err
	}
	if report.Verdict != docmaintain.ExamplePass {
		return 1, nil
	}
	return 0, nil
}

func score(args []string, stdout io.Writer) (int, error) {
	fs := flag.NewFlagSet("score", flag.ContinueOnError)
	tasks := fs.String("tasks", "", "task set JSON")
	key := fs.String("key", "", "answer key JSON")
	responses := fs.String("responses", "", "responses JSON")
	out := fs.String("out", "", "result JSON to write (must not exist)")
	if err := fs.Parse(args); err != nil {
		return 0, err
	}
	if *tasks == "" || *key == "" || *responses == "" || *out == "" {
		return 0, errors.New("--tasks, --key, --responses and --out are required")
	}
	read := func(p string) ([]byte, error) { return os.ReadFile(p) }
	t, err := read(*tasks)
	if err != nil {
		return 0, err
	}
	k, err := read(*key)
	if err != nil {
		return 0, err
	}
	r, err := read(*responses)
	if err != nil {
		return 0, err
	}
	res, err := docmaintain.ScoreBlinded(t, k, r)
	if err != nil {
		return 0, err
	}
	f, err := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return 0, err
	}
	if err := writeJSON(f, res); err != nil {
		f.Close()
		return 0, err
	}
	if err := f.Close(); err != nil {
		return 0, err
	}
	fmt.Fprintf(stdout, "%s %d/%d %s\n", res.Verdict, res.Correct, res.Total, *out)
	if res.Verdict != "PASS" {
		return 1, nil
	}
	return 0, nil
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
