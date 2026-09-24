// Package observations keeps bounded, local-only self-observation proposals.
package observations

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

const (
	maxFileBytes = 128 * 1024
	maxRowBytes  = 2048
)

var appendProcessLock sync.Mutex

var errLedgerNotRegular = errors.New("self-observation ledger or directory is not regular")

// CodeProhibitedContent identifies a SOL-V0-002 writer-contract refusal.
const CodeProhibitedContent = "observation-prohibited-content"

var admittedDegradations = map[string]bool{
	"compaction-critical-evidence-overflow":       true,
	"compaction-dirty-set-over-budget":            true,
	"compaction-untracked-paths-not-rehydratable": true,
	"frontier-authority-unavailable":              true,
	"host-version-unknown":                        true,
	"outcome-persistence-unavailable":             true,
}

var admittedUnsupportedCodes = map[string]bool{
	"unsupported-documentation-source":           true,
	"unsupported-feature-platform":               true,
	"unsupported-feature-repository":             true,
	"unsupported-frontier-context":               true,
	"unsupported-git-object-format":              true,
	"unsupported-harness-event":                  true,
	"unsupported-harness-host":                   true,
	"unsupported-hunk-selector":                  true,
	"unsupported-impact-option":                  true,
	"unsupported-impact-path":                    true,
	"unsupported-impact-path-suffix":             true,
	"unsupported-impact-platform":                true,
	"unsupported-impact-range":                   true,
	"unsupported-impact-repository":              true,
	"unsupported-impact-worktree":                true,
	"unsupported-lrf-context":                    true,
	"unsupported-object-alternates":              true,
	"unsupported-ocm-closure-profile":            true,
	"unsupported-ocm-json-profile":               true,
	"unsupported-ocm-output-path":                true,
	"unsupported-ocm-python-claims":              true,
	"unsupported-query-authority":                true,
	"unsupported-query-drift":                    true,
	"unsupported-query-history":                  true,
	"unsupported-query-intent":                   true,
	"unsupported-query-platform":                 true,
	"unsupported-query-repository":               true,
	"unsupported-query-task":                     true,
	"unsupported-query-trace-state":              true,
	"unsupported-repository-attributes":          true,
	"unsupported-spec":                           true,
	"unsupported-tree-mode":                      true,
	"unsupported-verify-syntax":                  true,
	"unsupported-working-tree-impact-path":       true,
	"unsupported-working-tree-impact-repository": true,
}

// unsupportedByDesign is SOL-V0-007's closed by-design registry: an admitted
// unsupported code mapped to the repository-relative decision record that
// rules the refusal intended. Triage reports such a code separately and never
// drafts it as a capability gap. An entry needs an accepted decision record.
var unsupportedByDesign = map[string]string{}

// KindAdapterDegradation is SOL-V0-010's row kind: one host adapter degradation per
// host, event, code set and hour window.
const KindAdapterDegradation = "adapter-degradation"

// adapterWindowLayout is the coarse UTC hour an adapter-degradation row carries; it is
// also the row's deduplication window.
const adapterWindowLayout = "2006-01-02T15Z"

const maxAdapterCodes = 8

var admittedAdapterHosts = codeSet("claude-code", "codex")

var admittedAdapterEvents = codeSet("file-change", "post-tool", "session-end", "session-start", "stop", "user-prompt")

// admittedAdapterCodes is the closed set of degradation reasons the codex and
// claude-code adapters return after resolving the project root.
var admittedAdapterCodes = codeSet(
	"adapter-host-kill-deadline", "corvint-envelope-terminator-collision", "corvint-event-rejected",
	"file-change-path-not-project-relative", "invalid-input", "invalid-session-identity",
	"invalid-start-source", "invalid-stop-hook-active", "malformed-corvint-output", "missing-prompt",
	"missing-session-identity", "prompt-over-query-bound",
)

// admittedAdapterRejections are the `dogfood event` codes a `corvint-event-rejected:<code>`
// reason may carry besides the dogfood and unsupported registries.
var admittedAdapterRejections = codeSet(
	"dogfood-event-context-drift", "dogfood-event-deadline", "dogfood-event-input-unavailable",
	"dogfood-event-native-budget", "dogfood-event-output-too-large", "dogfood-event-output-unavailable",
	"dogfood-event-policy-drift", "dogfood-event-repository-drift", "dogfood-event-unavailable",
	"invalid-dogfood-event-arguments", "invalid-dogfood-event-budget", "invalid-dogfood-event-input",
	"qualified-lifecycle-target-drift", "unsupported-dogfood-event", "unsupported-dogfood-event-host",
)

var admittedFalsifiers = map[string]bool{
	"history-consistent": true,
	"none":               true,
	"reference-resolves": true,
	"test-kills-mutant":  true,
	"verifier-accepts":   true,
}

var admittedProofVerdicts = map[string]bool{"FAIL": true, "NOT_RUN": true, "PASS": true}

