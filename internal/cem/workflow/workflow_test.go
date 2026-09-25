package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1", "HOME="+t.TempDir(), "XDG_CONFIG_HOME="+t.TempDir(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
		"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000")
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// makeRepo builds base and target commits: the target modifies one file and
// creates another.
func makeRepo(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "docs/rule.txt", "the frozen rule\nsecond line\nthird line\n")
	writeFile(t, root, "src/app.txt", "alpha\nbeta\ngamma\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "src/app.txt", "alpha\nBETA\ngamma\n")
	writeFile(t, root, "src/new.txt", "created\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "target")
	target := gitCmd(t, root, "rev-parse", "HEAD")
	return root, base, target
}

func openSession(t *testing.T, root string) *Session {
	t.Helper()
	session, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func ctx() context.Context { return context.Background() }

// citeAll cites every open hunk to the frozen rule via --lines.
func citeAll(t *testing.T, root string) {
	t.Helper()
	for {
		session := openSession(t, root)
		_, document, err := session.readMapInput(wire.ExcludedCEMPath)
		if err != nil {
			t.Fatal(err)
		}
		open := ""
		for _, hunk := range document.Hunks {
			if hunk.Disposition != "supported" {
				open = hunk.ID
				break
			}
		}
		if open == "" {
			return
		}
		_, err = session.Cite(ctx(), CiteOptions{
			MapPath: wire.ExcludedCEMPath, Hunk: open, EvidencePath: "docs/rule.txt",
			Lines: "1:2", Relation: "specification",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// TestTwoPhaseCanonicalFlow is CEM-CB-AT-011 / EX-001: prepare, cite, commit
// the candidate, then canonically verify HEAD with the exact envelope.
func TestTwoPhaseCanonicalFlow(t *testing.T) {
	root, base, target := makeRepo(t)
	session := openSession(t, root)
	prepared, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	if prepared["resumed"] != false {
		t.Fatal("fresh prepare reported resumed")
	}
	action := prepared["nextActions"].([]any)[0].([]any)
	rendered, _ := json.Marshal(action)
	if strings.Contains(string(rendered), "--patch") || !strings.Contains(string(rendered), `"--target","HEAD"`) {
		t.Fatalf("canonical next action malformed: %s", rendered)
	}
	// CEM-CB-016 / decision 0122: argv[0] is the contract command name, not the build name.
	if action[0] != "corvint" {
		t.Fatalf("next action command name = %v, want corvint", action[0])
	}
	citeAll(t, root)
	// The worktree still holds the target checkout; commit the candidate to
	// create the final revision the consumer independently resolves as HEAD.
	gitCmd(t, root, "add", wire.ExcludedCEMPath)
	gitCmd(t, root, "commit", "-qm", "candidate")
	// CEM-CB-AT-001: the convenience cache is not patch authority.
	if err := os.Remove(filepath.Join(session.gitRoot.Path(), "corvint", "change.patch")); err != nil {
		t.Fatal(err)
	}
	result, err := openSession(t, root).Read(ctx(), "status", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result["ok"] != true || result["state"] != "ready-for-ci" {
		encoded, _ := json.Marshal(result)
		t.Fatalf("canonical status not green: %s", encoded)
	}
	if result["patch"] != nil || result["patchSource"] != "canonical-derived" ||
		result["excludedPath"] != wire.ExcludedCEMPath || len(result["warnings"].([]any)) != 0 {
		t.Fatalf("frozen 0.2 status envelope violated: %+v", result)
	}
	verification := result["verification"].(map[string]any)
	if verification["assurance"] != "canonical" || verification["valid"] != true {
		t.Fatalf("verification: %+v", verification)
	}
	// verify and report must not carry the legacy patch field.
	verified, err := openSession(t, root).Read(ctx(), "verify", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, present := verified["patch"]; present {
		t.Fatal("0.2 verify carries a legacy patch field")
	}
	if verified["ok"] != true {
		encoded, _ := json.Marshal(verified)
		t.Fatalf("verify not green: %s", encoded)
	}
}

func TestCanonicalArgumentPrecedence(t *testing.T) {
	root, base, target := makeRepo(t)
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	// CEM-CB-AT-003: --patch on a 0.2 read fails invalid-arguments.
	_, err := session.Read(ctx(), "status", ReadOptions{
		MapPath: wire.ExcludedCEMPath, PatchGiven: true, PatchPath: "x.patch",
		ExpectedBase: base, Target: "HEAD",
	})
	if cemcode.CodeOf(err) != cemcode.InvalidArguments {
		t.Fatalf("got %v", err)
	}
	// CEM-CB-AT-004: expected-base before target.
	_, err = session.Read(ctx(), "verify", ReadOptions{MapPath: wire.ExcludedCEMPath})
	if cemcode.CodeOf(err) != cemcode.ExpectedBaseRequired {
		t.Fatalf("got %v", err)
	}
	_, err = session.Read(ctx(), "verify", ReadOptions{MapPath: wire.ExcludedCEMPath, ExpectedBase: base})
	if cemcode.CodeOf(err) != cemcode.TargetRequired {
		t.Fatalf("got %v", err)
	}
	// CEM-CB-AT-007: substituted base fails before derivation, surfaced in the
	// failure envelope (verification failures never emit a partial success).
	mismatched, err := session.Read(ctx(), "verify", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: target, Target: "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}
	verification := mismatched["verification"].(map[string]any)
	code := verification["issues"].([]any)[0].(map[string]any)["code"]
	if mismatched["ok"] != false || code != cemcode.BaseRevisionMismatch {
		t.Fatalf("substituted base: ok=%v code=%v", mismatched["ok"], code)
	}
}

// TestHiddenTargetChangeRejected is CEM-CB-AT-002: a target containing an
// extra hunk fails patch-digest-mismatch.
func TestHiddenTargetChangeRejected(t *testing.T) {
	root, base, target := makeRepo(t)
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	citeAll(t, root)
	writeFile(t, root, "src/hidden.txt", "smuggled\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "candidate plus hidden change")
	result, err := openSession(t, root).Read(ctx(), "status", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}
	verification := result["verification"].(map[string]any)
	issues := verification["issues"].([]any)
	if result["ok"] != false || verification["valid"] != false || len(issues) == 0 {
		t.Fatalf("status did not surface the failure: %+v", verification)
	}
	code := issues[0].(map[string]any)["code"]
	if code != cemcode.PatchDigestMismatch && code != cemcode.ExcludedArtifactMismatch {
		t.Fatalf("issue code %v", code)
	}
}

// TestTargetSidecarRawBinding is CEM-CB-EX-005: byte-identical passes; a
// one-byte mutation of the read input fails excluded-artifact-mismatch.
func TestTargetSidecarRawBinding(t *testing.T) {
	root, base, target := makeRepo(t)
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	citeAll(t, root)
	gitCmd(t, root, "add", wire.ExcludedCEMPath)
	gitCmd(t, root, "commit", "-qm", "candidate")
	mapPath := filepath.Join(root, ".corvint", "change.cem.json")
	pristine, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openSession(t, root).Read(ctx(), "verify", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: "HEAD",
	}); err != nil {
		t.Fatalf("byte-identical input rejected: %v", err)
	}
	// A semantically equivalent reserialization: one inserted space keeps the
	// JSON valid but changes the raw bytes bound to the committed sidecar.
	mutated := append([]byte("{ "), pristine[1:]...)
	if err := os.WriteFile(mapPath, mutated, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := openSession(t, root).Read(ctx(), "status", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}
	verification := result["verification"].(map[string]any)
	code := verification["issues"].([]any)[0].(map[string]any)["code"]
	if verification["valid"] != false || code != cemcode.ExcludedArtifactMismatch {
		t.Fatalf("mutated input: valid=%v code=%v", verification["valid"], code)
	}
}

// TestResumeRules is CEM-CB-CO-004: resumed 0.1 maps derive only the current
// invocation's patch selection, and non-default prepare maps are refused.
func TestResumeRules(t *testing.T) {
	root, base, target := makeRepo(t)
	session := openSession(t, root)
	prepared, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	cachePath := prepared["patch"].(string)
	// Replace the 0.2 candidate with a 0.1 map over the same canonical bytes.
	if err := os.Remove(filepath.Join(root, ".corvint", "change.cem.json")); err != nil {
		t.Fatal(err)
	}
	cacheBytes, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "exact.patch", string(cacheBytes))
	if _, err := openSession(t, root).Begin(ctx(), BeginOptions{
		PatchPath: "exact.patch", Base: base, Output: wire.ExcludedCEMPath,
	}); err != nil {
		t.Fatal(err)
	}
	// Resume with omitted --patch: next action has no --patch.
	first, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	if first["resumed"] != true {
		t.Fatal("0.1 map at default path did not resume")
	}
	firstAction, _ := json.Marshal(first["nextActions"])
	if strings.Contains(string(firstAction), "--patch") || strings.Contains(string(firstAction), "--expected-base") {
		t.Fatalf("resumed 0.1 omitted-patch action: %s", firstAction)
	}
	// Resume with explicit --patch: the action carries exactly that selection.
	second, err := openSession(t, root).Prepare(ctx(), PrepareOptions{
		Base: base, Target: target, Cache: "alt/cache.patch",
	})
	if err != nil {
		t.Fatal(err)
	}
	secondAction, _ := json.Marshal(second["nextActions"])
	if !strings.Contains(string(secondAction), `"--patch","alt/cache.patch"`) {
		t.Fatalf("resumed 0.1 explicit-patch action: %s", secondAction)
	}
	// Non-default prepare map path fails invalid-arguments with no write.
	_, err = openSession(t, root).Prepare(ctx(), PrepareOptions{
		Base: base, Target: target, MapPath: "custom.json",
	})
	if cemcode.CodeOf(err) != cemcode.InvalidArguments {
		t.Fatalf("got %v", err)
	}
}

// TestUnsafeExistingMapNotOverwritten: an unparseable existing candidate is
// refused without --replace and regenerated with it.
func TestUnsafeExistingMapNotOverwritten(t *testing.T) {
	root, base, target := makeRepo(t)
	writeFile(t, root, ".corvint/change.cem.json", "not json")
	_, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target})
	if err == nil {
		t.Fatal("unsafe existing map silently overwritten")
	}
	data, readErr := os.ReadFile(filepath.Join(root, ".corvint", "change.cem.json"))
	if readErr != nil || string(data) != "not json" {
		t.Fatal("refusal mutated the existing map")
	}
	prepared, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target, Replace: true})
	if err != nil || prepared["resumed"] != false {
		t.Fatalf("explicit replace failed: %v", err)
	}
}

// TestMapReadThroughSymlinkedParentIsUnavailable is CEM-PILOT-013: a map read
// whose parent directory is a symlink refuses with the read's own
// `map-unavailable`, never the write-side `publish-failed`.
func TestMapReadThroughSymlinkedParentIsUnavailable(t *testing.T) {
	root, base, target := makeRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(".corvint", filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	_, err := openSession(t, root).Read(ctx(), "verify", ReadOptions{MapPath: "linked/change.cem.json", ExpectedBase: base})
	if cemcode.CodeOf(err) != cemcode.MapUnavailable {
		t.Fatalf("map read through a symlinked parent: %v", err)
	}
}

// TestLegacyEnvelopeRows fixes the frozen 0.1 rows of the envelope table.
func TestLegacyEnvelopeRows(t *testing.T) {
	root, base, target := makeRepo(t)
	session := openSession(t, root)
	prepared, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	cacheBytes, err := os.ReadFile(prepared["patch"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".corvint", "change.cem.json")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "exact.patch", string(cacheBytes))
	if _, err := openSession(t, root).Begin(ctx(), BeginOptions{
		PatchPath: "exact.patch", Base: base, Output: wire.ExcludedCEMPath,
	}); err != nil {
		t.Fatal(err)
	}
	// Explicit --patch row.
	explicit, err := openSession(t, root).Read(ctx(), "status", ReadOptions{
		MapPath: wire.ExcludedCEMPath, PatchGiven: true, PatchPath: "exact.patch",
	})
	if err != nil {
		t.Fatal(err)
	}
	if explicit["patchSource"] != "explicit-out-of-band" || explicit["excludedPath"] != nil {
		t.Fatalf("explicit row: %+v", explicit)
	}
	if warnings, _ := json.Marshal(explicit["warnings"]); string(warnings) != `["patch-supplied-out-of-band"]` {
		t.Fatalf("explicit warnings: %s", warnings)
	}
	if explicit["patch"] != filepath.Join(root, "exact.patch") {
		t.Fatalf("explicit patch field: %v", explicit["patch"])
	}
	verification := explicit["verification"].(map[string]any)
	if verification["valid"] != true || verification["targetRevision"] != nil {
		t.Fatalf("0.1 no-target verification: %+v", verification)
	}
	if _, present := verification["assurance"]; present {
		t.Fatal("0.1 verification carries assurance")
	}
	// Omitted --patch row reads the per-worktree default written by prepare.
	// (prepare wrote it before the map was replaced by the 0.1 begin).
	omitted, err := openSession(t, root).Read(ctx(), "verify", ReadOptions{MapPath: wire.ExcludedCEMPath})
	if err != nil {
		t.Fatal(err)
	}
	if omitted["patchSource"] != "default-out-of-band" {
		t.Fatalf("omitted row: %+v", omitted)
	}
	if warnings, _ := json.Marshal(omitted["warnings"]); string(warnings) != `["patch-defaulted-out-of-band"]` {
		t.Fatalf("omitted warnings: %s", warnings)
	}
	if _, present := omitted["patch"]; present {
		t.Fatal("verify carries a legacy patch field")
	}
	// Optional matching expected base passes; mismatch fails.
	if _, err := openSession(t, root).Read(ctx(), "verify", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base,
	}); err != nil {
		t.Fatalf("matching expected base: %v", err)
	}
	_, err = openSession(t, root).Read(ctx(), "verify", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: target,
	})
	failed, _ := openSession(t, root).Read(ctx(), "status", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: target,
	})
	failedVerification := failed["verification"].(map[string]any)
	if failedVerification["valid"] != false {
		t.Fatal("mismatched expected base accepted")
	}
	_ = err
}

// TestMutationPreservesProfile is CEM-CB-CO-003.
func TestMutationPreservesProfile(t *testing.T) {
	root, base, target := makeRepo(t)
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Mark(ctx(), MarkOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", Disposition: "unknown", Reason: "insufficient-evidence",
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".corvint", "change.cem.json"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := wire.ParseMap(data)
	if err != nil {
		t.Fatalf("mutated 0.2 map no longer parses: %v", err)
	}
	if document.Spec != wire.Spec02 || document.ExcludedPath != wire.ExcludedCEMPath {
		t.Fatal("mark dropped the 0.2 profile or excludedPath")
	}
}

// TestConcurrentMarksAllSurvive is CEM-PILOT-013: concurrent in-place marks
// serialize their read-modify-write, so every successful mark survives.
func TestConcurrentMarksAllSurvive(t *testing.T) {
	const hunks = 12
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	for index := 1; index <= hunks; index++ {
		writeFile(t, root, fmt.Sprintf("f%02d.txt", index), "before\n")
	}
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	for index := 1; index <= hunks; index++ {
		writeFile(t, root, fmt.Sprintf("f%02d.txt", index), "after\n")
	}
	gitCmd(t, root, "commit", "-qam", "target")
	target := gitCmd(t, root, "rev-parse", "HEAD")
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	failures := make(chan error, hunks)
	for index := 1; index <= hunks; index++ {
		group.Add(1)
		go func(ordinal int) {
			defer group.Done()
			session, err := Open(root)
			if err == nil {
				_, err = session.Mark(ctx(), MarkOptions{
					MapPath: wire.ExcludedCEMPath, Hunk: fmt.Sprint(ordinal),
					Disposition: "unknown", Reason: "insufficient-evidence",
				})
			}
			failures <- err
		}(index)
	}
	group.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatalf("concurrent mark failed: %v", err)
		}
	}
	_, document, err := openSession(t, root).readMapInput(wire.ExcludedCEMPath)
	if err != nil {
		t.Fatal(err)
	}
	survived := 0
	for _, hunk := range document.Hunks {
		if hunk.Reason == "insufficient-evidence" {
			survived++
		}
	}
	if survived != hunks {
		t.Fatalf("%d of %d concurrent marks survived", survived, hunks)
	}
	if _, err := os.Lstat(filepath.Join(root, ".corvint", "change.cem.json.lock")); !os.IsNotExist(err) {
		t.Fatalf("map lock file left behind: %v", err)
	}
}

// TestMutationsLockTheirEffectiveOutput is CEM-PILOT-013: an explicit output
// serializes with every other mutation of that output, even when its input map
// is a different artifact.
func TestMutationsLockTheirEffectiveOutput(t *testing.T) {
	operations := map[string]func(*Session, string) error{
		"cite": func(session *Session, output string) error {
			_, err := session.Cite(ctx(), CiteOptions{
				MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "docs/rule.txt",
				Lines: "1:1", Relation: "specification", Output: output,
			})
			return err
		},
		"mark": func(session *Session, output string) error {
			_, err := session.Mark(ctx(), MarkOptions{
				MapPath: wire.ExcludedCEMPath, Hunk: "1", Disposition: "unknown",
				Reason: "insufficient-evidence", Output: output,
			})
			return err
		},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			root, base, target := makeRepo(t)
			session := openSession(t, root)
			if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
				t.Fatal(err)
			}
			mutationSession := openSession(t, root)
			output := ".corvint/staged.cem.json"
			releaseOutput, err := session.workRoot.Lock(output)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { releaseOutput() }()

			requested := make(chan string, 1)
			previous := lockMap
			lockMap = func(root *publish.Root, relative string) (func(), error) {
				requested <- relative
				return previous(root, relative)
			}
			t.Cleanup(func() { lockMap = previous })
			finished := make(chan error, 1)
			go func() { finished <- operation(mutationSession, output) }()

			var locked string
			select {
			case locked = <-requested:
			case <-time.After(30 * time.Second):
				t.Fatal("mutation did not reach its lock within the hang detector")
			}
			releaseOutput()
			releaseOutput = func() {}
			select {
			case err := <-finished:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(30 * time.Second):
				t.Fatal("mutation did not finish after the output lock was released")
			}
			if locked != output {
				t.Fatalf("mutation locked %q instead of effective output %q", locked, output)
			}
		})
	}
}

