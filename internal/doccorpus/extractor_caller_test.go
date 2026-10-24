package doccorpus

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// extractCallerProfile copies the fixture's committed profile bytes to a file outside the
// repository and returns its path and SHA-256.
func extractCallerProfile(t *testing.T, root, rev string) (string, string, []byte) {
	t.Helper()
	raw := []byte(git(t, root, "show", rev+":corvint/extract.json"))
	committed, err := os.ReadFile(filepath.Join(root, "corvint/extract.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(raw), bytes.TrimSpace(committed)) {
		t.Fatal("fixture profile differs from its commit")
	}
	name := filepath.Join(t.TempDir(), "caller-profile.json")
	if err := os.WriteFile(name, committed, 0600); err != nil {
		t.Fatal(err)
	}
	return name, Digest(committed), committed
}

func extractCaller(t *testing.T, root, rev, name, digest string) (ExtractorInventory, []byte) {
	t.Helper()
	data, err := BuildExtractorInventory(context.Background(), ExtractorInput{Root: root, Revision: rev, Profile: name, ProfileSHA256: digest})
	if err != nil {
		t.Fatal(err)
	}
	var inv ExtractorInventory
	if err := json.Unmarshal(data, &inv, json.RejectUnknownMembers(true)); err != nil {
		t.Fatal(err)
	}
	return inv, data
}

func extractEncoded(t *testing.T, value any) string {
	t.Helper()
	data, err := Encode(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// DCP-V1-478: a caller-owned profile outside the repository, pinned by a matching digest, extracts
// the same subjects, relations and gaps from Git at the revision as the same profile committed in
// the repository; the repository-declared inventory keeps its bytes.
func TestDCPV1478CallerProfileExtractsLikeTheCommittedProfile(t *testing.T) {
	t.Parallel()
	root, rev := extractFixture(t, true, nil)
	repo, repoBytes := extractInventory(t, root, rev)
	name, digest, _ := extractCallerProfile(t, root, rev)
	caller, _ := extractCaller(t, root, rev, name, digest)
	if caller.Repository != repo.Repository || caller.Revision != repo.Revision || caller.Status != "generated" {
		t.Fatalf("caller inventory identity = %s@%s %s, want %s@%s", caller.Repository, caller.Revision, caller.Status, repo.Repository, repo.Revision)
	}
	for _, pair := range [][2]any{
		{caller.Provider.Subjects, repo.Provider.Subjects},
		{caller.Provider.Relations, repo.Provider.Relations},
		{caller.Gaps, repo.Gaps},
		{caller.Provider.Source, repo.Provider.Source},
	} {
		if got, want := extractEncoded(t, pair[0]), extractEncoded(t, pair[1]); got != want {
			t.Fatalf("caller extraction differs from the committed profile:\n%s\n%s", got, want)
		}
	}
	if len(caller.Provider.Subjects) == 0 {
		t.Fatal("caller extraction found no subjects")
	}
	if bytes.Contains(repoBytes, []byte(`"origin"`)) || bytes.Contains(repoBytes, []byte(`"document"`)) || repo.Profile.Path != "corvint/extract.json" || repo.Profile.Blob == "" {
		t.Fatalf("repository-declared profile reference changed: %+v", repo.Profile)
	}
}

// DCP-V1-478: without a matching, well-formed digest, a caller-owned profile is refused with no
// output, and a profile inside the extracted repository is never a caller profile.
func TestDCPV1478CallerProfileRequiresAMatchingDigest(t *testing.T) {
	t.Parallel()
	root, rev := extractFixture(t, false, nil)
	name, digest, raw := extractCallerProfile(t, root, rev)
	outside := filepath.Dir(name)
	if err := os.Mkdir(filepath.Join(outside, "dir.json"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(name, filepath.Join(outside, "link.json")); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(root, "corvint", "extract.json")
	other := Digest(append(raw, ' '))
	for _, tc := range []struct {
		profile, digest, code, message string
	}{
		{name, "", "corpus-refused", "extractor profile is not a canonical repository-relative path; a caller-owned profile outside the repository also needs --profile-sha256"},
		{name, other, "corpus-refused", "caller-owned extractor profile does not match --profile-sha256 (file sha256 " + digest + ")"},
		{name, strings.ToUpper(digest), "corpus-refused", "--profile-sha256 must be 64 lowercase hexadecimal digits"},
		{name, digest[:63], "corpus-refused", "--profile-sha256 must be 64 lowercase hexadecimal digits"},
		{"corvint/extract.json", digest, "corpus-refused", "a caller-owned extractor profile must be a clean absolute path outside the repository"},
		{outside + "/./caller-profile.json", digest, "corpus-refused", "a caller-owned extractor profile must be a clean absolute path outside the repository"},
		{inside, digest, "corpus-refused", "a caller-owned extractor profile must lie outside the extracted repository; commit it and pass its repository path without --profile-sha256"},
		{filepath.Join(outside, "absent.json"), digest, "corpus-input-unavailable", "caller-owned extractor profile missing, unsafe or over bound"},
		{filepath.Join(outside, "dir.json"), digest, "corpus-input-unavailable", "caller-owned extractor profile missing, unsafe or over bound"},
		{filepath.Join(outside, "link.json"), digest, "corpus-input-unavailable", "caller-owned extractor profile missing, unsafe or over bound"},
	} {
		data, err := BuildExtractorInventory(context.Background(), ExtractorInput{Root: root, Revision: rev, Profile: tc.profile, ProfileSHA256: tc.digest})
		var refusal *Error
		if data != nil || !errors.As(err, &refusal) || refusal.Code != tc.code || refusal.Message != tc.message {
			t.Errorf("%s %q: data=%d err=%v, want %s: %s", tc.profile, tc.digest, len(data), err, tc.code, tc.message)
		}
	}
	// A matching digest still admits only a valid closed profile.
	bad := filepath.Join(outside, "bad.json")
	badRaw := []byte(`{"schema":"corvint-extractor-profile/1","provider":"app","version":"1","extractors":[],"extra":1}`)
	if err := os.WriteFile(bad, badRaw, 0600); err != nil {
		t.Fatal(err)
	}
	if data, err := BuildExtractorInventory(context.Background(), ExtractorInput{Root: root, Revision: rev, Profile: bad, ProfileSHA256: Digest(badRaw)}); data != nil || err == nil || !strings.Contains(err.Error(), "caller extractor profile") || strings.Contains(err.Error(), outside) {
		t.Fatalf("invalid caller profile: data=%d err=%v", len(data), err)
	}
}

// DCP-V1-503: a caller inventory records the origin, the exact-byte digest, the canonical profile
// document and its digest, a limitation and the digest in the subjects capability, and never the
// caller's local path.
func TestDCPV1503CallerProfileProvenanceIsRecorded(t *testing.T) {
	t.Parallel()
	root, rev := extractFixture(t, false, nil)
	_, _, raw := extractCallerProfile(t, root, rev)
	indented := append([]byte("\n  "), raw...) // not canonical: the pin is the file's exact bytes
	name := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(name, indented, 0600); err != nil {
		t.Fatal(err)
	}
	inv, data := extractCaller(t, root, rev, name, Digest(indented))
	ref := inv.Profile
	if ref.Origin != ExtractorProfileOriginCaller || ref.SHA256 != Digest(indented) || ref.Path != "" || ref.Blob != "" || ref.Document == nil {
		t.Fatalf("caller profile reference = %+v", ref)
	}
	canonical, err := Encode(*ref.Document)
	if err != nil || ref.CanonicalSHA256 != Digest(canonical) || ref.CanonicalSHA256 == ref.SHA256 {
		t.Fatalf("canonical document digest = %s, want %s (%v)", ref.CanonicalSHA256, Digest(canonical), err)
	}
	if ref.Document.Provider != "app" || len(ref.Document.Extractors) != len(extractProfile("")["extractors"].([]any)) {
		t.Fatalf("recorded document = %+v", ref.Document)
	}
	if bytes.Contains(data, []byte(filepath.Dir(name))) {
		t.Fatal("inventory records the caller's local path")
	}
	if inv.Limitations[len(inv.Limitations)-1] != callerProfileLimitation {
		t.Fatalf("limitations = %q", inv.Limitations)
	}
	if got := inv.Provider.Capabilities[0]; got.Name != "subjects" || got.Reason != "subjects matched by the caller-owned extractor profile sha256:"+Digest(indented) {
		t.Fatalf("subjects capability = %+v", got)
	}
}

// DCP-V1-504: the provider record of a caller inventory, committed unchanged and declared as a
// provider input, builds through the existing provider admission, and the built corpus still
// names the profile digest.
func TestDCPV1504CallerProfileProviderRecordBuildsUnchanged(t *testing.T) {
	t.Parallel()
	root, rev := extractFixture(t, false, nil)
	name, digest, _ := extractCallerProfile(t, root, rev)
	inv, _ := extractCaller(t, root, rev, name, digest)
	m, err := Inventory(context.Background(), root, rev, "app", "2026-10-24T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	record, err := Encode(inv.Provider)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "evidence"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "evidence/extract.json"), record, 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "evidence/extract.json")
	git(t, root, "commit", "-qm", "caller extractor record")
	recordRevision := git(t, root, "rev-parse", "HEAD")
	m.Providers = append(m.Providers, Provider{ID: "app", Kind: "records", Version: "1", Revision: recordRevision, Record: "evidence/extract.json"})
	m.Scopes = append(m.Scopes, Scope{"evidence/extract.json", recordRevision})
	m.Inputs = append(m.Inputs, Input{Path: "evidence/extract.json", Revision: recordRevision, Blob: git(t, root, "rev-parse", recordRevision+":evidence/extract.json"), SHA256: Digest(record), Provider: "app", Purpose: "provider"})
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	subjects := 0
	for _, s := range a.Subjects {
		if s.Provider == "app" {
			subjects++
			if s.Evidence.Trust != "generated" {
				t.Fatalf("extracted subject is not generated: %+v", s)
			}
		}
	}
	if subjects != len(inv.Provider.Subjects) {
		t.Fatalf("built corpus has %d extracted subjects, want %d", subjects, len(inv.Provider.Subjects))
	}
	built, err := Encode(a)
	if err != nil || !bytes.Contains(built, []byte("sha256:"+digest)) {
		t.Fatalf("built corpus does not name the caller profile digest (%v)", err)
	}
}

// DCP-V1-505: a caller extraction reads sources only from Git at the revision, writes neither the
// repository nor the profile, and returns identical bytes for identical inputs.
func TestDCPV1505CallerProfileExtractionIsReadOnly(t *testing.T) {
	t.Parallel()
	root, rev := extractFixture(t, true, nil)
	name, digest, raw := extractCallerProfile(t, root, rev)
	before := git(t, root, "status", "--porcelain", "--ignored")
	_, first := extractCaller(t, root, rev, name, digest)
	if err := os.WriteFile(filepath.Join(root, "app/config/routes.rb"), []byte("get '/changed', to: 'x'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, second := extractCaller(t, root, rev, name, digest)
	if !bytes.Equal(first, second) {
		t.Fatal("caller extraction read the working tree or is not deterministic")
	}
	if err := os.WriteFile(filepath.Join(root, "app/config/routes.rb"), []byte(extractFiles["app/config/routes.rb"]), 0600); err != nil {
		t.Fatal(err)
	}
	if after := git(t, root, "status", "--porcelain", "--ignored"); after != before {
		t.Fatalf("caller extraction wrote the repository: %q -> %q", before, after)
	}
	if now, err := os.ReadFile(name); err != nil || !bytes.Equal(now, raw) {
		t.Fatalf("caller extraction changed the profile (%v)", err)
	}
	entries, err := os.ReadDir(filepath.Dir(name))
	if err != nil || len(entries) != 1 {
		t.Fatalf("caller extraction wrote beside the profile: %v %v", entries, err)
	}
}
