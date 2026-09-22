package archiveverify

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"go/ast"
	"go/parser"
	"go/token"
	"hash/crc32"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func canonicalMembers() []member {
	return []member{
		{name: "corvint_linux_amd64/LICENSE", mode: 0o644, data: []byte("license")},
		{name: "corvint_linux_amd64/SHA256SUMS", mode: 0o644, data: []byte("sum")},
		{name: "corvint_linux_amd64/corvint", mode: 0o755, data: []byte("binary")},
	}
}

func canonicalContainerAccepted(t *testing.T, format string, raw []byte, want []member, maximum int64) {
	t.Helper()
	input := Input{Format: format, ArchiveBytes: raw, MaximumContentBytes: maximum}
	got, err := readMembers(input)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := compareMembers(got, want); err != nil {
		t.Fatalf("members: %v", err)
	}
	rebuilt, err := encodeCanonical(format, want)
	if err != nil || !bytes.Equal(raw, rebuilt) {
		t.Fatalf("canonical mismatch: %v", err)
	}
}

func TestIndependentCanonicalReaders(t *testing.T) {
	files := canonicalMembers()
	for _, format := range []string{"tar.gz", "zip"} {
		raw, err := encodeCanonical(format, files)
		if err != nil {
			t.Fatal(err)
		}
		canonicalContainerAccepted(t, format, raw, files, 1024)
	}
}

func TestRawFramingMutationsFailCanonicalComparison(t *testing.T) {
	files := canonicalMembers()
	for _, format := range []string{"tar.gz", "zip"} {
		raw, err := encodeCanonical(format, files)
		if err != nil {
			t.Fatal(err)
		}
		mutations := map[string][]byte{
			"leading":  append([]byte{0}, raw...),
			"trailing": append(append([]byte(nil), raw...), 0),
			"bit-flip": func() []byte { changed := append([]byte(nil), raw...); changed[len(changed)/2] ^= 1; return changed }(),
		}
		for name, changed := range mutations {
			t.Run(format+"-"+name, func(t *testing.T) {
				input := Input{Format: format, ArchiveBytes: changed, MaximumContentBytes: 1024}
				got, readErr := readMembers(input)
				rebuilt, rebuildErr := encodeCanonical(format, files)
				if readErr == nil && rebuildErr == nil && compareMembers(got, files) == nil && bytes.Equal(changed, rebuilt) {
					t.Fatal("mutation accepted")
				}
			})
		}
	}
}

func TestClosedInventoryMutationMatrix(t *testing.T) {
	want := canonicalMembers()
	mutations := map[string][]member{
		"missing":   append([]member(nil), want[:len(want)-1]...),
		"surplus":   append(append([]member(nil), want...), member{name: "corvint_linux_amd64/extra", mode: 0o644, data: []byte("x")}),
		"reordered": {want[1], want[0], want[2]},
		"nested":    {want[0], want[1], {name: "corvint_linux_amd64/payload.zip", mode: 0o644, data: []byte("PK")}},
	}
	for name, files := range mutations {
		t.Run(name, func(t *testing.T) {
			for _, format := range []string{"tar.gz", "zip"} {
				raw, err := encodeCanonical(format, files)
				if err != nil {
					t.Fatal(err)
				}
				got, err := readMembers(Input{Format: format, ArchiveBytes: raw, MaximumContentBytes: 1024})
				if err == nil && compareMembers(got, want) == nil {
					t.Fatalf("%s %s inventory accepted", format, name)
				}
			}
		})
	}
}

func TestTarGzipCanonicalMetadataMutationMatrix(t *testing.T) {
	want := canonicalMembers()
	tests := map[string]struct {
		files  []member
		osByte byte
		mutate func(*tar.Header, int)
	}{
		"gzip-os":   {files: want, osByte: 3},
		"owner":     {files: want, osByte: 255, mutate: func(header *tar.Header, _ int) { header.Uid = 1 }},
		"timestamp": {files: want, osByte: 255, mutate: func(header *tar.Header, _ int) { header.ModTime = time.Unix(1, 0) }},
		"mode": {files: want, osByte: 255, mutate: func(header *tar.Header, index int) {
			if index == 0 {
				header.Mode = 0o600
			}
		}},
		"order": {files: []member{want[1], want[0], want[2]}, osByte: 255},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			raw := encodeTarFixture(t, test.files, test.osByte, test.mutate)
			assertNotCanonical(t, "tar.gz", raw, want)
		})
	}
}