// TestMutationLockNeverRemovesAnInputMap is CEM-PILOT-013: an input map that
// happens to occupy the effective output's lock path is refused intact, never
// taken as the lock file and removed on release.
func TestMutationLockNeverRemovesAnInputMap(t *testing.T) {
	root, base, target := makeRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	prior, err := os.ReadFile(filepath.Join(root, ".corvint", "change.cem.json"))
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(root, "staged.cem.json.lock")
	if err := os.WriteFile(input, prior, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = openSession(t, root).Mark(ctx(), MarkOptions{
		MapPath: "staged.cem.json.lock", Hunk: "1", Disposition: "unknown",
		Reason: "insufficient-evidence", Output: "staged.cem.json",
	})
	if cemcode.CodeOf(err) != cemcode.PublishFailed {
		t.Fatalf("mark locked through an existing input map: %v", err)
	}
	if kept, readErr := os.ReadFile(input); readErr != nil || string(kept) != string(prior) {
		t.Fatalf("mark removed or changed its input map: %v", readErr)
	}
}

// TestCEMOutputsRefuseGitMetadata is CEM-PILOT-013: every public CEM
// mutation entry point rejects an explicit output inside a case-folded .git
// segment before repository metadata can be replaced.
func TestCEMOutputsRefuseGitMetadata(t *testing.T) {
	operations := map[string]struct {
		output string
		run    func(*testing.T, string, string, string, map[string]any) error
	}{
		"begin": {output: ".git/config", run: func(t *testing.T, root, base, _ string, prepared map[string]any) error {
			_, err := openSession(t, root).Begin(ctx(), BeginOptions{
				PatchPath: prepared["patch"].(string), Base: base, Output: ".git/config",
			})
			return err
		}},
		"begin case variant": {output: ".GiT/config", run: func(t *testing.T, root, base, _ string, prepared map[string]any) error {
			_, err := openSession(t, root).Begin(ctx(), BeginOptions{
				PatchPath: prepared["patch"].(string), Base: base, Output: ".GiT/config",
			})
			return err
		}},
		"cite": {output: ".git/config", run: func(t *testing.T, root, _, _ string, _ map[string]any) error {
			_, err := openSession(t, root).Cite(ctx(), CiteOptions{
				MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "docs/rule.txt",
				Lines: "1:1", Relation: "specification", Output: ".git/config",
			})
			return err
		}},
		"mark": {output: ".git/config", run: func(t *testing.T, root, _, _ string, _ map[string]any) error {
			_, err := openSession(t, root).Mark(ctx(), MarkOptions{
				MapPath: wire.ExcludedCEMPath, Hunk: "1", Disposition: "unknown",
				Reason: "insufficient-evidence", Output: ".git/config",
			})
			return err
		}},
		"report": {output: ".git/config", run: func(t *testing.T, root, base, _ string, _ map[string]any) error {
			citeAll(t, root)
			gitCmd(t, root, "add", wire.ExcludedCEMPath)
			gitCmd(t, root, "commit", "-qm", "candidate")
			_, err := openSession(t, root).Read(ctx(), "report", ReadOptions{
				MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: "HEAD", Output: ".git/config",
			})
			return err
		}},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			root, base, target := makeRepo(t)
			prepared, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target})
			if err != nil {
				t.Fatal(err)
			}
			configPath := filepath.Join(root, ".git", "config")
			before, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			err = operation.run(t, root, base, target, prepared)
			if cemcode.CodeOf(err) != cemcode.InvalidArguments {
				t.Fatalf("output %q: got %v", operation.output, err)
			}
			after, err := os.ReadFile(configPath)
			if err != nil || string(after) != string(before) {
				t.Fatalf("output %q changed Git config: %v", operation.output, err)
			}
			if operation.output != ".git/config" {
				variant, variantErr := os.Lstat(filepath.Join(root, filepath.FromSlash(operation.output)))
				config, configErr := os.Lstat(configPath)
				if variantErr == nil && (configErr != nil || !os.SameFile(variant, config)) {
					t.Fatalf("refused case variant %q was created as a distinct path", operation.output)
				}
				if variantErr != nil && !os.IsNotExist(variantErr) {
					t.Fatalf("case variant %q stat: %v", operation.output, variantErr)
				}
			}
		})
	}
}

