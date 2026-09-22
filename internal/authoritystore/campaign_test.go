package authoritystore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/localauthority"
)

type campaignFiles struct {
	raw   []byte
	reads int
	fail  bool
}

func (f *campaignFiles) read(path string, owner uint32, limit int) ([]byte, error) {
	f.reads++
	if f.fail || path != campaignPath || owner != 0 || limit != maxCampaign {
		return nil, errors.New("test-only-unavailable")
	}
	return append([]byte(nil), f.raw...), nil
}
func campaignFixture(t *testing.T) (RootDocument, []byte, qualificationCampaign, time.Time) {
	t.Helper()
	root, floor := rootFixture()
	root.Consumer = Image{"/Library/CorvintAuthority/versions/" + strings.Repeat("a", 64) + "/corvint", strings.Repeat("b", 64)}
	root.Adapter = Image{"/Library/CorvintAuthority/versions/" + strings.Repeat("a", 64) + "/authority-hook.json", strings.Repeat("c", 64)}
	root.Git = Image{"/Library/CorvintAuthority/versions/" + strings.Repeat("a", 64) + "/bin/git", strings.Repeat("d", 64)}
	now := time.Date(2026, 9, 8, 12, 0, 1, 0, time.UTC)
	floorRaw := encode(t, floor)
	c := qualificationCampaign{Profile: campaignProfile, Status: "NATIVE_QUALIFICATION_ONLY", CampaignID: strings.Repeat("e", 64), IssuedAt: now.Add(-time.Second).Format(time.RFC3339), ExpiresAt: now.Add(899 * time.Second).Format(time.RFC3339), RootID: root.RootID, Epoch: root.Epoch, Generation: root.Generation, RepositoryID: root.RepositoryID, RepositoryRoot: root.RepositoryRoot, PolicySHA256: root.PolicySHA256, MinimumGenerationSHA256: localauthority.BytesDigest(floorRaw), EnrollmentHandle: strings.Repeat("f", 64), Target: strings.Repeat("a", 40), Consumer: root.Consumer, Adapter: root.Adapter, Git: root.Git, Scope: "ALL_NATIVE_STOPS_IN_EXACT_REPOSITORY", Runtime: candidateRuntime{Topology: "app-owned-stdio", BootSessionUUID: "test-only-boot", AppInstance: ProcessInstance{PID: 101, Started: 1, StartedUsec: 2}, EngineInstance: ProcessInstance{PID: 102, Started: 1, StartedUsec: 3}, App: Image{"/Applications/TestOnly.app/app", strings.Repeat("a", 64)}, Engine: Image{"/Applications/TestOnly.app/engine", strings.Repeat("b", 64)}, AppCDHash: strings.Repeat("a", 40), EngineCDHash: strings.Repeat("b", 40), OSBuild: "test-only-build", Architecture: "arm64"}}
	return root, floorRaw, c, now
}
func TestCampaignClosedSchemaAndBinding(t *testing.T) {
	t.Run("PLE-V0-012 campaign parser binds finite scope and identities", func(t *testing.T) {
		root, floor, c, now := campaignFixture(t)
		var decoded qualificationCampaign
		if err := localauthority.Decode(encode(t, c), &decoded); err != nil || !decoded.valid(root, floor, c.EnrollmentHandle, now) {
			t.Fatalf("test fixture rejected: %v", err)
		}
		changes := map[string]func(*qualificationCampaign){
			"profile":               func(c *qualificationCampaign) { c.Profile = "corvint-protected-root/0" },
			"status":                func(c *qualificationCampaign) { c.Status = "FULL" },
			"scope":                 func(c *qualificationCampaign) { c.Scope = "ONE_AUTHENTICATED_TASK" },
			"id":                    func(c *qualificationCampaign) { c.CampaignID = "caller" },
			"future":                func(c *qualificationCampaign) { c.IssuedAt = now.Add(time.Second).Format(time.RFC3339) },
			"expired":               func(c *qualificationCampaign) { c.ExpiresAt = now.Format(time.RFC3339) },
			"duration":              func(c *qualificationCampaign) { c.ExpiresAt = now.Add(900 * time.Second).Format(time.RFC3339) },
			"fractional time":       func(c *qualificationCampaign) { c.IssuedAt = "2026-09-08T12:00:00.1Z" },
			"offset time":           func(c *qualificationCampaign) { c.IssuedAt = "2026-09-08T12:00:00+00:00" },
			"root":                  func(c *qualificationCampaign) { c.RootID += "-different" },
			"epoch":                 func(c *qualificationCampaign) { c.Epoch = "2" },
			"generation":            func(c *qualificationCampaign) { c.Generation = "3" },
			"policy":                func(c *qualificationCampaign) { c.PolicySHA256 = strings.Repeat("c", 64) },
			"floor":                 func(c *qualificationCampaign) { c.MinimumGenerationSHA256 = strings.Repeat("c", 64) },
			"repository id":         func(c *qualificationCampaign) { c.RepositoryID = strings.Repeat("c", 64) },
			"repository path":       func(c *qualificationCampaign) { c.RepositoryRoot = "/another" },
			"handle":                func(c *qualificationCampaign) { c.EnrollmentHandle = strings.Repeat("c", 64) },
			"target":                func(c *qualificationCampaign) { c.Target = "HEAD" },
			"consumer":              func(c *qualificationCampaign) { c.Consumer.SHA256 = strings.Repeat("c", 64) },
			"adapter":               func(c *qualificationCampaign) { c.Adapter.Path += "-different" },
			"git":                   func(c *qualificationCampaign) { c.Git.Path += "-different" },
			"engine pid":            func(c *qualificationCampaign) { c.Runtime.EngineInstance.PID = 0 },
			"engine birth":          func(c *qualificationCampaign) { c.Runtime.EngineInstance.Started = 0 },
			"microseconds":          func(c *qualificationCampaign) { c.Runtime.EngineInstance.StartedUsec = 1000000 },
			"same process":          func(c *qualificationCampaign) { c.Runtime.EngineInstance = c.Runtime.AppInstance },
			"topology":              func(c *qualificationCampaign) { c.Runtime.Topology = "unverified-proxy" },
			"unexpected socket":     func(c *qualificationCampaign) { c.Runtime.ControlSocket = "/tmp/socket" },
			"missing shared socket": func(c *qualificationCampaign) { c.Runtime.Topology = "shared-daemon" },
		}
		for name, change := range changes {
			t.Run(name, func(t *testing.T) {
				bad := c
				change(&bad)
				if bad.valid(root, floor, c.EnrollmentHandle, now) {
					t.Fatal("mismatched admission accepted")
				}
			})
		}
		var fields map[string]any
		if err := json.Unmarshal(encode(t, c), &fields); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"allowStopRemediation", "engineInstance", "issuedAt", "scope"} {
			t.Run("missing-"+name, func(t *testing.T) {
				var object map[string]any
				json.Unmarshal(encode(t, c), &object)
				if name == "engineInstance" {
					delete(object["runtime"].(map[string]any), name)
				} else {
					delete(object, name)
				}
				if localauthority.Decode(encode(t, object), &qualificationCampaign{}) == nil {
					t.Fatal("missing field accepted")
				}
			})
		}
		fields["allowStopRemediation"] = nil
		if localauthority.Decode(encode(t, fields), &qualificationCampaign{}) == nil {
			t.Fatal("null permission accepted")
		}
		fields["allowStopRemediation"] = false
		fields["qualifiedHostSHA256"] = strings.Repeat("a", 64)
		if localauthority.Decode(encode(t, fields), &qualificationCampaign{}) == nil {
			t.Fatal("candidate accepted completed evidence")
		}
		raw := encode(t, c)
		duplicate := bytes.Replace(raw, []byte(`"allowStopRemediation":false`), []byte(`"allowStopRemediation":false,"allowStopRemediation":true`), 1)
		if localauthority.Decode(duplicate, &qualificationCampaign{}) == nil {
			t.Fatal("duplicate permission accepted")
		}

	})
}
func TestCampaignMutationExpiryAndRemoval(t *testing.T) {
	root, floor, c, now := campaignFixture(t)
	before := encode(t, c)
	files := &campaignFiles{raw: before}
	if !campaignStillCurrent(files, before, root, floor, c.EnrollmentHandle, c.Target, now) {
		t.Fatal("stable campaign rejected")
	}
	if campaignStillCurrent(files, before, root, floor, c.EnrollmentHandle, c.Target, now.Add(899*time.Second)) {
		t.Fatal("expiry ignored")
	}
	if campaignStillCurrent(files, before, root, append(floor, ' '), c.EnrollmentHandle, c.Target, now) {
		t.Fatal("floor drift ignored")
	}
	if campaignStillCurrent(files, before, root, floor, c.EnrollmentHandle, strings.Repeat("b", 40), now) {
		t.Fatal("target swap ignored")
	}
	c.AllowStopRemediation = true
	files.raw = encode(t, c)
	if campaignStillCurrent(files, before, root, floor, c.EnrollmentHandle, c.Target, now) {
		t.Fatal("campaign mutation ignored")
	}
	files.fail = true
	if campaignStillCurrent(files, before, root, floor, c.EnrollmentHandle, c.Target, now) {
		t.Fatal("campaign removal ignored")
	}
}
func TestCampaignCannotRepairCompletedQualification(t *testing.T) {
	root, floor, c, now := campaignFixture(t)
	files := &campaignFiles{raw: encode(t, c)}
	root.HostQualification = &HostQualification{}
	if _, err := admitRuntime(context.Background(), root, files, floor, c.EnrollmentHandle, now); err == nil {
		t.Fatal("invalid completed root accepted")
	}
	if files.reads != 0 {
		t.Fatal("completed qualification fell back to campaign")
	}
	root.HostQualification = nil
	files.fail = true
	if _, err := admitRuntime(context.Background(), root, files, floor, c.EnrollmentHandle, now); err == nil {
		t.Fatal("missing campaign accepted")
	}
	files.fail = false
	files.raw = bytes.Repeat([]byte("x"), maxCampaign+1)
	if _, err := admitRuntime(context.Background(), root, files, floor, c.EnrollmentHandle, now); err == nil {
		t.Fatal("oversized campaign accepted")
	}
}

