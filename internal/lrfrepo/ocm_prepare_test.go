package lrfrepo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

func TestPrepareOCMResumesAfterExcludedSidecarCommit(t *testing.T) {
	root, base, target := makeOCMPrepareRepository(t)
	prepareCEM(t, root, base, target, false)
	options := PrepareOptions{
		MapPath: ".corvint/change.ocm.json", CEMPath: wire.ExcludedCEMPath,
		IntentPath: "docs/specs/intent.md", ExpectedBase: base, Target: target,
	}
	first, err := PrepareOCM(context.Background(), root, options)
	if err != nil || first.Resumed {
		t.Fatalf("first prepare: resumed=%t err=%v", first != nil && first.Resumed, err)
	}
	if _, err := MarkOCM(context.Background(), root, MarkOptions{
		MapPath: options.MapPath, Obligation: "OCM-TEST-001", Reason: "no-test-claim",
	}); err != nil {
		t.Fatal(err)
	}
	gitOCMPrepare(t, root, "add", wire.ExcludedCEMPath)
	gitOCMPrepare(t, root, "commit", "-qm", "commit CEM sidecar")
	movedTarget := gitOCMPrepare(t, root, "rev-parse", "HEAD")
	options.Target = movedTarget

	resumed, err := PrepareOCM(context.Background(), root, options)
	if err != nil {
		t.Fatal(err)
	}
	if !resumed.Resumed || resumed.Target != movedTarget {
		t.Fatalf("resumed=%t target=%s want=%s", resumed.Resumed, resumed.Target, movedTarget)
	}
	targetValue, _ := resumed.Document.Obj.Get("targetRevision")
	obligations, _ := resumed.Document.Obj.Get("obligations")
	reason, _ := obligations.Arr[0].Obj.Get("reason")
	if targetValue.Str != movedTarget || reason.Str != "no-test-claim" {
		t.Fatalf("target=%s reason=%s", targetValue.Str, reason.Str)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(options.MapPath)))
	if err != nil || !strings.Contains(string(raw), `"targetRevision":"`+movedTarget+`"`) {
		t.Fatalf("repinned map: err=%v bytes=%s", err, raw)
	}
}

func TestPrepareOCMChangedBaseStillRefuses(t *testing.T) {
	root, base, target := makeOCMPrepareRepository(t)
	prepareCEM(t, root, base, target, false)
	options := PrepareOptions{
		MapPath: ".corvint/change.ocm.json", CEMPath: wire.ExcludedCEMPath,
		IntentPath: "docs/specs/intent.md", ExpectedBase: base, Target: target,
	}
	if _, err := PrepareOCM(context.Background(), root, options); err != nil {
		t.Fatal(err)
	}
	gitOCMPrepare(t, root, "switch", "-qc", "changed-base", base)
	gitOCMPrepare(t, root, "commit", "--allow-empty", "-qm", "changed base")
	changedBase := gitOCMPrepare(t, root, "rev-parse", "HEAD")
	writeOCMPrepareFile(t, root, "internal/example.txt", "after\n")
	gitOCMPrepare(t, root, "add", "internal/example.txt")
	gitOCMPrepare(t, root, "commit", "-qm", "alternate target")
	changedTarget := gitOCMPrepare(t, root, "rev-parse", "HEAD")
	prepareCEM(t, root, changedBase, changedTarget, true)
	options.ExpectedBase = changedBase
	options.Target = changedTarget

	_, err := PrepareOCM(context.Background(), root, options)
	if CodeOf(err) != "map-outdated" {
		t.Fatalf("code=%q err=%v", CodeOf(err), err)
	}
}

func TestPrepareOCMRefusesAliasOfCEMPath(t *testing.T) {
	for _, alias := range []string{"absolute", "case-folded"} {
		t.Run(alias, func(t *testing.T) {
			root, base, target := makeOCMPrepareRepository(t)
			prepareCEM(t, root, base, target, false)
			cemFile := filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath))
			options := PrepareOptions{
				MapPath: wire.ExcludedCEMPath, CEMPath: cemFile, IntentPath: "docs/specs/intent.md",
				ExpectedBase: base, Target: target, Replace: true,
			}
			if alias == "case-folded" {
				options.MapPath, options.CEMPath = strings.ToUpper(wire.ExcludedCEMPath), wire.ExcludedCEMPath
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(options.MapPath))); err != nil {
					t.Skip("case-sensitive filesystem: the case-folded path is a different file")
				}
			}
			before, err := os.ReadFile(cemFile)
			if err != nil {
				t.Fatal(err)
			}
			_, err = PrepareOCM(context.Background(), root, options)
			after, _ := os.ReadFile(cemFile)
			if CodeOf(err) != "output-path-conflict" || string(after) != string(before) {
				t.Fatalf("code=%q err=%v cemChanged=%t", CodeOf(err), err, string(after) != string(before))
			}
		})
	}
}

