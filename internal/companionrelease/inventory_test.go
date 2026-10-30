package companionrelease

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Literal migration expectations are independent of the production selector.
var currentFixtureNames = map[string]string{
	"corvint": "corvint", "corvint-console": "corvint-console", "corvint-dashboard-snapshot": "corvint-dashboard-snapshot",
	"corvint-mcp": "corvint-mcp", "corvint-docs-mcp": "corvint-docs-mcp", "corvint-test-validity-mcp": "corvint-test-validity-mcp",
	"corvint-js-test-provider": "corvint-js-test-provider", "corvint-go-test-provider": "corvint-go-test-provider", "atm": "corvint-tasks",
}

func currentRetainedFixture(t *testing.T, mutate func(*BundleManifest)) string {
	t.Helper()
	dir := makeRetainedFixture(t)
	old, err := VerifyRetainedBundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := old.Manifest
	m.Profile = json.RawMessage(`"corvint-companion-bundle/0"`)
	rename := func(path string) string {
		for a, b := range currentFixtureNames {
			if path == "bin/"+a {
				return "bin/" + b
			}
		}
		path = strings.ReplaceAll(path, "corvint-taskman", "corvint-tasks")
		return path
	}
	for j := range m.Components {
		c := &m.Components[j]
		c.Name = currentFixtureNames[c.Name]
		c.BinaryPath = rename(c.BinaryPath)
		c.SourceArchivePath = rename(c.SourceArchivePath)
		if c.Module == "corvint" {
			c.Module = "corvint"
		} else {
			c.Module = "corvint-tasks"
		}
	}
	for j := range m.Artifacts {
		a := &m.Artifacts[j]
		a.Path = rename(a.Path)
		a.SourceArchivePath = rename(a.SourceArchivePath)
	}
	var entries []ArchiveEntry
	for _, e := range old.entries {
		if e.Path == "MANIFEST.json" || e.Path == "SHA256SUMS" || e.Path == "README.md" {
			continue
		}
		e.Path = rename(e.Path)
		if strings.HasSuffix(e.Path, "THIRD-PARTY-NOTICES.txt") {
			e.Data = []byte(strings.ReplaceAll(string(e.Data), "corvint-taskman", "corvint-tasks"))
		}
		entries = append(entries, e)
	}

	entries, err = assembleBundleEntries(entries, m)
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(&m)
		for j := range entries {
			if entries[j].Path == "MANIFEST.json" {
				entries[j].Data, err = renderManifestJSON(m)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		for j := range entries {
			if entries[j].Path == "SHA256SUMS" {
				entries[j].Data = renderSHA256SUMS(entries[:j])
			}
		}
	}
	archive, err := buildTarGz(entries)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256Hex(archive)
	for j := range old.SmokeSteps {
		step := &old.SmokeSteps[j]
		step.BundleSHA256 = digest
		step.InvokedPath = "/private/removed/" + rename(strings.TrimPrefix(step.InvokedPath, "/private/removed/"))
	}
	smoke, err := json.Marshal(struct {
		BundleSHA256 string      `json:"bundleSha256"`
		Steps        []SmokeStep `json:"steps"`
	}{digest, old.SmokeSteps})
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "fixture.tar.gz"), archive)
	mustWrite(t, filepath.Join(dir, "fixture.tar.gz.sha256"), []byte(digest+"  fixture.tar.gz\n"))
	mustWrite(t, filepath.Join(dir, "fixture.smoke.json"), smoke)
	return dir
}

func TestCurrentAndLegacyClosedBundleProfiles(t *testing.T) {
	t.Run("CRB-V0-017 current-and-legacy-closed-inventories", func(t *testing.T) {
		for _, current := range []bool{false, true} {
			dir := makeRetainedFixture(t)
			if current {
				dir = currentRetainedFixture(t, nil)
			}
			v, err := VerifyRetainedBundle(dir)
			if err != nil {
				t.Fatal(err)
			}
			_, _, source, err := v.CorvintSourceIdentity()
			if err != nil {
				t.Fatal(err)
			}
			if want := "source/corvint-src.tar.gz"; source != want {
				t.Fatalf("source %s", source)
			}
			dest := filepath.Join(t.TempDir(), "extract")
			if err := v.Extract(dest); err != nil {
				t.Fatal(err)
			}
			for old, new := range currentFixtureNames {
				name := old
				if current {
					name = new
				}
				if _, err := os.Stat(filepath.Join(dest, "bin", name)); err != nil {
					t.Fatal(err)
				}
			}
		}
	})
}

