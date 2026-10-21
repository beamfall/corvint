//go:build darwin || linux

package verify

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The native repository-envelope qualification (V1-0595 U2, CEM-V1-360..367):
// the 29 public cells of the S0E packet run in-process against fresh real Git
// repositories, so ordinary, origin-config, linked, relative-linked and
// SHA-256 checkouts are qualified natively and not only by the portable
// consumer's TestStableCanonicalEnvelopeCells.

// envelopeCell is one REPOSITORY-ENVELOPE.cells.json case.
type envelopeCell struct {
	ID             string         `json:"id"`
	ObjectFormat   string         `json:"objectFormat"`
	Map            string         `json:"map"`
	ExpectedBase   string         `json:"expectedBase"`
	Target         string         `json:"target"`
	ExpectedExit   int            `json:"expectedExit"`
	ExpectedResult map[string]any `json:"expectedResult"`
}

func loadEnvelopeCells(t *testing.T, packet *s0ePacket) []envelopeCell {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(packet.root, "inherited", "s0e", "REPOSITORY-ENVELOPE.cells.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Envelope string         `json:"envelope"`
		Cases    []envelopeCell `json:"cases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Envelope != "canonical-repository-bounded/1" || len(doc.Cases) != 29 {
		t.Fatalf("cells document names %q with %d cases, want canonical-repository-bounded/1 with 29", doc.Envelope, len(doc.Cases))
	}
	return doc.Cases
}

// envelopeGit runs the host Git for fixture construction only, with no
// ambient configuration.
func envelopeGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=/nonexistent-cem-envelope-home", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + os.DevNull, "LC_ALL=C", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@invalid"}
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// envelopeRepo runs a fresh `git init` (with --object-format=sha256 for the
// SHA-256 format), never touches the configuration Git wrote, and copies only
// the packet's literal objects of that format, each checked against its name.
func envelopeRepo(t *testing.T, packet *s0ePacket, dir, format, head string) {
	t.Helper()
	args := []string{"-c", "init.defaultBranch=main", "init", "-q"}
	if format == "sha256" {
		args = append(args, "--object-format=sha256")
	}
	envelopeGit(t, "/", append(args, dir)...)
	prefix := "fixtures/git-objects/" + format + "/"
	copied := 0
	for path, data := range packet.records {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		oid := strings.TrimSuffix(filepath.Base(path), ".object")
		var sum string
		if format == "sha1" {
			digest := sha1.Sum(data)
			sum = hex.EncodeToString(digest[:])
		} else {
			digest := sha256.Sum256(data)
			sum = hex.EncodeToString(digest[:])
		}
		if sum != oid {
			t.Fatalf("literal object %s does not hash to its name", oid)
		}
		writeLooseObject(t, filepath.Join(dir, ".git"), oid, data)
		copied++
	}
	if copied == 0 {
		t.Fatalf("packet has no %s literal objects", format)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "refs", "heads", "main"), []byte(head+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// envelopeFixture is one fresh fixture: B, the primary repository R and its
// .git G, the artifact root and the sentinel that hostile configuration names.
type envelopeFixture struct {
	t               *testing.T
	packet          *s0ePacket
	cell            envelopeCell
	base, repo, git string
	artifacts       string
	sentinel, fired string
	verified        string
}

func newEnvelopeFixture(t *testing.T, packet *s0ePacket, c envelopeCell) *envelopeFixture {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &envelopeFixture{t: t, packet: packet, cell: c, base: base, repo: filepath.Join(base, "repo"), artifacts: filepath.Join(base, "artifacts")}
	f.git, f.verified = filepath.Join(f.repo, ".git"), f.repo
	envelopeRepo(t, packet, f.repo, c.ObjectFormat, c.Target)
	if err := os.MkdirAll(f.artifacts, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, data := range packet.records {
		if strings.HasPrefix(path, "fixtures/artifacts/") {
			f.writeFile(filepath.Join(f.artifacts, strings.TrimPrefix(path, "fixtures/artifacts/")), string(data), 0o644)
		}
	}
	f.fired = filepath.Join(base, "sentinel-fired")
	f.sentinel = filepath.Join(base, "sentinel")
	f.writeFile(f.sentinel, "#!/bin/sh\necho \"$0 $*\" >> "+f.fired+"\nexit 1\n", 0o755)
	return f
}

func (f *envelopeFixture) writeFile(path, content string, mode os.FileMode) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	os.Remove(path)
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		f.t.Fatal(err)
	}
}

// worktree runs an actual `git worktree add --detach` at the cell's target and
// returns the worktree W and its administrative directory A.
func (f *envelopeFixture) worktree(extra ...string) (string, string) {
	f.t.Helper()
	worktree := filepath.Join(f.base, "wt")
	args := append(append([]string{"worktree", "add", "-q"}, extra...), "--detach", worktree, f.cell.Target)
	envelopeGit(f.t, f.repo, args...)
	return worktree, filepath.Join(f.git, "worktrees", "wt")
}

func (f *envelopeFixture) config(pairs ...[2]string) {
	f.t.Helper()
	for _, pair := range pairs {
		envelopeGit(f.t, f.repo, "config", pair[0], pair[1])
	}
}

// treeEntry returns the root tree of commit and the blob OID of name in it,
// read from the packet's literal objects.
func (f *envelopeFixture) treeEntry(commit, name string) (string, string) {
	f.t.Helper()
	object := func(oid string) []byte {
		data := f.packet.records["fixtures/git-objects/"+f.cell.ObjectFormat+"/"+oid+".object"]
		if data == nil {
			f.t.Fatalf("packet has no literal object %s", oid)
		}
		_, payload, _ := strings.Cut(string(data), "\x00")
		return []byte(payload)
	}
	header, _, _ := strings.Cut(string(object(commit)), "\n")
	tree, found := strings.CutPrefix(header, "tree ")
	if !found {
		f.t.Fatalf("commit %s has no tree header", commit)
	}
	size := len(commit) / 2
	entries := object(tree)
	for len(entries) > 0 {
		space := strings.IndexByte(string(entries), ' ')
		nul := strings.IndexByte(string(entries), 0)
		if space < 0 || nul < space || len(entries) < nul+1+size {
			f.t.Fatalf("tree %s is malformed", tree)
		}
		if string(entries[space+1:nul]) == name {
			return tree, hex.EncodeToString(entries[nul+1 : nul+1+size])
		}
		entries = entries[nul+1+size:]
	}
	f.t.Fatalf("tree %s has no %s", tree, name)
	return "", ""
}

func (f *envelopeFixture) remove(path string) {
	f.t.Helper()
	if err := os.Remove(path); err != nil {
		f.t.Fatal(err)
	}
}

func (f *envelopeFixture) rename(from, to string) {
	f.t.Helper()
	if err := os.Rename(from, to); err != nil {
		f.t.Fatal(err)
	}
}

func (f *envelopeFixture) symlink(target, link string) {
	f.t.Helper()
	if err := os.Symlink(target, link); err != nil {
		f.t.Fatal(err)
	}
}

// run verifies the cell's map over f.verified in process and reports whether
// the hostile-configuration sentinel executed.
func (f *envelopeFixture) run() (StableResult, int) {
	f.t.Helper()
	raw := f.packet.records["fixtures/"+f.cell.Map]
	if raw == nil {
		f.t.Fatalf("packet has no map %s", f.cell.Map)
	}
	result, exit := Stable(context.Background(), raw, StableOptions{Repository: f.verified, ExpectedBase: f.cell.ExpectedBase, Target: f.cell.Target, ArtifactRoot: f.artifacts})
	if fired, err := os.ReadFile(f.fired); err == nil {
		f.t.Errorf("sentinel executed: %s", strings.TrimSpace(string(fired)))
	}
	return result, exit
}

// envelopeCellRecipes are the cells document's recipes over a fresh fixture;
// the env return value is applied with t.Setenv before the call.
var envelopeCellRecipes = map[string]func(f *envelopeFixture) [][2]string{
	"ordinary-sha1": nil, "ordinary-sha256": nil,
	"origin-sha1": envelopeOrigin, "origin-sha256": envelopeOrigin,
	"linked-sha1":   func(f *envelopeFixture) [][2]string { f.verified, _ = f.worktree(); return nil },
	"linked-sha256": func(f *envelopeFixture) [][2]string { f.verified, _ = f.worktree(); return nil },
	"relative-sha1": envelopeRelative, "relative-sha256": envelopeRelative,
	"attributes": func(f *envelopeFixture) [][2]string {
		f.writeFile(filepath.Join(f.git, "info", "attributes"), "*.go diff=evil\n", 0o644)
		return nil
	},
	"comments": func(f *envelopeFixture) [][2]string {
		f.writeFile(filepath.Join(f.git, "info", "attributes"), "# comment only\n", 0o644)
		return nil
	},
	"alternates": func(f *envelopeFixture) [][2]string {
		f.writeFile(filepath.Join(f.git, "objects", "info", "alternates"), "/no/objects\n", 0o644)
		return nil
	},
	"bad-marker": func(f *envelopeFixture) [][2]string {
		f.rename(f.git, f.git+"-moved")
		f.writeFile(f.git, "not a gitfile\n", 0o644)
		return nil
	},
	"object-link": func(f *envelopeFixture) [][2]string {
		f.rename(filepath.Join(f.git, "objects"), filepath.Join(f.git, "objects-real"))
		f.symlink("objects-real", filepath.Join(f.git, "objects"))
		return nil
	},
	"root-link": func(f *envelopeFixture) [][2]string {
		f.verified = filepath.Join(f.base, "link")
		f.symlink(f.repo, f.verified)
		return nil
	},
	"shallow-complete": func(f *envelopeFixture) [][2]string { return f.shallow("3") },
	"shallow-missing":  func(f *envelopeFixture) [][2]string { return f.shallow("1") },
	"missing-base": func(f *envelopeFixture) [][2]string {
		f.remove(looseObjectPath(f.git, f.cell.ExpectedBase))
		return nil
	},
	"corrupt-base": func(f *envelopeFixture) [][2]string {
		writeObjectFile(f.t, f.git, f.cell.ExpectedBase, []byte("corrupt\x00not zlib"))
		return nil
	},
	"hostile": func(f *envelopeFixture) [][2]string {
		f.config([2]string{"diff.external", f.sentinel}, [2]string{"diff.evil.command", f.sentinel}, [2]string{"diff.evil.textconv", f.sentinel},
			[2]string{"filter.evil.clean", f.sentinel}, [2]string{"filter.evil.smudge", f.sentinel}, [2]string{"core.fsmonitor", f.sentinel},
			[2]string{"core.hooksPath", filepath.Join(f.base, "hooks")}, [2]string{"credential.helper", "!" + f.sentinel},
			[2]string{"core.repositoryformatversion", "1"}, [2]string{"extensions.partialClone", "origin"},
			[2]string{"remote.origin.url", "ext::" + f.sentinel + " %S"}, [2]string{"remote.origin.promisor", "true"})
		for _, hook := range []string{"pre-commit", "post-checkout", "reference-transaction", "fsmonitor-watchman"} {
			f.writeFile(filepath.Join(f.base, "hooks", hook), "#!/bin/sh\nexec "+f.sentinel+" hook\n", 0o755)
		}
		ambient := filepath.Join(f.base, "ambient.gitconfig")
		f.writeFile(ambient, "[diff]\n\texternal = "+f.sentinel+"\n[core]\n\tfsmonitor = "+f.sentinel+"\n\tattributesFile = "+filepath.Join(f.base, "ambient.attributes")+"\n", 0o644)
		f.writeFile(filepath.Join(f.base, "ambient.attributes"), "* diff=evil\n", 0o644)
		f.writeFile(filepath.Join(f.base, "home", ".gitconfig"), "[diff]\n\texternal = "+f.sentinel+"\n", 0o644)
		return [][2]string{{"GIT_CONFIG_GLOBAL", ambient}, {"GIT_CONFIG_SYSTEM", ambient}, {"GIT_CONFIG_NOSYSTEM", ""},
			{"GIT_CONFIG_COUNT", "1"}, {"GIT_CONFIG_KEY_0", "diff.external"}, {"GIT_CONFIG_VALUE_0", f.sentinel},
			{"GIT_DIR", "/trap/git-dir"}, {"GIT_WORK_TREE", "/trap/work-tree"}, {"GIT_OBJECT_DIRECTORY", "/trap/objects"},
			{"GIT_EXTERNAL_DIFF", f.sentinel}, {"GIT_REPLACE_REF_BASE", "refs/trap/"}, {"GIT_GRAFT_FILE", "/trap/graft"},
			{"XDG_CONFIG_HOME", filepath.Join(f.base, "home")}, {"HOME", filepath.Join(f.base, "home")}}
	},
	"ambient-alternates": func(f *envelopeFixture) [][2]string {
		return [][2]string{{"GIT_ALTERNATE_OBJECT_DIRECTORIES", "/no/alternate"}}
	},
	"linked-wrong-backpointer": func(f *envelopeFixture) [][2]string {
		var admin string
		f.verified, admin = f.worktree()
		f.writeFile(filepath.Join(admin, "gitdir"), "/not/reciprocal/.git\n", 0o644)
		return nil
	},
	"linked-missing-common": func(f *envelopeFixture) [][2]string {
		var admin string
		f.verified, admin = f.worktree()
		f.writeFile(filepath.Join(admin, "commondir"), "/missing/common\n", 0o644)
		return nil
	},
	"linked-attributes": func(f *envelopeFixture) [][2]string {
		var admin string
		f.verified, admin = f.worktree()
		f.writeFile(filepath.Join(admin, "info", "attributes"), "* diff=evil\n", 0o644)
		return nil
	},
	"pack-link": func(f *envelopeFixture) [][2]string {
		pack := filepath.Join(f.git, "objects", "pack")
		f.rename(pack, pack+"-real")
		f.symlink("pack-real", pack)
		return nil
	},
	"info-link": func(f *envelopeFixture) [][2]string {
		info := filepath.Join(f.git, "objects", "info")
		f.rename(info, info+"-real")
		f.symlink("info-real", info)
		return nil
	},
	"missing-promisor": func(f *envelopeFixture) [][2]string {
		f.remove(looseObjectPath(f.git, f.cell.ExpectedBase))
		f.config([2]string{"core.repositoryformatversion", "1"}, [2]string{"extensions.partialClone", "origin"},
			[2]string{"remote.origin.url", "ext::" + f.sentinel + " %S"}, [2]string{"remote.origin.promisor", "true"})
		return nil
	},
	"missing-target-tree": func(f *envelopeFixture) [][2]string {
		tree, _ := f.treeEntry(f.cell.Target, "app.txt")
		f.remove(looseObjectPath(f.git, tree))
		return nil
	},
	"missing-target-blob": func(f *envelopeFixture) [][2]string {
		_, blob := f.treeEntry(f.cell.Target, "app.txt")
		f.remove(looseObjectPath(f.git, blob))
		return nil
	},
	"corrupt-target-blob": func(f *envelopeFixture) [][2]string {
		_, blob := f.treeEntry(f.cell.Target, "app.txt")
		writeObjectFile(f.t, f.git, blob, []byte("corrupt\x00not zlib"))
		return nil
	},
}

func envelopeOrigin(f *envelopeFixture) [][2]string {
	f.config([2]string{"remote.origin.url", "/nonexistent/origin.git"}, [2]string{"user.name", "Inert"}, [2]string{"branch.main.remote", "origin"})
	return nil
}

// envelopeRelativePaths reports whether the host Git offers `git worktree
// add --relative-paths` (Git 2.48 or later), read from the command's own
// usage text in repository dir; CEM-V1-185 alone requires only Git 2.41.
func envelopeRelativePaths(t *testing.T, dir string) bool {
	t.Helper()
	command := exec.Command("git", "worktree", "add", "-h")
	command.Dir = dir
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=/nonexistent-cem-envelope-home", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + os.DevNull, "LC_ALL=C"}
	usage, _ := command.CombinedOutput()
	return strings.Contains(string(usage), "relative-paths")
}

func envelopeRelative(f *envelopeFixture) [][2]string {
	if !envelopeRelativePaths(f.t, f.repo) {
		f.t.Skipf("NOT_RUN %s: host Git has no `git worktree add --relative-paths` (Git 2.48 or later); relative worktree cells are unavailable on this host (CEM-V1-361)", f.cell.ID)
	}
	f.verified, _ = f.worktree("--relative-paths")
	marker, err := os.ReadFile(filepath.Join(f.verified, ".git"))
	if err != nil {
		f.t.Fatal(err)
	}
	if filepath.IsAbs(strings.TrimSpace(strings.TrimPrefix(string(marker), "gitdir: "))) {
		f.t.Fatal("worktree marker is not relative")
	}
	return nil
}

// shallow verifies an actual local file-URL clone of the given depth.
func (f *envelopeFixture) shallow(depth string) [][2]string {
	f.verified = filepath.Join(f.base, "clone")
	envelopeGit(f.t, f.base, "clone", "-q", "--depth="+depth, "--branch=main", "file://"+f.repo, f.verified)
	return nil
}

// TestStableCanonicalEnvelopeCellsNative runs every public envelope cell
// against the native verifier over fresh real repositories and compares the
// complete 23-key result and the exit code (CEM-V1-360..362). Every one of the
// 29 cells must match the packet exactly, code and verdictSha256 included; a
// cell is skipped only as NOT_RUN on a host that cannot build it (CEM-V1-367
// resolved by V1-2133).
func TestStableCanonicalEnvelopeCellsNative(t *testing.T) {
	packet := loadS0EPacket(t)
	requireStableLifecyclePlatform(t)
	cells := loadEnvelopeCells(t, packet)
	if len(cells) != 29 {
		t.Fatalf("envelope cells: got %d, want 29", len(cells))
	}
	exact, notRun := 0, 0
	defer func() {
		t.Logf("ENVELOPE-CELLS native exact=%d notRun=%d of %d", exact, notRun, len(cells))
		if !t.Failed() && exact+notRun != len(cells) {
			t.Errorf("envelope cells: %d exact and %d NOT_RUN of %d", exact, notRun, len(cells))
		}
	}()
	for _, c := range cells {
		recipe, known := envelopeCellRecipes[c.ID]
		if !known {
			t.Fatalf("no native recipe for cell %s", c.ID)
		}
		t.Run(c.ID, func(t *testing.T) {
			t.Cleanup(func() {
				if t.Skipped() {
					notRun++
				}
			})
			bound := s0eBindRuntime(t, packet, s0eCase{ID: c.ID, ExpectedExit: c.ExpectedExit, ExpectedResult: c.ExpectedResult})
			f := newEnvelopeFixture(t, packet, c)
			if recipe != nil {
				for _, kv := range recipe(f) {
					t.Setenv(kv[0], kv[1])
				}
			}
			result, exit := f.run()
			status := "PASS"
			if !requireS0EResult(t, bound, result, exit) {
				status = "FAIL"
			} else {
				exact++
			}
			t.Logf("ENVELOPE-CELL %s %s exit=%d stage=%s", c.ID, status, exit, result.Stage)
		})
	}
}

func envelopeCellByID(t *testing.T, cells []envelopeCell, id string) envelopeCell {
	t.Helper()
	for _, c := range cells {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no envelope cell %s", id)
	return envelopeCell{}
}

// envelopeAdversarial is one shape outside the public cells: a recipe over
// the named cell's fresh fixture and the frozen answer. An empty code means
// the full result must equal the cell's own expected result. A recipe calls
// accepted to prove its fixture is the cell's accepted repository before it
// introduces the one defect under test.
type envelopeAdversarial struct {
	id, cell, code string
	recipe         func(f *envelopeFixture, accepted func())
}

// externalAdmin moves the worktree's real administrative directory to dir,
// outside the common directory, rebases its relative commondir onto the common
// directory and keeps its absolute back-pointer, so the metadata stays
// reciprocal; marker is written into the worktree's .git file.
func (f *envelopeFixture) externalAdmin(dir, marker string) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		f.t.Fatal(err)
	}
	f.rename(filepath.Join(f.git, "worktrees", "wt"), dir)
	common, err := filepath.Rel(dir, f.git)
	if err != nil {
		f.t.Fatal(err)
	}
	f.writeFile(filepath.Join(dir, "commondir"), common+"\n", 0o644)
	f.writeFile(filepath.Join(f.verified, ".git"), "gitdir: "+marker+"\n", 0o644)
}

var envelopeAdversarialCases = []envelopeAdversarial{
	// An administrative directory outside the common directory, with a
	// reciprocal back-pointer and a relative commondir: the envelope does not
	// require root containment, so this is linked-sha1's repository.
	{"gitdir-outside-reciprocal", "linked-sha1", "", func(f *envelopeFixture, accepted func()) {
		outside := filepath.Join(f.base, "outside-admin")
		f.externalAdmin(outside, outside)
	}},
	// The same accepted outside directory with its back-pointer removed.
	{"gitdir-outside-no-backpointer", "linked-sha1", "repository-object-unavailable", func(f *envelopeFixture, accepted func()) {
		outside := filepath.Join(f.base, "outside-admin")
		f.externalAdmin(outside, outside)
		accepted()
		f.remove(filepath.Join(outside, "gitdir"))
	}},
	// A marker naming another repository's real worktree administrative
	// directory, whose back-pointer names that other worktree.
	{"gitdir-outside-foreign-worktree", "linked-sha1", "repository-object-unavailable", func(f *envelopeFixture, accepted func()) {
		other := filepath.Join(f.base, "other")
		envelopeRepo(f.t, f.packet, other, "sha1", f.cell.Target)
		envelopeGit(f.t, other, "worktree", "add", "-q", "--detach", filepath.Join(f.base, "other-wt"), f.cell.Target)
		f.writeFile(filepath.Join(f.verified, ".git"), "gitdir: "+filepath.Join(other, ".git", "worktrees", "other-wt")+"\n", 0o644)
	}},
	// A relative marker reaching valid, reciprocal outside metadata, first by
	// its real path (accepted) and then through a symbolic-link ancestor.
	{"gitdir-relative-symlinked-ancestor", "linked-sha1", "repository-object-unavailable", func(f *envelopeFixture, accepted func()) {
		f.externalAdmin(filepath.Join(f.base, "real-outside", "admin"), "../real-outside/admin")
		accepted()
		f.symlink("real-outside", filepath.Join(f.base, "link"))
		f.writeFile(filepath.Join(f.verified, ".git"), "gitdir: ../link/admin\n", 0o644)
	}},
	// commondir names a symbolic link that resolves to itself.
	{"commondir-symlink-loop", "linked-sha1", "repository-object-unavailable", func(f *envelopeFixture, accepted func()) {
		admin := filepath.Join(f.git, "worktrees", "wt")
		f.symlink("loop", filepath.Join(admin, "loop"))
		f.writeFile(filepath.Join(admin, "commondir"), "loop\n", 0o644)
	}},
	// commondir names a symbolic link to the real common directory.
	{"commondir-symlink", "linked-sha1", "repository-object-unavailable", func(f *envelopeFixture, accepted func()) {
		admin := filepath.Join(f.git, "worktrees", "wt")
		f.symlink(f.git, filepath.Join(admin, "common-link"))
		f.writeFile(filepath.Join(admin, "commondir"), "common-link\n", 0o644)
	}},
	// commondir is itself a symbolic link to a file holding "../..".
	{"commondir-file-symlink", "linked-sha1", "repository-object-unavailable", func(f *envelopeFixture, accepted func()) {
		admin := filepath.Join(f.git, "worktrees", "wt")
		f.writeFile(filepath.Join(f.base, "commondir-target"), "../..\n", 0o644)
		f.remove(filepath.Join(admin, "commondir"))
		f.symlink(filepath.Join(f.base, "commondir-target"), filepath.Join(admin, "commondir"))
	}},
	// A version 1 repository naming an extension this Git does not know.
	{"unknown-extension-v1", "ordinary-sha1", "git-read-failed", func(f *envelopeFixture, accepted func()) {
		f.config([2]string{"core.repositoryformatversion", "1"}, [2]string{"extensions.corvintUnknownExtension", "true"})
	}},
	// A version 0 repository naming an unknown extension: Git ignores the
	// extensions section below version 1, and so does the envelope.
	{"unknown-extension-v0", "ordinary-sha1", "", func(f *envelopeFixture, accepted func()) {
		f.config([2]string{"extensions.corvintUnknownExtension", "true"})
	}},
	// A SHA-256 configuration rewritten with [Extensions] before a mixed-case
	// [CoRe] section and mixed-case keys; Git section and key names are case
	// insensitive, so it is the same repository.
	{"sha256-extensions-first-mixed-case", "ordinary-sha256", "", func(f *envelopeFixture, accepted func()) {
		f.writeFile(filepath.Join(f.git, "config"), "[Extensions]\n\tObjectFormat = sha256\n[CoRe]\n\tRepositoryFormatVersion = 1\n\tFileMode = true\n\tBare = false\n", 0o644)
	}},
	// A SHA-1 object store whose configuration claims SHA-256.
	{"sha1-store-claims-sha256", "ordinary-sha1", "git-read-failed", func(f *envelopeFixture, accepted func()) {
		f.config([2]string{"core.repositoryformatversion", "1"}, [2]string{"extensions.objectFormat", "sha256"})
	}},
	// A SHA-256 object store whose configuration drops the extension.
	{"sha256-store-claims-sha1", "ordinary-sha256", "git-read-failed", func(f *envelopeFixture, accepted func()) {
		f.writeFile(filepath.Join(f.git, "config"), "[core]\n\trepositoryformatversion = 0\n\tbare = false\n", 0o644)
	}},
}

// TestStableEnvelopeAdversarialShapes freezes the native answer for shapes the
// public cells do not name (CEM-V1-363..366): every one is either the same
// repository as a public positive cell or a typed refusal at stage repository.
func TestStableEnvelopeAdversarialShapes(t *testing.T) {
	t.Parallel()
	packet := loadS0EPacket(t)
	requireStableLifecyclePlatform(t)
	cells := loadEnvelopeCells(t, packet)
	for _, a := range envelopeAdversarialCases {
		t.Run(a.id, func(t *testing.T) {
			c := envelopeCellByID(t, cells, a.cell)
			bound := s0eBindRuntime(t, packet, s0eCase{ID: c.ID, ExpectedExit: c.ExpectedExit, ExpectedResult: c.ExpectedResult})
			f := newEnvelopeFixture(t, packet, c)
			if strings.HasPrefix(a.cell, "linked-") {
				f.verified, _ = f.worktree()
			}
			a.recipe(f, func() {
				t.Helper()
				result, exit := f.run()
				if !requireS0EResult(t, bound, result, exit) {
					t.Fatalf("precondition: %s fixture is not %s's accepted repository before the defect", a.id, a.cell)
				}
			})
			result, exit := f.run()
			code := ""
			if result.Code != nil {
				code = *result.Code
			}
			t.Logf("ENVELOPE-ADVERSARIAL %s exit=%d outcome=%s stage=%s code=%s", a.id, exit, result.Outcome, result.Stage, code)
			if a.code == "" {
				requireS0EResult(t, bound, result, exit)
				return
			}
			if exit != 2 || result.Outcome != "UNSUPPORTED" || result.Stage != "repository" || code != a.code || result.Accept != nil {
				t.Errorf("got exit=%d outcome=%s stage=%s code=%q accept=%v, want exit=2 UNSUPPORTED repository %q accept=null", exit, result.Outcome, result.Stage, code, result.Accept, a.code)
			}
		})
	}
}