var admittedDogfoodErrorCodes = codeSet(
	"ambiguous-claim-selector", "base-mismatch", "base-revision-mismatch", "binary-patch",
	"bootstrap-intent-not-unknown", "bundle-map-uncommitted", "bundle-output-refused",
	"cem-map-digest-mismatch", "cem-patch-digest-mismatch",
	"changed-path-acquisition-failed", "cite-span-not-stable", "claim-conflict", "claim-not-reextractable",
	"claim-obligation-mismatch", "claim-selector-out-of-range", "diff-metadata-mismatch",
	"duplicate-claim", "duplicate-claim-reference", "duplicate-hunk-reference", "duplicate-key",
	"duplicate-obligation", "duplicate-or-unsorted-claims", "duplicate-or-unsorted-reference",
	"duplicate-requirement", "evidence-drift", "evidence-unavailable", "excluded-artifact-mismatch",
	"excluded-path-not-file", "expected-base-required", "extra-hunk-payload", "fabricated-claim-id",
	"fabricated-evidence-id", "git-budget-exceeded", "git-cancelled", "git-diff-failed",
	"git-diff-timeout", "git-exit-failure", "git-output-exceeded", "git-read-failed",
	"git-start-failed", "git-timeout", "intent-scope-drift", "intent-scope-mismatch",
	"internal-error", "invalid-arguments", "invalid-base-revision", "invalid-cem",
	"invalid-cem-digest", "invalid-claim-blob", "invalid-claim-extractor", "invalid-claim-references",
	"invalid-claims", "invalid-excluded-path", "invalid-field", "invalid-hunk-references",
	"invalid-integer", "invalid-intent", "invalid-intent-blob", "invalid-intent-scope",
	"invalid-json", "invalid-line-range", "invalid-linked", "invalid-object", "invalid-obligation",
	"invalid-obligation-id", "invalid-obligation-selector", "invalid-obligations", "invalid-ocm",
	"invalid-path", "invalid-reference-array", "invalid-reference-id", "invalid-requirement-prefix",
	"invalid-requirements-section", "invalid-selector", "invalid-span", "invalid-span-digest",
	"invalid-target-revision", "invalid-unknown", "invalid-unknown-reason", "line-range-out-of-range",
	"malformed-patch", "map-locked", "map-outdated", "map-too-large", "map-type", "map-unavailable",
	"missing-claim", "missing-evidence", "missing-field", "missing-hunk", "missing-intent-scope",
	"missing-requirements", "noncanonical-map", "not-an-object", "obligation-order-mismatch",
	"obligation-selector-out-of-range", "obligation-set-mismatch", "ocm-cli-error",
	"orphan-evidence", "output-failed", "output-path-conflict", "patch-digest-mismatch",
	"patch-unavailable", "path-traversal", "publish-failed", "record-failed", "record-index-failed",
	"repository-identity-changed", "repository-object-unavailable", "span-out-of-range",
	"target-mismatch", "target-required", "too-many-claims", "too-many-lines", "too-many-obligations",
	"uncited-hunk", "unknown-claim-reference", "unknown-field", "unknown-hunk-id",
	"unknown-hunk-reference", "unknown-obligation-id", "unproven-mechanical", "unsafe-output",
	"unsupported-hunk-selector", "unsupported-lrf-context", "unsupported-object-alternates",
	"unsupported-ocm-closure-profile", "unsupported-ocm-json-profile", "unsupported-ocm-output-path",
	"unsupported-ocm-python-claims", "unsupported-repository-attributes", "unsupported-spec",
	"unsupported-tree-mode", "unsupported-verify-syntax", "unsupported-without-basis",
)

