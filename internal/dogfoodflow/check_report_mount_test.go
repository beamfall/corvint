// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DWC-V0-929: report replacement must not require access to the Git mount.
func TestCheckDWCV0929ReportStaysOnWorktreeMount(t *testing.T) {
	root := t.TempDir()
	report := filepath.Join(root, "dogfood-report.json")
	original := []byte("{\n  \"complete\": true\n  ,\"dogfoodCheck\": null\n}\n")
	if err := os.WriteFile(report, original, 0o600); err != nil {
		t.Fatal(err)
	}
	c := newCheck(context.Background(), "dogfood-check", CheckOptions{Root: root}, nil, nil)
	// This metadata path does not exist and cannot receive a staging file.
	// The old implementation fails here before it can replace the report.
	c.gitDir = filepath.Join(root, "separate-unavailable-git-mount")
	c.report = report
	c.baseSHA, c.treeSHA = strings.Repeat("a", 64), strings.Repeat("b", 64)
	if err := c.record(true, 0); err != nil {
		t.Fatalf("replace report independently of Git mount: %v", err)
	}
	var got struct {
		Complete bool
		Check    struct {
			OutputsAgree bool
			Base         string `json:"baseVerifierSha256"`
		} `json:"dogfoodCheck"`
	}
	data, err := os.ReadFile(report)
	if err != nil || json.Unmarshal(data, &got) != nil || !got.Complete || !got.Check.OutputsAgree || got.Check.Base != "sha256:"+c.baseSHA {
		t.Fatalf("replacement report = %s (%v)", data, err)
	}
	info, err := os.Stat(report)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("private report mode: %v %v", info, err)
	}
	// A malformed input must remain intact and leave no staging artifact.
	if err := os.WriteFile(report, []byte("{\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := c.record(true, 0); err == nil {
		t.Fatal("accepted a report without its check slot")
	}
	data, _ = os.ReadFile(report)
	left, err := filepath.Glob(filepath.Join(root, ".dogfood-report.*.tmp"))
	if string(data) != "{\n}\n" || err != nil || len(left) != 0 {
		t.Fatalf("failed update changed report or left staging files: %s %v %v", data, left, err)
	}
}
