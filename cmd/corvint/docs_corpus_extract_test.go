package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

const extractCLIProfile = `{"schema":"corvint-extractor-profile/1","provider":"app","version":"1","extractors":[` +
	`{"id":"routes","subject":"endpoint","files":["app/routes.rb"],"pattern":"^\\s*(?P<method>get|post)\\s+'(?P<path>[^']+)'"}]}`

func extractCLIRepository(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	cemGit(t, root, "init", "-q")
	cemWrite(t, root, "app/routes.rb", "get '/orders'\npost '/orders'\n")
	cemWrite(t, root, "corvint/extract.json", extractCLIProfile)
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "extract")
	return root, cemGit(t, root, "rev-parse", "HEAD")
}

// DCP-V1-399: `docs corpus extract` prints exactly the producer's inventory bytes and refuses a
// missing profile or an unpinned revision without output.
func TestDCPV1399DocsCorpusExtractCLI(t *testing.T) {
	root, revision := extractCLIRepository(t)
	code, out, stderr := corpusCLI(t, root, "docs", "corpus", "extract", "--profile", "corvint/extract.json", "--revision", revision)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	want, err := doccorpus.BuildExtractorInventory(context.Background(), doccorpus.ExtractorInput{Root: root, Revision: revision, Profile: "corvint/extract.json"})
	if err != nil || out != string(want) {
		t.Fatalf("CLI output differs from the producer: %v\n%s\n%s", err, out, want)
	}
	var inventory doccorpus.ExtractorInventory
	if err := json.Unmarshal([]byte(out), &inventory); err != nil || inventory.Status != "generated" || len(inventory.Provider.Subjects) != 2 {
		t.Fatalf("inventory = %+v, %v", inventory, err)
	}
	for _, args := range [][]string{
		{"--profile", "corvint/absent.json", "--revision", revision},
		{"--profile", "corvint/extract.json", "--revision", "HEAD"},
		{"--revision", revision},
	} {
		code, out, stderr := corpusCLI(t, root, append([]string{"docs", "corpus", "extract"}, args...)...)
		if code == 0 || out != "" || strings.TrimSpace(stderr) == "" {
			t.Fatalf("%q: exit=%d out=%q stderr=%q", args, code, out, stderr)
		}
	}
}

// DCP-V1-506: `docs corpus extract --profile ABS --profile-sha256 HEX` prints exactly the
// producer's caller-profile inventory bytes; a missing or mismatched digest is refused with no
// output.
func TestDCPV1506DocsCorpusExtractCallerProfileCLI(t *testing.T) {
	root, revision := extractCLIRepository(t)
	profile := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(profile, []byte(extractCLIProfile), 0600); err != nil {
		t.Fatal(err)
	}
	digest := doccorpus.Digest([]byte(extractCLIProfile))
	code, out, stderr := corpusCLI(t, root, "docs", "corpus", "extract", "--profile", profile, "--profile-sha256", digest, "--revision", revision)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	want, err := doccorpus.BuildExtractorInventory(context.Background(), doccorpus.ExtractorInput{Root: root, Revision: revision, Profile: profile, ProfileSHA256: digest})
	if err != nil || out != string(want) {
		t.Fatalf("CLI output differs from the producer: %v\n%s\n%s", err, out, want)
	}
	var inventory doccorpus.ExtractorInventory
	if err := json.Unmarshal([]byte(out), &inventory); err != nil || inventory.Profile.Origin != "caller" || inventory.Profile.SHA256 != digest || len(inventory.Provider.Subjects) != 2 {
		t.Fatalf("inventory = %+v, %v", inventory, err)
	}
	for _, args := range [][]string{
		{"--profile", profile, "--revision", revision},
		{"--profile", profile, "--profile-sha256", strings.Repeat("0", 64), "--revision", revision},
		{"--profile", "corvint/extract.json", "--profile-sha256", digest, "--revision", revision},
	} {
		code, out, stderr := corpusCLI(t, root, append([]string{"docs", "corpus", "extract"}, args...)...)
		if code == 0 || out != "" || !strings.Contains(stderr, "corpus-refused") {
			t.Fatalf("%q: exit=%d out=%q stderr=%q", args, code, out, stderr)
		}
	}
}
