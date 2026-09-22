package localcompletion

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

func localGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root, "-c", "user.name=Local Fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
	raw, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, raw)
	}
	return strings.TrimSpace(string(raw))
}

func fixture(t *testing.T, argv []string) (string, string, Plan) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	localGit(t, root, "init", "-q", "-b", "main")
	for name, body := range map[string]string{".gitignore": ".corvint/\n", "intent.md": "# Intent\n\n## Requirements\n\n- `FIXTURE-001`: preserve behavior.\n", "source.go": "package fixture\n"} {
		if err = os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	localGit(t, root, "add", ".")
	localGit(t, root, "commit", "-qm", "base")
	plan := Plan{Base: localGit(t, root, "rev-parse", "HEAD"), Intents: []string{"intent.md"}, Checks: []Check{{ID: "test", Argv: argv, TimeoutSeconds: 5}}}
	return root, HashSession(t.Name()), plan
}

func beginFixture(t *testing.T, root, key string, plan Plan) *repository {
	t.Helper()
	raw, _ := json.Marshal(plan)
	result, err := Begin(context.Background(), root, key, raw)
	if err != nil || result.Lifecycle != "active" {
		t.Fatalf("begin: %#v %v", result, err)
	}
	repo, err := open(root, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.load(); err != nil {
		t.Fatal(err)
	}
	return repo
}

func hasUnmet(result Evaluation, prefix string) bool {
	for _, value := range result.Unmet {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func TestEnrollmentAndReadOnlyPolicy(t *testing.T) {
	t.Run("LCP-V0-002 enrollment", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"true"})
		repo := beginFixture(t, root, key, plan)
		raw, _ := json.Marshal(plan)
		if _, err := Begin(context.Background(), root, HashSession("other"), raw); err == nil {
			t.Fatal("second owner accepted")
		}
		unlock, err := repo.lock()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Verify(context.Background(), root, key, "test"); err == nil {
			t.Fatal("interleaving accepted")
		}
		unlock()
		plan.Checks[0].Argv = []string{"false"}
		raw, _ = json.Marshal(plan)
		if _, err = Begin(context.Background(), root, key, raw); err == nil {
			t.Fatal("plan replacement accepted")
		}
		saved, _ := repo.load()
		if !filepath.IsAbs(saved.Executables[0]) {
			t.Fatal("execution path not frozen")
		}
		// An interrupted two-file begin may retain state but no owner. Retrying
		// the exact enrollment repairs ownership under the same mutation lock.
		if err = os.Remove(filepath.Join(repo.directory, "owner")); err != nil {
			t.Fatal(err)
		}
		raw, _ = json.Marshal(saved.Plan)
		if _, err = Begin(context.Background(), root, key, raw); err != nil {
			t.Fatal(err)
		}
		if err = repo.requireOwner(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("LCP-V0-003 readonly", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"true"})
		result, err := Evaluate(context.Background(), root, key)
		if err != nil || result.Lifecycle != "inactive" || result.Satisfied {
			t.Fatalf("inactive: %#v %v", result, err)
		}
		if _, err = os.Stat(filepath.Join(root, ".git/corvint/local-completion")); !os.IsNotExist(err) {
			t.Fatal("read created state")
		}
		repo := beginFixture(t, root, key, plan)
		before, err := os.ReadFile(repo.local("state.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(root, "untracked"), []byte("preserve"), 0600); err != nil {
			t.Fatal(err)
		}
		result, err = Evaluate(context.Background(), root, key)
		if err != nil || !hasUnmet(result, "uncommitted-work") {
			t.Fatalf("dirty: %#v %v", result, err)
		}
		assertNextAction(t, result, key, "status")
		repeated, err := Evaluate(context.Background(), root, key)
		if err != nil {
			t.Fatal(err)
		}
		assertNextAction(t, repeated, key, "status")
		after, _ := os.ReadFile(repo.local("state.json"))
		if string(before) != string(after) {
			t.Fatal("status mutated state")
		}
		localGit(t, root, "add", "untracked")
		localGit(t, root, "commit", "-qm", "preserve completed work")
		result, err = Evaluate(context.Background(), root, key)
		if err != nil {
			t.Fatal(err)
		}
		assertNextAction(t, result, key, "verify", "--check", "test")
		result, err = Cancel(context.Background(), root, key)
		if err != nil || result.Lifecycle != "cancelled" || result.Satisfied {
			t.Fatalf("cancel: %#v %v", result, err)
		}
		if _, err = os.Stat(filepath.Join(root, "untracked")); err != nil {
			t.Fatal("lost work")
		}
		if len(result.NextActions) != 0 {
			t.Fatal("cancelled enrollment suggests more work")
		}
	})
}

func TestEnrollmentRefusesIntentAbsentFromBase(t *testing.T) {
	t.Run("LCP-V0-002 unresolvable intent", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"true"})
		plan.Intents = []string{"docs/specs/never-committed.md"}
		raw, _ := json.Marshal(plan)
		if _, err := Begin(context.Background(), root, key, raw); err == nil || err.Error() != "intent-path-not-found" {
			t.Fatalf("begin: %v", err)
		}
		// The refusal happens before repo.save writes state.json, so no
		// enrollment record was ever produced: the session stays inactive.
		if _, err := os.Stat(filepath.Join(root, ".git/corvint/local-completion", key, "state.json")); !os.IsNotExist(err) {
			t.Fatal("refused enrollment recorded state.json")
		}
		evaluation, err := Evaluate(context.Background(), root, key)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if evaluation.Lifecycle != "inactive" {
			t.Fatalf("evaluate: got lifecycle %q, want inactive", evaluation.Lifecycle)
		}
	})
}

