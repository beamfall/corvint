// SPDX-License-Identifier: Apache-2.0

package releasecandidate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"debug/buildinfo"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/docsenvelope"
	"github.com/Beamfall/corvint/internal/versionline"
)

// CoreSetApps are the members of the release catalog's `core` version set
// beside corvint (tools/release-binary/binaries.json, ARTIFACT-BIN-V0-001),
// sorted by name. The corvint archive stays one binary; a release ships each
// of these as its own archive and the installer installs them beside corvint
// in one directory, so `corvint docs` and the MCP servers run on an installed
// release (ARTIFACT-BIN-V0-509..511, CMS-V0-002, V1-5237). The release-binary
// tests hold this list equal to the catalog's set.
var CoreSetApps = []string{
	"corvint-cli",
	"corvint-docs",
	"corvint-docs-mcp",
	"corvint-mcp",
	"corvint-test-runner",
	"corvint-test-validity-mcp",
	"corvint-web-flows",
}

const (
	// appsManifestPath is the release-binary manifest the candidate carries as evidence.
	appsManifestPath  = "evidence/release-manifest.json"
	appManifestSchema = "corvint.release-manifest/1"
	appManifestFile   = "release-manifest.json"
	appArchiveRole    = "app-archive"
	appManifestRole   = "app-manifest"
	coreVersionSet    = "core"
	modulePath        = "github.com/Beamfall/corvint"
)

// appManifest mirrors the closed `corvint.release-manifest/1` document
// `release-binary manifest` writes (ARTIFACT-BIN-V0-003); unknown fields refuse.
type appManifest struct {
	Schema       string               `json:"schema"`
	Binaries     map[string]appRecord `json:"binaries"`
	DeclaredSkew []appSkew            `json:"declaredSkew"`
}

type appRecord struct {
	Version      string       `json:"version"`
	SourceCommit string       `json:"sourceCommit"`
	Build        string       `json:"build"`
	GoVersion    string       `json:"goVersion"`
	VersionSet   string       `json:"versionSet,omitempty"`
	Archives     []appArchive `json:"archives"`
}

type appArchive struct {
	GOOS         string `json:"goos"`
	GOARCH       string `json:"goarch"`
	Name         string `json:"name"`
	SHA256       string `json:"sha256"`
	Bytes        int64  `json:"bytes"`
	BinarySHA256 string `json:"binarySha256"`
}

type appSkew struct {
	VersionSet string            `json:"versionSet"`
	Reason     string            `json:"reason"`
	Members    map[string]string `json:"members"`
}

// shippedPlatforms are the `<goos>_<goarch>` pairs the candidate ships, from
// the shipped core archives; Windows stays excluded (PRS-V1-004).
func shippedPlatforms() []string {
	platforms := make([]string, 0, len(shippedCoreArchives))
	for _, archive := range shippedCoreArchives {
		platforms = append(platforms, strings.TrimSuffix(strings.TrimPrefix(archive, "corvint_"), ".tar.gz"))
	}
	return platforms
}

func splitPlatform(platform string) (goos, goarch string) {
	goos, goarch, _ = strings.Cut(platform, "_")
	return goos, goarch
}

func appArchiveName(app, platform string) string { return app + "_" + platform + ".tar.gz" }
func appArchivePath(app, platform string) string { return "apps/" + appArchiveName(app, platform) }

// appAssetCount is the number of app archives a candidate carries.
func appAssetCount() int { return len(CoreSetApps) * len(shippedCoreArchives) }

