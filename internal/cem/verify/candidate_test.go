package verify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

const candidateManifestSHA = "18de09b6d6ede40d34d973381372f97a5cf490a006da8a23312021f16d3282e5"

type candidatePacket struct {
	Repositories   []candidateRepo
	Cases          []candidateCase
	ArtifactSHA256 map[string]string
}
type candidateRepo struct{ Name, ObjectFormat, BaseRevision, ContentRevision, TargetRevision, Map, Patch string }
type candidateCase struct {
	Name, Repository, Map, Integrity, OmitArtifact, ChangeArtifact, Target, ArtifactDirectory string
	Exit                                                                                      int
}

func candidateGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	cmd.Dir = root
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_AUTHOR_NAME=Candidate Proof", "GIT_AUTHOR_EMAIL=proof@example.invalid", "GIT_COMMITTER_NAME=Candidate Proof", "GIT_COMMITTER_EMAIL=proof@example.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z"}
	b, e := cmd.Output()
	if e != nil {
		t.Fatalf("git %v: %v %s", args, e, b)
	}
	return strings.TrimSpace(string(b))
}
func candidateBuildRepo(t *testing.T, kit string, r candidateRepo) string {
	t.Helper()
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	candidateGit(t, root, "init", "-q", "--object-format="+r.ObjectFormat, "-b", "main")
	copyTree(t, filepath.Join(kit, "repository", "base"), root)
	candidateGit(t, root, "add", ".")
	candidateGit(t, root, "commit", "-qm", "candidate base")
	if got := candidateGit(t, root, "rev-parse", "HEAD"); got != r.BaseRevision {
		t.Fatalf("base %s", got)
	}
	candidateGit(t, root, "apply", filepath.Join(kit, r.Patch))
	candidateGit(t, root, "add", ".")
	candidateGit(t, root, "commit", "-qm", "candidate content")
	if got := candidateGit(t, root, "rev-parse", "HEAD"); got != r.ContentRevision {
		t.Fatalf("content %s", got)
	}
	if e := os.Mkdir(filepath.Join(root, ".corvint"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(root, wire.ExcludedCEMPath), readFixture(t, kit, r.Map), 0644); e != nil {
		t.Fatal(e)
	}
	candidateGit(t, root, "add", ".")
	candidateGit(t, root, "commit", "-qm", "candidate sidecar")
	if got := candidateGit(t, root, "rev-parse", "HEAD"); got != r.TargetRevision {
		t.Fatalf("target %s", got)
	}
	return root
}
func TestCandidatePortablePacket(t *testing.T) {
	kit := filepath.Join(repoRoot(t), "protocol", "cem-1.0")
	raw := readFixture(t, kit, "manifest.json")
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != candidateManifestSHA {
		t.Fatal("candidate packet manifest changed")
	}
	var packet candidatePacket
	if e := json.Unmarshal(raw, &packet); e != nil {
		t.Fatal(e)
	}
	for name, want := range packet.ArtifactSHA256 {
		sum := sha256.Sum256(readFixture(t, kit, name))
		if hex.EncodeToString(sum[:]) != want {
			t.Fatalf("artifact %s", name)
		}
	}
	roots, definitions := map[string]string{}, map[string]candidateRepo{}
	for _, r := range packet.Repositories {
		roots[r.Name] = candidateBuildRepo(t, kit, r)
		definitions[r.Name] = r
	}
	for _, tc := range packet.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			artRoot := t.TempDir()
			artifactDirectory := tc.ArtifactDirectory
			if artifactDirectory == "" {
				artifactDirectory = "artifacts"
			}
			copyTree(t, filepath.Join(kit, "artifacts"), filepath.Join(artRoot, artifactDirectory))
			if tc.OmitArtifact != "" {
				if e := os.Remove(filepath.Join(artRoot, tc.OmitArtifact)); e != nil {
					t.Fatal(e)
				}
			}
			if tc.ChangeArtifact != "" {
				if e := os.WriteFile(filepath.Join(artRoot, tc.ChangeArtifact), []byte("changed"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			repo, e := gitauth.Open(roots[tc.Repository], gitrun.NewDefaultBudget())
			if e != nil {
				t.Fatal(e)
			}
			def := definitions[tc.Repository]
			target := def.TargetRevision
			if tc.Target == "content" {
				target = def.ContentRevision
			}
			got, e := ExperimentalCandidate(t.Context(), repo, readFixture(t, kit, tc.Map), CandidateOptions{def.BaseRevision, target, artRoot})
			if (e == nil) != (tc.Exit == 0) || got.Integrity != tc.Integrity {
				t.Fatalf("got %+v, err %v", got, e)
			}
			if len(got.Limits) != 8 {
				t.Fatal("lost limitations")
			}
			for _, v := range got.Limits {
				if v != "NOT_OBSERVED" {
					t.Fatal("invented authority")
				}
			}
			if got.ReferenceIntegrity != "REFERENCE_INTEGRITY_ONLY" {
				t.Fatal("reference integrity overstated")
			}
		})
	}
	t.Run("independent baseline and symlink refusal", func(t *testing.T) {
		def := definitions["sha1"]
		repo, e := gitauth.Open(roots["sha1"], gitrun.NewDefaultBudget())
		if e != nil {
			t.Fatal(e)
		}
		if _, e = ExperimentalCandidate(t.Context(), repo, readFixture(t, kit, def.Map), CandidateOptions{def.TargetRevision, def.TargetRevision, kit}); e == nil {
			t.Fatal("wrong independent base accepted")
		}
		artRoot := t.TempDir()
		if e = os.Symlink(filepath.Join(kit, "artifacts"), filepath.Join(artRoot, "artifacts")); e != nil {
			t.Fatal(e)
		}
		if _, e = ExperimentalCandidate(t.Context(), repo, readFixture(t, kit, def.Map), CandidateOptions{def.BaseRevision, def.TargetRevision, artRoot}); e == nil {
			t.Fatal("artifact parent symlink admitted")
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, e = ExperimentalCandidate(ctx, repo, readFixture(t, kit, def.Map), CandidateOptions{def.BaseRevision, def.TargetRevision, kit}); e == nil {
			t.Fatal("cancelled verification admitted")
		}
	})
}