func TestRefusedEnrollmentConsumesNoGeneration(t *testing.T) {
	t.Run("LCP-V0-002 refusal leaves the generation bound intact", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"true"})
		valid, _ := json.Marshal(plan)
		plan.Intents = []string{"docs/specs/never-committed.md"}
		refused, _ := json.Marshal(plan)
		for attempt := 0; attempt < 16; attempt++ {
			if _, err := Begin(context.Background(), root, key, refused); err == nil || err.Error() != "intent-path-not-found" {
				t.Fatalf("begin attempt %d: %v", attempt, err)
			}
		}
		if _, err := Begin(context.Background(), root, key, valid); err != nil {
			t.Fatalf("valid begin after 16 refusals: %v", err)
		}
	})
}

func TestEnrollmentPinsSameChangeIntentAtHead(t *testing.T) {
	t.Run("LCP-V0-002 intent absent from base resolves at enrollment HEAD", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"true"})
		if err := os.WriteFile(filepath.Join(root, "proposed.md"), []byte("# Proposed\n"), 0600); err != nil {
			t.Fatal(err)
		}
		localGit(t, root, "add", "proposed.md")
		localGit(t, root, "commit", "-qm", "propose intent")
		head := localGit(t, root, "rev-parse", "HEAD")
		plan.Intents = []string{"proposed.md"}
		raw, _ := json.Marshal(plan)
		result, err := Begin(context.Background(), root, key, raw)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		want := IntentPointer{Path: "proposed.md", Revision: head, BlobHash: localGit(t, root, "rev-parse", head+":proposed.md")}
		if len(result.IntentPointers) != 1 || result.IntentPointers[0] != want {
			t.Fatalf("pointers: %#v, want %#v", result.IntentPointers, want)
		}
	})
}

func TestEnrollmentRefusesHeadNotDescendedFromBase(t *testing.T) {
	t.Run("LCP-V0-002 unrelated history", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"true"})
		// The orphan commit keeps the base tree, so only ancestry distinguishes it.
		localGit(t, root, "checkout", "-q", "--orphan", "unrelated")
		localGit(t, root, "commit", "-qm", "unrelated root")
		raw, _ := json.Marshal(plan)
		if _, err := Begin(context.Background(), root, key, raw); err == nil || err.Error() != "base-not-ancestor-of-target" {
			t.Fatalf("begin: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, ".git/corvint/local-completion", key, "state.json")); !os.IsNotExist(err) {
			t.Fatalf("refused enrollment recorded state.json: %v", err)
		}
	})
}

