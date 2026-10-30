package jstestprovider

import (
	"errors"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// credentialNamedFixture is a passing qualified receipt whose spec file and test titles name a
// credential concept, as an ordinary password-recovery suite does (V1-2790, GitHub #1890).
func credentialNamedFixture(t *testing.T) Receipt {
	t.Helper()
	r := qualifiedFixture(t)
	digest := strings.Repeat("e", 64)
	r.Identity.TestFileDigests = map[string]string{
		"password-recovery.spec.ts":                          digest,
		"/wt/tests/e2e/admin/auth/password-recovery.spec.ts": digest,
		"token-refresh.spec.ts":                              digest,
	}
	r.Tests[0].Name = "password: too short is rejected"
	r.Tests[0].FullName = "one > password recovery > password: too short is rejected"
	// The projection anchor `<path>/password-recovery.spec.ts:12` is itself a credential-name
	// `:` assignment to the writer screen, whatever directory the path starts with.
	r.Tests[0].Anchor = &Anchor{File: "/wt/tests/e2e/admin/auth/password-recovery.spec.ts", Line: 12}
	r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
	return r
}

// PWP-V3-011: provider-written path keys and test titles that name a credential concept are not
// credential assignments; the qualified document encodes although the whole-document writer
// screen matches it.
func TestPWPV3011CredentialNamedPathsAndTitlesEncode(t *testing.T) {
	r := credentialNamedFixture(t)
	data, err := EncodeQualified(r)
	if err != nil {
		t.Fatalf("credential-named spec path and title refused: %v (member %q)", err, refusalPointer(err))
	}
	if !secretscreen.MatchString(string(data)) {
		t.Fatal("fixture no longer reproduces the whole-document false positive")
	}
}

// PWP-V3-011: credential values keep refusing, in identifier members through the value-shape
// rules and in every other member as written.
func TestPWPV3011SecretValuesStillRefused(t *testing.T) {
	github := "ghp_" + strings.Repeat("a1", 18)
	for _, tc := range []struct {
		name    string
		mutate  func(*Receipt)
		pointer string
		secret  string
	}{
		{"vendor token in title", func(r *Receipt) { r.Tests[0].Name = "uses " + github }, "/receipt/tests/0/name", github},
		{"credentialed URL in full title", func(r *Receipt) { r.Tests[0].FullName = "one > https://admin:hunter2-secret@example.test/" }, "/receipt/tests/0/fullName", "hunter2-secret"},
		{"vendor token in path key", func(r *Receipt) { r.Identity.TestFileDigests[github+".spec.ts"] = strings.Repeat("f", 64) }, "/receipt/identity/testFileDigests", github},
		{"assignment in failure message", func(r *Receipt) { r.Tests[0].FailureMessage = "password=actual-private-value" }, "/receipt/tests/0/failureMessage", "actual-private-value"},
		{"assignment in environment", func(r *Receipt) { r.Identity.Environment = map[string]string{"API_TOKEN": "hunter2-secret"} }, "/receipt/identity/environment", "hunter2-secret"},
		{"assignment in project use", func(r *Receipt) { r.Tests[0].Project.Use = []byte(`{"password":"hunter2-secret"}`) }, "/receipt/tests/0/project/use", "hunter2-secret"},
		{"assignment in artifact path", func(r *Receipt) {
			r.Tests[0].Artifacts = []FailureArtifact{{Name: "trace", Path: "/repo/password=hunter2-secret.zip"}}
		}, "/receipt/tests/0/artifacts/0/path", "hunter2-secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := credentialNamedFixture(t)
			tc.mutate(&r)
			r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
			_, err := EncodeQualified(r)
			if err == nil {
				t.Fatal("secret-shaped value retained")
			}
			if err.Error() != "qualified-document-secret-shaped" {
				t.Fatalf("wire code changed: %q", err.Error())
			}
			pointer := refusalPointer(err)
			if tc.pointer != "" && !strings.HasPrefix(pointer, tc.pointer) {
				t.Fatalf("pointer %q, want prefix %q", pointer, tc.pointer)
			}
			if tc.secret != "" && strings.Contains(pointer, tc.secret) {
				t.Fatalf("pointer %q echoes the secret", pointer)
			}
			if secretscreen.MatchString(pointer) || strings.Contains(pointer, ".spec.ts") {
				t.Fatalf("pointer %q echoes matched or path text", pointer)
			}
		})
	}
}

