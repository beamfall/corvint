// SPDX-License-Identifier: Apache-2.0

package releasecandidate

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
)

// stubAppBinaries stands in for the app archive check the way tests stand in
// for verifyCoreBinary: fixture archives are scripts or marker bytes, not Go
// binaries with embedded build info.
func stubAppBinaries(t *testing.T) {
	t.Helper()
	previous := verifyAppBinary
	verifyAppBinary = func(raw []byte, app, goos, goarch, binaryDigest, goVersion, commit string) ([]byte, error) {
		return []byte("app-binary-" + app + "-" + goos + "-" + goarch), nil
	}
	t.Cleanup(func() { verifyAppBinary = previous })
}

// appArchiveFixture is one app archive's bytes: a real script archive for the
// host platform, whose executable prints `APP VERSION (build BUILD)`, and
// marker bytes elsewhere.
func appArchiveFixture(t *testing.T, app, platform, version, build string) []byte {
	t.Helper()
	if platform == runtime.GOOS+"_"+runtime.GOARCH && runtime.GOOS != "windows" {
		return appScriptArchive(t, app, platform, "#!/bin/sh\nprintf '%s\\n' '"+app+" "+version+" (build "+build+")'\n")
	}
	return []byte("app-archive-" + app + "-" + platform)
}

// appManifestFixture is the release-binary manifest recording every core-set
// app at one version, commit and toolchain for the catalog's targets (the
// AppPlatforms), plus one independent binary from an earlier release.
func appManifestFixture(t *testing.T, version, build, commit, goVersion string, archives map[string][]byte) appManifest {
	t.Helper()
	manifest := appManifest{Schema: appManifestSchema, Binaries: map[string]appRecord{}, DeclaredSkew: []appSkew{}}
	for _, app := range CoreSetApps {
		record := appRecord{Version: version, SourceCommit: commit, Build: build, GoVersion: goVersion, VersionSet: coreVersionSet}
		for _, platform := range appPlatforms() {
			goos, goarch := splitPlatform(platform)
			name := appArchiveName(app, platform)
			raw, present := archives[appArchivePath(app, platform)]
			if !present {
				raw = []byte("unshipped-" + name)
			}
			record.Archives = append(record.Archives, appArchive{GOOS: goos, GOARCH: goarch, Name: name, SHA256: digest(raw), Bytes: int64(len(raw)), BinarySHA256: digest([]byte("app-binary-" + app + "-" + goos + "-" + goarch))})
		}
		manifest.Binaries[app] = record
	}
	manifest.Binaries["corvint-analyzer-shell"] = appRecord{Version: "1.0.9", SourceCommit: strings.Repeat("9", 40), Build: "7", GoVersion: goVersion, Archives: []appArchive{{GOOS: "linux", GOARCH: "amd64", Name: "corvint-analyzer-shell_linux_amd64.tar.gz", SHA256: strings.Repeat("a", 64), Bytes: 1, BinarySHA256: strings.Repeat("b", 64)}}}
	return manifest
}

// appFiles builds the candidate app files: every shipped archive and the manifest.
func appFiles(t *testing.T, version, build, commit, goVersion string, mutate func(*appManifest)) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	for _, app := range CoreSetApps {
		for _, platform := range appPlatforms() {
			files[appArchivePath(app, platform)] = appArchiveFixture(t, app, platform, version, build)
		}
	}
	manifest := appManifestFixture(t, version, build, commit, goVersion, files)
	if mutate != nil {
		mutate(&manifest)
	}
	raw, err := canonicalJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	files[appsManifestPath] = raw
	return files
}

// addAppsCandidateFixture adds the app archives and the release manifest to a
// candidate fixture through its add callback.
func addAppsCandidateFixture(t *testing.T, add func(name, role string, raw []byte), version, build, commit, goVersion string) {
	t.Helper()
	for name, raw := range appFiles(t, version, build, commit, goVersion, nil) {
		role := appArchiveRole
		if name == appsManifestPath {
			role = appManifestRole
		}
		add(name, role, raw)
	}
}

