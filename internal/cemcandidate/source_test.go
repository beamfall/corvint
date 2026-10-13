package cemcandidate

import (
	"bytes"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

func TestClosedScalarTypes(t *testing.T) {
	r, _ := fixture(t, "sha1")
	raw := encode(t, r)
	for _, change := range []struct{ old, new string }{{`"criterionIndex":0`, `"criterionIndex":null`}, {`"criterionIndex":0`, `"criterionIndex":"0"`}, {`"criterionIndex":0`, `"criterionIndex":0.0`}, {`"criterionIndex":0`, `"criterionIndex":0e0`}, {`"sourcePrefix":""`, `"sourcePrefix":null`}, {`"sourcePrefix":""`, `"sourcePrefix":true`}, {`"sourcePrefix":""`, `"SourcePrefix":""`}} {
		b := bytes.Replace(raw, []byte(change.old), []byte(change.new), 1)
		if bytes.Equal(b, raw) {
			t.Fatal("mutation missed")
		}
		var got Request
		if Decode(b, &got) == nil {
			t.Fatalf("accepted scalar mutation %s", change.new)
		}
	}
	var empty Request
	if Decode([]byte(`{}`), &empty) == nil {
		t.Fatal("missing required fields accepted")
	}
	if Decode(append(raw, []byte(` {}`)...), &empty) == nil {
		t.Fatal("trailing JSON accepted")
	}
	planRaw, _ := Read(r.RunnerPlan.Path)
	var p tr.PlanDocument
	if Decode(bytes.Replace(planRaw, []byte(`"timeoutSeconds":0`), []byte(`"timeoutSeconds":null`), 1), &p) == nil {
		t.Fatal("native scalar null coerced")
	}
}
