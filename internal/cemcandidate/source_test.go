package cemcandidate

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
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
func TestSourceUsesGitBytesRatherThanWorkingDirectory(t *testing.T) {
	r, out := fixture(t, "sha1")
	if e := os.WriteFile(filepath.Join(r.Repository, "app.txt"), []byte("dirty checkout"), 0600); e != nil {
		t.Fatal(e)
	}
	result, e := Assemble(t.Context(), r, out)
	if e != nil {
		t.Fatal(e)
	}
	if result.DeclaredInputGitBinding != "DECLARED_INPUT_BYTES_MATCH_TARGET" || result.ExecutionAtCommit != "NOT_OBSERVED" {
		t.Fatal("wrong source binding scope")
	}
}
func TestAssemblePreservesOpaqueCaptureLimits(t *testing.T) {
	r, out := fixture(t, "sha1")
	result, e := Assemble(t.Context(), r, out)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := json.Marshal(result)
	if e != nil {
		t.Fatal(e)
	}
	for _, field := range []string{`"nativeCaptureSemanticVerification":"NOT_OBSERVED"`, `"executionAtCommit":"NOT_OBSERVED"`, `"referenceIntegrity":"REFERENCE_INTEGRITY_ONLY"`} {
		if !bytes.Contains(raw, []byte(field)) {
			t.Fatal("lost explicit limit", field)
		}
	}
}

func TestDeclaredInputRefusesGitSymlinksAndGitlinks(t *testing.T) {
	for _, kind := range []string{"symlink", "gitlink", "missing"} {
		t.Run(kind, func(t *testing.T) {
			r, _ := fixture(t, "sha1")
			name := "external"
			switch kind {
			case "symlink":
				if e := os.Symlink("app.txt", filepath.Join(r.Repository, name)); e != nil {
					t.Fatal(e)
				}
				git(t, r.Repository, "add", name)
			case "gitlink":
				git(t, r.Repository, "update-index", "--add", "--cacheinfo", "160000,"+r.ExpectedBase+","+name)
			}
			if kind != "missing" {
				git(t, r.Repository, "commit", "-qm", kind)
				r.Target = git(t, r.Repository, "rev-parse", "HEAD")
			}
			repo, e := gitauth.Open(r.Repository, gitrun.NewDefaultBudget())
			if e != nil {
				t.Fatal(e)
			}
			plan := tr.PlanDocument{Request: tr.Request{InputFiles: map[string]string{name: tr.Digest([]byte("app.txt"))}}}
			if _, e := sourceBinding(t.Context(), repo, r, plan); e == nil {
				t.Fatal("unsupported source admitted")
			}
		})
	}
}