func codeSet(values ...string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

// Error is a writer-contract refusal. Error deliberately returns only a
// stable code: the rejected value may itself be task, source, or command text
// and must not be echoed into another diagnostic surface.
type Error struct {
	Code  string
	Field string
}

func (err *Error) Error() string { return err.Code }

type Event struct {
	Kind             string   `json:"kind"`
	Event            string   `json:"event,omitempty"`
	ReceiptID        string   `json:"receiptId,omitempty"`
	SessionID        string   `json:"sessionIdSha256,omitempty"`
	TaskSHA256       string   `json:"taskSha256,omitempty"`
	Support          string   `json:"support,omitempty"`
	Freshness        string   `json:"freshness,omitempty"`
	Degradations     []string `json:"degradations,omitempty"`
	UncertaintyCount int      `json:"uncertaintyCount,omitempty"`
	OmittedCount     int      `json:"omittedCount,omitempty"`
	CriticalMissing  int      `json:"criticalMissingCount,omitempty"`
	Authoritative    int      `json:"authoritativeResultCount,omitempty"`
	LatencyMS        int64    `json:"latencyMs,omitempty"`
	TouchedPaths     []string `json:"touchedPaths,omitempty"`
	RankedPaths      []string `json:"rankedPaths,omitempty"`
	MissState        string   `json:"missState,omitempty"`
	Code             string   `json:"code,omitempty"`
	QueryIntent      string   `json:"queryIntent,omitempty"`
	Step             string   `json:"step,omitempty"`
	Status           string   `json:"status,omitempty"`
	Reason           string   `json:"reason,omitempty"`
	// Counts is a `proof` row: the verdict counts of one `prove` document,
	// keyed by falsifier then verdict, and nothing else about the document.
	Counts map[string]map[string]int `json:"counts,omitempty"`
	// Host, AdapterCodes, Window and CorvintVersion belong to an `adapter-degradation`
	// row (SOL-V0-010) only.
	Host           string   `json:"host,omitempty"`
	AdapterCodes   []string `json:"adapterCodes,omitempty"`
	Window         string   `json:"window,omitempty"`
	CorvintVersion string   `json:"corvintVersion,omitempty"`
}

// AdapterDegradationEvent is the SOL-V0-010 row for one adapter degradation reason.
// A `corvint-event-rejected:<code>` reason whose code no closed registry names keeps
// only its `corvint-event-rejected` prefix; any other unadmitted reason is refused by
// Append.
func AdapterDegradationEvent(host, event, reason, corvintVersion string, now time.Time) Event {
	return Event{Kind: KindAdapterDegradation, Host: host, Event: event, AdapterCodes: []string{adapterDegradationCode(reason)}, Window: now.UTC().Format(adapterWindowLayout), CorvintVersion: corvintVersion}
}

func adapterDegradationCode(reason string) string {
	base, code, qualified := strings.Cut(reason, ":")
	if !qualified || base != "corvint-event-rejected" || admittedAdapterRejection(code) {
		return reason
	}
	return base
}

func admittedAdapterRejection(code string) bool {
	return admittedAdapterRejections[code] || admittedDogfoodErrorCodes[code] || admittedUnsupportedCodes[code]
}

func TaskHash(task string) string {
	digest := sha256.Sum256([]byte(task))
	return hex.EncodeToString(digest[:])
}

// Append is deliberately fail-open. It only writes a bounded JSONL ledger below
// the repository worktree and never creates an index, trace, or agent-memory file.
func Append(root string, event Event) error {
	if err := validateWriterContract(event); err != nil {
		return err
	}
	if !ledgerIgnored(root) {
		return fmt.Errorf("self-observation ledger is not safely gitignored")
	}
	screenEvent(&event)
	encoded, err := json.Marshal(event)
	if err != nil || len(encoded)+1 > maxRowBytes {
		return fmt.Errorf("observation row exceeds bound")
	}
	directory := filepath.Join(root, ".corvint")
	path := filepath.Join(directory, "self-observations.jsonl")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	// A symlinked .corvint or ledger would move the write, the temporary sweep,
	// or the previous-row read outside the worktree.
	if info, err := os.Lstat(directory); err != nil || !info.IsDir() {
		return fmt.Errorf("self-observation ledger directory is not a directory")
	}
	appendProcessLock.Lock()
	defer appendProcessLock.Unlock()
	lock, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := lockObservationFile(lock); err != nil {
		return err
	}
	defer unlockObservationFile(lock)
	var previous []byte
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("self-observation ledger is not a regular file")
	}
	ledger, err := os.Open(path)
	if err == nil {
		defer ledger.Close()
		info, statErr := ledger.Stat()
		if statErr != nil {
			return statErr
		}
		previous, err = readPreviousRows(ledger, info.Size(), len(encoded)+1)
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if event.Kind == KindAdapterDegradation && retainsRow(previous, encoded) {
		return nil
	}
	data := append(append(previous, encoded...), '\n')
	return replaceLedger(directory, path, data)
}

// retainsRow reports whether previous already holds row as a complete line: SOL-V0-010
// deduplication, since a row's window names its hour.
func retainsRow(previous, row []byte) bool {
	line := append(append([]byte("\n"), row...), '\n')
	return bytes.Contains(append([]byte("\n"), previous...), line)
}

func readPreviousRows(reader io.ReaderAt, size int64, appendedBytes int) ([]byte, error) {
	readOffset := int64(0)
	readBytes := size
	rotating := size > int64(maxFileBytes-appendedBytes)
	if rotating {
		readBytes = maxFileBytes / 2
		readOffset = size - readBytes
	}
	previous := make([]byte, readBytes)
	if _, err := io.ReadFull(io.NewSectionReader(reader, readOffset, readBytes), previous); err != nil {
		return nil, err
	}
	if rotating && readOffset > 0 {
		if next := bytes.IndexByte(previous, '\n'); next >= 0 {
			previous = append([]byte(nil), previous[next+1:]...)
		}
	}
	return previous[:bytes.LastIndexByte(previous, '\n')+1], nil
}

// validateWriterContract enforces SOL-V0-002 before redaction. Each admitted
// row kind has a closed field set; its strings are hashes, machine codes, or
// normalized repository-relative paths, never free-form text.
func validateWriterContract(event Event) error {
	if !validObservationCode(event.Kind, 32) {
		return prohibitedContent("kind")
	}
	if event.Kind == KindAdapterDegradation {
		return validateAdapterDegradation(event)
	}
	if hasAdapterDegradationFields(event) {
		return prohibitedContent("adapter-degradation-fields")
	}
	switch event.Kind {
	case "event":
		if event.Code != "" || event.QueryIntent != "" || event.Step != "" || event.Status != "" || event.Reason != "" || len(event.Counts) != 0 {
			return prohibitedContent("event-schema")
		}
		if err := validateLifecycleFields(event); err != nil {
			return err
		}
	case "unsupported":
		if event.Event != "" || event.ReceiptID != "" || event.SessionID != "" || event.Support != "" || event.Freshness != "" || len(event.Degradations) != 0 || event.UncertaintyCount != 0 || event.OmittedCount != 0 || event.CriticalMissing != 0 || event.Authoritative != 0 || event.LatencyMS != 0 || len(event.TouchedPaths) != 0 || len(event.RankedPaths) != 0 || event.MissState != "" || event.Step != "" || event.Status != "" || event.Reason != "" || len(event.Counts) != 0 {
			return prohibitedContent("unsupported-schema")
		}
	case "dogfood-step":
		if event.Event != "" || event.ReceiptID != "" || event.SessionID != "" || event.TaskSHA256 != "" || event.Support != "" || event.Freshness != "" || len(event.Degradations) != 0 || event.UncertaintyCount != 0 || event.OmittedCount != 0 || event.CriticalMissing != 0 || event.Authoritative != 0 || event.LatencyMS != 0 || len(event.TouchedPaths) != 0 || len(event.RankedPaths) != 0 || event.MissState != "" || event.Code != "" || event.QueryIntent != "" || len(event.Counts) != 0 {
			return prohibitedContent("dogfood-step-schema")
		}
	case "proof":
		if event.Event != "" || event.ReceiptID != "" || event.SessionID != "" || event.TaskSHA256 != "" || event.Support != "" || event.Freshness != "" || len(event.Degradations) != 0 || event.UncertaintyCount != 0 || event.OmittedCount != 0 || event.CriticalMissing != 0 || event.Authoritative != 0 || event.LatencyMS != 0 || len(event.TouchedPaths) != 0 || len(event.RankedPaths) != 0 || event.MissState != "" || event.Code != "" || event.QueryIntent != "" || event.Step != "" || event.Status != "" || event.Reason != "" {
			return prohibitedContent("proof-schema")
		}
	default:
		return prohibitedContent("kind")
	}

	if event.Event != "" && event.Event != "session-start" && event.Event != "user-prompt" && event.Event != "file-change" {
		return prohibitedContent("event")
	}
	if event.ReceiptID != "" && (!strings.HasPrefix(event.ReceiptID, "harness-receipt:sha256:") || !validLowerHex(event.ReceiptID[len("harness-receipt:sha256:"):], 64)) {
		return prohibitedContent("receiptId")
	}
	if event.SessionID != "" && !validLowerHex(event.SessionID, 64) {
		return prohibitedContent("sessionIdSha256")
	}
	if event.TaskSHA256 != "" && !validLowerHex(event.TaskSHA256, 64) {
		return prohibitedContent("taskSha256")
	}
	if event.Support != "" && event.Support != "FALLBACK" {
		return prohibitedContent("support")
	}
	if event.Freshness != "" && event.Freshness != "fresh" && event.Freshness != "mixed-worktree" {
		return prohibitedContent("freshness")
	}
	for _, degradation := range event.Degradations {
		if !admittedDegradations[degradation] {
			return prohibitedContent("degradations")
		}
	}
	for _, path := range append(append([]string(nil), event.TouchedPaths...), event.RankedPaths...) {
		if !validObservationPath(path) {
			return prohibitedContent("paths")
		}
	}
	if event.MissState != "" && event.MissState != "OBSERVED" && event.MissState != "MISS_DETECTION_NOT_OBSERVED" {
		return prohibitedContent("missState")
	}
	if event.Code != "" && !admittedUnsupportedCodes[event.Code] {
		return prohibitedContent("code")
	}
	if event.QueryIntent != "" && event.QueryIntent != "unknown" && event.QueryIntent != "repository" && event.QueryIntent != "project-operations" && event.QueryIntent != "agent-tooling" {
		return prohibitedContent("queryIntent")
	}
	if event.Step != "" && !validDogfoodStep(event.Step) {
		return prohibitedContent("step")
	}
	if event.Status != "" && event.Status != "PRODUCED" && event.Status != "NOT_PRODUCED" {
		return prohibitedContent("status")
	}
	if event.Reason != "" && !validDogfoodReason(event.Reason) {
		return prohibitedContent("reason")
	}
	for falsifier, verdicts := range event.Counts {
		if !admittedFalsifiers[falsifier] {
			return prohibitedContent("counts-falsifier")
		}
		for verdict := range verdicts {
			if !admittedProofVerdicts[verdict] {
				return prohibitedContent("counts-verdict")
			}
		}
	}
	return nil
}

// validateAdapterDegradation admits exactly host, event, closed codes, an hour window
// and the Corvint version: no prompt text, path, tool input or repository content.
func validateAdapterDegradation(event Event) error {
	bare := Event{Kind: event.Kind, Event: event.Event, Host: event.Host, AdapterCodes: event.AdapterCodes, Window: event.Window, CorvintVersion: event.CorvintVersion}
	if !reflect.DeepEqual(bare, event) {
		return prohibitedContent("adapter-degradation-schema")
	}
	if !admittedAdapterHosts[event.Host] {
		return prohibitedContent("host")
	}
	if !admittedAdapterEvents[event.Event] {
		return prohibitedContent("event")
	}
	if !validAdapterCodes(event.AdapterCodes) {
		return prohibitedContent("adapterCodes")
	}
	if parsed, err := time.Parse(adapterWindowLayout, event.Window); err != nil || parsed.Format(adapterWindowLayout) != event.Window {
		return prohibitedContent("window")
	}
	if !validCorvintVersion(event.CorvintVersion) {
		return prohibitedContent("corvintVersion")
	}
	return nil
}

func hasAdapterDegradationFields(event Event) bool {
	return event.Host != "" || len(event.AdapterCodes) != 0 || event.Window != "" || event.CorvintVersion != ""
}

// validAdapterCodes admits a non-empty, sorted, duplicate-free set of closed codes.
func validAdapterCodes(codes []string) bool {
	if len(codes) == 0 || len(codes) > maxAdapterCodes {
		return false
	}
	for index, code := range codes {
		if index > 0 && codes[index-1] >= code {
			return false
		}
		if !validAdapterCode(code) {
			return false
		}
	}
	return true
}

func validAdapterCode(code string) bool {
	base, rejection, qualified := strings.Cut(code, ":")
	if !qualified {
		return admittedAdapterCodes[code]
	}
	return base == "corvint-event-rejected" && admittedAdapterRejection(rejection)
}

func validCorvintVersion(value string) bool {
	if value == "" || len(value) > 32 {
		return false
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '.' || character == '+' || character == '-') {
			return false
		}
	}
	return true
}

