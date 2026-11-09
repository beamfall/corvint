package workflow

import (
	"bytes"
	"context"
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
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/testplatform"
)

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitRun(dir, t.TempDir(), args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// gitRun runs git in dir with a hermetic identity, clock and empty home.
func gitRun(dir, home string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1", "HOME="+home, "XDG_CONFIG_HOME="+home,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
		"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000")
	out, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out)), nil
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

// makeRepo returns a private copy of the base/target fixture repository: the
// target modifies one file and creates another. The repository is built once
// per package (makeRepoTemplate) and copied, so each caller skips the seven git
// subprocesses; the fixed identity and clock keep base and target identical.
func makeRepo(t *testing.T) (string, string, string) {
	t.Helper()
	repoTemplate.once.Do(func() {
		repoTemplate.root, repoTemplate.base, repoTemplate.target, repoTemplate.err = makeRepoTemplate()
	})
	if repoTemplate.err != nil {
		t.Fatal(repoTemplate.err)
	}
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := copyTree(repoTemplate.root, root); err != nil {
		t.Fatal(err)
	}
	return root, repoTemplate.base, repoTemplate.target
}

var repoTemplate struct {
	once               sync.Once
	root, base, target string
	err                error
	parent             string
}

// makeRepoTemplate builds the makeRepo fixture under a package-lifetime
// directory that TestMain removes.
func makeRepoTemplate() (string, string, string, error) {
	parent, err := os.MkdirTemp(fixtureTempBase, "workflow-fixture-")
	if err != nil {
		return "", "", "", err
	}
	repoTemplate.parent = parent
	parent, err = filepath.EvalSymlinks(parent)
	if err != nil {
		return "", "", "", err
	}
	root, home := filepath.Join(parent, "repo"), filepath.Join(parent, "home")
	for _, dir := range []string{root, home} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			return "", "", "", err
		}
	}
	write := func(path, content string) error {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		return os.WriteFile(full, []byte(content), 0o644)
	}
	steps := []func() error{
		func() error { _, err := gitRun(root, home, "init", "-q", "-b", "main"); return err },
		func() error { return write("docs/rule.txt", "the frozen rule\nsecond line\nthird line\n") },
		func() error { return write("src/app.txt", "alpha\nbeta\ngamma\n") },
		func() error { _, err := gitRun(root, home, "add", "."); return err },
		func() error { _, err := gitRun(root, home, "commit", "-qm", "base"); return err },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return "", "", "", err
		}
	}
	base, err := gitRun(root, home, "rev-parse", "HEAD")
	if err != nil {
		return "", "", "", err
	}
	steps = []func() error{
		func() error { return write("src/app.txt", "alpha\nBETA\ngamma\n") },
		func() error { return write("src/new.txt", "created\n") },
		func() error { _, err := gitRun(root, home, "add", "."); return err },
		func() error { _, err := gitRun(root, home, "commit", "-qm", "target"); return err },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return "", "", "", err
		}
	}
	target, err := gitRun(root, home, "rev-parse", "HEAD")
	if err != nil {
		return "", "", "", err
	}
	return root, base, target, nil
}

// copyTree copies the regular files and directories under from into the
// existing directory to, preserving permission bits.
func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(from, path)
		if err != nil || relative == "." {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		destination := filepath.Join(to, relative)
		switch {
		case entry.IsDir():
			return os.Mkdir(destination, info.Mode().Perm())
		case info.Mode().IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(destination, data, info.Mode().Perm())
		default:
			return fmt.Errorf("fixture template holds non-regular file %s", path)
		}
	})
}

// readProducerMap reads the raw bytes and mechanics view of a cem/1.0 producer
// map exactly as a mutator does.
func (s *Session) readProducerMap(relative string) ([]byte, *wire.Map, error) {
	raw, err := s.readMapBytes(relative)
	if err != nil {
		return nil, nil, err
	}
	document, err := s.readMutableMap("test", relative)
	return raw, document, err
}

