package authorityevent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestQualificationExerciseUsesNativeDecisionWithoutFull(t *testing.T) {
	t.Run("PLE-V0-012 exercise decisions remain fallback unqualified", func(t *testing.T) {
		for _, state := range []string{"OPEN", "EMPTY"} {
			for _, active := range []bool{false, true} {
				for _, permitted := range []bool{false, true} {
					event := Event{Profile: Profile, Event: "stop", EnrollmentHandle: strings.Repeat("a", 64), StopHookActive: active}
					exercise := Resolution{State: state, UniverseSHA256: strings.Repeat("b", 64), RootCurrent: true, Exercise: &QualificationExercise{CampaignID: strings.Repeat("c", 64), StopPermitted: permitted}}
					qualified := Resolution{State: state, UniverseSHA256: exercise.UniverseSHA256, RootCurrent: true, QualifiedHostSHA256: strings.Repeat("d", 64), RemediationAllowed: permitted}
					got := Handle(context.Background(), event, func(context.Context, string) (Resolution, error) { return exercise, nil })
					normal := Handle(context.Background(), event, func(context.Context, string) (Resolution, error) { return qualified, nil })
					if got.Support != "FALLBACK" || got.Qualification != "UNQUALIFIED" || got.Authority != "VERIFIED" || got.QualifiedHostSHA256 != "" || len(got.QualifiedSurfaces) != 0 || got.Decision != normal.Decision || got.State != normal.State {
						t.Fatalf("exercise overclaim/parity: %+v", got)
					}
					output := NativeOutput(got)
					encoded, _ := json.Marshal(output)
					if !strings.Contains(string(encoded), "UNQUALIFIED native qualification exercise") || strings.Contains(string(encoded), "FULL") {
						t.Fatalf("missing unqualified native text: %s", encoded)
					}
					if (output["decision"] == "block") != (normal.Decision == "block") {
						t.Fatal("native decision diverged")
					}
				}
			}
		}

	})
}
func TestQualificationExerciseRejectsAmbiguousOrUnverifiedResolution(t *testing.T) {
	event := Event{Profile: Profile, Event: "stop", EnrollmentHandle: strings.Repeat("a", 64)}
	good := Resolution{State: "OPEN", UniverseSHA256: strings.Repeat("b", 64), RootCurrent: true, Exercise: &QualificationExercise{CampaignID: strings.Repeat("c", 64), StopPermitted: true}}
	for _, change := range []func(*Resolution){func(r *Resolution) { r.RootCurrent = false }, func(r *Resolution) { r.State = "UNKNOWN" }, func(r *Resolution) { r.UniverseSHA256 = "" }, func(r *Resolution) { r.QualifiedHostSHA256 = strings.Repeat("d", 64) }, func(r *Resolution) { r.SupportScope = "qualified-shared-runtime" }, func(r *Resolution) { r.QualifiedSurfaces = []string{"codex-desktop"} }, func(r *Resolution) { r.Exercise = &QualificationExercise{CampaignID: "caller", StopPermitted: true} }} {
		bad := good
		change(&bad)
		got := Handle(context.Background(), event, func(context.Context, string) (Resolution, error) { return bad, nil })
		if got.Decision != "release" || got.State != "UNKNOWN" || got.Support == "FULL" || NativeOutput(got)["decision"] == "block" {
			t.Fatalf("invalid exercise admitted: %+v", got)
		}
	}
	raw, _ := json.Marshal(event)
	for _, field := range []string{`"campaignId":"caller"`, `"qualification":"UNQUALIFIED"`, `"allowStopRemediation":true`, `"cwd":"/fixture"`} {
		attempted := append(append([]byte(nil), raw[:len(raw)-1]...), []byte(","+field+"}")...)
		if _, err := Parse(attempted); err == nil {
			t.Fatal("public event created campaign grant")
		}
	}
}