func validateLifecycleFields(event Event) error {
	switch event.Event {
	case "":
		if event.ReceiptID != "" || event.SessionID != "" || event.TaskSHA256 != "" || event.Support != "" || event.Freshness != "" || len(event.Degradations) != 0 || event.UncertaintyCount != 0 || event.OmittedCount != 0 || event.CriticalMissing != 0 || event.Authoritative != 0 || event.LatencyMS != 0 || len(event.TouchedPaths) != 0 || len(event.RankedPaths) != 0 || event.MissState != "" {
			return prohibitedContent("event-schema")
		}
		return nil
	case "session-start":
		if event.TaskSHA256 != "" || len(event.TouchedPaths) != 0 || len(event.RankedPaths) != 0 || event.MissState != "" {
			return prohibitedContent("session-start-schema")
		}
	case "user-prompt":
		if len(event.TouchedPaths) != 0 || len(event.RankedPaths) != 0 || event.MissState != "" {
			return prohibitedContent("user-prompt-schema")
		}
	case "file-change":
		if event.TaskSHA256 != "" {
			return prohibitedContent("file-change-schema")
		}
	default:
		return prohibitedContent("event")
	}
	return nil
}

func prohibitedContent(field string) *Error {
	return &Error{Code: CodeProhibitedContent, Field: field}
}