// openSession opens a session for a test that prepares, cites or marks a map.
// Each of those takes the CEM update lock, so on Windows the test skips on
// the lock's exact platform refusal (CEM-PILOT-033, V1-2522).
func openSession(t *testing.T, root string) *Session {
	t.Helper()
	testplatform.SkipWindowsRefusal(t, publish.LockRefusal())
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
		_, document, err := session.readProducerMap(wire.ExcludedCEMPath)
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
// the candidate, then verify the committed revision through the stable status
// with the exact next action argv (CEM-V1-215).
func TestTwoPhaseCanonicalFlow(t *testing.T) {
	t.Parallel()
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
	want, _ := json.Marshal(stableNextAction(base, target, root))
	if strings.Contains(string(rendered), "--patch") || string(rendered) != string(want) {
		t.Fatalf("canonical next action malformed: %s", rendered)
	}
	// CEM-CB-016 / decision 0122: argv[0] is the contract command name, not the build name.
	if action[0] != "corvint" {
		t.Fatalf("next action command name = %v, want corvint", action[0])
	}
	citeAll(t, root)
	// The worktree still holds the target checkout; commit the candidate to
	// create the final revision the consumer independently resolves.
	gitCmd(t, root, "add", wire.ExcludedCEMPath)
	gitCmd(t, root, "commit", "-qm", "candidate")
	head := strings.TrimSpace(gitCmd(t, root, "rev-parse", "HEAD"))
	// CEM-CB-AT-001: the convenience cache is not patch authority.
	if err := os.Remove(filepath.Join(session.gitRoot.Path(), "corvint", "change.patch")); err != nil {
		t.Fatal(err)
	}
	result := stableStatus(t, root, wire.ExcludedCEMPath, base, head, PolicyLimits{})
	if result["ok"] != true || stableOutcome(result) != "ACCEPT" {
		encoded, _ := json.Marshal(result)
		t.Fatalf("stable status not green: %s", encoded)
	}
	if _, present := result["patch"]; present {
		t.Fatal("stable status carries a legacy patch field")
	}
	verified, err := openSession(t, root).Read(ctx(), "verify", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: head,
		ArtifactRoot: root, ArtifactsGiven: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, present := verified["patch"]; present {
		t.Fatal("stable verify carries a legacy patch field")
	}
	if verified["ok"] != true {
		encoded, _ := json.Marshal(verified)
		t.Fatalf("verify not green: %s", encoded)
	}
}

// TestHiddenTargetChangeRejected is CEM-CB-AT-002: a target containing an
// extra hunk is rejected by the stable status.
func TestHiddenTargetChangeRejected(t *testing.T) {
	t.Parallel()
	root, base, target := makeRepo(t)
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	citeAll(t, root)
	writeFile(t, root, "src/hidden.txt", "smuggled\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "candidate plus hidden change")
	head := strings.TrimSpace(gitCmd(t, root, "rev-parse", "HEAD"))
	result := stableStatus(t, root, wire.ExcludedCEMPath, base, head, PolicyLimits{})
	verification, _ := result["stableVerification"].(StableVerification)
	if result["ok"] != false || verification.Result.Outcome != "REJECT" || len(verification.Result.IssueCodes) == 0 {
		encoded, _ := json.Marshal(result)
		t.Fatalf("status did not surface the failure: %s", encoded)
	}
	if code := verification.Result.IssueCodes[0]; code != cemcode.PatchDigestMismatch && code != cemcode.ExcludedArtifactMismatch {
		t.Fatalf("issue code %v", code)
	}
}

// TestTargetSidecarRawBinding is CEM-CB-EX-005 on the one wire (CEM-V1-215):
// the stable verifier accepts a byte-identical committed sidecar, and a
// semantically equivalent reserialization of the read map fails
// excluded-artifact-mismatch.
func TestTargetSidecarRawBinding(t *testing.T) {
	t.Parallel()
	root, base, target := makeRepo(t)
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	citeAll(t, root)
	gitCmd(t, root, "add", wire.ExcludedCEMPath)
	gitCmd(t, root, "commit", "-qm", "candidate")
	head := strings.TrimSpace(gitCmd(t, root, "rev-parse", "HEAD"))
	mapPath := filepath.Join(root, ".corvint", "change.cem.json")
	pristine, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	if result := stableStatus(t, root, wire.ExcludedCEMPath, base, head, PolicyLimits{}); stableOutcome(result) != "ACCEPT" {
		encoded, _ := json.Marshal(result)
		t.Fatalf("byte-identical input rejected: %s", encoded)
	}
	// One inserted space keeps the JSON valid but changes the raw bytes bound
	// to the committed sidecar.
	mutated := append([]byte("{ "), pristine[1:]...)
	if err := os.WriteFile(mapPath, mutated, 0o644); err != nil {
		t.Fatal(err)
	}
	result := stableStatus(t, root, wire.ExcludedCEMPath, base, head, PolicyLimits{})
	verification, _ := result["stableVerification"].(StableVerification)
	if result["ok"] != false || verification.Result.Outcome != "REJECT" || len(verification.Result.IssueCodes) != 1 || verification.Result.IssueCodes[0] != cemcode.ExcludedArtifactMismatch {
		encoded, _ := json.Marshal(result)
		t.Fatalf("mutated input: %s", encoded)
	}
}

// TestResumeRules is CEM-CB-CO-004 on the one wire (CEM-V1-215): a cem/1.0
// map written by begin at the default path resumes, its next action is the
// stable status, which never carries a --patch selection whether the cache was
// omitted or named, and non-default prepare maps are refused.
func TestResumeRules(t *testing.T) {
	t.Parallel()
	root, base, target := makeRepo(t)
	session := openSession(t, root)
	prepared, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	cachePath := prepared["patch"].(string)
	// Replace the prepared candidate with a begin map over the same bytes.
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
		t.Fatal("begin map at default path did not resume")
	}
	firstAction, _ := json.Marshal(first["nextActions"])
	if strings.Contains(string(firstAction), "--patch") || !strings.Contains(string(firstAction), `"cem","status"`) || !strings.Contains(string(firstAction), `"--artifacts"`) {
		t.Fatalf("resumed omitted-patch action: %s", firstAction)
	}
	// Resume with explicit --patch: the stable action still carries none.
	second, err := openSession(t, root).Prepare(ctx(), PrepareOptions{
		Base: base, Target: target, Cache: "alt/cache.patch",
	})
	if err != nil {
		t.Fatal(err)
	}
	secondAction, _ := json.Marshal(second["nextActions"])
	if second["resumed"] != true || string(secondAction) != string(firstAction) {
		t.Fatalf("resumed explicit-patch action: %s", secondAction)
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
	t.Parallel()
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
	t.Parallel()
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

// TestMutationPreservesProfile is CEM-CB-CO-003.
func TestMutationPreservesProfile(t *testing.T) {
	t.Parallel()
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
	document, err := wire.ParseStable(data)
	if err != nil {
		t.Fatalf("mutated cem/1.0 map no longer parses: %v", err)
	}
	if document.Change.Spec != wire.StableSpec || document.Change.ExcludedPath != wire.ExcludedCEMPath {
		t.Fatal("mark dropped the cem/1.0 profile or excludedPath")
	}
}

// TestConcurrentMarksAllSurvive is CEM-PILOT-013: concurrent in-place marks
// serialize their read-modify-write, so every successful mark survives.
func TestConcurrentMarksAllSurvive(t *testing.T) {
	t.Parallel()
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
	_, document, err := openSession(t, root).readProducerMap(wire.ExcludedCEMPath)
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	return makeCiteSpanRepoFrom(t, "alpha\nbeta\ngamma\n", targetContent)
}

func makeCiteSpanRepoFrom(t *testing.T, baseContent, targetContent string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "src/app.txt", baseContent)
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// TestCiteAmbiguousRefusalNamesTextAlignmentV11418 is V1-1418 and
// DCW-V0-063: base lines next to repeated if/else/fi lines that git -U0
// leaves unchanged are still refused, because stability matches text and the
// change adds another copy; the refusal names that alignment and the cause.
func TestCiteAmbiguousRefusalNamesTextAlignmentV11418(t *testing.T) {
	t.Parallel()
	block := func(name string) string {
		return "if " + name + "; then\n  run " + name + "\nelse\n  skip\nfi\n"
	}
	baseContent := "# head\n" + block("a") + block("b") + "# tail\n"
	// Edit line 1 and insert a third block after line 12; base lines 9:11
	// (else, skip, fi of block b) stay unedited: git -U0 leaves them out, and
	// the canonical -U3 hunk carries 10:11 only as context.
	targetContent := "# HEAD\n" + block("a") + block("b") + "# tail\n" + block("c")
	root, base, target := makeCiteSpanRepoFrom(t, baseContent, targetContent)
	diff := gitCmd(t, root, "diff", "-U0", base, target, "--", "src/app.txt")
	for _, header := range []string{"@@ -1 +1 @@", "@@ -12,0 +13,5 @@"} {
		if !strings.Contains(diff, header) {
			t.Fatalf("git -U0 alignment changed, want %q in:\n%s", header, diff)
		}
	}
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	_, err := session.Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt",
		Lines: "9:11", Relation: "implementation",
	})
	if cemcode.CodeOf(err) != cemcode.CiteSpanNotStable {
		t.Fatalf("got %v, want cite-span-not-stable", err)
	}
	want := "would be ambiguous after applying its CEM hunk group; base lines 9:11 are edited by no hunk (at most hunk context), so git's line alignment leaves them unchanged, but the stability check matches the span text, which occurs 3 times in the changed file"
	if !strings.Contains(cemcode.MessageOf(err), want) {
		t.Fatalf("refusal %q, want %q", cemcode.MessageOf(err), want)
	}
}

// TestSpanUneditedByHunksFollowsGitAlignmentV11418 is V1-1418: context lines
// leave a base line unchanged; a removed line or an insertion inside the span
// does not.
func TestSpanUneditedByHunksFollowsGitAlignmentV11418(t *testing.T) {
	t.Parallel()
	// Old lines: 1 context, 2 removed, insertion before 3, 3 context,
	// insertion before 4, 4 context.
	group := &patch.Group{Hunks: []*patch.Hunk{{
		OldRange: wire.Range{Start: 1, Count: 4},
		Body: []patch.BodyLine{
			{Prefix: ' '}, {Prefix: '-'}, {Prefix: '+'}, {Prefix: ' '}, {Prefix: '+'}, {Prefix: ' '},
		},
	}}}
	for _, test := range []struct {
		start, end int64
		want       bool
	}{{1, 1, true}, {2, 2, false}, {3, 3, true}, {3, 4, false}, {4, 4, true}, {1, 3, false}} {
		if got := spanUneditedByHunks(group, test.start, test.end); got != test.want {
			t.Fatalf("lines %d:%d outside=%v, want %v", test.start, test.end, got, test.want)
		}
	}
}

// TestCiteRefusesDeletedEvidenceSpanWithVerifierClassification is CEM-CB-005.
func TestCiteRefusesDeletedEvidenceSpanWithVerifierClassification(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// TestPrepareCacheAliasingRefused is CEM-PILOT-013: a convenience-cache
// selection may never alias or shadow the frozen map path, fresh or resumed,
// nor take the map's lock or interrupted-publication backup name, which a
// later prepare would restore as the map.
func TestPrepareCacheAliasingRefused(t *testing.T) {
	t.Parallel()
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
	if readErr != nil || !strings.Contains(string(data), `"spec":"cem/1.0"`) {
		t.Fatalf("map clobbered by refused aliasing attempt: %v", readErr)
	}
}

// TestPrepareCacheAliasingCaseVariants: on a case-insensitive worktree
// volume, a case-variant spelling of the map path is the same physical file
// and must be refused exactly like the exact spelling.
func TestPrepareCacheAliasingCaseVariants(t *testing.T) {
	t.Parallel()
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

// TestCiteRefusesNegativeByteSpan: a library caller's negative --bytes start
// fails invalid-span instead of slicing the evidence blob out of range.
func TestCiteRefusesNegativeByteSpan(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// TestMarkRefusesARetiredMapBeforeMutation pins that a retired 0.1 map is
// refused before any mutation, with the migration fix (CEM-V1-216).
func TestMarkRefusesARetiredMapBeforeMutation(t *testing.T) {
	t.Parallel()
	root, _, _ := makeRepo(t)
	writeFile(t, root, "staged.cem.json", `{"spec":"cem/0.1","baseRevision":"4ca153370afd9bd8c6034ad73acc3925150ab681",`+
		`"patchSha256":"dec61287f7b726144fc19d67f0e07f3c40410c28bc19831a4b0f9fb96487717c",`+
		`"evidence":[],"hunks":[{"id":"hunk:sha256:07461a992e03e064986720e365dc4bb477da7cefe73da853f51bcb70e2c3100c",`+
		`"path":"src/app.py","oldRange":{"start":1,"count":2},"newRange":{"start":1,"count":2},`+
		`"disposition":"unknown","reason":"no-evidence","basis":[]}]}`)
	_, err := openSession(t, root).Mark(ctx(), MarkOptions{
		MapPath: "staged.cem.json", Hunk: "1", Disposition: "mechanical", Reason: "rename", Output: "out.cem.json",
	})
	if cemcode.CodeOf(err) != cemcode.UnsupportedSpec || !strings.Contains(err.Error(), "fix: "+wire.RetiredSpecFix) {
		t.Fatalf("got %v, want unsupported-spec with the migration fix", err)
	}
}

// TestCEMV1322RemovedSpanRefusalNamesRuleAndNearestStableSpans pins
// CEM-V1-322: citing base lines the change removes keeps the code
// cite-span-not-stable, states the stability rule, and names the nearest
// stable base spans above and below, each of which then cites.
func TestCEMV1322RemovedSpanRefusalNamesRuleAndNearestStableSpans(t *testing.T) {
	t.Parallel()
	baseContent := "one\ntwo\nthree\nfour\nfive\nsix\n"
	root, base, target := makeCiteSpanRepoFrom(t, baseContent, "one\ntwo\nfive\nsix\n")
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	_, err := session.Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt",
		Lines: "3:4", Relation: "implementation",
	})
	if cemcode.CodeOf(err) != cemcode.CiteSpanNotStable {
		t.Fatalf("got %v, want cite-span-not-stable", err)
	}
	message := cemcode.MessageOf(err)
	for _, want := range []string{"would be stale", citeSpanRule, "nearest stable base spans: --lines 1:2, --lines 5:6"} {
		if !strings.Contains(message, want) {
			t.Fatalf("refusal %q lacks %q", message, want)
		}
	}
	for _, lines := range []string{"1:2", "5:6"} {
		if _, err := session.Cite(ctx(), CiteOptions{
			MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt",
			Lines: lines, Relation: "implementation",
		}); err != nil {
			t.Fatalf("named span %s does not cite: %v", lines, err)
		}
	}
}

// TestSpanLinesBoundariesV11436 is V1-1436: spanLines maps a byte span to the
// one-based inclusive base lines at the first line, the last line, a span
// ending on a newline, and a last line with no trailing newline.
func TestSpanLinesBoundariesV11436(t *testing.T) {
	t.Parallel()
	blob := []byte("a\nbb\nccc\n")
	unterminated := []byte("a\nb")
	for _, test := range []struct {
		blob       []byte
		span       wire.Span
		start, end int64
	}{
		{blob, wire.Span{Start: 0, End: 1}, 1, 1},
		{blob, wire.Span{Start: 0, End: 2}, 1, 1},
		{blob, wire.Span{Start: 2, End: 5}, 2, 2},
		{blob, wire.Span{Start: 1, End: 3}, 1, 2},
		{blob, wire.Span{Start: 5, End: 9}, 3, 3},
		{blob, wire.Span{Start: 0, End: 9}, 1, 3},
		{unterminated, wire.Span{Start: 2, End: 3}, 2, 2},
	} {
		start, end := spanLines(test.blob, test.span)
		if start != test.start || end != test.end {
			t.Fatalf("span %d-%d of %q = lines %d:%d, want %d:%d",
				test.span.Start, test.span.End, test.blob, start, end, test.start, test.end)
		}
	}
}

// TestSpanUneditedByHunksZeroCountHunksV11436 is V1-1436: a zero-count old
// range names the line before the insertion, so the inserted lines split only
// spans that straddle that point; a pure deletion edits exactly its lines.
func TestSpanUneditedByHunksZeroCountHunksV11436(t *testing.T) {
	t.Parallel()
	insertion := &patch.Group{Hunks: []*patch.Hunk{{
		OldRange: wire.Range{Start: 2, Count: 0},
		Body:     []patch.BodyLine{{Prefix: '+'}, {Prefix: '+'}},
	}}}
	atStart := &patch.Group{Hunks: []*patch.Hunk{{
		OldRange: wire.Range{Start: 0, Count: 0},
		Body:     []patch.BodyLine{{Prefix: '+'}},
	}}}
	deletion := &patch.Group{Hunks: []*patch.Hunk{{
		OldRange: wire.Range{Start: 3, Count: 2},
		Body:     []patch.BodyLine{{Prefix: '-'}, {Prefix: '-'}},
	}}}
	for _, test := range []struct {
		name       string
		group      *patch.Group
		start, end int64
		want       bool
	}{
		{"insertion after 2 splits 2:3", insertion, 2, 3, false},
		{"insertion after 2 leaves 2:2", insertion, 2, 2, true},
		{"insertion after 2 leaves 3:3", insertion, 3, 3, true},
		{"insertion at start leaves 1:1", atStart, 1, 1, true},
		{"insertion at start leaves 1:2", atStart, 1, 2, true},
		{"deletion of 3-4 leaves 2:2", deletion, 2, 2, true},
		{"deletion of 3-4 edits 3:3", deletion, 3, 3, false},
		{"deletion of 3-4 edits 4:4", deletion, 4, 4, false},
		{"deletion of 3-4 leaves 5:5", deletion, 5, 5, true},
	} {
		if got := spanUneditedByHunks(test.group, test.start, test.end); got != test.want {
			t.Fatalf("%s: got %v, want %v", test.name, got, test.want)
		}
	}
}

// TestSpanUneditedByHunksNoNewlineHunkV11436 is V1-1436: git's
// "\ No newline at end of file" marker is not a body line, so it advances no
// base line; the parsed hunk of a real canonical diff edits only the
// unterminated last line.
func TestSpanUneditedByHunksNoNewlineHunkV11436(t *testing.T) {
	t.Parallel()
	root, base, target := makeCiteSpanRepoFrom(t, "alpha\nbeta\ngamma", "alpha\nbeta\ngamma\ndelta\n")
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	raw, err := session.repository.CanonicalDiff(ctx(), base, target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("\\ No newline at end of file")) {
		t.Fatalf("fixture diff carries no no-newline marker:\n%s", raw)
	}
	parsed, err := patch.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(parsed.Groups))
	}
	group := parsed.Groups[0]
	for _, test := range []struct {
		start, end int64
		want       bool
	}{{1, 1, true}, {2, 2, true}, {1, 2, true}, {3, 3, false}, {2, 3, false}} {
		if got := spanUneditedByHunks(group, test.start, test.end); got != test.want {
			t.Fatalf("lines %d:%d unedited=%v, want %v", test.start, test.end, got, test.want)
		}
	}
}
