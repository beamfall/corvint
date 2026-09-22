package main

// Status values are closed. NOT_RUN is visible evidence of an unexecuted
// check, never a skip and never a pass.
const (
	statusPass   = "PASS"
	statusFail   = "FAIL"
	statusNotRun = "NOT_RUN"
)

// Reason is the typed refusal vocabulary. Every failure names exactly one kind
// so a red gate is diagnosable without reading the transcript.
const (
	reasonToolchainMismatch    = "toolchain-mismatch"
	reasonGoModDirective       = "gomod-directive-mismatch"
	reasonGoModToolchainLine   = "gomod-toolchain-directive-present"
	reasonDirtyTree            = "dirty-worktree"
	reasonOutputInsideRepo     = "output-inside-repository"
	reasonBuildFailed          = "build-failed"
	reasonNotByteIdentical     = "not-byte-identical"
	reasonBuildInfoMismatch    = "buildinfo-mismatch"
	reasonUndeclaredDependency = "undeclared-runtime-dependency"
	reasonLegalFileMissing     = "legal-file-missing"
	reasonLegalFileAltered     = "legal-file-altered"
	reasonSmokeFailed          = "smoke-failed"
)

type Reason struct {
	Kind   string `json:"kind"`
	Target string `json:"target,omitempty"`
	Detail string `json:"detail"`
}

type Report struct {
	Schema       string           `json:"schema"`
	Commit       string           `json:"commit"`
	Tree         string           `json:"tree"`
	Toolchain    ToolchainReport  `json:"toolchain"`
	Profile      ProfileReport    `json:"profile"`
	Targets      []TargetReport   `json:"targets"`
	LegalFiles   []LegalReport    `json:"legalFiles"`
	Dependencies DependencyReport `json:"dependencies"`
	Pending      []string         `json:"pendingEvidence"`
	Verdict      string           `json:"verdict"`
	Reasons      []Reason         `json:"reasons"`
	// build is the build number stamped into every target (PUB-V0-021).
	build string
}

type ToolchainReport struct {
	SelectedGoVersion string `json:"selectedGoVersion"`
	ExpectedGoVersion string `json:"expectedGoVersion"`
	GoToolchainEnv    string `json:"gotoolchainEnv"`
	GoModDirective    string `json:"goModDirective"`
	ToolchainLines    int    `json:"goModToolchainLines"`
	Status            string `json:"status"`
}

type ProfileReport struct {
	Package     string            `json:"package"`
	BinaryName  string            `json:"binaryName"`
	BuildFlags  []string          `json:"buildFlags"`
	Environment map[string]string `json:"environment"`
}

// BuildReport records one of the two same-profile builds. DistinctCache is
// recorded because a shared build cache would prove cache reuse, not
// determinism.
type BuildReport struct {
	SHA256        string `json:"sha256"`
	Bytes         int64  `json:"bytes"`
	DistinctCache string `json:"distinctCache"`
}

type TargetReport struct {
	GOOS          string            `json:"goos"`
	GOARCH        string            `json:"goarch"`
	Artifact      string            `json:"artifact"`
	BuildA        BuildReport       `json:"buildA"`
	BuildB        BuildReport       `json:"buildB"`
	ByteIdentical bool              `json:"byteIdentical"`
	Divergence    string            `json:"divergence,omitempty"`
	SHA256        string            `json:"sha256"`
	BuildInfo     map[string]string `json:"buildInfo"`
	Native        bool              `json:"native"`
	Smoke         SmokeReport       `json:"smoke"`
}

type SmokeReport struct {
	Status              string `json:"status"`
	Reason              string `json:"reason,omitempty"`
	VersionOutput       string `json:"versionOutput,omitempty"`
	QueryIntent         string `json:"queryIntent,omitempty"`
	QueryOK             bool   `json:"queryOk"`
	RepositoryUnchanged bool   `json:"repositoryUnchanged"`
}

type LegalReport struct {
	Path     string `json:"path"`
	SHA256   string `json:"sha256"`
	Expected string `json:"expectedSha256"`
	Status   string `json:"status"`
}

type DependencyReport struct {
	ModuleRequirements int      `json:"moduleRequirements"`
	BinaryDependencies []string `json:"binaryDependencies"`
	Status             string   `json:"status"`
}

func (r *Report) fail(kind, target, detail string) {
	r.Reasons = append(r.Reasons, Reason{Kind: kind, Target: target, Detail: detail})
}

func (r Report) byteIdenticalCount() int {
	count := 0
	for _, target := range r.Targets {
		if target.ByteIdentical {
			count++
		}
	}
	return count
}

func (r Report) smokeCount(status string) int {
	count := 0
	for _, target := range r.Targets {
		if target.Smoke.Status == status {
			count++
		}
	}
	return count
}