func validObservationCode(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '_' || character == '-') {
			return false
		}
	}
	return true
}

func validDogfoodStep(value string) bool {
	switch value {
	case "cem-cite", "cem-prepare", "cem-status", "local-outcome", "ocm-aggregate", "prechange-impact", "prechange-query":
		return true
	}
	for _, prefix := range []string{"ocm-prepare-", "ocm-status-"} {
		if strings.HasPrefix(value, prefix) && len(value) == len(prefix)+3 {
			digits := value[len(prefix):]
			return digits >= "001" && digits <= "016"
		}
	}
	return false
}

func validDogfoodReason(value string) bool {
	if len(value) > 96 {
		return false
	}
	switch value {
	case "cem-map-not-produced", "citation-plan-not-provided", "citation-plan-unavailable",
		"intent-scope-drift", "invalid-citation-plan", "invalid-record-admission-output",
		"missing-intent-scope", "no-source-paths", "none", "not-ready", "outcome-input-not-provided":
		return true
	}
	if admittedDogfoodErrorCodes[value] || admittedUnsupportedCodes[value] {
		return true
	}
	if strings.HasPrefix(value, "exit-") && len(value) > len("exit-") {
		for _, character := range value[len("exit-"):] {
			if character < '0' || character > '9' {
				return false
			}
		}
		return true
	}
	return false
}

func validLowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func validObservationPath(value string) bool {
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 1024 || strings.HasPrefix(value, "/") {
		return false
	}
	if !lineSafe(value) {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// lineSafe reports whether value holds no control character or Unicode line
// or paragraph separator, so it cannot break a triage line when rendered.
func lineSafe(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) || character == '\u2028' || character == '\u2029' {
			return false
		}
	}
	return true
}

// renderedKeysLineSafe reports whether every string triage renders as a key
// from this row is lineSafe. Rows are hand-editable, so the writer contract
// cannot be assumed on read.
func renderedKeysLineSafe(event Event) bool {
	keys := append(append(append([]string{event.Code, event.QueryIntent}, event.Degradations...), event.TouchedPaths...), event.RankedPaths...)
	for falsifier := range event.Counts {
		keys = append(keys, falsifier)
	}
	for _, key := range keys {
		if !lineSafe(key) {
			return false
		}
	}
	return true
}

