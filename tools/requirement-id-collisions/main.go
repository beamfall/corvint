// SPDX-License-Identifier: Apache-2.0
// requirement-id-collisions is the TLF-V0-886 check (V1-2909): it reads every
// local branch and detached worktree head not contained in the base ref and
// reports each requirement ID that two of those refs, or one of them and the
// base, define differently. It writes nothing.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

const (
	finding  = "REQUIREMENT_ID_COLLISION"
	maxHeads = 256
	specs    = "docs/specs/*.md"
)

// definition is the “ - `ID`: “ bullet of docs/specs/*.md.
var definition = regexp.MustCompile("^- `([A-Z][A-Z0-9]*-V[0-9]+-[0-9]+)`:(.*)$")

type head struct{ ref, commit string }

// site is one ID's defining bullet on one ref: its file and first line.
type site struct{ ref, file, text string }

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("requirement-id-collisions", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, `usage: requirement-id-collisions [--repo DIR] [--base REF]

Reads every local branch and detached worktree head not contained in --base
(default origin/main) and reports REQUIREMENT_ID_COLLISION for each
requirement ID (a "- `+"`ID`"+`:" bullet in docs/specs/*.md) that is new on two
refs with different first lines, defined in two files of one ref, or defined
on a ref in a different file than on the base (TLF-V0-886). Writes nothing.
Exit 0 when none, 1 on a collision, 2 on a read failure.

Example: go run ./tools/requirement-id-collisions --base origin/main
`)
	}
	repo := fs.String("repo", ".", "repository checkout to read")
	base := fs.String("base", "origin/main", "base ref the lanes are compared with")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "requirement-id-collisions: no positional argument is accepted")
		return 2
	}
	collisions, scanned, err := check(*repo, *base)
	if err != nil {
		fmt.Fprintf(stderr, "requirement-id-collisions: %v\n", err)
		return 2
	}
	for _, c := range collisions {
		fmt.Fprintln(stdout, c)
	}
	fmt.Fprintf(stdout, "scanned %d lane head(s) against %s:", len(scanned), *base)
	for _, h := range scanned {
		fmt.Fprintf(stdout, " %s@%s", h.ref, h.commit[:12])
	}
	fmt.Fprintln(stdout)
	if len(collisions) > 0 {
		fmt.Fprintf(stdout, "%d collision(s)\n", len(collisions))
		return 1
	}
	return 0
}

func check(repo, base string) ([]string, []head, error) {
	out, err := git(repo, "rev-parse", "--verify", "--end-of-options", base+"^{commit}")
	if err != nil {
		return nil, nil, err
	}
	baseCommit := strings.TrimSpace(out)
	heads, err := laneHeads(repo, baseCommit)
	if err != nil {
		return nil, nil, err
	}
	baseDefs, err := definitions(repo, "base", baseCommit)
	if err != nil {
		return nil, nil, err
	}
	var collisions []string
	report := func(id, why string, sites ...site) {
		line := finding + " " + id + " " + why
		for _, s := range sites {
			line += " " + s.ref + ":" + s.file
		}
		collisions = append(collisions, line)
	}
	fresh := map[string][]site{} // IDs the base lacks, by lane site
	for _, h := range heads {
		defs, err := definitions(repo, h.ref, h.commit)
		if err != nil {
			return nil, nil, err
		}
		for _, id := range sortedKeys(defs) {
			sites := defs[id]
			if files := distinctFiles(sites); len(files) > 1 {
				report(id, "defined in more than one file of one ref", sites...)
			}
			if b, ok := baseDefs[id]; ok {
				if b[0].file != sites[0].file {
					report(id, "defined in a different file than on the base", b[0], sites[0])
				}
				continue
			}
			fresh[id] = append(fresh[id], sites[0])
		}
	}
	for _, id := range sortedKeys(fresh) {
		sites := fresh[id]
		for i := 1; i < len(sites); i++ {
			if sites[i].file != sites[0].file || sites[i].text != sites[0].text {
				report(id, "new on two refs with different definitions", sites...)
				break
			}
		}
	}
	return collisions, heads, nil
}

