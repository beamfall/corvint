// SPDX-License-Identifier: Apache-2.0

// Command shell-test-select prints the `make shell-test` steps an affected plan
// selects, one make target per line (AFP-V0-2401, V1-3489). The PR test
// selection runs Go packages only; the script self-tests registered as
// `shell-test` steps ran only in `make gate`, so a change a self-test exercises
// reached main untested. This maps each selected shell test unit
// (`shell:script/NAME_test.sh`) to the step whose recipe runs it, so a change
// runs the self-tests it can affect and no others.
//
// It fails closed: every step is printed (FULL, with the reason on stderr) when
// the plan is unreadable, is not a successful affected-plan/1, names the
// Makefile as dirty (every step runs through it) or carries any unknown outside
// the shell allowlist: a LANGUAGE_FRONTIER gateselect.AllowedUnknown admits other than a `shell:`
// one (only shell:foreign-invocation-unmodelled, which bears on non-shell callers, is kept), a
// NO_SELECTABLE_TEST (no step tests such a unit, FULL included) and a
// well-formed UNSELECTED_BINARY_CONSUMER (no shell step runs it). An
// UNOWNED_DIRTY_PATH is not admitted: a shell test may read an unowned path by
// a word no plugin resolves. A step whose recipe names no `script/*_test.*`
// file always runs. A selected shell test that no step runs is named on
// stderr, since FULL would not run it either.
//
//	shell-test-select --plan PLAN.json [--makefile Makefile]
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/Beamfall/corvint/internal/gateselect"
)

const usage = `usage: shell-test-select --plan PLAN.json [--makefile Makefile]

Prints, one per line, the make targets of the shell-test steps the affected
plan (corvint affected --base BASE output) selects; every step when the plan
cannot narrow them, with the reason on stderr (AFP-V0-2401).

Example:
  corvint affected --base "$(git merge-base HEAD origin/main)" > plan.json
  make $(shell-test-select --plan plan.json)
`

const maxPlanBytes = 64 << 20

// Step is one shell-test prerequisite and the script tests its recipe runs.
type Step struct {
	Target  string
	Scripts []string
}

var scriptTest = regexp.MustCompile(`script/[A-Za-z0-9_.-]+_test\.[A-Za-z0-9]+`)

// ReadSteps parses the `shell-test:` prerequisite list and each listed
// target's recipe lines from a Makefile.
func ReadSteps(r io.Reader) ([]Step, error) {
	lines := []string{}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	var targets []string
	for index := 0; index < len(lines); index++ {
		if !strings.HasPrefix(lines[index], "shell-test:") {
			continue
		}
		for line := strings.TrimPrefix(lines[index], "shell-test:"); ; {
			continued := strings.HasSuffix(strings.TrimSpace(line), "\\")
			for _, word := range strings.Fields(strings.TrimSuffix(strings.TrimSpace(line), "\\")) {
				if strings.HasPrefix(word, "#") {
					break
				}
				targets = append(targets, word)
			}
			if !continued || index+1 >= len(lines) {
				break
			}
			index++
			line = lines[index]
		}
		break
	}
	if len(targets) == 0 {
		return nil, errors.New("the Makefile has no shell-test prerequisites")
	}
	steps := make([]Step, 0, len(targets))
	for _, target := range targets {
		step := Step{Target: target}
		for index, line := range lines {
			if !strings.HasPrefix(line, target+":") {
				continue
			}
			for _, recipe := range lines[index+1:] {
				if !strings.HasPrefix(recipe, "\t") {
					break
				}
				step.Scripts = append(step.Scripts, scriptTest.FindAllString(recipe, -1)...)
			}
		}
		sort.Strings(step.Scripts)
		steps = append(steps, step)
	}
	return steps, nil
}

type plan struct {
	OK      bool   `json:"ok"`
	Profile string `json:"profile"`
	Plan    *struct {
		Dirty    []string `json:"dirty"`
		Selected []struct {
			UnitID string `json:"unitId"`
		} `json:"selected"`
		Unknown []struct {
			Reason string `json:"reason"`
			Detail string `json:"detail"`
		} `json:"unknown"`
	} `json:"plan"`
}

