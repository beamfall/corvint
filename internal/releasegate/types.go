// Package releasegate verifies one manifest-selected, immutable Git release tree.
// It is experimental evidence only; it never builds, publishes, or promotes a release.
package releasegate

import "context"

const (
	maxTreeBytes     = 64 << 20
	maxBlobBytes     = 32 << 20
	maxManifestBytes = 1 << 20
	maxArchiveBytes  = 32 << 20
	maxArchiveFiles  = 1_000
	maxArchiveTotal  = 64 << 20
	maxScanBytes     = 128 << 20
	maxArchiveDepth  = 3
)

type Status string

const (
	Pass        Status = "PASS"
	Fail        Status = "FAIL"
	Unsupported Status = "UNSUPPORTED"
	NotRun      Status = "NOT_RUN"
)

// Allowance names an inert, unshipped fixture blob. It never exempts an archive
// member or an artifact-inventory member from scanning.
type Allowance struct {
	Path       string `json:"path"`
	BlobSHA256 string `json:"blobSha256"`
}

// Evidence locates exactly the bytes which produced a finding. Spans are
// zero-based half-open byte offsets in the named blob or archive member.
// EnclosingMembers lists, outermost first, the archive member paths that
// contain MemberPath's archive when the member is nested.
type Evidence struct {
	Path             string   `json:"path"`
	BlobOID          string   `json:"blobOid"`
	BlobSHA256       string   `json:"blobSha256"`
	EnclosingMembers []string `json:"enclosingMembers,omitempty"`
	MemberPath       string   `json:"memberPath,omitempty"`
	MemberSHA        string   `json:"memberSha256,omitempty"`
	SpanStart        int      `json:"spanStart"`
	SpanEnd          int      `json:"spanEnd"`
	SpanSHA256       string   `json:"spanSha256"`
	Line             int      `json:"line,omitempty"`
	Column           int      `json:"column,omitempty"`
}

type Finding struct {
	Kind     string   `json:"kind"`
	Detail   string   `json:"detail"`
	Evidence Evidence `json:"evidence"`
}

// Checks is deliberately complete for GPK-V0-026. This experimental gate can
// only decide safety; all other evidence remains visible, never implied.
type Checks struct {
	Parity          Status `json:"parity"`
	Safety          Status `json:"safety"`
	Performance     Status `json:"performance"`
	Packaging       Status `json:"packaging"`
	CorvintDogfood  Status `json:"corvintDogfood"`
	BeamfallDogfood Status `json:"beamfallDogfood"`
}

type Report struct {
	Commit             string    `json:"commit"`
	Tree               string    `json:"tree"`
	ManifestPath       string    `json:"manifestPath"`
	ManifestBlobOID    string    `json:"manifestBlobOid"`
	ManifestBlobSHA256 string    `json:"manifestBlobSha256"`
	GitExecutable      string    `json:"gitExecutable,omitempty"`
	GitSHA256          string    `json:"gitSha256,omitempty"`
	Checks             Checks    `json:"checks"`
	Findings           []Finding `json:"findings"`
}

// Blocking reports are intentionally non-promotable. A caller must inspect
// the report rather than treating a zero findings list as release approval.
func (r Report) Blocking() bool {
	return r.Checks.Parity != Pass || r.Checks.Safety != Pass ||
		r.Checks.Performance != Pass || r.Checks.Packaging != Pass ||
		r.Checks.CorvintDogfood != Pass || r.Checks.BeamfallDogfood != Pass
}

type Options struct {
	Root          string
	Commit        string
	ManifestPath  string // slash-separated path in the scanned commit tree
	GitExecutable string // externally supplied absolute executable path
	GitSHA256     string // externally pinned executable digest
	PolicyPath    string // externally supplied absolute policy path
	PolicySHA256  string // externally pinned policy digest
}

// Manifest is canonical only when decoded from ManifestPath in the exact
// scanned tree. Report records both the manifest blob identities and the
// resolved commit/tree identities, binding policy and scan without a
// self-referential manifest hash.
// ArtifactInventory is the complete accepted shipping inventory, sorted and
// unique. BuildPackages are the selected production package roots.
type Manifest struct {
	ArtifactInventory    []string            `json:"artifactInventory"`
	ArtifactModes        map[string]string   `json:"artifactModes"`
	ArtifactSHA256       string              `json:"artifactSha256"`
	Allowances           []Allowance         `json:"allowances"`
	BuildPackages        []string            `json:"buildPackages"`
	BuildTargets         []BuildTarget       `json:"buildTargets"`
	CorePackages         []string            `json:"corePackages"`
	AnalyzerPackages     []string            `json:"analyzerPackages"`
	PluginPackages       []string            `json:"pluginPackages"`
	CoreSizeCeiling      int64               `json:"coreSizeCeilingBytes"`
	PluginSizeCeilings   map[string]int64    `json:"pluginSizeCeilingBytes"`
	PluginArtifacts      map[string][]string `json:"pluginArtifacts"`
	AnalyzerSizeCeilings map[string]int64    `json:"analyzerSizeCeilingBytes"`
}
type BuildTarget struct {
	GOOS        string   `json:"goos"`
	GOARCH      string   `json:"goarch"`
	Compiler    string   `json:"compiler"`
	ReleaseTags []string `json:"releaseTags"`
	CustomTags  []string `json:"customTags"`
	CGOEnabled  bool     `json:"cgoEnabled"`
}

type treeEntry struct {
	mode, oid string
	size      int64
	path      string
}
type gitRun func(context.Context, string, int, ...string) ([]byte, error)
