package doccorpus

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// TestDCIV1025CheckoutRefusalKeepsGitauthReason: a further checkout that is a real
// `git clone --shared` (objects/info/alternates) is refused at manifest and at build time
// naming its alias or repository and carrying gitauth's unsupported-object-alternates code and
// its repack/re-clone fix, not an anonymous "checkout unavailable" (GitHub #2009).
func TestDCIV1025CheckoutRefusalKeepsGitauthReason(t *testing.T) {
	t.Parallel()
	app, tests := workspaceRepos(t)
	m := workspaceManifest(t, app, tests)
	shared := filepath.Join(filepath.Dir(app), "tests-shared")
	git(t, filepath.Dir(app), "clone", "-q", "--shared", tests, shared)
	want := func(err error, prefix string) {
		t.Helper()
		var e *Error
		if !errors.As(err, &e) || e.Code != "corpus-input-unavailable" || !strings.HasPrefix(e.Message, prefix+": unsupported-object-alternates: ") || !strings.Contains(e.Message, "git repack -a -d") {
			t.Fatalf("an alternates clone should be refused naming %q with the gitauth code and fix: %v", prefix, err)
		}
	}
	_, err := InventoryWorkspace(context.Background(), app, git(t, app, "rev-parse", "HEAD"), []string{"docs"}, ManifestSchema, "2026-10-29T00:00:00Z", []WorkspaceRepository{{Alias: "e2e", Root: shared, Scopes: []string{"pages"}}})
	want(err, "workspace repository e2e checkout unavailable")
	m.AdditionalRepositories[0].Root = "../tests-shared"
	_, err = Build(context.Background(), app, m)
	want(err, "additional repository checkout unavailable: "+m.AdditionalRepositories[0].ID)
}