func TestCampaignEngineLifetimeAndDecimalWire(t *testing.T) {
	_, _, c, _ := campaignFixture(t)
	pins := c.Runtime.pins()
	if !pins.acceptsEngine(c.Runtime.EngineInstance) {
		t.Fatal("exact engine rejected")
	}
	for _, change := range []func(*ProcessInstance){func(p *ProcessInstance) { p.PID++ }, func(p *ProcessInstance) { p.Started++ }, func(p *ProcessInstance) { p.StartedUsec++ }} {
		different := c.Runtime.EngineInstance
		change(&different)
		if pins.acceptsEngine(different) {
			t.Fatal("restarted/reused engine accepted")
		}
	}
	normal := HostQualification{Profile: "corvint-native-qualified-host/0", Surface: "codex-desktop", AppInstance: c.Runtime.AppInstance}
	raw := encode(t, normal)
	var again HostQualification
	if err := localauthority.Decode(raw, &again); err != nil || again.AppInstance != normal.AppInstance {
		t.Fatalf("completed host wire: %v", err)
	}
	if !bytes.Contains(raw, []byte(`"pid":"101"`)) {
		t.Fatal("process identity was not decimal string")
	}
	for _, replacement := range []string{`"pid":101`, `"pid":"0101"`, `"pid":null`, `"pid":"1e2"`, `"pid":"-1"`} {
		bad := bytes.Replace(raw, []byte(`"pid":"101"`), []byte(replacement), 1)
		if localauthority.Decode(bad, &HostQualification{}) == nil {
			t.Fatal("noncanonical process wire accepted")
		}
	}
}