func TestFinishRefusesHeadMovedToUnrelatedHistory(t *testing.T) {
	t.Run("LCP-V0-002 head moved after begin", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"true"})
		beginFixture(t, root, key, plan)
		// A valid enrollment already exists; move HEAD to an orphan commit that
		// keeps the enrolled tree, so only ancestry distinguishes it from the base.
		localGit(t, root, "checkout", "-q", "--orphan", "unrelated")
		localGit(t, root, "commit", "-qm", "unrelated root")
		before, err := os.ReadFile(filepath.Join(root, ".git/corvint/local-completion", key, "state.json"))
		if err != nil {
			t.Fatal(err)
		}
		result, err := Finish(context.Background(), root, key, nil)
		if err != nil {
			t.Fatalf("finish: %v", err)
		}
		if !hasUnmet(result, "base-not-ancestor-of-target") || result.Satisfied {
			t.Fatalf("finish: got %#v, want unmet base-not-ancestor-of-target and not satisfied", result)
		}
		after, err := os.ReadFile(filepath.Join(root, ".git/corvint/local-completion", key, "state.json"))
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Fatal("finish wrote local state despite unrelated HEAD")
		}
		status, err := Evaluate(context.Background(), root, key)
		if err != nil || !hasUnmet(status, "base-not-ancestor-of-target") || status.Satisfied {
			t.Fatalf("status: %#v %v, want unmet base-not-ancestor-of-target and not satisfied", status, err)
		}
		assertNextAction(t, status, key, "status")
	})
}

