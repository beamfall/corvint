// SPDX-License-Identifier: Apache-2.0

// Package integrations holds static checks for the host integration packages under
// protocol/integrations (docs/specs/spec-toolchain-adapters-v0.md, STA-V0-008 and STA-V0-009).
// The upstream install and validation run in the spec-toolchain-conformance job (STA-V0-011).
package integrations

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func readFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.FromSlash(rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func requireLines(t *testing.T, name, text string, lines ...string) {
	t.Helper()
	have := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		have[strings.TrimRight(line, " \r")] = true
	}
	for _, line := range lines {
		if !have[line] {
			t.Errorf("%s: missing line %q", name, line)
		}
	}
}

// shellLines returns the non-empty lines of every ```bash fenced block in a Markdown file.
func shellLines(text string) []string {
	var out []string
	inBlock := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case !inBlock && trimmed == "```bash":
			inBlock = true
		case inBlock && trimmed == "```":
			inBlock = false
		case inBlock && trimmed != "":
			out = append(out, trimmed)
		}
	}
	return out
}

var readOnlyCommand = regexp.MustCompile(`^(corvint( --root "\$REPO_ROOT")? (context|affected|review) |\.specify/scripts/bash/check-prerequisites\.sh --json --paths-only$)`)

// mutating names commands or verbs an advisory read-only step must never instruct.
var mutating = regexp.MustCompile(`git (commit|add|push|stash|reset|checkout)|corvint (index|record|dogfood|cem|ocm|lease|step)\b|--learn|rm -`)

func checkReadOnlyInstructions(t *testing.T, name, text string) {
	t.Helper()
	lines := shellLines(text)
	if len(lines) == 0 {
		t.Errorf("%s: no bash steps", name)
	}
	for _, line := range lines {
		if !readOnlyCommand.MatchString(line) {
			t.Errorf("%s: step %q is not an allowed read-only command", name, line)
		}
	}
	if m := mutating.FindString(text); m != "" {
		t.Errorf("%s: instructs mutating command %q", name, m)
	}
	for _, phrase := range []string{"NOT_PRODUCED", "never", "commit"} {
		if !strings.Contains(text, phrase) {
			t.Errorf("%s: missing %q in the read-only rules", name, phrase)
		}
	}
}

func checkFrontmatter(t *testing.T, name, text string, keys ...string) {
	t.Helper()
	if !strings.HasPrefix(text, "---\n") {
		t.Fatalf("%s: does not start with YAML frontmatter", name)
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		t.Fatalf("%s: unterminated frontmatter", name)
	}
	front := text[4 : 4+end+1]
	for _, key := range keys {
		if !strings.Contains(front, "\n"+key+": ") && !strings.HasPrefix(front, key+": ") {
			t.Errorf("%s: frontmatter lacks %q", name, key)
		}
	}
	if !strings.Contains(text, "SPDX-License-Identifier: Apache-2.0") {
		t.Errorf("%s: missing Apache-2.0 SPDX identifier", name)
	}
}

