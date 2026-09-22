package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/authorityevent"
	"github.com/Beamfall/corvint/internal/authoritystore"
	"github.com/Beamfall/corvint/internal/localcompletion"
)

func directTestScope(candidate bool) authoritystore.LifecycleScope {
	s := qualifiedTestScope("/test-only")
	s.Direct = true
	s.HostSHA256 = s.AppSHA256
	s.RuntimeAdmissionEvidenceSHA256 = s.EngineSHA256
	s.AppSHA256 = ""
	s.EngineSHA256 = ""
	s.SupportScope = "qualified-direct-native-runtime"
	s.QualifiedSurfaces = s.QualifiedSurfaces[1:]
	if candidate {
		s.SupportScope = "candidate-direct-native-runtime"
		s.QualifiedHostSHA256 = ""
		s.EvidenceSHA256 = ""
		s.QualifiedSurfaces = []authoritystore.SurfaceQualification{}
	}
	return s
}
func directTestEnvelope(event string, candidate bool) map[string]any {
	s := directTestScope(candidate)
	result := qualifiedEnvelope(options{event: event}, map[string]any{}, qualifiedTestRepo(), localcompletion.Evaluation{Lifecycle: "inactive"}, s)
	if event == "session-start" || event == "user-prompt" {
		result["context"] = map[string]any{"profile": "corvint-dogfood-prompt/0", "test_only_escaped": strings.Repeat("\"\\\n😀", 200)}
	}
	return result
}
func TestDirectQualifiedWireAndBudget(t *testing.T) {
	t.Run("DCLI-V0-004 caller asserted unattributed native output", func(t *testing.T) {
		for _, event := range []string{"session-start", "user-prompt", "session-end"} {
			c := directTestEnvelope(event, true)
			f := directTestEnvelope(event, false)
			cr, err := qualifiedLifecycleBytes(c, 8000)
			if err != nil {
				t.Fatal(err)
			}
			fr, err := qualifiedLifecycleBytes(f, 8000)
			if err != nil {
				t.Fatal(err)
			}
			cn, err := qualifiedNativeBytes(cr)
			if err != nil {
				t.Fatalf("candidate %s: %v", event, err)
			}
			fn, err := qualifiedNativeBytes(fr)
			if err != nil {
				t.Fatalf("full %s: %v", event, err)
			}
			if len(fn)-len(cn) > 512 || len(fn) > 8000 {
				t.Fatal("DCLI-V0-005 final native reservation exceeded")
			}
			t.Logf("event=%s candidate=%d full=%d expansion=%d reserve=512", event, len(cn), len(fn), len(fn)-len(cn))
			if bytes.Contains(fr, []byte(`"appSHA256"`)) || bytes.Contains(fr, []byte(`"engineSHA256"`)) || f["requestProvenance"] != "caller-asserted" || f["authority"] != "NONE" {
				t.Fatal("legacy identity or authority leak")
			}
		}
		for _, change := range []func(map[string]any){
			func(r map[string]any) { r["profile"] = qualifiedLifecycleProfile },
			func(r map[string]any) { r["qualifiedHost"].(map[string]any)["appSHA256"] = strings.Repeat("a", 64) },
			func(r map[string]any) { r["qualifiedHost"].(map[string]any)["engineSHA256"] = "" },
			func(r map[string]any) { r["qualifiedHost"].(map[string]any)["eventSurface"] = "codex-cli" },
			func(r map[string]any) {
				r["qualifiedHost"].(map[string]any)["qualifiedSurfaces"] = []map[string]any{{"surface": "codex-desktop", "evidenceSHA256": strings.Repeat("a", 64)}}
			},
			func(r map[string]any) { delete(r["qualifiedHost"].(map[string]any), "runtimeAdmissionEvidenceSHA256") },
			func(r map[string]any) { r["qualifiedHost"].(map[string]any)["host"] = "claude" },
		} {
			r := directTestEnvelope("session-end", false)
			change(r)
			raw, err := qualifiedLifecycleBytes(r, 8000)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = validateQualifiedResult(raw); err == nil {
				t.Fatal("mixed/false direct qualification rendered")
			}
		}
		raw := []byte(`{"profile":"corvint-qualified-lifecycle/1","event":"stop","input":{"stopHookActive":false}}`)
		if r, err := parseQualifiedRequest(raw); err != nil || r.profile != directQualifiedLifecycleProfile {
			t.Fatal("direct normalized request refused")
		}
		var object map[string]any
		json.Unmarshal(raw, &object)
		object["host"] = "codex"
		bad, _ := json.Marshal(object)
		if _, err := parseQualifiedRequest(bad); err == nil {
			t.Fatal("caller host accepted")
		}

	})
}
func TestDirectStopRemainsSeparateAndRecursive(t *testing.T) {
	for _, candidate := range []bool{true, false} {
		for _, recursive := range []bool{true, false} {
			s := directTestScope(candidate)
			input := map[string]any{"stopHookActive": recursive}
			r := qualifiedEnvelope(options{event: "stop"}, input, qualifiedTestRepo(), localcompletion.Evaluation{Lifecycle: "active"}, s)
			stop := authorityevent.Resolution{RootCurrent: true, State: "OPEN", UniverseSHA256: strings.Repeat("f", 64), QualifiedHostSHA256: s.QualifiedHostSHA256, RemediationAllowed: true}
			if candidate {
				stop.Exercise = &authorityevent.QualificationExercise{CampaignID: strings.Repeat("e", 64), StopPermitted: true}
			}
			if err := qualifiedStop(r, input, authoritystore.LifecycleResolution{Scope: s, Stop: stop}); err != nil {
				t.Fatal(err)
			}
			raw, err := qualifiedLifecycleBytes(r, 8000)
			if err != nil {
				t.Fatal(err)
			}
			native, err := qualifiedNativeBytes(raw)
			if err != nil || len(native) > 8000 {
				t.Fatal("direct Stop output", err)
			}
			if bytes.Contains(native, []byte(`"decision":"block"`)) == recursive {
				t.Fatal("recursive/native decision mismatch")
			}
		}
	}
	// A requested wire version cannot borrow the other profile even through an
	// internal observer seam. No public root or resolver substitution is added.
	resolve := func(ctx context.Context, stop bool, observe authoritystore.LifecycleObserver) (authoritystore.LifecycleResolution, error) {
		return authoritystore.LifecycleResolution{}, observe(ctx, qualifiedTestScope("/test-only"))
	}
	if _, err := qualifiedLifecycle(context.Background(), qualifiedRequest{profile: directQualifiedLifecycleProfile, event: "session-end", input: map[string]any{}}, resolve); err == nil {
		t.Fatal("cross-profile scope accepted")
	}
	var out bytes.Buffer
	if runNativeHook(context.Background(), []string{"--qualified-direct-lifecycle"}, strings.NewReader(`{}`), &out, &bytes.Buffer{}) != 0 || bytes.Contains(out.Bytes(), []byte(`"decision":"block"`)) {
		t.Fatal("unadmitted native direct mode blocked")
	}
}
