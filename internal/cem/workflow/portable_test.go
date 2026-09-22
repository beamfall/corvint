package workflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

type portableManifest struct {
	BaseRevision, BaseMessage, Timestamp string
	Cases                                []portableCase
	ArtifactSHA256                       map[string]string
}

type portableCase struct {
	Name, Map, LegacyMap, Patch, TargetRevision string
	Accept                                      bool
	UnknownHunks                                int
	Drift                                       json.RawMessage
}

func portableGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", args...)
	command.Dir = root
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull,
		"GIT_AUTHOR_NAME=Portable Proof", "GIT_AUTHOR_EMAIL=proof@example.invalid",
		"GIT_COMMITTER_NAME=Portable Proof", "GIT_COMMITTER_EMAIL=proof@example.invalid",
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z"}
	out, err := command.Output()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func portableRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func portableRepository(t *testing.T, kit string, manifest portableManifest, testCase portableCase) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	portableGit(t, root, "init", "-q", "--object-format=sha1", "-b", "main")
	for _, path := range []string{"a.txt", "b.txt", "app.txt"} {
		writeFile(t, root, path, string(portableRead(t, filepath.Join(kit, "repository/base", path))))
	}
	portableGit(t, root, "add", ".")
	portableGit(t, root, "commit", "-qm", manifest.BaseMessage)
	if got := portableGit(t, root, "rev-parse", "HEAD"); got != manifest.BaseRevision {
		t.Fatalf("base = %s, want %s", got, manifest.BaseRevision)
	}
	portableGit(t, root, "apply", filepath.Join(kit, testCase.Patch))
	writeFile(t, root, wire.ExcludedCEMPath, string(portableRead(t, filepath.Join(kit, testCase.Map))))
	portableGit(t, root, "add", "-A")
	portableGit(t, root, "commit", "-qm", testCase.Name)
	if got := portableGit(t, root, "rev-parse", "HEAD"); got != testCase.TargetRevision {
		t.Fatalf("target = %s, want %s", got, testCase.TargetRevision)
	}
	return root
}

func TestPortableCanonicalVectors(t *testing.T) {
	t.Run("CEM-CB-003 portable evidence and unknowns", testPortableCanonicalVectors)
}

func testPortableCanonicalVectors(t *testing.T) {
	kit, err := filepath.Abs("../../../protocol/cem-0.2")
	if err != nil {
		t.Fatal(err)
	}
	raw := portableRead(t, filepath.Join(kit, "manifest.json"))
	if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != "9389102480c910ddb1366d385702bcf44a72dadbbece9e68bce683420f64983a" {
		t.Fatalf("portable manifest digest changed: %s", got)
	}
	var manifest portableManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Cases) != 6 || len(manifest.ArtifactSHA256) != 21 {
		t.Fatal("portable vector matrix changed")
	}
	for path, want := range manifest.ArtifactSHA256 {
		if got := fmt.Sprintf("%x", sha256.Sum256(portableRead(t, filepath.Join(kit, path)))); got != want {
			t.Fatalf("artifact %s digest = %s, want %s", path, got, want)
		}
	}
	for _, testCase := range manifest.Cases {
		t.Run("CEM-CB-003 portable "+testCase.Name, func(t *testing.T) {
			root := portableRepository(t, kit, manifest, testCase)
			options := ReadOptions{MapPath: wire.ExcludedCEMPath, ExpectedBase: manifest.BaseRevision, Target: testCase.TargetRevision}
			result, err := openSession(t, root).Read(context.Background(), "status", options)
			if err != nil {
				t.Fatal(err)
			}
			verification := result["verification"].(map[string]any)
			if verification["valid"] != testCase.Accept || verification["assurance"] != "canonical" || verification["targetRevision"] != testCase.TargetRevision {
				t.Fatalf("verification = %#v", verification)
			}
			rows := verification["drift"].([]any)
			for _, row := range rows {
				delete(row.(map[string]any), "baseBlobOid")
			}
			got, err := json.Marshal(rows)
			if err != nil {
				t.Fatal(err)
			}
			var want any
			if err := json.Unmarshal(testCase.Drift, &want); err != nil {
				t.Fatal(err)
			}
			canonicalWant, _ := json.Marshal(want)
			if !bytes.Equal(got, canonicalWant) {
				t.Fatalf("drift = %s, want %s", got, canonicalWant)
			}
			if result["counts"].(map[string]any)["unknown"] != testCase.UnknownHunks {
				t.Fatalf("unknowns lost: %#v", result["counts"])
			}
			if testCase.UnknownHunks != 0 {
				row := result["worklist"].([]any)[0].(map[string]any)
				if row["disposition"] != "unknown" || row["reason"] != "no-evidence" || row["next"] != "cite-or-mark" {
					t.Fatalf("unknown reason lost: %#v", row)
				}
				zero := 0
				options.Limits.MaxUnknown = &zero
				strict, err := openSession(t, root).Read(context.Background(), "status", options)
				if err != nil || strict["ok"] != false || strict["state"] != "incomplete" {
					t.Fatalf("unknown promoted to complete: %#v, %v", strict, err)
				}
			}
			if testCase.Name == "stable" {
				portableAuthorityFailures(t, root, options)
			}
		})
	}
}

func portableAuthorityFailures(t *testing.T, root string, valid ReadOptions) {
	t.Helper()
	for _, testCase := range []struct {
		name, code string
		options    ReadOptions
	}{
		{"CEM-CB-010 missing base", "expected-base-required", ReadOptions{MapPath: valid.MapPath, Target: valid.Target}},
		{"CEM-CB-010 missing target", "target-required", ReadOptions{MapPath: valid.MapPath, ExpectedBase: valid.ExpectedBase}},
		{"CEM-CB-010 wrong base", "base-revision-mismatch", ReadOptions{MapPath: valid.MapPath, ExpectedBase: valid.Target, Target: valid.Target}},
		{"CEM-CB-012 external patch", "invalid-arguments", ReadOptions{MapPath: valid.MapPath, ExpectedBase: valid.ExpectedBase, Target: valid.Target, PatchGiven: true, PatchPath: "unused.patch"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := openSession(t, root).Read(context.Background(), "status", testCase.options)
			assertPortableFailure(t, result, err, testCase.code)
		})
	}
	t.Run("CEM-CB-009 raw artifact mismatch", func(t *testing.T) {
		path := filepath.Join(root, wire.ExcludedCEMPath)
		writeFile(t, root, "copied.cem.json", string(portableRead(t, path))+"\n")
		options := valid
		options.MapPath = "copied.cem.json"
		result, err := openSession(t, root).Read(context.Background(), "status", options)
		assertPortableFailure(t, result, err, "excluded-artifact-mismatch")
	})
}

func assertPortableFailure(t *testing.T, result map[string]any, err error, want string) {
	t.Helper()
	if err != nil {
		if cemcode.CodeOf(err) != want {
			t.Fatalf("error = %v, want %s", err, want)
		}
		return
	}
	verification := result["verification"].(map[string]any)
	issues := verification["issues"].([]any)
	if result["ok"] != false || verification["valid"] != false || len(issues) != 1 || issues[0].(map[string]any)["code"] != want {
		t.Fatalf("result = %#v, want %s", result, want)
	}
}
