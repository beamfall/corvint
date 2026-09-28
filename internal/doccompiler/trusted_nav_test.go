package doccompiler

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func trustedNavGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", root}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v %s", args, e, b)
	}
	return strings.TrimSpace(string(b))
}
func trustedNavFixture(t *testing.T, config string) (string, TrustedNavRequest) {
	t.Helper()
	python := os.Getenv("CORVINT_TRUSTED_NAV_PYTHON")
	if python == "" {
		t.Skip("real pinned MkDocs fixture requires CORVINT_TRUSTED_NAV_PYTHON")
	}
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "docs"), 0700)
	for name, data := range map[string]string{"mkdocs.yml": config, "docs/index.md": "# Home\n", "requirements.lock": "mkdocs==1.6.1\n"} {
		if e := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); e != nil {
			t.Fatal(e)
		}
	}
	trustedNavGit(t, root, "init", "-q")
	trustedNavGit(t, root, "add", ".")
	trustedNavGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "fixture")
	raw, e := navProbe(context.Background(), python, map[string]any{"mode": "inventory"}, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	var inventory struct {
		Hash string `json:"inventory_sha256"`
	}
	if e = json.Unmarshal(raw, &inventory); e != nil {
		t.Fatal(e)
	}
	executable, e := os.ReadFile(python)
	if e != nil {
		t.Fatal(e)
	}
	return root, TrustedNavRequest{Profile: TrustedNavRequestProfile, Revision: trustedNavGit(t, root, "rev-parse", "HEAD"), Config: "mkdocs.yml", ConfigSHA256: sha256Hex([]byte(config)), Environment: TrustedNavEnvironment{Python: python, PythonSHA256: sha256Hex(executable), InventorySHA256: inventory.Hash, Lock: "requirements.lock", LockSHA256: sha256Hex([]byte("mkdocs==1.6.1\n"))}, Nav: []TrustedNavEntry{{"Home", "index.md"}, {"New", "new.md"}}, Documents: []TrustedNavDocument{{"docs/new.md", "# New\n", sha256Hex([]byte("# New\n"))}}}
}

// TestTrustedNavRealRoundTrip proves TPN-V0-001..006 through real pinned MkDocs.
func TestTrustedNavRealRoundTrip(t *testing.T) {
	config := "# café\r\nsite_name: Demo\r\nnav:\r\n  - Home: index.md\r\n# keep me\r\nuse_directory_urls: false\r\n"
	root, req := trustedNavFixture(t, config)
	raw, _ := CanonicalJSON(req)
	output, err := TrustedNavigation(context.Background(), root, raw, true)
	if err != nil {
		t.Fatal(err)
	}
	var result trustedNavResult
	if err = json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	if result.State != "NAVIGATION_VALIDATED" || result.BuildStrict != "NOT_RUN" || result.OfflineQualified != "NOT_OBSERVED" {
		t.Fatalf("wrong claims: %s", output)
	}
	expected := strings.Replace(config, "- Home: index.md", `[{"Home":"index.md"},{"New":"new.md"}]`, 1)
	if result.CandidateSHA256 != sha256Hex([]byte(expected)) {
		t.Fatalf("candidate bytes differ: span=%d:%d replacement=%q", result.Snapshot.Start, result.Snapshot.End, result.Replacement)
	}
	if result.PatchSHA256 != sha256Hex([]byte(result.ProposalPatch)) {
		t.Fatal("patch digest")
	}
	if got, _ := os.ReadFile(filepath.Join(root, "mkdocs.yml")); !bytes.Equal(got, []byte(config)) {
		t.Fatal("source mutated")
	}
	if trustedNavGit(t, root, "status", "--porcelain") != "" {
		t.Fatal("Git state mutated")
	}
	patch := filepath.Join(t.TempDir(), "proposal.patch")
	os.WriteFile(patch, []byte(result.ProposalPatch), 0600)
	trustedNavGit(t, root, "apply", "--check", patch)
	// Applying here is a test assertion in a disposable fixture, never the product path.
	trustedNavGit(t, root, "apply", patch)
	if got, _ := os.ReadFile(filepath.Join(root, "mkdocs.yml")); string(got) != expected {
		t.Fatal("diff does not reproduce candidate")
	}
}