// appsOutputFixture writes a release-binary output directory: release-manifest.json
// and the shipped app archives, as `release-binary build` and `manifest` leave them.
func appsOutputFixture(t *testing.T, version, build, commit, goVersion string) string {
	t.Helper()
	directory := t.TempDir()
	for name, raw := range appFiles(t, version, build, commit, goVersion, nil) {
		if err := os.WriteFile(filepath.Join(directory, filepath.Base(name)), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}

func appAssets(files map[string][]byte) map[string]Asset {
	assets := map[string]Asset{}
	for name, raw := range files {
		role := appArchiveRole
		if name == appsManifestPath {
			role = appManifestRole
		}
		assets[name] = Asset{Path: name, Role: role, SHA256: digest(raw), SizeBytes: int64(len(raw))}
	}
	return assets
}

// ARTIFACT-BIN-V0-522: a release-binary manifest for the catalog's three
// targets is a complete app manifest: the candidate asks for no darwin/amd64
// or Windows app archive, and every app platform is a shipped core platform.
func TestARTIFACTBINV0522AppArchivesFollowTheCatalogTargets(t *testing.T) {
	stubAppBinaries(t)
	if !slices.Equal(AppPlatforms, []string{"darwin_arm64", "linux_amd64", "linux_arm64"}) {
		t.Fatalf("AppPlatforms %v", AppPlatforms)
	}
	for _, platform := range AppPlatforms {
		if !slices.Contains(shippedPlatforms(), platform) {
			t.Errorf("app platform %s is not a shipped core archive platform %v", platform, shippedPlatforms())
		}
	}
	commit := strings.Repeat("a", 40)
	manifest := Manifest{Version: "0.5.0a1", BuildNumber: "9", GoVersion: "go1.27.3", Sources: []SourceIdentity{{Name: "corvint", Commit: commit, Tree: strings.Repeat("b", 40)}}}
	files := appFiles(t, "0.5.0a1", "9", commit, "go1.27.3", nil)
	if err := verifyApps(files, appAssets(files), manifest); err != nil {
		t.Fatalf("a three-target release manifest must verify: %v", err)
	}
	if got, want := len(files)-1, len(CoreSetApps)*3; got != want || appAssetCount() != want {
		t.Fatalf("candidate app archives %d (count %d), want %d", got, appAssetCount(), want)
	}
	for name := range files {
		if strings.Contains(name, "darwin_amd64") || strings.Contains(name, "windows") {
			t.Errorf("candidate carries %s", name)
		}
	}
}

// ARTIFACT-BIN-V0-510: the candidate binds every core-set app archive to the
// release manifest and the manifest to its own release identity; any drift
// between the three refuses.
func TestARTIFACTBINV0510VerifyAppsBindsManifestAndArchivesToTheRelease(t *testing.T) {
	stubAppBinaries(t)
	commit := strings.Repeat("a", 40)
	manifest := Manifest{Version: "0.5.0a1", BuildNumber: "9", GoVersion: "go1.27.3", Sources: []SourceIdentity{{Name: "corvint", Commit: commit, Tree: strings.Repeat("b", 40)}}}
	files := appFiles(t, "0.5.0a1", "9", commit, "go1.27.3", nil)
	if err := verifyApps(files, appAssets(files), manifest); err != nil {
		t.Fatalf("agreeing app set refused: %v", err)
	}
	cases := []struct {
		name, want string
		files      func() map[string][]byte
	}{
		{"app missing from the manifest", "release manifest lacks corvint-docs", func() map[string][]byte {
			return appFiles(t, "0.5.0a1", "9", commit, "go1.27.3", func(m *appManifest) { delete(m.Binaries, "corvint-docs") })
		}},
		{"app at another version", "release manifest records corvint-docs at 0.5.0a2", func() map[string][]byte {
			return appFiles(t, "0.5.0a1", "9", commit, "go1.27.3", func(m *appManifest) {
				record := m.Binaries["corvint-docs"]
				record.Version = "0.5.0a2"
				m.Binaries["corvint-docs"] = record
			})
		}},
		{"app from another commit", "release manifest records corvint-mcp at 0.5.0a1@" + strings.Repeat("c", 40), func() map[string][]byte {
			return appFiles(t, "0.5.0a1", "9", commit, "go1.27.3", func(m *appManifest) {
				record := m.Binaries["corvint-mcp"]
				record.SourceCommit = strings.Repeat("c", 40)
				m.Binaries["corvint-mcp"] = record
			})
		}},
		{"app with another toolchain", "release manifest records corvint-docs-mcp", func() map[string][]byte {
			return appFiles(t, "0.5.0a1", "9", commit, "go1.27.3", func(m *appManifest) {
				record := m.Binaries["corvint-docs-mcp"]
				record.GoVersion = "go1.26.0"
				m.Binaries["corvint-docs-mcp"] = record
			})
		}},
		{"app outside the core set", "release manifest records corvint-mcp", func() map[string][]byte {
			return appFiles(t, "0.5.0a1", "9", commit, "go1.27.3", func(m *appManifest) {
				record := m.Binaries["corvint-mcp"]
				record.VersionSet = ""
				m.Binaries["corvint-mcp"] = record
			})
		}},
		{"declared skew in the core set", "declares version skew in the core set", func() map[string][]byte {
			return appFiles(t, "0.5.0a1", "9", commit, "go1.27.3", func(m *appManifest) {
				m.DeclaredSkew = append(m.DeclaredSkew, appSkew{VersionSet: "core", Reason: "docs-only fix"})
			})
		}},
		{"shipped archive row missing", "release manifest lacks corvint-docs_linux_arm64.tar.gz", func() map[string][]byte {
			return appFiles(t, "0.5.0a1", "9", commit, "go1.27.3", func(m *appManifest) {
				record := m.Binaries["corvint-docs"]
				record.Archives = record.Archives[:len(record.Archives)-1]
				m.Binaries["corvint-docs"] = record
			})
		}},
		{"archive bytes drifted", "candidate app archive corvint-docs-mcp_darwin_arm64.tar.gz disagrees with the release manifest", func() map[string][]byte {
			files := appFiles(t, "0.5.0a1", "9", commit, "go1.27.3", nil)
			files[appArchivePath("corvint-docs-mcp", "darwin_arm64")] = []byte("changed")
			return files
		}},
		{"archive asset missing", "candidate app asset apps/corvint-mcp_linux_amd64.tar.gz is missing", func() map[string][]byte {
			files := appFiles(t, "0.5.0a1", "9", commit, "go1.27.3", nil)
			delete(files, appArchivePath("corvint-mcp", "linux_amd64"))
			return files
		}},
		{"unknown manifest field", "candidate release manifest", func() map[string][]byte {
			files := appFiles(t, "0.5.0a1", "9", commit, "go1.27.3", nil)
			files[appsManifestPath] = []byte(strings.Replace(string(files[appsManifestPath]), `"declaredSkew"`, `"signedBy": "x", "declaredSkew"`, 1))
			return files
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			files := test.files()
			if err := verifyApps(files, appAssets(files), manifest); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want refusal containing %q, got %v", test.want, err)
			}
		})
	}
	// A candidate at another version than the manifest records refuses too.
	other := manifest
	other.Version = "0.5.0a2"
	if err := verifyApps(files, appAssets(files), other); err == nil || !strings.Contains(err.Error(), "the candidate is 0.5.0a2") {
		t.Fatalf("manifest at another version than the candidate accepted: %v", err)
	}
	// The real app archive check reads the archive, not markers.
	if _, err := verifyAppArchiveBinary([]byte("not an archive"), "corvint-docs", "linux", "amd64", strings.Repeat("a", 64), "go1.27.3", strings.Repeat("c", 40)); err == nil {
		t.Fatal("non-archive bytes accepted as an app archive")
	}
}

// ARTIFACT-BIN-V0-510: an app executable is accepted only when it carries the
// candidate's source commit; the manifest's claim alone is not enough.
func TestARTIFACTBINV0510AppExecutableCarriesTheCandidateCommit(t *testing.T) {
	commit, other := strings.Repeat("c", 40), strings.Repeat("d", 40)
	stamped := []byte("\x00go build\x00" + commit + "\x00")
	for name, test := range map[string]struct {
		binary   []byte
		settings map[string]string
		commit   string
		want     string
	}{
		"stamped":            {stamped, nil, commit, ""},
		"stamped, same vcs":  {stamped, map[string]string{"vcs.revision": commit, "vcs.modified": "false"}, commit, ""},
		"another commit":     {[]byte("\x00" + other + "\x00"), nil, commit, "not stamped with source commit " + commit},
		"unstamped":          {[]byte("no stamp"), nil, commit, "not stamped with source commit"},
		"no commit to match": {stamped, nil, "", "not stamped with source commit"},
		"vcs disagrees":      {stamped, map[string]string{"vcs.revision": other}, commit, "built from " + other},
		"modified tree":      {stamped, map[string]string{"vcs.revision": commit, "vcs.modified": "true"}, commit, "modified tree"},
	} {
		err := appCommitAgrees(test.binary, test.settings, test.commit)
		if test.want == "" && err != nil || test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
			t.Errorf("%s: got %v, want %q", name, err, test.want)
		}
	}
}

