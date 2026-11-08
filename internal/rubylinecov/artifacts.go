// SPDX-License-Identifier: Apache-2.0

package rubylinecov

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadSelectionArtifacts decodes core-captured collector directories without
// executing Ruby or writing files (RSP-V0-005, RSP-V0-048). Each directory must
// contain the complete, immutable process-tree capture of one invocation. The
// caller owns capture provenance and pins sources separately for SelectionEvidence.
// Isolated directories are optional: missing or divergent isolated outcomes keep
// coverage incomplete. No caller-supplied boolean can certify isolation.
func ReadSelectionArtifacts(suiteDirectory string, isolatedDirectories []string) (*Run, error) {
	if len(isolatedDirectories) > 4096 {
		return nil, fmt.Errorf("ruby-coverage-cap-exceeded")
	}
	budget := int64(128 << 20)
	readArtifacts := func(dir string) (*Run, error) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		if len(entries) > 4096 {
			return nil, fmt.Errorf("ruby-coverage-cap-exceeded")
		}
		for _, entry := range entries {
			name := entry.Name()
			// Closed collector artifact vocabulary: future forms need explicit support.
			if name != "suite.jsonl" && !(strings.HasPrefix(name, "child-") && strings.HasSuffix(name, ".jsonl")) {
				return nil, fmt.Errorf("ruby-coverage-artifact-unsupported")
			}
			info, err := entry.Info()
			if err != nil {
				return nil, err
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("ruby-coverage-artifact-unsupported")
			}
			budget -= info.Size()
			if info.Size() > 16<<20 || budget < 0 {
				return nil, fmt.Errorf("ruby-coverage-cap-exceeded")
			}
		}
		run, err := read(filepath.Clean(dir))
		if err != nil {
			return nil, err
		}
		// Admit only the terminal states understood by the collector adapter.
		seen := map[string]bool{}
		for _, out := range run.Outcomes {
			key := out.Package + "\x00" + out.Name
			if out.Package == "" || out.Name == "" || seen[key] {
				return nil, fmt.Errorf("ruby-coverage-identity-invalid")
			}
			seen[key] = true
			switch out.Status {
			case "passed", "failed", "pending":
			default:
				return nil, fmt.Errorf("ruby-coverage-status-unsupported")
			}
		}
		return run, nil
	}
	run, err := readArtifacts(suiteDirectory)
	if err != nil {
		return nil, err
	}
	isolated := map[string]*Run{}
	for _, dir := range isolatedDirectories {
		one, err := readArtifacts(dir)
		if err != nil {
			return nil, err
		}
		if len(one.Outcomes) != 1 {
			return nil, fmt.Errorf("ruby-coverage-isolation-invalid")
		}
		out := one.Outcomes[0]
		key := out.Package + "\x00" + out.Name
		if isolated[key] != nil {
			return nil, fmt.Errorf("ruby-coverage-isolation-duplicate")
		}
		isolated[key] = one
	}
	return isolateArtifacts(run, func(_ int, out Outcome) (*Run, error) {
		return isolated[out.Package+"\x00"+out.Name], nil
	}), nil
}
