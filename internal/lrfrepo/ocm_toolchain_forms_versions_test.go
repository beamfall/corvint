package lrfrepo

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestSTAV0001_VersionsManifest checks the STA-V0-001 support manifest and the install pins the
// spec-toolchain-conformance job verifies against it: exactly the two upstream toolchains, each
// with a release version, a registry checksum and a check date, and the npm lock and the hashed
// Spec Kit requirements naming the same release and checksum. Whether the pins equal the
// registry's latest release is STA-V0-012's operator check, not this test's.
func TestSTAV0001_VersionsManifest(t *testing.T) {
	dir := filepath.Join("..", "..", "interop", "spec-toolchains")
	var manifest struct {
		Profile       string `json:"profile"`
		SupportPolicy string `json:"supportPolicy"`
		Toolchains    []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Source  struct {
				Kind       string `json:"kind"`
				Package    string `json:"package"`
				Tarball    string `json:"tarball"`
				Repository string `json:"repository"`
				Tag        string `json:"tag"`
				Archive    string `json:"archive"`
			} `json:"source"`
			TarballSha256 string `json:"tarballSha256"`
			NpmIntegrity  string `json:"npmIntegrity"`
			Commit        string `json:"commit"`
			ArchiveSha256 string `json:"archiveSha256"`
			Checked       string `json:"checked"`
		} `json:"toolchains"`
	}
	for _, partial := range []string{"sha512-", "sha512-???", "sha512-" + strings.Repeat("A", 86) + "=", "sha256-" + strings.Repeat("A", 86) + "=="} {
		if fullSRISha512(partial) {
			t.Fatalf("fullSRISha512(%q) = true, want a partial digest refused", partial)
		}
	}
	readJSONStrict(t, filepath.Join(dir, "versions.json"), &manifest)
	if manifest.Profile != "corvint-spec-toolchain-versions/0" {
		t.Fatalf("profile = %q", manifest.Profile)
	}
	if !strings.Contains(manifest.SupportPolicy, "untested") {
		t.Fatalf("supportPolicy must say other versions are untested: %q", manifest.SupportPolicy)
	}
	semver := regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	sha256Hex := regexp.MustCompile(`^[0-9a-f]{64}$`)
	commitHex := regexp.MustCompile(`^[0-9a-f]{40}$`)
	seen := map[string]bool{}
	for _, tc := range manifest.Toolchains {
		seen[tc.Name] = true
		if !semver.MatchString(tc.Version) {
			t.Errorf("%s: version %q is not a release version", tc.Name, tc.Version)
		}
		if _, err := time.Parse("2006-01-02", tc.Checked); err != nil {
			t.Errorf("%s: checked %q is not a date: %v", tc.Name, tc.Checked, err)
		}
		switch tc.Name {
		case "openspec":
			if tc.Source.Kind != "npm" || tc.Source.Package != "@fission-ai/openspec" {
				t.Errorf("openspec source = %+v", tc.Source)
			}
			wantTarball := "https://registry.npmjs.org/@fission-ai/openspec/-/openspec-" + tc.Version + ".tgz"
			if tc.Source.Tarball != wantTarball {
				t.Errorf("openspec tarball = %q, want %q", tc.Source.Tarball, wantTarball)
			}
			if !sha256Hex.MatchString(tc.TarballSha256) {
				t.Errorf("openspec tarballSha256 = %q", tc.TarballSha256)
			}
			if !fullSRISha512(tc.NpmIntegrity) {
				t.Errorf("openspec npmIntegrity = %q is not a full SRI sha512 digest", tc.NpmIntegrity)
			}
			checkNpmPins(t, dir, tc.Version, tc.Source.Tarball, tc.NpmIntegrity)
		case "spec-kit":
			if tc.Source.Kind != "git-tag" || tc.Source.Repository != "github/spec-kit" || tc.Source.Tag != "v"+tc.Version {
				t.Errorf("spec-kit source = %+v", tc.Source)
			}
			if !commitHex.MatchString(tc.Commit) {
				t.Errorf("spec-kit commit = %q", tc.Commit)
			}
			wantArchive := "https://github.com/github/spec-kit/archive/" + tc.Commit + ".tar.gz"
			if tc.Source.Archive != wantArchive {
				t.Errorf("spec-kit archive = %q, want %q", tc.Source.Archive, wantArchive)
			}
			if _, err := hex.DecodeString(tc.ArchiveSha256); err != nil || !sha256Hex.MatchString(tc.ArchiveSha256) {
				t.Errorf("spec-kit archiveSha256 = %q", tc.ArchiveSha256)
			}
			checkSpecKitPins(t, dir, tc.Source.Archive, tc.ArchiveSha256)
		default:
			t.Errorf("unexpected toolchain %q", tc.Name)
		}
	}
	if len(manifest.Toolchains) != 2 || !seen["openspec"] || !seen["spec-kit"] {
		t.Fatalf("toolchains = %v, want exactly openspec and spec-kit", seen)
	}
}

