// SPDX-License-Identifier: Apache-2.0
// declaration-compat-check fails when a closed-shape .corvint planner declaration
// holds a member that the same file lacks at a CI planner pin (AFP-V0-710,
// docs/specs/affected-plan-v0.md). A planner built from that pin refuses the
// unknown member, so it drops the whole declaration and falls back from
// narrowing; this check names the file, position, member and pin first.
//
//	declaration-compat-check [--root DIR] [--workflow FILE]
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// closed maps each planner declaration to the object positions whose member set
// its decoder closes: "" is the top-level object, and "*" stands for every
// element of an array or every value of a map.
var closed = map[string][]string{
	".corvint/documentation-paths.json":         {""},
	".corvint/existence-readers.json":           {"", "/entries/*"},
	".corvint/gated-root-locators.json":         {"", "/packages/*"},
	".corvint/runtime-readers.json":             {"", "/entries/*"},
	".corvint/shell-foreign-callers.json":       {"", "/scripts/*"},
	".corvint/shell-read-scopes.json":           {"", "/tests/*"},
	".corvint/test-binary-consumers.json":       {""},
	".corvint/test-binary-execs.json":           {""},
	".corvint/test-nested-sandboxes.json":       {""},
	".corvint/test-read-scope-literals.json":    {""},
	".corvint/test-read-scopes.json":            {""},
	".corvint/unbounded-reader-exclusions.json": {""},
	".corvint/unbounded-readers.json":           {""},
}

// notPlanner lists the .corvint/*.json files no planner or selector reads.
var notPlanner = map[string]string{
	".corvint/dogfood-report.json":    "the dogfood run report",
	".corvint/evidence-reach.json":    "read by `corvint runs` (internal/evidencereach), not by the planner",
	".corvint/work-queue-policy.json": "the Tasks work-queue policy",
}

// changeEvidence is the change-evidence map a dogfood run tracks between its
// bind and its seal; no planner reads it as a declaration.
const changeEvidence = ".corvint/change.cem.json"

