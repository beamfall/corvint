package analyzerdotnet

import (
	"bytes"
	"encoding/json"
	"io"
	"path"
	"strings"
)

// admissibleInput is the family-to-path gate, applied before any content is
// decoded. Pinning each family's filename is load-bearing rather than
// cosmetic: the fact matrix freezes which input family may witness which fact,
// so without this gate a Directory.Build.props submitted under the
// `dotnet.project` family could mint `dotnet.target.framework` facts the matrix
// reserves for a real project file.
func admissibleInput(input Input) string {
	base := path.Base(input.Path)
	switch input.Family {
	case "dotnet.global-json":
		return admit(base == "global.json")
	case "dotnet.slnx":
		return admit(strings.HasSuffix(base, ".slnx") && len(base) > len(".slnx"))
	case "dotnet.project":
		return admit(projectExtension(base))
	case "dotnet.central-packages":
		return admit(base == "Directory.Packages.props")
	case "dotnet.directory-build":
		return admit(base == "Directory.Build.props" || base == "Directory.Build.targets")
	case "dotnet.sln", "dotnet.packages-lock-v1", "dotnet.nuget-config":
		// Named by the matrix, deliberately not implemented by this candidate.
		// The whole family rejects; it is never accepted and ignored.
		return "UNSUPPORTED_SCHEMA"
	}
	return "UNKNOWN_FAMILY"
}

func admit(ok bool) string {
	if ok {
		return ""
	}
	return "UNSUPPORTED_SCHEMA"
}

func projectExtension(base string) bool {
	for _, extension := range [...]string{".csproj", ".fsproj", ".vbproj"} {
		if strings.HasSuffix(base, extension) && len(base) > len(extension) {
			return true
		}
	}
	return false
}

// parseInto routes one already-digest-proven input to its family parser.
func parseInto(input Input, body []byte, collector *factCollector) string {
	switch input.Family {
	case "dotnet.global-json":
		return parseGlobalJSON(input, body, collector)
	case "dotnet.slnx":
		return parseSolutionXML(input, body, collector)
	case "dotnet.central-packages":
		return parseCentralPackages(input, body, collector)
	case "dotnet.project", "dotnet.directory-build":
		return parseProject(input, body, collector)
	}
	return "UNSUPPORTED_SCHEMA"
}

type globalDocument struct {
	SDK *globalSDK `json:"sdk"`
}

type globalSDK struct {
	Version *string `json:"version"`
}

// parseGlobalJSON accepts exactly {"sdk":{"version":"8.0.423"|"9.0.316"}}. The
// value is a declaration about the SDK a repository asks for, never a fact
// about an installed SDK: nothing on this host is consulted.
func parseGlobalJSON(input Input, body []byte, collector *factCollector) string {
	trimmed := bytes.TrimRight(body, " \t\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return "MALFORMED_INPUT"
	}
	var document globalDocument
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return "UNKNOWN_FIELD"
	}
	// Decode stops at the end of the first value, so anything after the object
	// would otherwise be accepted and ignored. This runs before the duplicate
	// scan so that trailing bytes are reported as malformed input rather than
	// as the duplicate the shared scanner would also flag them as.
	if _, err := decoder.Token(); err != io.EOF {
		return "MALFORMED_INPUT"
	}
	if !boundedDistinctNames(trimmed) {
		return "DUPLICATE_VALUE"
	}
	if document.SDK == nil || document.SDK.Version == nil {
		return "MALFORMED_INPUT"
	}
	if !dotnetSDKVersion(*document.SDK.Version) {
		return "UNSUPPORTED_SCHEMA"
	}
	return collector.add(input, "dotnet.sdk.declaration", "dotnet-sdk", "declares",
		*document.SDK.Version, collector.request.ScopeID)
}

