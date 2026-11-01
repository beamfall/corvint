package localcompletion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLCPV0880PreservationHoldGuardsCancel covers LCP-V0-880..882 (V1-0581):
// a coordinator's hold makes cancel of the ACTIVE enrollment refuse, naming
// the distinct-checkout route, and leaves its state byte-identical; a second
// checkout enrolls freely while the first stays ACTIVE; only a release that
// records the authorization admits the cancel, which keeps the observations.
func TestLCPV0880PreservationHoldGuardsCancel(t *testing.T) {
	ctx := context.Background()
	root, key, plan := fixture(t, []string{"true"})
	repo := beginFixture(t, root, key, plan)
	if _, err := Verify(ctx, root, key, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := Hold(ctx, root, key, "", ""); err == nil || err.Error() != "invalid-local-completion-option" {
		t.Fatalf("hold with neither option: %v", err)
	}
	if _, err := Hold(ctx, root, key, "r", "a"); err == nil || err.Error() != "invalid-local-completion-option" {
		t.Fatalf("hold with both options: %v", err)
	}
	if _, err := Hold(ctx, root, key, "token ghp_"+strings.Repeat("a", 36), ""); err == nil || err.Error() != "hold-text-secret-screened" {
		t.Fatalf("secret-bearing reason: %v", err)
	}
	if result, err := Hold(ctx, root, key, "coordinator: preserve ACTIVE; new base goes to a fresh checkout", ""); err != nil || result.Lifecycle != "active" {
		t.Fatalf("hold: %#v %v", result, err)
	}
	if _, err := Hold(ctx, root, key, "again", ""); err == nil || err.Error() != "preservation-hold-exists" {
		t.Fatalf("second hold: %v", err)
	}
	before, _ := os.ReadFile(repo.local("state.json"))
	_, err := Cancel(ctx, root, key, "base moved")
	var held *HoldError
	if err == nil || err.Error() != "enrollment-preservation-hold" || !errors.As(err, &held) {
		t.Fatalf("cancel under a hold: %v", err)
	}
	for _, want := range []string{`"coordinator: preserve ACTIVE; new base goes to a fresh checkout"`, "begin a new enrollment in a distinct checkout", "--release-authorization"} {
		if !strings.Contains(held.Detail(), want) {
			t.Errorf("detail %q lacks %q", held.Detail(), want)
		}
	}
	if after, _ := os.ReadFile(repo.local("state.json")); !bytes.Equal(before, after) {
		t.Fatal("a refused cancel changed the enrollment")
	}

	// The distinct-checkout route: another worktree enrolls its own key while
	// the held enrollment stays ACTIVE.
	other := filepath.Join(t.TempDir(), "other")
	localGit(t, root, "worktree", "add", "-q", "--detach", other, "HEAD")
	otherKey := HashSession(t.Name() + "/other")
	raw, _ := json.Marshal(plan)
	if result, err := Begin(ctx, other, otherKey, raw); err != nil || result.Lifecycle != "active" {
		t.Fatalf("begin in a distinct checkout: %#v %v", result, err)
	}
	if result, err := Evaluate(ctx, root, key); err != nil || result.Lifecycle != "active" {
		t.Fatalf("held enrollment after the distinct begin: %#v %v", result, err)
	}

	// An authorized release keeps the hold with its authorization, then the
	// cancel proceeds and keeps the saved observation.
	if _, err := Hold(ctx, root, key, "", "coordinator approved cancel 2026-10-31"); err != nil {
		t.Fatalf("release: %v", err)
	}
	record, err := os.ReadFile(filepath.Join(repo.directory, "holds", key+".releases", "release-001.json"))
	if err != nil || !bytes.Contains(record, []byte(`"authorization":"coordinator approved cancel 2026-10-31"`)) || !bytes.Contains(record, []byte(`"reason":"coordinator: preserve ACTIVE`)) {
		t.Fatalf("release record %s: %v", record, err)
	}
	if _, err := Hold(ctx, root, key, "", "again"); err == nil || err.Error() != "preservation-hold-absent" {
		t.Fatalf("release without a hold: %v", err)
	}
	result, err := Cancel(ctx, root, key, "coordinator approved")
	if err != nil || result.Lifecycle != "cancelled" {
		t.Fatalf("authorized cancel: %#v %v", result, err)
	}
	saved, err := repo.load()
	if err != nil || len(saved.Observations) != 1 {
		t.Fatalf("cancel kept %d observations: %v", len(saved.Observations), err)
	}
	if _, err := Hold(ctx, root, key, "late", ""); err == nil || err.Error() != "preservation-hold-requires-active" {
		t.Fatalf("hold on a cancelled enrollment: %v", err)
	}
	// The hold and its records never count as an enrollment generation.
	if count, err := repo.generationCount(); err != nil || count != 1 {
		t.Fatalf("generations=%d %v", count, err)
	}
}