func TestActualVerificationAndSecretRefusal(t *testing.T) {
	t.Run("LCP-V0-004 verification", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"printf", "%s", "literal $(touch should-not-exist)"})
		repo := beginFixture(t, root, key, plan)
		result, err := Verify(context.Background(), root, key, "test")
		if err != nil || hasUnmet(result, "selected-check-unverified") {
			t.Fatalf("verify: %#v %v", result, err)
		}
		saved, _ := repo.load()
		observed := saved.Observations[0]
		raw, _ := os.ReadFile(observed.Stdout.Path)
		if string(raw) != "literal $(touch should-not-exist)" {
			t.Fatalf("argv interpreted: %s", raw)
		}
		if err = os.WriteFile(observed.Stdout.Path, []byte("changed"), 0600); err != nil {
			t.Fatal(err)
		}
		result, err = Evaluate(context.Background(), root, key)
		if err != nil || !hasUnmet(result, "selected-check-unverified") {
			t.Fatal("changed log qualified")
		}
	})
	t.Run("failed", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"false"})
		beginFixture(t, root, key, plan)
		result, err := Verify(context.Background(), root, key, "test")
		if err != nil || !hasUnmet(result, "selected-check-unverified") {
			t.Fatalf("failed check qualified: %#v %v", result, err)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"sleep", "5"})
		plan.Checks[0].TimeoutSeconds = 1
		repo := beginFixture(t, root, key, plan)
		result, err := Verify(context.Background(), root, key, "test")
		if err != nil || !hasUnmet(result, "selected-check-unverified") {
			t.Fatalf("timeout qualified: %#v %v", result, err)
		}
		saved, loadErr := repo.load()
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		if !saved.Observations[0].TimedOut {
			t.Fatal("timeout lost")
		}
	})
	t.Run("secret-plan", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"printf", "token=fixture-secret"})
		raw, _ := json.Marshal(plan)
		if _, err := Begin(context.Background(), root, key, raw); err == nil {
			t.Fatal("secret plan stored")
		}
		if _, err := os.Stat(filepath.Join(root, ".git/corvint/local-completion")); !os.IsNotExist(err) {
			t.Fatal("secret rejection wrote state")
		}
	})
	t.Run("secret-log", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"sh", "-c", "printf '%s%s' 'tok' 'en=fixture-secret'"})
		repo := beginFixture(t, root, key, plan)
		result, err := Verify(context.Background(), root, key, "test")
		if err != nil || !hasUnmet(result, "selected-check-unverified") {
			t.Fatalf("secret output qualified: %#v %v", result, err)
		}
		saved, _ := repo.load()
		if !saved.Observations[0].SecretScreened {
			t.Fatal("missing explicit refusal")
		}
		filepath.WalkDir(repo.directory, func(name string, entry os.DirEntry, walkErr error) error {
			if walkErr == nil && !entry.IsDir() {
				raw, _ := os.ReadFile(name)
				if strings.Contains(string(raw), "token=fixture-secret") {
					t.Errorf("secret persisted: %s", name)
				}
			}
			return walkErr
		})
	})
	t.Run("go-verbose-pass-log", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"sh", "-c", "printf '%b%b' '--- PA' 'SS: TestExample (0.00s)\\n'"})
		repo := beginFixture(t, root, key, plan)
		result, err := Verify(context.Background(), root, key, "test")
		if err != nil || hasUnmet(result, "selected-check-unverified") {
			t.Fatalf("verbose Go pass output did not qualify: %#v %v", result, err)
		}
		saved, _ := repo.load()
		if saved.Observations[0].SecretScreened {
			t.Fatal("verbose Go pass marker was secret-screened")
		}
		output, err := os.ReadFile(saved.Observations[0].Stdout.Path)
		if err != nil || string(output) != "--- PASS: TestExample (0.00s)\n" {
			t.Fatalf("verbose Go pass output = %q, %v", output, err)
		}
	})
	t.Run("go-verbose-pass-log-with-secret", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"sh", "-c", "printf '%b%b%b%b' '--- PA' 'SS: TestExample (0.00s)\\n' 'pa' 'ss: synthetic123\\n'"})
		repo := beginFixture(t, root, key, plan)
		result, err := Verify(context.Background(), root, key, "test")
		if err != nil || !hasUnmet(result, "selected-check-unverified") {
			t.Fatalf("real secret after verbose Go pass output qualified: %#v %v", result, err)
		}
		saved, _ := repo.load()
		if !saved.Observations[0].SecretScreened {
			t.Fatal("real secret after verbose Go pass marker was not screened")
		}
	})
	t.Run("go-verbose-pass-log-with-embedded-secret", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"sh", "-c", "printf '\\055\\055\\055\\040\\120\\101\\123\\123\\072\\040\\124\\145\\163\\164\\105\\170\\141\\155\\160\\154\\145\\057\\160\\141\\163\\163\\075\\163\\171\\156\\164\\150\\145\\164\\151\\143\\061\\062\\063\\040\\050\\060\\056\\060\\060\\163\\051'"})
		repo := beginFixture(t, root, key, plan)
		result, err := Verify(context.Background(), root, key, "test")
		if err != nil || !hasUnmet(result, "selected-check-unverified") {
			t.Fatalf("secret in verbose Go pass test name qualified: %#v %v", result, err)
		}
		saved, _ := repo.load()
		if !saved.Observations[0].SecretScreened {
			t.Fatal("secret in verbose Go pass test name was not screened")
		}
	})
}