// TestCEMOutputsRefuseMapSidecars is CEM-PILOT-013: a report or derived map
// cannot occupy its input map's lock or interrupted-publication backup name.
func TestCEMOutputsRefuseMapSidecars(t *testing.T) {
	lock := wire.ExcludedCEMPath + ".lock"
	backup := wire.ExcludedCEMPath + ".bak-0011223344556677"
	operations := map[string]struct {
		output string
		run    func(*testing.T, string, string) error
	}{
		"cite lock": {output: lock, run: func(t *testing.T, root, _ string) error {
			_, err := openSession(t, root).Cite(ctx(), CiteOptions{
				MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "docs/rule.txt",
				Lines: "1:1", Relation: "specification", Output: lock,
			})
			return err
		}},
		"mark backup": {output: backup, run: func(t *testing.T, root, _ string) error {
			_, err := openSession(t, root).Mark(ctx(), MarkOptions{
				MapPath: wire.ExcludedCEMPath, Hunk: "1", Disposition: "unknown",
				Reason: "insufficient-evidence", Output: backup,
			})
			return err
		}},
		"report lock": {output: lock, run: func(t *testing.T, root, base string) error {
			citeAll(t, root)
			gitCmd(t, root, "add", wire.ExcludedCEMPath)
			gitCmd(t, root, "commit", "-qm", "candidate")
			_, err := openSession(t, root).Read(ctx(), "report", ReadOptions{
				MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: "HEAD", Output: lock,
			})
			return err
		}},
		"report backup": {output: backup, run: func(t *testing.T, root, base string) error {
			citeAll(t, root)
			gitCmd(t, root, "add", wire.ExcludedCEMPath)
			gitCmd(t, root, "commit", "-qm", "candidate")
			_, err := openSession(t, root).Read(ctx(), "report", ReadOptions{
				MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: "HEAD", Output: backup,
			})
			return err
		}},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			root, base, target := makeRepo(t)
			if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
				t.Fatal(err)
			}
			if err := operation.run(t, root, base); cemcode.CodeOf(err) != cemcode.InvalidArguments {
				t.Fatalf("output %q: got %v", operation.output, err)
			}
			if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(operation.output))); !os.IsNotExist(err) {
				t.Fatalf("refused sidecar output %q was created", operation.output)
			}
		})
	}
}

