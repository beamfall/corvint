package convert

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// testGit runs git hermetically in dir with a fixed identity and date.
func testGit(t *testing.T, dir string, stdin []byte, env []string, name, email, date string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "core.autocrlf=false", "-c", "gc.auto=0"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull,
		"GIT_AUTHOR_NAME=" + name, "GIT_AUTHOR_EMAIL=" + email, "GIT_COMMITTER_NAME=" + name, "GIT_COMMITTER_EMAIL=" + email,
		"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}, env...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, stderr.Bytes())
	}
	return strings.TrimSpace(string(out))
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := realTempDir(t)
	testGit(t, dir, nil, nil, "x", "x@example.invalid", "2000-01-01T00:00:00+0000", "init", "-q", "--object-format=sha1", "-b", "main")
	return dir
}

// plumbingCommit writes one commit of exact file bytes without a checkout.
func plumbingCommit(t *testing.T, dir string, files map[string][]byte, parent, message, name, email, date string) string {
	t.Helper()
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")}
	for path, data := range files {
		oid := testGit(t, dir, data, nil, name, email, date, "hash-object", "-w", "--no-filters", "--stdin")
		testGit(t, dir, nil, env, name, email, date, "update-index", "--add", "--cacheinfo", "100644,"+oid+","+path)
	}
	tree := testGit(t, dir, nil, env, name, email, date, "write-tree")
	args := []string{"commit-tree", tree, "-m", message}
	if parent != "" {
		args = append(args, "-p", parent)
	}
	return testGit(t, dir, nil, env, name, email, date, args...)
}

func stableVerify(t *testing.T, repository string, converted []byte, base, target string) verify.StableResult {
	t.Helper()
	r, exit := verify.Stable(context.Background(), converted, verify.StableOptions{Repository: repository, ExpectedBase: base, Target: target, ArtifactRoot: realTempDir(t)})
	t.Logf("stable exit %d", exit)
	return r
}

func code(r verify.StableResult) string {
	if r.Code == nil {
		return ""
	}
	return *r.Code
}

// TestCEMV1099ConvertGitReaderStates drives the production reader over real
// commits: the exact source at the sidecar, no sidecar, and other bytes.
func TestCEMV1099ConvertGitReaderStates(t *testing.T) {
	f := fixtures[0]
	src := readTestdata(t, f.name+".source.json")
	dir := initRepo(t)
	id := []string{"x", "x@example.invalid", "2000-01-01T00:00:00+0000"}
	exact := plumbingCommit(t, dir, map[string][]byte{"a.txt": []byte("a\n"), wire.ExcludedCEMPath: src}, "", "exact", id[0], id[1], id[2])
	absent := plumbingCommit(t, dir, map[string][]byte{"a.txt": []byte("a\n")}, "", "absent", id[0], id[1], id[2])
	other := plumbingCommit(t, dir, map[string][]byte{wire.ExcludedCEMPath: []byte("{}\n")}, "", "other", id[0], id[1], id[2])
	reader := NewGitReader(context.Background(), dir)
	for _, c := range []struct{ commit, want string }{{exact, BindingExact}, {absent, BindingAbsent}, {strings.Repeat("0", 40), BindingUnavailable}} {
		_, r := mustConvert(t, src, Inputs{BindCommit: c.commit, Git: reader})
		if r.SourceBinding != c.want {
			t.Fatalf("commit %s: binding %s, want %s", c.commit, r.SourceBinding, c.want)
		}
	}
	if _, r, err := Convert(src, Inputs{BindCommit: other, Git: reader}); cemcode.CodeOf(err) != CodeSourceBindingMismatch || r.SourceBinding != BindingMismatch {
		t.Fatalf("other bytes: %+v %v", r, err)
	}
}

