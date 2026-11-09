package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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

// DCX-V1-399: `docs corpus extract` prints exactly the producer's inventory bytes and refuses a
// missing profile or an unpinned revision without output.
func TestDCXV1399DocsCorpusExtractCLI(t *testing.T) {
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

// DCX-V1-506: `docs corpus extract --profile ABS --profile-sha256 HEX` prints exactly the
// producer's caller-profile inventory bytes; a missing or mismatched digest is refused with no
// output.
func TestDCXV1506DocsCorpusExtractCallerProfileCLI(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("caller-owned extractor profiles need the darwin/linux no-follow reader")
	}
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
	// An explicitly empty digest is refused, never read as the repository-profile route.
	code, out, stderr = corpusCLI(t, root, "docs", "corpus", "extract", "--profile", "corvint/extract.json", "--profile-sha256=", "--revision", revision)
	if code == 0 || out != "" || !strings.Contains(stderr, "invalid-arguments") || !strings.Contains(stderr, "--profile-sha256 must not be empty") {
		t.Fatalf("empty --profile-sha256: exit=%d out=%q stderr=%q", code, out, stderr)
	}
}

// DCX-V1-560: `docs corpus extract --schema 2` prints exactly the producer's adoption-provider
// inventory; --schema 1 prints the default bytes and another value is refused with no output.
func TestDCXV1560DocsCorpusExtractSchemaCLI(t *testing.T) {
	root, revision := extractCLIRepository(t)
	base := []string{"docs", "corpus", "extract", "--profile", "corvint/extract.json", "--revision", revision}
	_, plain, _ := corpusCLI(t, root, base...)
	code, out, stderr := corpusCLI(t, root, append(base, "--schema", "1")...)
	if code != 0 || out != plain {
		t.Fatalf("--schema 1 differs from the default: %d %s", code, stderr)
	}
	code, out, stderr = corpusCLI(t, root, append(base, "--schema", "2")...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	want, err := doccorpus.BuildExtractorInventory(context.Background(), doccorpus.ExtractorInput{Root: root, Revision: revision, Profile: "corvint/extract.json", Schema: "2"})
	var inventory doccorpus.ExtractorInventory
	if err != nil || out != string(want) || json.Unmarshal([]byte(out), &inventory) != nil || inventory.Provider.Schema != doccorpus.AdoptionProviderSchema {
		t.Fatalf("--schema 2 output: %v %+v", err, inventory.Provider.Schema)
	}
	if code, out, _ := corpusCLI(t, root, append(base, "--schema", "3")...); code != 2 || out != "" {
		t.Fatalf("--schema 3 accepted: %d %q", code, out)
	}
}