// TestPrepareRecoversInterruptedPairBackup is CEM-PILOT-013: a pair publication
// interrupted after setting the prior map aside is restored, never treated as
// a missing map; more than one candidate backup is refused.
func TestPrepareRecoversInterruptedPairBackup(t *testing.T) {
	root, base, target := makeRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	if _, err := openSession(t, root).Mark(ctx(), MarkOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", Disposition: "unknown", Reason: "conflicting-evidence",
	}); err != nil {
		t.Fatal(err)
	}
	mapPath := filepath.Join(root, ".corvint", "change.cem.json")
	prior, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(mapPath, mapPath+".bak-0011223344556677"); err != nil {
		t.Fatal(err)
	}
	prepared, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target})
	if err != nil || prepared["resumed"] != true {
		t.Fatalf("interrupted pair backup was not resumed: %v %v", prepared, err)
	}
	restored, err := os.ReadFile(mapPath)
	if err != nil || string(restored) != string(prior) {
		t.Fatalf("restored map differs from the prior map: %v", err)
	}
	if _, err := os.Lstat(mapPath + ".bak-0011223344556677"); !os.IsNotExist(err) {
		t.Fatalf("recovered backup left behind: %v", err)
	}
	for _, suffix := range []string{".bak-0011223344556677", ".bak-8899aabbccddeeff"} {
		if err := os.WriteFile(mapPath+suffix, prior, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(mapPath); err != nil {
		t.Fatal(err)
	}
	_, err = openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target})
	if cemcode.CodeOf(err) != cemcode.MapUnavailable || !strings.Contains(err.Error(), "backups") {
		t.Fatalf("ambiguous interrupted backups were not refused: %v", err)
	}
	if _, statErr := os.Lstat(mapPath); !os.IsNotExist(statErr) {
		t.Fatal("refusal published a fresh map over ambiguous backups")
	}
}

// TestPrepareRemovesValidatedStalePairBackup is CEM-PILOT-013: a crash after
// the new map landed can leave the old map backup beside a valid final map.
// Once prepare validates that final map, it removes that single stale backup
// so a later interrupted update cannot create an ambiguous pair.
func TestPrepareRemovesValidatedStalePairBackup(t *testing.T) {
	root, base, target := makeRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	mapPath := filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath))
	prior, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	backup := mapPath + ".bak-0011223344556677"
	if err := os.WriteFile(backup, prior, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target})
	if err != nil || result["resumed"] != true {
		t.Fatalf("valid final map did not resume: %v %v", result, err)
	}
	if _, err := os.Lstat(backup); !os.IsNotExist(err) {
		t.Fatalf("validated stale backup remains: %v", err)
	}
}