func TestSTAV0008_SpecKitExtensionManifest(t *testing.T) {
	const dir = "speckit/corvint/"
	manifest := readFile(t, dir+"extension.yml")
	requireLines(t, "extension.yml", manifest,
		"# SPDX-License-Identifier: Apache-2.0",
		`schema_version: "1.0"`,
		"extension:",
		"  id: corvint",
		"  license: Apache-2.0",
		"  effect: read-only",
		"requires:",
		`  speckit_version: ">=1.2.1,<2"`,
		"  tools:",
		"    - name: corvint",
		"provides:",
		"  commands:",
		"    - name: speckit.corvint.context",
		"      file: commands/speckit.corvint.context.md",
		"    - name: speckit.corvint.review",
		"      file: commands/speckit.corvint.review.md",
		"hooks:",
		"  before_implement:",
		"    command: speckit.corvint.context",
		"  after_implement:",
		"    command: speckit.corvint.review",
	)
	// Both hooks are optional: no `optional: false` anywhere and one `optional: true` per hook.
	if strings.Contains(manifest, "optional: false") || strings.Count(manifest, "    optional: true\n") != 2 {
		t.Errorf("extension.yml: both hooks must be optional: true")
	}
	if strings.Contains(manifest, "read-write") {
		t.Errorf("extension.yml: effect must be read-only")
	}
	commandName := regexp.MustCompile(`^\s+- name: (.+)$`)
	for _, line := range strings.Split(manifest, "\n") {
		if m := commandName.FindStringSubmatch(line); m != nil && m[1] != "corvint" {
			if !regexp.MustCompile(`^speckit\.corvint\.[a-z0-9-]+$`).MatchString(m[1]) {
				t.Errorf("extension.yml: command %q outside the speckit.corvint namespace", m[1])
			}
		}
	}
	for _, cmd := range []string{"context", "review"} {
		name := "speckit.corvint." + cmd + ".md"
		text := readFile(t, dir+"commands/"+name)
		checkFrontmatter(t, name, text, "description")
		checkReadOnlyInstructions(t, name, text)
		if !strings.Contains(text, "corvint") || !strings.Contains(text, "$ARGUMENTS") {
			t.Errorf("%s: missing corvint step or $ARGUMENTS", name)
		}
	}
	context := readFile(t, dir+"commands/speckit.corvint.context.md")
	for _, want := range []string{"FEATURE_SPEC", "IMPL_PLAN", "context --task"} {
		if !strings.Contains(context, want) {
			t.Errorf("context command: missing %q", want)
		}
	}
	review := readFile(t, dir+"commands/speckit.corvint.review.md")
	for _, want := range []string{"corvint affected --base", "corvint review --base"} {
		if !strings.Contains(review, want) {
			t.Errorf("review command: missing %q", want)
		}
	}
	readme := readFile(t, dir+"README.md")
	if !strings.HasPrefix(readme, "<!-- SPDX-License-Identifier: Apache-2.0 -->") ||
		!strings.Contains(readme, "specify extension add --dev") ||
		!strings.Contains(readme, "specify extension remove corvint") {
		t.Errorf("speckit README: missing SPDX header, install or remove command")
	}
}

func TestSTAV0009_OpenSpecIntegration(t *testing.T) {
	const dir = "openspec/"
	config := readFile(t, dir+"config.yaml")
	requireLines(t, "config.yaml", config,
		"# SPDX-License-Identifier: Apache-2.0",
		"context: |",
		"rules:",
		"  tasks:",
		"operations:",
		"  apply:",
		"    guidance:",
	)
	rules := config[strings.Index(config, "\nrules:\n"):]
	for _, want := range []string{"corvint context --task", "corvint review --base", "before apply", "after apply"} {
		if !strings.Contains(rules, want) {
			t.Errorf("config.yaml rules: missing %q", want)
		}
	}
	if m := mutating.FindString(config); m != "" {
		t.Errorf("config.yaml: instructs mutating command %q", m)
	}

	skill := readFile(t, dir+"skills/corvint-openspec/SKILL.md")
	checkFrontmatter(t, "SKILL.md", skill, "name", "description", "allowed-tools", "license")
	requireLines(t, "SKILL.md", skill, "name: corvint-openspec", "license: Apache-2.0")
	allowed := regexp.MustCompile(`(?m)^allowed-tools: (.+)$`).FindStringSubmatch(skill)
	if allowed == nil || strings.Contains(allowed[1], "Bash(corvint:*)") || strings.Contains(allowed[1], "Bash(git:*)") {
		t.Errorf("SKILL.md: allowed-tools must name only read-only subcommands")
	}
	checkReadOnlyInstructions(t, "SKILL.md", skill)

	readme := readFile(t, dir+"README.md")
	if !strings.HasPrefix(readme, "<!-- SPDX-License-Identifier: Apache-2.0 -->") {
		t.Errorf("openspec README: missing SPDX header")
	}
	for _, want := range []string{
		"--intent openspec/specs/<capability>/spec.md --intent-form openspec-spec",
		"--intent openspec/changes/<id>/specs/<capability>/spec.md --intent-form openspec-delta",
		"corvint ocm prepare --target",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("openspec README: missing %q", want)
		}
	}
}
