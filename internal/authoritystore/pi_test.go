package authoritystore

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/localauthority"
)

func piFixture() PiRuntime {
	r := PiRuntime(directFixture())
	r.Topology = "protected-pi-sdk"
	r.Host = "pi"
	r.Surface = "pi-native"
	r.HostImage.Path = "/test-only/pi-protected"
	return r
}
func piQualificationFixture() *PiQualification {
	d := strings.Repeat("d", 64)
	return &PiQualification{Profile: piQualificationProfile, EvidenceSHA256: d, Runtime: piFixture(), QualifiedSurfaces: []SurfaceQualification{{Surface: "pi-tui", EvidenceSHA256: d}, {Surface: "pi-rpc", EvidenceSHA256: strings.Repeat("e", 64)}}}
}
func TestPiClosedAuthorityWire(t *testing.T) {
	t.Run("PPI-V0-005 mixed profiles and incomplete surface admission refuse", func(t *testing.T) {
		root, floor := rootFixture()
		root.Profile = PiRootProfile
		root.PiQualification = piQualificationFixture()
		raw := encode(t, root)
		decoded, _, _, err := decodeRoot(raw, encode(t, floor))
		if err != nil || !decoded.PiQualification.valid() || decoded.DirectQualification != nil || decoded.HostQualification != nil {
			t.Fatal(err)
		}
		for _, profile := range []string{RootProfile, DirectRootProfile} {
			if _, _, _, err := decodeRoot(bytes.Replace(raw, []byte(PiRootProfile), []byte(profile), 1), encode(t, floor)); err == nil {
				t.Fatal("mixed profile accepted")
			}
		}
		for _, name := range []string{"hostInstance", "hostImage", "hostCDHash", "runtimeAdmissionEvidenceSHA256", "bootSessionUUID", "parentPolicy"} {
			var fields map[string]any
			json.Unmarshal(raw, &fields)
			delete(fields["hostQualification"].(map[string]any)["runtime"].(map[string]any), name)
			if _, _, _, err := decodeRoot(encode(t, fields), encode(t, floor)); err == nil {
				t.Fatalf("absent %s accepted", name)
			}
		}
		for _, mode := range []string{"one-surface", "reversed", "foreign", "empty-evidence", "codex-runtime", "argv-surface"} {
			q := piQualificationFixture()
			switch mode {
			case "one-surface":
				q.QualifiedSurfaces = q.QualifiedSurfaces[:1]
			case "reversed":
				q.QualifiedSurfaces[0], q.QualifiedSurfaces[1] = q.QualifiedSurfaces[1], q.QualifiedSurfaces[0]
			case "foreign":
				q.QualifiedSurfaces[1].Surface = "codex-cli"
			case "empty-evidence":
				q.QualifiedSurfaces[1].EvidenceSHA256 = ""
			case "codex-runtime":
				q.Runtime.Host = "codex"
			case "argv-surface":
				q.Runtime.Surface = "pi-tui"
			}
			root.PiQualification = q
			if _, _, _, err := decodeRoot(encode(t, root), encode(t, floor)); err == nil {
				t.Fatal(mode)
			}
		}
		if DirectRuntime(piFixture()).valid() || PiRuntime(directFixture()).valid() {
			t.Fatal("runtime identities interchangeable")
		}
	})
}
func TestPiCampaignRemainsUnqualified(t *testing.T) {
	t.Run("PPI-V0-008 bounded candidate cannot borrow qualification", func(t *testing.T) {
		root, floor, c, now := campaignFixture(t)
		root.Profile = PiRootProfile
		c.Profile = piCampaignProfile
		c.Scope = lifecycleCampaignScope
		r := piFixture()
		c.PiRuntime = &r
		c.Runtime = candidateRuntime{}
		raw := encode(t, c)
		var decoded qualificationCampaign
		if localauthority.Decode(raw, &decoded) != nil || !decoded.validScope(root, floor, c.EnrollmentHandle, now, lifecycleCampaignScope) {
			t.Fatal("valid Pi candidate refused")
		}
		for _, mode := range []string{"expired", "too-long", "wrong-root", "wrong-profile", "standalone-stop", "target", "mixed-runtime"} {
			bad := c
			changedRoot := root
			switch mode {
			case "expired":
				bad.ExpiresAt = now.Format(time.RFC3339)
			case "too-long":
				bad.ExpiresAt = now.Add(901 * time.Second).Format(time.RFC3339)
			case "wrong-root":
				changedRoot.Profile = DirectRootProfile
			case "wrong-profile":
				bad.Profile = directCampaignProfile
			case "standalone-stop":
				bad.Scope = stopCampaignScope
			case "target":
				bad.Target = "HEAD"
			case "mixed-runtime":
				d := directFixture()
				bad.DirectRuntime = &d
			}
			if bad.validScope(changedRoot, floor, c.EnrollmentHandle, now, lifecycleCampaignScope) {
				t.Fatal(mode)
			}
		}
		scope, err := (runtimeAdmission{campaign: &c, scope: lifecycleCampaignScope}).lifecycleScope(root)
		if err != nil || !scope.Pi || scope.Direct || scope.QualifiedHostSHA256 != "" || len(scope.QualifiedSurfaces) != 0 || scope.ExpectedTarget != c.Target {
			t.Fatal("candidate promoted")
		}
		root.PiQualification = &PiQualification{}
		files := &campaignFiles{raw: raw}
		if _, err := admitLifecycleRuntime(context.Background(), root, files, floor, c.EnrollmentHandle, now); err == nil || files.reads != 0 {
			t.Fatal("invalid qualification borrowed candidate")
		}
	})
}
func TestPiLifecycleCurrentScopeAndPrivacy(t *testing.T) {
	t.Run("PPI-V0-006 non-Stop privacy and after-read guards", func(t *testing.T) {
		for _, mode := range []string{"valid", "root-drift", "runtime-drift", "legacy-root", "codex-root", "revoked"} {
			t.Run(mode, func(t *testing.T) {
				root, files := lifecycleFixture(t)
				root.Profile = PiRootProfile
				root.HostQualification = nil
				root.PiQualification = piQualificationFixture()
				root.Revoked = mode == "revoked"
				files.data["accepted-root.json"] = encode(t, root)
				t.Chdir(root.RepositoryRoot)
				checks, observations := 0, 0
				check := func(context.Context, RootDocument, *qualificationCampaign) error {
					checks++
					if mode == "runtime-drift" && checks == 2 {
						return errUnavailable
					}
					return nil
				}
				observe := func(_ context.Context, s LifecycleScope) error {
					observations++
					if !s.Pi || s.Direct || len(s.QualifiedSurfaces) != 2 {
						t.Fatal("wrong scope")
					}
					if mode == "root-drift" {
						files.data["accepted-root.json"] = append(files.data["accepted-root.json"], ' ')
					}
					return nil
				}
				profile := lifecycleProfile(PiRootProfile)
				if mode == "legacy-root" {
					profile = RootProfile
				}
				if mode == "codex-root" {
					profile = DirectRootProfile
				}
				result, err := resolveLifecycleContextProfile(context.Background(), files, observe, check, profile)
				if (err == nil) != (mode == "valid") {
					t.Fatalf("%s: %v", mode, err)
				}
				if mode == "valid" && (checks != 2 || observations != 1 || !result.Scope.Pi || result.Stop.RootCurrent) {
					t.Fatal("bad bracketing")
				}
				for _, name := range files.reads {
					if name != "accepted-root.json" && name != "minimum-generation.json" && name != "execution-policy.json" {
						t.Fatal("non-Stop read publication/private evidence")
					}
				}
			})
		}
	})
}
