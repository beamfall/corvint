package frontier

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/lrf"
)

func baseRequest(t *testing.T) Request {
	t.Helper()
	return Request{
		CEMBytes:     []byte(validCEMBytes),
		OCMBytes:     []byte(validOCMBytes),
		ExpectedBase: oidOf("base"),
		Target:       oidOf("target"),
		Verifier:     stubVerifier{universe: scenario{}.universe(t)},
		TCQ:          &stubTCQ{result: TCQResult{ID: tcqRef("static")}},
	}
}

// CF-V0-001 and the first two adversarial-matrix rows: CEM 0.1, or OCM bound to
// CEM 0.1, is `unsupported-frontier-context` with no stdout — and it is decided
// BEFORE revision-required validation.
func TestUnsupportedProfileRejection(t *testing.T) {
	cases := []struct {
		name         string
		cem          string
		ocm          string
		expectedBase string
		target       string
	}{
		{"cem 0.1", `{"spec":"cem/0.1"}`, validOCMBytes, oidOf("base"), oidOf("target")},
		{"ocm bound to cem 0.1", validCEMBytes, `{"cem":{"spec":"cem/0.1"},"spec":"ocm/0.1-experimental"}`, oidOf("base"), oidOf("target")},
		{"unknown ocm profile", validCEMBytes, `{"spec":"ocm/0.2"}`, oidOf("base"), oidOf("target")},
		{"cem 0.1 with no expected base", `{"spec":"cem/0.1"}`, validOCMBytes, "", oidOf("target")},
		{"cem 0.1 with no target", `{"spec":"cem/0.1"}`, validOCMBytes, oidOf("base"), ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := baseRequest(t)
			request.CEMBytes = []byte(testCase.cem)
			request.OCMBytes = []byte(testCase.ocm)
			request.ExpectedBase = testCase.expectedBase
			request.Target = testCase.target
			document, encoded, err := Compute(context.Background(), request)
			if CodeOf(err) != CodeUnsupportedContext {
				t.Fatalf("code %q, want %q", CodeOf(err), CodeUnsupportedContext)
			}
			if encoded != nil || document.ID != "" {
				t.Fatal("operational failure must emit no frontier result")
			}
		})
	}
}

// CF-V0-021 stage 4: a missing revision is reported with the inherited code,
// expected base before target.
func TestMissingRevisionOrder(t *testing.T) {
	request := baseRequest(t)
	request.ExpectedBase = ""
	request.Target = ""
	if _, _, err := Compute(context.Background(), request); CodeOf(err) != "expected-base-required" {
		t.Fatalf("code %q, want expected-base-required first", CodeOf(err))
	}
	request.ExpectedBase = oidOf("base")
	if _, _, err := Compute(context.Background(), request); CodeOf(err) != "target-required" {
		t.Fatalf("code %q, want target-required", CodeOf(err))
	}
}

// CF-V0-003: the dynamic test bundle is all-or-none. Every partial combination
// fails `invalid-frontier-input`.
func TestDynamicTupleIsAllOrNone(t *testing.T) {
	present := []byte("x")
	cases := []struct {
		name                              string
		command, observation, junitReport []byte
	}{
		{"command only", present, nil, nil},
		{"observation only", nil, present, nil},
		{"report only", nil, nil, present},
		{"command and observation", present, present, nil},
		{"command and report", present, nil, present},
		{"observation and report", nil, present, present},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := baseRequest(t)
			request.Command = testCase.command
			request.Observation = testCase.observation
			request.JUnitReport = testCase.junitReport
			if _, _, err := Compute(context.Background(), request); CodeOf(err) != CodeInvalidInput {
				t.Fatalf("code %q, want %q", CodeOf(err), CodeInvalidInput)
			}
		})
	}
}

