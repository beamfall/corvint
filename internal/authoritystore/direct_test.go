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

func directFixture() DirectRuntime {
	return DirectRuntime{Topology: "direct-native-cli", Host: "codex", Surface: "codex-cli", BootSessionUUID: "12345678-1234-1234-1234-123456789ABC", HostInstance: ProcessInstance{PID: 321, Started: 10, StartedUsec: 20}, HostImage: Image{Path: "/Applications/test-only-codex", SHA256: strings.Repeat("a", 64)}, HostCDHash: strings.Repeat("b", 40), RuntimeAdmissionEvidenceSHA256: strings.Repeat("c", 64), ParentPolicy: "immediate-host", OSBuild: "test-only-build", Architecture: "arm64"}
}
func TestDirectClosedWire(t *testing.T) {
	t.Run("DCLI-V0-001 closed direct host identity", func(t *testing.T) {
		root, floor := rootFixture()
		root.Profile = DirectRootProfile
		root.DirectQualification = &DirectQualification{Profile: directQualificationProfile, EvidenceSHA256: strings.Repeat("d", 64), Runtime: directFixture()}
		raw := encode(t, root)
		decoded, _, _, err := decodeRoot(raw, encode(t, floor))
		if err != nil || decoded.DirectQualification == nil || decoded.HostQualification != nil || !decoded.DirectQualification.valid() {
			t.Fatalf("DCLI-V0-001 direct root: %v", err)
		}
		if bytes.Contains(raw, []byte(`"app"`)) || bytes.Contains(raw, []byte(`"engine"`)) {
			t.Fatal("legacy aliases in direct wire")
		}
		for _, name := range []string{"app", "engine", "hostInstance", "callerHost"} {
			var fields map[string]any
			json.Unmarshal(raw, &fields)
			fields["hostQualification"].(map[string]any)[name] = map[string]any{}
			var r RootDocument
			if localauthority.Decode(encode(t, fields), &r) == nil {
				t.Fatalf("accepted extra %s", name)
			}
		}
		for _, name := range []string{"host", "surface", "hostInstance", "hostImage", "hostCDHash", "runtimeAdmissionEvidenceSHA256", "parentPolicy", "bootSessionUUID"} {
			var fields map[string]any
			json.Unmarshal(raw, &fields)
			delete(fields["hostQualification"].(map[string]any)["runtime"].(map[string]any), name)
			var r RootDocument
			if localauthority.Decode(encode(t, fields), &r) == nil {
				t.Fatalf("accepted absent %s", name)
			}
		}
		for _, bad := range [][]byte{
			bytes.Replace(raw, []byte(DirectRootProfile), []byte(RootProfile), 1),
			bytes.Replace(raw, []byte(`"host":"codex"`), []byte(`"host":"codex","host":"codex"`), 1),
		} {
			if _, _, _, err := decodeRoot(bad, encode(t, floor)); err == nil {
				t.Fatal("mixed or duplicate profile accepted")
			}
		}
		for name, mutate := range map[string]func(*DirectRuntime){
			"other-host": func(r *DirectRuntime) { r.Host = "claude" }, "desktop": func(r *DirectRuntime) { r.Surface = "codex-desktop" }, "bridge": func(r *DirectRuntime) { r.ParentPolicy = "ancestor" }, "topology": func(r *DirectRuntime) { r.Topology = "app-owned-stdio" }, "pid": func(r *DirectRuntime) { r.HostInstance.PID = 1 }, "birth": func(r *DirectRuntime) { r.HostInstance.Started = 0 }, "usec": func(r *DirectRuntime) { r.HostInstance.StartedUsec = 1000000 }, "boot": func(r *DirectRuntime) { r.BootSessionUUID = "invented" }, "arch": func(r *DirectRuntime) { r.Architecture = "amd64" }, "path": func(r *DirectRuntime) { r.HostImage.Path = "/tmp/../codex" }, "hash": func(r *DirectRuntime) { r.HostImage.SHA256 = "" }, "audit": func(r *DirectRuntime) { r.RuntimeAdmissionEvidenceSHA256 = "" }, "cdhash": func(r *DirectRuntime) { r.HostCDHash = strings.Repeat("A", 40) },
		} {
			t.Run(name, func(t *testing.T) {
				r := directFixture()
				mutate(&r)
				if r.valid() {
					t.Fatal("DCLI-V0-006 invalid pin accepted")
				}
			})
		}

	})
}
func TestDirectCampaignBindingsAndNoCompletedFallback(t *testing.T) {
	root, floor, c, now := campaignFixture(t)
	root.Profile = DirectRootProfile
	c.Profile = directCampaignProfile
	c.Scope = lifecycleCampaignScope
	r := directFixture()
	c.DirectRuntime = &r
	c.Runtime = candidateRuntime{}
	raw := encode(t, c)
	var decoded qualificationCampaign
	if localauthority.Decode(raw, &decoded) != nil || !decoded.validScope(root, floor, c.EnrollmentHandle, now, lifecycleCampaignScope) {
		t.Fatal("direct campaign refused")
	}
	for name, mutate := range map[string]func(*qualificationCampaign){
		"legacy-scope": func(c *qualificationCampaign) { c.Scope = stopCampaignScope }, "legacy-profile": func(c *qualificationCampaign) { c.Profile = campaignProfile }, "expired": func(c *qualificationCampaign) { c.ExpiresAt = now.Format(time.RFC3339) }, "duration": func(c *qualificationCampaign) { c.ExpiresAt = now.Add(900 * time.Second).Format(time.RFC3339) }, "floor": func(c *qualificationCampaign) { c.MinimumGenerationSHA256 = strings.Repeat("1", 64) }, "target": func(c *qualificationCampaign) { c.Target = "HEAD" }, "image": func(c *qualificationCampaign) { c.Consumer.SHA256 = strings.Repeat("2", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			bad := c
			mutate(&bad)
			if bad.validScope(root, floor, c.EnrollmentHandle, now, lifecycleCampaignScope) {
				t.Fatal("campaign drift accepted")
			}
		})
	}
	legacy := root
	legacy.Profile = RootProfile
	if c.validScope(legacy, floor, c.EnrollmentHandle, now, lifecycleCampaignScope) || c.valid(root, floor, c.EnrollmentHandle, now) {
		t.Fatal("cross-version or standalone campaign accepted")
	}
	files := &campaignFiles{raw: raw}
	root.DirectQualification = &DirectQualification{}
	if _, err := admitLifecycleRuntime(context.Background(), root, files, floor, c.EnrollmentHandle, now); err == nil || files.reads != 0 {
		t.Fatal("invalid completed qualification borrowed campaign")
	}
	root.DirectQualification = nil
	admission := runtimeAdmission{campaign: &c, scope: lifecycleCampaignScope}
	scope, err := admission.lifecycleScope(root)
	if err != nil || !scope.Direct || scope.HostSHA256 != r.HostImage.SHA256 || scope.AppSHA256 != "" || scope.QualifiedHostSHA256 != "" || scope.EvidenceSHA256 != "" || len(scope.QualifiedSurfaces) != 0 {
		t.Fatal("candidate invented qualification")
	}
	root.DirectQualification = &DirectQualification{Profile: directQualificationProfile, EvidenceSHA256: strings.Repeat("d", 64), Runtime: r}
	scope, err = lifecycleScope(root)
	if err != nil || scope.QualifiedHostSHA256 == "" || len(scope.QualifiedSurfaces) != 1 || scope.QualifiedSurfaces[0].Surface != "codex-cli" {
		t.Fatal("completed projection failed")
	}
}

