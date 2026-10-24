package gateselect

import (
	"fmt"
	"regexp"
	"strings"
)

// The go.work directive arguments the selector's base reader checks
// (AFP-V0-403), mirroring parsedManifest.add, manifestReplacement, pathMajor
// and majorMatches in internal/liveverify/affected/golang/nested.go.

// Workspace refusal reasons. Each names one way a base go.work fails the
// planner's strict grammar, so the selector's base is unknown.
const (
	WorkspaceUnreadable       = "GO_WORK_UNREADABLE"
	WorkspaceLexical          = "GO_WORK_LEXICAL"
	WorkspaceUnknownDirective = "GO_WORK_UNKNOWN_DIRECTIVE"
	WorkspaceBlock            = "GO_WORK_BLOCK"
	WorkspaceUnterminated     = "GO_WORK_UNTERMINATED_BLOCK"
	WorkspaceMalformedGo      = "GO_WORK_MALFORMED_GO"
	WorkspaceMalformedTool    = "GO_WORK_MALFORMED_TOOLCHAIN"
	WorkspaceMalformedGodebug = "GO_WORK_MALFORMED_GODEBUG"
	WorkspaceMalformedUse     = "GO_WORK_MALFORMED_USE"
	WorkspaceMalformedReplace = "GO_WORK_MALFORMED_REPLACE"
)

// WorkspaceRefusal is why baseWorkspace refuses a base go.work: the 1-based
// line (0 for the whole file) and one of the Workspace* reasons.
type WorkspaceRefusal struct {
	Line   int
	Reason string
}

func (refusal *WorkspaceRefusal) Error() string {
	return fmt.Sprintf("gateselect: go.work line %d: %s", refusal.Line, refusal.Reason)
}

var (
	// workspaceCanonicalVersion, workspaceGoVersion and
	// workspaceToolchainVersion are the planner's canonicalVersion, goVersion
	// and toolchainVersion.
	workspaceCanonicalVersion = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)` +
		`(-(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*)?(\+incompatible)?$`)
	workspaceGoVersion        = regexp.MustCompile(`^([1-9][0-9]*)\.(0|[1-9][0-9]*)(\.(0|[1-9][0-9]*))?([a-z]+[0-9]+)?$`)
	workspaceToolchainVersion = regexp.MustCompile(`^default$|^go1($|\.)`)
)

// workspaceDirectiveReason checks one directive's arguments as the planner
// does and returns the refusal reason, or "" for a valid directive. seen
// records the go and toolchain directives, which may each appear once.
func workspaceDirectiveReason(verb string, args []string, quoted []bool, seen map[string]bool) string {
	reasons := map[string]string{
		"go": WorkspaceMalformedGo, "toolchain": WorkspaceMalformedTool, "godebug": WorkspaceMalformedGodebug,
		"use": WorkspaceMalformedUse, "replace": WorkspaceMalformedReplace,
	}
	malformed := reasons[verb]
	punctuation := func(index int) bool {
		return !quoted[index] && len(args[index]) == 1 && strings.Contains(workspacePunctuation, args[index])
	}
	for index, arg := range args {
		// The go tool reserves quotes inside an unquoted argument.
		if punctuation(index) || (!quoted[index] && strings.ContainsAny(arg, "\"'`")) {
			return malformed
		}
	}
	switch verb {
	case "go", "toolchain":
		if seen[verb] || len(args) != 1 || args[0] == "" || quoted[0] {
			return malformed
		}
		seen[verb] = true
		if (verb == "go" && !workspaceGoVersion.MatchString(args[0])) || (verb == "toolchain" && !workspaceToolchainVersion.MatchString(args[0])) {
			return malformed
		}
	case "godebug":
		if len(args) != 1 || quoted[0] {
			return malformed
		}
		key, _, found := strings.Cut(args[0], "=")
		if !found || key == "" || strings.ContainsAny(args[0], "\"`',") {
			return malformed
		}
	case "use":
		if len(args) != 1 || args[0] == "" {
			return malformed
		}
	case "replace":
		if !workspaceReplacement(args, quoted) {
			return malformed
		}
	}
	return ""
}

// workspaceReplacement is the planner's manifestReplacement check of
// `old [version] => new [version]`.
func workspaceReplacement(args []string, quoted []bool) bool {
	arrow := -1
	for index, arg := range args {
		if arg == "=>" && !quoted[index] {
			arrow = index
			break
		}
	}
	right := len(args) - arrow - 1
	version := func(index int) bool { return workspaceCanonicalVersion.MatchString(args[index]) }
	if arrow < 1 || arrow > 2 || right < 1 || right > 2 || args[0] == "" || args[arrow+1] == "" ||
		(arrow == 2 && !version(1)) || (right == 2 && !version(arrow+2)) {
		return false
	}
	if _, ok := workspacePathMajor(args[0]); !ok || (arrow == 2 && !workspaceMajorMatches(args[0], args[1])) {
		return false
	}
	return workspaceDirectoryPath(args[arrow+1]) == (right == 1)
}

// workspacePathMajor is the planner's pathMajor.
func workspacePathMajor(modulePath string) (string, bool) {
	if strings.HasPrefix(modulePath, "gopkg.in/") {
		end := strings.TrimSuffix(modulePath, "-unstable")
		prefix := strings.TrimRight(end, "0123456789")
		if !strings.HasSuffix(prefix, ".v") || len(prefix) < 3 || len(prefix) == len(end) {
			return "", false
		}
		major := modulePath[len(prefix)-2:]
		if major[2] == '0' && major != ".v0" {
			return "", false
		}
		return major, true
	}
	start := len(modulePath)
	dot := false
	for start > 0 && (modulePath[start-1] >= '0' && modulePath[start-1] <= '9' || modulePath[start-1] == '.') {
		dot = dot || modulePath[start-1] == '.'
		start--
	}
	if start <= 1 || start == len(modulePath) || modulePath[start-1] != 'v' || modulePath[start-2] != '/' {
		return "", true
	}
	major := modulePath[start-2:]
	if dot || len(major) <= 2 || major[2] == '0' || major == "/v1" {
		return "", false
	}
	return major, true
}

// workspaceMajorMatches is the planner's majorMatches.
func workspaceMajorMatches(modulePath, canonical string) bool {
	major, ok := workspacePathMajor(modulePath)
	if !ok {
		return false
	}
	versionMajor, _, _ := strings.Cut(canonical, ".")
	major = strings.TrimSuffix(major, "-unstable")
	switch {
	case major == "":
		return versionMajor == "v0" || versionMajor == "v1" || strings.HasSuffix(canonical, "+incompatible")
	case major == ".v1" && strings.HasPrefix(canonical, "v0.0.0-"):
		return true
	default:
		return versionMajor == major[1:]
	}
}

// workspaceDirectoryPath is the planner's directoryPath.
func workspaceDirectoryPath(value string) bool {
	return value == "." || value == ".." || strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") ||
		strings.HasPrefix(value, "/") || strings.HasPrefix(value, `.\`) || strings.HasPrefix(value, `..\`) ||
		(len(value) > 1 && value[1] == ':')
}
