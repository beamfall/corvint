package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// AHI-016 (docs/specs/agent-harness-integration-v0.md): a user prompt past the
// query bound is served by a derived query made only of the prompt's explicit
// anchors, copied verbatim, and the injected context discloses that derivation.
// Nothing is stored: a stored prompt would be a read-path mutation outside the
// self-observation ledger (AGENTS.md invariant 4) and would persist prompt text
// (AHI-005). A prompt whose complete anchor set is empty or does not fit keeps
// the prompt-over-query-bound refusal; no anchor subset is ever chosen.
var (
	promptBacktickSpan  = regexp.MustCompile("`[ -_a-~]{1,128}`")
	promptToken         = regexp.MustCompile(`[A-Za-z0-9_][A-Za-z0-9_./-]*[A-Za-z0-9_]`)
	promptPathAnchor    = regexp.MustCompile(`^(?:[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.-]+$|^[A-Za-z0-9_-]{2,}\.[A-Za-z][A-Za-z0-9]{0,7}$`)
	promptIdentifier    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	promptIdentifierHow = regexp.MustCompile(`[A-Za-z0-9]_[A-Za-z0-9]|[a-z][A-Z]`)
	promptRequirementID = regexp.MustCompile(`^[A-Z][A-Z0-9]*(?:-[A-Z0-9]+)*-[0-9]+$`)
)

type promptAnchor struct {
	offset int
	text   string
}

func promptOverQueryBound(prompt string) bool {
	return utf8.RuneCountInString(prompt) > gokernel.MaxQueryCharacters || len(prompt) > gokernel.MaxTaskBytes
}

// deriveOverBoundQuery returns the derived query and its disclosure, or two
// empty strings when the prompt must keep the refusal.
func deriveOverBoundQuery(prompt string) (string, string) {
	spans := promptBacktickSpan.FindAllStringIndex(prompt, -1)
	anchors := make([]promptAnchor, 0, len(spans))
	for _, span := range spans {
		anchors = append(anchors, promptAnchor{span[0], prompt[span[0]+1 : span[1]-1]})
	}
	masked := []byte(prompt)
	for _, span := range spans {
		copy(masked[span[0]:span[1]], strings.Repeat(" ", span[1]-span[0]))
	}
	for _, token := range promptToken.FindAllIndex(masked, -1) {
		anchors = appendLexicalAnchor(anchors, token[0], string(masked[token[0]:token[1]]))
	}
	sort.SliceStable(anchors, func(left, right int) bool { return anchors[left].offset < anchors[right].offset })
	texts := distinctAnchorTexts(anchors)
	query := strings.Join(texts, " ")
	if query == "" || promptOverQueryBound(query) {
		return "", ""
	}
	return query, fmt.Sprintf("Corvint prompt bound (trusted adapter disclosure): the prompt has %d characters and %d bytes, over the %d-character/%d-byte query bound. Retrieval used only a derived query of %d distinct explicit anchors (backtick spans, paths, identifiers, requirement IDs) copied verbatim from the prompt, %d characters long. The rest of the prompt was not queried; this context makes no claim about it and may omit evidence the full prompt would select.\n",
		utf8.RuneCountInString(prompt), len(prompt), gokernel.MaxQueryCharacters, gokernel.MaxTaskBytes, len(texts), len(query))
}

func appendLexicalAnchor(anchors []promptAnchor, offset int, token string) []promptAnchor {
	identifier := promptIdentifier.MatchString(token) && promptIdentifierHow.MatchString(token)
	if !identifier && !promptPathAnchor.MatchString(token) && !promptRequirementID.MatchString(token) {
		return anchors
	}
	return append(anchors, promptAnchor{offset, token})
}

func distinctAnchorTexts(anchors []promptAnchor) []string {
	seen := map[string]bool{}
	texts := make([]string, 0, len(anchors))
	for _, anchor := range anchors {
		if seen[anchor.text] {
			continue
		}
		seen[anchor.text] = true
		texts = append(texts, anchor.text)
	}
	return texts
}

// promptBoundDisclosure is the disclosure for a user-prompt payload that
// normalizeAdapterInput served through a derived query, and "" otherwise.
func promptBoundDisclosure(event string, payload map[string]any) string {
	prompt, _ := payload["prompt"].(string)
	prompt = strings.TrimSpace(prompt)
	if event != "user-prompt" {
		return ""
	}
	if !promptOverQueryBound(prompt) {
		return ""
	}
	_, disclosure := deriveOverBoundQuery(prompt)
	return disclosure
}

// promptBoundReserve is the disclosure's encoded size inside the host frame.
func promptBoundReserve(disclosure string) int {
	encoded, _ := json.Marshal(disclosure)
	return len(encoded) - 2
}

// withPromptBoundDisclosure places the disclosure as trusted adapter text ahead
// of the rendered additionalContext, outside the untrusted-data envelope.
func withPromptBoundDisclosure(output map[string]any, disclosure string) map[string]any {
	hook, _ := output["hookSpecificOutput"].(map[string]any)
	context, ok := hook["additionalContext"].(string)
	if !ok {
		return output
	}
	hook["additionalContext"] = disclosure + context
	return output
}