// laneHeads lists local branches and detached worktree heads that share
// history with baseCommit (contain one of its root commits) and are not
// contained in it, each commit once, sorted by ref. A head of an unrelated
// lineage is no lane of the base and is not read.
func laneHeads(repo, baseCommit string) ([]head, error) {
	rootsOut, err := git(repo, "rev-list", "--max-parents=0", "--end-of-options", baseCommit)
	if err != nil {
		return nil, err
	}
	roots := strings.Fields(rootsOut)
	args := []string{"for-each-ref", "--format=%(objectname) %(refname)", "--no-merged=" + baseCommit}
	for _, r := range roots {
		args = append(args, "--contains="+r)
	}
	refs, err := git(repo, append(args, "refs/heads/")...)
	if err != nil {
		return nil, err
	}
	wt, err := git(repo, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var all []head
	for _, line := range strings.Split(refs, "\n") {
		if commit, ref, ok := strings.Cut(line, " "); ok {
			all = append(all, head{ref: ref, commit: commit})
		}
	}
	for _, block := range strings.Split(wt, "\n\n") {
		var path, commit string
		detached := false
		for _, line := range strings.Split(block, "\n") {
			switch k, v, _ := strings.Cut(line, " "); k {
			case "worktree":
				path = v
			case "HEAD":
				commit = v
			case "detached":
				detached = true
			}
		}
		if detached && commit != "" && sharesRoot(repo, commit, roots) {
			all = append(all, head{ref: "worktree:" + path, commit: commit})
		}
	}
	seen := map[string]bool{}
	var out []head
	for _, h := range all {
		if seen[h.commit] {
			continue
		}
		seen[h.commit] = true
		if _, err := git(repo, "merge-base", "--is-ancestor", "--end-of-options", h.commit, baseCommit); err == nil {
			continue
		}
		out = append(out, h)
	}
	if len(out) > maxHeads {
		return nil, fmt.Errorf("%d lane heads exceed %d", len(out), maxHeads)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ref < out[j].ref })
	return out, nil
}

// definitions reads every defining bullet of commit's docs/specs/*.md.
func definitions(repo, ref, commit string) (map[string][]site, error) {
	// Explicit format flags win over repository grep.* configuration, so the
	// output is always commit:path:text (TLF-V0-886).
	out, err := git(repo, "grep", "-I", "--no-color", "--full-name", "--no-line-number", "--no-column", "-E", "^- `[A-Z][A-Z0-9]*-V[0-9]+-[0-9]+`:", commit, "--", specs)
	if err != nil {
		if exit, ok := err.(*exitError); ok && exit.code == 1 && exit.stderr == "" {
			return map[string][]site{}, nil // no match
		}
		return nil, err
	}
	defs := map[string][]site{}
	prefix := commit + ":"
	for _, line := range strings.Split(out, "\n") {
		rest, ok := strings.CutPrefix(line, prefix)
		if !ok {
			continue
		}
		file, text, ok := strings.Cut(rest, ":")
		if !ok {
			continue
		}
		m := definition.FindStringSubmatch(text)
		if m == nil {
			continue
		}
		defs[m[1]] = append(defs[m[1]], site{ref: ref, file: file, text: strings.TrimSpace(m[2])})
	}
	return defs, nil
}

func sharesRoot(repo, commit string, roots []string) bool {
	for _, r := range roots {
		if _, err := git(repo, "merge-base", "--is-ancestor", "--end-of-options", r, commit); err == nil {
			return true
		}
	}
	return false
}

func distinctFiles(sites []site) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range sites {
		if !seen[s.file] {
			seen[s.file] = true
			out = append(out, s.file)
		}
	}
	return out
}

func sortedKeys(m map[string][]site) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

type exitError struct {
	code   int
	stderr string
}

func (e *exitError) Error() string {
	return fmt.Sprintf("git exited %d: %s", e.code, strings.TrimSpace(e.stderr))
}

// git runs one read-only git command under a sanitized environment that
// reads no global or system configuration, takes no optional locks, never
// lazily fetches a missing promisor object and may use no transport, so a
// partial clone fails the read instead of writing .git (TLF-V0-886).
func git(repo string, args ...string) (string, error) {
	c := exec.Command("git", append([]string{"-c", "core.fsmonitor=false", "-c", "credential.helper=", "-c", "protocol.allow=never"}, args...)...)
	c.Dir = repo
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			env = append(env, kv)
		}
	}
	c.Env = append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0",
		"GIT_NO_LAZY_FETCH=1", "GIT_ALLOW_PROTOCOL=", "GIT_NO_REPLACE_OBJECTS=1", "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	c.Stdout, c.Stderr = &stdout, &stderr
	if err := c.Run(); err != nil {
		if x, ok := err.(*exec.ExitError); ok {
			return "", &exitError{code: x.ExitCode(), stderr: stderr.String()}
		}
		return "", err
	}
	return stdout.String(), nil
}
