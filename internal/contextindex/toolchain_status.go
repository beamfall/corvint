package contextindex

import (
	"regexp"
	"strings"
)

// toolchainRecord applies the STA-V0-007 path classification to a document
// record: a toolchain path replaces the content-derived status and carries the
// upstream lifecycle as native_state.
func toolchainRecord(record Record, lines []string, sources map[string]Source) Record {
	if classified, native, ok := toolchainDocumentStatus(record.Path, lines, sources); ok {
		record.Fields["status"] = classified
		if native != "" {
			record.Fields["native_state"] = native
		}
	}
	return record
}

// specKitConstitution is Spec Kit's project constitution, which the index
// reads as project instructions (STA-V0-007).
const specKitConstitution = ".specify/memory/constitution.md"

var (
	openSpecArchiveDate = regexp.MustCompile(`^([0-9]{4}-[0-9]{2}-[0-9]{2})-.`)
	specKitStatusHeader = regexp.MustCompile(`^\*\*Status\*\*:[\t ]*(.*?)[\t ]*$`)
)

// toolchainDocumentStatus classifies OpenSpec and Spec Kit documents by their
// pinned path (STA-V0-007). It reports the index status, the upstream native
// state when the layout carries one, and whether the path is a toolchain path
// at all. The path decides the label: an archived or proposed OpenSpec
// document never takes a binding status from its own text. The only content
// read is a Spec Kit feature spec's `**Status**:` header above its first
// level-2 heading, matching the OCM reader.
func toolchainDocumentStatus(value string, lines []string, sources map[string]Source) (status, native string, ok bool) {
	parts := strings.Split(value, "/")
	switch {
	case len(parts) >= 3 && parts[0] == "openspec" && parts[1] == "specs" && parts[len(parts)-1] == "spec.md":
		return "current", "", true
	case len(parts) >= 4 && parts[0] == "openspec" && parts[1] == "changes" && parts[2] == "archive":
		// Anything under archive/ stays archived even when its directory does
		// not carry the YYYY-MM-DD- prefix; only the date is then unreported.
		if match := openSpecArchiveDate.FindStringSubmatch(parts[3]); match != nil && len(parts) >= 5 {
			native = match[1]
		}
		return "archived", native, true
	case len(parts) >= 4 && parts[0] == "openspec" && parts[1] == "changes":
		return "proposed", "", true
	case len(parts) == 3 && parts[0] == "specs" && parts[2] == "spec.md" && specKitPresent(sources):
		fence := ""
		for _, raw := range lines {
			open := fence
			fence = nextFence(fence, strings.TrimSpace(raw))
			if open != "" || fence != "" {
				continue
			}
			if strings.HasPrefix(raw, "## ") || strings.TrimRight(raw, "\r") == "##" {
				break
			}
			if match := specKitStatusHeader.FindStringSubmatch(strings.TrimRight(raw, "\r")); match != nil && match[1] != "" {
				return strings.ToLower(match[1]), match[1], true
			}
		}
		return "draft", "", true
	}
	return "", "", false
}

// openSpecChangePath reports whether a path lies under openspec/changes/, where
// STA-V0-007 forbids a binding label whatever the document's kind.
func openSpecChangePath(value string) bool {
	return strings.HasPrefix(value, "openspec/changes/")
}

// specKitPresent reports whether a .specify/ directory exists at the
// repository root, which in a pinned tree means it holds an indexed source.
func specKitPresent(sources map[string]Source) bool {
	for candidate := range sources {
		if strings.HasPrefix(candidate, ".specify/") {
			return true
		}
	}
	return false
}
