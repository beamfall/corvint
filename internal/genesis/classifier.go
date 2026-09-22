package genesis

import (
	"bytes"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	classifications  = [...]string{"INCLUDED", "EXCLUDED", "UNSUPPORTED", "UNKNOWN"}
	sourceClassOrder = [...]string{
		"INSTRUCTIONS",
		"DOCUMENTATION",
		"SPECIFICATION",
		"TEST",
		"E2E_TEST",
		"CI",
		"MANIFEST",
		"LOCKFILE",
		"OWNERSHIP",
		"RUNBOOK",
		"INCIDENT",
		"SCHEMA",
		"CONFIGURATION",
		"CODE",
		"ASSET",
		"OTHER_TEXT",
	}
	boundaryOrder = [...]string{"GENERATED_CANDIDATE", "VENDOR_CANDIDATE"}
)

var specificationName = regexp.MustCompile(`(?:^|[-_.])(adr|prd|rfc|spec|tdd)(?:[-_.]|$)`)

var codeSuffixes = stringSet(
	".c", ".cc", ".clj", ".cljs", ".cpp", ".cs", ".css", ".dart",
	".ex", ".exs", ".fs", ".fsx", ".go", ".h", ".hpp", ".html",
	".java", ".js", ".jsx", ".kt", ".kts", ".less", ".lua", ".m",
	".mm", ".php", ".pl", ".pm", ".py", ".rb", ".rs", ".sass",
	".scala", ".scss", ".sh", ".sql", ".svelte", ".swift", ".tsx",
	".ts", ".vue", ".zig",
)

var documentSuffixes = stringSet(".adoc", ".md", ".mdx", ".rst", ".txt")
var configSuffixes = stringSet(".cfg", ".conf", ".ini", ".json", ".toml", ".xml", ".yaml", ".yml")
var schemaSuffixes = stringSet(".graphql", ".graphqls", ".proto", ".sql")
var binaryAssetSuffixes = stringSet(
	".7z", ".a", ".avi", ".bmp", ".class", ".db", ".dylib", ".eot",
	".exe", ".gif", ".gz", ".ico", ".jar", ".jpeg", ".jpg", ".mov",
	".mp3", ".mp4", ".o", ".otf", ".pdf", ".png", ".pyc", ".so",
	".sqlite", ".tar", ".tgz", ".ttf", ".wav", ".webm", ".webp",
	".woff", ".woff2", ".zip",
)

var manifestNames = stringSet(
	"build.gradle", "build.gradle.kts", "cargo.toml", "composer.json",
	"deno.json", "deno.jsonc", "gemfile", "go.mod", "mix.exs", "package.json",
	"pom.xml", "project.clj", "pyproject.toml", "requirements.txt", "setup.cfg",
	"setup.py", "swift.package", "package.swift",
)

var lockNames = stringSet(
	"bun.lock", "bun.lockb", "cargo.lock", "composer.lock", "gemfile.lock",
	"go.sum", "package-lock.json", "pnpm-lock.yaml", "poetry.lock", "uv.lock",
	"yarn.lock",
)

var instructionNames = stringSet("agents.md", "claude.md", "gemini.md", "copilot-instructions.md")
var ownershipNames = stringSet("codeowners", "maintainers", "owners")
var ciNames = stringSet(".gitlab-ci.yml", "azure-pipelines.yml", "jenkinsfile")
var documentNames = stringSet("readme", "changelog", "license", "notice")
var configNames = stringSet("makefile", "dockerfile")
var entrypointNames = stringSet("main.c", "main.cpp", "main.go", "main.java", "main.py", "main.rs", "server.js", "server.ts")

// Entry is already-read Git tree metadata consumed by ClassifyEntry.
type Entry struct {
	Mode       string
	ObjectType string
	OID        string
	Path       *string
	PathSHA256 string
	Size       *int64
}

// Limits contains the validated Genesis limits used by the classifier.
type Limits struct {
	MaxEntries        int
	MaxTreeBytes      int
	MaxBlobBytes      int64
	MaxTotalBlobBytes int64
	MaxPathBytes      int
	MaxReceiptBytes   int
	GitTimeout        time.Duration
	TotalTimeout      time.Duration
}

// Record is the exact per-entry Genesis inventory record.
type Record struct {
	Boundaries     []string `json:"boundaries"`
	Classification string   `json:"classification"`
	Classes        []string `json:"classes"`
	Mode           string   `json:"mode"`
	ObjectType     string   `json:"objectType"`
	OID            string   `json:"oid"`
	Path           *string  `json:"path"`
	PathSHA256     string   `json:"pathSha256"`
	Reason         string   `json:"reason"`
	Roles          []string `json:"roles"`
	Size           *int64   `json:"size"`
}