// CF-V0-003: both modes bind the recomputed TCQ ID; only the recorded mode
// differs, and neither mode upgrades authority.
func TestStaticAndDynamicModesBothBindTCQIdentity(t *testing.T) {
	cases := []struct {
		name string
		set  func(*Request)
		want TestMode
	}{
		{"static", func(*Request) {}, TestModeStatic},
		{"dynamic", func(request *Request) {
			request.Command = []byte(`{"spec":"tcq-command/0"}`)
			request.Observation = []byte(`{"spec":"tcq-observation/0"}`)
			request.JUnitReport = []byte("<testsuite/>")
		}, TestModeDynamicCallerReported},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := baseRequest(t)
			recomputer := &stubTCQ{result: TCQResult{ID: tcqRef("bound")}}
			request.TCQ = recomputer
			testCase.set(&request)
			document, _, err := Compute(context.Background(), request)
			if err != nil {
				t.Fatalf("compute failed: %q", CodeOf(err))
			}
			if document.Inputs.TestMode != testCase.want {
				t.Fatalf("test mode %s, want %s", document.Inputs.TestMode, testCase.want)
			}
			if document.Inputs.TCQID != tcqRef("bound") {
				t.Fatalf("tcq id %s, want the recomputed identity", document.Inputs.TCQID)
			}
			if recomputer.request == nil || recomputer.request.Mode != testCase.want {
				t.Fatal("the recomputer must receive the same mode the result records")
			}
		})
	}
}

// CF-V0-021 and CF-V0-022: an admitted upstream code passes through exactly,
// with no prefix or message matching, and emits no frontier result.
func TestUpstreamCodeTranslation(t *testing.T) {
	cases := []struct {
		name string
		set  func(*Request)
		want string
	}{
		{"inherited verifier code", func(request *Request) {
			request.Verifier = stubVerifier{err: cemcode.New(cemcode.BaseRevisionMismatch, "mismatch")}
		}, "base-revision-mismatch"},
		{"forged sidecar", func(request *Request) {
			request.Verifier = stubVerifier{err: cemcode.New(cemcode.ExcludedArtifactMismatch, "forged")}
		}, "excluded-artifact-mismatch"},
		{"unknown verifier code", func(request *Request) {
			request.Verifier = stubVerifier{err: codedError{value: "some-future-code"}}
		}, CodeInternalError},
		{"bare verifier error", func(request *Request) {
			request.Verifier = stubVerifier{err: errors.New("boom")}
		}, CodeInternalError},
		{"admitted tcq code", func(request *Request) {
			request.TCQ = &stubTCQ{err: codedError{value: "invalid-junit"}}
		}, "invalid-junit"},
		{"tcq resource exhaustion", func(request *Request) {
			request.TCQ = &stubTCQ{err: codedError{value: "tcq-resource-exhausted"}}
		}, CodeResourceExhausted},
		{"unknown tcq code", func(request *Request) {
			request.TCQ = &stubTCQ{err: codedError{value: "tcq-future-code"}}
		}, CodeInternalError},
		{"tcq without identity", func(request *Request) {
			request.TCQ = &stubTCQ{result: TCQResult{}}
		}, CodeInternalError},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := baseRequest(t)
			testCase.set(&request)
			_, encoded, err := Compute(context.Background(), request)
			if CodeOf(err) != testCase.want {
				t.Fatalf("code %q, want %q", CodeOf(err), testCase.want)
			}
			if encoded != nil {
				t.Fatal("operational failure must emit no frontier result")
			}
		})
	}
}

// CF-V0-022: `unsupported-lrf-context` is the only explicit LRF code admitted
// through; every other LRF failure is a Frontier internal error.
func TestLRFCodeTranslation(t *testing.T) {
	cases := []struct {
		name string
		set  func(*VerifiedUniverse)
		want string
	}{
		{"unsupported context", func(universe *VerifiedUniverse) {
			universe.LRFRequest.Context.CEMSpec = "cem/0.3"
		}, "unsupported-lrf-context"},
		{"invalid request", func(universe *VerifiedUniverse) {
			universe.LRFRequest.Context.CEMMapSHA256 = "not-a-digest"
		}, CodeInternalError},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			universe := scenario{}.universe(t)
			testCase.set(&universe)
			request := baseRequest(t)
			request.Verifier = stubVerifier{universe: universe}
			if _, _, err := Compute(context.Background(), request); CodeOf(err) != testCase.want {
				t.Fatalf("code %q, want %q", CodeOf(err), testCase.want)
			}
		})
	}
}