// verifyApps binds every core-set app archive to the release manifest and the
// manifest to the candidate's own release identity: each app is recorded at
// the candidate's version, source commit and toolchain, in the `core` set with
// no declared skew, and each shipped archive's bytes, inventory, inner
// checksum and embedded build identity agree with its row (ARTIFACT-BIN-V0-510).
func verifyApps(files map[string][]byte, assets map[string]Asset, manifest Manifest) error {
	var apps appManifest
	if err := decodeClosed(files[appsManifestPath], &apps); err != nil {
		return fmt.Errorf("candidate release manifest: %w", err)
	}
	if apps.Schema != appManifestSchema {
		return fmt.Errorf("candidate release manifest schema is %q, expected %s", apps.Schema, appManifestSchema)
	}
	for _, skew := range apps.DeclaredSkew {
		if skew.VersionSet == coreVersionSet {
			return fmt.Errorf("release manifest declares version skew in the %s set: the installed set must be one version", coreVersionSet)
		}
	}
	commit := manifest.Sources[0].Commit
	for _, app := range CoreSetApps {
		record, present := apps.Binaries[app]
		if !present {
			return fmt.Errorf("release manifest lacks %s", app)
		}
		if record.Version != manifest.Version || record.SourceCommit != commit || record.GoVersion != manifest.GoVersion || record.VersionSet != coreVersionSet {
			return fmt.Errorf("release manifest records %s at %s@%s (%s, set %q); the candidate is %s@%s (%s, set %s)", app, record.Version, record.SourceCommit, record.GoVersion, record.VersionSet, manifest.Version, commit, manifest.GoVersion, coreVersionSet)
		}
		rows := map[string]appArchive{}
		for _, row := range record.Archives {
			if _, duplicate := rows[row.Name]; duplicate || row.Name == "" {
				return fmt.Errorf("release manifest lists %s archive %q twice", app, row.Name)
			}
			rows[row.Name] = row
		}
		for _, platform := range shippedPlatforms() {
			name := appArchiveName(app, platform)
			row, recorded := rows[name]
			if !recorded {
				return fmt.Errorf("release manifest lacks %s", name)
			}
			goos, goarch := splitPlatform(platform)
			if row.GOOS != goos || row.GOARCH != goarch {
				return fmt.Errorf("release manifest row %s is for %s/%s", name, row.GOOS, row.GOARCH)
			}
			assetPath := appArchivePath(app, platform)
			raw, present := files[assetPath]
			if !present || assets[assetPath].Role != appArchiveRole {
				return fmt.Errorf("candidate app asset %s is missing or has another role", assetPath)
			}
			if row.SHA256 != digest(raw) || row.Bytes != int64(len(raw)) {
				return fmt.Errorf("candidate app archive %s disagrees with the release manifest", name)
			}
			if _, err := verifyAppBinary(raw, app, goos, goarch, row.BinarySHA256, manifest.GoVersion, commit); err != nil {
				return err
			}
		}
	}
	return nil
}

// verifyAppBinary is the app archive check; tests stand in bytes for it as
// they do for verifyCoreBinary.
var verifyAppBinary = verifyAppArchiveBinary

// verifyAppArchiveBinary rechecks one app archive the way verifyCoreArchiveBinary
// rechecks a corvint archive: the closed inventory and modes, the binary's
// digest against the manifest row, the inner SHA256SUMS row and the embedded
// build identity for `cmd/<app>` on the target. It then requires the
// executable itself to carry the candidate's source commit, so an app built
// from another commit is refused whatever the manifest claims.
func verifyAppArchiveBinary(raw []byte, app, goos, goarch, binaryDigest, goVersion, commit string) ([]byte, error) {
	target := coreTarget{GOOS: goos, GOARCH: goarch, BinaryName: app, ArchiveName: appArchiveName(app, goos+"_"+goarch), RetainedBinary: coreDigest{SHA256: binaryDigest}}
	binary, err := verifyArchiveBinary(raw, target, modulePath+"/cmd/"+app, goVersion)
	if err != nil {
		return nil, err
	}
	settings := map[string]string{}
	if info, err := buildinfo.Read(bytes.NewReader(binary)); err == nil {
		for _, setting := range info.Settings {
			settings[setting.Key] = setting.Value
		}
	}
	if err := appCommitAgrees(binary, settings, commit); err != nil {
		return nil, fmt.Errorf("app archive %s: %w", target.ArchiveName, err)
	}
	return binary, nil
}

// appCommitAgrees reports whether an app executable was built from commit.
// The release builder stamps every core-set app with `-X main.sourceCommit`
// (ARTIFACT-BIN-V0-002) and builds with -trimpath and -buildvcs=false, which
// keep the linker flags and the Git stamp out of the build settings, so the
// stamp is read where the linker put it: the commit's 40 hex digits in the
// executable's bytes. A build that does carry a Git stamp must name the same
// commit with an unmodified tree.
func appCommitAgrees(binary []byte, settings map[string]string, commit string) error {
	if len(commit) != 40 || !bytes.Contains(binary, []byte(commit)) {
		return fmt.Errorf("executable is not stamped with source commit %s", commit)
	}
	if revision, stamped := settings["vcs.revision"]; stamped && revision != commit {
		return fmt.Errorf("executable was built from %s, not %s", revision, commit)
	}
	if settings["vcs.modified"] == "true" {
		return fmt.Errorf("executable was built from a modified tree at %s", commit)
	}
	return nil
}