// Select returns the steps to run, each selected shell test no step runs, and
// the FULL reason ("" when narrowed).
func Select(raw []byte, steps []Step) (run []string, unstepped []string, full string) {
	all := func(reason string) ([]string, []string, string) {
		targets := make([]string, 0, len(steps))
		for _, step := range steps {
			targets = append(targets, step.Target)
		}
		return targets, nil, reason
	}
	var p plan
	if err := json.Unmarshal(raw, &p); err != nil {
		return all("the plan is unreadable: " + err.Error())
	}
	if !p.OK || p.Profile != "affected-plan/1" || p.Plan == nil {
		return all("the plan is not a successful affected-plan/1")
	}
	// A nil slice means either a missing array or JSON null, neither of which is
	// evidence that the planner found no entries (AFP-V0-2401).
	if p.Plan.Dirty == nil || p.Plan.Selected == nil || p.Plan.Unknown == nil {
		return all("the plan is incomplete: dirty, selected and unknown must be arrays")
	}
	for _, dirty := range p.Plan.Dirty {
		if !validPlanPath(dirty) {
			return all("the plan has a malformed dirty path")
		}
		if dirty == "Makefile" {
			return all("the Makefile is dirty, and every shell-test step runs through it")
		}
	}
	for _, entry := range p.Plan.Unknown {
		if entry.Reason == "" || entry.Detail == "" {
			return all("the plan has an incomplete unknown entry")
		}
		switch {
		case entry.Reason == "LANGUAGE_FRONTIER" && strings.HasPrefix(entry.Detail, "shell:") && entry.Detail != "shell:foreign-invocation-unmodelled":
			// A shell frontier (an invalid or changed shell declaration, an unreadable or
			// dynamically sourced script) can hide a shell test's own edge or input.
			return all("the plan names the shell frontier " + entry.Detail + ", which may hide a shell test's edge or input")
		case entry.Reason == "LANGUAGE_FRONTIER" && gateselect.AllowedUnknown(entry.Reason, entry.Detail):
		case entry.Reason == "NO_SELECTABLE_TEST":
		case entry.Reason == "UNSELECTED_BINARY_CONSUMER":
			if _, ok := gateselect.BinaryConsumer(entry.Reason, entry.Detail); !ok {
				return all("the plan names a malformed UNSELECTED_BINARY_CONSUMER " + fmt.Sprintf("%q", entry.Detail))
			}
		default:
			return all(fmt.Sprintf("the plan names the unknown %s %s, which may hide a shell test's read", entry.Reason, entry.Detail))
		}
	}
	selected := map[string]bool{}
	for _, unit := range p.Plan.Selected {
		language, file, ok := strings.Cut(unit.UnitID, ":")
		if !ok || !unitLanguage.MatchString(language) || !validPlanPath(file) {
			return all("the plan has a missing or malformed selected unitId")
		}
		selected[file] = true
	}
	stepped := map[string]bool{}
	for _, step := range steps {
		chosen := len(step.Scripts) == 0
		for _, script := range step.Scripts {
			stepped[script] = true
			chosen = chosen || selected[script]
		}
		if chosen {
			run = append(run, step.Target)
		}
	}
	for _, unit := range p.Plan.Selected {
		if language, file, ok := strings.Cut(unit.UnitID, ":"); ok && language == "shell" && scriptTest.MatchString(file) && !stepped[file] {
			unstepped = append(unstepped, file)
		}
	}
	sort.Strings(unstepped)
	return run, unstepped, ""
}

var unitLanguage = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func validPlanPath(value string) bool {
	return fs.ValidPath(value) && !strings.ContainsRune(value, '\\') &&
		!strings.ContainsFunc(value, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) })
}

func main() {
	os.Exit(execute(os.Args[1:], os.Stdout, os.Stderr))
}

func execute(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("shell-test-select", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	planPath := flags.String("plan", "", "affected-plan/1 JSON")
	makefile := flags.String("makefile", "Makefile", "Makefile holding the shell-test list")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *planPath == "" {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return 0
		}
		fmt.Fprint(stderr, usage)
		return 2
	}
	file, err := os.Open(*makefile)
	if err != nil {
		fmt.Fprintf(stderr, "shell-test-select: %v\n", err)
		return 2
	}
	steps, err := ReadSteps(file)
	file.Close()
	if err != nil {
		fmt.Fprintf(stderr, "shell-test-select: %v\n", err)
		return 2
	}
	raw, err := readBounded(*planPath)
	if err != nil {
		raw = nil // an unreadable plan runs every step
	}
	run, unstepped, full := Select(raw, steps)
	if full != "" {
		fmt.Fprintf(stderr, "shell-test-select: FULL %d of %d steps: %s\n", len(run), len(steps), full)
	} else {
		fmt.Fprintf(stderr, "shell-test-select: NARROWED %d of %d steps\n", len(run), len(steps))
	}
	for _, script := range unstepped {
		fmt.Fprintf(stderr, "shell-test-select: selected %s is no shell-test step; run it by its own driver\n", script)
	}
	for _, target := range run {
		fmt.Fprintln(stdout, target)
	}
	return 0
}

func readBounded(name string) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxPlanBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxPlanBytes {
		return nil, fmt.Errorf("%s exceeds %d bytes", name, maxPlanBytes)
	}
	return raw, nil
}
