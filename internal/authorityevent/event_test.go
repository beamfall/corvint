package authorityevent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestClosedWire(t *testing.T) {
	good := `{"profile":"corvint-authority-event/0","event":"stop","enrollmentHandle":"` + strings.Repeat("a", 64) + `","stopHookActive":false}`
	if _, err := Parse([]byte(good)); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{strings.Replace(good, `"event":"stop"`, `"event":"stop","event":"stop"`, 1), strings.Replace(good, `"event":"stop"`, `"event":"stop","root":"/tmp/fake"`, 1), strings.Replace(good, `"stopHookActive":false`, `"stopHookActive":null`, 1), strings.Replace(good, `"stopHookActive":false`, `"FULL":true`, 1), good + ` {}`, strings.Repeat(" ", 4097) + good} {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Fatalf("accepted invalid wire: %.100s", raw)
		}
	}
}

func TestUnavailableNeverBlocksOrClaimsAuthority(t *testing.T) {
	event := Event{Profile: Profile, Event: "stop", EnrollmentHandle: strings.Repeat("a", 64)}
	for _, resolve := range []Resolver{nil, func(context.Context, string) (Resolution, error) {
		return Resolution{}, errors.New("private diagnostic")
	}, func(context.Context, string) (Resolution, error) {
		return Resolution{State: "EMPTY", UniverseSHA256: strings.Repeat("b", 64)}, nil
	}} {
		got := Handle(context.Background(), event, resolve)
		if got.Authority != "NONE" || got.Support != "UNAVAILABLE" || got.Decision != "release" || got.State != "UNKNOWN" {
			t.Fatalf("overclaim: %+v", got)
		}
		if strings.Contains(got.Reason, "private") {
			t.Fatal("diagnostic leaked")
		}
	}
}

func TestComputedOpenSingleRemediationAndEmpty(t *testing.T) {
	t.Run("PLE-V0-009 one remediation releases recursion unresolved", func(t *testing.T) {
		event := Event{Profile: Profile, Event: "stop", EnrollmentHandle: strings.Repeat("a", 64)}
		r := Resolution{State: "OPEN", UniverseSHA256: strings.Repeat("b", 64), RootCurrent: true, QualifiedHostSHA256: strings.Repeat("c", 64), RemediationAllowed: true}
		resolve := func(context.Context, string) (Resolution, error) { return r, nil }
		got := Handle(context.Background(), event, resolve)
		if got.State != "OPEN" || got.Decision != "block" || got.Authority != "VERIFIED" {
			t.Fatalf("%+v", got)
		}
		event.StopHookActive = true
		if got = Handle(context.Background(), event, resolve); got.Decision != "release" || got.Reason != "continuation-limit" {
			t.Fatalf("%+v", got)
		}
		r.State = "EMPTY"
		if got = Handle(context.Background(), event, resolve); got.Decision != "release" || got.State != "EMPTY" {
			t.Fatalf("%+v", got)
		}
		r.QualifiedHostSHA256 = ""
		if got = Handle(context.Background(), event, resolve); got.State != "UNKNOWN" {
			t.Fatalf("unqualified host: %+v", got)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if got = Handle(ctx, event, resolve); got.State != "UNKNOWN" {
			t.Fatalf("cancelled: %+v", got)
		}

	})
}

func TestNativeOutputCannotBlockUnavailable(t *testing.T) {
	result := Result{Authority: "NONE", Support: "UNAVAILABLE", State: "OPEN", Decision: "block"}
	if NativeOutput(result)["decision"] != "" {
		t.Fatal("unavailable blocked")
	}
	result.Authority = "VERIFIED"
	result.Support = "FULL"
	if NativeOutput(result)["decision"] != "block" {
		t.Fatal("verified open did not block")
	}
	result.State = "EMPTY"
	if NativeOutput(result)["decision"] != "" {
		t.Fatal("empty blocked")
	}
}

func TestSharedRuntimeNeverAttributesClientSurface(t *testing.T) {
	event := Event{Profile: Profile, Event: "stop", EnrollmentHandle: strings.Repeat("a", 64)}
	result := Handle(context.Background(), event, func(context.Context, string) (Resolution, error) {
		return Resolution{State: "EMPTY", UniverseSHA256: strings.Repeat("b", 64), RootCurrent: true, QualifiedHostSHA256: strings.Repeat("c", 64), SupportScope: "qualified-shared-runtime", QualifiedSurfaces: []string{"codex-desktop"}}, nil
	})
	if result.Support != "FULL" || result.EventSurface != "unattributed" || len(result.QualifiedSurfaces) != 1 {
		t.Fatal("shared qualification scope lost")
	}
	if !strings.Contains(NativeOutput(result)["systemMessage"], "unattributed") {
		t.Fatal("native display claims exclusive surface")
	}
}
