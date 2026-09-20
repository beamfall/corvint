package companionrelease

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func coreRetainedFixture(t *testing.T, mutate func(map[string]json.RawMessage, *[]ArchiveEntry)) string {
	t.Helper()
	return versionedRetainedFixture(t, true, mutate)
}

func versionedRetainedFixture(t *testing.T, core bool, mutate func(map[string]json.RawMessage, *[]ArchiveEntry)) string {
	t.Helper()
	dir := currentRetainedFixture(t, nil)
	old, err := VerifyRetainedBundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := old.Manifest
	m.Profile = json.RawMessage(`"corvint-companion-bundle/1"`)
	if core {
		m.Profile = json.RawMessage(`"corvint-companion-bundle/2"`)
		m.VSIXTools = VSIXToolchainManifest{}
		m.Artifacts = m.Artifacts[1:]
	}
	pi := ArchiveEntry{Path: "plugins/pi/index.ts", Mode: 0644, Data: []byte("// pi fixture\n")}
	a := m.Artifacts[len(m.Artifacts)-1]
	a.Name, a.Path, a.SHA256, a.SizeBytes = "pi", "plugins/pi/", treeDigestForTest(pi), int64(len(pi.Data))
	m.Artifacts = append(m.Artifacts, a)
	var entries []ArchiveEntry
	for _, e := range old.entries {
		if e.Path == "MANIFEST.json" || e.Path == "SHA256SUMS" || e.Path == "README.md" || (core && (strings.HasPrefix(e.Path, "extensions/") || strings.HasSuffix(e.Path, "VSIX-BUILD-ONLY-NPM.txt"))) {
			continue
		}
		entries = append(entries, e)
	}
	entries = append(entries, pi)
	entries, err = assembleBundleEntries(entries, m)
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		var fields map[string]json.RawMessage
		for _, e := range entries {
			if e.Path == "MANIFEST.json" {
				if err := json.Unmarshal(e.Data, &fields); err != nil {
					t.Fatal(err)
				}
			}
		}
		mutate(fields, &entries)
		for j := range entries {
			if entries[j].Path == "MANIFEST.json" {
				entries[j].Data, err = json.Marshal(fields)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		var withoutSums []ArchiveEntry
		for _, e := range entries {
			if e.Path != "SHA256SUMS" {
				withoutSums = append(withoutSums, e)
			}
		}
		entries = append(withoutSums, ArchiveEntry{Path: "SHA256SUMS", Mode: 0644, Data: renderSHA256SUMS(withoutSums)})
	}
	archive, err := buildTarGz(entries)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256Hex(archive)
	if core {
		var corvint ComponentManifest
		for _, component := range m.Components {
			if component.Name == "corvint" {
				corvint = component
			}
		}
		for _, name := range []string{"corvint-version-identity", "corvint-affected-selection", "corvint-playwright-external-discovery", "corvint-documentation-corpus-discovery", "corvint-work-queue-observation"} {
			old.SmokeSteps = append(old.SmokeSteps, SmokeStep{Name: name, OK: true, ComponentSHA256: corvint.BinarySHA256, SourceCommit: corvint.Commit, SourceTree: corvint.Tree, InvokedPath: "/private/removed/" + corvint.BinaryPath})
		}
	}
	for j := range old.SmokeSteps {
		old.SmokeSteps[j].BundleSHA256 = digest
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

func TestCoreBundleProfileRoundTrip(t *testing.T) {
	t.Run("CRB-V0-019 exact core retained extraction", func(t *testing.T) {
		v, err := VerifyRetainedBundle(coreRetainedFixture(t, nil))
		if err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(t.TempDir(), "extract")
		if err := v.Extract(dest); err != nil {
			t.Fatal(err)
		}
		if len(v.Manifest.Components) != 9 || len(v.Manifest.Artifacts) != 5 {
			t.Fatal("wrong inventory")
		}
		body, err := os.ReadFile(filepath.Join(dest, "MANIFEST.json"))
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(body, &fields); err != nil {
			t.Fatal(err)
		}
		if len(fields) != 7 || fields["vsixToolchain"] != nil {
			t.Fatal("wrong core shape")
		}
		for _, name := range []string{"codex", "claude-code", "gemini-cli", "opencode", "pi"} {
			if _, err := os.Stat(filepath.Join(dest, "plugins", name)); err != nil {
				t.Fatal(err)
			}
		}
		for _, name := range currentFixtureNames {
			if _, err := os.Stat(filepath.Join(dest, "bin", name)); err != nil {
				t.Fatal(err)
			}
		}
		for _, e := range v.entries {
			if strings.HasPrefix(e.Path, "extensions/") || strings.HasSuffix(e.Path, "VSIX-BUILD-ONLY-NPM.txt") {
				t.Fatal(e.Path)
			}
		}
	})
}

func TestCoreBundleRejectsEditorToolchain(t *testing.T) {
	t.Run("CRB-V0-019 forbidden editor toolchain", func(t *testing.T) {
		for _, value := range []string{"null", "{}", `{"nodeVersion":"v22.23.2"}`} {
			t.Run(value, func(t *testing.T) {
				dir := coreRetainedFixture(t, func(m map[string]json.RawMessage, _ *[]ArchiveEntry) { m["vsixToolchain"] = json.RawMessage(value) })
				if _, err := VerifyRetainedBundle(dir); err == nil {
					t.Fatal("accepted editor field")
				}
			})
		}
	})
}

func TestCoreBundleRejectsProfileDowngrade(t *testing.T) {
	t.Run("CRB-V0-019 closed profile downgrade refusal", func(t *testing.T) {
		for _, profile := range []string{"absent", "null", `""`, `"corvint-companion-bundle/0"`, `"corvint-companion-bundle/1"`, `"corvint-companion-bundle/3"`} {
			t.Run(profile, func(t *testing.T) {
				dir := coreRetainedFixture(t, func(m map[string]json.RawMessage, _ *[]ArchiveEntry) {
					if profile == "absent" {
						delete(m, "profile")
					} else {
						m["profile"] = json.RawMessage(profile)
					}
				})
				if _, err := VerifyRetainedBundle(dir); err == nil {
					t.Fatal("accepted profile downgrade")
				}
			})
		}
	})
}

func TestCoreBundleRejectsMixedInventory(t *testing.T) {
	t.Run("CRB-V0-019 mixed archive refusal", func(t *testing.T) {
		for _, path := range []string{"extensions/corvint-vscode-0.1.0.vsix", "notices/corvint/VSIX-BUILD-ONLY-NPM.txt", "plugins/other/file"} {
			t.Run(path, func(t *testing.T) {
				dir := coreRetainedFixture(t, func(_ map[string]json.RawMessage, e *[]ArchiveEntry) {
					*e = append(*e, ArchiveEntry{Path: path, Mode: 0644, Data: []byte("extra")})
				})
				if _, err := VerifyRetainedBundle(dir); err == nil {
					t.Fatal("accepted extra member")
				}
			})
		}
		dir := coreRetainedFixture(t, func(m map[string]json.RawMessage, _ *[]ArchiveEntry) {
			m["artifacts"] = json.RawMessage(strings.ReplaceAll(string(m["artifacts"]), "FALLBACK", "UNQUALIFIED"))
		})
		if _, err := VerifyRetainedBundle(dir); err == nil {
			t.Fatal("accepted invented support")
		}
	})
}

func TestPiRetainedProfileRoundTrip(t *testing.T) {
	t.Run("CRB-V0-019 retained profile one reader", func(t *testing.T) {
		v, err := VerifyRetainedBundle(versionedRetainedFixture(t, false, nil))
		if err != nil {
			t.Fatal(err)
		}
		if len(v.Manifest.Artifacts) != 6 || v.Manifest.VSIXTools.NodeSHA256 == "" {
			t.Fatal("legacy profile one changed")
		}
		if err := v.Extract(filepath.Join(t.TempDir(), "extract")); err != nil {
			t.Fatal(err)
		}
	})
}

func TestCoreBundleRejectsMissingAndUnknownFields(t *testing.T) {
	t.Run("CRB-V0-019 exact core fields", func(t *testing.T) {
		for _, field := range []string{"target", "goVersion", "gitVersion", "notRunTargets", "components", "artifacts"} {
			t.Run(field, func(t *testing.T) {
				dir := coreRetainedFixture(t, func(m map[string]json.RawMessage, _ *[]ArchiveEntry) { delete(m, field) })
				if _, err := VerifyRetainedBundle(dir); err == nil {
					t.Fatal("accepted missing field")
				}
			})
		}
		dir := coreRetainedFixture(t, func(m map[string]json.RawMessage, _ *[]ArchiveEntry) { m["extra"] = json.RawMessage(`null`) })
		if _, err := VerifyRetainedBundle(dir); err == nil {
			t.Fatal("accepted unknown field")
		}
	})
}
