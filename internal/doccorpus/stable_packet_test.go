package doccorpus

// This file duplicates the public S0E repository-envelope packet materializer
// of internal/cem/cli/stable_read_test.go: test helpers are not shared across
// packages, and the packet bytes, not the helper, are the fixture.

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
	"runtime"
	"strings"
	"testing"
)

// stableReadPacket is the public S0E repository-envelope packet the Core read
// tests materialize: one ordinary sha1 repository, its artifacts and its sealed
// positive cem/1.0 map.
const stableReadPacket = "../../protocol/cem-1.0/stable/repository-envelope-packet"

const stableReadAcceptCase = "normal-exit-lingering-descendant-contained"

// stableReadTupleRoot holds one sealed packet per structural tuple admitted
// after the frozen packet's release (decision 0522, CEM-V1-050).
const stableReadTupleRoot = "../../protocol/cem-1.0/structural-tuples"

// admittedAcceptGo returns the running release when its sealed tuple packet
// substitutes exactly the frozen packet's goVersion, so the public ACCEPT
// result holds there too; otherwise it returns the frozen release unchanged.
func admittedAcceptGo(frozen string) string {
	running := runtime.Version()
	data, err := os.ReadFile(filepath.Join(stableReadTupleRoot, running, "TUPLE.json"))
	if err != nil {
		return frozen
	}
	var tuple struct {
		Tuple            string `json:"tuple"`
		PredecessorTuple string `json:"predecessorTuple"`
		Substitution     struct {
			Field, From, To string
		} `json:"substitution"`
	}
	if json.Unmarshal(data, &tuple) != nil || tuple.Tuple != running || tuple.PredecessorTuple != frozen ||
		tuple.Substitution.Field != "expectedResult.runtime.goVersion" || tuple.Substitution.From != frozen || tuple.Substitution.To != running {
		return frozen
	}
	return running
}

type stableReadFixture struct {
	repo, artifacts      string
	raw                  []byte // the sealed positive cem/1.0 map, committed at the target as its sidecar
	expectedBase, target string
	acceptGo             string // the Go release whose public result is ACCEPT
}

// materializeStableRead builds the packet's repository under a fresh directory
// and returns the sealed map's bytes; nothing is written to its worktree.
func materializeStableRead(t *testing.T) stableReadFixture {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("NOT_RUN: the stable verifier's owned Git lifecycle is observed on Darwin and Linux only")
	}
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(stableReadPacket, name))
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
	fixture := stableReadFixture{}
	for _, c := range cases.Cases {
		if c.ID == stableReadAcceptCase {
			fixture.expectedBase, _ = c.ExpectedResult["expectedBase"].(string)
			fixture.target, _ = c.ExpectedResult["targetRevision"].(string)
			runtimeField, _ := c.ExpectedResult["runtime"].(map[string]any)
			fixture.acceptGo, _ = runtimeField["goVersion"].(string)
		}
	}
	if fixture.expectedBase == "" || fixture.target == "" || fixture.acceptGo == "" {
		t.Fatalf("public packet has no complete case %s", stableReadAcceptCase)
	}
	fixture.acceptGo = admittedAcceptGo(fixture.acceptGo)
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixture.repo, fixture.artifacts = filepath.Join(base, "repo"), filepath.Join(base, "artifacts")
	command := exec.Command("git", "init", "-q", "--object-format=sha1", fixture.repo)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if err := os.MkdirAll(fixture.artifacts, 0o755); err != nil {
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
				path := filepath.Join(fixture.repo, ".git", "objects", oid[:2], oid[2:])
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, compressed.Bytes(), 0o444); err != nil {
					t.Fatal(err)
				}
			case strings.HasPrefix(record.Path, "fixtures/artifacts/"):
				if err := os.WriteFile(filepath.Join(fixture.artifacts, filepath.Base(record.Path)), data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	fixture.raw = read("inherited/fixtures/maps/sha1-positive-sealed.json")
	return fixture
}
