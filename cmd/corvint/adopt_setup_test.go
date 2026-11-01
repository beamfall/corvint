package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupFixture is a fully set up repository: committed AGENTS.md naming
// Corvint, project-scope Claude Code plugin enablement, every Corvint local
// state path ignored, and a current index snapshot.
func setupFixture(t *testing.T) string {
	t.Helper()
	root := impactCLIRepository(t)
	files := map[string]string{
		"AGENTS.md":                           "# Agents\n\nRun `corvint context` before editing.\n",
		".claude/settings.json":               "{\"enabledPlugins\": {\"corvint@corvint\": true}}\n",
		".gitignore":                          strings.Join(setupIgnoredPaths, "\n") + "\n",
		".github/cishards/test-commands.json": `{"profile":"corvint-test-commands/0","go":{"env":{"GOTOOLCHAIN":"local"},"flags":["-count=1"]},"languages":{"shell":["make","shell-test"],"typescript":{"none":"no suite"}}}` + "\n",
	}
	for name, content := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "setup")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--root", root, "index"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("index exit=%d stderr=%s", code, stderr.String())
	}
	return root
}

// setupStateDigest digests every file under root, .git included, so a write
// anywhere (worktree, Git index, snapshot store) changes it.
func setupStateDigest(t *testing.T, root string) string {
	t.Helper()
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		if !entry.Type().IsRegular() {
			fmt.Fprintf(hash, "%s\x00%v\x00", path, entry.Type())
			return nil
		}
		body, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrPermission) {
			fmt.Fprintf(hash, "%s\x00unreadable\x00", path)
			return nil
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(hash, "%s\x00%x\x00", path, sha256.Sum256(body))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func runSetup(t *testing.T, root string) map[string]map[string]any {
	t.Helper()
	before := setupStateDigest(t, root)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--root", root, "adopt", "--setup"}, strings.NewReader(""), &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if setupStateDigest(t, root) != before {
		t.Fatal("adopt --setup wrote repository state")
	}
	var payload struct {
		Mutates bool `json:"mutates"`
		Setup   struct {
			Profile  string           `json:"profile"`
			State    string           `json:"state"`
			GapCount int              `json:"gap_count"`
			Checks   []map[string]any `json:"checks"`
		} `json:"setup"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Mutates || payload.Setup.Profile != setupReportProfile {
		t.Fatalf("payload: %s", stdout.String())
	}
	checks := map[string]map[string]any{}
	gaps := 0
	for _, check := range payload.Setup.Checks {
		checks[check["class"].(string)] = check
		if check["status"] != "ok" {
			gaps++
			if check["fix"] == nil || len(check["evidence"].([]any)) == 0 {
				t.Fatalf("gap without evidence or fix: %v", check)
			}
		}
	}
	if len(checks) != 5 || gaps != payload.Setup.GapCount || (gaps == 0) != (payload.Setup.State == "READY") {
		t.Fatalf("report: %s", stdout.String())
	}
	return checks
}

// ADS-V0-001..004: a fully set up fixture reports no gap; each fixture missing
// one setup piece reports exactly that gap class with evidence and a fix.
func TestAdoptSetupReportsEachGapClass(t *testing.T) {
	// The host's global Git excludes must not decide the ignored-paths class.
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Run("fully set up", func(t *testing.T) {
		for class, check := range runSetup(t, setupFixture(t)) {
			if check["status"] != "ok" {
				t.Fatalf("%s: %v", class, check)
			}
		}
	})
	t.Run("codex adapter hook", func(t *testing.T) {
		root := setupFixture(t)
		os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte("{}\n"), 0o644)
		os.MkdirAll(filepath.Join(root, ".codex"), 0o755)
		os.WriteFile(filepath.Join(root, ".codex", "hooks.json"), []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"corvint adapter codex"}]}]}}`), 0o644)
		if check := runSetup(t, root)["hooks"]; check["status"] != "ok" {
			t.Fatalf("hooks: %v", check)
		}
	})
	for _, tc := range []struct {
		class   string
		break_  func(t *testing.T, root string)
		witness string
	}{
		{"index-snapshot", func(t *testing.T, root string) {
			os.WriteFile(filepath.Join(root, "pkg", "new.go"), []byte("package main\n"), 0o644)
			affectedGit(t, root, "add", ".")
			affectedGit(t, root, "commit", "-qm", "advance HEAD past the snapshot")
		}, "snapshot: "},
		{"agent-instructions", func(t *testing.T, root string) {
			os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Agents\n"), 0o644)
		}, "AGENTS.md: present, does not name corvint"},
		{"hooks", func(t *testing.T, root string) {
			os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte("{}\n"), 0o644)
		}, ".claude/settings.json: present, no enabled corvint plugin or corvint adapter hook"},
		{"hooks", func(t *testing.T, root string) {
			os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte(`{"enabledPlugins":{"corvint@corvint":false}}`), 0o644)
		}, ".claude/settings.json: present, no enabled corvint plugin or corvint adapter hook"},
		{"hooks", func(t *testing.T, root string) {
			os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte(`{"enabledPlugins":{"corvint@corvint":true}`), 0o644)
		}, ".claude/settings.json: present, not valid JSON"},
		{"ignored-paths", func(t *testing.T, root string) {
			// `corvint index` writes .corvint/.gitignore for index/ and the self-observation
			// ledger; the opt-in unplanned-read ledger relies on the repository's rules.
			os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".context-corvint/\n"), 0o644)
		}, ".corvint/unplanned-reads.jsonl: not ignored .corvint/unplanned-reads.enabled: not ignored"},
		{"test-commands", func(t *testing.T, root string) {
			os.Remove(filepath.Join(root, ".github", "cishards", "test-commands.json"))
		}, ".github/cishards/test-commands.json: absent"},
		{"test-commands", func(t *testing.T, root string) {
			os.WriteFile(filepath.Join(root, ".github", "cishards", "test-commands.json"), []byte(`{"profile":"other/0","go":{}}`), 0o644)
		}, `profile is "other/0"`},
	} {
		t.Run(tc.class+"/"+tc.witness, func(t *testing.T) {
			root := setupFixture(t)
			tc.break_(t, root)
			for class, check := range runSetup(t, root) {
				gap := check["status"] == "gap"
				if gap != (class == tc.class) {
					t.Fatalf("%s: %v", class, check)
				}
				if gap && !strings.Contains(fmt.Sprint(check["evidence"]), tc.witness) {
					t.Fatalf("%s evidence: %v", class, check["evidence"])
				}
			}
		})
	}
}

