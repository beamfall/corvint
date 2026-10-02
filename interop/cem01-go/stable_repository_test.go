package main

// S0E canonical-repository-bounded conformance: the FULL-RESULT cases, the
// repository-envelope cells and the logical ledgers of the public packet named
// by CEM01_S0E_PACKET, or the in-tree packet when unset. Each case compares all
// 21 result keys exactly; results are written to CEM01_S0E_CONFORMANCE_OUT when
// set.

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const (
	s0eBase           = "bff36ef7f94a44f0b43dd71ba7125e0185f01fe1"
	s0eSealed         = "df752f00a5487e6f913b728a1a4addbfe194b989"
	s0eSidecarless    = "45721f1cb4ce430d59e4a495898186c60b5bec49"
	s0eLedgerBoundary = "c82a33df6ae9eea3b210fa4002b3c5bcf66d4206"
	s0eSealedMap      = "fixtures/maps/sha1-positive-sealed.json"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == stableKeeperProtocol && stableKeeperControlPresent() {
		stableKeeperMain()
	}
	if os.Getenv("CEM01_TEST_CLI") == "1" {
		main()
	}
	code := m.Run()
	s0eWriteConformance()
	os.Exit(code)
}

// --- conformance record -------------------------------------------------------

type s0eRecord struct {
	ID                int    `json:"id,omitempty"`
	Name              string `json:"name"`
	Mode              string `json:"mode"`
	Status            string `json:"status"`
	FirstDifferingKey string `json:"firstDifferingKey,omitempty"`
	ExpectedExit      int    `json:"expectedExit"`
	ActualExit        int    `json:"actualExit"`
	Toolchain         string `json:"toolchain"`
	Note              string `json:"note,omitempty"`
}

var s0eConformance = struct {
	sync.Mutex
	FullResults []s0eRecord `json:"fullResultCases"`
	Cells       []s0eRecord `json:"envelopeCells"`
	Ledgers     []s0eRecord `json:"ledgers"`
	Topology    []s0eRecord `json:"topologyControls"`
}{}

func s0eRecordAdd(list *[]s0eRecord, r s0eRecord) {
	s0eConformance.Lock()
	r.Toolchain = runtime.Version()
	*list = append(*list, r)
	s0eConformance.Unlock()
}

func s0eWriteConformance() {
	out := os.Getenv("CEM01_S0E_CONFORMANCE_OUT")
	if out == "" {
		return
	}
	s0eConformance.Lock()
	defer s0eConformance.Unlock()
	if len(s0eConformance.FullResults)+len(s0eConformance.Cells)+len(s0eConformance.Ledgers)+len(s0eConformance.Topology) == 0 {
		return
	}
	for _, l := range [][]s0eRecord{s0eConformance.FullResults, s0eConformance.Cells, s0eConformance.Ledgers, s0eConformance.Topology} {
		sort.SliceStable(l, func(i, j int) bool {
			if l[i].ID != l[j].ID {
				return l[i].ID < l[j].ID
			}
			return l[i].Name < l[j].Name
		})
	}
	b, _ := json.MarshalIndent(map[string]any{
		"profile":          "cem01-go-s0e-portable-conformance/1",
		"comparison":       "complete 21-key exact equality of the decoded result object plus exit status; no normalization",
		"fullResultCases":  s0eConformance.FullResults,
		"envelopeCells":    s0eConformance.Cells,
		"ledgers":          s0eConformance.Ledgers,
		"topologyControls": s0eConformance.Topology,
	}, "", "  ")
	_ = os.WriteFile(out, append(b, '\n'), 0o644)
}

// s0eCompare returns the first differing key in sorted order ("" when equal).
func s0eCompare(got stableResult, gotExit int, want map[string]any, wantExit int) string {
	b, err := json.Marshal(got)
	if err != nil {
		return "marshal"
	}
	var g map[string]any
	if err := json.Unmarshal(b, &g); err != nil {
		return "unmarshal"
	}
	if len(want) != 21 || len(g) != 21 {
		return "key-count"
	}
	keys := []string{}
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !reflect.DeepEqual(g[k], want[k]) {
			return k
		}
	}
	if gotExit != wantExit {
		return "exit"
	}
	return ""
}

// --- packet and fixtures ------------------------------------------------------

var (
	s0eOnce        sync.Once
	s0eFiles       map[string][]byte
	s0eErr         error
	s0ePacketCache struct {
		sync.Mutex
		path string
		err  error
	}
)

func s0eDefaultPacket() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("cannot locate stable_repository_test.go")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "protocol", "cem-1.0", "stable", "repository-envelope-packet")), nil
}