func TestDirectLifecycleProfileIsolationAndCurrentness(t *testing.T) {
	for _, mode := range []string{"valid", "legacy-context", "legacy-stop", "runtime-second", "root-after", "floor-after", "policy-after"} {
		t.Run(mode, func(t *testing.T) {
			root, files := lifecycleFixture(t)
			root.Profile = DirectRootProfile
			root.HostQualification = nil
			root.DirectQualification = &DirectQualification{Profile: directQualificationProfile, EvidenceSHA256: strings.Repeat("d", 64), Runtime: directFixture()}
			files.data["accepted-root.json"] = encode(t, root)
			t.Chdir(root.RepositoryRoot)
			checks, observations := 0, 0
			check := func(context.Context, RootDocument, *qualificationCampaign) error {
				checks++
				if mode == "runtime-second" && checks == 2 {
					return errUnavailable
				}
				return nil
			}
			observe := func(ctx context.Context, s LifecycleScope) error {
				observations++
				if !s.Direct || s.AppSHA256 != "" || s.HostSHA256 == "" {
					t.Fatal("wrong direct scope")
				}
				names := map[string]string{"root-after": "accepted-root.json", "floor-after": "minimum-generation.json", "policy-after": "execution-policy.json"}
				if name := names[mode]; name != "" {
					files.data[name] = append(files.data[name], ' ')
				}
				return nil
			}
			if mode == "legacy-stop" {
				if _, err := resolveObserved(context.Background(), "", files, time.Now(), observe); err == nil {
					t.Fatal("standalone/legacy Stop entered direct root")
				}
				if checks != 0 || observations != 0 || len(files.reads) != 2 {
					t.Fatal("legacy Stop read beyond version guard")
				}
				return
			}
			profile := lifecycleProfile(DirectRootProfile)
			if mode == "legacy-context" {
				profile = RootProfile
			}
			result, err := resolveLifecycleContextProfile(context.Background(), files, observe, check, profile)
			if (err == nil) != (mode == "valid") {
				t.Fatalf("%s: %v", mode, err)
			}
			if mode == "valid" && (checks != 2 || observations != 1 || !result.Scope.Direct || result.Stop.RootCurrent) {
				t.Fatal("direct context did not bracket exactly once")
			}
			for _, name := range files.reads {
				if name != "accepted-root.json" && name != "minimum-generation.json" && name != "execution-policy.json" {
					t.Fatal("non-Stop read protected publication")
				}
			}
		})
	}
}
