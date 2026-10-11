package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// cemStablePacket is the public S0E repository-envelope packet.
const cemStablePacket = "../../protocol/cem-1.0/stable/repository-envelope-packet"

type cemStableRootFixture struct {
	repo, alias, artifacts string
	expectedBase, target   string
}

// materializeCEMStableRoot builds the packet's sha1 repository and artifacts
// under a symlink-free directory, adds a symlinked alias of the repository, and
// writes the sealed positive map into the worktree as change.cem.json without
// its structural mechanical hunk, so every Go release reaches repository
// admission instead of the verifier's structural runtime refusal.
func materializeCEMStableRoot(t *testing.T) cemStableRootFixture {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("NOT_RUN: the stable verifier's owned Git lifecycle is observed on Darwin and Linux only")
	}
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(cemStablePacket, name))
		if err != nil {
			t.Fatalf("public packet: %v", err)
		}
		return data
	}
	var cases struct {
		Cases []struct {
			ID             string         `json:"id"`
			ExpectedResult map[string]any `json:"expectedResult"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(read("FULL-RESULT-CASES.json"), &cases); err != nil {
		t.Fatal(err)
	}
	f := cemStableRootFixture{}
	for _, c := range cases.Cases {
		if c.ID == "normal-exit-lingering-descendant-contained" {
			f.expectedBase, _ = c.ExpectedResult["expectedBase"].(string)
			f.target, _ = c.ExpectedResult["targetRevision"].(string)
		}
	}
	if f.expectedBase == "" || f.target == "" {
		t.Fatal("public packet has no complete positive case")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f.repo, f.alias, f.artifacts = filepath.Join(base, "repo"), filepath.Join(base, "alias"), filepath.Join(base, "artifacts")
	command := exec.Command("git", "init", "-q", "--object-format=sha1", f.repo)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if err := os.MkdirAll(f.artifacts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(f.repo, f.alias); err != nil {
		t.Fatal(err)
	}
	for _, packName := range []string{"inherited/fixtures/FIXTURES.pack.json", "inherited/fixtures/FIXTURES-ledger-boundary.pack.json"} {
		var pack struct {
			Records []struct{ Path, Type, Sha256, Base64 string } `json:"records"`
		}
		if err := json.Unmarshal(read(packName), &pack); err != nil {
			t.Fatal(err)
		}
		for _, record := range pack.Records {
			if record.Type != "regular" {
				continue
			}
			data, err := base64.StdEncoding.DecodeString(record.Base64)
			if digest := sha256.Sum256(data); err != nil || hex.EncodeToString(digest[:]) != record.Sha256 {
				t.Fatalf("fixture record %s does not match its digest", record.Path)
			}
			destination := ""
			switch {
			case strings.HasPrefix(record.Path, "fixtures/git-objects/sha1/"):
				oid := strings.TrimSuffix(filepath.Base(record.Path), ".object")
				if sum := sha1.Sum(data); hex.EncodeToString(sum[:]) != oid {
					t.Fatalf("object record %s is not the raw object of its name", record.Path)
				}
				var compressed bytes.Buffer
				writer := zlib.NewWriter(&compressed)
				writer.Write(data)
				writer.Close()
				destination, data = filepath.Join(f.repo, ".git", "objects", oid[:2], oid[2:]), compressed.Bytes()
			case strings.HasPrefix(record.Path, "fixtures/artifacts/"):
				destination = filepath.Join(f.artifacts, filepath.Base(record.Path))
			default:
				continue
			}
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(destination, data, 0o444); err != nil {
				t.Fatal(err)
			}
		}
	}
	document, err := wire.ParseStable(read("inherited/fixtures/maps/sha1-positive-sealed.json"))
	if err != nil {
		t.Fatal(err)
	}
	kept := document.Change.Hunks[:0]
	for _, hunk := range document.Change.Hunks {
		if hunk.Disposition != "mechanical" || !wire.StructuralReasons[hunk.Reason] {
			kept = append(kept, hunk)
		}
	}
	document.Change.Hunks = kept
	encoded, err := wire.EncodeStable(document)
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, f.repo, "change.cem.json", string(encoded))
	return f
}

// runCEMStableEntry runs one argv through the real command entry and decodes
// its single stdout JSON document; a stable read writes nothing to stderr.
func runCEMStableEntry(t *testing.T, args ...string) (int, map[string]any) {
	t.Helper()
	code, stdout, stderr := runCLI(t, args...)
	var document map[string]any
	if stderr != "" || json.Unmarshal([]byte(stdout), &document) != nil {
		t.Fatalf("%v: exit %d stdout %q stderr %q", args, code, stdout, stderr)
	}
	return code, document
}

// CEM-V1-017: stable admission judges the --root value byte for byte, as
// `cem verify-stable --repository` judges the same string. For every spelling of
// the root (absolute or relative, clean or not, through the alias or the real
// directory, or explicitly empty) Core status and verify reproduce
// verify-stable's complete result and exit; an omitted --root is the working
// directory. Parsing the argv resolves the root for 0.x reads only.
func TestCEMStableReadKeepsCallerRoot(t *testing.T) {
	f := materializeCEMStableRoot(t)
	base := filepath.Dir(f.repo)
	for _, c := range []struct {
		dir        string
		core       []string
		repository string
		stage      string
	}{
		{base, []string{"--root", f.repo}, f.repo, ""},
		{base, []string{"--root", f.alias}, f.alias, "repository"},
		{base, []string{"--root", f.alias + "/../repo"}, f.alias + "/../repo", "repository"},
		{base, []string{"--root", f.repo + "/"}, f.repo + "/", "repository"},
		{base, []string{"--root", "./repo"}, "./repo", "arguments"},
		{base, []string{"--root", "repo"}, "repo", "arguments"},
		{base, []string{"--root", "./alias"}, "./alias", "arguments"},
		{f.repo, []string{"--root", ""}, "", "arguments"},
		{f.repo, []string{"--root="}, "", "arguments"},
		{f.repo, nil, f.repo, ""},
	} {
		t.Chdir(c.dir)
		wantExit, want := runCEMStableEntry(t, "cem", "verify-stable", "--repository", c.repository,
			"--map", filepath.Join(f.repo, "change.cem.json"), "--expected-base", f.expectedBase,
			"--target", f.target, "--artifacts", f.artifacts)
		stage, _ := want["stage"].(string)
		if c.stage != stage && (c.stage != "" || stage == "arguments" || stage == "repository" || stage == "runtime") {
			t.Fatalf("fixture does not discriminate %q: verify-stable stage %q, want %q", c.core, stage, c.stage)
		}
		for _, action := range []string{"verify", "status"} {
			args := append(append([]string{}, c.core...), "cem", action, "--map", "change.cem.json",
				"--expected-base", f.expectedBase, "--target", f.target, "--artifacts", f.artifacts)
			exit, envelope := runCEMStableEntry(t, args...)
			if exit != wantExit || !reflect.DeepEqual(envelope["stableVerification"], want) {
				t.Errorf("%q cem %s in %s = exit %d %v\nverify-stable --repository %q = exit %d %v",
					c.core, action, c.dir, exit, envelope["stableVerification"], c.repository, wantExit, want)
			}
		}
	}
}

// CEM-V1-019: a 0.x read through the same root spellings keeps Core's root:
// normalized and symlink-resolved when --root names a path, and the working
// directory when --root is explicitly empty, exactly as when it is omitted. Each
// reports the symlink-resolved map path, so 0.x output bytes are unchanged by the
// stable caller-root route.
func TestCEMLegacyReadKeepsResolvedRoot(t *testing.T) {
	f := materializeCEMStableRoot(t)
	base := filepath.Dir(f.repo)
	cemWrite(t, f.repo, "legacy.cem.json", `{"spec":"cem/0.2","baseRevision":"`+f.expectedBase+`",`+
		`"patchSha256":"`+strings.Repeat("0", 64)+`","excludedPath":".corvint/change.cem.json","evidence":[],"hunks":[]}`)
	for _, c := range []struct {
		dir  string
		core []string
	}{
		{base, []string{"--root", f.alias}},
		{base, []string{"--root", f.alias + "/../repo"}},
		{base, []string{"--root", f.repo + "/"}},
		{base, []string{"--root", "./alias"}},
		{base, []string{"--root", "repo"}},
		{f.alias, []string{"--root", ""}},
		{f.alias, []string{"--root="}},
		{f.alias, nil},
	} {
		t.Chdir(c.dir)
		args := append(append([]string{}, c.core...), "cem", "status", "--map", "legacy.cem.json",
			"--expected-base", f.expectedBase, "--target", f.target)
		exit, envelope := runCEMStableEntry(t, args...)
		if exit != 1 || envelope["map"] != filepath.Join(f.repo, "legacy.cem.json") || envelope["state"] != "invalid" {
			t.Errorf("0.x status %q in %s = exit %d map %v state %v", c.core, c.dir, exit, envelope["map"], envelope["state"])
		}
	}
}
