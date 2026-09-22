package doccompiler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

var revisionPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func plan(environment Environment, options PlanOptions) (PatchPlan, error) {
	if environment.ProjectRoot == "" || environment.Config.Path == "" {
		return PatchPlan{}, failure("invalid-environment", "discovery environment is incomplete")
	}
	if _, err := verifyPinnedSource(environment.ProjectRoot, SourcePin{Path: environment.Config.Path, SHA256: environment.Config.SHA256, Size: environment.Config.Size}, defaultConfigBytes); err != nil {
		return PatchPlan{}, err
	}
	docsRoot, err := resolveConfigRelative(environment.Config.Path, environment.Observations.DocsDir)
	if err != nil {
		return PatchPlan{}, failure("invalid-docs-dir", "docs_dir is not contained")
	}
	documents := make([]DocumentPatch, 0, len(options.Documents))
	evidence := make([]Evidence, 0)
	sources := map[string]SourcePin{
		environment.Config.Path: {Path: environment.Config.Path, SHA256: environment.Config.SHA256, Size: environment.Config.Size},
	}
	seenDocuments := map[string]struct{}{}
	seenClaims := map[string]struct{}{}
	for _, proposal := range options.Documents {
		document, documentEvidence, documentSources, err := planDocument(environment.ProjectRoot, docsRoot, proposal, seenClaims)
		if err != nil {
			return PatchPlan{}, err
		}
		if _, exists := seenDocuments[document.Path]; exists {
			return PatchPlan{}, failure("duplicate-document", "document proposal is duplicated: %s", document.Path)
		}
		seenDocuments[document.Path] = struct{}{}
		documents = append(documents, document)
		evidence = append(evidence, documentEvidence...)
		for _, source := range documentSources {
			sources[source.Path] = source
		}
	}
	nav, navSources, err := planNav(environment, docsRoot, options.Nav, seenDocuments)
	if err != nil {
		return PatchPlan{}, err
	}
	for _, source := range navSources {
		sources[source.Path] = source
	}
	sort.Slice(documents, func(left, right int) bool { return documents[left].Path < documents[right].Path })
	sort.Slice(evidence, func(left, right int) bool { return evidenceKey(evidence[left]) < evidenceKey(evidence[right]) })
	sourceList := make([]SourcePin, 0, len(sources))
	for _, source := range sources {
		sourceList = append(sourceList, source)
	}
	sort.Slice(sourceList, func(left, right int) bool { return sourceList[left].Path < sourceList[right].Path })
	result := PatchPlan{
		Profile:      ExperimentalPatchPlanProfile,
		ConfigSHA256: environment.Config.SHA256,
		Documents:    documents,
		Nav:          nav,
		Evidence:     evidence,
		Sources:      sourceList,
	}
	digest, err := planDigest(result)
	if err != nil {
		return PatchPlan{}, err
	}
	result.PlanSHA256 = digest
	return result, nil
}