func TestZIPCanonicalMetadataMutationMatrix(t *testing.T) {
	want := canonicalMembers()
	tests := map[string]struct {
		files  []member
		mutate func(*zip.FileHeader, int)
	}{
		"creator":   {files: want, mutate: func(header *zip.FileHeader, _ int) { header.CreatorVersion = 20 }},
		"timestamp": {files: want, mutate: func(header *zip.FileHeader, _ int) { header.ModifiedDate = 0x22 }},
		"mode": {files: want, mutate: func(header *zip.FileHeader, index int) {
			if index == 0 {
				header.ExternalAttrs = uint32((0o100000|0o600)&0xffff) << 16
			}
		}},
		"order": {files: []member{want[1], want[0], want[2]}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			raw := encodeZIPFixture(t, test.files, test.mutate)
			assertNotCanonical(t, "zip", raw, want)
		})
	}
}

func assertNotCanonical(t *testing.T, format string, raw []byte, want []member) {
	t.Helper()
	got, readErr := readMembers(Input{Format: format, ArchiveBytes: raw, MaximumContentBytes: 1024})
	canonical, canonicalErr := encodeCanonical(format, want)
	if readErr == nil && canonicalErr == nil && compareMembers(got, want) == nil && bytes.Equal(raw, canonical) {
		t.Fatal("noncanonical fixture accepted")
	}
}

