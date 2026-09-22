package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestGateExcludesCompanionReleaseGate binds PUB-V0-002's first sentence: "The existing native
// CLI archive gate MUST remain independent." Nothing checked that `make gate` stays free of
// `companion-release-gate` (which rebuilds the bundled binaries twice each and is deliberately opt-in
// per docs/specs/public-release-v0.md's Acceptance and rollback section); a Makefile edit that
// folded the two together would have passed every other check. The check walks every target
// `gate` reaches, so a prerequisite of a prerequisite, or a recipe that runs the companion gate
// itself, also fails.
func TestGateExcludesCompanionReleaseGate(t *testing.T) {
	t.Run("PUB-V0-002 core-gate-independence", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
		if err != nil {
			t.Fatal(err)
		}
		makefile := string(data)

		if !strings.Contains(makefile, "\ncompanion-release-gate:") {
			t.Fatal("Makefile no longer defines a standalone `companion-release-gate:` target")
		}
		prerequisites, recipes := makefileRules(makefile)
		if _, ok := prerequisites["gate"]; !ok {
			t.Fatal("Makefile has no `gate:` target line")
		}

		seen := map[string]bool{"gate": true}
		queue := []string{"gate"}
		for len(queue) > 0 {
			target := queue[0]
			queue = queue[1:]
			if strings.Contains(recipes[target], "companion-release-gate") {
				t.Fatalf("`gate` reaches %s, whose recipe runs the companion release gate, breaking archive-gate independence:\n%s", target, recipes[target])
			}
			for _, prerequisite := range prerequisites[target] {
				if prerequisite == "companion-release-gate" {
					t.Fatalf("`gate` reaches %s, which depends on companion-release-gate, breaking archive-gate independence", target)
				}
				if !seen[prerequisite] {
					seen[prerequisite] = true
					queue = append(queue, prerequisite)
				}
			}
		}
	})
}

// makefileRules returns each explicit rule's prerequisites and its tab-indented recipe text.
func makefileRules(makefile string) (map[string][]string, map[string]string) {
	rule := regexp.MustCompile(`^([A-Za-z0-9_.-]+):(?:[^=]|$)(.*)$`)
	prerequisites := map[string][]string{}
	recipes := map[string]string{}
	current := ""
	for _, line := range strings.Split(makefile, "\n") {
		if strings.HasPrefix(line, "\t") {
			recipes[current] += line + "\n"
			continue
		}
		match := rule.FindStringSubmatch(line)
		if match == nil {
			current = ""
			continue
		}
		current = match[1]
		prerequisites[current] = append(prerequisites[current], strings.Fields(strings.TrimPrefix(line, current+":"))...)
	}
	return prerequisites, recipes
}
