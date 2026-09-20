package releasecandidate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Beamfall/corvint/internal/companionrelease"
)

type VerifiedCandidate struct {
	Directory     string
	Manifest      Manifest
	Qualification Qualification
	files         map[string][]byte
}

var verifyForInstall = Verify

// Verify admits a closed candidate only when its checksum, manifest asset
// inventory, and qualification rows agree with every retained regular file.
func Verify(directory string) (*VerifiedCandidate, error) {
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	directories := map[string]bool{".": true}
	err = filepath.WalkDir(resolved, func(full string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if full == resolved {
			return nil
		}
		relative, relErr := filepath.Rel(resolved, full)
		if relErr != nil {
			return relErr
		}
		relative = filepath.ToSlash(relative)
		info, infoErr := entry.Info()
		if infoErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("candidate entry %s is not a real file or directory", relative)
		}
		if entry.IsDir() {
			directories[relative] = true
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("candidate entry %s is not regular", relative)
		}
		raw, readErr := readRegular(full, maxInputBytes)
		if readErr != nil {
			return readErr
		}
		files[relative] = raw
		return nil
	})
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := decodeClosed(files["MANIFEST.json"], &manifest); err != nil {
		return nil, fmt.Errorf("candidate manifest: %w", err)
	}
	if manifest.Profile != manifestProfile || !versionPattern.MatchString(manifest.Version) || manifest.CorvintVersion != "Corvint "+manifest.Version+" (build "+manifest.BuildNumber+")" || manifest.GoVersion != "go1.27.1" || len(manifest.Sources) != 2 || manifest.Sources[0].Name != "corvint" || manifest.Sources[1].Name != "corvint-tasks" {
		return nil, fmt.Errorf("candidate manifest identity is invalid")
	}
	for _, source := range manifest.Sources {
		if !objectPattern.MatchString(source.Commit) || !objectPattern.MatchString(source.Tree) {
			return nil, fmt.Errorf("candidate source identity is invalid")
		}
	}
	if err := validateCandidateChecksums(files); err != nil {
		return nil, err
	}
	assets := map[string]Asset{}
	for _, asset := range manifest.Assets {
		raw, present := files[asset.Path]
		if _, exists := assets[asset.Path]; exists || !present || asset.Role == "" || asset.Path == "MANIFEST.json" || asset.Path == "SHA256SUMS" || asset.SHA256 != digest(raw) || asset.SizeBytes != int64(len(raw)) {
			return nil, fmt.Errorf("candidate asset row %s is invalid", asset.Path)
		}
		assets[asset.Path] = asset
	}
	if len(assets) != len(files)-2 {
		return nil, fmt.Errorf("candidate asset inventory is not closed")
	}
	wantDirectories := map[string]bool{".": true}
	for name := range files {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			wantDirectories[parent] = true
		}
	}
	if len(directories) != len(wantDirectories) {
		return nil, fmt.Errorf("candidate directory inventory is not closed")
	}
	for name := range directories {
		if !wantDirectories[name] {
			return nil, fmt.Errorf("unexpected candidate directory %s", name)
		}
	}
	var qualification Qualification
	if err := decodeClosed(files["QUALIFICATION.json"], &qualification); err != nil {
		return nil, fmt.Errorf("candidate qualification: %w", err)
	}
	if err := validateQualification(qualification); err != nil {
		return nil, err
	}
	if err := validateCandidateEvidence(files, manifest, assets, qualification); err != nil {
		return nil, err
	}
	return &VerifiedCandidate{Directory: resolved, Manifest: manifest, Qualification: qualification, files: files}, nil
}

func validateCandidateChecksums(files map[string][]byte) error {
	sums, err := parseCandidateChecksums(files["SHA256SUMS"])
	if err != nil {
		return err
	}
	if len(sums) != len(files)-1 {
		return fmt.Errorf("candidate checksum inventory is not closed")
	}
	for name, raw := range files {
		if name == "SHA256SUMS" {
			continue
		}
		if sums[name] != digest(raw) {
			return fmt.Errorf("candidate checksum mismatch for %s", name)
		}
	}
	return nil
}