func TestExplicitSidecarReuseRequiresCanonicalMap(t *testing.T) {
	root, key, plan := fixture(t, []string{"true"})
	plan.Checks[0].AllowCemSidecarOnlyReuse = true
	os.WriteFile(filepath.Join(root, "source.go"), []byte("package fixture\nconst Answer = 42\n"), 0600)
	localGit(t, root, "add", "source.go")
	localGit(t, root, "commit", "-qm", "change")
	repo := beginFixture(t, root, key, plan)
	if _, err := Verify(context.Background(), root, key, "test"); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	for _, args := range [][]string{{"prepare", "--base", plan.Base, "--target", "HEAD"}, {"cite", "--map", ".corvint/change.cem.json", "--hunk", "1", "--evidence-path", "intent.md", "--lines", "1:5", "--relation", "specification"}} {
		if cli.Run(context.Background(), root, args, &stdout, &stderr) != 0 {
			t.Fatalf("cem %v: %s", args, stderr.String())
		}
	}
	localGit(t, root, "add", "-f", ".corvint/change.cem.json")
	localGit(t, root, "commit", "-qm", "sidecar")
	result, err := Evaluate(context.Background(), root, key)
	if err != nil || hasUnmet(result, "selected-check-unverified") {
		t.Fatalf("valid sidecar reuse: %#v %v", result, err)
	}
	saved, _ := repo.load()
	if saved.Observations[0].Target == result.Target {
		t.Fatal("reused check rewritten as current execution")
	}
	os.WriteFile(filepath.Join(root, "source.go"), []byte("package fixture\nconst Answer = 43\n"), 0600)
	localGit(t, root, "add", "source.go")
	localGit(t, root, "commit", "-qm", "drift")
	result, err = Evaluate(context.Background(), root, key)
	if err != nil || !hasUnmet(result, "selected-check-unverified") {
		t.Fatal("source drift reused")
	}
}

func TestReenrollmentPreservesCompletedGenerations(t *testing.T) {
	root, key, plan := fixture(t, []string{"true"})
	repo := beginFixture(t, root, key, plan)
	if _, err := Verify(context.Background(), root, key, "test"); err != nil {
		t.Fatal(err)
	}
	saved, err := repo.load()
	if err != nil {
		t.Fatal(err)
	}
	originalLog := saved.Observations[0].Stdout
	if _, err = Cancel(context.Background(), root, key); err != nil {
		t.Fatal(err)
	}
	// Explicit enrollment of the same cancelled plan starts a fresh bounded
	// generation while leaving the original actual observation readable.
	raw, _ := json.Marshal(plan)
	result, err := Begin(context.Background(), root, key, raw)
	if err != nil || result.Lifecycle != "active" || !hasUnmet(result, "selected-check-unverified") {
		t.Fatalf("same-plan reenroll: %#v %v", result, err)
	}
	current, _ := repo.load()
	if current.Generation == saved.Generation {
		t.Fatal("reused prior generation")
	}
	if !artifactsCurrent([]artifact{originalLog}) {
		t.Fatal("prior log changed")
	}
	if _, err = os.Stat(filepath.Join(repo.directory, key, saved.Generation, "state.json")); err != nil {
		t.Fatal("history missing")
	}
	if _, err = Cancel(context.Background(), root, key); err != nil {
		t.Fatal(err)
	}
	plan.Checks[0].Argv = []string{"printf", "new"}
	raw, _ = json.Marshal(plan)
	result, err = Begin(context.Background(), root, key, raw)
	if err != nil || result.Lifecycle != "active" {
		t.Fatalf("revised plan: %#v %v", result, err)
	}
}

func TestGenerationDirectoryBounds(t *testing.T) {
	t.Run("LCP-V0-012 bound", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"true"})
		repo := beginFixture(t, root, key, plan)
		saved, err := repo.load()
		if err != nil {
			t.Fatal(err)
		}
		directory := filepath.Join(repo.directory, key)
		for count := 2; count <= 15; count++ {
			if err := os.Mkdir(filepath.Join(directory, "generation-"+strconv.Itoa(count)), 0700); err != nil {
				t.Fatal(err)
			}
		}
		if next, err := repo.nextGeneration(saved.PlanDigest); err != nil || !strings.HasSuffix(next, "-016") {
			t.Fatalf("last permitted generation: %q %v", next, err)
		}
		if err := repo.archiveGeneration(saved); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(directory, "generation-16"), 0700); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.nextGeneration(saved.PlanDigest); err == nil || err.Error() != "enrollment-generation-bound-exceeded" {
			t.Fatalf("seventeenth generation admitted: %v", err)
		}
		if err := repo.archiveGeneration(saved); err == nil || err.Error() != "enrollment-generation-bound-exceeded" {
			t.Fatalf("archive ignored generation bound: %v", err)
		}
		// Non-directory entries also count against the enumeration budget.
		if err := os.WriteFile(filepath.Join(directory, "unexpected"), nil, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.generationCount(); err == nil || err.Error() != "enrollment-generation-bound-exceeded" {
			t.Fatalf("extra entry ignored: %v", err)
		}
	})
}

