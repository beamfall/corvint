// SPDX-License-Identifier: Apache-2.0

// Command runner-qualification evaluates retained core experiments. It is an
// operator-invoked development tool, not a core read command or release binary.
package main

import (
	"context"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	q "github.com/Beamfall/corvint/internal/runnerqualification"
)

const maxBytes = 32 << 20
const usage = `Usage: runner-qualification replay -input FILE -checkout DIR -out FILE -budget D

Evaluate a retained core qualification experiment over locally pinned merges.
Required: capture input FILE, operator checkout DIR, new output FILE, and trusted
CI wall-time ceiling D (for example 20m). No default ceiling is invented.
Example: runner-qualification replay -input captures.json -checkout /src/project -out qualification.json -budget 20m

Input is the internal runnerqualification.Replay value. The tool verifies Git
pins, regenerates the seeded draw and evaluates every retained FULL result. It
writes an UNADMITTED candidate record; it does not execute tests, generate live
captures, schedule CI, or confer narrowing authority. A live core Driver must
collect the captures through its runner and mutation pieces first.
Exit 0: all requested classes passed; 1: unqualified; 2: invalid invocation or I/O.
Use runner-qualification help --json for the command tree.
`

// Kept local to this unshipped development tool; no core command-tree change.
const tree = `{"binary":"runner-qualification","commands":[{"path":["replay"],"usage":"runner-qualification replay -input FILE -checkout DIR -out FILE -budget D","flags":["-input","-checkout","-out","-budget"],"effect":"write-unadmitted-qualification-record"}]}`

func main() { os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr)) }
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "help") {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if len(args) == 2 && args[0] == "help" && args[1] == "--json" {
		fmt.Fprintln(stdout, tree)
		return 0
	}
	if len(args) == 0 || args[0] != "replay" {
		fmt.Fprint(stderr, usage)
		return 2
	}
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	fs.SetOutput(stderr)
	input := fs.String("input", "", "retained experiment JSON")
	checkout := fs.String("checkout", "", "operator checkout path")
	outPath := fs.String("out", "", "new unadmitted record path")
	budget := fs.Duration("budget", 0, "trusted CI cost ceiling")
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 || *input == "" || *checkout == "" || *outPath == "" || *budget <= 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	refuse := func(err error) int { fmt.Fprintf(stderr, "runner-qualification: %v\n", err); return 2 }
	f, err := os.Open(*input)
	if err != nil {
		return refuse(err)
	}
	b, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	closeErr := f.Close()
	if err != nil {
		return refuse(err)
	}
	if closeErr != nil {
		return refuse(closeErr)
	}
	if len(b) > maxBytes {
		return refuse(fmt.Errorf("input-capped"))
	}
	var replay q.Replay
	// Retained captures use encoding/json's numeric nanoseconds. Keep that
	// representation while preserving v2's strict member and duplicate checks.
	if err = jsonv2.Unmarshal(b, &replay, json.FormatDurationAsNano(true), jsonv2.RejectUnknownMembers(true)); err != nil {
		return refuse(err)
	}
	root, err := filepath.Abs(*checkout)
	if err != nil {
		return refuse(err)
	}
	replay.Request.Checkout = root
	ctx, cancel := context.WithTimeout(ctx, *budget)
	defer cancel()
	record := q.ReplayRetained(ctx, replay, verifyPins)
	if record.Budget > *budget {
		record.Complete = false
		record.Reasons = append(record.Reasons, "qualification-cost-ceiling-exceeded")
	}
	// Output is an explicit operator write and never overwrites an existing
	// evidence file (including the input). It is not an admission artifact.
	f, err = os.OpenFile(*outPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return refuse(err)
	}
	err = json.NewEncoder(f).Encode(record)
	closeErr = f.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(*outPath)
		if err != nil {
			return refuse(err)
		}
		return refuse(closeErr)
	}
	passed := record.Complete && len(record.Reasons) == 0 && len(record.Classes) > 0
	for _, c := range record.Classes {
		fmt.Fprintf(stdout, "%s: trials=%d unkilled=%d misses=%d upper95=%.9g population=%d passed=%t (population only; UNADMITTED)\n", c.Class, c.Trials, c.Unkilled, len(c.Misses), c.Upper95, c.PopulationSize, c.Passed)
		passed = passed && c.Passed
	}
	if passed {
		return 0
	}
	fmt.Fprintf(stdout, "unqualified: %s; record=%s\n", strings.Join(record.Reasons, ","), *outPath)
	return 1
}

type boundedOutput struct{ b strings.Builder }

func (b *boundedOutput) Write(p []byte) (int, error) {
	if b.b.Len()+len(p) > 4096 {
		return 0, fmt.Errorf("git-output-capped")
	}
	return b.b.Write(p)
}
func gitOutput(ctx context.Context, root string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "git", append([]string{"-C", root, "--no-replace-objects"}, args...)...)
	var out boundedOutput
	c.Stdout = &out
	c.Stderr = io.Discard
	if err := c.Run(); err != nil {
		return "", fmt.Errorf("git-pin-check-failed:%w", err)
	}
	return strings.TrimSpace(out.b.String()), nil
}
func verifyPins(ctx context.Context, d q.Declaration, root string, merges []q.Merge) error {
	commit, err := gitOutput(ctx, root, "rev-parse", "--verify", d.Corpus.Commit+"^{commit}")
	if err != nil {
		return err
	}
	if commit != d.Corpus.Commit {
		return fmt.Errorf("corpus-pin-mismatch")
	}
	for _, m := range merges {
		parents, err := gitOutput(ctx, root, "show", "-s", "--format=%P", m.Commit)
		if err != nil {
			return err
		}
		fields := strings.Fields(parents)
		if len(fields) != 2 || fields[0] != m.Base || m.Head != m.Commit {
			return fmt.Errorf("merge-parent-pins-mismatch")
		}
		if _, err = gitOutput(ctx, root, "merge-base", "--is-ancestor", m.Commit, d.Corpus.Commit); err != nil {
			return fmt.Errorf("merge-outside-pinned-corpus:%w", err)
		}
	}
	return nil
}