func encodeTarFixture(t *testing.T, files []member, osByte byte, mutate func(*tar.Header, int)) []byte {
	t.Helper()
	var output bytes.Buffer
	gz, err := gzip.NewWriterLevel(&output, gzip.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	gz.Header = gzip.Header{ModTime: time.Unix(0, 0), OS: osByte}
	tw := tar.NewWriter(gz)
	for index, file := range files {
		header := &tar.Header{Name: file.name, Mode: file.mode, Size: int64(len(file.data)), Typeflag: tar.TypeReg, Format: tar.FormatUSTAR, ModTime: time.Unix(0, 0)}
		if mutate != nil {
			mutate(header, index)
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(file.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func encodeZIPFixture(t *testing.T, files []member, mutate func(*zip.FileHeader, int)) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for index, file := range files {
		header := &zip.FileHeader{Name: file.name, Method: zip.Store, CreatorVersion: uint16(3<<8) | 20, ReaderVersion: 20, ModifiedDate: 0x21, CRC32: crc32.ChecksumIEEE(file.data), CompressedSize64: uint64(len(file.data)), UncompressedSize64: uint64(len(file.data)), ExternalAttrs: uint32((0o100000|file.mode)&0xffff) << 16}
		if mutate != nil {
			mutate(header, index)
		}
		entry, err := writer.CreateRaw(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(file.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestTarSpecialAndZIPDescriptorEncryptionAreRejected(t *testing.T) {
	for _, typeflag := range []byte{tar.TypeSymlink, tar.TypeLink, tar.TypeChar, tar.TypeBlock, tar.TypeFifo, tar.TypeGNUSparse, tar.TypeXHeader} {
		var tarBuffer bytes.Buffer
		gz := gzip.NewWriter(&tarBuffer)
		tw := tar.NewWriter(gz)
		header := &tar.Header{Name: "root/special", Typeflag: typeflag, Linkname: "x", Format: tar.FormatGNU}
		if typeflag == tar.TypeXHeader {
			header.Format = tar.FormatPAX
		}
		if err := tw.WriteHeader(header); err != nil {
			continue
		}
		_ = tw.Close()
		_ = gz.Close()
		if _, err := readTarGzip(tarBuffer.Bytes(), 1024); err == nil {
			t.Fatalf("tar type %q accepted", typeflag)
		}
	}
	var zipBuffer bytes.Buffer
	zw := zip.NewWriter(&zipBuffer)
	header := &zip.FileHeader{Name: "root/a", Method: zip.Store, Flags: 1}
	entry, err := zw.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte("x"))
	_ = zw.Close()
	if _, err := readZIP(zipBuffer.Bytes(), 1024); err == nil || !strings.Contains(err.Error(), "noncanonical-zip") {
		t.Fatalf("encrypted/descriptor zip accepted: %v", err)
	}
}

func TestZIPCommentExtraCompressionAndArchiveCommentAreRejected(t *testing.T) {
	for name, configure := range map[string]func(*zip.FileHeader){
		"member-comment": func(header *zip.FileHeader) { header.Comment = "x" },
		"extra":          func(header *zip.FileHeader) { header.Extra = []byte{1, 0, 0, 0} },
		"compression":    func(header *zip.FileHeader) { header.Method = zip.Deflate },
	} {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			writer := zip.NewWriter(&output)
			header := &zip.FileHeader{Name: "root/a", Method: zip.Store}
			configure(header)
			entry, err := writer.CreateHeader(header)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = entry.Write([]byte("x"))
			_ = writer.Close()
			if _, err := readZIP(output.Bytes(), 1024); err == nil {
				t.Fatal("noncanonical zip accepted")
			}
		})
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	_ = writer.SetComment("archive-comment")
	_ = writer.Close()
	if _, err := readZIP(output.Bytes(), 1024); err == nil {
		t.Fatal("archive comment accepted")
	}
}

func TestResourceLimitsFailClosed(t *testing.T) {
	files := []member{{name: "root/a", mode: 0o644, data: bytes.Repeat([]byte("x"), 64)}}
	for _, format := range []string{"tar.gz", "zip"} {
		raw, err := encodeCanonical(format, files)
		if err != nil {
			t.Fatal(err)
		}
		input := Input{Format: format, ArchiveBytes: raw, MaximumContentBytes: 10}
		if _, err := readMembers(input); err == nil || !strings.Contains(err.Error(), "resource-limit") {
			t.Fatalf("%s limit accepted: %v", format, err)
		}
	}
}

func TestForbiddenMemberPathMatrix(t *testing.T) {
	for _, name := range []string{"", "/abs", ".", "../x", "root/../x", "root\\x", "root//x", "root/./x"} {
		if err := validateName(name, map[string]struct{}{}, map[string]struct{}{}); err == nil {
			t.Fatalf("unsafe path %q accepted", name)
		}
	}
	seen, folded := map[string]struct{}{}, map[string]struct{}{}
	if err := validateName("root/a", seen, folded); err != nil {
		t.Fatal(err)
	}
	if err := validateName("root/a", seen, folded); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate accepted: %v", err)
	}
	if err := validateName("ROOT/A", seen, folded); err == nil || !strings.Contains(err.Error(), "casefold") {
		t.Fatalf("casefold collision accepted: %v", err)
	}
}

func TestChecksumGrammarAndBindingMatrix(t *testing.T) {
	digest := strings.Repeat("a", 64)
	valid := []byte(digest + "  corvint\n")
	if err := verifyChecksum(valid, "corvint", digest); err != nil {
		t.Fatal(err)
	}
	mutations := [][]byte{
		[]byte(strings.ToUpper(digest) + "  corvint\n"),
		[]byte(digest + " corvint\n"),
		[]byte(digest + " *corvint\n"),
		[]byte(digest + "  corvint"),
		[]byte(digest + "  corvint\r\n"),
		[]byte(digest + "  other\n"),
		[]byte(digest + "  corvint\n" + digest + "  other\n"),
		append(append([]byte(nil), valid...), 0),
	}
	for index, mutation := range mutations {
		if err := verifyChecksum(mutation, "corvint", digest); err == nil {
			t.Fatalf("checksum mutation %d accepted", index)
		}
	}
}

func TestBuildInfoTargetRevisionAndProfileMatrix(t *testing.T) {
	commit := strings.Repeat("a", 40)
	input := Input{GOOS: "linux", GOARCH: "amd64", Commit: commit, GoVersion: "go1.27.1", PackagePath: "github.com/Beamfall/corvint/cmd/corvint"}
	clean := map[string]string{"GOOS": "linux", "GOARCH": "amd64", "CGO_ENABLED": "0", "-trimpath": "true", "vcs.revision": commit, "vcs.modified": "false"}
	if err := verifyBuildInfoValues(input, input.GoVersion, input.PackagePath, 0, clean); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct{ key, value string }{
		{"GOOS", "darwin"}, {"GOARCH", "arm64"}, {"CGO_ENABLED", "1"}, {"-trimpath", "false"},
		{"vcs.revision", strings.Repeat("b", 40)}, {"vcs.modified", "true"},
	} {
		changed := map[string]string{}
		for key, value := range clean {
			changed[key] = value
		}
		changed[mutation.key] = mutation.value
		if err := verifyBuildInfoValues(input, input.GoVersion, input.PackagePath, 0, changed); err == nil {
			t.Fatalf("build info mutation %s accepted", mutation.key)
		}
	}
	for name, arguments := range map[string]struct {
		goVersion, packagePath string
		dependencies           int
	}{
		"toolchain":  {"go1.28.0", input.PackagePath, 0},
		"package":    {input.GoVersion, "example.com/other", 0},
		"dependency": {input.GoVersion, input.PackagePath, 1},
	} {
		if err := verifyBuildInfoValues(input, arguments.goVersion, arguments.packagePath, arguments.dependencies, clean); err == nil {
			t.Fatalf("%s mismatch accepted", name)
		}
	}
}

func TestDoubleBuildAssemblyAndIndependenceEvidence(t *testing.T) {
	emptyDigest := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	manifest := []byte(`{"schema":"corvint.release-artifact-v0","toolchain":{"goVersion":"go1.27.1","goModDirective":"1.27.1","forbiddenGoModDirective":"toolchain","gotoolchain":"local"},"profile":{"package":"./cmd/corvint","modulePath":"github.com/Beamfall/corvint","binaryName":"corvint","buildFlags":["-trimpath"],"environment":{"CGO_ENABLED":"0","GOENV":"off","GOTOOLCHAIN":"local","GOPROXY":"off","GOFLAGS":"-mod=readonly","GOSUMDB":"off"}},"targets":[{"goos":"darwin","goarch":"amd64","binarySuffix":""},{"goos":"darwin","goarch":"arm64","binarySuffix":""},{"goos":"linux","goarch":"amd64","binarySuffix":""},{"goos":"linux","goarch":"arm64","binarySuffix":""},{"goos":"windows","goarch":"amd64","binarySuffix":".exe"}],"legalFiles":[{"path":"LICENSE","sha256":"` + emptyDigest + `"},{"path":"LICENSE-APACHE-2.0","sha256":"` + emptyDigest + `"},{"path":"LICENSING.md","sha256":"` + emptyDigest + `"},{"path":"PROVENANCE.md","sha256":"` + emptyDigest + `"}],"smoke":{},"pendingEvidence":[]}`)
	base := Input{
		GOOS: "linux", GOARCH: "amd64", ArchiveName: "corvint_linux_amd64.tar.gz", Format: "tar.gz", Root: "corvint_linux_amd64", BinaryName: "corvint",
		Commit: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40), GoVersion: "go1.27.1", PackagePath: "github.com/Beamfall/corvint/cmd/corvint", ManifestBytes: manifest,
		LooseBinary: []byte("a"), SecondBinary: []byte("b"), LooseGateBinary: []byte("a"), ArchiveBytes: []byte("a"), SecondArchive: []byte("a"),
		LegalFiles: []ExpectedFile{{Name: "LICENSE"}, {Name: "LICENSE-APACHE-2.0"}, {Name: "LICENSING.md"}, {Name: "PROVENANCE.md"}}, MaximumArchiveBytes: 10, MaximumContentBytes: 10,
	}
	legalMutation := base
	legalMutation.LegalFiles = append([]ExpectedFile(nil), base.LegalFiles...)
	legalMutation.LegalFiles[0].Data = []byte("changed")
	if _, err := Verify(legalMutation); err == nil || !strings.Contains(err.Error(), "legal-authority") {
		t.Fatalf("legal mutation accepted: %v", err)
	}
	manifestMutation := base
	manifestMutation.ManifestBytes = bytes.Replace(base.ManifestBytes, []byte(`"GOENV":"off"`), []byte(`"GOENV":"ambient"`), 1)
	if _, err := Verify(manifestMutation); err == nil || !strings.Contains(err.Error(), "environment-authority") {
		t.Fatalf("manifest mutation accepted: %v", err)
	}
	if _, err := Verify(base); err == nil || !strings.Contains(err.Error(), "double-build") {
		t.Fatalf("binary mismatch accepted: %v", err)
	}
	base.SecondBinary = base.LooseBinary
	looseGateMutation := base
	looseGateMutation.LooseGateBinary = []byte("other")
	if _, err := Verify(looseGateMutation); err == nil || !strings.Contains(err.Error(), "loose-gate-binary") {
		t.Fatalf("loose reproducibility-gate mismatch accepted: %v", err)
	}
	base.SecondArchive = []byte("b")
	if _, err := Verify(base); err == nil || !strings.Contains(err.Error(), "double-assembly") {
		t.Fatalf("archive mismatch accepted: %v", err)
	}
	base.SecondArchive = base.ArchiveBytes
	targetMutation := base
	targetMutation.GOOS, targetMutation.GOARCH = "freebsd", "386"
	if _, err := Verify(targetMutation); err == nil || !strings.Contains(err.Error(), "target-authority") {
		t.Fatalf("unpinned target accepted: %v", err)
	}
}

func TestVerifierProductionFilesCannotImportAssemblerOrExecution(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			name, _ := strconv.Unquote(imported.Path.Value)
			if strings.Contains(name, "archivebuild") || name == "os/exec" || strings.HasPrefix(name, "net") {
				t.Fatalf("%s imports forbidden dependency %s", entry.Name(), name)
			}
		}
		ast.Inspect(file, func(node ast.Node) bool { return true })
	}
	if _, err := os.Stat(filepath.Join("..", "archivebuild", "archivebuild.go")); err != nil {
		t.Fatal(err)
	}
}