func validateCandidateEvidence(files map[string][]byte, manifest Manifest, assets map[string]Asset, qualification Qualification) error {
	var report coreReport
	if err := decodeClosed(files["evidence/core-verification-report.json"], &report); err != nil {
		return fmt.Errorf("candidate core report: %w", err)
	}
	if err := validateCoreReportShape(report); err != nil || report.Revision != manifest.Sources[0].Commit || report.Tree != manifest.Sources[0].Tree || report.Toolchain != manifest.GoVersion {
		return fmt.Errorf("candidate core report identity disagrees")
	}
	coreSums, err := parseChecksums(files["evidence/core-SHA256SUMS"])
	if err != nil || len(coreSums) != 5 {
		return fmt.Errorf("candidate core checksum evidence is invalid")
	}
	for _, target := range report.Targets {
		if coreSums[target.ArchiveName] != target.RetainedArchive.SHA256 {
			return fmt.Errorf("candidate core checksum report disagrees for %s", target.ArchiveName)
		}
	}
	wantRoles := map[string]int{"core-archive": 4, "core-gate-checksums": 1, "core-gate-report": 1, "companion-archive": 1, "companion-checksum": 1, "companion-installed-smoke": 1, "corvint-source": 1, "corvint-tasks-source": 1, "qualification-receipt": 1, "release-notes": 1}
	roleCounts := map[string]int{}
	for _, asset := range assets {
		if _, admitted := wantRoles[asset.Role]; !admitted {
			return fmt.Errorf("candidate asset role %s is not admitted", asset.Role)
		}
		roleCounts[asset.Role]++
	}
	for role, count := range wantRoles {
		if roleCounts[role] != count {
			return fmt.Errorf("candidate asset role %s count is %d, expected %d", role, roleCounts[role], count)
		}
	}
	for _, archive := range shippedCoreArchives {
		path := "core/" + archive
		raw, present := files[path]
		if !present || assets[path].Role != "core-archive" || coreSums[archive] != digest(raw) {
			return fmt.Errorf("candidate core asset %s disagrees", archive)
		}
		matched := false
		for _, target := range report.Targets {
			if target.ArchiveName == archive && target.RetainedArchive.SHA256 == digest(raw) && target.RetainedArchive.Bytes == int64(len(raw)) && target.AssemblyA == target.AssemblyB && target.AssemblyA == target.RetainedArchive && target.BuildA == target.BuildB && target.BuildA == target.RetainedBinary {
				matched = true
			}
		}
		if !matched {
			return fmt.Errorf("candidate core report does not bind %s", archive)
		}
	}
	archivePath, checksumPath, smokePath, err := candidateCompanionPaths(assets)
	if err != nil {
		return err
	}
	temporary, err := os.MkdirTemp("", "corvint-candidate-verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	for _, candidatePath := range []string{archivePath, checksumPath, smokePath} {
		if err := os.WriteFile(filepath.Join(temporary, filepath.Base(candidatePath)), files[candidatePath], 0o600); err != nil {
			return err
		}
	}
	bundle, err := companionrelease.VerifyRetainedBundle(temporary)
	if err != nil {
		return fmt.Errorf("candidate companion evidence: %w", err)
	}
	commit, tree, corvintSourcePath, err := bundle.CorvintSourceIdentity()
	if err != nil || commit != manifest.Sources[0].Commit || tree != manifest.Sources[0].Tree || bundle.Manifest.GoVersion != manifest.GoVersion || bundle.Manifest.GitVersion != manifest.GitVersion {
		return fmt.Errorf("candidate companion Corvint identity disagrees")
	}
	tasks, tasksSourcePath, err := companionTasksIdentity(bundle)
	if err != nil || tasks != manifest.Sources[1] {
		return fmt.Errorf("candidate companion Tasks identity disagrees")
	}
	for candidatePath, bundlePath := range map[string]string{"source/corvint-src.tar.gz": corvintSourcePath, "source/corvint-tasks-src.tar.gz": tasksSourcePath} {
		entry, present := bundle.Entry(bundlePath)
		if !present || !bytes.Equal(entry, files[candidatePath]) {
			return fmt.Errorf("candidate source archive %s disagrees", candidatePath)
		}
	}
	for _, row := range qualification.Rows {
		if row.Status == "PASS" {
			if _, present := files[row.Evidence]; !present {
				return fmt.Errorf("qualification evidence %s is absent", row.Evidence)
			}
		}
	}
	return nil
}

func candidateCompanionPaths(assets map[string]Asset) (archive, checksum, smoke string, err error) {
	for path, asset := range assets {
		switch asset.Role {
		case "companion-archive":
			if archive != "" {
				return "", "", "", fmt.Errorf("duplicate companion archive role")
			}
			archive = path
		case "companion-checksum":
			if checksum != "" {
				return "", "", "", fmt.Errorf("duplicate companion checksum role")
			}
			checksum = path
		case "companion-installed-smoke":
			if smoke != "" {
				return "", "", "", fmt.Errorf("duplicate companion smoke role")
			}
			smoke = path
		}
	}
	if archive == "" || checksum == "" || smoke == "" || filepath.Base(checksum) != filepath.Base(archive)+".sha256" || strings.TrimSuffix(filepath.Base(smoke), ".smoke.json") != strings.TrimSuffix(filepath.Base(archive), ".tar.gz") {
		return "", "", "", fmt.Errorf("candidate companion evidence set is incomplete")
	}
	return archive, checksum, smoke, nil
}