// TestCiteLinesBound proves --lines applies the frozen LF record bound.
func TestCiteLinesBound(t *testing.T) {
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	huge := strings.Repeat("x\n", 262145)
	writeFile(t, root, "huge.txt", huge)
	writeFile(t, root, "small.txt", "one\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "small.txt", "two\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "target")
	target := gitCmd(t, root, "rev-parse", "HEAD")
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	_, err = session.Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "huge.txt",
		Lines: "1:2", Relation: "specification",
	})
	if cemcode.CodeOf(err) != cemcode.TooManyLines {
		t.Fatalf("got %v, want too-many-lines", err)
	}
}

// TestCiteNoLongerRefusesChangedEvidencePath is CEM-CB-005.
func TestCiteNoLongerRefusesChangedEvidencePath(t *testing.T) {
	root, base, target := makeRepo(t)
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target, Cache: "custom.patch"}); err != nil {
		t.Fatal(err)
	}
	result, err := session.Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt",
		Lines: "1:1", Relation: "implementation",
	})
	if err != nil {
		t.Fatalf("changed-path base span refused: %v", err)
	}
	if evidenceID, ok := result["evidenceId"].(string); !ok || evidenceID == "" {
		t.Fatal("successful changed-path citation omitted evidenceId")
	}
}

func makeCiteSpanRepo(t *testing.T, targetContent string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "src/app.txt", "alpha\nbeta\ngamma\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "src/app.txt", targetContent)
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "target")
	target := gitCmd(t, root, "rev-parse", "HEAD")
	return root, base, target
}

// TestCiteAcceptsRelocatedEvidenceSpan is CEM-CB-005.
func TestCiteAcceptsRelocatedEvidenceSpan(t *testing.T) {
	root, base, target := makeCiteSpanRepo(t, "prefix\nalpha\nbeta\ngamma\n")
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt",
		Lines: "2:2", Relation: "implementation",
	}); err != nil {
		t.Fatalf("relocated span refused: %v", err)
	}
}

// TestCiteRefusesStaleEvidenceSpan is CEM-CB-005.
func TestCiteRefusesStaleEvidenceSpan(t *testing.T) {
	root, base, target := makeCiteSpanRepo(t, "alpha\nBETA\ngamma\n")
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	_, err := session.Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt",
		Lines: "2:2", Relation: "implementation",
	})
	if cemcode.CodeOf(err) != cemcode.CiteSpanNotStable {
		t.Fatalf("got %v, want cite-span-not-stable", err)
	}
	if !strings.Contains(cemcode.MessageOf(err), "stale") {
		t.Fatalf("stale refusal omitted classification: %v", err)
	}
}

// TestCiteRemovedIntentRefusalNamesBasePin is CEM-CB-005 (decision 0165): a
// span the change removes is refused with its base blob pin as a detail token.
func TestCiteRemovedIntentRefusalNamesBasePin(t *testing.T) {
	root, base, target := makeCiteSpanRepo(t, "alpha\ngamma\n")
	baseBlob := gitCmd(t, root, "rev-parse", base+":src/app.txt")
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	_, err := session.Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt",
		Lines: "2:2", Relation: "implementation",
	})
	if cemcode.CodeOf(err) != cemcode.CiteSpanNotStable {
		t.Fatalf("got %v, want cite-span-not-stable", err)
	}
	if want := "removed-intent." + baseBlob + ".6-11"; !strings.Contains(cemcode.MessageOf(err), want) {
		t.Fatalf("removed-intent refusal omitted base pin %q: %v", want, err)
	}
}

// TestCiteRefusesAmbiguousEvidenceSpan is CEM-CB-005.
func TestCiteRefusesAmbiguousEvidenceSpan(t *testing.T) {
	root, base, target := makeCiteSpanRepo(t, "alpha\nbeta\nalpha\ngamma\n")
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	_, err := session.Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt",
		Lines: "1:1", Relation: "implementation",
	})
	if cemcode.CodeOf(err) != cemcode.CiteSpanNotStable {
		t.Fatalf("got %v, want cite-span-not-stable", err)
	}
	if !strings.Contains(cemcode.MessageOf(err), "ambiguous") {
		t.Fatalf("ambiguous refusal omitted classification: %v", err)
	}
}

// TestCiteRefusesDeletedEvidenceSpanWithVerifierClassification is CEM-CB-005.
func TestCiteRefusesDeletedEvidenceSpanWithVerifierClassification(t *testing.T) {
	root, base, _ := makeCiteSpanRepo(t, "prefix\nalpha\nbeta\ngamma\n")
	if err := os.Remove(filepath.Join(root, "src", "app.txt")); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-qm", "delete evidence")
	target := gitCmd(t, root, "rev-parse", "HEAD")
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	_, err := session.Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt",
		Lines: "2:2", Relation: "implementation",
	})
	if cemcode.CodeOf(err) != cemcode.CiteSpanNotStable {
		t.Fatalf("got %v, want cite-span-not-stable", err)
	}
	if !strings.Contains(cemcode.MessageOf(err), "deleted") {
		t.Fatalf("deleted refusal omitted verifier classification: %v", err)
	}
}

// TestCiteRefusesRenamedSourceEvidenceWithVerifierClassification is CEM-CB-005.
func TestCiteRefusesRenamedSourceEvidenceWithVerifierClassification(t *testing.T) {
	root, base, _ := makeCiteSpanRepo(t, "prefix\nalpha\nbeta\ngamma\n")
	gitCmd(t, root, "mv", "src/app.txt", "src/renamed.txt")
	writeFile(t, root, "src/renamed.txt", "prefix\nalpha\nBETA\ngamma\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-qm", "rename evidence")
	target := gitCmd(t, root, "rev-parse", "HEAD")
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	_, err := session.Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt",
		Lines: "2:2", Relation: "implementation",
	})
	if cemcode.CodeOf(err) != cemcode.CiteSpanNotStable {
		t.Fatalf("got %v, want cite-span-not-stable", err)
	}
	if !strings.Contains(cemcode.MessageOf(err), "deleted") {
		t.Fatalf("renamed-source refusal omitted verifier classification: %v", err)
	}
}

func TestCiteAcceptsUnchangedEvidencePath(t *testing.T) {
	root, base, target := makeRepo(t)
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	result, err := session.Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "docs/rule.txt",
		Lines: "1:1", Relation: "specification",
	})
	if err != nil {
		t.Fatal(err)
	}
	if evidenceID, ok := result["evidenceId"].(string); !ok || evidenceID == "" {
		t.Fatal("successful citation omitted evidenceId")
	}
}

// TestDeterministicEnvelopes is CEM-CB-DE-002: fresh sessions emit
// byte-identical JSON.
func TestDeterministicEnvelopes(t *testing.T) {
	root, base, target := makeRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	citeAll(t, root)
	gitCmd(t, root, "add", wire.ExcludedCEMPath)
	gitCmd(t, root, "commit", "-qm", "candidate")
	render := func() string {
		result, err := openSession(t, root).Read(ctx(), "status", ReadOptions{
			MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: "HEAD",
		})
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		return string(encoded)
	}
	first, second := render(), render()
	if first != second {
		t.Fatalf("fresh envelopes disagree:\n%s\n%s", first, second)
	}
}

