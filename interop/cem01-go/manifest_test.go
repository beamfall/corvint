package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

type suiteManifest struct {
	Repository struct {
		Root      string            `json:"root"`
		Author    string            `json:"author"`
		Timestamp string            `json:"timestamp"`
		Message   string            `json:"message"`
		Revisions map[string]string `json:"revisions"`
	} `json:"repository"`
	Valid []struct {
		Name         string `json:"name"`
		ObjectFormat string `json:"objectFormat"`
		Map          string `json:"map"`
		Patch        string `json:"patch"`
	} `json:"valid"`
	Invalid []struct {
		Name         string `json:"name"`
		Map          string `json:"map"`
		Patch        string `json:"patch"`
		PatchRecipe  string `json:"patchRecipe"`
		ExpectedCode string `json:"expectedCode"`
	} `json:"invalid"`
	Drift []struct {
		Name           string  `json:"name"`
		Map            string  `json:"map"`
		Patch          string  `json:"patch"`
		TargetPatch    *string `json:"targetPatch"`
		Timestamp      *string `json:"timestamp"`
		Message        *string `json:"message"`
		TargetRevision string  `json:"targetRevision"`
		Accept         bool    `json:"accept"`
		Status         string  `json:"status"`
		TargetBlobOID  *string `json:"targetBlobOid"`
		TargetSpan     *span   `json:"targetSpan"`
	} `json:"drift"`
	ProducerJobs []struct {
		Name         string `json:"name"`
		ObjectFormat string `json:"objectFormat"`
		Patch        string `json:"patch"`
		ExpectedMap  string `json:"expectedMap"`
	} `json:"producerJobs"`
	ArtifactSHA256 map[string]string `json:"artifactSha256"`
}

type processResponse struct {
	Accept bool        `json:"accept"`
	Spec   string      `json:"spec"`
	Drift  []driftItem `json:"drift"`
	Code   string      `json:"code,omitempty"`
}

// This supplemental packet does not alter the frozen CEM 0.1 manifest.
func TestPortableProfileCompatibility(t *testing.T) {
	t.Run("CEM-CB-004 historical reader compatibility", testPortableProfileCompatibility)
}

func testPortableProfileCompatibility(t *testing.T) {
	kit, err := filepath.Abs("../../protocol/cem-0.2")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(kit, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if shaHex(raw) != "9389102480c910ddb1366d385702bcf44a72dadbbece9e68bce683420f64983a" {
		t.Fatal("portable manifest digest changed")
	}
	var manifest struct {
		Author, Timestamp, BaseRevision, BaseMessage string
		ArtifactSHA256                               map[string]string
		Cases                                        []struct {
			Name, Map, LegacyMap, Patch, TargetRevision string
			Accept                                      bool
			Drift                                       []driftItem
		}
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Cases) != 6 || len(manifest.ArtifactSHA256) != 21 {
		t.Fatal("portable vector matrix changed")
	}
	verifyArtifactDigests(t, kit, manifest.ArtifactSHA256)
	exe := filepath.Join(t.TempDir(), "cem01-go")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", exe, ".")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v: %s", err, out)
	}
	recipe := suiteManifest{}
	recipe.Repository.Root = "repository/base"
	recipe.Repository.Author = manifest.Author
	recipe.Repository.Timestamp = manifest.Timestamp
	recipe.Repository.Message = manifest.BaseMessage
	for _, testCase := range manifest.Cases {
		t.Run("CEM-CB-004 historical reader "+testCase.Name, func(t *testing.T) {
			repo := filepath.Join(t.TempDir(), "repo")
			reconstructRepository(t, kit, repo, "sha1", recipe)
			if got := strings.TrimSpace(gitManifest(t, repo, nil, "rev-parse", "HEAD")); got != manifest.BaseRevision {
				t.Fatalf("base = %s, want %s", got, manifest.BaseRevision)
			}
			gitManifest(t, repo, nil, "apply", "--index", filepath.Join(kit, testCase.Patch))
			mapBytes, err := os.ReadFile(filepath.Join(kit, testCase.Map))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(repo, ".corvint"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(repo, ".corvint/change.cem.json"), mapBytes, 0o644); err != nil {
				t.Fatal(err)
			}
			gitManifest(t, repo, nil, "add", ".corvint/change.cem.json")
			gitManifest(t, repo, commitEnv(t, manifest.Author, manifest.Timestamp), "commit", "-qm", testCase.Name)
			if got := strings.TrimSpace(gitManifest(t, repo, nil, "rev-parse", "HEAD")); got != testCase.TargetRevision {
				t.Fatalf("target = %s, want %s", got, testCase.TargetRevision)
			}
			status, rejected, _ := invokeConsumer(t, exe, repo, filepath.Join(kit, testCase.Map), filepath.Join(kit, testCase.Patch), testCase.TargetRevision)
			if status != 1 || rejected.Accept {
				t.Fatalf("historical reader failed to reject 0.2: exit=%d response=%+v", status, rejected)
			}
			status, accepted, _ := invokeConsumer(t, exe, repo, filepath.Join(kit, testCase.LegacyMap), filepath.Join(kit, testCase.Patch), testCase.TargetRevision)
			wantStatus := 1
			if testCase.Accept {
				wantStatus = 0
			}
			if status != wantStatus || accepted.Accept != testCase.Accept || !reflect.DeepEqual(accepted.Drift, testCase.Drift) {
				t.Fatalf("legacy result exit=%d response=%+v, want exit=%d accept=%v drift=%+v", status, accepted, wantStatus, testCase.Accept, testCase.Drift)
			}
			if testCase.Name == "stable" {
				legacyBytes, err := os.ReadFile(filepath.Join(kit, testCase.LegacyMap))
				if err != nil {
					t.Fatal(err)
				}
				var legacy cemMap
				if err := json.Unmarshal(legacyBytes, &legacy); err != nil {
					t.Fatal(err)
				}
				legacy.Hunks[0].Basis = append(legacy.Hunks[0].Basis, legacy.Hunks[0].Basis[0])
				duplicateBytes, err := json.Marshal(legacy)
				if err != nil {
					t.Fatal(err)
				}
				duplicatePath := filepath.Join(t.TempDir(), "duplicate.cem.json")
				if err := os.WriteFile(duplicatePath, duplicateBytes, 0o600); err != nil {
					t.Fatal(err)
				}
				status, rejected, _ := invokeConsumer(t, exe, repo, duplicatePath, filepath.Join(kit, testCase.Patch), testCase.TargetRevision)
				if status != 1 || rejected.Accept || rejected.Code != "duplicate-basis" {
					t.Fatalf("duplicate within one hunk: exit=%d response=%+v", status, rejected)
				}
			}
		})
	}
}