func TestOriginalReceiptsAreArchivedOrExplicitlySecretRefused(t *testing.T) {
	root, key, plan := fixture(t, []string{"true"})
	directory := filepath.Join(root, ".git/corvint")
	os.MkdirAll(directory, 0700)
	original := []byte(`{"task":"caller-owned prechange question"}`)
	secret := []byte(`{"task":"token=fixture-secret"}`)
	os.WriteFile(filepath.Join(directory, "prechange-query.json"), original, 0600)
	os.WriteFile(filepath.Join(directory, "prechange-impact.json"), secret, 0600)
	repo := beginFixture(t, root, key, plan)
	archived, err := os.ReadFile(repo.local("initial-prechange-query.json"))
	if err != nil || string(archived) != string(original) {
		t.Fatal("original chronology lost")
	}
	if _, err = os.Stat(repo.local("initial-prechange-impact.json")); !os.IsNotExist(err) {
		t.Fatal("secret duplicated")
	}
	refusal, err := os.ReadFile(repo.local("initial-prechange-impact.json.refusal.json"))
	if err != nil || strings.Contains(string(refusal), "fixture-secret") || !strings.Contains(string(refusal), digest(secret)) {
		t.Fatal("missing bounded refusal")
	}
	retained, _ := os.ReadFile(filepath.Join(directory, "prechange-impact.json"))
	if string(retained) != string(secret) {
		t.Fatal("original modified")
	}
}

func TestNoSourceAdmissionDoesNotRequireTrace(t *testing.T) {
	root, key, plan := fixture(t, []string{"true"})
	beginFixture(t, root, key, plan)
	os.WriteFile(filepath.Join(root, "image.png"), []byte{0, 1, 2, 3}, 0600)
	localGit(t, root, "add", "image.png")
	localGit(t, root, "commit", "-qm", "binary asset")
	target := localGit(t, root, "rev-parse", "HEAD")
	recorded, err := tracerecordrepo.RecordDogfood(context.Background(), root, plan.Base, target, tracerecordrepo.Input{Task: "fixture no source", Verification: []string{"true"}, Outcome: "passed"})
	if err != nil || recorded.State != "no-source-paths" || recorded.Recorded != nil {
		t.Fatalf("actual admission: %#v %v", recorded, err)
	}
	repo, _ := open(root, key)
	saved, _ := repo.load()
	snap, _ := repo.snapshot(context.Background())
	evidence, _ := json.Marshal(map[string]any{"ok": true, "state": recorded.State, "base": recorded.Base, "target": recorded.Target, "admitted": recorded.Admitted})
	writeFile(filepath.Join(root, ".git/corvint/local-outcome.json"), evidence)
	report, _ := json.Marshal(map[string]any{"base": plan.Base, "target": target, "complete": true, "localOutcomeEvidenceSha256": "sha256:" + digest(evidence)})
	writeFile(filepath.Join(root, ".corvint/dogfood-report.json"), report)
	if err = repo.checkCoordinator(saved, snap, false); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, ".context-corvint/traces")); !os.IsNotExist(err) {
		t.Fatal("no-source created traces")
	}
}