// screenEvent redacts secret-shaped content from every admitted textual
// surface as defence in depth after the SOL-V0-002 writer contract has
// refused free-form task, source, and command text.
func screenEvent(event *Event) {
	event.Support, _ = secretscreen.Screen(event.Support)
	event.Freshness, _ = secretscreen.Screen(event.Freshness)
	event.Code, _ = secretscreen.Screen(event.Code)
	event.QueryIntent, _ = secretscreen.Screen(event.QueryIntent)
	event.Step, _ = secretscreen.Screen(event.Step)
	event.Reason, _ = secretscreen.Screen(event.Reason)
	if len(event.Degradations) > 0 {
		screened := make([]string, len(event.Degradations))
		for index, degradation := range event.Degradations {
			screened[index], _ = secretscreen.Screen(degradation)
		}
		event.Degradations = screened
	}
	event.TouchedPaths = screenStrings(event.TouchedPaths)
	event.RankedPaths = screenStrings(event.RankedPaths)
	event.Host, _ = secretscreen.Screen(event.Host)
	event.Window, _ = secretscreen.Screen(event.Window)
	event.CorvintVersion, _ = secretscreen.Screen(event.CorvintVersion)
	event.AdapterCodes = screenStrings(event.AdapterCodes)
}

func screenStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	screened := make([]string, len(values))
	for index, value := range values {
		screened[index], _ = secretscreen.Screen(value)
	}
	return screened
}

// removeStaleTemporaries clears temporaries left by a writer that died between
// write and rename (a killed harness hook leaves one, and its deferred remove
// never runs). The caller holds the directory lock, so every temporary present
// here belongs to a dead process; a stale one would keep the tree dirty.
func removeStaleTemporaries(directory string) {
	stale, _ := filepath.Glob(filepath.Join(directory, ".self-observations.*"))
	for _, path := range stale {
		os.Remove(path)
	}
}

