// Package archiveverify independently verifies Go release archives.
//
// It deliberately does not import the assembler package. Canonical container
// bytes are reconstructed here so a shared encoder bug cannot satisfy both
// sides of the gate.
package archiveverify

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/conformance/release-artifact-v0/archivewire"
)

type ExpectedFile = archivewire.ExpectedFile
type Input = archivewire.Input
type Result = archivewire.Result

type member struct {
	name string
	mode int64
	data []byte
}

func Verify(input Input) (Result, error) {
	result := Result{Schema: archivewire.ResultSchema, Verdict: "FAIL",
		ArchiveSHA256: digest(input.ArchiveBytes), ArchiveBytes: int64(len(input.ArchiveBytes)),
		BinarySHA256: digest(input.LooseBinary), BinaryBytes: int64(len(input.LooseBinary)),
	}
	if input.MaximumArchiveBytes <= 0 || input.MaximumArchiveBytes > archivewire.HardMaxArchiveBytes || input.MaximumContentBytes <= 0 || input.MaximumContentBytes > archivewire.HardMaxContentBytes {
		return result, fmt.Errorf("resource-limit")
	}
	if err := verifyManifestAuthority(input); err != nil {
		return result, err
	}
	wantArchive, wantFormat := "corvint_"+input.GOOS+"_"+input.GOARCH+".tar.gz", "tar.gz"
	if input.GOOS == "windows" {
		wantArchive, wantFormat = "corvint_windows_amd64.zip", "zip"
	}
	wantRoot := strings.TrimSuffix(strings.TrimSuffix(wantArchive, ".gz"), ".tar")
	if wantFormat == "zip" {
		wantRoot = strings.TrimSuffix(wantArchive, ".zip")
	}
	if input.ArchiveName != wantArchive || input.Format != wantFormat || input.Root != wantRoot {
		return result, fmt.Errorf("archive-target-shape-mismatch")
	}
	wantBinary := "corvint"
	if input.GOOS == "windows" {
		wantBinary += ".exe"
	}
	if input.BinaryName != wantBinary || len(input.LegalFiles) != 4 {
		return result, fmt.Errorf("archive-inventory-authority-mismatch")
	}
	for index, name := range []string{"LICENSE", "LICENSE-APACHE-2.0", "LICENSING.md", "PROVENANCE.md"} {
		if input.LegalFiles[index].Name != name {
			return result, fmt.Errorf("archive-legal-inventory-mismatch")
		}
	}
	if !bytes.Equal(input.LooseBinary, input.SecondBinary) {
		return result, fmt.Errorf("double-build-mismatch")
	}
	if !bytes.Equal(input.LooseBinary, input.LooseGateBinary) {
		return result, fmt.Errorf("loose-gate-binary-mismatch")
	}
	if !bytes.Equal(input.ArchiveBytes, input.SecondArchive) {
		return result, fmt.Errorf("double-assembly-mismatch")
	}
	if int64(len(input.ArchiveBytes)) > input.MaximumArchiveBytes || int64(len(input.SecondArchive)) > input.MaximumArchiveBytes {
		return result, fmt.Errorf("archive-resource-limit")
	}
	members, err := readMembers(input)
	if err != nil {
		return result, err
	}
	expected := expectedMembers(input)
	if err := compareMembers(members, expected); err != nil {
		return result, err
	}
	binaryPath := path.Join(input.Root, input.BinaryName)
	binary := memberByName(members, binaryPath)
	if !bytes.Equal(binary.data, input.LooseBinary) {
		return result, fmt.Errorf("loose-binary-mismatch")
	}
	checksumPath := path.Join(input.Root, "SHA256SUMS")
	if err := verifyChecksum(memberByName(members, checksumPath).data, input.BinaryName, result.BinarySHA256); err != nil {
		return result, fmt.Errorf("binary-checksum-mismatch")
	}
	if err := verifyBuildInfo(input, binary.data); err != nil {
		return result, err
	}
	canonical, err := encodeCanonical(input.Format, expected)
	if err != nil {
		return result, err
	}
	if !bytes.Equal(canonical, input.ArchiveBytes) {
		return result, fmt.Errorf("noncanonical-container-framing-or-metadata")
	}
	result.Verdict = "PASS"
	return result, nil
}