func TestPrepareOCMRefusesUnreadableExistingMap(t *testing.T) {
	root, base, target := makeOCMPrepareRepository(t)
	prepareCEM(t, root, base, target, false)
	options := PrepareOptions{
		MapPath: ".corvint/change.ocm.json", CEMPath: wire.ExcludedCEMPath,
		IntentPath: "docs/specs/intent.md", ExpectedBase: base, Target: target,
	}
	mapFile := filepath.Join(root, filepath.FromSlash(options.MapPath))
	oversized := []byte(`{"pad":"` + strings.Repeat("x", maxOCMBytes) + `"}`)
	if err := os.WriteFile(mapFile, oversized, 0600); err != nil {
		t.Fatal(err)
	}
	_, err := PrepareOCM(context.Background(), root, options)
	after, _ := os.ReadFile(mapFile)
	if CodeOf(err) != "ocm-cli-error" || !strings.Contains(err.Error(), "OCM map exceeds") || len(after) != len(oversized) {
		t.Fatalf("without --replace: err=%v size=%d", err, len(after))
	}
	options.Replace = true
	if _, err := PrepareOCM(context.Background(), root, options); err != nil {
		t.Fatalf("with --replace: %v", err)
	}
}

func TestPrepareOCMRefusesStructurallyInvalidSameBindingMap(t *testing.T) {
	root, base, target := makeOCMPrepareRepository(t)
	prepareCEM(t, root, base, target, false)
	options := PrepareOptions{
		MapPath: ".corvint/change.ocm.json", CEMPath: wire.ExcludedCEMPath,
		IntentPath: "docs/specs/intent.md", ExpectedBase: base, Target: target,
	}
	prepared, err := PrepareOCM(context.Background(), root, options)
	if err != nil {
		t.Fatal(err)
	}
	obligations, _ := prepared.Document.Obj.Get("obligations")
	objectMember(obligations.Arr[0].Obj, "reason", stringValue("invalid-reason"))
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(options.MapPath)), canonicalOCMBytes(prepared.Document), 0o600); err != nil {
		t.Fatal(err)
	}
	checked, err := ReadOCM(context.Background(), root, OCMReadOptions{
		OCMPath: options.MapPath, CEMPath: options.CEMPath, ExpectedBase: base, Target: target,
		ExpectedBaseGiven: true, TargetGiven: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := checked.Verification["issues"].([]any)[0].(map[string]any)["code"]
	if _, err := PrepareOCM(context.Background(), root, options); CodeOf(err) != want {
		t.Fatalf("code=%q want=%q err=%v", CodeOf(err), want, err)
	}
}

func TestBootstrapIntentRequiresUnknownCEMHunk(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitOCMPrepare(t, root, "init", "-q", "-b", "main")
	writeOCMPrepareFile(t, root, "policy.md", "governing policy\n")
	gitOCMPrepare(t, root, "add", ".")
	gitOCMPrepare(t, root, "commit", "-qm", "base")
	base := gitOCMPrepare(t, root, "rev-parse", "HEAD")
	writeOCMPrepareFile(t, root, "docs/specs/bootstrap.md", "# Intent\n\n## Requirements\n\n- `OCM-V0-009`: preserve bootstrap unknowns.\n")
	gitOCMPrepare(t, root, "add", ".")
	gitOCMPrepare(t, root, "commit", "-qm", "target")
	target := gitOCMPrepare(t, root, "rev-parse", "HEAD")
	prepareCEM(t, root, base, target, false)
	options := PrepareOptions{
		MapPath: ".corvint/change.ocm.json", CEMPath: wire.ExcludedCEMPath,
		IntentPath: "docs/specs/bootstrap.md", ExpectedBase: base, Target: target,
	}
	if _, err := PrepareOCM(context.Background(), root, options); err != nil {
		t.Fatalf("OCM-V0-009 unknown bootstrap hunk: %v", err)
	}
	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Cite(context.Background(), workflow.CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "policy.md",
		Lines: "1:1", Relation: "decision",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareOCM(context.Background(), root, options); CodeOf(err) != "bootstrap-intent-not-unknown" {
		t.Fatalf("code=%q err=%v", CodeOf(err), err)
	}
}

func makeOCMPrepareRepository(t *testing.T) (string, string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitOCMPrepare(t, root, "init", "-q", "-b", "main")
	writeOCMPrepareFile(t, root, "docs/specs/intent.md", "# Intent\n\n## Requirements\n\n- `OCM-TEST-001`: preserve the review.\n\n## Non-goals\n")
	writeOCMPrepareFile(t, root, "internal/example.txt", "before\n")
	gitOCMPrepare(t, root, "add", ".")
	gitOCMPrepare(t, root, "commit", "-qm", "base")
	base := gitOCMPrepare(t, root, "rev-parse", "HEAD")
	writeOCMPrepareFile(t, root, "internal/example.txt", "after\n")
	gitOCMPrepare(t, root, "add", "internal/example.txt")
	gitOCMPrepare(t, root, "commit", "-qm", "target")
	return root, base, gitOCMPrepare(t, root, "rev-parse", "HEAD")
}

func prepareCEM(t *testing.T, root, base, target string, replace bool) {
	t.Helper()
	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Prepare(context.Background(), workflow.PrepareOptions{
		Base: base, Target: target, Replace: replace,
	}); err != nil {
		t.Fatal(err)
	}
}

func writeOCMPrepareFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitOCMPrepare(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = root
	command.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1", "HOME="+t.TempDir(), "XDG_CONFIG_HOME="+t.TempDir(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
		"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
	return strings.TrimSpace(string(output))
}
