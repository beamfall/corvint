package verify

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/patch"
)

// s0ePublicRootEnv names the directory of the public S0E repair packet
// (FULL-RESULT-CASES.json and its inherited fixture pack). When unset, tests
// use the in-repository packet path below.
const s0ePublicRootEnv = "CEM_S0E_PUBLIC_ROOT"

const s0ePublicRootDefault = "protocol/cem-1.0/stable/repository-envelope-packet"

type s0eCase struct {
	ID             string         `json:"id"`
	Status         string         `json:"status"`
	ExpectedExit   int            `json:"expectedExit"`
	ExpectedResult map[string]any `json:"expectedResult"`
}

// s0ePacket is the public packet: its cases, its fixture records by path and
// its candidate maps by SHA-256.
type s0ePacket struct {
	root    string
	cases   []s0eCase
	records map[string][]byte
	maps    map[string][]byte
}

func loadS0EPacket(t *testing.T) *s0ePacket {
	t.Helper()
	root := os.Getenv(s0ePublicRootEnv)
	if root == "" {
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("public packet: cannot resolve test file path")
		}
		root = filepath.Join(filepath.Dir(file), "..", "..", "..", s0ePublicRootDefault)
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			t.Fatalf("public packet: %s is unset and default packet directory %s is unavailable", s0ePublicRootEnv, root)
		}
	}
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("public packet: %v", err)
		}
		return data
	}
	packet := &s0ePacket{root: root, records: map[string][]byte{}, maps: map[string][]byte{}}
	var cases struct {
		Cases []s0eCase `json:"cases"`
	}
	if err := json.Unmarshal(read("FULL-RESULT-CASES.json"), &cases); err != nil {
		t.Fatal(err)
	}
	packet.cases = cases.Cases
	for _, packName := range []string{
		"inherited/fixtures/FIXTURES.pack.json",
		"inherited/fixtures/FIXTURES-ledger-boundary.pack.json",
	} {
		var pack struct {
			Records []struct {
				Path, Type, Sha256, Base64 string
			} `json:"records"`
		}
		if err := json.Unmarshal(read(packName), &pack); err != nil {
			t.Fatal(err)
		}
		for _, record := range pack.Records {
			if record.Type != "regular" {
				continue
			}
			data, err := base64.StdEncoding.DecodeString(record.Base64)
			if err != nil || stableDigest(data) != record.Sha256 {
				t.Fatalf("fixture record %s does not match its digest", record.Path)
			}
			packet.records[record.Path] = data
		}
	}
	for _, name := range []string{
		"inherited/fixtures/maps/sha1-positive-sealed.json", "inherited/fixtures/maps/sha1-positive-sidecarless.json",
		"sha1-ledger-336-evidence.json", "sha1-ledger-337-evidence.json", "sha1-nonstructural-sidecarless.json",
	} {
		data := read(name)
		packet.maps[stableDigest(data)] = data
	}
	return packet
}