// TestReportDefaultsUnderGitDir keeps report artifacts per-worktree.
func TestReportDefaultsUnderGitDir(t *testing.T) {
	root, base, target := makeRepo(t)
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	citeAll(t, root)
	gitCmd(t, root, "add", wire.ExcludedCEMPath)
	gitCmd(t, root, "commit", "-qm", "candidate")
	result, err := openSession(t, root).Read(ctx(), "report", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := result["report"].(string)
	if !strings.HasPrefix(reportPath, filepath.Join(root, ".git")) {
		t.Fatalf("report escaped the per-worktree Git directory: %s", reportPath)
	}
	content, readErr := os.ReadFile(reportPath)
	if readErr != nil || !strings.Contains(string(content), "Change Evidence Map review") {
		t.Fatalf("report content: %v", readErr)
	}
	if _, present := result["patch"]; present {
		t.Fatal("report carries a legacy patch field")
	}
}

// TestReportAbsoluteOutput (V1-0141): an absolute --output outside the repository is written there;
// one inside the worktree or the Git directory is refused with invalid-arguments before any write.
func TestReportAbsoluteOutput(t *testing.T) {
	root, base, target := makeRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "review.md")
	result, err := openSession(t, root).Read(ctx(), "report", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target, Output: outside,
	})
	if err != nil {
		t.Fatal(err)
	}
	content, readErr := os.ReadFile(result["report"].(string))
	if readErr != nil || !strings.Contains(string(content), "Change Evidence Map review") {
		t.Fatalf("report %v at %v: %v", result["report"], outside, readErr)
	}
	for _, inside := range []string{filepath.Join(root, "review.md"), filepath.Join(root, ".git", "review.md")} {
		_, err := openSession(t, root).Read(ctx(), "report", ReadOptions{
			MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target, Output: inside,
		})
		if cemcode.CodeOf(err) != cemcode.InvalidArguments {
			t.Fatalf("%s: got %v, want invalid-arguments", inside, err)
		}
		if _, statErr := os.Lstat(inside); !os.IsNotExist(statErr) {
			t.Fatalf("%s: refused report was written", inside)
		}
	}
}

// TestStagePrecedenceBeforeRepositoryValidation is CEM-CB-AT-008 for the
// native build: with the repository boundary broken (a configured alternate),
// stage-2 map validation, stage-3 profile-forbidden arguments, and stage-4
// profile-required inputs all report their own codes; only a clean stage 1–4
// pass reaches the stage-6 alternate denial.
func TestStagePrecedenceBeforeRepositoryValidation(t *testing.T) {
	root, base, target := makeRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	alternates := filepath.Join(root, ".git", "objects", "info", "alternates")
	if err := os.WriteFile(alternates, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	mapPath := filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath))
	pristine, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	// Stage 2 wins: a closed-schema violation reports before the alternate.
	corrupted := strings.Replace(string(pristine), `"spec": "cem/0.2"`, `"spec": "cem/0.2", "surplus": true`, 1)
	if err := os.WriteFile(mapPath, []byte(corrupted), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = openSession(t, root).Read(ctx(), "status", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: "HEAD",
	})
	if cemcode.CodeOf(err) != cemcode.UnknownField {
		t.Fatalf("stage 2 vs alternate: got %v", err)
	}
	if err := os.WriteFile(mapPath, pristine, 0o644); err != nil {
		t.Fatal(err)
	}
	// Stage 3 wins: --patch on 0.2 reports before the alternate.
	_, err = openSession(t, root).Read(ctx(), "status", ReadOptions{
		MapPath: wire.ExcludedCEMPath, PatchGiven: true, PatchPath: "x.patch",
		ExpectedBase: base, Target: "HEAD",
	})
	if cemcode.CodeOf(err) != cemcode.InvalidArguments {
		t.Fatalf("stage 3 vs alternate: got %v", err)
	}
	// Stage 4 wins: a missing required input reports before the alternate.
	_, err = openSession(t, root).Read(ctx(), "status", ReadOptions{
		MapPath: wire.ExcludedCEMPath, Target: "HEAD",
	})
	if cemcode.CodeOf(err) != cemcode.ExpectedBaseRequired {
		t.Fatalf("stage 4 vs alternate: got %v", err)
	}
	// Clean stages 1–4 finally reach the stage-6 denial.
	_, err = openSession(t, root).Read(ctx(), "status", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: "HEAD",
	})
	if cemcode.CodeOf(err) != cemcode.UnsupportedObjectAlternates {
		t.Fatalf("stage 6 denial: got %v", err)
	}
}

// TestPrepareCacheAliasingRefused is CEM-PILOT-013: a convenience-cache
// selection may never alias or shadow the frozen map path, fresh or resumed,
// nor take the map's lock or interrupted-publication backup name, which a
// later prepare would restore as the map.
func TestPrepareCacheAliasingRefused(t *testing.T) {
	root, base, target := makeRepo(t)
	siblings := []string{wire.ExcludedCEMPath + ".lock", wire.ExcludedCEMPath + ".bak-0011223344556677"}
	for _, cache := range append([]string{wire.ExcludedCEMPath, "./" + wire.ExcludedCEMPath, ".corvint"}, siblings...) {
		_, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target, Cache: cache})
		if cemcode.CodeOf(err) != cemcode.InvalidArguments {
			t.Fatalf("cache %q: got %v", cache, err)
		}
	}
	for _, sibling := range siblings {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(sibling))); !os.IsNotExist(err) {
			t.Fatalf("refused cache %q was written", sibling)
		}
	}
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	_, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target, Cache: wire.ExcludedCEMPath})
	if cemcode.CodeOf(err) != cemcode.InvalidArguments {
		t.Fatalf("resumed aliasing: got %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath)))
	if readErr != nil || !strings.Contains(string(data), `"spec": "cem/0.2"`) {
		t.Fatalf("map clobbered by refused aliasing attempt: %v", readErr)
	}
}

// TestFailureAssuranceTracksCanonicalBinding is the CEM-CB-014 boundary: a
// failure before target binding and canonical derivation reports
// structural-only assurance, while a failure after both completed keeps
// canonical.
func TestFailureAssuranceTracksCanonicalBinding(t *testing.T) {
	root, base, target := makeRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	citeAll(t, root)
	gitCmd(t, root, "add", wire.ExcludedCEMPath)
	gitCmd(t, root, "commit", "-qm", "candidate")
	// Pre-binding failure: the independent expected base disagrees with the
	// map (stage 7, before target binding).
	result, err := openSession(t, root).Read(ctx(), "status", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: target, Target: "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}
	verification := result["verification"].(map[string]any)
	issue := verification["issues"].([]any)[0].(map[string]any)
	if issue["code"] != cemcode.BaseRevisionMismatch || verification["assurance"] != "structural-only" {
		t.Fatalf("pre-binding failure: %+v", verification)
	}
	// Post-binding failure: the committed sidecar binds byte-exactly but its
	// recorded digest is wrong (stage 8, after derivation).
	mapPath := filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath))
	data, readErr := os.ReadFile(mapPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	document, parseErr := wire.ParseMap(data)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	corrupted := strings.Replace(string(data), document.PatchSha256, strings.Repeat("0", 64), 1)
	if err := os.WriteFile(mapPath, []byte(corrupted), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "add", wire.ExcludedCEMPath)
	gitCmd(t, root, "commit", "-qm", "digest corrupted")
	result, err = openSession(t, root).Read(ctx(), "status", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}
	verification = result["verification"].(map[string]any)
	issue = verification["issues"].([]any)[0].(map[string]any)
	if issue["code"] != cemcode.PatchDigestMismatch || verification["assurance"] != "canonical" {
		t.Fatalf("post-binding failure: %+v", verification)
	}
}

// TestPrepareCacheAliasingCaseVariants: on a case-insensitive worktree
// volume, a case-variant spelling of the map path is the same physical file
// and must be refused exactly like the exact spelling.
func TestPrepareCacheAliasingCaseVariants(t *testing.T) {
	root, base, target := makeRepo(t)
	if err := os.WriteFile(filepath.Join(root, "probe-case"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, "PROBE-CASE")); err != nil {
		t.Skip("volume is case-sensitive")
	}
	for _, cache := range []string{".CORVINT/change.cem.json", ".corvint/CHANGE.CEM.JSON", ".Corvint"} {
		_, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target, Cache: cache})
		if cemcode.CodeOf(err) != cemcode.InvalidArguments {
			t.Fatalf("cache %q: got %v", cache, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath))); !os.IsNotExist(err) {
		t.Fatal("refused aliasing attempt still wrote the map path")
	}
}