func TestManifestProcessBoundaryConformance(t *testing.T) {
	if testing.Short() {
		t.Skip("full Git/process conformance")
	}
	kit, err := filepath.Abs(filepath.Join("..", "cem-0.1"))
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile(filepath.Join(kit, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest suiteManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Valid) != 7 || len(manifest.Invalid) != 19 || len(manifest.Drift) != 6 || len(manifest.ProducerJobs) != 5 {
		t.Fatalf("unexpected matrix counts valid=%d invalid=%d drift=%d producer=%d", len(manifest.Valid), len(manifest.Invalid), len(manifest.Drift), len(manifest.ProducerJobs))
	}
	verifyArtifactDigests(t, kit, manifest.ArtifactSHA256)

	exe := filepath.Join(t.TempDir(), "cem01-go")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", exe, ".")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v: %s", err, out)
	}

	repos := map[string]string{}
	for _, format := range []string{"sha1", "sha256"} {
		repo := filepath.Join(t.TempDir(), format)
		reconstructRepository(t, kit, repo, format, manifest)
		got := strings.TrimSpace(gitManifest(t, repo, nil, "rev-parse", "HEAD"))
		if got != manifest.Repository.Revisions[format] {
			t.Fatalf("%s base revision got %s", format, got)
		}
		repos[format] = repo
	}

	t.Run("valid-7", func(t *testing.T) {
		for _, tc := range manifest.Valid {
			t.Run(tc.Name, func(t *testing.T) {
				status, r, _ := invokeConsumer(t, exe, repos[tc.ObjectFormat], filepath.Join(kit, tc.Map), filepath.Join(kit, tc.Patch), "")
				if status != 0 || !r.Accept || len(r.Drift) != 0 {
					t.Fatalf("status=%d response=%+v", status, r)
				}
			})
		}
	})

	t.Run("invalid-19", func(t *testing.T) {
		for _, tc := range manifest.Invalid {
			t.Run(tc.Name, func(t *testing.T) {
				patch := filepath.Join(kit, tc.Patch)
				if tc.PatchRecipe != "" {
					patch = materializeRecipe(t, tc.PatchRecipe)
				}
				status, r, _ := invokeConsumer(t, exe, repos["sha1"], filepath.Join(kit, tc.Map), patch, "")
				if status != 1 || r.Accept {
					t.Fatalf("status=%d response=%+v", status, r)
				}
				if tc.ExpectedCode != "" && r.Code != tc.ExpectedCode {
					t.Fatalf("code=%q want %q", r.Code, tc.ExpectedCode)
				}
			})
		}
	})

	t.Run("drift-6", func(t *testing.T) {
		for _, tc := range manifest.Drift {
			t.Run(tc.Name, func(t *testing.T) {
				repo := repos["sha1"]
				if tc.TargetPatch != nil {
					repo = filepath.Join(t.TempDir(), tc.Name)
					reconstructRepository(t, kit, repo, "sha1", manifest)
					gitManifest(t, repo, nil, "apply", "--index", filepath.Join(kit, *tc.TargetPatch))
					env := commitEnv(t, manifest.Repository.Author, *tc.Timestamp)
					gitManifest(t, repo, env, "commit", "-q", "-m", *tc.Message)
				}
				gotRevision := strings.TrimSpace(gitManifest(t, repo, nil, "rev-parse", "HEAD"))
				if gotRevision != tc.TargetRevision {
					t.Fatalf("target revision got %s", gotRevision)
				}
				status, r, _ := invokeConsumer(t, exe, repo, filepath.Join(kit, tc.Map), filepath.Join(kit, tc.Patch), tc.TargetRevision)
				wantStatus := 1
				if tc.Accept {
					wantStatus = 0
				}
				if status != wantStatus || r.Accept != tc.Accept || len(r.Drift) == 0 {
					t.Fatalf("status=%d response=%+v", status, r)
				}
				for _, item := range r.Drift {
					if item.Status != tc.Status || !sameOptionalString(item.TargetBlobOID, tc.TargetBlobOID) || !sameSpan(item.TargetSpan, tc.TargetSpan) {
						t.Fatalf("drift=%+v expected status=%s oid=%v span=%v", item, tc.Status, tc.TargetBlobOID, tc.TargetSpan)
					}
				}
			})
		}
	})

	t.Run("producer-expected-maps-5", func(t *testing.T) {
		for _, tc := range manifest.ProducerJobs {
			t.Run(tc.Name, func(t *testing.T) {
				status, r, _ := invokeConsumer(t, exe, repos[tc.ObjectFormat], filepath.Join(kit, tc.ExpectedMap), filepath.Join(kit, tc.Patch), "")
				if status != 0 || !r.Accept {
					t.Fatalf("status=%d response=%+v", status, r)
				}
			})
		}
	})
}