type verifierManifest struct {
	Schema    string `json:"schema"`
	Toolchain struct {
		GoVersion      string `json:"goVersion"`
		GoModDirective string `json:"goModDirective"`
		Forbidden      string `json:"forbiddenGoModDirective"`
		GoToolchain    string `json:"gotoolchain"`
	} `json:"toolchain"`
	Profile struct {
		Package     string            `json:"package"`
		ModulePath  string            `json:"modulePath"`
		BinaryName  string            `json:"binaryName"`
		BuildFlags  []string          `json:"buildFlags"`
		Environment map[string]string `json:"environment"`
	} `json:"profile"`
	Targets []struct {
		GOOS   string `json:"goos"`
		GOARCH string `json:"goarch"`
		Suffix string `json:"binarySuffix"`
	} `json:"targets"`
	Legal []struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"legalFiles"`
	Smoke   json.RawMessage `json:"smoke"`
	Pending json.RawMessage `json:"pendingEvidence"`
}

func verifyManifestAuthority(input Input) error {
	var manifest verifierManifest
	if err := archivewire.DecodeStrict(input.ManifestBytes, &manifest); err != nil {
		return fmt.Errorf("manifest-authority-invalid")
	}
	if manifest.Schema != "corvint.release-artifact-v0" || manifest.Toolchain.GoVersion != "go1.27.1" || manifest.Toolchain.GoModDirective != "1.27.1" || manifest.Toolchain.Forbidden != "toolchain" || manifest.Toolchain.GoToolchain != "local" {
		return fmt.Errorf("manifest-toolchain-authority-mismatch")
	}
	if input.GoVersion != "go1.27.1" || input.PackagePath != "github.com/Beamfall/corvint/cmd/corvint" || !hexString(input.Commit, 40) || !hexString(input.Tree, 40) {
		return fmt.Errorf("request-immutable-identity-mismatch")
	}
	if manifest.Profile.Package != "./cmd/corvint" || manifest.Profile.ModulePath != "github.com/Beamfall/corvint" || manifest.Profile.BinaryName != "corvint" || len(manifest.Profile.BuildFlags) != 1 || manifest.Profile.BuildFlags[0] != "-trimpath" {
		return fmt.Errorf("manifest-profile-authority-mismatch")
	}
	wantEnvironment := map[string]string{"CGO_ENABLED": "0", "GOENV": "off", "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOFLAGS": "-mod=readonly", "GOSUMDB": "off"}
	if len(manifest.Profile.Environment) != len(wantEnvironment) {
		return fmt.Errorf("manifest-environment-authority-mismatch")
	}
	for key, value := range wantEnvironment {
		if manifest.Profile.Environment[key] != value {
			return fmt.Errorf("manifest-environment-authority-mismatch")
		}
	}
	wantTargets := [][3]string{{"darwin", "amd64", ""}, {"darwin", "arm64", ""}, {"linux", "amd64", ""}, {"linux", "arm64", ""}, {"windows", "amd64", ".exe"}}
	if len(manifest.Targets) != len(wantTargets) {
		return fmt.Errorf("manifest-target-authority-mismatch")
	}
	for index, want := range wantTargets {
		got := manifest.Targets[index]
		if got.GOOS != want[0] || got.GOARCH != want[1] || got.Suffix != want[2] {
			return fmt.Errorf("manifest-target-authority-mismatch")
		}
	}
	wantSuffix, targetFound := "", false
	for _, want := range wantTargets {
		if input.GOOS == want[0] && input.GOARCH == want[1] {
			wantSuffix, targetFound = want[2], true
			break
		}
	}
	if !targetFound || input.BinaryName != manifest.Profile.BinaryName+wantSuffix {
		return fmt.Errorf("request-target-authority-mismatch")
	}
	wantLegal := []string{"LICENSE", "LICENSE-APACHE-2.0", "LICENSING.md", "PROVENANCE.md"}
	if len(manifest.Legal) != len(wantLegal) || len(input.LegalFiles) != len(wantLegal) {
		return fmt.Errorf("manifest-legal-authority-mismatch")
	}
	for index, name := range wantLegal {
		if manifest.Legal[index].Path != name || input.LegalFiles[index].Name != name || digest(input.LegalFiles[index].Data) != manifest.Legal[index].SHA256 {
			return fmt.Errorf("manifest-legal-authority-mismatch")
		}
	}
	return nil
}

func hexString(value string, length int) bool {
	if len(value) != length {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}

func expectedMembers(input Input) []member {
	binaryDigest := digest(input.LooseBinary)
	files := []member{
		{name: path.Join(input.Root, input.BinaryName), mode: 0o755, data: input.LooseBinary},
		{name: path.Join(input.Root, "SHA256SUMS"), mode: 0o644, data: []byte(binaryDigest + "  " + input.BinaryName + "\n")},
	}
	for _, legal := range input.LegalFiles {
		files = append(files, member{name: path.Join(input.Root, legal.Name), mode: 0o644, data: legal.Data})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return files
}

func readMembers(input Input) ([]member, error) {
	switch input.Format {
	case "tar.gz":
		return readTarGzip(input.ArchiveBytes, input.MaximumContentBytes)
	case "zip":
		return readZIP(input.ArchiveBytes, input.MaximumContentBytes)
	default:
		return nil, fmt.Errorf("unsupported-archive-format")
	}
}

func readTarGzip(raw []byte, maximumContent int64) ([]member, error) {
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid-gzip-framing: %w", err)
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	var members []member
	seen, folded := map[string]struct{}{}, map[string]struct{}{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid-tar-framing: %w", err)
		}
		if len(members) == archivewire.MaxMembers {
			return nil, fmt.Errorf("surplus-member")
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return nil, fmt.Errorf("special-member")
		}
		if header.Format != tar.FormatUSTAR {
			return nil, fmt.Errorf("non-ustar-member")
		}
		if header.Uid != 0 || header.Gid != 0 || header.Uname != "" || header.Gname != "" {
			return nil, fmt.Errorf("noncanonical-owner-metadata")
		}
		if !header.ModTime.Equal(time.Unix(0, 0)) || !header.AccessTime.IsZero() || !header.ChangeTime.IsZero() {
			return nil, fmt.Errorf("noncanonical-timestamp")
		}
		if err := validateName(header.Name, seen, folded); err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(reader, maximumContent+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > maximumContent {
			return nil, fmt.Errorf("member-resource-limit")
		}
		maximumContent -= int64(len(data))
		members = append(members, member{name: header.Name, mode: header.Mode, data: data})
	}
	return members, nil
}

func readZIP(raw []byte, maximumContent int64) ([]member, error) {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("invalid-zip-framing: %w", err)
	}
	if reader.Comment != "" || len(reader.File) > archivewire.MaxMembers {
		return nil, fmt.Errorf("zip-comment-or-surplus-member")
	}
	seen, folded := map[string]struct{}{}, map[string]struct{}{}
	members := make([]member, 0, len(reader.File))
	for _, file := range reader.File {
		if err := validateName(file.Name, seen, folded); err != nil {
			return nil, err
		}
		if file.Method != zip.Store || file.Flags != 0 || len(file.Extra) != 0 || file.Comment != "" {
			return nil, fmt.Errorf("noncanonical-zip-member")
		}
		if file.ModifiedDate != 0x21 || file.ModifiedTime != 0 || file.CreatorVersion != uint16(3<<8)|20 || file.ReaderVersion != 20 {
			return nil, fmt.Errorf("noncanonical-zip-metadata")
		}
		if file.ExternalAttrs != uint32((0o100000|file.Mode().Perm())&0xffff)<<16 {
			return nil, fmt.Errorf("noncanonical-zip-external-attributes")
		}
		mode := int64(file.Mode().Perm())
		if mode != 0o755 && mode != 0o644 {
			return nil, fmt.Errorf("noncanonical-mode")
		}
		handle, err := file.Open()
		if err != nil {
			return nil, err
		}
		if int64(file.UncompressedSize64) > maximumContent {
			return nil, fmt.Errorf("member-resource-limit")
		}
		data, readErr := io.ReadAll(io.LimitReader(handle, maximumContent+1))
		closeErr := handle.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if int64(len(data)) > maximumContent {
			return nil, fmt.Errorf("member-resource-limit")
		}
		maximumContent -= int64(len(data))
		members = append(members, member{name: file.Name, mode: mode, data: data})
	}
	return members, nil
}

func validateName(name string, seen, folded map[string]struct{}) error {
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") || path.Clean(name) != name || strings.HasPrefix(name, "../") || name == "." {
		return fmt.Errorf("unsafe-member-path")
	}
	if _, exists := seen[name]; exists {
		return fmt.Errorf("duplicate-member")
	}
	seen[name] = struct{}{}
	fold := strings.ToLower(name)
	if _, exists := folded[fold]; exists {
		return fmt.Errorf("casefold-collision")
	}
	folded[fold] = struct{}{}
	return nil
}

func compareMembers(got, want []member) error {
	if len(got) != len(want) {
		return fmt.Errorf("closed-member-inventory-mismatch")
	}
	for index := range want {
		if got[index].name != want[index].name {
			return fmt.Errorf("member-order-or-name-mismatch")
		}
		if got[index].mode != want[index].mode {
			return fmt.Errorf("member-mode-mismatch")
		}
		if !bytes.Equal(got[index].data, want[index].data) {
			return fmt.Errorf("member-bytes-mismatch:%s", want[index].name)
		}
	}
	return nil
}

func memberByName(members []member, name string) member {
	for _, candidate := range members {
		if candidate.name == name {
			return candidate
		}
	}
	return member{}
}

func verifyBuildInfo(input Input, binary []byte) error {
	info, err := buildinfo.Read(bytes.NewReader(binary))
	if err != nil {
		return fmt.Errorf("buildinfo-unreadable: %w", err)
	}
	settings := map[string]string{}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	return verifyBuildInfoValues(input, info.GoVersion, info.Path, len(info.Deps), settings)
}

func verifyBuildInfoValues(input Input, goVersion, packagePath string, dependencyCount int, settings map[string]string) error {
	expected := map[string]string{
		"GOOS": input.GOOS, "GOARCH": input.GOARCH, "CGO_ENABLED": "0", "-trimpath": "true",
		"vcs.revision": input.Commit, "vcs.modified": "false",
	}
	if goVersion != input.GoVersion || packagePath != input.PackagePath || dependencyCount != 0 {
		return fmt.Errorf("buildinfo-identity-mismatch")
	}
	for key, value := range expected {
		if settings[key] != value {
			return fmt.Errorf("buildinfo-setting-mismatch:%s", key)
		}
	}
	return nil
}

func verifyChecksum(content []byte, binaryName, binaryDigest string) error {
	want := binaryDigest + "  " + binaryName + "\n"
	if string(content) != want {
		return fmt.Errorf("checksum grammar or binding mismatch")
	}
	return nil
}

func encodeCanonical(format string, files []member) ([]byte, error) {
	if format == "zip" {
		return encodeZIP(files)
	}
	return encodeTarGzip(files)
}

func encodeTarGzip(files []member) ([]byte, error) {
	var output bytes.Buffer
	gz, err := gzip.NewWriterLevel(&output, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	gz.Header = gzip.Header{ModTime: time.Unix(0, 0), OS: 255}
	tw := tar.NewWriter(gz)
	for _, file := range files {
		header := &tar.Header{Name: file.name, Mode: file.mode, Size: int64(len(file.data)), Typeflag: tar.TypeReg, Format: tar.FormatUSTAR, ModTime: time.Unix(0, 0)}
		if err := tw.WriteHeader(header); err != nil {
			return nil, err
		}
		if _, err := tw.Write(file.data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func encodeZIP(files []member) ([]byte, error) {
	var output bytes.Buffer
	zw := zip.NewWriter(&output)
	for _, file := range files {
		header := &zip.FileHeader{Name: file.name, Method: zip.Store, CreatorVersion: uint16(3<<8) | 20, ReaderVersion: 20, ModifiedDate: 0x21, CRC32: crc32.ChecksumIEEE(file.data), CompressedSize64: uint64(len(file.data)), UncompressedSize64: uint64(len(file.data)), ExternalAttrs: uint32((0o100000|file.mode)&0xffff) << 16}
		entry, err := zw.CreateRaw(header)
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write(file.data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func digest(content []byte) string {
	value := sha256.Sum256(content)
	return hex.EncodeToString(value[:])
}
