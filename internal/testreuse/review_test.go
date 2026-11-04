// SPDX-License-Identifier: Apache-2.0

package testreuse

import (
	"context"
	json "encoding/json/v2"
	"os"
	"runtime"
	"strings"
	"testing"
)

// TestRuntimeEnvironmentEnablesSkippedFailure covers AFP-V0-2322: a pass
// containing a skipped qualification cannot stand for its enabled execution.
func TestRuntimeEnvironmentEnablesSkippedFailure(t *testing.T) {
	root := fixtureRepo(t)
	const key = "CORVINT_REUSE_QUALIFICATION"
	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "b/b_test.go", `package b
import ("os"; "testing")
func TestQualification(t *testing.T) {
    value, present := os.LookupEnv("CORVINT_REUSE_QUALIFICATION")
    if !present { t.Skip("qualification disabled") }
    if value != "working" { t.Fatal("enabled qualification failed") }
}
`)
	commitAll(t, root, "qualification")
	stream := goTest(t, root, true, "-json", "./b")
	if !strings.Contains(string(stream), `"Action":"skip"`) {
		t.Fatal("baseline must skip qualification")
	}
	record := recordAt(t, root, Source{Kind: SourceLocal}, allPassed)
	wantUnits(t, "unchanged environment", decideAt(t, root, localPolicy, record), map[string]string{"b": ReasonRecordDigestEqual})
	for _, value := range []string{"", "broken"} {
		t.Setenv(key, value)
		wantUnits(t, "qualification enabled: "+value, decideAt(t, root, localPolicy, record), map[string]string{"b": ReasonIdentityChanged})
		goTest(t, root, false, "./b")
	}
	// Persist only the digest of the environment, never its arbitrary values.
	_, identity := observe(t, root, nil)
	body, err := json.Marshal(identity)
	if err != nil || strings.Contains(string(body), key) || strings.Contains(string(body), "broken") {
		t.Fatalf("runtime environment exposed: %s: %v", body, err)
	}
}

// TestDependencyModuleLanguageFault covers AFP-V0-2321: the language version
// and checksum inputs of a local replacement govern its compiled sources.
func TestDependencyModuleLanguageFault(t *testing.T) {
	root := fixtureRepo(t)
	writeFixture(t, root, "go.mod", "module "+fixtureModule+"\n\ngo 1.27\nrequire example.com/dependency v0.0.0\nreplace example.com/dependency => ./dependency\n")
	writeFixture(t, root, "dependency/go.mod", "module example.com/dependency\n\ngo 1.22\n")
	writeFixture(t, root, "dependency/value.go", "package dependency\nfunc Value() int { n := 0; for range 1 { n++ }; return n }\n")
	writeFixture(t, root, "b/b.go", "package b\nimport \"example.com/dependency\"\nfunc B() int { return dependency.Value() }\n")
	commitAll(t, root, "local replacement")
	goTest(t, root, true, "./a")
	record := recordAt(t, root, Source{Kind: SourceLocal}, allPassed)
	wantUnits(t, "unchanged replacement", decideAt(t, root, localPolicy, record), map[string]string{"a": ReasonRecordDigestEqual})
	writeFixture(t, root, "dependency/go.sum", "")
	wantUnits(t, "untracked checksum", decideAt(t, root, localPolicy, record), map[string]string{"a": ReasonDigestError})
	commitAll(t, root, "checksum created")
	wantUnits(t, "checksum addition", decideAt(t, root, localPolicy, record), map[string]string{"a": ReasonDigestChanged})
	record = recordAt(t, root, Source{Kind: SourceLocal}, allPassed)
	writeFixture(t, root, "dependency/go.mod", "module example.com/dependency\n\ngo 1.21\n")
	wantUnits(t, "dirty manifest", decideAt(t, root, localPolicy, record), map[string]string{"a": ReasonDigestError})
	commitAll(t, root, "language fault")
	wantUnits(t, "language fault", decideAt(t, root, localPolicy, record), map[string]string{"a": ReasonDigestChanged})
	failed := goTest(t, root, false, "./a")
	if !strings.Contains(string(failed), "go1.22") {
		t.Fatalf("expected language version fault: %s", failed)
	}
}

// TestAssemblyIncludedHeaderFault covers AFP-V0-2321: non-standard assembly
// is refused even when its included header is outside the declared read scope.
func TestAssemblyIncludedHeaderFault(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("fault fixture uses amd64 assembly")
	}
	root := fixtureRepo(t)
	writeFixture(t, root, "b/b.go", "package b\nfunc B() int\n")
	writeFixture(t, root, "shared/value.h", "#define VALUE 1\n")
	writeFixture(t, root, "b/value_amd64.s", "#include \"../shared/value.h\"\nTEXT ·B(SB),$0-8\nMOVQ $VALUE, ret+0(FP)\nRET\n")
	commitAll(t, root, "assembly include")
	goTest(t, root, true, "./a")
	record := recordAt(t, root, Source{Kind: SourceLocal}, allPassed)
	for _, value := range []string{"#define VALUE 1\n", "#define VALUE 2\n"} {
		writeFixture(t, root, "shared/value.h", value)
		commitAll(t, root, "header")
		units := decideAt(t, root, localPolicy, record)
		wantUnits(t, "assembly closure", units, map[string]string{"a": ReasonDigestError, "b": ReasonDigestError})
		if !strings.HasPrefix(units["a"].Detail, "assembly-input:") {
			t.Errorf("missing assembly refusal: %+v", units["a"])
		}
	}
	// Go's build cache does not discover this escaping header either; force
	// a fresh compilation to observe the header fault independently of reuse.
	failed := goTest(t, root, false, "-a", "./a")
	if !strings.Contains(string(failed), "FAIL") {
		t.Fatalf("expected included-header fault: %s", failed)
	}
}

// TestRuntimeEnvironmentCanonicalization covers AFP-V0-2322: order and
// shadowed values are immaterial, while unset and empty remain different.
func TestRuntimeEnvironmentCanonicalization(t *testing.T) {
	root := fixtureRepo(t)
	snapshot, _ := observe(t, root, nil)
	base := fixtureEnv()
	identity := func(extra ...string) string {
		t.Helper()
		observed, err := ObserveIdentity(context.Background(), snapshot, "", append(append([]string{}, base...), extra...), fixtureArgv, nil)
		if err != nil {
			t.Fatal(err)
		}
		digest, err := observed.Digest()
		if err != nil {
			t.Fatal(err)
		}
		return digest
	}
	if identity("REUSE_ENV_A=old", "REUSE_ENV_A=new", "REUSE_ENV_B=1") != identity("REUSE_ENV_B=1", "REUSE_ENV_A=new") {
		t.Error("environment order or overridden values changed identity")
	}
	if identity() == identity("REUSE_EMPTY=") {
		t.Error("unset and empty environments must differ")
	}
	if identity("=C:=before", "=D:=stable") == identity("=C:=after", "=D:=stable") {
		t.Error("drive environment entries must retain their distinct names")
	}
}
