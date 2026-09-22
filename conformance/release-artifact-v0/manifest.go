package main

import (
	"fmt"
	"os"
	"regexp"
)

const manifestSchema = "corvint.release-artifact-v0"

// requiredTargets is the exact platform set GPK-V0-018 names for the release
// gate. A manifest that omits one is rejected; a dropped target must be a spec
// change, never a quiet manifest edit.
var requiredTargets = []Target{
	{GOOS: "darwin", GOARCH: "amd64"},
	{GOOS: "darwin", GOARCH: "arm64"},
	{GOOS: "linux", GOARCH: "amd64"},
	{GOOS: "linux", GOARCH: "arm64"},
	{GOOS: "windows", GOARCH: "amd64", Suffix: ".exe"},
}

var requiredLegalFiles = []string{"LICENSE", "LICENSE-APACHE-2.0", "LICENSING.md", "PROVENANCE.md"}

// Manifest is the frozen release profile. It is the authority for what is
// built and how; the checker never infers a target, a flag, or a digest.
type Manifest struct {
	Schema    string      `json:"schema"`
	Toolchain Toolchain   `json:"toolchain"`
	Profile   Profile     `json:"profile"`
	Targets   []Target    `json:"targets"`
	Legal     []LegalFile `json:"legalFiles"`
	Smoke     Smoke       `json:"smoke"`
	Pending   []string    `json:"pendingEvidence"`
}

// Toolchain pins the exact selected local toolchain. GPK-V0-018 requires the
// gate to fail rather than download or select another.
type Toolchain struct {
	GoVersion          string `json:"goVersion"`
	GoModDirective     string `json:"goModDirective"`
	ForbiddenDirective string `json:"forbiddenGoModDirective"`
	GoToolchainEnv     string `json:"gotoolchain"`
}

type Profile struct {
	Package     string            `json:"package"`
	ModulePath  string            `json:"modulePath"`
	BinaryName  string            `json:"binaryName"`
	BuildFlags  []string          `json:"buildFlags"`
	Environment map[string]string `json:"environment"`
}

type Target struct {
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
	Suffix string `json:"binarySuffix"`
}

func (t Target) id() string { return t.GOOS + "/" + t.GOARCH }

// LegalFile pins the exact bytes GPK-V0-020 forbids the migration to alter.
type LegalFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Smoke struct {
	VersionArgument    string `json:"versionArgument"`
	ExpectedVersion    string `json:"expectedVersion"`
	QueryTask          string `json:"queryTask"`
	QueryLimit         string `json:"queryLimit"`
	ExpectedIntent     string `json:"expectedIntent"`
	FixtureInstruction string `json:"fixtureInstructions"`
	CrossCompileReason string `json:"crossCompileNotRunReason"`
}

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var gitObjectPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

func loadManifest(path string) (Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := decodeStrictJSON(raw, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("manifest %s: %w", path, err)
	}
	if err := manifest.validate(); err != nil {
		return Manifest{}, fmt.Errorf("manifest %s: %w", path, err)
	}
	return manifest, nil
}

// validate is fail-closed: every profile property GPK-V0-018 mandates is
// checked here, so a weakened manifest cannot silently weaken the gate.
func (m Manifest) validate() error {
	if m.Schema != manifestSchema {
		return fmt.Errorf("schema must be %q", manifestSchema)
	}
	if err := m.Toolchain.validate(); err != nil {
		return err
	}
	if err := m.Profile.validate(); err != nil {
		return err
	}
	if err := m.validateTargets(); err != nil {
		return err
	}
	if err := m.validateLegal(); err != nil {
		return err
	}
	return m.Smoke.validate()
}

func (t Toolchain) validate() error {
	if t.GoVersion != "go1.27.1" {
		return fmt.Errorf("toolchain.goVersion must be exactly go1.27.1")
	}
	if t.GoModDirective != "1.27.1" {
		return fmt.Errorf("toolchain.goModDirective must be exactly 1.27.1")
	}
	if t.ForbiddenDirective != "toolchain" {
		return fmt.Errorf("toolchain.forbiddenGoModDirective must be %q", "toolchain")
	}
	if t.GoToolchainEnv != "local" {
		return fmt.Errorf("toolchain.gotoolchain must be %q", "local")
	}
	return nil
}

func (p Profile) validate() error {
	if p.BinaryName != "corvint" || p.Package != "./cmd/corvint" || p.ModulePath != "github.com/Beamfall/corvint" {
		return fmt.Errorf("profile package, modulePath, and binaryName must equal the pinned Corvint profile")
	}
	if len(p.BuildFlags) != 1 || p.BuildFlags[0] != "-trimpath" {
		return fmt.Errorf("profile.buildFlags must be exactly [-trimpath]")
	}
	return p.validateEnvironment()
}

// validateEnvironment enforces the CGO, offline, and toolchain-selection
// properties as manifest invariants rather than caller discipline.
func (p Profile) validateEnvironment() error {
	required := map[string]string{"CGO_ENABLED": "0", "GOENV": "off", "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOFLAGS": "-mod=readonly", "GOSUMDB": "off"}
	if len(p.Environment) != len(required) {
		return fmt.Errorf("profile.environment must contain exactly the closed release environment")
	}
	for key, want := range required {
		if p.Environment[key] != want {
			return fmt.Errorf("profile.environment[%s] must be %q, found %q", key, want, p.Environment[key])
		}
	}
	return nil
}

func (m Manifest) validateTargets() error {
	if len(m.Targets) != len(requiredTargets) {
		return fmt.Errorf("manifest targets must contain exactly %d ordered rows", len(requiredTargets))
	}
	for index, required := range requiredTargets {
		if m.Targets[index] != required {
			return fmt.Errorf("manifest target row %d is %s suffix %q; want %s suffix %q", index, m.Targets[index].id(), m.Targets[index].Suffix, required.id(), required.Suffix)
		}
	}
	return nil
}

func (m Manifest) validateLegal() error {
	if len(m.Legal) != len(requiredLegalFiles) {
		return fmt.Errorf("legalFiles must contain exactly %d ordered files", len(requiredLegalFiles))
	}
	for index, file := range m.Legal {
		if file.Path != requiredLegalFiles[index] || !digestPattern.MatchString(file.SHA256) {
			return fmt.Errorf("legal file %q requires a lowercase 64-hex sha256", file.Path)
		}
	}
	return nil
}

func (s Smoke) validate() error {
	if s.VersionArgument == "" || s.ExpectedVersion == "" {
		return fmt.Errorf("smoke.versionArgument and smoke.expectedVersion are required")
	}
	if s.QueryTask == "" || s.QueryLimit == "" || s.ExpectedIntent == "" {
		return fmt.Errorf("smoke.queryTask, smoke.queryLimit, and smoke.expectedIntent are required")
	}
	if s.CrossCompileReason == "" {
		return fmt.Errorf("smoke.crossCompileNotRunReason is required; a cross-built target is NOT_RUN, never inferred")
	}
	return nil
}
