// Package releasecandidate closes the already-qualified core and companion
// artifacts into one immutable, versioned local publication candidate.
package releasecandidate

type Options struct {
	CoreDirectory      string
	CompanionDirectory string
	SourceRoot         string
	Scratch            string
	OutputParent       string
	Version            string
}

type Asset struct {
	Path      string `json:"path"`
	Role      string `json:"role"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"sizeBytes"`
}

type SourceIdentity struct {
	Name   string `json:"name"`
	Commit string `json:"commit"`
	Tree   string `json:"tree"`
}

type Manifest struct {
	Profile        string           `json:"profile"`
	Version        string           `json:"version"`
	BuildNumber    string           `json:"buildNumber"`
	CorvintVersion string           `json:"corvintVersion"`
	GoVersion      string           `json:"goVersion"`
	GitVersion     string           `json:"gitVersion"`
	Sources        []SourceIdentity `json:"sources"`
	Assets         []Asset          `json:"assets"`
}

type QualificationRow struct {
	Platform string `json:"platform"`
	Workflow string `json:"workflow"`
	Status   string `json:"status"`
	Evidence string `json:"evidence"`
}

type Qualification struct {
	Profile string             `json:"profile"`
	Rows    []QualificationRow `json:"rows"`
}

type Result struct {
	Directory     string
	Manifest      Manifest
	Qualification Qualification
}

type coreReport struct {
	Schema         string       `json:"schema"`
	Revision       string       `json:"revision"`
	Tree           string       `json:"tree"`
	ManifestSHA256 string       `json:"manifestSha256"`
	Toolchain      string       `json:"toolchain"`
	Verdict        string       `json:"verdict"`
	Limits         coreLimits   `json:"limits"`
	Targets        []coreTarget `json:"targets"`
	Reasons        []coreReason `json:"reasons"`
}

type coreLimits struct {
	Targets             int   `json:"targets"`
	MembersPerArchive   int   `json:"membersPerArchive"`
	MaximumArchiveBytes int64 `json:"maximumArchiveBytes"`
	MaximumTotalBytes   int64 `json:"maximumTotalArchiveBytes"`
	MaximumReportBytes  int64 `json:"maximumReportBytes"`
	MaximumWitnessBytes int64 `json:"maximumWitnessBytes"`
}

type coreDigest struct {
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type coreTarget struct {
	GOOS            string     `json:"goos"`
	GOARCH          string     `json:"goarch"`
	BinaryName      string     `json:"binaryName"`
	ArchiveName     string     `json:"archiveName"`
	BuildA          coreDigest `json:"buildA"`
	BuildB          coreDigest `json:"buildB"`
	AssemblyA       coreDigest `json:"assemblyA"`
	AssemblyB       coreDigest `json:"assemblyB"`
	RetainedBinary  coreDigest `json:"retainedBinary"`
	RetainedArchive coreDigest `json:"retainedArchive"`
}

type coreReason struct {
	Code   string `json:"code"`
	Target string `json:"target,omitempty"`
}