// ARTIFACT-BIN-V0-511: the installer extracts every core-set app beside corvint
// in one directory, writes one checksum row per executable, and refuses a set
// whose members do not report one version.
func TestARTIFACTBINV0511InstallPlacesAppsBesideCorvintAtOneVersion(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("qualified installer supports darwin and linux")
	}
	platform := runtime.GOOS + "_" + runtime.GOARCH
	stub := func(docsLine string) {
		previous := verifyForInstall
		verifyForInstall = func(context.Context, string) (*VerifiedCandidate, error) {
			files := map[string][]byte{"core/corvint_" + platform + ".tar.gz": coreArchiveFixture(t, platform)}
			for _, app := range CoreSetApps {
				line := app + " 0.5.0a1 (build 9)"
				if app == "corvint-docs" && docsLine != "" {
					line = docsLine
				}
				files[appArchivePath(app, platform)] = appScriptArchive(t, app, platform, "#!/bin/sh\nprintf '%s\\n' '"+line+"'\n")
			}
			return &VerifiedCandidate{Manifest: Manifest{Version: "0.5.0a1", CorvintVersion: "Corvint 0.5.0a1 (build 9)"}, files: files}, nil
		}
		t.Cleanup(func() { verifyForInstall = previous })
	}
	stub("")
	store := filepath.Join(canonicalTemp(t), "store")
	installed, err := InstallCore(t.Context(), t.TempDir(), store)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(installed)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	want := append([]string{"SHA256SUMS", "corvint"}, CoreSetApps...)
	sort.Strings(want)
	if strings.Join(names, " ") != strings.Join(want, " ") {
		t.Fatalf("installed %v, want %v", names, want)
	}
	for _, app := range CoreSetApps {
		info, err := os.Stat(filepath.Join(installed, app))
		if err != nil || info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("%s not installed executable: %v", app, err)
		}
	}
	sums, err := os.ReadFile(filepath.Join(installed, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSuffix(string(sums), "\n"), "\n")
	if len(rows) != 1+len(CoreSetApps) {
		t.Fatalf("installed SHA256SUMS has %d rows: %s", len(rows), sums)
	}
	for _, row := range rows {
		sum, name, _ := strings.Cut(row, "  ")
		raw, err := os.ReadFile(filepath.Join(installed, name))
		if err != nil || digest(raw) != sum {
			t.Fatalf("installed checksum row %q does not bind the installed file: %v", row, err)
		}
	}

	stub("corvint-docs 0.5.0a2 (build 10)")
	mismatched := filepath.Join(canonicalTemp(t), "store2")
	if _, err := InstallCore(t.Context(), t.TempDir(), mismatched); err == nil || !strings.Contains(err.Error(), "not one release") || !strings.Contains(err.Error(), "corvint-docs is version 0.5.0a2") {
		t.Fatalf("set with a corvint-docs at another version installed: %v", err)
	}
	if entries, err := os.ReadDir(filepath.Join(mismatched, "corvint", "0.5.0a1")); err == nil && len(entries) != 0 {
		t.Fatalf("refused install left %v", entries)
	}
}

