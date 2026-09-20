package releasecandidate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const maxInputBytes = 512 << 20

var (
	digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	objectPattern = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
)

var coreFiles = []string{
	"SHA256SUMS",
	"corvint_darwin_amd64.tar.gz",
	"corvint_darwin_arm64.tar.gz",
	"corvint_linux_amd64.tar.gz",
	"corvint_linux_arm64.tar.gz",
	"corvint_windows_amd64.zip",
	"verification-report.json",
}

var shippedCoreArchives = []string{
	"corvint_darwin_amd64.tar.gz",
	"corvint_darwin_arm64.tar.gz",
	"corvint_linux_amd64.tar.gz",
	"corvint_linux_arm64.tar.gz",
}

func verifyCore(directory string) (coreReport, map[string][]byte, error) {
	var zero coreReport
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return zero, nil, err
	}
	entries, err := os.ReadDir(resolved)
	if err != nil {
		return zero, nil, err
	}
	if len(entries) != len(coreFiles) {
		return zero, nil, fmt.Errorf("core artifact set has %d entries, expected %d", len(entries), len(coreFiles))
	}
	want := map[string]bool{}
	for _, name := range coreFiles {
		want[name] = true
	}
	files := map[string][]byte{}
	for _, entry := range entries {
		info, infoErr := entry.Info()
		if infoErr != nil || !want[entry.Name()] || !info.Mode().IsRegular() {
			return zero, nil, fmt.Errorf("unexpected core artifact entry %q", entry.Name())
		}
		raw, readErr := readRegular(filepath.Join(resolved, entry.Name()), maxInputBytes)
		if readErr != nil {
			return zero, nil, readErr
		}
		files[entry.Name()] = raw
	}
	sums, err := parseChecksums(files["SHA256SUMS"])
	if err != nil {
		return zero, nil, err
	}
	if len(sums) != 5 {
		return zero, nil, fmt.Errorf("core checksum set has %d rows, expected 5", len(sums))
	}
	for _, name := range coreFiles[1:6] {
		if sums[name] != digest(files[name]) {
			return zero, nil, fmt.Errorf("core checksum mismatch for %s", name)
		}
	}
	var report coreReport
	if err := decodeClosed(files["verification-report.json"], &report); err != nil {
		return zero, nil, fmt.Errorf("core report: %w", err)
	}
	if err := validateCoreReport(report, files); err != nil {
		return zero, nil, err
	}
	return report, files, nil
}

func validateCoreReport(report coreReport, files map[string][]byte) error {
	if err := validateCoreReportShape(report); err != nil {
		return err
	}
	for _, target := range report.Targets {
		if target.RetainedArchive.SHA256 != digest(files[target.ArchiveName]) || target.RetainedArchive.Bytes != int64(len(files[target.ArchiveName])) {
			return fmt.Errorf("core target %s retained archive disagrees", target.ArchiveName)
		}
		if _, err := verifyCoreBinary(files[target.ArchiveName], target); err != nil {
			return err
		}
	}
	return nil
}

func validateCoreReportShape(report coreReport) error {
	if report.Schema != "corvint.release-go-archive-report.v2" || report.Verdict != "PASS" || report.Toolchain != "go1.27.1" || len(report.Reasons) != 0 || !objectPattern.MatchString(report.Revision) || !objectPattern.MatchString(report.Tree) || !digestPattern.MatchString(report.ManifestSHA256) {
		return fmt.Errorf("core report is not an exact PASS report")
	}
	expected := []struct{ goos, goarch, binary, archive string }{
		{"darwin", "amd64", "corvint", "corvint_darwin_amd64.tar.gz"},
		{"darwin", "arm64", "corvint", "corvint_darwin_arm64.tar.gz"},
		{"linux", "amd64", "corvint", "corvint_linux_amd64.tar.gz"},
		{"linux", "arm64", "corvint", "corvint_linux_arm64.tar.gz"},
		{"windows", "amd64", "corvint.exe", "corvint_windows_amd64.zip"},
	}
	if len(report.Targets) != len(expected) || report.Limits.Targets != len(expected) || report.Limits.MembersPerArchive != 6 || report.Limits.MaximumArchiveBytes <= 0 || report.Limits.MaximumArchiveBytes > 64<<20 || report.Limits.MaximumTotalBytes < report.Limits.MaximumArchiveBytes || report.Limits.MaximumTotalBytes > 5*(64<<20) || report.Limits.MaximumReportBytes != 1<<20 || report.Limits.MaximumWitnessBytes != 64<<10 {
		return fmt.Errorf("core report limits or target count are invalid")
	}
	for index, target := range report.Targets {
		want := expected[index]
		if target.GOOS != want.goos || target.GOARCH != want.goarch || target.BinaryName != want.binary || target.ArchiveName != want.archive {
			return fmt.Errorf("core report target %d disagrees", index)
		}
		if target.BuildA != target.BuildB || target.BuildA != target.RetainedBinary || target.AssemblyA != target.AssemblyB || target.AssemblyA != target.RetainedArchive {
			return fmt.Errorf("core target %s reproducibility evidence disagrees", target.ArchiveName)
		}
		for _, row := range []coreDigest{target.BuildA, target.BuildB, target.AssemblyA, target.AssemblyB, target.RetainedBinary, target.RetainedArchive} {
			if !digestPattern.MatchString(row.SHA256) || row.Bytes <= 0 {
				return fmt.Errorf("core target %s has invalid digest evidence", target.ArchiveName)
			}
		}
	}
	return nil
}

func parseChecksums(raw []byte) (map[string]string, error) {
	rows := map[string]string{}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, fmt.Errorf("empty checksum file")
	}
	previous := ""
	for _, line := range lines {
		parts := strings.Split(line, "  ")
		if len(parts) != 2 || !digestPattern.MatchString(parts[0]) || parts[1] == "" || filepath.Base(parts[1]) != parts[1] || parts[1] <= previous {
			return nil, fmt.Errorf("invalid checksum row %q", line)
		}
		if _, exists := rows[parts[1]]; exists {
			return nil, fmt.Errorf("duplicate checksum row %s", parts[1])
		}
		rows[parts[1]], previous = parts[0], parts[1]
	}
	return rows, nil
}

func decodeClosed(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}

func readRegular(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return nil, fmt.Errorf("read bounded %s: %w", path, err)
	}
	return raw, nil
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func renderChecksums(files map[string][]byte) []byte {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var output strings.Builder
	for _, name := range names {
		fmt.Fprintf(&output, "%s  %s\n", digest(files[name]), name)
	}
	return []byte(output.String())
}
