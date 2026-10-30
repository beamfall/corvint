// Package archivewire contains data-only request and result types shared
// across the archive builder and the separately compiled offline verifier.
package archivewire

const (
	RequestSchema       = "corvint.release-go-archive-verifier-request.v1"
	ResultSchema        = "corvint.release-go-archive-verifier-result.v1"
	MaxMembers          = 6
	HardMaxArchiveBytes = int64(64 << 20)
	HardMaxContentBytes = int64(64 << 20)
)

type ExpectedFile struct {
	Name string `json:"name"`
	Mode int64  `json:"mode"`
	Data []byte `json:"data"`
}

type Input struct {
	ArchiveName         string
	Format              string
	Root                string
	BinaryName          string
	GOOS                string
	GOARCH              string
	Commit              string
	Tree                string
	GoVersion           string
	PackagePath         string
	ManifestBytes       []byte
	ArchiveBytes        []byte
	LooseBinary         []byte
	SecondBinary        []byte
	LooseGateBinary     []byte
	SecondArchive       []byte
	LegalFiles          []ExpectedFile
	MaximumArchiveBytes int64
	MaximumContentBytes int64
}

type Request struct {
	Schema              string         `json:"schema"`
	ArchiveName         string         `json:"archiveName"`
	Format              string         `json:"format"`
	Root                string         `json:"root"`
	BinaryName          string         `json:"binaryName"`
	GOOS                string         `json:"goos"`
	GOARCH              string         `json:"goarch"`
	Commit              string         `json:"commit"`
	Tree                string         `json:"tree"`
	GoVersion           string         `json:"goVersion"`
	PackagePath         string         `json:"packagePath"`
	ManifestBytes       []byte         `json:"manifestBytes"`
	ArchiveAPath        string         `json:"archiveAPath"`
	ArchiveBPath        string         `json:"archiveBPath"`
	BinaryAPath         string         `json:"binaryAPath"`
	BinaryBPath         string         `json:"binaryBPath"`
	LooseGateBinaryPath string         `json:"looseGateBinaryPath"`
	LegalFiles          []ExpectedFile `json:"legalFiles"`
	MaximumArchiveBytes int64          `json:"maximumArchiveBytes"`
	MaximumContentBytes int64          `json:"maximumContentBytes"`
}

type Result struct {
	Schema        string `json:"schema"`
	Verdict       string `json:"verdict"`
	Reason        string `json:"reason,omitempty"`
	ArchiveSHA256 string `json:"archiveSha256,omitempty"`
	ArchiveBytes  int64  `json:"archiveBytes,omitempty"`
	BinarySHA256  string `json:"binarySha256,omitempty"`
	BinaryBytes   int64  `json:"binaryBytes,omitempty"`
}