// parseSolutionXML reads the XML solution format. Its namespace is empty, its
// root is Solution, and its only child shape is Project with a single Path.
func parseSolutionXML(input Input, body []byte, collector *factCollector) string {
	root, why := parseClosedXML(body, "")
	if why != "" {
		return why
	}
	if root.name != "Solution" {
		return "UNSUPPORTED_SCHEMA"
	}
	if why := requireNoAttributes(root); why != "" {
		return why
	}
	if !root.blank() {
		return "MALFORMED_INPUT"
	}
	base := baseDirectory(input.Path)
	seen := map[string]bool{}
	for _, child := range root.children {
		if child.name != "Project" {
			return "UNSUPPORTED_SCHEMA"
		}
		if why := requireEmpty(child); why != "" {
			return why
		}
		if len(child.attributes) != 1 {
			return "UNKNOWN_FIELD"
		}
		raw, present := child.attribute("Path")
		if !present {
			return "UNKNOWN_FIELD"
		}
		if why := noExpansion(raw); why != "" {
			return why
		}
		resolved, why := resolveRelative(base, raw)
		if why != "" {
			return why
		}
		if seen[resolved] {
			return "DUPLICATE_VALUE"
		}
		seen[resolved] = true
		if why := collector.add(input, "dotnet.solution.project", collector.request.ScopeID,
			"contains-project", resolved, resolved); why != "" {
			return why
		}
	}
	return ""
}

// parseCentralPackages reads Directory.Packages.props.
//
// A PackageVersion only governs a restore when central management is switched
// on, so a file whose ManagePackageVersionsCentrally is absent or false is
// validated and yields nothing. Emitting `central-version` from it would assert
// a resolution MSBuild would not perform.
func parseCentralPackages(input Input, body []byte, collector *factCollector) string {
	root, why := parseClosedXML(body, "", msbuildNamespace)
	if why != "" {
		return why
	}
	if root.name != "Project" {
		return "UNSUPPORTED_SCHEMA"
	}
	if why := requireNoAttributes(root); why != "" {
		return why
	}
	if !root.blank() {
		return "MALFORMED_INPUT"
	}
	central, switched := false, false
	declared := make([]attribute, 0, 8)
	seen := map[string]bool{}
	for _, group := range root.children {
		if group.name != "PropertyGroup" && group.name != "ItemGroup" {
			return "UNSUPPORTED_SCHEMA"
		}
		if why := requirePlainGroup(group); why != "" {
			return why
		}
		if group.name == "PropertyGroup" {
			enabled, why := centralSwitch(group, &switched)
			if why != "" {
				return why
			}
			central = central || enabled
			continue
		}
		versions, why := packageVersions(group, seen)
		if why != "" {
			return why
		}
		declared = append(declared, versions...)
	}
	if !central {
		return ""
	}
	for _, version := range declared {
		if why := collector.add(input, "dotnet.package.central", version.name,
			"central-version", version.value, collector.request.ScopeID); why != "" {
			return why
		}
	}
	return ""
}

// centralSwitch reads one PropertyGroup. switched spans every group in the
// file: a second declaration is a duplicate wherever it appears, since MSBuild's
// last-wins reading of it is an evaluation this profile does not perform.
func centralSwitch(group element, switched *bool) (bool, string) {
	enabled := false
	for _, property := range group.children {
		if property.name != "ManagePackageVersionsCentrally" {
			return false, "UNSUPPORTED_SCHEMA"
		}
		if why := requireNoAttributes(property); why != "" {
			return false, why
		}
		if len(property.children) != 0 {
			return false, "UNSUPPORTED_SCHEMA"
		}
		if !dotnetBoolean(property.text) {
			return false, "UNSUPPORTED_SCHEMA"
		}
		if *switched {
			return false, "DUPLICATE_VALUE"
		}
		*switched = true
		enabled = property.text == "true"
	}
	return enabled, ""
}

func packageVersions(group element, seen map[string]bool) ([]attribute, string) {
	collected := make([]attribute, 0, len(group.children))
	for _, item := range group.children {
		if item.name != "PackageVersion" {
			return nil, "UNSUPPORTED_SCHEMA"
		}
		if why := requireEmpty(item); why != "" {
			return nil, why
		}
		if len(item.attributes) != 2 {
			return nil, "UNKNOWN_FIELD"
		}
		include, hasInclude := item.attribute("Include")
		version, hasVersion := item.attribute("Version")
		if !hasInclude || !hasVersion {
			return nil, "UNKNOWN_FIELD"
		}
		if !dotnetName(include) || !coreVersion(version) {
			return nil, "UNSUPPORTED_SCHEMA"
		}
		if seen[include] {
			return nil, "DUPLICATE_VALUE"
		}
		seen[include] = true
		collected = append(collected, attribute{name: include, value: version})
	}
	return collected, ""
}