func planDocument(root, docsRoot string, proposal DocumentProposal, seenClaims map[string]struct{}) (DocumentPatch, []Evidence, []SourcePin, error) {
	if proposal.Markdown != "" {
		return DocumentPatch{}, nil, nil, failure("unadmitted-markdown", "experimental plans accept only structured claims; Markdown must be empty")
	}
	path, err := cleanRelative(proposal.Path)
	if err != nil {
		return DocumentPatch{}, nil, nil, err
	}
	if strings.ToLower(filepath.Ext(path)) != ".md" {
		return DocumentPatch{}, nil, nil, failure("invalid-document-path", "document proposal must be Markdown: %s", path)
	}
	projectPath, err := cleanRelative(filepath.ToSlash(filepath.Join(filepath.FromSlash(docsRoot), filepath.FromSlash(path))))
	if err != nil {
		return DocumentPatch{}, nil, nil, err
	}
	if err := rejectExistingSymlinkComponents(root, projectPath); err != nil {
		return DocumentPatch{}, nil, nil, err
	}
	claims := append([]Claim(nil), proposal.Claims...)
	sort.Slice(claims, func(left, right int) bool { return claims[left].ID < claims[right].ID })
	evidence := make([]Evidence, 0)
	sources := make([]SourcePin, 0)
	for _, claim := range claims {
		if err := validateClaim(claim, seenClaims); err != nil {
			return DocumentPatch{}, nil, nil, err
		}
	}
	rendered := renderDocument(claims)
	proposedDigest := sha256.Sum256([]byte(rendered))
	emptyDigest := sha256.Sum256(nil)
	document := DocumentPatch{
		Path:           projectPath,
		Operation:      "create_file",
		ReviewRequired: true,
		ProposedSHA256: hex.EncodeToString(proposedDigest[:]),
		Patch: BytePatch{
			StartByte: 0, EndByte: 0,
			OriginalSHA256:    hex.EncodeToString(emptyDigest[:]),
			ReplacementSHA256: hex.EncodeToString(proposedDigest[:]),
			Replacement:       rendered,
		},
		RenderedMarkdown: rendered,
		Claims:           claims,
	}
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(projectPath))); err == nil {
		currentRaw, current, readErr := readPinned(root, projectPath, defaultSourceBytes)
		if readErr != nil {
			return DocumentPatch{}, nil, nil, readErr
		}
		insertion := rendered
		if len(currentRaw) > 0 && currentRaw[len(currentRaw)-1] != '\n' {
			insertion = "\n" + insertion
		}
		candidate := append(append([]byte(nil), currentRaw...), []byte(insertion)...)
		candidateDigest := sha256.Sum256(candidate)
		insertionDigest := sha256.Sum256([]byte(insertion))
		document.Operation = "insert_after"
		document.CurrentSHA256 = current.SHA256
		document.ProposedSHA256 = hex.EncodeToString(candidateDigest[:])
		document.RenderedMarkdown = string(candidate)
		document.Patch.StartByte = len(currentRaw)
		document.Patch.EndByte = len(currentRaw)
		document.Patch.OriginalSHA256 = hex.EncodeToString(emptyDigest[:])
		document.Patch.ReplacementSHA256 = hex.EncodeToString(insertionDigest[:])
		document.Patch.Replacement = insertion
		sources = append(sources, current)
	} else if !os.IsNotExist(err) {
		return DocumentPatch{}, nil, nil, failure("file-unavailable", "cannot inspect document target %s", projectPath)
	}
	return document, evidence, sources, nil
}

func validateClaim(claim Claim, seen map[string]struct{}) error {
	if strings.TrimSpace(claim.ID) == "" || strings.TrimSpace(claim.Text) == "" {
		return failure("invalid-claim", "claim ID and text are required")
	}
	if _, exists := seen[claim.ID]; exists {
		return failure("duplicate-claim", "claim ID is duplicated: %s", claim.ID)
	}
	seen[claim.ID] = struct{}{}
	if claim.Status != "SUPPORTED" && claim.Status != "CONFLICTED" && claim.Status != "UNKNOWN" {
		return failure("invalid-claim-status", "claim %s has an invalid status", claim.ID)
	}
	if claim.Status == "SUPPORTED" && len(claim.Evidence) == 0 {
		return failure("unsupported-supported-claim", "SUPPORTED claim %s has no evidence", claim.ID)
	}
	if claim.Status != "UNKNOWN" {
		return failure("immutable-evidence-unavailable", "experimental planning cannot qualify %s claim %s without immutable evidence and authority verification", claim.Status, claim.ID)
	}
	if len(claim.Evidence) != 0 {
		return failure("immutable-evidence-unavailable", "experimental planning cannot authenticate evidence revisions or authority for claim %s", claim.ID)
	}
	if strings.TrimSpace(claim.Uncertainty) == "" {
		return failure("missing-uncertainty", "claim %s must explain its uncertainty", claim.ID)
	}
	return nil
}

