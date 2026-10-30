package main

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/authorityevent"
	"github.com/Beamfall/corvint/internal/authoritystore"
	"github.com/Beamfall/corvint/internal/localcompletion"
)

func qualifiedPiTestScope(root string, candidate bool) authoritystore.LifecycleScope {
	s := qualifiedTestScope(root)
	s.Pi = true
	s.AppSHA256 = ""
	s.EngineSHA256 = ""
	s.HostSHA256 = strings.Repeat("b", 64)
	s.RuntimeAdmissionEvidenceSHA256 = strings.Repeat("c", 64)
	s.SupportScope = "qualified-protected-pi-runtime"
	s.QualifiedSurfaces = []authoritystore.SurfaceQualification{{Surface: "pi-tui", EvidenceSHA256: strings.Repeat("d", 64)}, {Surface: "pi-rpc", EvidenceSHA256: strings.Repeat("e", 64)}}
	if candidate {
		s.QualifiedHostSHA256 = ""
		s.EvidenceSHA256 = ""
		s.QualifiedSurfaces = []authoritystore.SurfaceQualification{}
		s.SupportScope = "candidate-protected-pi-runtime"
	}
	return s
}
func TestPiQualifiedLifecycleWireAndBudget(t *testing.T) {
	t.Run("PPI-V0-005 closed Pi host and profile", func(t *testing.T) {
		request := `{"profile":"corvint-qualified-lifecycle/2","event":"stop","input":{"stopHookActive":false}}`
		if _, err := parseQualifiedRequest([]byte(request)); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"host", "root", "surface", "qualifiedHost", "enrollmentHandle"} {
			raw := strings.Replace(request, `"event":`, `"`+key+`":"caller","event":`, 1)
			if _, err := parseQualifiedRequest([]byte(raw)); err == nil {
				t.Fatal("caller scope accepted")
			}
		}
		root := queryCLIRepository(t)
		for _, candidate := range []bool{false, true} {
			for _, event := range []string{"session-start", "user-prompt", "session-end"} {
				input := map[string]any{}
				if event == "session-start" {
					input["startSource"] = "compact"
				}
				if event == "user-prompt" {
					input["task"] = "AGENTS.md"
				}
				scope := qualifiedPiTestScope(root, candidate)
				calls := 0
				resolver := func(ctx context.Context, stop bool, observe authoritystore.LifecycleObserver) (authoritystore.LifecycleResolution, error) {
					calls++
					if stop {
						t.Fatal("non-Stop requested Frontier")
					}
					return authoritystore.LifecycleResolution{Scope: scope}, observe(ctx, scope)
				}
				result, err := qualifiedLifecycle(context.Background(), qualifiedRequest{profile: piQualifiedLifecycleProfile, event: event, input: input}, resolver)
				if err != nil || calls != 1 {
					t.Fatal(err)
				}
				encoded, err := qualifiedLifecycleBytes(result, 8000)
				if err != nil {
					t.Fatal(err)
				}
				parsed, err := validateQualifiedResult(encoded)
				if err != nil || parsed.QualifiedHost.Host != "pi" || parsed.QualifiedHost.EventSurface != "unattributed" || parsed.Authority != "NONE" {
					t.Fatal("invalid Pi result", err)
				}
				if candidate != (parsed.Support == "FALLBACK") {
					t.Fatal("candidate promoted")
				}
				if native, err := qualifiedNativeBytes(encoded); err != nil || len(native) > 8000 {
					t.Fatal("native budget", err)
				}
				scope.Pi = false
				if _, err := qualifiedLifecycle(context.Background(), qualifiedRequest{profile: piQualifiedLifecycleProfile, event: event, input: input}, resolver); err == nil {
					t.Fatal("cross profile scope accepted")
				}
			}
		}
	})
}
func TestPiQualifiedStopComposition(t *testing.T) {
	t.Run("PPI-V0-006 EMPTY preserves local policy and recursive OPEN releases", func(t *testing.T) {
		for _, state := range []string{"OPEN", "EMPTY"} {
			for _, recursive := range []bool{false, true} {
				for _, candidate := range []bool{false, true} {
					scope := qualifiedPiTestScope("/test-only", candidate)
					input := map[string]any{"stopHookActive": recursive}
					result := qualifiedEnvelope(options{event: "stop"}, input, qualifiedTestRepo(), localcompletion.Evaluation{Lifecycle: "active"}, scope)
					originalLocal := result["completion"].(map[string]any)["decision"]
					stop := authorityevent.Resolution{RootCurrent: true, State: state, UniverseSHA256: strings.Repeat("f", 64), QualifiedHostSHA256: scope.QualifiedHostSHA256, RemediationAllowed: true}
					if candidate {
						stop.Exercise = &authorityevent.QualificationExercise{CampaignID: strings.Repeat("e", 64), StopPermitted: true}
					}
					if err := qualifiedStop(result, input, authoritystore.LifecycleResolution{Scope: scope, Stop: stop}); err != nil {
						t.Fatal(err)
					}
					if (result["decision"].(map[string]any)["decision"] == "block") == recursive {
						t.Fatal("wrong bounded composition")
					}
					if result["completion"].(map[string]any)["decision"] != originalLocal || !recursive && originalLocal != "block" {
						t.Fatal("local policy erased")
					}
					encoded, err := qualifiedLifecycleBytes(result, 8000)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := validateQualifiedResult(encoded); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	})
}
