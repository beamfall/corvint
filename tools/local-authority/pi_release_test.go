package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/authoritystore"
	"github.com/Beamfall/corvint/internal/localauthority"
)

func TestPiReleaseProfileIsolation(t *testing.T) {
	t.Run("PPI-V0-005 PPI-V0-009 exact image pair and distinct release identity", func(t *testing.T) {
		sum := strings.Repeat("a", 64)
		m := releaseManifest{Profile: piReleaseProfile, SourceRevision: strings.Repeat("b", 40), AdapterTemplateSHA256: sum, Files: []releaseFile{{"authority-hook.json", sum, "0444"}, {"bin/git", sum, "0555"}, {"corvint", sum, "0555"}, {"go/bin/go", sum, "0555"}, {"local-authority", sum, "0555"}, {"pi-build.json", sum, "0444"}, {"pi-protected", sum, "0555"}}}
		m.ReleaseID = sourceReleaseID(m)
		if err := validateRelease(m); err != nil {
			t.Fatal(err)
		}
		legacy := m
		legacy.Profile = "corvint-authority-release/1"
		if sourceReleaseID(legacy) == m.ReleaseID || validateRelease(legacy) == nil {
			t.Fatal("mixed legacy manifest accepted")
		}
		for _, name := range []string{"pi-protected", "pi-build.json", "../pi-protected", "pi-provider.js"} {
			bad := m
			bad.Files = append([]releaseFile(nil), m.Files...)
			bad.Files[len(bad.Files)-1].Path = name
			if name == "pi-protected" {
				bad.Files[len(bad.Files)-1].Mode = "0444"
			}
			bad.ReleaseID = sourceReleaseID(bad)
			if validateRelease(bad) == nil {
				t.Fatal("bad Pi file accepted", name)
			}
		}
	})
}

func TestPiBuildManifestDetectsCopiedImageDrift(t *testing.T) {
	t.Run("PPI-V0-001 PPI-V0-009 final copied image pair matches reviewed build", func(t *testing.T) {
		dir := t.TempDir()
		sum := strings.Repeat("a", 64)
		m := map[string]any{"schema": "corvint-pi-protected-build/0", "status": "EXPERIMENTAL_UNQUALIFIED", "pi": "0.85.1", "node": "22.23.2", "bun": "1.3.11", "nodeArchiveSHA256": sum, "lockSHA256": sum, "bundleSHA256": sum, "workerSHA256": sum, "photonSHA256": sum, "entitlementsSHA256": sum, "assets": map[string]string{"fixture": sum}, "images": map[string]string{"pi-protected": digest([]byte("host")), "corvint": digest([]byte("consumer"))}}
		for name, data := range map[string][]byte{"pi-protected": []byte("host"), "corvint": []byte("consumer")} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, "pi-build.json"), mustJSONLine(m), 0644); err != nil {
			t.Fatal(err)
		}
		if err := validatePiBuildFiles(dir, filepath.Join(dir, "corvint"), "pi-build.json"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "corvint"), []byte("changed"), 0755); err != nil {
			t.Fatal(err)
		}
		if validatePiBuildFiles(dir, filepath.Join(dir, "corvint"), "pi-build.json") == nil {
			t.Fatal("image drift accepted")
		}
	})
}

func TestPiInstallPreservesOtherAdmission(t *testing.T) {
	t.Run("PPI-V0-009 existing non-Pi admission is never replaced", func(t *testing.T) {
		for _, profile := range []string{authoritystore.RootProfile, authoritystore.DirectRootProfile, "corvint-protected-root/99"} {
			r := authoritystore.RootDocument{Profile: profile, Admission: "OPERATOR_ACCEPTED", RootID: strings.Repeat("a", 64), RepositoryRoot: "/fixture"}
			raw, err := localauthority.Canonical(r)
			if err != nil {
				t.Fatal(err)
			}
			if piStoreAdmission(raw) == nil {
				t.Fatal("other admission accepted", profile)
			}
		}
		if piStoreAdmission([]byte(`{"profile":"corvint-protected-root/3"}`)) == nil {
			t.Fatal("incomplete admission accepted")
		}
	})
}