func replaceLedger(directory, path string, data []byte) error {
	removeStaleTemporaries(directory)
	temporary, err := os.CreateTemp(directory, ".self-observations.*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	defer temporary.Close()
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	directoryFile, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer directoryFile.Close()
	return directoryFile.Sync()
}

// ledgerIgnored is intentionally conservative. A lifecycle receipt must never
// make a repository dirty merely to record an advisory observation.
func ledgerIgnored(root string) bool {
	return CorvintEntriesIgnored(root, "self-observations.jsonl", ".self-observations.*")
}

// CorvintEntriesIgnored reports whether one ignore file, the root .gitignore or
// .corvint/.gitignore, ignores every named .corvint entry by its exact rule or by
// ignoring all of .corvint, and holds no negation. The unplanned-read ledger
// shares this conservative check (URE-V0-003).
func CorvintEntriesIgnored(root string, names ...string) bool {
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err == nil && ignoreCoversEach(data, names, rootIgnoreRules) {
		return true
	}
	data, err = os.ReadFile(filepath.Join(root, ".corvint", ".gitignore"))
	return err == nil && ignoreCoversEach(data, names, corvintIgnoreRules)
}

func rootIgnoreRules(name string) map[string]bool {
	return map[string]bool{".corvint/": true, "/.corvint/": true, ".corvint/" + name: true, "/.corvint/" + name: true}
}

func corvintIgnoreRules(name string) map[string]bool {
	return map[string]bool{"*": true, "/*": true, name: true, "/" + name: true}
}

func ignoreCoversEach(data []byte, names []string, rules func(string) map[string]bool) bool {
	for _, name := range names {
		if !ignoreRulesCover(data, rules(name)) {
			return false
		}
	}
	return true
}

func ignoreRulesCover(data []byte, accepted map[string]bool) bool {
	covered := false
	for _, line := range strings.Split(string(data), "\n") {
		entry := strings.TrimSpace(line)
		if strings.HasPrefix(entry, "!") {
			return false
		}
		if accepted[entry] {
			covered = true
		}
	}
	return covered
}

func keepNewest(data []byte, maximum int) []byte {
	if len(data) <= maximum {
		return data
	}
	start := len(data) - maximum
	if next := strings.IndexByte(string(data[start:]), '\n'); next >= 0 {
		start += next + 1
	}
	return append([]byte(nil), data[start:]...)
}

type Digest struct {
	Events       int
	Degradations map[string]int
	Misses       map[string]int
	Unsupported  map[string]int
	ZeroResults  int
	BudgetOmit   int
	Latencies    []int64
	// Proofs counts `proof` rows; Verdicts sums their counts by falsifier
	// then verdict.
	Proofs   int
	Verdicts map[string]map[string]int
	// Adapter counts retained `adapter-degradation` rows by host/event/code, and
	// AdapterLatest keeps each key's newest hour window (SOL-V0-010).
	Adapter       map[string]int
	AdapterLatest map[string]string
	// SkippedRows counts ledger rows over maxRowBytes: self-observation-ledger-v0.md
	// SOL-V0-005 (failure modes, "malformed ledger row") requires triage to skip
	// an oversized row and keep reading bounded rows rather than abort.
	SkippedRows int
}

// Falsification is the share of judged rows (PASS or FAIL; NOT_RUN and
// `none` rows are not judgments) whose claim was falsified, over every
// `proof` row the ledger retains.
type Falsification struct {
	Proofs int    `json:"proofs"`
	Judged int    `json:"judged"`
	Failed int    `json:"failed"`
	Rate   string `json:"rate"`
}

// openLedger opens the ledger for triage only when `.corvint` is a real
// directory and the ledger a regular file, verified against the opened file,
// so a symlink cannot move the read outside the worktree.
func openLedger(root string) (*os.File, error) {
	directory := filepath.Join(root, ".corvint")
	directoryInfo, err := os.Lstat(directory)
	if err != nil {
		return nil, err
	}
	if !directoryInfo.IsDir() {
		return nil, errLedgerNotRegular
	}
	path := filepath.Join(directory, "self-observations.jsonl")
	linkInfo, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !linkInfo.Mode().IsRegular() {
		return nil, errLedgerNotRegular
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err == nil && os.SameFile(linkInfo, opened) {
		return file, nil
	}
	file.Close()
	return nil, errLedgerNotRegular
}

func Read(root string) (Digest, error) {
	result := Digest{Degradations: map[string]int{}, Misses: map[string]int{}, Unsupported: map[string]int{}, Verdicts: map[string]map[string]int{}, Adapter: map[string]int{}, AdapterLatest: map[string]string{}}
	file, err := openLedger(root)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	defer file.Close()
	reader := bufio.NewReader(io.LimitReader(file, maxFileBytes+1))
	for {
		row, err := reader.ReadBytes('\n')
		if trimmed := bytes.TrimSuffix(row, []byte("\n")); len(trimmed) > 0 {
			if len(trimmed) > maxRowBytes {
				result.SkippedRows++
			} else if len(row) > 0 && row[len(row)-1] == '\n' {
				readRow(&result, trimmed)
			}
		}
		if err != nil {
			if err == io.EOF {
				return result, nil
			}
			return result, err
		}
	}
}

// readRow folds one well-formed ledger row into the digest. A row that fails
// to parse, or whose rendered key could break a triage line, is silently
// skipped, same as an oversized row counted by the caller.
func readRow(result *Digest, row []byte) {
	if !strictJSONObject(row) {
		return
	}
	var event Event
	if json.Unmarshal(row, &event) != nil {
		return
	}
	if !renderedKeysLineSafe(event) {
		return
	}
	if event.Kind == "event" {
		result.Events++
	}
	for _, code := range event.Degradations {
		result.Degradations[code]++
	}
	if event.OmittedCount > 0 {
		result.BudgetOmit++
	}
	if event.Kind == "event" && event.Event == "user-prompt" && event.Authoritative == 0 {
		result.ZeroResults++
	}
	if event.LatencyMS > 0 {
		result.Latencies = append(result.Latencies, event.LatencyMS)
	}
	if event.Event == "file-change" && event.MissState == "OBSERVED" {
		for _, path := range misses(event.TouchedPaths, event.RankedPaths) {
			result.Misses[path]++
		}
	}
	if event.Kind == "unsupported" {
		result.Unsupported[event.Code+"/"+event.QueryIntent]++
	}
	if event.Kind == "proof" && wellFormedProof(event.Counts) {
		result.Proofs++
		addVerdicts(result.Verdicts, event.Counts)
	}
	if event.Kind == KindAdapterDegradation && validateAdapterDegradation(event) == nil {
		addAdapterDegradation(result, event)
	}
}

func strictJSONObject(row []byte) bool {
	if !utf8.Valid(row) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(row))
	decoder.UseNumber()
	if !strictJSONValue(decoder, true) {
		return false
	}
	_, err := decoder.Token()
	return err == io.EOF
}

func strictJSONValue(decoder *json.Decoder, objectOnly bool) bool {
	token, err := decoder.Token()
	if err != nil {
		return false
	}
	delimiter, composite := token.(json.Delim)
	if objectOnly && (!composite || delimiter != '{') {
		return false
	}
	if !composite {
		return true
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			key, ok := keyToken.(string)
			if err != nil || !ok || seen[key] {
				return false
			}
			seen[key] = true
			if !strictJSONValue(decoder, false) {
				return false
			}
		}
		end, err := decoder.Token()
		return err == nil && end == json.Delim('}')
	case '[':
		for decoder.More() {
			if !strictJSONValue(decoder, false) {
				return false
			}
		}
		end, err := decoder.Token()
		return err == nil && end == json.Delim(']')
	default:
		return false
	}
}

func addAdapterDegradation(result *Digest, event Event) {
	for _, code := range event.AdapterCodes {
		key := event.Host + "/" + event.Event + "/" + code
		result.Adapter[key]++
		if event.Window > result.AdapterLatest[key] {
			result.AdapterLatest[key] = event.Window
		}
	}
}

// maxProofCount bounds one verdict count: a proof judges at most a few hundred
// rows, and the bound keeps a hand-edited row from overflowing the sums.
const maxProofCount = 1 << 20

var proofVerdicts = map[string]bool{"PASS": true, "FAIL": true, "NOT_RUN": true}

// wellFormedProof skips a proof row the rate cannot trust: an unknown verdict
// or a count outside [0, maxProofCount]. The falsifier names are prove's own
// closed set; `none` is accepted here and excluded from the judgment below.
func wellFormedProof(counts map[string]map[string]int) bool {
	for _, verdicts := range counts {
		for verdict, count := range verdicts {
			if !proofVerdicts[verdict] || count < 0 || count > maxProofCount {
				return false
			}
		}
	}
	return len(counts) > 0
}

func addVerdicts(total, counts map[string]map[string]int) {
	for falsifier, verdicts := range counts {
		if total[falsifier] == nil {
			total[falsifier] = map[string]int{}
		}
		for verdict, count := range verdicts {
			total[falsifier][verdict] += count
		}
	}
}