// extractAppBinary refuses an archive whose inner checksum does not bind its
// executable and one whose notices differ from the installed corvint archive.
func TestARTIFACTBINV0511ExtractAppBinaryRefusesChecksumAndNoticeDrift(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script archives")
	}
	platform := runtime.GOOS + "_" + runtime.GOARCH
	stage := t.TempDir()
	good := appScriptArchive(t, "corvint-docs", platform, "#!/bin/sh\nexit 0\n")
	if _, err := extractAppBinary(good, stage, "corvint-docs_"+platform, "corvint-docs"); err != nil {
		t.Fatal(err)
	}
	withoutChecksum := scriptArchive(t, "corvint-docs", platform, "#!/bin/sh\nexit 0\n", false)
	if _, err := extractAppBinary(withoutChecksum, t.TempDir(), "corvint-docs_"+platform, "corvint-docs"); err == nil || !strings.Contains(err.Error(), "internal checksum disagrees") {
		t.Fatalf("archive without an agreeing inner checksum extracted: %v", err)
	}
	if _, err := extractAppBinary(good, t.TempDir(), "corvint-mcp_"+platform, "corvint-mcp"); err == nil {
		t.Fatal("archive under another root extracted")
	}
	// A member name with a separator of either platform or a ".." is refused
	// before any file under the stage is read.
	for _, member := range []string{`sub\LICENSE`, "..LICENSE"} {
		odd := scriptArchiveWithMembers(t, "corvint-docs", platform, "#!/bin/sh\nexit 0\n", map[string][]byte{member: []byte("n\n")})
		if _, err := extractAppBinary(odd, t.TempDir(), "corvint-docs_"+platform, "corvint-docs"); err == nil || !strings.Contains(err.Error(), "outside its root") {
			t.Fatalf("archive with member %q extracted: %v", member, err)
		}
	}
	// A notice member must equal the one the corvint archive installed.
	stage = t.TempDir()
	if err := os.WriteFile(filepath.Join(stage, "LICENSE"), []byte("corvint notice\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	withNotice := scriptArchiveWithMembers(t, "corvint-docs", platform, "#!/bin/sh\nexit 0\n", map[string][]byte{"LICENSE": []byte("other notice\n")})
	if _, err := extractAppBinary(withNotice, stage, "corvint-docs_"+platform, "corvint-docs"); err == nil || !strings.Contains(err.Error(), "notice LICENSE disagrees") {
		t.Fatalf("archive with a differing notice extracted: %v", err)
	}
	same := scriptArchiveWithMembers(t, "corvint-docs", platform, "#!/bin/sh\nexit 0\n", map[string][]byte{"LICENSE": []byte("corvint notice\n")})
	if _, err := extractAppBinary(same, stage, "corvint-docs_"+platform, "corvint-docs"); err != nil {
		t.Fatalf("archive with the same notice refused: %v", err)
	}
}