// TestEncodeMapMatchesPythonOracleWithLineSeparator pins mapSha256 parity for
// the line- and paragraph-separator runes (code points 0x2028 and 0x2029):
// encoding/json escapes both unconditionally even with SetEscapeHTML(false),
// where the Python oracle's json.dumps(indent=2, sort_keys=True,
// ensure_ascii=False) leaves them raw, forking the digest the two runtimes
// must agree on. A second fixture covers the adjacent edge the naive fix (a
// plain byte replace of the escape sequence) gets wrong: a path that already
// contains a literal backslash immediately before the literal text "u2028" is
// itself escaped to a doubled backslash by the encoder, and a substring
// replace mistakes that pair's second backslash for a genuine rune escape.
// Both expected digests were computed once by running the oracle's exact
// json.dumps call over the equivalent document with python3's stdlib json and
// hashlib.
func TestEncodeMapMatchesPythonOracleWithLineSeparator(t *testing.T) {
	lineSeparator := string(rune(0x2028))
	backslash := string(rune(92))
	cases := []struct {
		name string
		path string
		want string
	}{
		{"rune", "src/example" + lineSeparator + "note.go", "63cdcae5012f761746e549bf218f4b9c375883cc6b4faeeca8841343a12ed6ac"},
		{"literal backslash then u2028 text", "src/" + backslash + "u2028literal.go", "1ebad4d692d76416396404ec0852cfee3124eba40f73576aafdbf19f41ab653b"},
	}
	for _, item := range cases {
		document := &wire.Map{
			Spec:         wire.Spec01,
			BaseRevision: strings.Repeat("a", 40),
			PatchSha256:  strings.Repeat("b", 64),
			Evidence: []wire.Evidence{{
				ID:         "evidence:sha256:" + strings.Repeat("d", 64),
				BlobOid:    strings.Repeat("c", 40),
				Path:       item.path,
				Span:       wire.Span{Start: 0, End: 10},
				SpanSha256: strings.Repeat("e", 64),
			}},
		}
		encoded := encodeMap(document)
		digest := sha256.Sum256(encoded)
		got := hex.EncodeToString(digest[:])
		if got != item.want {
			t.Errorf("%s: mapSha256 = %s, want %s (Python oracle digest for the identical document)\nencoded = %q", item.name, got, item.want, encoded)
		}
	}
}

// TestRenderReportTextFencesBacktickPath is a regression test for F7: a
// repository path containing a backtick must not break out of the code span
// wrapping it in the rendered CEM review report. wire.ValidatePath rejects
// bytes below 0x20 but allows backticks, so a changed file's path can carry
// one.
func TestRenderReportTextFencesBacktickPath(t *testing.T) {
	document := &wire.Map{Spec: wire.Spec01, BaseRevision: strings.Repeat("a", 40), PatchSha256: strings.Repeat("b", 64)}
	verification := map[string]any{
		"valid": true,
		"drift": []any{map[string]any{
			"evidenceId": "evidence:sha256:" + strings.Repeat("d", 64),
			"path":       "a`b.go",
			"status":     "stale",
		}},
	}
	counts := map[string]any{"total": 1, "supported": 0, "unknown": 1, "mechanical": 0}
	work := []any{map[string]any{
		"selector": 1, "path": "a`b.go", "id": "hunk:sha256:" + strings.Repeat("c", 64),
		"disposition": "unknown", "reason": "no-basis", "next": "cite evidence",
	}}

	report := renderReportText(document, verification, counts, work, nil)

	if !strings.Contains(report, "``a`b.go``") {
		t.Fatalf("expected the backtick path to be fenced with a longer run of backticks in both the drift and worklist sections:\n%s", report)
	}
	if strings.Count(report, "``a`b.go``") != 2 {
		t.Fatalf("expected the fenced path in both the drift and worklist sections, got:\n%s", report)
	}
}

// TestCiteRefusesNegativeByteSpan: a library caller's negative --bytes start
// fails invalid-span instead of slicing the evidence blob out of range.
func TestCiteRefusesNegativeByteSpan(t *testing.T) {
	root, base, target := makeRepo(t)
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	_, err := session.Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "docs/rule.txt",
		Bytes: "-1:3", Relation: "specification",
	})
	if cemcode.CodeOf(err) != cemcode.InvalidSpan {
		t.Fatalf("got %v, want invalid-span", err)
	}
}

// TestMarkRejectsNonCanonicalOrdinal is CEM-PILOT-003: a decimal selector that
// is not canonical and one-based fails rather than resolving to a hunk.
func TestMarkRejectsNonCanonicalOrdinal(t *testing.T) {
	root, base, target := makeRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	for _, selector := range []string{"01", "+1"} {
		_, err := openSession(t, root).Mark(ctx(), MarkOptions{
			MapPath: wire.ExcludedCEMPath, Hunk: selector, Disposition: "unknown", Reason: "insufficient-evidence",
		})
		if err == nil {
			t.Fatalf("selector %q resolved to a hunk", selector)
		}
	}
}