func TestCampaignProcessWireWidthsAndMissingFields(t *testing.T) {
	_, _, c, _ := campaignFixture(t)
	for _, pin := range []ProcessInstance{{PID: 2, Started: 1, StartedUsec: 0}, {PID: ^uint32(0), Started: ^uint64(0), StartedUsec: 999999}} {
		c.Runtime.AppInstance = pin
		c.Runtime.EngineInstance = pin
		raw := encode(t, c)
		var again qualificationCampaign
		if err := localauthority.Decode(raw, &again); err != nil || again.Runtime.AppInstance != pin || again.Runtime.EngineInstance != pin {
			t.Fatalf("candidate widths: %v", err)
		}
		normal := HostQualification{AppInstance: pin}
		var roundtrip HostQualification
		if err := localauthority.Decode(encode(t, normal), &roundtrip); err != nil || roundtrip.AppInstance != pin {
			t.Fatalf("normal widths: %v", err)
		}
	}
	raw := encode(t, HostQualification{AppInstance: ProcessInstance{PID: 101, Started: 2, StartedUsec: 3}})
	for _, fragment := range []string{`"pid":"101",`, `"started":"2",`, `"startedUsec":"3"`} {
		bad := bytes.Replace(raw, []byte(fragment), nil, 1)
		if localauthority.Decode(bad, &HostQualification{}) == nil {
			t.Fatal("missing birth field accepted")
		}
	}
}