func TestCurrentBundleRejectsInvalidProfilesAndMixedIdentities(t *testing.T) {
	t.Run("CRB-V0-017 ambiguous-profile-and-mixed-identity-refusal", func(t *testing.T) {
		cases := map[string]func(*BundleManifest){
			"absent":         func(m *BundleManifest) { m.Profile = nil },
			"null":           func(m *BundleManifest) { m.Profile = json.RawMessage(`null`) },
			"empty":          func(m *BundleManifest) { m.Profile = json.RawMessage(`""`) },
			"unknown":        func(m *BundleManifest) { m.Profile = json.RawMessage(`"corvint-companion-bundle/1"`) },
			"type":           func(m *BundleManifest) { m.Profile = json.RawMessage(`{}`) },
			"retired-name":   func(m *BundleManifest) { m.Components[0].Name = "corvint-go" },
			"arbitrary-path": func(m *BundleManifest) { m.Components[0].BinaryPath = "bin/other" },
		}
		for name, mutate := range cases {
			t.Run(name, func(t *testing.T) {
				if _, err := VerifyRetainedBundle(currentRetainedFixture(t, mutate)); err == nil {
					t.Fatal("invalid current inventory accepted")
				}
			})
		}
	})
}

func TestPiClosedProfileInventories(t *testing.T) {
	t.Run("CRB-V0-017 exact Pi profile inventory and bytes", func(t *testing.T) {
		dir := currentRetainedFixture(t, nil)
		old, err := VerifyRetainedBundle(dir)
		if err != nil {
			t.Fatal(err)
		}
		// Profile zero remains the literal five-artifact contract.
		names := []string{}
		for _, a := range old.Manifest.Artifacts {
			names = append(names, a.Name)
		}
		sort.Strings(names)
		if strings.Join(names, ",") != "claude-code,codex,corvint-vscode,gemini-cli,opencode" {
			t.Fatal(names)
		}
		m := old.Manifest
		m.Profile = json.RawMessage(`"corvint-companion-bundle/1"`)
		if validateClosedManifest(old.entries, m) == nil {
			t.Fatal("profile one accepted without Pi")
		}
		entry := ArchiveEntry{Path: "plugins/pi/index.ts", Mode: 0644, Data: []byte("// public Pi fixture\n")}
		a := m.Artifacts[1]
		a.Name = "pi"
		a.Path = "plugins/pi/"
		a.Kind = "host-package-tree"
		a.SizeBytes = int64(len(entry.Data))
		h := sha256.New()
		fmt.Fprintf(h, "%s\x00%06o\x00%d\x00", entry.Path, entry.Mode, len(entry.Data))
		h.Write(entry.Data)
		a.SHA256 = hex.EncodeToString(h.Sum(nil))
		m.Artifacts = append(m.Artifacts, a)
		entries := append(append([]ArchiveEntry{}, old.entries...), entry)
		if err := validateClosedManifest(entries, m); err != nil {
			t.Fatal(err)
		}
		for _, profile := range []json.RawMessage{json.RawMessage(`"corvint-companion-bundle/0"`), nil, json.RawMessage(`null`), json.RawMessage(`""`), json.RawMessage(`"corvint-companion-bundle/2"`)} {
			m.Profile = profile
			if validateClosedManifest(entries, m) == nil {
				t.Fatalf("accepted Pi under %s", profile)
			}
		}
		m.Profile = json.RawMessage(`"corvint-companion-bundle/1"`)
		entries[len(entries)-1].Data = []byte("tampered")
		if validateClosedManifest(entries, m) == nil {
			t.Fatal("accepted changed Pi bytes")
		}
	})
}