func validateQualification(qualification Qualification) error {
	platforms := []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64"}
	workflows := []string{"core-archive", "companion-bundle", "version-identity", "affected-selection", "playwright-external-discovery", "documentation-corpus-discovery", "work-queue-observation"}
	if qualification.Profile != qualificationProfile || len(qualification.Rows) != len(platforms)*len(workflows) {
		return fmt.Errorf("candidate qualification profile or row count is invalid")
	}
	want := map[string]string{}
	for _, platform := range platforms {
		for _, workflow := range workflows {
			status := "NOT_RUN"
			if workflow == "core-archive" || platform == "darwin/arm64" {
				status = "PASS"
			}
			want[platform+"\x00"+workflow] = status
		}
	}
	seen := map[string]bool{}
	for _, row := range qualification.Rows {
		key := row.Platform + "\x00" + row.Workflow
		expected, admitted := want[key]
		if seen[key] || !admitted || (row.Status != "PASS" && row.Status != "FAIL" && row.Status != "NOT_RUN") || row.Status == "FAIL" || row.Status != expected || row.Evidence == "" {
			return fmt.Errorf("invalid qualification row %s/%s", row.Platform, row.Workflow)
		}
		seen[key] = true
	}
	if len(seen) != len(want) {
		return fmt.Errorf("qualification inventory is incomplete")
	}
	return nil
}

func parseCandidateChecksums(raw []byte) (map[string]string, error) {
	rows := map[string]string{}
	previous := ""
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		parts := strings.Split(line, "  ")
		name := ""
		if len(parts) == 2 {
			name = parts[1]
		}
		if len(parts) != 2 || !digestPattern.MatchString(parts[0]) || name == "" || path.IsAbs(name) || path.Clean(name) != name || strings.HasPrefix(name, "../") || name <= previous {
			return nil, fmt.Errorf("invalid candidate checksum row %q", line)
		}
		rows[name], previous = parts[0], name
	}
	return rows, nil
}

// InstallCore extracts the host core archive into a new version/platform path.
// It never writes or changes a current/latest selector, so coexistence and
// rollback are explicit selection of an already-verified immutable path.
func InstallCore(ctx context.Context, candidateDirectory, store string) (string, error) {
	if !filepath.IsAbs(candidateDirectory) || !filepath.IsAbs(store) {
		return "", fmt.Errorf("candidate and store paths must be absolute")
	}
	verified, err := verifyForInstall(candidateDirectory)
	if err != nil {
		return "", err
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return "", fmt.Errorf("core install is unsupported on %s", runtime.GOOS)
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	archivePath := "core/corvint_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	archive, ok := verified.files[archivePath]
	if !ok {
		return "", fmt.Errorf("candidate lacks host core archive %s", archivePath)
	}
	parent := filepath.Join(store, "corvint", verified.Manifest.Version)
	target := filepath.Join(parent, platform)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(parent, ".install-")
	if err != nil {
		return "", err
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(stage)
		}
	}()
	if err := extractCoreArchive(archive, stage, "corvint_"+runtime.GOOS+"_"+runtime.GOARCH); err != nil {
		return "", err
	}
	binary := filepath.Join(stage, "corvint")
	command := exec.CommandContext(ctx, binary, "--version")
	command.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + stage, "LANG=C", "LC_ALL=C"}
	out, err := command.Output()
	if err != nil || strings.TrimSpace(string(out)) != verified.Manifest.CorvintVersion {
		return "", fmt.Errorf("installed core version identity mismatch")
	}
	if err := promoteNoReplace(parent, filepath.Base(stage), platform); err != nil {
		return "", fmt.Errorf("install without replacement: %w", err)
	}
	complete = true
	return target, nil
}

func extractCoreArchive(raw []byte, target, expectedRoot string) error {
	gzipReader, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	seen := map[string]bool{}
	for {
		header, nextErr := reader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nextErr
		}
		clean := path.Clean(header.Name)
		prefix := expectedRoot + "/"
		if clean == expectedRoot && header.Typeflag == tar.TypeDir {
			continue
		}
		if !strings.HasPrefix(clean, prefix) {
			return fmt.Errorf("core archive member %s escapes expected root", header.Name)
		}
		relative := strings.TrimPrefix(clean, prefix)
		if relative == "" || path.IsAbs(relative) || strings.HasPrefix(relative, "../") || seen[relative] {
			return fmt.Errorf("invalid core archive member %s", header.Name)
		}
		seen[relative] = true
		full := filepath.Join(target, filepath.FromSlash(relative))
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(full, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if header.Size < 0 || header.Size > maxInputBytes {
				return fmt.Errorf("core archive member %s exceeds bound", header.Name)
			}
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			content, err := io.ReadAll(io.LimitReader(reader, header.Size+1))
			if err != nil || int64(len(content)) != header.Size {
				return fmt.Errorf("read core archive member %s", header.Name)
			}
			mode := os.FileMode(0o644)
			if header.FileInfo().Mode()&0o111 != 0 {
				mode = 0o755
			}
			if err := os.WriteFile(full, content, mode); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported core archive member %s", header.Name)
		}
	}
	if !seen["corvint"] {
		return fmt.Errorf("core archive lacks corvint executable")
	}
	return nil
}