func verifyArtifactDigests(t *testing.T, kit string, expected map[string]string) {
	t.Helper()
	keys := make([]string, 0, len(expected))
	for path := range expected {
		keys = append(keys, path)
	}
	sort.Strings(keys)
	for _, path := range keys {
		b, err := os.ReadFile(filepath.Join(kit, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("artifact %s: %v", path, err)
		}
		d := sha256.Sum256(b)
		if got := hex.EncodeToString(d[:]); got != expected[path] {
			t.Fatalf("artifact %s digest %s", path, got)
		}
	}
}

func reconstructRepository(t *testing.T, kit, repo, format string, manifest suiteManifest) {
	t.Helper()
	gitManifest(t, ".", nil, "init", "-q", "--object-format="+format, repo)
	source := filepath.Join(kit, filepath.FromSlash(manifest.Repository.Root))
	err := filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil || rel == "." {
			return err
		}
		dest := filepath.Join(repo, rel)
		if info.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("non-regular fixture %s", rel)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		t.Fatal(err)
	}
	gitManifest(t, repo, nil, "-c", "core.autocrlf=false", "add", "--all")
	gitManifest(t, repo, commitEnv(t, manifest.Repository.Author, manifest.Repository.Timestamp), "commit", "-q", "-m", manifest.Repository.Message)
}

func commitEnv(t *testing.T, author, timestamp string) []string {
	t.Helper()
	a, err := mail.ParseAddress(author)
	if err != nil {
		t.Fatal(err)
	}
	return []string{"GIT_AUTHOR_NAME=" + a.Name, "GIT_AUTHOR_EMAIL=" + a.Address, "GIT_COMMITTER_NAME=" + a.Name, "GIT_COMMITTER_EMAIL=" + a.Address,
		"GIT_AUTHOR_DATE=" + timestamp, "GIT_COMMITTER_DATE=" + timestamp}
}

func gitManifest(t *testing.T, repo string, extraEnv []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = append(os.Environ(), append([]string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0"}, extraEnv...)...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, b)
	}
	return string(b)
}

func invokeConsumer(t *testing.T, exe, repo, mapFile, patchFile, target string) (int, processResponse, []byte) {
	t.Helper()
	args := []string{"verify", "--repository", repo, "--map", mapFile, "--patch", patchFile}
	if target != "" {
		args = append(args, "--target", target)
	}
	cmd := exec.Command(exe, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	status := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("invoke: %v", err)
		}
		status = exitErr.ExitCode()
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.Bytes())
	}
	dec := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	var r processResponse
	if err := dec.Decode(&r); err != nil {
		t.Fatalf("stdout %q: %v", stdout.Bytes(), err)
	}
	if dec.Decode(new(any)) != io.EOF || r.Spec != specVersion || r.Drift == nil {
		t.Fatalf("invalid protocol stdout %q", stdout.Bytes())
	}
	return status, r, stdout.Bytes()
}

func materializeRecipe(t *testing.T, id string) string {
	t.Helper()
	if id != "cem/0.1-lf-overflow-x-lines" {
		t.Fatalf("unknown recipe %q", id)
	}
	b := bytes.Repeat([]byte("x\n"), 262145)
	if len(b) != 524290 || bytes.Count(b, []byte{'\n'}) != 262145 || shaHex(b) != "cfecb16854630a4d141b429fdfdcdc73bb48124ac47375b5896c634037c9eee6" {
		t.Fatal("recipe mismatch")
	}
	p := filepath.Join(t.TempDir(), "overflow.patch")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func sameOptionalString(a, b *string) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
func sameSpan(a *nullableSpan, b *span) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Start == b.Start && a.End == b.End
}