// Classifications returns the canonical classification vocabulary in wire order.
func Classifications() []string {
	return append([]string(nil), classifications[:]...)
}

// SourceClassOrder returns the canonical source-class vocabulary in wire order.
func SourceClassOrder() []string {
	return append([]string(nil), sourceClassOrder[:]...)
}

// BoundaryOrder returns the canonical boundary vocabulary in wire order.
func BoundaryOrder() []string {
	return append([]string(nil), boundaryOrder[:]...)
}

// ClassifyEntry deterministically classifies one already-read Git tree entry.
func ClassifyEntry(
	entry Entry,
	blobs map[string][]byte,
	exhaustedOIDs map[string]struct{},
	exclusions []string,
	limits Limits,
) Record {
	classes := make([]string, 0)
	boundaries := make([]string, 0)
	roles := make([]string, 0)
	if entry.Path != nil {
		classes = sourceClasses(*entry.Path)
		boundaries = pathBoundaries(*entry.Path)
		roles = pathRoles(*entry.Path)
	}

	classification := "INCLUDED"
	reason := "tracked-text"
	switch {
	case entry.Path == nil:
		classification, reason = "UNSUPPORTED", "unsafe-or-non-utf8-path"
	case matchesExclusion(*entry.Path, exclusions):
		classification, reason = "EXCLUDED", "caller-declared-prefix"
	case entry.Mode == "160000" && entry.ObjectType == "commit":
		classification, reason = "UNSUPPORTED", "gitlink"
	case entry.Mode == "120000" && entry.ObjectType == "blob":
		classification, reason = "UNSUPPORTED", "symlink"
	case entry.Mode != "100644" && entry.Mode != "100755" || entry.ObjectType != "blob":
		classification, reason = "UNSUPPORTED", "special-tree-entry"
	case entry.Size == nil:
		classification, reason = "UNKNOWN", "blob-size-unavailable"
	case contains(binaryAssetSuffixes, pathSuffix(*entry.Path)):
		classification, reason = "UNSUPPORTED", "binary-asset"
	case *entry.Size > limits.MaxBlobBytes:
		classification, reason = "UNSUPPORTED", "blob-too-large"
	case hasOID(exhaustedOIDs, entry.OID):
		classification, reason = "UNKNOWN", "blob-budget-exhausted"
	case !hasBlob(blobs, entry.OID):
		classification, reason = "UNKNOWN", "blob-unavailable"
	case !utf8.Valid(blobs[entry.OID]):
		classification, reason = "UNSUPPORTED", "non-utf8-or-binary"
	case bytes.IndexByte(blobs[entry.OID], 0) >= 0:
		classification, reason = "UNSUPPORTED", "binary-content"
	}

	return Record{
		Boundaries:     boundaries,
		Classification: classification,
		Classes:        classes,
		Mode:           entry.Mode,
		ObjectType:     entry.ObjectType,
		OID:            entry.OID,
		Path:           clonePointer(entry.Path),
		PathSHA256:     entry.PathSHA256,
		Reason:         reason,
		Roles:          roles,
		Size:           clonePointer(entry.Size),
	}
}

func sourceClasses(path string) []string {
	parts, name, stem, suffix := pathParts(path)
	classes := make(map[string]struct{})
	isTest := containsAny(parts[:len(parts)-1], "test", "tests", "testing", "specs_test") ||
		strings.HasPrefix(name, "test_") || strings.HasSuffix(stem, "_test") ||
		strings.Contains(name, ".test.") || strings.Contains(name, ".spec.") && contains(codeSuffixes, suffix)
	isE2E := containsAny(parts, "e2e", "integration", "integration-tests", "system-tests") ||
		containsSubstring(name, "playwright", "cypress", "selenium")
	isCI := len(parts) >= 3 && parts[0] == ".github" && parts[1] == "workflows" ||
		contains(ciNames, name) || containsAny(parts, ".buildkite")
	isInstruction := contains(instructionNames, name) ||
		len(parts) >= 3 && parts[0] == ".github" && parts[1] == "instructions" && strings.HasSuffix(name, ".instructions.md")
	isSpecification := contains(documentSuffixes, suffix) &&
		(containsAny(parts[:len(parts)-1], "adr", "adrs", "decision", "decisions", "prd", "prds", "rfc", "rfcs", "spec", "specs", "tdd", "tdds") || specificationName.MatchString(stem))
	isRunbook := containsAny(parts, "runbook", "runbooks") || strings.Contains(stem, "runbook")
	isIncident := containsAny(parts, "incident", "incidents", "postmortem", "postmortems") || containsSubstring(stem, "incident", "postmortem")
	isSchema := contains(schemaSuffixes, suffix) || containsAny(parts, "migration", "migrations", "schema", "schemas") || containsSubstring(stem, "openapi", "schema")

	addIf(classes, "INSTRUCTIONS", isInstruction)
	addIf(classes, "DOCUMENTATION", contains(documentSuffixes, suffix) || contains(documentNames, name) || isRunbook || isIncident)
	addIf(classes, "SPECIFICATION", isSpecification)
	addIf(classes, "TEST", isTest || isE2E)
	addIf(classes, "E2E_TEST", isE2E)
	addIf(classes, "CI", isCI)
	addIf(classes, "MANIFEST", contains(manifestNames, name))
	addIf(classes, "LOCKFILE", contains(lockNames, name) || strings.HasSuffix(name, ".lock"))
	addIf(classes, "OWNERSHIP", contains(ownershipNames, name))
	addIf(classes, "RUNBOOK", isRunbook)
	addIf(classes, "INCIDENT", isIncident)
	addIf(classes, "SCHEMA", isSchema)
	addIf(classes, "CONFIGURATION", isCI || contains(manifestNames, name) || contains(lockNames, name) || strings.HasSuffix(name, ".lock") || contains(configSuffixes, suffix) || strings.HasPrefix(name, ".env") || contains(configNames, name))
	addIf(classes, "CODE", contains(codeSuffixes, suffix))
	addIf(classes, "ASSET", contains(binaryAssetSuffixes, suffix) || suffix == ".svg")
	if len(classes) == 0 {
		classes["OTHER_TEXT"] = struct{}{}
	}
	return ordered(classes, sourceClassOrder[:])
}