// TestCEMV1103ConvertUnavailableObjects converts with no pinned object
// available: the bytes equal the golden and stable verification abstains with
// an explicit UNSUPPORTED git-read-failed at the repository stage, the same
// refusal the 0.x verifier gives for the source.
func TestCEMV1103ConvertUnavailableObjects(t *testing.T) {
	f := fixtures[0]
	src := readTestdata(t, f.name+".source.json")
	dir := initRepo(t)
	out, r := mustConvert(t, src, Inputs{BindCommit: f.bind, Git: NewGitReader(context.Background(), dir)})
	if r.SourceBinding != BindingUnavailable || !bytes.Equal(out, readTestdata(t, f.name+".converted.json")) {
		t.Fatalf("binding %s, golden equal %v", r.SourceBinding, bytes.Equal(out, readTestdata(t, f.name+".converted.json")))
	}
	doc, err := wire.ParseMap(src)
	if err != nil {
		t.Fatal(err)
	}
	res := stableVerify(t, dir, out, doc.BaseRevision, f.bind)
	if res.Outcome != "UNSUPPORTED" || res.Stage != "repository" || code(res) != cemcode.GitReadFailed {
		t.Fatalf("stable = %s/%s", res.Outcome, code(res))
	}
	repo, err := gitauth.Open(dir, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := verify.Canonical(context.Background(), repo, doc, verify.CanonicalOptions{ExpectedBase: doc.BaseRevision, Target: f.bind, RawMapBytes: src}); cemcode.CodeOf(err) != cemcode.GitReadFailed {
		t.Fatalf("0.2 source = %v", err)
	}
}

// TestCEMV1104ConvertParityCEM03 rebuilds the cem/0.3 vector repository from
// its manifest: source and converted map both ACCEPT, with equal hunks.
func TestCEMV1104ConvertParityCEM03(t *testing.T) {
	f := fixtures[2]
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "protocol", "cem-0.3", "vectors", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Commits []struct {
			Name, Repository, Message, Revision string
			Files                               []struct{ Path, Mode, Content string }
		}
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	dir := initRepo(t)
	revs := map[string]string{}
	for _, c := range manifest.Commits {
		if c.Repository != "sha1" || (c.Name != "semantic/base" && c.Name != "semantic/target") || revs[c.Name] != "" {
			continue
		}
		files := map[string][]byte{}
		for _, file := range c.Files {
			files[file.Path] = []byte(file.Content)
		}
		got := plumbingCommit(t, dir, files, "", c.Message, "Portable Proof", "proof@example.invalid", "2026-01-01T00:00:00Z")
		if got != c.Revision {
			t.Fatalf("%s rebuilt as %s, manifest %s", c.Name, got, c.Revision)
		}
		revs[c.Name] = got
	}
	base, target := revs["semantic/base"], revs["semantic/target"]
	if target != f.bind {
		t.Fatalf("target %s, fixture bind %s", target, f.bind)
	}
	src := readTestdata(t, f.name+".source.json")
	out := readTestdata(t, f.name+".converted.json")
	res := stableVerify(t, dir, out, base, target)
	if res.Outcome != "ACCEPT" || res.Axes["changeIntegrity"] != "VERIFIED" || res.Sidecar != "ABSENT" {
		j, _ := json.Marshal(res)
		t.Fatalf("stable = %s", j)
	}
	assertHunksCarry(t, src, res)
	doc, err := wire.ParseMap(src)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := gitauth.Open(dir, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := verify.Canonical(context.Background(), repo, doc, verify.CanonicalOptions{ExpectedBase: base, Target: target, RawMapBytes: src}); err != nil {
		t.Fatalf("0.3 source = %v", err)
	}
}

// TestCEMV1104ConvertParityCEM01 rebuilds the cem/0.1 conformance repository.
// cem/0.1 binds an out-of-band patch whose index lines are not Git's, so its
// patchSha256 is not the canonical base..target digest. The converted map
// rejects patch-digest-mismatch, as the source does against the canonical
// patch; only the out-of-band patch, which cem/1.0 cannot carry, accepts the
// source (the CEM-V1-104 0.1 limit).
func TestCEMV1104ConvertParityCEM01(t *testing.T) {
	f := fixtures[1]
	conf := filepath.Join("..", "..", "..", "conformance", "cem-0.1")
	dir := initRepo(t)
	id := []string{"CEM Conformance", "cem@example.invalid", "2000-01-01T00:00:00+0000"}
	baseFiles := map[string][]byte{}
	for _, p := range []string{"docs/rule.txt", "src/app.py"} {
		b, err := os.ReadFile(filepath.Join(conf, "base", filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		baseFiles[p] = b
	}
	base := plumbingCommit(t, dir, baseFiles, "", "CEM 0.1 base", id[0], id[1], id[2])
	if base != "4ca153370afd9bd8c6034ad73acc3925150ab681" {
		t.Fatalf("0.1 base rebuilt as %s", base)
	}
	patch, err := os.ReadFile(filepath.Join(conf, "change.patch"))
	if err != nil {
		t.Fatal(err)
	}
	index := []string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")}
	testGit(t, dir, nil, index, id[0], id[1], id[2], "read-tree", base)
	testGit(t, dir, patch, index, id[0], id[1], id[2], "apply", "--cached", "-")
	tree := testGit(t, dir, nil, index, id[0], id[1], id[2], "write-tree")
	target := testGit(t, dir, nil, nil, id[0], id[1], id[2], "commit-tree", tree, "-p", base, "-m", "CEM 0.1 target")
	if target != f.bind {
		t.Fatalf("0.1 target rebuilt as %s, fixture bind %s", target, f.bind)
	}
	src := readTestdata(t, f.name+".source.json")
	res := stableVerify(t, dir, readTestdata(t, f.name+".converted.json"), base, target)
	if res.Outcome != "REJECT" || res.Stage != "verification" || len(res.IssueCodes) != 1 || res.IssueCodes[0] != cemcode.PatchDigestMismatch {
		t.Fatalf("stable = %s/%s/%s %v", res.Outcome, res.Stage, code(res), res.IssueCodes)
	}
	doc, err := wire.ParseMap(src)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := gitauth.Open(dir, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := repo.CanonicalDiff(context.Background(), base, target)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(canonical, patch) {
		t.Fatal("conformance patch unexpectedly equals the canonical diff")
	}
	// The stable comparison point: the source checked against the canonical
	// base..target patch refuses with the code the converted map carries.
	if _, err := verify.Exact(context.Background(), repo, doc, canonical, verify.ExactOptions{ExpectedBase: base}); cemcode.CodeOf(err) != cemcode.PatchDigestMismatch {
		t.Fatalf("0.1 source with canonical patch = %v", err)
	}
	// Only the out-of-band patch, which cem/1.0 has no member for, accepts it.
	if _, err := verify.Exact(context.Background(), repo, doc, patch, verify.ExactOptions{ExpectedBase: base}); err != nil {
		t.Fatalf("0.1 source exact = %v", err)
	}
}

func assertHunksCarry(t *testing.T, src []byte, res verify.StableResult) {
	t.Helper()
	var m struct {
		Hunks []any `json:"hunks"`
	}
	if err := json.Unmarshal(src, &m); err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(m.Hunks)
	b, _ := json.Marshal(res.Hunks)
	if !bytes.Equal(a, b) {
		t.Fatalf("verified hunks differ from source hunks:\n%s\n%s", a, b)
	}
}

// realTempDir is t.TempDir with symlinks resolved: the stable boundary admits
// only ordinary directories, and the macOS temporary root is reached through
// the /var symlink.
func realTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