func checkNpmPins(t *testing.T, dir, version, tarball, integrity string) {
	t.Helper()
	var pkg struct {
		Private      bool              `json:"private"`
		Dependencies map[string]string `json:"dependencies"`
	}
	readJSON(t, filepath.Join(dir, "npm", "package.json"), &pkg)
	if !pkg.Private || len(pkg.Dependencies) != 1 || pkg.Dependencies["@fission-ai/openspec"] != version {
		t.Errorf("npm/package.json must privately pin exactly @fission-ai/openspec %s: %+v", version, pkg)
	}
	var lock struct {
		LockfileVersion int `json:"lockfileVersion"`
		Packages        map[string]struct {
			Version   string `json:"version"`
			Resolved  string `json:"resolved"`
			Integrity string `json:"integrity"`
		} `json:"packages"`
	}
	readJSON(t, filepath.Join(dir, "npm", "package-lock.json"), &lock)
	if lock.LockfileVersion != 3 {
		t.Errorf("package-lock lockfileVersion = %d, want 3", lock.LockfileVersion)
	}
	entry := lock.Packages["node_modules/@fission-ai/openspec"]
	if entry.Version != version || entry.Resolved != tarball || entry.Integrity != integrity {
		t.Errorf("package-lock openspec entry = %+v, want %s %s %s", entry, version, tarball, integrity)
	}
	for path, p := range lock.Packages {
		if path == "" {
			continue
		}
		if !fullSRISha512(p.Integrity) || !strings.HasPrefix(p.Resolved, "https://registry.npmjs.org/") {
			t.Errorf("package-lock %s is not a full-sha512-verified npm registry tarball: %+v", path, p)
		}
	}
}

// fullSRISha512 reports whether integrity is `sha512-` and 88 canonical base64
// characters decoding to exactly 64 bytes, the rule
// script/spec-toolchain-conformance.sh applies before npm ci: a truncated or
// empty digest would let npm verify nothing.
func fullSRISha512(integrity string) bool {
	encoded, ok := strings.CutPrefix(integrity, "sha512-")
	if !ok || len(encoded) != 88 {
		return false
	}
	digest, err := base64.StdEncoding.Strict().DecodeString(encoded)
	return err == nil && len(digest) == 64
}

func checkSpecKitPins(t *testing.T, dir, archive, archiveSha256 string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, "speckit", "requirements.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := "specify-cli @ " + archive + " \\\n    --hash=sha256:" + archiveSha256 + "\n"
	if !strings.Contains(string(body), want) {
		t.Errorf("speckit/requirements.txt does not pin %q", want)
	}
	for _, name := range []string{"requirements.txt", "build-requirements.txt"} {
		body, err := os.ReadFile(filepath.Join(dir, "speckit", name))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "#") {
				continue
			}
			if !strings.HasSuffix(line, " \\") {
				t.Errorf("speckit/%s:%d: requirement %q has no --hash continuation", name, i+1, line)
			}
		}
	}
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func readJSONStrict(t *testing.T, path string, v any) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}