// TestTrustedNavRefusals retains TPN-V0-002..005 stale and authority boundaries.
func TestTrustedNavRefusals(t *testing.T) {
	root, req := trustedNavFixture(t, "site_name: Demo\nnav:\n  - Home: index.md\n")
	for _, test := range []struct {
		name string
		edit func(*TrustedNavRequest)
	}{
		{"config", func(r *TrustedNavRequest) { r.ConfigSHA256 = strings.Repeat("0", 64) }},
		{"revision", func(r *TrustedNavRequest) { r.Revision = strings.Repeat("0", 40) }},
		{"interpreter", func(r *TrustedNavRequest) { r.Environment.PythonSHA256 = strings.Repeat("0", 64) }},
		{"inventory", func(r *TrustedNavRequest) { r.Environment.InventorySHA256 = strings.Repeat("0", 64) }},
		{"lock", func(r *TrustedNavRequest) { r.Environment.LockSHA256 = strings.Repeat("0", 64) }},
		{"overlap", func(r *TrustedNavRequest) { r.Documents[0].Path = "docs/index.md" }},
		{"escape", func(r *TrustedNavRequest) { r.Nav[0].Path = "../outside.md" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := req
			r.Nav = append([]TrustedNavEntry{}, req.Nav...)
			r.Documents = append([]TrustedNavDocument{}, req.Documents...)
			test.edit(&r)
			raw, _ := CanonicalJSON(r)
			if _, e := TrustedNavigation(context.Background(), root, raw, true); e == nil {
				t.Fatal("accepted stale/invalid input")
			}
			if trustedNavGit(t, root, "status", "--porcelain") != "" {
				t.Fatal("mutated on refusal")
			}
		})
	}
	raw, _ := CanonicalJSON(req)
	if _, e := TrustedNavigation(context.Background(), root, raw, false); e == nil {
		t.Fatal("trust bypass")
	}
}

// TestTrustedNavRejectsLexicalAuthority covers TPN-V0-003: only the loader owns nav.
func TestTrustedNavRejectsLexicalAuthority(t *testing.T) {
	for _, config := range []string{
		"site_name: Demo\nextra: 'nav: [{Home: index.md}]'\n",
		"site_name: Demo\nnav: [{Home: index.md}]\nnav: [{Other: index.md}]\n",
		"site_name: Demo\nnav: &n [{Home: index.md}]\n",
		"site_name: Demo\nnav: !ENV [NAV]\n",
		"site_name: Demo\ndocs_dir: ../outside\nnav: [{Home: index.md}]\n",
		"site_name: Demo\nplugins: [search]\nnav: [{Home: index.md}]\n",
	} {
		t.Run(sha256Hex([]byte(config))[:8], func(t *testing.T) {
			root, r := trustedNavFixture(t, config)
			raw, _ := CanonicalJSON(r)
			if _, e := TrustedNavigation(context.Background(), root, raw, true); e == nil {
				t.Fatal("unqualified authority accepted")
			}
		})
	}
}

func TestTrustedNavReviewRegressions(t *testing.T) {
	root, req := trustedNavFixture(t, "site_name: Demo\nnav: [{Home: index.md}]\n")
	t.Run("empty-document", func(t *testing.T) {
		r := req
		r.Documents = []TrustedNavDocument{{"docs/new.md", "", sha256Hex(nil)}}
		raw, _ := CanonicalJSON(r)
		if _, err := TrustedNavigation(t.Context(), root, raw, true); err == nil {
			t.Fatal("empty document admitted with invalid diff")
		}
	})
	t.Run("proposed-directory-case", func(t *testing.T) {
		r := req
		r.Documents = []TrustedNavDocument{{"docs/Guide/a.md", "a", sha256Hex([]byte("a"))}, {"docs/guide/b.md", "b", sha256Hex([]byte("b"))}}
		r.Nav = []TrustedNavEntry{{"Home", "index.md"}}
		raw, _ := CanonicalJSON(r)
		if _, err := TrustedNavigation(t.Context(), root, raw, true); err == nil {
			t.Fatal("case-ambiguous directories admitted")
		}
	})
	t.Run("replacement-ref", func(t *testing.T) {
		replacement := trustedNavGit(t, root, "rev-parse", "HEAD^{tree}")
		commit := trustedNavGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit-tree", replacement, "-m", "replacement")
		trustedNavGit(t, root, "replace", req.Revision, commit)
		t.Cleanup(func() { trustedNavGit(t, root, "replace", "-d", req.Revision) })
		raw, _ := CanonicalJSON(req)
		if _, err := TrustedNavigation(t.Context(), root, raw, true); err == nil {
			t.Fatal("replacement object admitted")
		}
	})
}

func TestTrustedNavCleanupFailureSuppressesSuccess(t *testing.T) {
	output := []byte("success")
	var err error
	finishTrustedNav("/owned/temp", &output, &err, func(string) error { return os.ErrPermission })
	if len(output) != 0 || errorCode(err) != "trusted-nav-cleanup" || !strings.Contains(err.Error(), "/owned/temp") {
		t.Fatalf("output=%s err=%v", output, err)
	}
}
