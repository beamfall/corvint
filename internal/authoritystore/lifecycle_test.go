package authoritystore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/localauthority"
)

type lifecycleFiles struct {
	data  map[string][]byte
	reads []string
}

func (f *lifecycleFiles) read(path string, owner uint32, limit int) ([]byte, error) {
	f.reads = append(f.reads, path)
	raw, ok := f.data[path]
	if !ok || len(raw) > limit {
		return nil, errUnavailable
	}
	return append([]byte(nil), raw...), nil
}
func lifecycleFixture(t *testing.T) (RootDocument, *lifecycleFiles) {
	t.Helper()
	root, floor := rootFixture()
	repo := bindingRepo(t)
	root.RepositoryRoot, root.Git = repo.RepositoryRoot, repo.Git
	root.ReaderUID = strconv.Itoa(os.Getuid())
	if root.AuthorityUID == root.ReaderUID {
		root.AuthorityUID = "601"
		root.AuthorityGID = "601"
	}
	d := strings.Repeat("a", 64)
	root.HostQualification = &HostQualification{Profile: "corvint-native-qualified-host/0", Surface: "codex-desktop", EvidenceSHA256: d, App: Image{SHA256: d}, Engine: Image{SHA256: d}, OSBuild: "test-only", Architecture: "arm64", Topology: "app-owned-stdio"}
	policy := []byte(`{"test-only-policy":true}`)
	root.PolicySHA256 = localauthority.BytesDigest(policy)
	return root, &lifecycleFiles{data: map[string][]byte{"accepted-root.json": encode(t, root), "minimum-generation.json": encode(t, floor), "execution-policy.json": policy}}
}
func TestLifecycleContextHeldScopeWithoutEnrollment(t *testing.T) {
	t.Run("QLF-V0-002 held context scope", func(t *testing.T) {
		root, files := lifecycleFixture(t)
		t.Chdir(root.RepositoryRoot)
		observations, checks := 0, 0
		observe := func(ctx context.Context, scope LifecycleScope) error {
			observations++
			if scope.RepositoryRoot != root.RepositoryRoot || scope.QualifiedHostSHA256 == "" {
				t.Fatal("wrong held scope")
			}
			return nil
		}
		// This test seam bypasses native qualification only to exercise actual Git,
		// cwd and protected file bracketing. It is not host admission evidence.
		check := func(context.Context, RootDocument, *qualificationCampaign) error { checks++; return nil }
		result, err := resolveLifecycleContext(context.Background(), files, observe, check)
		if err != nil || observations != 1 || checks != 2 || result.Scope.RepositoryRoot != root.RepositoryRoot || result.Stop.RootCurrent {
			t.Fatalf("context: %#v %v observations=%d checks=%d", result, err, observations, checks)
		}
		for _, path := range files.reads {
			if path != "accepted-root.json" && path != "minimum-generation.json" && path != "execution-policy.json" {
				t.Fatalf("context consulted enrollment/publication: %s", path)
			}
		}
	})
}
func TestLifecycleContextDriftAndUnqualifiedRefuse(t *testing.T) {
	t.Run("QLF-V0-002 current scope guards", func(t *testing.T) {
		for _, mode := range []string{"revoked", "unqualified", "policy", "floor", "root", "runtime", "cwd", "config", "observation", "cancelled"} {
			t.Run(mode, func(t *testing.T) {
				root, files := lifecycleFixture(t)
				t.Chdir(root.RepositoryRoot)
				if mode == "revoked" {
					root.Revoked = true
					files.data["accepted-root.json"] = encode(t, root)
				}
				if mode == "unqualified" {
					root.HostQualification = nil
					files.data["accepted-root.json"] = encode(t, root)
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				observed := false
				checks := 0
				observe := func(context.Context, LifecycleScope) error {
					observed = true
					switch mode {
					case "root":
						files.data["accepted-root.json"] = append(files.data["accepted-root.json"], ' ')
					case "floor":
						files.data["minimum-generation.json"] = append(files.data["minimum-generation.json"], ' ')
					case "policy":
						files.data["execution-policy.json"] = []byte("drift")
					case "cwd":
						if err := os.Chdir(t.TempDir()); err != nil {
							t.Fatal(err)
						}
					case "config":
						if err := os.WriteFile(filepath.Join(root.RepositoryRoot, ".git", "config"), []byte("[core]\n bare = true\n"), 0600); err != nil {
							t.Fatal(err)
						}
					case "observation":
						return errors.New("test-only-refusal")
					case "cancelled":
						cancel()
					}
					return nil
				}
				check := func(context.Context, RootDocument, *qualificationCampaign) error {
					checks++
					if mode == "runtime" && checks > 1 {
						return errors.New("test-only-runtime-drift")
					}
					return nil
				}
				if result, err := resolveLifecycleContext(ctx, files, observe, check); err == nil || result.Scope.RepositoryRoot != "" {
					t.Fatalf("accepted drift: %#v %v", result, err)
				}
				if (mode == "revoked" || mode == "unqualified") && observed {
					t.Fatal("unqualified/revoked root reached context")
				}
			})
		}
	})
}
func TestLifecycleActivePointerClosedAndPinned(t *testing.T) {
	t.Run("QLF-V0-003 active pointer binding", func(t *testing.T) {
		handle := strings.Repeat("a", 64)
		raw := encode(t, ActiveEnrollment{Profile: "corvint-protected-active-enrollment/0", EnrollmentHandle: handle})
		files := &lifecycleFiles{data: map[string][]byte{"public/active-enrollment.json": raw}}
		pinned, got, err := readActiveEnrollment(files, 600)
		if err != nil || got != handle {
			t.Fatal(err)
		}
		files.data["public/active-enrollment.json"] = encode(t, ActiveEnrollment{Profile: "corvint-protected-active-enrollment/0", EnrollmentHandle: strings.Repeat("b", 64)})
		if unchanged(files, "public/active-enrollment.json", 600, pinned) == nil {
			t.Fatal("active pointer swap accepted")
		}
		for _, bad := range []string{`{}`, `{"profile":"corvint-protected-active-enrollment/0","enrollmentHandle":"../../fake"}`, `{"profile":"corvint-protected-active-enrollment/0","enrollmentHandle":"` + handle + `","root":"/fake"}`} {
			files.data["public/active-enrollment.json"] = []byte(bad)
			if _, _, err := readActiveEnrollment(files, 600); err == nil {
				t.Fatal("invalid protected pointer accepted")
			}
		}
	})
}

func TestLifecycleCandidateContextUsesSameReadAndRejectsDrift(t *testing.T) {
	t.Run("QLF-V0-009 candidate currentness", func(t *testing.T) {
		for _, mode := range []string{"stable", "old-scope", "missing", "expired", "campaign-drift", "campaign-removed", "runtime-drift"} {
			t.Run(mode, func(t *testing.T) {
				root, files := lifecycleFixture(t)
				root.HostQualification = nil
				template, _, candidate, _ := campaignFixture(t)
				root.Consumer, root.Adapter = template.Consumer, template.Adapter
				t.Chdir(root.RepositoryRoot)
				bindingGit(t, root.RepositoryRoot, "-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "test-only-base")
				target := strings.TrimSpace(bindingGit(t, root.RepositoryRoot, "rev-parse", "HEAD"))
				candidate.Scope = lifecycleCampaignScope
				candidate.RootID, candidate.Epoch, candidate.Generation = root.RootID, root.Epoch, root.Generation
				candidate.RepositoryID, candidate.RepositoryRoot, candidate.PolicySHA256 = root.RepositoryID, root.RepositoryRoot, root.PolicySHA256
				candidate.MinimumGenerationSHA256 = localauthority.BytesDigest(files.data["minimum-generation.json"])
				candidate.Consumer, candidate.Adapter, candidate.Git = root.Consumer, root.Adapter, root.Git
				candidate.Target = target
				now := time.Now().UTC().Truncate(time.Second)
				candidate.IssuedAt, candidate.ExpiresAt = now.Add(-time.Second).Format(time.RFC3339), now.Add(300*time.Second).Format(time.RFC3339)
				switch mode {
				case "old-scope":
					candidate.Scope = stopCampaignScope
				case "expired":
					candidate.ExpiresAt = now.Format(time.RFC3339)
				}
				files.data["accepted-root.json"] = encode(t, root)
				if mode != "missing" {
					files.data[campaignPath] = encode(t, candidate)
				}
				observed, checks := 0, 0
				check := func(_ context.Context, r RootDocument, c *qualificationCampaign) error {
					checks++
					if r.HostQualification != nil || c == nil {
						t.Fatal("candidate synthesized qualification")
					}
					if mode == "runtime-drift" && checks > 1 {
						return errUnavailable
					}
					return nil
				}
				observe := func(_ context.Context, scope LifecycleScope) error {
					observed++
					if scope.ExpectedTarget != target || scope.QualifiedHostSHA256 != "" || scope.EvidenceSHA256 != "" || len(scope.QualifiedSurfaces) != 0 || scope.SupportScope != "candidate-native-runtime" || scope.AppSHA256 != candidate.Runtime.App.SHA256 {
						t.Fatalf("candidate upgraded %#v", scope)
					}
					switch mode {
					case "campaign-drift":
						files.data[campaignPath] = append(files.data[campaignPath], ' ')
					case "campaign-removed":
						delete(files.data, campaignPath)
					}
					return nil
				}
				result, err := resolveLifecycleContext(context.Background(), files, observe, check)
				if mode == "stable" {
					if err != nil || observed != 1 || checks != 2 || result.Scope.QualifiedHostSHA256 != "" {
						t.Fatalf("candidate read failed %#v %v observed=%d checks=%d", result, err, observed, checks)
					}
				} else if err == nil || result.Scope.RepositoryRoot != "" {
					t.Fatalf("candidate drift accepted %#v %v", result, err)
				}
				for _, path := range files.reads {
					if strings.HasPrefix(path, "public/") || strings.HasPrefix(path, "private/") {
						t.Fatalf("candidate context opened enrollment/publication %s", path)
					}
				}
			})
		}
	})
}
func TestLifecycleCampaignScopeDoesNotBroadenLegacy(t *testing.T) {
	t.Run("QLF-V0-008 campaign scope isolation", func(t *testing.T) {
		root, floor, candidate, now := campaignFixture(t)
		if !candidate.valid(root, floor, candidate.EnrollmentHandle, now) {
			t.Fatal("legacy campaign changed")
		}
		if candidate.validScope(root, floor, candidate.EnrollmentHandle, now, lifecycleCampaignScope) {
			t.Fatal("Stop-only campaign admitted lifecycle")
		}
		candidate.Scope = lifecycleCampaignScope
		if candidate.valid(root, floor, candidate.EnrollmentHandle, now) {
			t.Fatal("legacy Stop admitted new scope")
		}
		if !candidate.validScope(root, floor, candidate.EnrollmentHandle, now, lifecycleCampaignScope) {
			t.Fatal("explicit lifecycle scope rejected")
		}
		files := &lifecycleFiles{data: map[string][]byte{campaignPath: encode(t, candidate)}}
		root.HostQualification = &HostQualification{Profile: "invalid-present-document"}
		admission, err := readLifecycleContextAdmission(files, root, floor, now)
		if err != nil || admission.campaign != nil || len(files.reads) != 0 {
			t.Fatal("present qualification borrowed campaign")
		}
		if verifyLifecycleRuntime(context.Background(), root, &candidate) == nil {
			t.Fatal("invalid present qualification used candidate pins")
		}
	})
}