// Falsification reports the rate over every falsifier; the digest is
// meaningful only when Proofs is positive.
func (digest Digest) Falsification() Falsification {
	judged, failed := 0, 0
	for falsifier, verdicts := range digest.Verdicts {
		if falsifier == "none" {
			continue
		}
		judged += verdicts["PASS"] + verdicts["FAIL"]
		failed += verdicts["FAIL"]
	}
	return Falsification{Proofs: digest.Proofs, Judged: judged, Failed: failed, Rate: rate(failed, judged)}
}

func misses(touched, ranked []string) []string {
	seen := map[string]bool{}
	for _, path := range ranked {
		seen[path] = true
	}
	result := make([]string, 0)
	for _, path := range touched {
		if !seen[path] {
			result = append(result, path)
		}
	}
	return result
}

func Render(root string, limit int, output io.Writer) error {
	if limit < 1 {
		return fmt.Errorf("limit must be positive")
	}
	data, err := Read(root)
	if err != nil {
		return err
	}
	lines := []string{fmt.Sprintf("SELF-OBSERVATIONS events=%d zero-authoritative-rate=%s budget-omission-rate=%s latency-p50=%dms latency-p95=%dms", data.Events, rate(data.ZeroResults, data.Events), rate(data.BudgetOmit, data.Events), percentile(data.Latencies, 50), percentile(data.Latencies, 95))}
	if data.SkippedRows > 0 {
		lines = append(lines, fmt.Sprintf("SKIPPED-ROWS count=%d oversized", data.SkippedRows))
	}
	lines = append(lines, falsificationLines(data)...)
	lines = append(lines, ranked("DEGRADATION", data.Degradations, data.Events, true)...)
	lines = append(lines, ranked("ROUTING-MISS", data.Misses, data.Events, false)...)
	lines = append(lines, ranked("UNSUPPORTED", data.Unsupported, data.Events, false)...)
	lines = append(lines, adapterLines(data)...)
	for _, line := range lines[:min(limit, min(120, len(lines)))] {
		if _, err := fmt.Fprintln(output, line); err != nil {
			return err
		}
	}
	return nil
}

// falsificationLines is the repository's falsification rate over the retained
// proofs, then one line per falsifier in name order. Nothing is drafted: the
// rate is the number, not a finding.
func falsificationLines(data Digest) []string {
	if data.Proofs == 0 {
		return nil
	}
	overall := data.Falsification()
	lines := []string{fmt.Sprintf("FALSIFICATION proofs=%d judged=%d failed=%d rate=%s", overall.Proofs, overall.Judged, overall.Failed, overall.Rate)}
	names := make([]string, 0, len(data.Verdicts))
	for name := range data.Verdicts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		verdicts := data.Verdicts[name]
		judged, failed := verdicts["PASS"]+verdicts["FAIL"], verdicts["FAIL"]
		if name == "none" {
			judged, failed = 0, 0
		}
		lines = append(lines, fmt.Sprintf("FALSIFIER key=%s judged=%d failed=%d not-run=%d rate=%s", name, judged, failed, verdicts["NOT_RUN"], rate(failed, judged)))
	}
	return lines
}

// adapterLines tallies retained adapter degradations by host/event/code. A count is
// the number of hour windows that recorded the code, not its occurrences.
func adapterLines(data Digest) []string {
	keys := make([]string, 0, len(data.Adapter))
	for key := range data.Adapter {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("ADAPTER-DEGRADATION windows=%d key=%s latest=%s", data.Adapter[key], key, data.AdapterLatest[key]))
	}
	return lines
}

func ranked(kind string, values map[string]int, total int, standing bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		label := "DRAFT ideas.md"
		if standing && total > 0 && values[key]*100 >= total*90 {
			label = "STANDING DRAFT fixes.md"
		}
		if kind == "UNSUPPORTED" && values[key] >= 5 {
			label = unsupportedDraft(key)
		}
		if decision := byDesignDecision(kind, key); decision != "" {
			result = append(result, fmt.Sprintf("UNSUPPORTED-BY-DESIGN count=%d key=%s decision=%s", values[key], key, decision))
			continue
		}
		result = append(result, fmt.Sprintf("%s count=%d key=%s %s", kind, values[key], key, label))
	}
	return result
}

// byDesignDecision is the decision record for an UNSUPPORTED code/intent key
// whose code the by-design registry names, and "" for any other key.
func byDesignDecision(kind, key string) string {
	if kind != "UNSUPPORTED" {
		return ""
	}
	code, _, _ := strings.Cut(key, "/")
	return unsupportedByDesign[code]
}

func unsupportedDraft(key string) string {
	if strings.HasPrefix(key, "unsupported-query-") {
		return "CAPABILITY-GAP DRAFT ideas.md owning-spec=go-production-kernel-migration-v0.md"
	}
	return "CAPABILITY-GAP DRAFT ideas.md unsupported-with-no-owning-spec"
}

func percentile(values []int64, percent int) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	index := (len(sorted)*percent+99)/100 - 1
	return sorted[index]
}

func rate(part, total int) string {
	if total == 0 {
		return "NOT_OBSERVED"
	}
	return fmt.Sprintf("%d/%d", part, total)
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