// PWP-V3-011: a credential written as a name=value assignment inside a test title or a path is
// still refused; only a credential name that assigns nothing is screened as an identifier.
func TestPWPV3011NameValueInIdentifierRefuses(t *testing.T) {
	for name, title := range map[string]string{
		"equals":      "login with password=hunter2",
		"colon":       "token: hunter2 is accepted",
		"quoted name": `uses "secret": "hunter2"`,
	} {
		t.Run(name, func(t *testing.T) {
			r := credentialNamedFixture(t)
			r.Tests[0].Name = title
			r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
			_, err := EncodeQualified(r)
			if err == nil || err.Error() != qualifiedSecretShaped {
				t.Fatalf("credential assignment in a title accepted: %v", err)
			}
			if strings.Contains(refusalPointer(err), "hunter2") {
				t.Fatalf("pointer %q echoes the secret", refusalPointer(err))
			}
		})
	}
}

// PWP-V3-012: a refusal pointer never prints a dynamic member name, even one too short or plain to
// match the screen on its own.
func TestPWPV3012PointerWithholdsDynamicKeys(t *testing.T) {
	r := credentialNamedFixture(t)
	r.Identity.Environment = map[string]string{"hunter2": "password=hunter2"}
	_, err := EncodeQualified(r)
	if err == nil || err.Error() != qualifiedSecretShaped {
		t.Fatalf("credential value in the environment accepted: %v", err)
	}
	if pointer := refusalPointer(err); pointer != "/receipt/identity/environment" {
		t.Fatalf("pointer %q, want the environment member without its key", pointer)
	}
}

// PWP-V3-011: a keep-reporters record whose config inputs are credential-named files encodes; a
// credential value in its reporter evidence still refuses.
func TestPWPV3011KeepReportersCredentialNamedInputs(t *testing.T) {
	control, keep := keepQualificationPair(t)
	for _, r := range []*Receipt{&control, &keep} {
		r.Identity.ConfigInputDigests["/repo/password-policy.cjs"] = strings.Repeat("d", 64)
		r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
	}
	q := QualifyKeepReporters(control, nil, keep, nil)
	if q.Verdict != KeepReportersQualified {
		t.Fatalf("fixture not qualified: %v", q.Reasons)
	}
	data, err := EncodeKeepReportersQualification(q)
	if err != nil {
		t.Fatalf("credential-named config input refused: %v (member %q)", err, refusalPointer(err))
	}
	if !strings.Contains(string(data), "password-policy.cjs") {
		t.Fatal("credential-named config input not bound")
	}
	q.ConfigInputs["/repo/ghp_"+strings.Repeat("a1", 18)+".cjs"] = strings.Repeat("d", 64)
	if _, err := EncodeKeepReportersQualification(q); err == nil || err.Error() != "qualified-document-secret-shaped" || refusalPointer(err) != "/configInputs" {
		t.Fatalf("vendor token in a config input path: %v (member %q)", err, refusalPointer(err))
	}
}

// PWP-V3-012: only schema member names and array indices are printed; a path, title, dynamic key
// or numeric-looking key ends the pointer at its parent.
func TestPWPV3012PointerNeverEchoesText(t *testing.T) {
	schema := map[string]bool{"receipt": true, "tests": true, "failureMessage": true, "identity": true, "testFileDigests": true, "environment": true}
	for _, tc := range []struct {
		path  []string
		index []bool
		want  string
	}{
		{[]string{"receipt", "tests", "0", "failureMessage"}, []bool{false, false, true, false}, "/receipt/tests/0/failureMessage"},
		{[]string{"receipt", "identity", "testFileDigests", "/repo/password-recovery.spec.ts"}, make([]bool, 4), "/receipt/identity/testFileDigests"},
		{[]string{"receipt", "identity", "environment", "ghp_" + strings.Repeat("a1", 18)}, make([]bool, 4), "/receipt/identity/environment"},
		{[]string{"receipt", "identity", "environment", "hunter2"}, make([]bool, 4), "/receipt/identity/environment"},
		{[]string{"receipt", "identity", "environment", "123456"}, make([]bool, 4), "/receipt/identity/environment"},
		{nil, nil, ""},
	} {
		if got := safePointer(documentMember{path: tc.path, index: tc.index}, schema); got != tc.want {
			t.Fatalf("safePointer(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func refusalPointer(err error) string {
	var refusal *QualifiedSecretError
	if errors.As(err, &refusal) {
		return refusal.Pointer
	}
	return "<not a secret-shaped refusal>"
}