// TestPrepareRefusesOversizeMapUnderMissingNamedRoot is CEM-PILOT-002: an
// unreadable existing map is refused without --replace even when the
// repository path happens to contain the word "missing".
func TestPrepareRefusesOversizeMapUnderMissingNamedRoot(t *testing.T) {
	original, base, target := makeRepo(t)
	root := filepath.Join(filepath.Dir(original), "missing")
	if err := os.Rename(original, root); err != nil {
		t.Fatal(err)
	}
	oversize := strings.Repeat(" ", wire.MaxMapBytes+1)
	writeFile(t, root, wire.ExcludedCEMPath, oversize)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err == nil {
		t.Fatal("oversize existing map silently overwritten")
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath)))
	if err != nil || string(data) != oversize {
		t.Fatal("refusal mutated the existing map")
	}
}

// TestReportRefusesCaseVariantOfMapPath: on a case-insensitive volume a report
// output that differs from the map path only in case would overwrite the map.
func TestReportRefusesCaseVariantOfMapPath(t *testing.T) {
	root, base, target := makeRepo(t)
	if err := os.WriteFile(filepath.Join(root, "probe-case"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, "PROBE-CASE")); err != nil {
		t.Skip("volume is case-sensitive")
	}
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	mapPath := filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath))
	before, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = openSession(t, root).Read(ctx(), "report", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target, Output: ".CORVINT/change.cem.json",
	})
	if cemcode.CodeOf(err) != cemcode.InvalidArguments {
		t.Fatalf("got %v, want invalid-arguments", err)
	}
	after, err := os.ReadFile(mapPath)
	if err != nil || string(after) != string(before) {
		t.Fatal("report overwrote the map through a case variant")
	}
}

// TestReportRefusesNormalizationVariantOfMapPath: on a normalization-insensitive
// volume an NFD spelling of an NFC map path names the same file, which case
// folding does not see; the report must refuse rather than overwrite the map.
func TestReportRefusesNormalizationVariantOfMapPath(t *testing.T) {
	root, base, target := makeRepo(t)
	composed, decomposed := "caf\u00e9.cem.json", "cafe\u0301.cem.json"
	if err := os.WriteFile(filepath.Join(root, composed), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, decomposed)); err != nil {
		t.Skip("volume is normalization-sensitive")
	}
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath)))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, composed), before, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = openSession(t, root).Read(ctx(), "report", ReadOptions{
		MapPath: composed, ExpectedBase: base, Target: target, Output: decomposed,
	})
	if cemcode.CodeOf(err) != cemcode.InvalidArguments {
		t.Fatalf("got %v, want invalid-arguments", err)
	}
	after, err := os.ReadFile(filepath.Join(root, composed))
	if err != nil || string(after) != string(before) {
		t.Fatal("report overwrote the map through a normalization variant")
	}
}

// makeGoRepo commits base and target revisions of one Go file.
func makeGoRepo(t *testing.T, base, target string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "pkg/a.go", base)
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	baseSHA := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "pkg/a.go", target)
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "target")
	return root, baseSHA, gitCmd(t, root, "rev-parse", "HEAD")
}

const importReorderBase = "package pkg\n\nimport (\n\t\"strings\"\n\t\"fmt\"\n)\n\n// Shout upper-cases s.\nfunc Shout(s string) string { return fmt.Sprint(strings.ToUpper(s)) }\n"
const importReorderTarget = "package pkg\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\n// Shout upper-cases s.\nfunc Shout(s string) string { return fmt.Sprint(strings.ToUpper(s)) }\n"

// markStructural prepares, marks hunk 1 mechanical with reason, commits the
// map, and returns the status envelope (CEM-SM-006).
func markStructural(t *testing.T, root, base, target, reason string) map[string]any {
	t.Helper()
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Mark(ctx(), MarkOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", Disposition: "mechanical", Reason: reason,
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, wire.ExcludedCEMPath))
	if err != nil {
		t.Fatal(err)
	}
	document, err := wire.ParseMap(data)
	if err != nil {
		t.Fatal(err)
	}
	if document.Spec != wire.Spec03 {
		t.Fatalf("spec after structural mark = %q, want %q", document.Spec, wire.Spec03)
	}
	gitCmd(t, root, "add", wire.ExcludedCEMPath)
	gitCmd(t, root, "commit", "-qm", "candidate")
	result, err := openSession(t, root).Read(ctx(), "status", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// TestMarkStructuralReasonUpgradesToSpec03AndVerifies is the end-to-end
// cem/0.3 path: mark upgrades the map, and the LLM-free verifier proves the
// import reorder from the base blob and patch alone.
func TestMarkStructuralReasonUpgradesToSpec03AndVerifies(t *testing.T) {
	root, base, target := makeGoRepo(t, importReorderBase, importReorderTarget)
	result := markStructural(t, root, base, target, "import-reorder")
	verification := result["verification"].(map[string]any)
	if result["ok"] != true || verification["valid"] != true {
		encoded, _ := json.Marshal(result)
		t.Fatalf("structural status not green: %s", encoded)
	}
	if counts := result["counts"]; fmt.Sprint(counts) != "map[mechanical:1 supported:0 total:1 unknown:0]" {
		t.Fatalf("counts = %v", counts)
	}
}

// TestMarkStructuralReasonWrongClassIsRefused pins that a structural claim
// the verifier cannot reproduce fails unproven-mechanical rather than being
// accepted on the author's word.
func TestMarkStructuralReasonWrongClassIsRefused(t *testing.T) {
	root, base, target := makeGoRepo(t, importReorderBase, importReorderTarget)
	result := markStructural(t, root, base, target, "rename")
	verification := result["verification"].(map[string]any)
	issues := verification["issues"].([]any)
	if result["ok"] != false || verification["valid"] != false || len(issues) == 0 {
		t.Fatalf("wrong-class claim accepted: %+v", verification)
	}
	if code := issues[0].(map[string]any)["code"]; code != cemcode.UnprovenMechanical {
		t.Fatalf("issue code %v, want %s", code, cemcode.UnprovenMechanical)
	}
}

// TestMarkStructuralReasonRefusedOnSpec01 pins that a 0.1 map never gains
// the structural vocabulary.
func TestMarkStructuralReasonRefusedOnSpec01(t *testing.T) {
	root, _, _ := makeRepo(t)
	writeFile(t, root, "staged.cem.json", `{"spec":"cem/0.1","baseRevision":"4ca153370afd9bd8c6034ad73acc3925150ab681",`+
		`"patchSha256":"dec61287f7b726144fc19d67f0e07f3c40410c28bc19831a4b0f9fb96487717c",`+
		`"evidence":[],"hunks":[{"id":"hunk:sha256:07461a992e03e064986720e365dc4bb477da7cefe73da853f51bcb70e2c3100c",`+
		`"path":"src/app.py","oldRange":{"start":1,"count":2},"newRange":{"start":1,"count":2},`+
		`"disposition":"unknown","reason":"no-evidence","basis":[]}]}`)
	_, err := openSession(t, root).Mark(ctx(), MarkOptions{
		MapPath: "staged.cem.json", Hunk: "1", Disposition: "mechanical", Reason: "rename", Output: "out.cem.json",
	})
	if cemcode.CodeOf(err) != cemcode.InvalidArguments {
		t.Fatalf("got %v, want invalid-arguments", err)
	}
}
