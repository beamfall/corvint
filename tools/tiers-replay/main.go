// Command tiers-replay re-measures the AFP-V0-480 shadow tiers over historical
// merges (AFP-V0-991, V1-3664). For each of the last N first-parent merges of
// a ref it exports the merge's own tree, runs this checkout's
// .github/forward-rows/tiers.sh there against the repository's object store,
// so `go list` reads that merge's packages while tiers.jq is the one under
// test, and writes one JSON line per merge. It never checks out, commits or
// writes repository state; each export is removed after its merge.
//
// Usage: go run ./tools/tiers-replay [-n 20] [-ref origin/main] [-repo .] [-out FILE]
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// row is one merge's replayed tier decision.
type row struct {
	Merge          string   `json:"merge"`
	Base           string   `json:"base"`
	Subject        string   `json:"subject"`
	ChangedFiles   int      `json:"changedFiles"`
	Decision       string   `json:"decision"`
	Changed        int      `json:"changed"`
	Hop1           int      `json:"hop1"`
	Hop2           int      `json:"hop2"`
	Universe       int      `json:"universe"`
	Full           []string `json:"full,omitempty"`
	TierIncomplete []string `json:"tierIncomplete,omitempty"`
	Millis         *int64   `json:"millis"`
}

// summary closes the output: how many merges each decision covered.
type summary struct {
	Summary  bool           `json:"summary"`
	Ref      string         `json:"ref"`
	Merges   int            `json:"merges"`
	Decision map[string]int `json:"decisions"`
	Seconds  float64        `json:"seconds"`
}

func main() {
	n := flag.Int("n", 20, "number of first-parent merges to replay")
	ref := flag.String("ref", "origin/main", "ref whose first-parent merges are replayed")
	repo := flag.String("repo", ".", "repository root")
	out := flag.String("out", "", "JSONL output file (default stdout)")
	flag.Parse()
	if *n < 1 || *n > 500 || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: tiers-replay [-n 1..500] [-ref REF] [-repo DIR] [-out FILE]")
		os.Exit(2)
	}
	w := io.Writer(os.Stdout)
	if *out != "" {
		file, err := os.Create(*out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "tiers-replay:", err)
			os.Exit(2)
		}
		defer file.Close()
		w = file
	}
	if err := replay(*repo, *ref, *n, w); err != nil {
		fmt.Fprintln(os.Stderr, "tiers-replay:", err)
		os.Exit(1)
	}
}

// replay writes one row per merge, newest first, then the summary line.
func replay(repo, ref string, n int, w io.Writer) error {
	started := time.Now()
	root, err := git(repo, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	common, err := git(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	script := filepath.Join(root, ".github", "forward-rows", "tiers.sh")
	if _, err := os.Stat(script); err != nil {
		return err
	}
	list, err := git(root, "rev-list", "--first-parent", "--merges", "-n", fmt.Sprint(n), ref)
	if err != nil {
		return err
	}
	merges := strings.Fields(list)
	if len(merges) == 0 {
		return fmt.Errorf("%s has no first-parent merge", ref)
	}
	encoder := json.NewEncoder(w)
	decisions := map[string]int{}
	for _, merge := range merges {
		r, err := replayMerge(root, common, script, merge)
		if err != nil {
			return fmt.Errorf("merge %s: %w", merge, err)
		}
		decisions[r.Decision]++
		if err := encoder.Encode(r); err != nil {
			return err
		}
	}
	return encoder.Encode(summary{Summary: true, Ref: ref, Merges: len(merges), Decision: decisions, Seconds: time.Since(started).Round(time.Millisecond).Seconds()})
}

// replayMerge exports merge's tree and runs tiers.sh over merge^1..merge in it.
func replayMerge(root, common, script, merge string) (row, error) {
	base, err := git(root, "rev-parse", merge+"^1")
	if err != nil {
		return row{}, err
	}
	subject, _ := git(root, "log", "-1", "--format=%s", merge)
	changed, err := git(root, "diff", "--name-only", "--no-renames", base, merge, "--")
	if err != nil {
		return row{}, err
	}
	dir, err := os.MkdirTemp("", "tiers-replay-")
	if err != nil {
		return row{}, err
	}
	defer os.RemoveAll(dir)
	tree := filepath.Join(dir, "tree")
	if err := os.Mkdir(tree, 0o700); err != nil {
		return row{}, err
	}
	archive := exec.Command("git", "-C", root, "archive", "--format=tar", merge)
	extract := exec.Command("tar", "-x", "-C", tree)
	pipe, err := archive.StdoutPipe()
	if err != nil {
		return row{}, err
	}
	extract.Stdin = pipe
	if err := extract.Start(); err != nil {
		return row{}, err
	}
	if err := archive.Run(); err != nil {
		return row{}, fmt.Errorf("git archive: %w", err)
	}
	if err := extract.Wait(); err != nil {
		return row{}, fmt.Errorf("tar: %w", err)
	}
	output := filepath.Join(dir, "tiers.json")
	command := exec.Command("sh", script, base, merge, output)
	command.Dir = tree
	command.Env = append(os.Environ(), "GIT_DIR="+common)
	if text, err := command.CombinedOutput(); err != nil {
		return row{}, fmt.Errorf("tiers.sh: %v: %s", err, bytes.TrimSpace(text))
	}
	raw, err := os.ReadFile(output)
	if err != nil {
		return row{}, err
	}
	return decide(merge, base, subject, len(strings.Fields(changed)), raw)
}

// decide reads one tiers.sh document into a row: FULL with its reasons,
// INCOMPLETE when the tiers were not measured, else NARROWED with the tier
// sizes.
func decide(merge, base, subject string, changedFiles int, raw []byte) (row, error) {
	var document struct {
		Tiers *struct {
			Changed, Hop1, Hop2 json.RawMessage
			Full                []string
			Universe            int
		}
		TierIncomplete []string
		Millis         *int64
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		return row{}, err
	}
	r := row{Merge: merge, Base: base, Subject: subject, ChangedFiles: changedFiles, TierIncomplete: document.TierIncomplete, Millis: document.Millis}
	switch {
	case document.Tiers == nil || len(document.TierIncomplete) != 0:
		r.Decision = "INCOMPLETE"
	case len(document.Tiers.Full) != 0:
		r.Decision, r.Full, r.Universe = "FULL", document.Tiers.Full, document.Tiers.Universe
	default:
		r.Decision, r.Universe = "NARROWED", document.Tiers.Universe
		for _, tier := range []struct {
			raw   json.RawMessage
			count *int
		}{{document.Tiers.Changed, &r.Changed}, {document.Tiers.Hop1, &r.Hop1}, {document.Tiers.Hop2, &r.Hop2}} {
			var packages []string
			if err := json.Unmarshal(tier.raw, &packages); err != nil {
				return row{}, errors.New("a tier is neither FULL nor a package list")
			}
			*tier.count = len(packages)
		}
	}
	return r, nil
}

// git runs one git command in dir and returns its trimmed stdout.
func git(dir string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, bytes.TrimSpace(stderr.Bytes()))
	}
	return strings.TrimSpace(string(output)), nil
}
