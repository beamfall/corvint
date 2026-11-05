// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DCW-V0-7409: publication needs only the destination directory, not writable
// staging on the Git mount. The lane's EXDEV probe qualifies the actual mounts.
func TestDCWV07409ReportUsesDestinationMount(t *testing.T) {
	root := t.TempDir()
	gitPath := filepath.Join(root, "separate-git-mount")
	if err := os.WriteFile(gitPath, []byte("unavailable for staging"), 0600); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(root, "dogfood-report.json")
	original := []byte("{\n  \"profile\": \"test\"\n  ,\"dogfoodCheck\": null\n}\n")
	if err := os.WriteFile(name, original, 0600); err != nil {
		t.Fatal(err)
	}
	c := &check{flow: flow{gitDir: gitPath}, report: name, baseSHA: strings.Repeat("a", 64), treeSHA: strings.Repeat("b", 64)}
	if err := c.record(true, 0); err != nil {
		t.Fatalf("report publication depended on Git staging: %v", err)
	}
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Check struct {
			OutputsAgree bool `json:"outputsAgree"`
		} `json:"dogfoodCheck"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || !result.Check.OutputsAgree {
		t.Fatalf("incomplete report %q: %v", raw, err)
	}
	if marker, err := os.ReadFile(gitPath); err != nil || string(marker) != "unavailable for staging" {
		t.Fatalf("Git path changed: %q %v", marker, err)
	}
	if pending, _ := filepath.Glob(filepath.Join(root, ".claims-*.tmp")); len(pending) != 0 {
		t.Fatalf("publication left staging files: %v", pending)
	}
}

// DCW-V0-7409: the reused atomic publisher preserves the destination on a
// failed rename and removes its private temporary file.
func TestDCWV07409FailedPublicationPreservesDestination(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "report.json")
	if err := os.Mkdir(name, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(name, "preserved")
	if err := os.WriteFile(marker, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeClaimsFile(name, []byte("replacement")); err == nil {
		t.Fatal("replacement of directory succeeded")
	}
	if raw, err := os.ReadFile(marker); err != nil || string(raw) != "original" {
		t.Fatalf("destination changed: %q %v", raw, err)
	}
	if pending, _ := filepath.Glob(filepath.Join(root, ".claims-*.tmp")); len(pending) != 0 {
		t.Fatalf("failed publication left staging files: %v", pending)
	}
}
