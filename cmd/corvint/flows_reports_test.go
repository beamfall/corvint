// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

// AFL-V1-001, AFL-V1-007, PROD-DOCS-011: `corvint flows status|impact|gaps` run in corvint. With a
// stand-in corvint-docs first on PATH, none reaches it; with no corvint-docs anywhere on PATH,
// each still prints its report and exits 0.
func TestAFLV1001V1007ReportsRunWithCorvintAlone_V15623(t *testing.T) {
	fx := newShopFixture(t)
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	reports := []struct {
		args   []string
		schema string
	}{
		{[]string{"status", "--flows", "flows"}, "application-flow-status/0"},
		{[]string{"status", "--flows", "flows", "--base", fx.b, "--evidence", fx.evidence}, "application-flow-status/0"},
		{[]string{"impact", "--flows", "flows", "--base", fx.b}, "application-flow-impact/1"},
		{[]string{"gaps", "--flows", "flows", "--evidence", fx.evidence}, "application-flow-gaps/1"},
	}
	runReports := func(t *testing.T, capture string) {
		for _, report := range reports {
			var stdout, stderr bytes.Buffer
			code := run(append([]string{"--root", fx.root, "flows"}, report.args...), strings.NewReader(""), &stdout, &stderr)
			var document struct {
				Schema string `json:"schema"`
			}
			if code != 0 || stderr.Len() != 0 || json.Unmarshal(stdout.Bytes(), &document) != nil || document.Schema != report.schema {
				t.Fatalf("corvint flows %v: exit %d stderr %q stdout %q", report.args, code, &stderr, &stdout)
			}
			if capture != "" {
				if _, err := os.Stat(capture); err == nil {
					t.Fatalf("corvint flows %v reached corvint-docs", report.args)
				}
			}
		}
	}
	t.Run("stand-in first on PATH", func(t *testing.T) {
		capture := fakeDocsApp(t)
		t.Setenv("PATH", os.Getenv("PATH")+string(os.PathListSeparator)+filepath.Dir(gitPath))
		runReports(t, capture)
	})
	t.Run("corvint-docs absent", func(t *testing.T) {
		alone := t.TempDir()
		if err := os.Symlink(gitPath, filepath.Join(alone, "git")); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", alone)
		if _, err := exec.LookPath("corvint-docs"); err == nil {
			t.Fatal("corvint-docs is still on PATH")
		}
		runReports(t, "")
	})
}

// MTT-V1-009, AFL-V1-003, AFL-V1-007: corvint.flows_status, corvint.flows_impact and
// corvint.flows_gaps on corvint mcp --tool-profile flows return, as their receipts, the report
// `corvint flows VERB` prints for the same fixture and inputs: the same canonical bytes.
func TestMTTV1009NativeFlowReportToolsMatchCLI(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the repository probe is qualified only on Darwin and Linux")
	}
	fx := newShopFixture(t)
	root, err := filepath.EvalSymlinks(fx.root)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := bridge.NewFlows(root)
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, tool := range registry.Tools() {
		listed[tool.Name] = true
	}
	for _, tool := range []string{bridge.ToolFlowsStatus, bridge.ToolFlowsImpact, bridge.ToolFlowsGaps} {
		if !listed[tool] {
			t.Fatalf("corvint mcp --tool-profile flows does not list %s", tool)
		}
	}
	for _, tc := range []struct {
		tool, arguments string
		cli             []string
	}{
		{bridge.ToolFlowsStatus, `{"flows":"flows"}`, []string{"status", "--flows", "flows"}},
		{bridge.ToolFlowsStatus, `{"flows":"flows","base":"` + fx.b + `"}`, []string{"status", "--flows", "flows", "--base", fx.b}},
		{bridge.ToolFlowsImpact, `{"flows":"flows","base":"` + fx.b + `"}`, []string{"impact", "--flows", "flows", "--base", fx.b}},
		{bridge.ToolFlowsGaps, `{"flows":"flows"}`, []string{"gaps", "--flows", "flows"}},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(append([]string{"--root", root, "flows"}, tc.cli...), strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Fatalf("corvint flows %v: exit %d %s", tc.cli, code, &stderr)
		}
		result, failure := registry.Call(context.Background(), tc.tool, []byte(tc.arguments))
		if failure != nil || result.State != "READY" {
			t.Fatalf("%s %s: %+v %v", tc.tool, tc.arguments, result, failure)
		}
		receipt, err := json.Marshal(result.Receipt)
		if err != nil {
			t.Fatal(err)
		}
		if tool, cli := canonicalJSON(t, receipt), canonicalJSON(t, stdout.Bytes()); !bytes.Equal(tool, cli) {
			t.Fatalf("%s %s differs from corvint flows %v:\ntool %s\ncli  %s", tc.tool, tc.arguments, tc.cli, tool, cli)
		}
	}
}

// canonicalJSON re-encodes one JSON body with sorted members and exact numbers.
func canonicalJSON(t *testing.T, raw []byte) []byte {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