func (p *s0ePacket) find(t *testing.T, id string) s0eCase {
	t.Helper()
	for _, c := range p.cases {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("public packet has no case %s", id)
	return s0eCase{}
}

// materialize builds the original ordinary-sha1 literal repository under base
// from the packet's object records and returns R and the artifact root.
func (p *s0ePacket) materialize(t *testing.T, base string) (repo, artifacts string) {
	t.Helper()
	repo, artifacts = filepath.Join(base, "repo"), filepath.Join(base, "artifacts")
	command := exec.Command("git", "init", "-q", "--object-format=sha1", repo)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if err := os.MkdirAll(artifacts, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, data := range p.records {
		switch {
		case strings.HasPrefix(path, "fixtures/git-objects/sha1/"):
			oid := strings.TrimSuffix(filepath.Base(path), ".object")
			if sum := sha1.Sum(data); hex.EncodeToString(sum[:]) != oid {
				t.Fatalf("object record %s is not the raw object of its name", path)
			}
			writeLooseObject(t, filepath.Join(repo, ".git"), oid, data)
		case strings.HasPrefix(path, "fixtures/artifacts/"):
			if err := os.WriteFile(filepath.Join(artifacts, filepath.Base(path)), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return repo, artifacts
}

func looseObjectPath(gitDir, oid string) string {
	return filepath.Join(gitDir, "objects", oid[:2], oid[2:])
}

// writeLooseObject replaces the loose object file of oid with the zlib
// stream of raw, which need not hash to oid.
func writeLooseObject(t *testing.T, gitDir, oid string, raw []byte) {
	t.Helper()
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	writer.Write(raw)
	writer.Close()
	writeObjectFile(t, gitDir, oid, compressed.Bytes())
}

func writeObjectFile(t *testing.T, gitDir, oid string, data []byte) {
	t.Helper()
	path := looseObjectPath(gitDir, oid)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	os.Remove(path)
	if err := os.WriteFile(path, data, 0o444); err != nil {
		t.Fatal(err)
	}
}

// requireS0EResult compares the complete closed result and the exit code with
// the public expectation: all 21 keys, no normalization.
func requireS0EResult(t *testing.T, c s0eCase, result StableResult, exit int) bool {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	ok := exit == c.ExpectedExit
	if !ok {
		t.Errorf("exit = %d, want %d", exit, c.ExpectedExit)
	}
	if len(c.ExpectedResult) != 21 {
		t.Fatalf("public expectation has %d keys, want the closed 21", len(c.ExpectedResult))
	}
	keys := map[string]bool{}
	for key := range got {
		keys[key] = true
	}
	for key := range c.ExpectedResult {
		keys[key] = true
	}
	names := make([]string, 0, len(keys))
	for key := range keys {
		names = append(names, key)
	}
	sort.Strings(names)
	for _, key := range names {
		if !reflect.DeepEqual(got[key], c.ExpectedResult[key]) {
			ok = false
			have, _ := json.Marshal(got[key])
			want, _ := json.Marshal(c.ExpectedResult[key])
			if len(have) > 400 {
				have = append(have[:400], "..."...)
			}
			if len(want) > 400 {
				want = append(want[:400], "..."...)
			}
			t.Errorf("%s = %s, want %s", key, have, want)
		}
	}
	return ok
}

func s0eMapDigest(c s0eCase) string {
	digest, _ := c.ExpectedResult["mapSha256"].(string)
	return digest
}

func stableResultDebug(result StableResult) string {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

func stableRepositoryGit(t *testing.T, repo, stdin string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	command.Dir = repo
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@invalid",
		"GIT_AUTHOR_DATE=2026-10-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-10-01T00:00:00Z")
	command.Stdin = strings.NewReader(stdin)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestStableRepositoryRejectsCreateOverAuthenticatedEmptyBaseTree(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	// The stable boundary intentionally refuses symlink ancestors, including
	// Darwin's /var alias used by t.TempDir.
	fixture, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(fixture, "repo")
	command := exec.Command("git", "init", "-q", "--object-format=sha1", repo)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	empty := stableRepositoryGit(t, repo, "", "mktree")
	baseTree := stableRepositoryGit(t, repo, "040000 tree "+empty+"\tp\n", "mktree")
	base := stableRepositoryGit(t, repo, "", "commit-tree", baseTree, "-m", "base")
	blob := stableRepositoryGit(t, repo, "created\n", "hash-object", "-w", "--stdin")
	targetTree := stableRepositoryGit(t, repo, "100644 blob "+blob+"\tp\n", "mktree")
	target := stableRepositoryGit(t, repo, "", "commit-tree", targetTree, "-p", base, "-m", "target")
	stableRepositoryGit(t, repo, "", "update-ref", "refs/heads/main", target)
	stableRepositoryGit(t, repo, "", "symbolic-ref", "HEAD", "refs/heads/main")
	patchBytes := []byte(stableRepositoryGit(t, repo, "", "diff", "--full-index", "--no-renames", base, target) + "\n")
	parsed, err := patch.Parse(patchBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Hunks) != 1 {
		t.Fatalf("parsed hunks = %d, want1", len(parsed.Hunks))
	}
	h := parsed.Hunks[0]
	raw, err := json.Marshal(map[string]any{
		"spec": "cem/1.0", "baseRevision": base, "excludedPath": ".corvint/change.cem.json", "patchSha256": stableDigest(patchBytes),
		"hunks":    []any{map[string]any{"id": h.ID, "path": h.DisplayPath, "oldRange": map[string]int64{"start": h.OldRange.Start, "count": h.OldRange.Count}, "newRange": map[string]int64{"start": h.NewRange.Start, "count": h.NewRange.Count}, "disposition": "unknown", "reason": "no-evidence", "basis": []any{}}},
		"evidence": []any{}, "criterionBindings": []any{}, "runnerReceipts": []any{}, "criterionLinks": []any{}, "artifacts": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	artifacts := filepath.Join(fixture, "artifacts")
	if err := os.Mkdir(artifacts, 0o755); err != nil {
		t.Fatal(err)
	}
	result, exit := Stable(context.Background(), raw, StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: artifacts})
	if exit != 1 || result.Outcome != "REJECT" || result.Stage != "verification" || len(result.IssueCodes) != 1 || result.IssueCodes[0] != cemcode.InvalidField || result.Axes["changeIntegrity"] != "FAILED" {
		t.Fatalf("Stable exit=%d result=%s, want REJECT verification invalid-field with failed integrity", exit, stableResultDebug(result))
	}
}
