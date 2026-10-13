package tcq

import (
	"errors"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// stableSeam is a scripted stable upstream verifier: it returns exactly the
// binding or error it was given and records that it was called.
type stableSeam struct {
	binding StableBinding
	err     error
	calls   int
}

func (seam *stableSeam) VerifyStableOCM(_, _ []byte, _, _ string) (StableBinding, error) {
	seam.calls++
	return seam.binding, seam.err
}

const (
	stableTestBase   = "1111111111111111111111111111111111111111"
	stableTestTarget = "2222222222222222222222222222222222222222"
)

var stableTestCEM = []byte(`{"spec":"cem/1.0"}` + "\n")

func stableTestOCM(mapSHA256, patchSHA256 string) []byte {
	return canonicalJSON(jsonObject(
		member{"cem", jsonObject(member{"mapSha256", jsonString(mapSHA256)}, member{"patchSha256", jsonString(patchSHA256)})},
		member{"claims", jsonArray(nil)},
		member{"obligations", jsonArray(nil)},
		member{"spec", jsonString(OCMSpec)},
		member{"targetRevision", jsonString(stableTestTarget)},
	))
}

func stableTestBinding() StableBinding {
	return StableBinding{
		BaseRevision: stableTestBase, TargetRevision: stableTestTarget,
		MapSha256: sha256Hex(stableTestCEM), PatchSha256: strings.Repeat("a", 64),
	}
}

func stableCode(err error) string {
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Code
	}
	return ""
}

// TestCEMV1122StableEnvelope (CEM-V1-122): the stable result carries its own
// profile, the accepted map and patch digests and a stable-domain identity
// that differs from the frozen tcq/0 identity of the same computation, and a
// cached result verifies byte for byte.
func TestCEMV1122StableEnvelope(t *testing.T) {
	binding := stableTestBinding()
	request := Request{CEM: stableTestCEM, OCM: stableTestOCM(binding.MapSha256, binding.PatchSha256), ExpectedBase: stableTestBase, Target: stableTestTarget}
	result, err := EvaluateStable(nil, &stableSeam{binding: binding}, request)
	if err != nil {
		t.Fatal(err)
	}
	value, err := wire.Parse(result.Raw())
	if err != nil {
		t.Fatal(err)
	}
	if profile, _ := value.Obj.Get("profile"); profile.Str != StableProfile {
		t.Fatalf("profile = %q", profile.Str)
	}
	cem, _ := value.Obj.Get("cem")
	if spec, _ := cem.Obj.Get("spec"); spec.Str != wire.StableSpec {
		t.Fatalf("cem.spec = %q", spec.Str)
	}
	body := withoutMember(value, "id")
	if result.ID() != stableTCQPrefix+domainHash(domainTCQStable, canonicalValue(body)) {
		t.Fatal("stable identity does not rebind from the stable body")
	}
	frozen := withoutMember(body, "cem")
	frozen.Obj.Values["profile"] = jsonString(Profile)
	if result.ID() == tcqPrefix+domainHash(domainTCQ, canonicalValue(frozen)) {
		t.Fatal("stable identity collides with the tcq/0 identity")
	}
	request.Expected = result.Raw()
	if _, err := EvaluateStable(nil, &stableSeam{binding: binding}, request); err != nil {
		t.Fatalf("cached stable result did not verify: %v", err)
	}
	request.Expected = append(result.Raw()[:len(result.Raw())-1:len(result.Raw())-1], ' ', '\n')
	if _, err := EvaluateStable(nil, &stableSeam{binding: binding}, request); stableCode(err) == "" {
		t.Fatalf("edited cached result: %v", err)
	}
}

// TestCEMV1121StableAdmissionRefusals (CEM-V1-121, CEM-V1-123): cem/0.2 never
// enters the stable route, a verifier refusal passes through with no result,
// and a binding or OCM digest that is not the exact input map refuses.
func TestCEMV1121StableAdmissionRefusals(t *testing.T) {
	binding := stableTestBinding()
	ocm := stableTestOCM(binding.MapSha256, binding.PatchSha256)
	legacy := Request{CEM: []byte(`{"spec":"cem/0.2"}` + "\n"), OCM: ocm, ExpectedBase: stableTestBase, Target: stableTestTarget}
	seam := &stableSeam{binding: binding}
	if _, err := EvaluateStable(nil, seam, legacy); stableCode(err) != CodeUnsupportedCEMProfile || seam.calls != 0 {
		t.Fatalf("cem/0.2 on the stable route: %v calls=%d", err, seam.calls)
	}
	request := Request{CEM: stableTestCEM, OCM: ocm, ExpectedBase: stableTestBase, Target: stableTestTarget}
	refused := errors.New("stable refusal")
	if result, err := EvaluateStable(nil, &stableSeam{err: refused}, request); !errors.Is(err, refused) || result.Raw() != nil {
		t.Fatalf("verifier refusal: %v", err)
	}
	wrongMap := binding
	wrongMap.MapSha256 = strings.Repeat("b", 64)
	if _, err := EvaluateStable(nil, &stableSeam{binding: wrongMap}, request); stableCode(err) != CodeInvalidTCQ {
		t.Fatalf("binding to another map: %v", err)
	}
	stale := request
	stale.OCM = stableTestOCM(strings.Repeat("c", 64), binding.PatchSha256)
	if _, err := EvaluateStable(nil, &stableSeam{binding: binding}, stale); stableCode(err) != CodeInvalidTCQ {
		t.Fatalf("OCM bound to another map: %v", err)
	}
}