func planNav(environment Environment, docsRoot string, entries []NavEntry, proposed map[string]struct{}) (NavPatch, []SourcePin, error) {
	entries = append([]NavEntry(nil), entries...)
	sources := make([]SourcePin, 0)
	seen := map[string]struct{}{}
	for index := range entries {
		entries[index].Title = strings.TrimSpace(entries[index].Title)
		path, err := cleanRelative(entries[index].Path)
		if err != nil || entries[index].Title == "" || strings.ToLower(filepath.Ext(path)) != ".md" {
			return NavPatch{}, nil, failure("invalid-nav-entry", "nav entries require a title and contained Markdown path")
		}
		entries[index].Path = path
		projectPath, err := cleanRelative(filepath.ToSlash(filepath.Join(filepath.FromSlash(docsRoot), filepath.FromSlash(path))))
		if err != nil {
			return NavPatch{}, nil, err
		}
		if _, exists := seen[projectPath]; exists {
			return NavPatch{}, nil, failure("duplicate-nav-entry", "nav path is duplicated: %s", path)
		}
		seen[projectPath] = struct{}{}
		if _, exists := proposed[projectPath]; exists {
			continue
		}
		_, source, err := readPinned(environment.ProjectRoot, projectPath, defaultSourceBytes)
		if err != nil {
			return NavPatch{}, nil, err
		}
		sources = append(sources, source)
	}
	sort.Slice(entries, func(left, right int) bool {
		if entries[left].Path == entries[right].Path {
			return entries[left].Title < entries[right].Title
		}
		return entries[left].Path < entries[right].Path
	})
	nav := NavPatch{
		ConfigPath: environment.Config.Path, ConfigSHA256: environment.Config.SHA256,
		Operation: "edit_nav", ReviewRequired: len(entries) > 0, CurrentNavSHA256: environment.Observations.NavSHA256,
		Entries: entries,
	}
	if len(entries) == 0 {
		return nav, sources, nil
	}
	configRaw, _, err := readPinned(environment.ProjectRoot, environment.Config.Path, defaultConfigBytes)
	if err != nil {
		return NavPatch{}, nil, err
	}
	start, end, found, err := topLevelSectionSpan(configRaw, "nav")
	if err != nil {
		return NavPatch{}, nil, err
	}
	replacement := renderNav(entries)
	if !found && len(configRaw) > 0 && configRaw[len(configRaw)-1] != '\n' {
		replacement = "\n" + replacement
	}
	originalDigest := sha256.Sum256(configRaw[start:end])
	replacementDigest := sha256.Sum256([]byte(replacement))
	nav.Patch = BytePatch{
		StartByte: start, EndByte: end,
		OriginalSHA256:    hex.EncodeToString(originalDigest[:]),
		ReplacementSHA256: hex.EncodeToString(replacementDigest[:]),
		Replacement:       replacement,
	}
	nav.ProposedNavSHA256 = proposedNavDigest(entries)
	return nav, sources, nil
}

func topLevelSectionSpan(raw []byte, target string) (int, int, bool, error) {
	start := -1
	offset := 0
	for offset < len(raw) {
		next := strings.IndexByte(string(raw[offset:]), '\n')
		lineEnd := len(raw)
		if next >= 0 {
			lineEnd = offset + next + 1
		}
		line := strings.TrimSuffix(string(raw[offset:lineEnd]), "\n")
		observed := stripComment(strings.TrimSuffix(line, "\r"))
		trimmed := strings.TrimSpace(observed)
		indent := len(observed) - len(strings.TrimLeft(observed, " "))
		key, _, hasKey := splitKeyValue(trimmed)
		if indent == 0 && hasKey {
			if start >= 0 && key != target {
				return start, offset, true, nil
			}
			if key == target {
				if start >= 0 {
					return 0, 0, false, failure("ambiguous-nav-owner", "config contains duplicate top-level nav keys")
				}
				start = offset
			}
		}
		offset = lineEnd
	}
	if start >= 0 {
		return start, len(raw), true, nil
	}
	return len(raw), len(raw), false, nil
}

func renderNav(entries []NavEntry) string {
	var output strings.Builder
	output.WriteString("nav:\n")
	for _, entry := range entries {
		output.WriteString(navEntryYAML(yamlDoubleQuoted(entry.Title), yamlDoubleQuoted(entry.Path)))
	}
	return output.String()
}

// yamlImplicitKeyLimit is the longest implicit mapping key, in stream characters, that PyYAML
// (and libyaml) accept; a longer quoted title is written as an explicit "? " key instead.
const yamlImplicitKeyLimit = 1024

func navEntryYAML(title, path string) string {
	if utf8.RuneCountInString(title) > yamlImplicitKeyLimit {
		return fmt.Sprintf("  - ? %s\n    : %s\n", title, path)
	}
	return fmt.Sprintf("  - %s: %s\n", title, path)
}

// yamlDoubleQuoted is a JSON string, which is a YAML double-quoted scalar, with the code points
// json.Marshal leaves literal but YAML cannot carry (U+007F..U+009F, NEL folds as a line break,
// U+FFFE, U+FFFF) written as \uXXXX so the loaded value is the exact input string.
func yamlDoubleQuoted(value string) string {
	encoded, _ := json.Marshal(value)
	var output strings.Builder
	for _, character := range string(encoded) {
		output.WriteString(yamlQuotedRune(character))
	}
	return output.String()
}