// tracked lists the .corvint/*.json files Git tracks directly under .corvint:
// what a pinned planner reads in CI. A dogfood run's ignored local outputs are
// never candidates.
func tracked(root string) ([]string, error) {
	out, err := exec.Command("git", "-C", root, "ls-files", "-z", "--", ".corvint").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files .corvint: %w", err)
	}
	var names []string
	for _, name := range strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		if strings.HasPrefix(name, ".corvint/") && !strings.Contains(strings.TrimPrefix(name, ".corvint/"), "/") && strings.HasSuffix(name, ".json") {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err == nil {
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	return names, nil
}

var pinPattern = regexp.MustCompile(`(?m)^\s+(CORVINT_PR_TOOL_SOURCE|CORVINT_TEST_SHADOW_SOURCE): "([0-9a-f]{40})"`)

type pin struct{ name, commit string }

// pins returns the distinct non-empty planner pins of the workflow, in order.
func pins(workflow []byte) []pin {
	var out []pin
	seen := map[string]bool{}
	for _, m := range pinPattern.FindAllSubmatch(workflow, -1) {
		commit := string(m[2])
		if seen[commit] {
			for i := range out {
				if out[i].commit == commit {
					out[i].name += "+" + string(m[1])
				}
			}
			continue
		}
		seen[commit] = true
		out = append(out, pin{string(m[1]), commit})
	}
	return out
}

// members returns the member names of every object at position in doc.
func members(doc any, position string) map[string]bool {
	nodes := []any{doc}
	for _, segment := range strings.Split(strings.TrimPrefix(position, "/"), "/") {
		if segment == "" {
			continue
		}
		var next []any
		for _, node := range nodes {
			switch value := node.(type) {
			case map[string]any:
				if segment == "*" {
					for _, child := range value {
						next = append(next, child)
					}
				} else if child, ok := value[segment]; ok {
					next = append(next, child)
				}
			case []any:
				if segment == "*" {
					next = append(next, value...)
				}
			}
		}
		nodes = next
	}
	out := map[string]bool{}
	for _, node := range nodes {
		if object, ok := node.(map[string]any); ok {
			for key := range object {
				out[key] = true
			}
		}
	}
	return out
}

func decode(body []byte) (any, error) {
	var doc any
	err := json.Unmarshal(body, &doc)
	return doc, err
}

func profile(doc any) string {
	if object, ok := doc.(map[string]any); ok {
		if value, ok := object["profile"].(string); ok {
			return value
		}
	}
	return ""
}

// atPin returns the file's bytes at commit, nil when the file is absent there.
func atPin(root, commit, name string) ([]byte, error) {
	command := exec.Command("git", "-C", root, "cat-file", "-e", commit+"^{commit}")
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("pin %s is not in this clone; fetch it (git fetch origin %s)", commit, commit)
	}
	show := exec.Command("git", "-C", root, "show", commit+":"+name)
	var stderr bytes.Buffer
	show.Stderr = &stderr
	out, err := show.Output()
	if err != nil {
		if strings.Contains(stderr.String(), "does not exist") || strings.Contains(stderr.String(), "exists on disk, but not in") {
			return nil, nil
		}
		return nil, fmt.Errorf("git show %s:%s: %v: %s", commit, name, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// check returns one finding per member, unclassified file or unreadable input.
func check(root, workflowPath string) ([]string, error) {
	workflow, err := os.ReadFile(filepath.Join(root, workflowPath))
	if err != nil {
		return nil, err
	}
	pinned := pins(workflow)
	names, err := tracked(root)
	if err != nil {
		return nil, err
	}
	var findings []string
	for _, name := range names {
		absolute := filepath.Join(root, filepath.FromSlash(name))
		if _, ok := notPlanner[name]; ok || name == changeEvidence {
			continue
		}
		positions, ok := closed[name]
		if !ok {
			findings = append(findings, fmt.Sprintf("%s is neither a closed planner declaration nor listed as no planner input; classify it in tools/declaration-compat-check (AFP-V0-710)", name))
			continue
		}
		body, err := os.ReadFile(absolute)
		if err != nil {
			return nil, err
		}
		now, err := decode(body)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		for _, p := range pinned {
			old, err := atPin(root, p.commit, name)
			if err != nil {
				return nil, err
			}
			if old == nil {
				continue
			}
			then, err := decode(old)
			if err != nil {
				return nil, fmt.Errorf("%s at %s: %w", name, p.commit, err)
			}
			if profile(then) != profile(now) {
				continue
			}
			for _, position := range positions {
				known := members(then, position)
				var added []string
				for member := range members(now, position) {
					if !known[member] {
						added = append(added, member)
					}
				}
				sort.Strings(added)
				where := position
				if where == "" {
					where = "the top level"
				}
				for _, member := range added {
					findings = append(findings, fmt.Sprintf("%s at %s: member %q is absent at pin %s (%s), whose planner refuses it and stops narrowing; put it in a new declaration file, which an older planner never reads, or give the file a new profile value (AFP-V0-710)",
						name, where, member, p.commit, p.name))
				}
			}
		}
	}
	sort.Strings(findings)
	return findings, nil
}

func run(root, workflow string, stdout, stderr io.Writer) int {
	findings, err := check(root, workflow)
	if err != nil {
		fmt.Fprintln(stderr, "declaration-compat-check:", err)
		return 1
	}
	for _, finding := range findings {
		fmt.Fprintln(stderr, "declaration-compat-check: FAIL "+finding)
	}
	if len(findings) > 0 {
		return 1
	}
	fmt.Fprintln(stdout, "declaration-compat-check: ok")
	return 0
}

func main() {
	root := flag.String("root", ".", "repository root")
	workflow := flag.String("workflow", ".github/workflows/ci.yml", "workflow holding the planner pins, relative to the root")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: declaration-compat-check [--root DIR] [--workflow FILE]")
		os.Exit(2)
	}
	os.Exit(run(*root, *workflow, os.Stdout, os.Stderr))
}
