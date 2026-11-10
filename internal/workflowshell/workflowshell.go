// SPDX-License-Identifier: Apache-2.0

// Package workflowshell derives, from the text of a GitHub Actions workflow,
// the shell argv a `run:` step of it executes under, so a test that extracts a
// step and runs it locally uses the shell CI uses (CIN-V0-8426). GitHub runs a
// step with no `shell:` through `bash -e {0}` on Linux and macOS, and a step or
// workflow that declares `shell: bash` through `bash --noprofile --norc -eo
// pipefail {0}`; the two differ in pipefail, which decides whether a failed
// stage of a pipeline fails the step.
package workflowshell

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

// Argv returns the interpreter argv of the workflow's run steps, without the
// script argument: the top-level `defaults.run.shell` decides. It refuses a
// workflow whose steps set `shell:` themselves, since an extracted fragment
// cannot be matched to its step, and a shell GitHub does not run as bash or sh.
func Argv(workflow []byte) ([]string, error) {
	shell, err := declared(workflow)
	if err != nil {
		return nil, err
	}
	switch shell {
	case "":
		return []string{"bash", "-e"}, nil
	case "bash":
		return []string{"bash", "--noprofile", "--norc", "-eo", "pipefail"}, nil
	case "sh":
		return []string{"sh", "-e"}, nil
	}
	return nil, fmt.Errorf("defaults.run.shell is %q, which this helper does not run", shell)
}

// declared returns the value of the top-level `defaults: run: shell:` key, or
// "" when the workflow sets none.
func declared(workflow []byte) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(workflow))
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	shell := ""
	inDefaults, inRun := false, false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		// A step's keys may follow its list dash on the same line.
		item := strings.TrimPrefix(trimmed, "- ")
		switch {
		case indent == 0:
			inDefaults, inRun = trimmed == "defaults:", false
		case inDefaults && indent == 2:
			inRun = trimmed == "run:"
		case inDefaults && inRun && indent == 4 && strings.HasPrefix(trimmed, "shell:"):
			shell = value(strings.TrimPrefix(trimmed, "shell:"))
		case !inDefaults && strings.HasPrefix(item, "shell:"):
			return "", fmt.Errorf("a step or job sets %q, so the workflow default does not decide every step", item)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return shell, nil
}

// value strips a trailing comment and surrounding quotes from a scalar.
func value(text string) string {
	if i := strings.Index(text, " #"); i >= 0 {
		text = text[:i]
	}
	text = strings.TrimSpace(text)
	return strings.Trim(text, `"'`)
}