// extractAppBinary adds one app's executable from its archive to the staged
// install directory beside corvint: the binary at mode 0755 after its inner
// SHA256SUMS row agrees, and nothing else. The archive's notices must be the
// bytes the corvint archive already placed there; a differing notice refuses,
// so one directory never carries two releases' notices (ARTIFACT-BIN-V0-511).
func extractAppBinary(raw []byte, stage, expectedRoot, app string) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	members := map[string][]byte{}
	var binary []byte
	for {
		header, nextErr := reader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, nextErr
		}
		clean := path.Clean(header.Name)
		if clean == expectedRoot && header.Typeflag == tar.TypeDir {
			continue
		}
		relative, inRoot := strings.CutPrefix(clean, expectedRoot+"/")
		// A member is one plain name: no separator of either platform and no "..",
		// so the notice comparison below never reads outside the stage.
		if !inRoot || relative == "" || strings.ContainsAny(relative, `/\`) || strings.Contains(relative, "..") || header.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("app archive %s member %s is outside its root or not a regular file", app, header.Name)
		}
		if _, seen := members[relative]; seen || len(members) >= 6 || header.Size < 0 || header.Size > maxInputBytes {
			return nil, fmt.Errorf("app archive %s inventory is invalid", app)
		}
		content, err := io.ReadAll(io.LimitReader(reader, header.Size+1))
		if err != nil || int64(len(content)) != header.Size {
			return nil, fmt.Errorf("read app archive %s member %s", app, header.Name)
		}
		members[relative] = content
		if relative == app {
			if header.FileInfo().Mode()&0o111 == 0 {
				return nil, fmt.Errorf("app archive %s executable is not executable", app)
			}
			binary = content
		}
	}
	if binary == nil {
		return nil, fmt.Errorf("app archive %s lacks its executable", app)
	}
	if string(members["SHA256SUMS"]) != digest(binary)+"  "+app+"\n" {
		return nil, fmt.Errorf("app archive %s internal checksum disagrees", app)
	}
	for name, content := range members {
		if name == app || name == "SHA256SUMS" {
			continue
		}
		installed, err := os.ReadFile(filepath.Join(stage, name))
		if err != nil || !bytes.Equal(installed, content) {
			return nil, fmt.Errorf("app archive %s notice %s disagrees with the installed corvint archive", app, name)
		}
	}
	if err := os.WriteFile(filepath.Join(stage, app), binary, 0o755); err != nil {
		return nil, err
	}
	return binary, nil
}

// installApps extracts every core-set app beside corvint in stage, rewrites
// the staged SHA256SUMS as one row per installed executable, and probes each
// app's --version against corvint's own line: an installed set is one release
// or it is not installed (CMS-V0-002).
func installApps(ctx context.Context, verified *VerifiedCandidate, stage, platform string) error {
	rows := map[string]string{}
	coreBinary, err := os.ReadFile(filepath.Join(stage, "corvint"))
	if err != nil {
		return err
	}
	rows["corvint"] = digest(coreBinary)
	for _, app := range CoreSetApps {
		raw, present := verified.files[appArchivePath(app, platform)]
		if !present {
			return fmt.Errorf("candidate lacks host app archive %s", appArchivePath(app, platform))
		}
		binary, err := extractAppBinary(raw, stage, app+"_"+platform, app)
		if err != nil {
			return err
		}
		rows[app] = digest(binary)
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	names := make([]string, 0, len(rows))
	for name := range rows {
		names = append(names, name)
	}
	sort.Strings(names)
	var sums strings.Builder
	for _, name := range names {
		sums.WriteString(rows[name] + "  " + name + "\n")
	}
	if err := os.WriteFile(filepath.Join(stage, "SHA256SUMS"), []byte(sums.String()), 0o644); err != nil {
		return err
	}
	core, err := docsenvelope.ParseVersionLine(verified.Manifest.CorvintVersion)
	if err != nil {
		return err
	}
	for _, app := range CoreSetApps {
		line, err := probeVersionLine(ctx, filepath.Join(stage, app), stage)
		if err != nil {
			return fmt.Errorf("installed %s: %w", app, err)
		}
		if reason := docsenvelope.Mismatch(core, line); reason != "" {
			return fmt.Errorf("installed set is not one release: %s", reason)
		}
	}
	return nil
}

// probeVersionLine runs `binary --version` the way probeCoreVersion does and
// parses the line it prints.
func probeVersionLine(ctx context.Context, binary, directory string) (versionline.Line, error) {
	stdout, err := probeStdout(ctx, binary, directory, "--version")
	if err != nil {
		return versionline.Line{}, err
	}
	return docsenvelope.ParseVersionLine(stdout)
}

// probeDocsShim runs the installed `corvint docs --help`, the shape V1-5237
// names, and requires exit 0 with output: the installed set answers the docs
// group (ARTIFACT-BIN-V0-511).
func probeDocsShim(ctx context.Context, binary, directory string) error {
	stdout, err := probeStdout(ctx, binary, directory, "docs", "--help")
	if err != nil {
		return fmt.Errorf("installed corvint docs --help: %w", err)
	}
	if strings.TrimSpace(stdout) == "" {
		return fmt.Errorf("installed corvint docs --help printed nothing")
	}
	return nil
}