func TestContextAbstentionTerminalArtifactsRemainCurrent(t *testing.T) {
	root, key, plan := fixture(t, []string{"true"})
	repo := beginFixture(t, root, key, plan)
	files := map[string][]byte{
		filepath.Join(root, ".corvint/dogfood-report.json"):                  []byte(`{"contextAbstentionEvidenceSha256":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`),
		filepath.Join(root, ".corvint/change.ocm-status.json"):               []byte("ocm\n"),
		filepath.Join(root, ".git/corvint/local-outcome.json"):               []byte("outcome\n"),
		repo.local("final-check.stdout"):                                     []byte("stdout\n"),
		repo.local("final-check.stderr"):                                     []byte("stderr\n"),
		filepath.Join(root, ".git/corvint/prechange-impact-abstention.json"): []byte("artifact\n"),
		filepath.Join(root, ".git/corvint/prechange-impact.argv"):            []byte("argv\x00"),
		filepath.Join(root, ".git/corvint/prechange-impact.json"):            []byte{},
		filepath.Join(root, ".git/corvint/prechange-impact.stderr"):          []byte("refusal\n"),
	}
	for name, raw := range files {
		if err := writeFile(name, raw); err != nil {
			t.Fatal(err)
		}
	}
	paths := repo.terminalPaths()
	if len(paths) != 9 {
		t.Fatalf("terminal paths=%v", paths)
	}
	artifacts := make([]artifact, 0, len(paths))
	for _, name := range paths {
		item, err := artifactFor(name)
		if err != nil {
			t.Fatal(err)
		}
		artifacts = append(artifacts, item)
	}
	terminal := &terminal{Artifacts: artifacts}
	if !repo.terminalCurrent(terminal) {
		t.Fatal("fresh context evidence is not current")
	}
	for _, name := range paths[5:] {
		original := files[name]
		if err := writeFile(name, append(append([]byte{}, original...), 'x')); err != nil {
			t.Fatal(err)
		}
		if repo.terminalCurrent(terminal) {
			t.Fatalf("terminal remained current after %s drift", filepath.Base(name))
		}
		if err := writeFile(name, original); err != nil {
			t.Fatal(err)
		}
	}
}

func TestExecutionPhasesDoNotReuseExpiredGitReadBudgets(t *testing.T) {
	root, key, plan := fixture(t, []string{"true"})
	repo := beginFixture(t, root, key, plan)
	expired, err := gitauth.Open(root, gitrun.NewBudget(1, time.Nanosecond))
	if err != nil {
		t.Fatal(err)
	}
	repo.auth = expired
	if _, err = expired.Resolve(context.Background(), "HEAD"); err == nil {
		t.Fatal("fixture budget was not expired")
	}
	snapshot, err := repo.snapshot(context.Background())
	if err != nil || snapshot.target != plan.Base || !snapshot.clean {
		t.Fatalf("next read phase inherited execution wall time: %#v %v", snapshot, err)
	}
}

func TestMalformedNullScalarsCannotBecomeSuccessfulEvidence(t *testing.T) {
	root, key, plan := fixture(t, []string{"true"})
	repo := beginFixture(t, root, key, plan)
	if _, err := Verify(context.Background(), root, key, "test"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(repo.local("state.json"))
	for _, field := range []string{"exit", "timedOut", "cancelled"} {
		t.Run(field, func(t *testing.T) {
			var object map[string]any
			json.Unmarshal(raw, &object)
			object["observations"].([]any)[0].(map[string]any)[field] = nil
			changed, _ := json.Marshal(object)
			os.WriteFile(repo.local("state.json"), changed, 0600)
			if _, err := repo.load(); err == nil {
				t.Fatal("null observation scalar accepted")
			}
		})
	}
	var object map[string]any
	json.Unmarshal(raw, &object)
	object["terminal"] = map[string]any{"reportSet": strings.Repeat("0", 64), "checkExit": nil, "artifacts": []any{}}
	changed, _ := json.Marshal(object)
	os.WriteFile(repo.local("state.json"), changed, 0600)
	if _, err := repo.load(); err == nil {
		t.Fatal("null terminal check exit accepted")
	}
}