func TestAdoptSetupRefusesOtherOptions(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"init", "--setup"}, {"adopt", "--setup", "--full-receipt"}, {"adopt", "--setup=1"}} {
		var stdout, stderr bytes.Buffer
		if code := runContext(context.Background(), append([]string{"--root", root}, args...), strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 {
			t.Fatalf("%v: exit=%d stdout=%s", args, code, stdout.String())
		}
	}
}

// ADS-V0-001: a check that cannot observe its input is `unobserved` and still
// carries a fix: a snapshot probe that fails (a cancelled read) and a PATH
// without git.
func TestAdoptSetupUnobservedChecksCarryAFix(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := setupFixture(t)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if check := setupIndexCheck(cancelled, root); check.Status != "unobserved" || check.Fix == "" || !strings.Contains(fmt.Sprint(check.Evidence), "error: ") {
		t.Fatalf("probe error: %+v", check)
	}
	t.Setenv("PATH", t.TempDir())
	if check := setupIgnoreCheck(context.Background(), root); check.Status != "unobserved" || check.Fix == "" || !strings.Contains(fmt.Sprint(check.Evidence), "git: not found") {
		t.Fatalf("no git: %+v", check)
	}
}

// ADS-V0-800..802: the hook fix names resources an adopter can resolve, the ignore check names the
// shared index store instead of asking to ignore .corvint/index/, and a readable declaration
// reports its commands and the declared languages corvint impact abstains on.
func TestADS800SetupFixesResolveInAdopterRepositories(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := setupFixture(t)
	os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte("{}\n"), 0o644)
	checks := runSetup(t, root)
	fix := fmt.Sprint(checks["hooks"]["fix"])
	if strings.Contains(fix, "see integrations/") || !strings.Contains(fix, "https://github.com/beamfall/corvint/tree/main/integrations/claude-code") {
		t.Fatalf("hooks fix names a Corvint-repo path: %q", fix)
	}
	ignore := fmt.Sprint(checks["ignored-paths"]["evidence"])
	if checks["ignored-paths"]["status"] != "ok" || strings.Contains(ignore, ".corvint/index/") || !strings.Contains(ignore, "index store: ") || !strings.Contains(ignore, filepath.Join("corvint", "index")+" is under the Git common directory") {
		t.Fatalf("ignored-paths does not match the real index location: %v", checks["ignored-paths"])
	}
	tests := fmt.Sprint(checks["test-commands"]["evidence"])
	for _, want := range []string{"go: GOTOOLCHAIN=local go test -count=1", "shell: make shell-test", "typescript: none (no suite)", "no corvint impact rule (impact abstains on their files): shell"} {
		if !strings.Contains(tests, want) {
			t.Fatalf("test-commands evidence lacks %q: %s", want, tests)
		}
	}
}