func yamlQuotedRune(character rune) string {
	if !yamlCannotCarry(character) {
		return string(character)
	}
	return fmt.Sprintf(`\u%04x`, character)
}

func yamlCannotCarry(character rune) bool {
	return (character >= 0x7F && character <= 0x9F) || character == 0xFFFE || character == 0xFFFF
}

// proposedNavDigest reproduces the authority probe's nav_sha256 input, Python
// json.dumps(sort_keys=True, separators=(",",":")) with default ensure_ascii, over the
// one-member mappings renderNav writes.
func proposedNavDigest(entries []NavEntry) string {
	var encoded strings.Builder
	encoded.WriteByte('[')
	for index, entry := range entries {
		if index > 0 {
			encoded.WriteByte(',')
		}
		encoded.WriteByte('{')
		writePythonASCIIString(&encoded, entry.Title)
		encoded.WriteByte(':')
		writePythonASCIIString(&encoded, entry.Path)
		encoded.WriteByte('}')
	}
	encoded.WriteByte(']')
	digest := sha256.Sum256([]byte(encoded.String()))
	return hex.EncodeToString(digest[:])
}

var pythonShortEscapes = map[rune]string{'"': `\"`, '\\': `\\`, '\b': `\b`, '\f': `\f`, '\n': `\n`, '\r': `\r`, '\t': `\t`}

// writePythonASCIIString writes Python's ensure_ascii string spelling: printable ASCII
// literal, short escapes, and every other code point as lower-case \uXXXX, using a
// UTF-16 surrogate pair above U+FFFF. Invalid UTF-8 is U+FFFD, as renderNav's
// json.Marshal writes it into the candidate config.
func writePythonASCIIString(output *strings.Builder, value string) {
	output.WriteByte('"')
	for _, character := range value {
		output.WriteString(pythonASCIIRune(character))
	}
	output.WriteByte('"')
}

func pythonASCIIRune(character rune) string {
	if escape, short := pythonShortEscapes[character]; short {
		return escape
	}
	if character >= ' ' && character <= '~' {
		return string(character)
	}
	if character > 0xFFFF {
		high, low := utf16.EncodeRune(character)
		return fmt.Sprintf(`\u%04x\u%04x`, high, low)
	}
	return fmt.Sprintf(`\u%04x`, character)
}

func renderDocument(claims []Claim) string {
	var output strings.Builder
	output.WriteString("\n\n---\n\n## Evidence and uncertainty\n\nExperimental candidate claims; immutable evidence and authority remain unverified.\n")
	for _, claim := range claims {
		fmt.Fprintf(&output, "\n- %s **UNKNOWN** — %s\n", claimLiteral(claim.ID), claimLiteral(claim.Text))
		fmt.Fprintf(&output, "  - Uncertainty: %s\n", claimLiteral(claim.Uncertainty))
	}
	return output.String()
}

// Quote controls and use a delimiter longer than any caller backtick run so every
// field remains one literal inline value, including Markdown and HTML payloads.
func claimLiteral(value string) string {
	quoted := strconv.Quote(value)
	longest, run := 0, 0
	for _, character := range quoted {
		if character == '`' {
			run++
			longest = max(longest, run)
			continue
		}
		run = 0
	}
	delimiter := strings.Repeat("`", longest+1)
	return delimiter + quoted + delimiter
}

func verifyPinnedSource(root string, expected SourcePin, limit int64) (SourcePin, error) {
	_, actual, err := readPinned(root, expected.Path, limit)
	if err != nil {
		return SourcePin{}, err
	}
	if actual != expected {
		return SourcePin{}, failure("stale-source", "%s no longer matches its plan pin", expected.Path)
	}
	return actual, nil
}

func planDigest(value PatchPlan) (string, error) {
	value.PlanSHA256 = ""
	encoded, err := CanonicalJSON(value)
	if err != nil {
		return "", failure("plan-encoding-failed", "cannot encode patch plan")
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func evidenceKey(value Evidence) string {
	return strings.Join([]string{value.Path, value.Revision, value.SHA256, fmt.Sprint(value.StartLine), fmt.Sprint(value.EndLine)}, "\x00")
}