func pathBoundaries(path string) []string {
	parts, name, _, _ := pathParts(path)
	values := make(map[string]struct{})
	addIf(values, "VENDOR_CANDIDATE", containsAny(parts, "node_modules", "third_party", "vendor", "vendored"))
	generated := containsAny(parts[:len(parts)-1], ".next", "build", "coverage", "dist", "generated") ||
		strings.Contains(name, ".generated.") || containsSuffix(name, "_generated.py", "_generated.go", ".g.cs")
	addIf(values, "GENERATED_CANDIDATE", generated)
	return ordered(values, boundaryOrder[:])
}

func pathRoles(path string) []string {
	parts, name, _, _ := pathParts(path)
	roles := make(map[string]struct{})
	entrypoint := contains(entrypointNames, name) || containsAny(parts[:1], "cmd", "bin")
	addIf(roles, "ENTRYPOINT_CANDIDATE", entrypoint)
	addIf(roles, "ROUTE_CANDIDATE", containsAny(parts, "route", "routes", "router", "routers") || strings.Contains(name, "route"))
	addIf(roles, "JOB_CANDIDATE", containsAny(parts, "job", "jobs", "worker", "workers") || containsSubstring(name, "job", "worker"))
	return ordered(roles, []string{"ENTRYPOINT_CANDIDATE", "JOB_CANDIDATE", "ROUTE_CANDIDATE"})
}

func matchesExclusion(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func pathParts(path string) ([]string, string, string, string) {
	parts := strings.Split(path, "/")
	for index := range parts {
		parts[index] = pythonLower(parts[index])
	}
	name := parts[len(parts)-1]
	suffix := suffixOf(name)
	stem := strings.TrimSuffix(name, suffix)
	return parts, name, stem, suffix
}

func pathSuffix(path string) string {
	_, _, _, suffix := pathParts(path)
	return suffix
}

func suffixOf(name string) string {
	index := strings.LastIndexByte(name, '.')
	if index <= 0 || index == len(name)-1 {
		return ""
	}
	return name[index:]
}

func pythonLower(value string) string {
	if !strings.ContainsRune(value, '\u0130') {
		return strings.ToLower(value)
	}
	var lowered strings.Builder
	for _, character := range value {
		if character == '\u0130' {
			lowered.WriteString("i\u0307")
			continue
		}
		lowered.WriteRune(unicode.ToLower(character))
	}
	return lowered.String()
}

func stringSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func contains[T comparable](values map[T]struct{}, value T) bool {
	_, ok := values[value]
	return ok
}

func containsAny(values []string, candidates ...string) bool {
	for _, value := range values {
		for _, candidate := range candidates {
			if value == candidate {
				return true
			}
		}
	}
	return false
}

func containsSubstring(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func containsSuffix(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.HasSuffix(value, candidate) {
			return true
		}
	}
	return false
}

func addIf(values map[string]struct{}, value string, condition bool) {
	if condition {
		values[value] = struct{}{}
	}
}

func ordered(values map[string]struct{}, order []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range order {
		if contains(values, value) {
			result = append(result, value)
		}
	}
	return result
}

func hasOID(values map[string]struct{}, oid string) bool {
	_, ok := values[oid]
	return ok
}

func hasBlob(values map[string][]byte, oid string) bool {
	_, ok := values[oid]
	return ok
}

func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