func s0eValidatePacket(p string) error {
	raw, err := os.ReadFile(filepath.Join(p, "manifest.json"))
	if err != nil {
		return err
	}
	var manifest struct {
		Files []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
			Bytes  int64  `json:"bytes"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return err
	}
	if len(manifest.Files) == 0 {
		return errors.New("manifest has no files")
	}
	for _, f := range manifest.Files {
		clean := filepath.Clean(filepath.FromSlash(f.Path))
		if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("manifest path escapes packet: %s", f.Path)
		}
		b, err := os.ReadFile(filepath.Join(p, clean))
		if err != nil {
			return fmt.Errorf("%s: %w", f.Path, err)
		}
		if int64(len(b)) != f.Bytes || shaHex(b) != f.SHA256 {
			return fmt.Errorf("%s: digest/size mismatch", f.Path)
		}
	}
	return nil
}

func s0ePacket(t *testing.T) string {
	t.Helper()
	p := os.Getenv("CEM01_S0E_PACKET")
	if p == "" {
		var err error
		p, err = s0eDefaultPacket()
		if err != nil {
			t.Fatal(err)
		}
	}
	s0ePacketCache.Lock()
	if s0ePacketCache.path != p {
		s0ePacketCache.path = p
		s0ePacketCache.err = s0eValidatePacket(p)
	}
	err := s0ePacketCache.err
	s0ePacketCache.Unlock()
	if err != nil {
		t.Fatalf("S0E packet %s failed manifest validation: %v", p, err)
	}
	return p
}

func s0eFixtures(t *testing.T) map[string][]byte {
	t.Helper()
	p := s0ePacket(t)
	s0eOnce.Do(func() {
		s0eFiles = map[string][]byte{}
		for _, name := range []string{"FIXTURES.pack.json", "FIXTURES-ledger-boundary.pack.json"} {
			raw, err := os.ReadFile(filepath.Join(p, "inherited", "fixtures", name))
			if err != nil {
				s0eErr = err
				return
			}
			var pack struct {
				Records []struct {
					Path, Type, SHA256, Base64 string
				}
			}
			if err := json.Unmarshal(raw, &pack); err != nil {
				s0eErr = err
				return
			}
			for _, r := range pack.Records {
				if r.Type != "regular" {
					continue
				}
				b, err := base64.StdEncoding.DecodeString(r.Base64)
				if err != nil || shaHex(b) != r.SHA256 {
					s0eErr = fmt.Errorf("fixture %s does not match its digest", r.Path)
					return
				}
				s0eFiles[r.Path] = b
			}
		}
	})
	if s0eErr != nil {
		t.Fatal(s0eErr)
	}
	return s0eFiles
}

func s0eJSON(t *testing.T, name string, v any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(s0ePacket(t), name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatal(err)
	}
}

func s0eGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=/nonexistent-cem01-home", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "LC_ALL=C",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@invalid"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func s0eWrite(t *testing.T, p string, b []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(p)
	if err := os.WriteFile(p, b, mode); err != nil {
		t.Fatal(err)
	}
}

func s0eZlib(b []byte) []byte {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, _ = w.Write(b)
	_ = w.Close()
	return buf.Bytes()
}

// s0eObject returns the literal uncompressed object record for oid.
func s0eObject(t *testing.T, files map[string][]byte, format, oid string) []byte {
	t.Helper()
	b, ok := files["fixtures/git-objects/"+format+"/"+oid+".object"]
	if !ok {
		t.Fatalf("no literal object %s", oid)
	}
	return b
}

func s0eOID(t *testing.T, files map[string][]byte, format, prefix string) string {
	t.Helper()
	found := ""
	for p := range files {
		if strings.HasPrefix(p, "fixtures/git-objects/"+format+"/"+prefix) {
			if found != "" {
				t.Fatalf("ambiguous object prefix %s", prefix)
			}
			found = strings.TrimSuffix(path.Base(p), ".object")
		}
	}
	if found == "" {
		t.Fatalf("no object with prefix %s", prefix)
	}
	return found
}

func s0ePayload(t *testing.T, rec []byte) []byte {
	t.Helper()
	i := bytes.IndexByte(rec, 0)
	if i < 0 {
		t.Fatal("bad literal object record")
	}
	return rec[i+1:]
}

// s0eTreeEntry returns the oid of name in the root tree of commit.
func s0eTreeEntry(t *testing.T, files map[string][]byte, format, commit, name string) (tree, oid string) {
	t.Helper()
	oidLen := len(commit)
	tree, ok := stableCommitTree(s0ePayload(t, s0eObject(t, files, format, commit)), oidLen)
	if !ok {
		t.Fatal("bad commit")
	}
	recs, ok := stableParseTree(s0ePayload(t, s0eObject(t, files, format, tree)), oidLen)
	if !ok {
		t.Fatal("bad tree")
	}
	for _, r := range recs {
		if r.name == name {
			return tree, r.en.oid
		}
	}
	t.Fatalf("no %s in %s", name, tree)
	return "", ""
}

func s0eObjectPath(gitDir, oid string) string {
	return filepath.Join(gitDir, "objects", oid[:2], oid[2:])
}

// s0eRepo makes a fresh primary repository holding only the literal objects.
func s0eRepo(t *testing.T, files map[string][]byte, dir, format, head string) {
	t.Helper()
	args := []string{"-c", "init.defaultBranch=main", "init", "-q"}
	if format == "sha256" {
		args = append(args, "--object-format=sha256")
	}
	s0eGit(t, "/", append(args, dir)...)
	prefix := "fixtures/git-objects/" + format + "/"
	n := 0
	for p, b := range files {
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		oid := strings.TrimSuffix(path.Base(p), ".object")
		var sum string
		if format == "sha1" {
			h := sha1.Sum(b)
			sum = hex.EncodeToString(h[:])
		} else {
			h := sha256.Sum256(b)
			sum = hex.EncodeToString(h[:])
		}
		if sum != oid {
			t.Fatalf("literal object %s does not hash to its name", oid)
		}
		s0eWrite(t, s0eObjectPath(filepath.Join(dir, ".git"), oid), s0eZlib(b), 0o444)
		n++
	}
	if n == 0 {
		t.Fatalf("no %s literal objects", format)
	}
	s0eWrite(t, filepath.Join(dir, ".git", "refs", "heads", "main"), []byte(head+"\n"), 0o644)
}

func s0eTemp(t *testing.T) string {
	t.Helper()
	b, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func s0eArtifacts(t *testing.T, files map[string][]byte, dir string) {
	t.Helper()
	for p, b := range files {
		if strings.HasPrefix(p, "fixtures/artifacts/") {
			s0eWrite(t, filepath.Join(dir, strings.TrimPrefix(p, "fixtures/artifacts/")), b, 0o644)
		}
	}
}

// --- full-result runs ---------------------------------------------------------

type s0eRun struct {
	t         *testing.T
	files     map[string][]byte
	B, R      string
	repo      string
	target    string
	artifacts string
	raw       []byte
	hooks     *stableHooks
	ctx       context.Context
	cancel    context.CancelFunc
	git       string
	mode      string
	notes     []string
	after     []func() string
}

func newS0eRun(t *testing.T) *s0eRun {
	files := s0eFixtures(t)
	B := s0eTemp(t)
	R := filepath.Join(B, "repo")
	s0eRepo(t, files, R, "sha1", s0eSealed)
	arts := filepath.Join(B, "artifacts")
	s0eArtifacts(t, files, arts)
	ctx, cancel := context.WithTimeout(context.Background(), verificationBudget)
	t.Cleanup(cancel)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	return &s0eRun{t: t, files: files, B: B, R: R, repo: R, target: s0eSealed, artifacts: arts, raw: files[s0eSealedMap],
		hooks: &stableHooks{}, ctx: ctx, cancel: cancel, git: git, mode: "RUN_REAL"}
}

func (r *s0eRun) note(s string) { r.notes = append(r.notes, s) }

func (r *s0eRun) seam(s string) {
	r.mode = "SEAM_INJECTED"
	r.note(s)
}

func s0ePhase(p string) func(*stableTx) bool { return func(t *stableTx) bool { return t.Phase == p } }
func s0eOp(op string) func(*stableTx) bool {
	return func(t *stableTx) bool { return t.Operation == op }
}

// cancelOn cancels the caller context once, at point, on the first matching transaction.
func (r *s0eRun) cancelOn(point string, match func(*stableTx) bool) {
	var once sync.Once
	prev := r.hooks.tx
	r.hooks.tx = func(t *stableTx, p string) {
		if prev != nil {
			prev(t, p)
		}
		if p == point && match(t) {
			once.Do(r.cancel)
		}
	}
}

func (r *s0eRun) onEvent(name string, f func()) {
	prev := r.hooks.event
	r.hooks.event = func(n string) {
		if prev != nil {
			prev(n)
		}
		if n == name {
			f()
		}
	}
}

func (r *s0eRun) shim(name, body string) string {
	p := filepath.Join(r.B, "shims", name)
	s0eWrite(r.t, p, []byte("#!/bin/sh\n"+body+"\n"), 0o755)
	return p
}

func (r *s0eRun) gitFor(op, p string) {
	r.hooks.gitPath = func(t *stableTx) string {
		if t.Operation == op {
			return p
		}
		return ""
	}
}

func (r *s0eRun) opTimeoutFor(op string, d time.Duration) {
	r.hooks.opTimeout = func(t *stableTx) time.Duration {
		if t.Operation == op {
			return d
		}
		return 0
	}
}

// linked builds T-LINKED: W is a reciprocal linked worktree of R.
func (r *s0eRun) linked() (W, A, G string) {
	G = filepath.Join(r.R, ".git")
	W = filepath.Join(r.B, "wt")
	A = filepath.Join(G, "worktrees", "wt")
	s0eGit(r.t, r.R, "worktree", "add", "-q", "--detach", W, r.target)
	s0eWrite(r.t, filepath.Join(W, ".git"), []byte("gitdir: "+A+"\n"), 0o644)
	s0eWrite(r.t, filepath.Join(A, "gitdir"), []byte(W+"/.git\n"), 0o644)
	s0eWrite(r.t, filepath.Join(A, "commondir"), []byte("../..\n"), 0o644)
	if _, err := os.Lstat(filepath.Join(A, "info", "attributes")); err == nil {
		r.t.Fatal("T-LINKED admin must not have info/attributes")
	}
	r.repo = W
	return W, A, G
}

func (r *s0eRun) replaceObject(oid string, b []byte) {
	s0eWrite(r.t, s0eObjectPath(filepath.Join(r.R, ".git"), oid), b, 0o444)
}

func (r *s0eRun) removeObject(oid string) {
	if err := os.Remove(s0eObjectPath(filepath.Join(r.R, ".git"), oid)); err != nil {
		r.t.Fatal(err)
	}
}

func (r *s0eRun) packetMap(name string) []byte {
	b, err := os.ReadFile(filepath.Join(s0ePacket(r.t), name))
	if err != nil {
		r.t.Fatal(err)
	}
	return b
}

// lingerCheck records a descendant pid written by a shim and asserts it is gone.
func (r *s0eRun) lingerCheck(pidFile string) {
	r.after = append(r.after, func() string {
		b, err := os.ReadFile(pidFile)
		if err != nil {
			return "descendant pid not recorded"
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
		if err != nil {
			return "descendant pid unreadable"
		}
		p, _ := os.FindProcess(pid)
		if p != nil && p.Signal(syscall.Signal(0)) == nil {
			return fmt.Sprintf("descendant %d still present", pid)
		}
		return ""
	})
}

var s0eSetups = map[int]func(r *s0eRun){
	1: func(r *s0eRun) { r.cancel(); r.note("caller context cancelled before the call") },
	2: func(r *s0eRun) { r.cancelOn("spawned", s0ePhase("target")) },
	3: func(r *s0eRun) { r.cancelOn("spawned", s0ePhase("base-sidecar")) },
	4: func(r *s0eRun) { r.cancelOn("spawned", s0ePhase("initial-target-sidecar")) },
	5: func(r *s0eRun) { r.cancelOn("spawned", s0ePhase("target-sidecar")) },
	6: func(r *s0eRun) { r.cancelOn("spawned", s0eOp("canonical-diff-child")) },
	7: func(r *s0eRun) {
		r.cancelOn("spawned", func(t *stableTx) bool { return strings.HasPrefix(t.Phase, "evidence:") })
	},
	8: func(r *s0eRun) { r.cancelOn("spawned", s0ePhase("drift-target")) },
	9: func(r *s0eRun) { r.onEvent("before-artifacts", r.cancel) },
	10: func(r *s0eRun) {
		r.onEvent("after-artifacts", func() {
			if err := os.Rename(r.R, r.R+".replaced"); err != nil {
				r.t.Error(err)
			}
			if err := os.Mkdir(r.R, 0o755); err != nil {
				r.t.Error(err)
			}
		})
		r.note("root renamed and recreated as an ordinary directory (G11)")
	},
	11: func(r *s0eRun) { r.onEvent("before-final-checkpoint", r.cancel) },
	12: func(r *s0eRun) {
		r.seam("ownerFail(establish) on transaction 1")
		r.hooks.ownerFail = func(t *stableTx, step string) bool { return step == "establish" && t.Ordinal == 1 }
	},
	13: func(r *s0eRun) {
		r.seam("holdKeeper on transaction 1 + closeFail at final owner close (G03)")
		r.hooks.holdKeeper = true
		r.hooks.closeFail = func() bool { return true }
	},
	14: func(r *s0eRun) {
		pid := filepath.Join(r.B, "linger.pid")
		r.gitFor("canonical-diff-child", r.shim("linger", "/bin/sleep 300 &\necho $! > "+pid+"\nexec "+r.git+` "$@"`))
		r.lingerCheck(pid)
		r.note("trusted diff shim: real git output, exit 0, same-group sleep descendant")
	},
	15: func(r *s0eRun) {
		r.gitFor("canonical-diff-child", r.shim("slow", "exec /bin/sleep 30"))
		r.opTimeoutFor("canonical-diff-child", 300*time.Millisecond)
		r.note("per-op deadline shortened to 300ms for the diff; caller clock not controlled")
	},
	16: func(r *s0eRun) {
		r.gitFor("canonical-diff-child", r.shim("big", "/usr/bin/head -c 9000000 /dev/zero"))
	},
	17: func(r *s0eRun) { r.gitFor("canonical-diff-child", r.shim("fail", "exit 1")) },
	18: func(r *s0eRun) {
		r.gitFor("canonical-diff-child", r.shim("big", "/usr/bin/head -c 9000000 /dev/zero"))
		r.cancelOn("before-commit", s0eOp("canonical-diff-child"))
	},
	19: func(r *s0eRun) {
		r.gitFor("canonical-diff-child", r.shim("bigfail", "/usr/bin/head -c 9000000 /dev/zero\nexit 1"))
	},
	20: func(r *s0eRun) {
		r.seam("ownerFail(retire) on the diff + caller cancel at spawn")
		r.cancelOn("spawned", s0eOp("canonical-diff-child"))
		r.hooks.ownerFail = func(t *stableTx, step string) bool {
			return step == "retire" && t.Operation == "canonical-diff-child"
		}
	},
	21: func(r *s0eRun) { r.target = s0eSidecarless; r.raw = r.packetMap("sha1-nonstructural-sidecarless.json") },
	22: func(r *s0eRun) {
		r.target = s0eSidecarless
		r.raw = r.files["fixtures/maps/sha1-positive-sidecarless.json"]
	},
	23: func(r *s0eRun) {
		p := filepath.Join(r.R, ".git", "info", "attributes")
		s0eWrite(r.t, p, []byte("# inert\n"), 0o644)
		if err := os.Chmod(p, 0); err != nil {
			r.t.Fatal(err)
		}
		r.t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
		r.note("real EACCES: chmod 000 on an inert common info/attributes (G06)")
	},
	24: func(r *s0eRun) {
		info := filepath.Join(r.R, ".git", "objects", "info")
		if err := os.Rename(info, info+"-real"); err != nil {
			r.t.Fatal(err)
		}
		if err := os.Symlink("info-real", info); err != nil {
			r.t.Fatal(err)
		}
		r.note("real ELOOP: objects/info is a symlink (G06)")
	},
	25: func(r *s0eRun) {
		_, A, _ := r.linked()
		r.seam("fs seam returns EIO at the first probe of A/commondir")
		r.hooks.fs = func(point, p string) error {
			if point == "open" && p == filepath.Join(A, "commondir") {
				return syscall.EIO
			}
			return nil
		}
	},
	26: func(r *s0eRun) {
		pack := filepath.Join(r.R, ".git", "objects", "pack")
		var once sync.Once
		r.hooks.fs = func(point, p string) error {
			if point == "opened" && p == pack {
				once.Do(func() {
					if err := os.Rename(pack, pack+"-old"); err != nil {
						r.t.Error(err)
					}
					if err := os.Mkdir(pack, 0o755); err != nil {
						r.t.Error(err)
					}
				})
			}
			return nil
		}
		r.note("objects/pack really renamed and recreated after open, synchronized by the fs seam")
	},
	27: func(r *s0eRun) {
		g := filepath.Join(r.R, ".git")
		if err := os.Rename(g, g+"-real"); err != nil {
			r.t.Fatal(err)
		}
		if err := os.Symlink(".git-real", g); err != nil {
			r.t.Fatal(err)
		}
	},
	28: func(r *s0eRun) {
		_, A, _ := r.linked()
		s0eWrite(r.t, filepath.Join(A, "commondir"), nil, 0o644)
	},
	29: func(r *s0eRun) {
		_, A, G := r.linked()
		s0eWrite(r.t, filepath.Join(A, "commondir"), []byte(G+"\n"), 0o644)
	},
	30: func(r *s0eRun) {
		W, A, _ := r.linked()
		s0eWrite(r.t, filepath.Join(W, ".git"), []byte("gitdir: "+A+"\r\n"), 0o644)
	},
	31: func(r *s0eRun) {
		W, A, _ := r.linked()
		s0eWrite(r.t, filepath.Join(W, ".git"), []byte("gitdir: "+A+"\n\n"), 0o644)
	},
	32: func(r *s0eRun) {
		W, A, _ := r.linked()
		s0eWrite(r.t, filepath.Join(W, ".git"), []byte("gitdir: "+A+"/\xff/..\n"), 0o644)
	},
	33: func(r *s0eRun) {
		W, A, _ := r.linked()
		n := 4088 - len(A)
		if n < 0 {
			r.t.Fatal("A too long for metadata-over4096")
		}
		q := strings.Repeat("/.", n/2)
		if n%2 == 1 {
			q = "/" + strings.Repeat("/.", (n-1)/2)
		}
		b := []byte("gitdir: " + A + q + "\n")
		if len(b) != 4097 || filepath.Clean(A+q) != A {
			r.t.Fatalf("metadata-over4096 construction is %d bytes", len(b))
		}
		s0eWrite(r.t, filepath.Join(W, ".git"), b, 0o644)
	},
	34: func(r *s0eRun) {
		s0eWrite(r.t, filepath.Join(r.R, ".git", "objects", "info", "http-alternates"), nil, 0o644)
	},
	35: func(r *s0eRun) { r.removeObject(s0eOID(r.t, r.files, "sha1", "632d2d79")) },
	36: func(r *s0eRun) {
		r.replaceObject(s0eOID(r.t, r.files, "sha1", "632d2d79"), []byte("corrupt\x00not zlib"))
	},
	37: func(r *s0eRun) { r.wrongHash(s0eOID(r.t, r.files, "sha1", "632d2d79")) },
	38: func(r *s0eRun) { r.launchFailure("target-sidecar") },
	39: func(r *s0eRun) { r.removeObject(s0eOID(r.t, r.files, "sha1", "36e5ee3e")) },
	40: func(r *s0eRun) {
		r.replaceObject(s0eOID(r.t, r.files, "sha1", "36e5ee3e"), []byte("corrupt\x00not zlib"))
	},
	41: func(r *s0eRun) { r.wrongHash(s0eOID(r.t, r.files, "sha1", "36e5ee3e")) },
	42: func(r *s0eRun) { r.launchFailure("evidence:") },
	43: func(r *s0eRun) { r.cancelOn("spawned", s0ePhase("repeated-target-sidecar")) },
	44: func(r *s0eRun) {
		r.gitFor("canonical-root-pair", r.shim("badheader", "printf 'bad header\\n'\nexit 1"))
	},
	45: func(r *s0eRun) {
		r.gitFor("canonical-root-pair", r.shim("overflow", "printf '"+s0eBase+" commit 4194304\\n'\nexec /usr/bin/head -c 5000000 /dev/zero"))
		r.opTimeoutFor("canonical-root-pair", 150*time.Millisecond)
		r.hooks.tx = func(t *stableTx, p string) {
			if p == "before-commit" && t.Operation == "canonical-root-pair" {
				time.Sleep(500 * time.Millisecond)
			}
		}
		r.note("valid batch header then bytes past the 4MiB limit; per-op deadline 150ms elapses before arbitration")
	},
	46: func(r *s0eRun) {
		r.seam("root-pair limit lowered to 100 bytes over real git output + cause seam nonzero")
		r.hooks.limit = func(t *stableTx) int64 {
			if t.Operation == "canonical-root-pair" {
				return 100
			}
			return 0
		}
		r.hooks.cause = func(t *stableTx) string {
			if t.Operation == "canonical-root-pair" {
				return "nonzero"
			}
			return ""
		}
	},
	47: func(r *s0eRun) {
		r.gitFor("canonical-root-pair", r.shim("badbig", "printf 'bad header\\n'\nexec /usr/bin/head -c 5000000 /dev/zero"))
	},
	48: func(r *s0eRun) {
		r.seam("caller cancel substituted for the controlled outer clock")
		r.gitFor("canonical-diff-child", r.shim("block", "exec /bin/sleep 300"))
		r.cancelOn("spawned", s0eOp("canonical-diff-child"))
	},
	50: func(r *s0eRun) {
		r.seam("caller cancel substituted for the controlled outer clock; holdKeeper keeps one session keeper until close")
		r.hooks.holdKeeper = true
		r.onEvent("close-start", r.cancel)
	},
	52: func(r *s0eRun) { r.target = s0eLedgerBoundary; r.raw = r.packetMap("sha1-ledger-336-evidence.json") },
	53: func(r *s0eRun) { r.target = s0eLedgerBoundary; r.raw = r.packetMap("sha1-ledger-337-evidence.json") },
	54: func(r *s0eRun) {
		r.target = s0eLedgerBoundary
		r.raw = r.packetMap("sha1-ledger-337-evidence.json")
		n := 0
		r.hooks.tx = func(t *stableTx, p string) {
			if p == "spawned" && strings.HasPrefix(t.Phase, "drift:") {
				n++
				if n == 2 {
					r.cancel()
				}
			}
		}
	},
	55: func(r *s0eRun) {
		r.seam("per-op deadline 200ms on a blocking diff shim; caller cancel after the git-timeout commitment")
		r.gitFor("canonical-diff-child", r.shim("block", "exec /bin/sleep 30"))
		r.opTimeoutFor("canonical-diff-child", 200*time.Millisecond)
		r.cancelOn("committed", s0eOp("canonical-diff-child"))
	},
	56: func(r *s0eRun) {
		r.seam("per-op deadline 200ms on a blocking diff shim; caller cancel before arbitration")
		r.gitFor("canonical-diff-child", r.shim("block", "exec /bin/sleep 30"))
		r.opTimeoutFor("canonical-diff-child", 200*time.Millisecond)
		r.cancelOn("before-commit", s0eOp("canonical-diff-child"))
	},
}

// Cases 49 and 51 extend 48 and 50, so they are registered after initialization.
func init() {
	s0eSetups[49] = func(r *s0eRun) {
		s0eSetups[48](r)
		r.note("ownerFail(retire) forces unobserved cleanup")
		r.hooks.ownerFail = func(t *stableTx, step string) bool {
			return step == "retire" && t.Operation == "canonical-diff-child"
		}
	}
	s0eSetups[51] = func(r *s0eRun) {
		s0eSetups[50](r)
		r.note("closeFail forces unobserved close")
		r.hooks.closeFail = func() bool { return true }
	}
}

func (r *s0eRun) wrongHash(oid string) {
	_, other := s0eTreeEntry(r.t, r.files, "sha1", s0eSealed, "app.txt")
	r.replaceObject(oid, s0eZlib(s0eObject(r.t, r.files, "sha1", other)))
	r.note("valid compressed bytes of target app.txt stored under the requested oid")
}

func (r *s0eRun) launchFailure(phase string) {
	missing := filepath.Join(r.B, "missing", "git")
	r.hooks.gitPath = func(t *stableTx) string {
		if t.Operation == "normal-blob" && strings.HasPrefix(t.Phase, phase) {
			return missing
		}
		return ""
	}
	r.note("keeper exec of a nonexistent git path")
}

type s0eFullCase struct {
	ID             string         `json:"id"`
	ExpectedExit   int            `json:"expectedExit"`
	ExpectedResult map[string]any `json:"expectedResult"`
	InputFixture   string         `json:"inputFixture"`
}

func TestStableCanonicalFullResults(t *testing.T) {
	var doc struct {
		Cases []s0eFullCase `json:"cases"`
	}
	s0eJSON(t, "FULL-RESULT-CASES.json", &doc)
	if len(doc.Cases) != 56 {
		t.Fatalf("expected 56 cases, got %d", len(doc.Cases))
	}
	go124 := strings.HasPrefix(runtime.Version(), "go1.24.")
	for i, c := range doc.Cases {
		id := i + 1
		t.Run(fmt.Sprintf("%02d-%s", id, c.ID), func(t *testing.T) {
			if (id == 21 || id == 22) != go124 {
				t.Skipf("case %d runs only under %s", id, map[bool]string{true: "go1.24.13", false: "go1.27.1"}[id == 21 || id == 22])
			}
			if !stableOwnerSupported() {
				t.Skip("process containment is not qualified on " + runtime.GOOS)
			}
			r := newS0eRun(t)
			s0eSetups[id](r)
			got, code := runStableCanonical(r.ctx, r.repo, r.raw, s0eBase, r.target, r.artifacts, r.hooks)
			rec := s0eRecord{ID: id, Name: c.ID, Mode: r.mode, ExpectedExit: c.ExpectedExit, ActualExit: code, Note: strings.Join(r.notes, "; ")}
			rec.FirstDifferingKey = s0eCompare(got, code, c.ExpectedResult, c.ExpectedExit)
			for _, f := range r.after {
				if msg := f(); msg != "" {
					rec.FirstDifferingKey = "postcondition"
					rec.Note = strings.TrimPrefix(rec.Note+"; "+msg, "; ")
				}
			}
			rec.Status = "PASS"
			if rec.FirstDifferingKey != "" {
				rec.Status = "FAIL"
				b, _ := json.Marshal(got)
				t.Errorf("case %d %s: first differing key %q (exit %d want %d)\n%s", id, c.ID, rec.FirstDifferingKey, code, c.ExpectedExit, b)
			}
			s0eRecordAdd(&s0eConformance.FullResults, rec)
		})
	}
}

// TestStableCanonicalLinkedControl proves T-LINKED itself is admitted and accepted.
func TestStableCanonicalLinkedControl(t *testing.T) {
	if !stableOwnerSupported() {
		t.Skip("process containment is not qualified on " + runtime.GOOS)
	}
	r := newS0eRun(t)
	r.linked()
	if _, e := stableAdmit(r.repo, nil); e != nil {
		t.Fatalf("T-LINKED admission: %s", e.code)
	}
	got, code := runStableCanonical(r.ctx, r.repo, r.raw, s0eBase, r.target, r.artifacts, nil)
	rec := s0eRecord{Name: "T-LINKED-control", Mode: "RUN_REAL", ExpectedExit: 0, ActualExit: code, Status: "PASS"}
	if code != 0 || got.Outcome != "ACCEPT" || got.Sidecar != "EXACT" {
		rec.Status, rec.FirstDifferingKey = "FAIL", "outcome"
		t.Errorf("T-LINKED control: exit %d outcome %s", code, got.Outcome)
	}
	s0eRecordAdd(&s0eConformance.Topology, rec)
}

func TestStableCanonicalProofBlobsDoNotChargeNormalAggregate(t *testing.T) {
	if !stableOwnerSupported() {
		t.Skip("process containment is not qualified on " + runtime.GOOS)
	}
	r := newS0eRun(t)
	adm, e := stableAdmit(r.repo, r.hooks)
	if e != nil {
		t.Fatalf("admit: %s", e.code)
	}
	r.hooks.aggregate = func(*stableSession) int64 { return 1 }
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	s := &stableSession{ctx: r.ctx, hooks: r.hooks, exe: exe, git: r.git, admin: adm.admin, env: stableGitEnv(), oidLen: len(s0eBase), charged: map[string]bool{}}
	defer stableClose(s)
	baseTree, _, e := s.resolve("expected-base", s0eBase)
	if e != nil {
		t.Fatalf("base resolve: %s", e.code)
	}
	targetTree, _, e := s.resolve("target", r.target)
	if e != nil {
		t.Fatalf("target resolve: %s", e.code)
	}
	canon, e := s.canonical(s0eBase, r.target, baseTree, targetTree)
	if e != nil {
		t.Fatalf("canonical proof blobs must not charge the normal aggregate: %s", e.code)
	}
	if len(canon.proof) == 0 {
		t.Fatal("canonical proof read no blobs")
	}
	if s.bytes != 0 || len(s.charged) != 0 {
		t.Fatalf("proof bytes charged normal aggregate: bytes=%d charged=%d", s.bytes, len(s.charged))
	}
}

// --- ledgers ------------------------------------------------------------------

func TestStableCanonicalLedgers(t *testing.T) {
	var doc struct {
		Ledgers []struct {
			ID                  string           `json:"id"`
			Rows                []map[string]any `json:"rows"`
			Admitted            *int             `json:"admitted"`
			FirstRefusedOrdinal *int             `json:"firstRefusedOrdinal"`
		} `json:"ledgers"`
	}
	s0eJSON(t, "LOGICAL-LEDGERS.json", &doc)
	for _, l := range doc.Ledgers {
		t.Run(l.ID, func(t *testing.T) {
			if !stableOwnerSupported() {
				t.Skip("process containment is not qualified on " + runtime.GOOS)
			}
			r := newS0eRun(t)
			switch l.ID {
			case "positive-sidecar-absent":
				r.target, r.raw = s0eSidecarless, r.files["fixtures/maps/sha1-positive-sidecarless.json"]
			case "positive-sidecar-present-and-changed-level":
			case "boundary-336":
				r.target, r.raw = s0eLedgerBoundary, r.packetMap("sha1-ledger-336-evidence.json")
			case "boundary-337":
				r.target, r.raw = s0eLedgerBoundary, r.packetMap("sha1-ledger-337-evidence.json")
			default:
				t.Fatalf("unknown ledger %s", l.ID)
			}
			rows := []stableLedgerRow{}
			r.hooks.ledger = func(row stableLedgerRow) { rows = append(rows, row) }
			_, code := runStableCanonical(r.ctx, r.repo, r.raw, s0eBase, r.target, r.artifacts, r.hooks)
			want := l.Rows
			note := fmt.Sprintf("%d rows compared", len(want))
			if l.FirstRefusedOrdinal != nil {
				want = want[:*l.FirstRefusedOrdinal]
				note = fmt.Sprintf("rows 1..%d compared (row %d is the refused reservation; later rows are would-have-been calls)", len(want), len(want))
			}
			b, _ := json.Marshal(rows)
			var got []map[string]any
			_ = json.Unmarshal(b, &got)
			rec := s0eRecord{Name: l.ID, Mode: "RUN_REAL", ActualExit: code, Status: "PASS", Note: note}
			if l.FirstRefusedOrdinal != nil {
				rec.ExpectedExit = 2
			}
			if len(got) != len(want) {
				rec.Status, rec.FirstDifferingKey = "FAIL", fmt.Sprintf("row-count %d want %d", len(got), len(want))
			} else {
				for i := range want {
					if !reflect.DeepEqual(got[i], want[i]) {
						rec.Status, rec.FirstDifferingKey = "FAIL", fmt.Sprintf("row %d", i+1)
						break
					}
				}
			}
			if code != rec.ExpectedExit && rec.Status == "PASS" {
				rec.Status, rec.FirstDifferingKey = "FAIL", "exit"
			}
			if rec.Status != "PASS" {
				t.Errorf("ledger %s: %s", l.ID, rec.FirstDifferingKey)
			}
			s0eRecordAdd(&s0eConformance.Ledgers, rec)
		})
	}
}

// --- envelope cells (CLI subprocess) -----------------------------------------

type s0eCell struct {
	ID             string         `json:"id"`
	ObjectFormat   string         `json:"objectFormat"`
	Map            string         `json:"map"`
	ExpectedBase   string         `json:"expectedBase"`
	Target         string         `json:"target"`
	ExpectedExit   int            `json:"expectedExit"`
	ExpectedResult map[string]any `json:"expectedResult"`
}

func s0eSentinel(t *testing.T, B string) (string, string) {
	fired := filepath.Join(B, "sentinel-fired")
	p := filepath.Join(B, "sentinel")
	s0eWrite(t, p, []byte("#!/bin/sh\necho \"$0 $*\" >> "+fired+"\nexit 1\n"), 0o755)
	return p, fired
}

func TestStableCanonicalEnvelopeCells(t *testing.T) {
	var doc struct {
		Cases []s0eCell `json:"cases"`
	}
	s0eJSON(t, filepath.Join("inherited", "s0e", "REPOSITORY-ENVELOPE.cells.json"), &doc)
	if len(doc.Cases) != 29 {
		t.Fatalf("expected 29 cells, got %d", len(doc.Cases))
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range doc.Cases {
		t.Run(c.ID, func(t *testing.T) {
			if !stableOwnerSupported() {
				t.Skip("process containment is not qualified on " + runtime.GOOS)
			}
			files := s0eFixtures(t)
			B := s0eTemp(t)
			R := filepath.Join(B, "repo")
			s0eRepo(t, files, R, c.ObjectFormat, c.Target)
			G := filepath.Join(R, ".git")
			arts := filepath.Join(B, "artifacts")
			s0eArtifacts(t, files, arts)
			mapPath := filepath.Join(B, "map.json")
			s0eWrite(t, mapPath, files["fixtures/"+c.Map], 0o644)
			repo := R
			env := []string{}
			note := ""
			sentinel, fired := s0eSentinel(t, B)
			mustRename := func(a, b string) {
				if err := os.Rename(a, b); err != nil {
					t.Fatal(err)
				}
			}
			mustSymlink := func(a, b string) {
				if err := os.Symlink(a, b); err != nil {
					t.Fatal(err)
				}
			}
			worktree := func(extra ...string) (string, string) {
				W := filepath.Join(B, "wt")
				s0eGit(t, R, append(append([]string{"worktree", "add", "-q"}, extra...), "--detach", W, c.Target)...)
				return W, filepath.Join(G, "worktrees", "wt")
			}
			switch c.ID {
			case "ordinary-sha1", "ordinary-sha256":
			case "origin-sha1", "origin-sha256":
				s0eGit(t, R, "config", "remote.origin.url", "/nonexistent/origin.git")
				s0eGit(t, R, "config", "user.name", "Inert")
				s0eGit(t, R, "config", "branch.main.remote", "origin")
			case "linked-sha1", "linked-sha256":
				repo, _ = worktree()
			case "relative-sha1", "relative-sha256":
				repo, _ = worktree("--relative-paths")
				b, _ := os.ReadFile(filepath.Join(repo, ".git"))
				if filepath.IsAbs(strings.TrimSpace(strings.TrimPrefix(string(b), "gitdir: "))) {
					t.Fatal("worktree marker is not relative")
				}
			case "attributes":
				s0eWrite(t, filepath.Join(G, "info", "attributes"), []byte("*.go diff=evil\n"), 0o644)
			case "comments":
				s0eWrite(t, filepath.Join(G, "info", "attributes"), []byte("# comment only\n"), 0o644)
			case "alternates":
				s0eWrite(t, filepath.Join(G, "objects", "info", "alternates"), []byte("/no/objects\n"), 0o644)
			case "bad-marker":
				mustRename(G, G+"-moved")
				s0eWrite(t, G, []byte("not a gitfile\n"), 0o644)
			case "object-link":
				mustRename(filepath.Join(G, "objects"), filepath.Join(G, "objects-real"))
				mustSymlink("objects-real", filepath.Join(G, "objects"))
			case "root-link":
				repo = filepath.Join(B, "link")
				mustSymlink(R, repo)
			case "shallow-complete", "shallow-missing":
				depth := map[string]string{"shallow-complete": "3", "shallow-missing": "1"}[c.ID]
				repo = filepath.Join(B, "clone")
				s0eGit(t, B, "clone", "-q", "--depth="+depth, "--branch=main", "file://"+R, repo)
			case "missing-base":
				if err := os.Remove(s0eObjectPath(G, c.ExpectedBase)); err != nil {
					t.Fatal(err)
				}
			case "corrupt-base":
				s0eWrite(t, s0eObjectPath(G, c.ExpectedBase), []byte("corrupt\x00not zlib"), 0o444)
			case "hostile":
				for _, kv := range [][2]string{
					{"diff.external", sentinel}, {"diff.evil.command", sentinel}, {"diff.evil.textconv", sentinel},
					{"filter.evil.clean", sentinel}, {"filter.evil.smudge", sentinel}, {"core.fsmonitor", sentinel},
					{"core.hooksPath", filepath.Join(B, "hooks")}, {"credential.helper", "!" + sentinel},
					{"core.repositoryformatversion", "1"}, {"extensions.partialClone", "origin"},
					{"remote.origin.url", "ext::" + sentinel + " %S"}, {"remote.origin.promisor", "true"},
				} {
					s0eGit(t, R, "config", kv[0], kv[1])
				}
				for _, h := range []string{"pre-commit", "post-checkout", "reference-transaction", "fsmonitor-watchman"} {
					s0eWrite(t, filepath.Join(B, "hooks", h), []byte("#!/bin/sh\nexec "+sentinel+" hook\n"), 0o755)
				}
				ambient := filepath.Join(B, "ambient.gitconfig")
				s0eWrite(t, ambient, []byte("[diff]\n\texternal = "+sentinel+"\n[core]\n\tfsmonitor = "+sentinel+"\n\tattributesFile = "+filepath.Join(B, "ambient.attributes")+"\n"), 0o644)
				s0eWrite(t, filepath.Join(B, "ambient.attributes"), []byte("* diff=evil\n"), 0o644)
				s0eWrite(t, filepath.Join(B, "home", ".gitconfig"), []byte("[diff]\n\texternal = "+sentinel+"\n"), 0o644)
				env = append(env, "GIT_CONFIG_GLOBAL="+ambient, "GIT_CONFIG_SYSTEM="+ambient, "GIT_CONFIG_NOSYSTEM=",
					"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=diff.external", "GIT_CONFIG_VALUE_0="+sentinel,
					"GIT_DIR=/trap/git-dir", "GIT_WORK_TREE=/trap/work-tree", "GIT_OBJECT_DIRECTORY=/trap/objects",
					"GIT_EXTERNAL_DIFF="+sentinel, "GIT_REPLACE_REF_BASE=refs/trap/", "GIT_GRAFT_FILE=/trap/graft",
					"XDG_CONFIG_HOME="+filepath.Join(B, "home"))
				note = "sentinel must not execute"
			case "ambient-alternates":
				env = append(env, "GIT_ALTERNATE_OBJECT_DIRECTORIES=/no/alternate")
			case "linked-wrong-backpointer":
				var A string
				repo, A = worktree()
				s0eWrite(t, filepath.Join(A, "gitdir"), []byte("/not/reciprocal/.git\n"), 0o644)
			case "linked-missing-common":
				var A string
				repo, A = worktree()
				s0eWrite(t, filepath.Join(A, "commondir"), []byte("/missing/common\n"), 0o644)
			case "linked-attributes":
				var A string
				repo, A = worktree()
				s0eWrite(t, filepath.Join(A, "info", "attributes"), []byte("* diff=evil\n"), 0o644)
			case "pack-link":
				p := filepath.Join(G, "objects", "pack")
				mustRename(p, p+"-real")
				mustSymlink("pack-real", p)
			case "info-link":
				p := filepath.Join(G, "objects", "info")
				mustRename(p, p+"-real")
				mustSymlink("info-real", p)
			case "missing-promisor":
				if err := os.Remove(s0eObjectPath(G, c.ExpectedBase)); err != nil {
					t.Fatal(err)
				}
				for _, kv := range [][2]string{{"core.repositoryformatversion", "1"}, {"extensions.partialClone", "origin"},
					{"remote.origin.url", "ext::" + sentinel + " %S"}, {"remote.origin.promisor", "true"}} {
					s0eGit(t, R, "config", kv[0], kv[1])
				}
				note = "sentinel must not execute"
			case "missing-target-tree":
				tree, _ := s0eTreeEntry(t, files, c.ObjectFormat, c.Target, "app.txt")
				if err := os.Remove(s0eObjectPath(G, tree)); err != nil {
					t.Fatal(err)
				}
				note = "target root tree deleted"
			case "missing-target-blob":
				_, blob := s0eTreeEntry(t, files, c.ObjectFormat, c.Target, "app.txt")
				if err := os.Remove(s0eObjectPath(G, blob)); err != nil {
					t.Fatal(err)
				}
				note = "target app.txt blob deleted"
			case "corrupt-target-blob":
				_, blob := s0eTreeEntry(t, files, c.ObjectFormat, c.Target, "app.txt")
				s0eWrite(t, s0eObjectPath(G, blob), []byte("corrupt\x00not zlib"), 0o444)
				note = "target app.txt blob replaced by invalid compressed bytes"
			default:
				t.Fatalf("no recipe for cell %s", c.ID)
			}
			cmd := exec.Command(exe, "verify-stable-canonical", "--repository", repo, "--map", mapPath,
				"--expected-base", c.ExpectedBase, "--target", c.Target, "--artifacts", arts)
			cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + filepath.Join(B, "home"), "TMPDIR=" + os.Getenv("TMPDIR"), "CEM01_TEST_CLI=1"}, env...)
			var stdout bytes.Buffer
			cmd.Stdout = &stdout
			err := cmd.Run()
			code := 0
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				code = ee.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			var got stableResult
			rec := s0eRecord{Name: c.ID, Mode: "RUN_REAL", ExpectedExit: c.ExpectedExit, ActualExit: code, Note: note}
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
				rec.FirstDifferingKey = "stdout-json"
			} else {
				rec.FirstDifferingKey = s0eCompare(got, code, c.ExpectedResult, c.ExpectedExit)
			}
			if b, err := os.ReadFile(fired); err == nil {
				rec.FirstDifferingKey = "sentinel-executed"
				rec.Note += "; " + strings.TrimSpace(string(b))
			}
			rec.Status = "PASS"
			if rec.FirstDifferingKey != "" {
				rec.Status = "FAIL"
				t.Errorf("cell %s: first differing key %q (exit %d want %d)\n%s", c.ID, rec.FirstDifferingKey, code, c.ExpectedExit, stdout.Bytes())
			}
			s0eRecordAdd(&s0eConformance.Cells, rec)
		})
	}
}