// CF-V0-022: an aggregate LRF `relevance-bound-exceeded` result is
// `frontier-resource-exhausted`. The issue itself is never passed through as an
// operational code.
func TestAggregateLRFBoundIsResourceExhaustion(t *testing.T) {
	hunks := make([]hunkSpec, 0, lrf.DefaultLimits().Edges+1)
	for index := 0; index <= lrf.DefaultLimits().Edges; index++ {
		name := "bound" + strings.Repeat("x", index%3) + strconv.Itoa(index)
		hunks = append(hunks, hunkSpec{
			name: name, disposition: CEMSupported, reason: "evidence-backed",
			body: "widgetregistry accepts entries", newPath: "src/" + strconv.Itoa(index) + ".go",
			bases: []string{"spec"},
		})
	}
	request := baseRequest(t)
	request.Verifier = stubVerifier{universe: scenario{
		evidence: []evidenceSpec{closingEvidence}, hunks: hunks,
	}.universe(t)}
	_, encoded, err := Compute(context.Background(), request)
	if CodeOf(err) != CodeResourceExhausted {
		t.Fatalf("code %q, want %q", CodeOf(err), CodeResourceExhausted)
	}
	if strings.Contains(string(RenderError(err)), "relevance-bound-exceeded") {
		t.Fatal("an LRF result issue must not appear in a frontier error envelope")
	}
	if encoded != nil {
		t.Fatal("operational failure must emit no frontier result")
	}
}

// CF-V0-022: a caught timeout is resource exhaustion; a caught interruption for
// which the process can still render is `frontier-interrupted`.
func TestCaughtDisruptionTranslation(t *testing.T) {
	cases := []struct {
		name string
		make func() context.Context
		want string
	}{
		{"timeout", func() context.Context {
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			cancel()
			return ctx
		}, CodeResourceExhausted},
		{"interruption", func() context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx
		}, CodeInterrupted},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, _, err := Compute(testCase.make(), baseRequest(t)); CodeOf(err) != testCase.want {
				t.Fatalf("code %q, want %q", CodeOf(err), testCase.want)
			}
		})
	}
}

// CF-V0-021 stage 2: the inherited raw byte ceilings are checked before any
// verification work.
func TestInheritedRawCeilings(t *testing.T) {
	request := baseRequest(t)
	request.CEMBytes = make([]byte, maxCEMRawBytes+1)
	if _, _, err := Compute(context.Background(), request); CodeOf(err) != "map-unavailable" {
		t.Fatalf("code %q, want map-unavailable", CodeOf(err))
	}
	request = baseRequest(t)
	request.OCMBytes = make([]byte, maxOCMRawBytes+1)
	if _, _, err := Compute(context.Background(), request); CodeOf(err) != "map-too-large" {
		t.Fatalf("code %q, want map-too-large", CodeOf(err))
	}
}

// CF-V0-001: a call missing its library seams is `invalid-frontier-input`.
func TestCallShapeRejection(t *testing.T) {
	cases := []struct {
		name string
		set  func(*Request)
	}{
		{"no verifier", func(request *Request) { request.Verifier = nil }},
		{"no tcq recomputer", func(request *Request) { request.TCQ = nil }},
		{"no cem bytes", func(request *Request) { request.CEMBytes = nil }},
		{"no ocm bytes", func(request *Request) { request.OCMBytes = nil }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := baseRequest(t)
			testCase.set(&request)
			if _, _, err := Compute(context.Background(), request); CodeOf(err) != CodeInvalidInput {
				t.Fatalf("code %q, want %q", CodeOf(err), CodeInvalidInput)
			}
		})
	}
}

// CF-V0-006: `objectFormat` is exactly sha1|sha256, so an unadmitted value
// cannot silently produce an unreproducible universe identity.
func TestUnadmittedObjectFormat(t *testing.T) {
	universe := scenario{}.universe(t)
	universe.ObjectFormat = "blake3"
	request := baseRequest(t)
	request.Verifier = stubVerifier{universe: universe}
	if _, _, err := Compute(context.Background(), request); CodeOf(err) != CodeUnsupportedContext {
		t.Fatalf("code %q, want %q", CodeOf(err), CodeUnsupportedContext)
	}
}

// CF-V0-024: neither the error envelope nor its human rendering echoes any
// unverified value. The canaries below appear in the rejected input.
func TestErrorOutputCarriesNoCanary(t *testing.T) {
	const canary = "CANARY-SECRET-VALUE"
	request := baseRequest(t)
	request.CEMBytes = []byte(`{"spec":"cem/0.1","note":"` + canary + `"}`)
	request.ExpectedBase = canary
	_, _, err := Compute(context.Background(), request)
	envelope := string(RenderError(err))
	if envelope != `{"code":"`+CodeUnsupportedContext+`","profile":"`+ErrorProfile+`"}`+"\n" {
		t.Fatalf("envelope %q is not the exact bounded CF-V0-022 object", envelope)
	}
	if strings.Contains(envelope, canary) || strings.Contains(RenderErrorHuman(err), canary) {
		t.Fatal("error output must not echo an unverified value")
	}
}
