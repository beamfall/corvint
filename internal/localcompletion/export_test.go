package localcompletion

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestLCPV0883ExportImportRestoresActiveEnrollment covers LCP-V0-883..885
// (V1-0580) for an ACTIVE enrollment: a worktree restored at the same commit
// with a new Git directory starts inactive, imports the bundle and evaluates
// exactly as the source did (lifecycle, unmet, qualified check), its check
// logs relocated; tampered, foreign-key, moved-HEAD and replacing imports are
// refused, and an export never overwrites a file.
func TestLCPV0883ExportImportRestoresActiveEnrollment(t *testing.T) {
	ctx := context.Background()
	root, key, plan := fixture(t, []string{"true"})
	beginFixture(t, root, key, plan)
	if _, err := Verify(ctx, root, key, "test"); err != nil {
		t.Fatal(err)
	}
	source, err := Evaluate(ctx, root, key)
	if err != nil {
		t.Fatal(err)
	}
	// The bundle sits under a symlinked ancestor, as macOS /tmp and /var are
	// (LCP-V0-884); a symlinked bundle itself is still refused.
	linked := filepath.Join(t.TempDir(), "linked")
	if err = os.Symlink(t.TempDir(), linked); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(linked, "bundle.json")
	if _, err = Export(ctx, root, key, bundle); err != nil {
		t.Fatalf("export: %v", err)
	}
	symlinked := filepath.Join(t.TempDir(), "symlinked.json")
	if err = os.Symlink(bundle, symlinked); err != nil {
		t.Fatal(err)
	}
	if _, err = Export(ctx, root, key, bundle); err == nil || err.Error() != "export-output-exists" {
		t.Fatalf("export over a file: %v", err)
	}
	restored := filepath.Join(t.TempDir(), "restored")
	localGit(t, root, "worktree", "add", "-q", "--detach", restored, "HEAD")
	restored, _ = filepath.EvalSymlinks(restored)
	if before, err := Evaluate(ctx, restored, key); err != nil || before.Lifecycle != "inactive" {
		t.Fatalf("restored worktree before import: %#v %v", before, err)
	}

	tampered := filepath.Join(t.TempDir(), "tampered.json")
	raw, _ := os.ReadFile(bundle)
	if err = os.WriteFile(tampered, []byte(strings.Replace(string(raw), `"lifecycle":"active"`, `"lifecycle":"satisfied"`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = Import(ctx, restored, key, tampered); err == nil || err.Error() != "import-bundle-invalid" {
		t.Fatalf("tampered bundle: %v", err)
	}
	if _, err = Import(ctx, restored, key, symlinked); err == nil || err.Error() != "import-bundle-unreadable" {
		t.Fatalf("symlinked bundle: %v", err)
	}
	if _, err = Import(ctx, restored, HashSession("another"), bundle); err == nil || err.Error() != "import-session-mismatch" {
		t.Fatalf("another key: %v", err)
	}
	moved := filepath.Join(t.TempDir(), "moved")
	localGit(t, root, "worktree", "add", "-q", "--detach", moved, "HEAD")
	if err = os.WriteFile(filepath.Join(moved, "source.go"), []byte("package fixture\n\nconst moved = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	localGit(t, moved, "commit", "-qam", "moved")
	if _, err = Import(ctx, moved, key, bundle); err == nil || err.Error() != "import-target-mismatch" {
		t.Fatalf("moved HEAD: %v", err)
	}

	got, err := Import(ctx, restored, key, bundle)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if got.Lifecycle != source.Lifecycle || got.Satisfied || !reflect.DeepEqual(got.Unmet, source.Unmet) || got.Target != source.Target || len(got.Checks) != 1 || got.Checks[0].Qualified != source.Checks[0].Qualified {
		t.Fatalf("restored %#v\nsource %#v", got, source)
	}
	gitDir := localGit(t, restored, "rev-parse", "--absolute-git-dir")
	if !strings.HasPrefix(got.Checks[0].Stdout, gitDir+"/") || got.Checks[0].Stdout == source.Checks[0].Stdout {
		t.Errorf("check log %s not relocated into %s", got.Checks[0].Stdout, gitDir)
	}
	if _, err = Import(ctx, restored, key, bundle); err == nil || err.Error() != "import-would-replace" {
		t.Fatalf("second import: %v", err)
	}
	// The restored enrollment is live: its owner can keep working.
	if result, err := Verify(ctx, restored, key, "test"); err != nil || !result.Checks[0].Qualified {
		t.Fatalf("verify after import: %#v %v", result, err)
	}
}

// restoreFixture exports key's enrollment from root and adds a detached
// worktree at HEAD to import it into.
func restoreFixture(t *testing.T, root, key string) (bundle, restored string) {
	t.Helper()
	bundle = filepath.Join(t.TempDir(), "bundle.json")
	if _, err := Export(context.Background(), root, key, bundle); err != nil {
		t.Fatalf("export: %v", err)
	}
	restored = filepath.Join(t.TempDir(), "restored")
	localGit(t, root, "worktree", "add", "-q", "--detach", restored, "HEAD")
	restored, _ = filepath.EvalSymlinks(restored)
	return bundle, restored
}

// TestLCPV0884ImportRefusesPlanNamingSourceRoot: a frozen plan whose check
// names the source root cannot move to another worktree, since relocating
// its argv would break the plan digest; the import refuses before writing
// anything and the restored worktree stays inactive (LCP-V0-884).
func TestLCPV0884ImportRefusesPlanNamingSourceRoot(t *testing.T) {
	ctx := context.Background()
	root, key, plan := fixture(t, nil)
	plan.Checks[0].Argv = []string{"test", "-d", filepath.Join(root, "fixture")}
	beginFixture(t, root, key, plan)
	bundle, restored := restoreFixture(t, root, key)
	if _, err := Import(ctx, restored, key, bundle); err == nil || err.Error() != "import-plan-not-relocatable" {
		t.Fatalf("import of a plan naming the source root: %v", err)
	}
	target, err := open(restored, key)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"owner", target.session} {
		if _, err := os.Lstat(filepath.Join(target.directory, name)); !os.IsNotExist(err) {
			t.Errorf("the refused import wrote %s: %v", name, err)
		}
	}
	if after, err := Evaluate(ctx, restored, key); err != nil || after.Lifecycle != "inactive" {
		t.Fatalf("restored worktree after the refusal: %#v %v", after, err)
	}
}

// TestLCPV0883RestoredHoldStillGuardsCancel: the preservation hold and its
// release history travel with the bundle, so a restored held ACTIVE
// enrollment still refuses a cancel (LCP-V0-880, LCP-V0-883).
func TestLCPV0883RestoredHoldStillGuardsCancel(t *testing.T) {
	ctx := context.Background()
	root, key, plan := fixture(t, []string{"true"})
	repo := beginFixture(t, root, key, plan)
	for _, step := range [][2]string{{"first hold", ""}, {"", "coordinator released the first hold"}, {"second hold", ""}} {
		if _, err := Hold(ctx, root, key, step[0], step[1]); err != nil {
			t.Fatalf("hold %q: %v", step, err)
		}
	}
	release, err := os.ReadFile(filepath.Join(repo.releasesPath(), "release-001.json"))
	if err != nil {
		t.Fatal(err)
	}
	bundle, restored := restoreFixture(t, root, key)
	if _, err = Import(ctx, restored, key, bundle); err != nil {
		t.Fatalf("import: %v", err)
	}
	var held *HoldError
	if _, err = Cancel(ctx, restored, key, "base moved"); !errors.As(err, &held) || held.Reason != "second hold" {
		t.Fatalf("cancel of the restored held enrollment: %v", err)
	}
	target, err := open(restored, key)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(target.releasesPath(), "release-001.json")); err != nil || string(got) != string(release) {
		t.Errorf("release record not restored: %q %v", got, err)
	}
}

// TestLCPV0885RelocationNeverLaundersAStaleReportSet: relocation re-digests
// a report set only when its digest held at the source, so a stale set keeps
// its stale digest and review after the move, while a valid one is re-digested
// with its review carried.
func TestLCPV0885RelocationNeverLaundersAStaleReportSet(t *testing.T) {
	root, key, plan := fixture(t, []string{"true"})
	repo := beginFixture(t, root, key, plan)
	saved, err := repo.load()
	if err != nil {
		t.Fatal(err)
	}
	report := artifact{Path: repo.local("report-x-00.md"), Digest: strings.Repeat("b", 64)}
	bundle := &exportBundle{GitDir: repo.auth.GitDir, Root: repo.auth.Root}
	target := &repository{auth: repo.auth, directory: "/restored/.git/corvint/local-completion", session: key}
	moved := *target.auth
	moved.GitDir, moved.Root = "/restored/.git", "/restored"
	target.auth = &moved
	for _, valid := range []bool{false, true} {
		set := &reportSet{PlanDigest: saved.PlanDigest, Target: strings.Repeat("c", 40), Bindings: []artifact{}, Reports: []artifact{report}, Observations: []observation{}}
		set.Digest = valueDigest(*set)
		if !valid {
			set.Digest = strings.Repeat("a", 64)
		}
		saved.ReportSet, saved.Review = set, set.Digest
		raw, _ := json.Marshal(saved)
		out, err := relocateState(raw, bundle, target)
		if err != nil {
			t.Fatal(err)
		}
		var got state
		if err = json.Unmarshal(out, &got); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(got.ReportSet.Reports[0].Path, "/restored/.git/") {
			t.Errorf("valid=%t: report not relocated: %s", valid, got.ReportSet.Reports[0].Path)
		}
		copy := *got.ReportSet
		copy.Digest = ""
		recomputed := valueDigest(copy) == got.ReportSet.Digest
		if recomputed != valid || got.Review != got.ReportSet.Digest || (!valid && got.ReportSet.Digest != strings.Repeat("a", 64)) {
			t.Errorf("valid=%t: digest %s recomputed=%t review %s", valid, got.ReportSet.Digest, recomputed, got.Review)
		}
	}
}
