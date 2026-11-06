// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
)

// DCW-V0-380: inputs-check must preserve a committed sidecar on success,
// refusal, and reuse, so the next bind starts from a clean worktree.
func TestDCWV0380InputsCheckPreservesTrackedCEM(t *testing.T) {
	for _, bad := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "refusal"}[bad], func(t *testing.T) {
			root, base := preflightRepository(t, nil)
			sidecar := filepath.Join(root, finishLaneSidecar)
			var prepared, prepareErrors bytes.Buffer
			if code := cemcli.Run(context.Background(), root, []string{"prepare", "--base", base, "--target", "HEAD"}, &prepared, &prepareErrors); code != 0 {
				t.Fatalf("fixture prepare exit=%d: %s", code, &prepareErrors)
			}
			original, err := os.ReadFile(sidecar)
			if err != nil {
				t.Fatal(err)
			}
			testGit(t, root, "add", finishLaneSidecar)
			testGit(t, root, "commit", "-q", "-m", "previous binding")
			if err := os.Chmod(sidecar, 0600); err != nil {
				t.Fatal(err)
			}
			plan := "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n"
			if bad {
				plan = strings.Replace(plan, "3:3", "10:10", 1)
			}
			options := ChangeOptions{Root: root, Base: base, Citations: writeInput(t, plan), IntentsFile: writeInput(t, "#no-intent-declared\n"), Verify: "go version", Outcome: "passed"}
			options.Steps = Runner{Path: writeInput(t, "pinned step executable\n"), Run: func(ctx context.Context, root string, args []string, out, errOut io.Writer) int {
				if len(args) > 0 && args[0] == "cem" {
					return cemcli.Run(ctx, root, args[1:], out, errOut)
				}
				return 1
			}}
			for i := 0; i < 2; i++ {
				var stdout, stderr bytes.Buffer
				code, err := FinishLane(context.Background(), FinishLaneOptions{Change: options, Preflight: true}, &stdout, &stderr)
				if err != nil {
					t.Fatal(err)
				}
				out, errText := stdout.String(), stderr.String()
				if i == 1 && !bad && !strings.Contains(out, "reused=inputs-check") {
					t.Fatalf("receipt not reused: %s", out)
				}
				if (code == 0) == bad {
					t.Fatalf("code=%d stdout=%s stderr=%s", code, out, errText)
				}
				data, err := os.ReadFile(sidecar)
				if err != nil || !bytes.Equal(data, original) {
					t.Fatalf("sidecar changed: %q %v", data, err)
				}
				fi, err := os.Stat(sidecar)
				if err != nil || fi.Mode().Perm() != 0600 {
					t.Fatalf("mode changed: %v %v", fi, err)
				}
				if status := testGit(t, root, "status", "--porcelain", "--untracked-files=no"); status != "" {
					t.Fatalf("tracked files changed: %s", status)
				}
			}
		})
	}
}
