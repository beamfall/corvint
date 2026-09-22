package doccompiler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// Closed vocabularies of the admitted HDCV0-027..029 plan wire (decision 0231).
const (
	PlanEnvironmentPlanOnly = "plan-only"
	PlanSnapshotNotRun      = "NOT_RUN"

	OperationCreateFile  = "create_file"
	OperationInsertAfter = "insert_after"
	OperationEditNav     = "edit_nav"

	TargetAbsent  = "ABSENT"
	TargetPresent = "PRESENT"

	maxPlanOperations   = 2_000
	maxPlanSourceFiles  = 10_000
	maxPlanSourceBytes  = 128 << 20
	maxPlanTargetBytes  = 1 << 20
	maxPlanReasonBytes  = 4_096
	unverifiedAnchorSrc = "anchor-source-unverified"
)

// AdmittedPlan is the closed corvint-human-documentation-plan/0 wire of HDCV0-027.
// Build, offline, and artifact digests are receipt-only and have no member here.
type AdmittedPlan struct {
	Profile               string                    `json:"profile"`
	SourceIdentity        PlanSourceIdentity        `json:"source_identity"`
	EnvironmentPin        PlanEnvironmentPin        `json:"environment_pin"`
	ConfigurationSnapshot PlanConfigurationSnapshot `json:"configuration_snapshot"`
	Clauses               []Clause                  `json:"clauses"`
	Operations            []PlanOperation           `json:"operations"`
	CandidatePatchSHA256  string                    `json:"candidate_patch_sha256"`
	Uncertainty           []string                  `json:"uncertainty"`
	LimitsConsumed        PlanLimits                `json:"limits_consumed"`
	Exclusions            []PlanExclusion           `json:"exclusions"`
}

// PlanSourceIdentity is the Git revision plus every consumed source, ordered by path.
type PlanSourceIdentity struct {
	Revision string       `json:"revision"`
	Sources  []PlanSource `json:"sources"`
}

type PlanSource struct {
	Path   string `json:"path"`
	Mode   string `json:"mode"`
	Blob   string `json:"blob"`
	SHA256 string `json:"sha256"`
}

// PlanEnvironmentPin is the HDCV0-CAP-001 plan-only variant; no execution pin is admitted.
type PlanEnvironmentPin struct {
	Kind string `json:"kind"`
}

// PlanConfigurationSnapshot records that no MkDocs loader ran for a plan-only request.
type PlanConfigurationSnapshot struct {
	Status string `json:"status"`
}

// PlanOperation is one HDCV0-028 operation with its optimistic-concurrency precondition.
type PlanOperation struct {
	ID                string   `json:"id"`
	Kind              string   `json:"kind"`
	Target            string   `json:"target"`
	TargetState       string   `json:"target_state"`
	TargetBlob        string   `json:"target_blob"`
	TargetSHA256      string   `json:"target_sha256"`
	StartByte         int      `json:"start_byte"`
	EndByte           int      `json:"end_byte"`
	ReplacementSHA256 string   `json:"replacement_sha256"`
	ClauseIDs         []string `json:"clause_ids"`
	Reason            string   `json:"reason"`
}

type PlanLimits struct {
	Anchors     int `json:"anchors"`
	Clauses     int `json:"clauses"`
	Operations  int `json:"operations"`
	SourceBytes int `json:"source_bytes"`
	SourceFiles int `json:"source_files"`
}

type PlanExclusion struct {
	Code string `json:"code"`
	Path string `json:"path"`
}

// AdmittedPlanRequest is the plan-only compiler input: candidate clauses and
// the documentation targets that should carry their admitted rendering.
type AdmittedPlanRequest struct {
	Clauses   []Clause
	Documents []PlanDocument
}

type PlanDocument struct {
	ID        string
	Target    string
	ClauseIDs []string
	Reason    string
}

var planUncertainty = []string{"configuration-snapshot-not-run", "environment-plan-only"}

// CompileAdmittedPlan admits the request clauses (HDCV0-023..026) and emits the
// canonical plan plus the proposal patch whose SHA-256 the plan binds. It reads
// only index and never touches a worktree or starts a process.
func CompileAdmittedPlan(index *contextindex.Index, request AdmittedPlanRequest) ([]byte, []byte, error) {
	if !revisionPattern.MatchString(index.CommitRevision) {
		return nil, nil, failure("invalid-source", "index revision must be 40 or 64 lowercase hex digits")
	}
	if len(request.Documents) > maxPlanOperations {
		return nil, nil, failure("plan-limit-exceeded", "more than %d operations", maxPlanOperations)
	}
	admitted, err := AdmitClauses(index, request.Clauses)
	if err != nil {
		return nil, nil, err
	}
	byID := make(map[string]Clause, len(admitted))
	for _, clause := range admitted {
		clause.Anchors = append([]Anchor{}, clause.Anchors...)
		byID[clause.ID] = clause
	}
	operations, replacements, err := planOperations(index, request.Documents, byID)
	if err != nil {
		return nil, nil, err
	}
	sources, exclusions := consumedSources(index, admitted, operations)
	plan := AdmittedPlan{
		Profile:               PlanProfile,
		SourceIdentity:        PlanSourceIdentity{Revision: index.CommitRevision, Sources: sources},
		EnvironmentPin:        PlanEnvironmentPin{Kind: PlanEnvironmentPlanOnly},
		ConfigurationSnapshot: PlanConfigurationSnapshot{Status: PlanSnapshotNotRun},
		Clauses:               make([]Clause, 0, len(admitted)),
		Operations:            operations,
		Uncertainty:           append([]string{}, planUncertainty...),
		LimitsConsumed:        PlanLimits{Clauses: len(admitted), Operations: len(operations), SourceFiles: len(sources)},
		Exclusions:            exclusions,
	}
	for _, clause := range admitted {
		plan.Clauses = append(plan.Clauses, byID[clause.ID])
		plan.LimitsConsumed.Anchors += len(clause.Anchors)
	}
	for _, source := range sources {
		plan.LimitsConsumed.SourceBytes += sourceSize(index, source.Path)
	}
	if plan.LimitsConsumed.SourceFiles > maxPlanSourceFiles || plan.LimitsConsumed.SourceBytes > maxPlanSourceBytes {
		return nil, nil, failure("plan-limit-exceeded", "consumed sources exceed %d files or %d bytes", maxPlanSourceFiles, maxPlanSourceBytes)
	}
	patch := proposalPatch(index, operations, replacements)
	plan.CandidatePatchSHA256 = sha256Hex(patch)
	encoded, err := CanonicalJSON(plan)
	if err != nil {
		return nil, nil, err
	}
	return encoded, patch, nil
}

func planOperations(index *contextindex.Index, documents []PlanDocument, clauses map[string]Clause) ([]PlanOperation, map[string][]byte, error) {
	operations := make([]PlanOperation, 0, len(documents))
	replacements := map[string][]byte{}
	seenIDs := map[string]bool{}
	for _, document := range documents {
		if !validClauseID(document.ID) {
			return nil, nil, failure("invalid-operation", "operation ID must be 1..%d bytes of UTF-8 without whitespace or controls", maxClauseIDBytes)
		}
		if seenIDs[document.ID] {
			return nil, nil, failure("duplicate-operation", "operation ID is duplicated: %s", document.ID)
		}
		seenIDs[document.ID] = true
		operation, replacement, err := planOperation(index, document, clauses)
		if err != nil {
			return nil, nil, err
		}
		if _, overlaps := replacements[operation.Target]; overlaps {
			return nil, nil, failure("overlapping-operation", "more than one operation targets %s", operation.Target)
		}
		for _, planned := range operations {
			if foldedPathOverlap(planned.Target, operation.Target) {
				return nil, nil, failure("overlapping-operation", "operation target %s overlaps %s by path prefix or letter case", operation.Target, planned.Target)
			}
		}
		replacements[operation.Target] = replacement
		operations = append(operations, operation)
	}
	sort.Slice(operations, func(left, right int) bool { return operations[left].Target < operations[right].Target })
	return operations, replacements, nil
}

func planOperation(index *contextindex.Index, document PlanDocument, clauses map[string]Clause) (PlanOperation, []byte, error) {
	target, err := planTarget(document.Target)
	if err != nil {
		return PlanOperation{}, nil, err
	}
	if !validReason(document.Reason) {
		return PlanOperation{}, nil, failure("invalid-operation", "operation %s reason must be non-empty UTF-8 of at most %d bytes without control, format, or line/paragraph separator characters", document.ID, maxPlanReasonBytes)
	}
	selected, clauseIDs, err := operationClauses(document, clauses)
	if err != nil {
		return PlanOperation{}, nil, err
	}
	prose, err := RenderAdmittedProse(index, selected)
	if err != nil {
		return PlanOperation{}, nil, err
	}
	operation := PlanOperation{ID: document.ID, Target: target, ClauseIDs: clauseIDs, Reason: document.Reason}
	original, present, err := targetPrecondition(index, target)
	if err != nil {
		return PlanOperation{}, nil, err
	}
	if !present {
		operation.Kind, operation.TargetState = OperationCreateFile, TargetAbsent
		operation.ReplacementSHA256 = sha256Hex(prose)
		return operation, prose, nil
	}
	source := index.Sources[target]
	insertion := append(insertionSeparator(original), prose...)
	operation.Kind, operation.TargetState = OperationInsertAfter, TargetPresent
	operation.TargetBlob, operation.TargetSHA256 = source.BlobHash, sha256Hex(original)
	operation.StartByte, operation.EndByte = len(original), len(original)
	operation.ReplacementSHA256 = sha256Hex(insertion)
	return operation, insertion, nil
}

// planTarget admits a Markdown path whose diff header needs no Git quoting.
func planTarget(target string) (string, error) {
	clean, err := cleanRelative(target)
	if err != nil {
		return "", err
	}
	if clean != target || !strings.HasSuffix(clean, ".md") {
		return "", failure("invalid-target", "target must be a clean repository-relative .md path")
	}
	for position := 0; position < len(clean); position++ {
		character := clean[position]
		if character <= ' ' || character > '~' || character == '"' || character == '\\' {
			return "", failure("invalid-target", "target byte 0x%02x is outside the admitted path set", character)
		}
	}
	for _, component := range strings.Split(clean, "/") {
		if strings.EqualFold(component, ".git") {
			return "", failure("invalid-target", "target %s names a Git metadata component", clean)
		}
	}
	return clean, nil
}

func validReason(reason string) bool {
	if strings.TrimSpace(reason) == "" || len(reason) > maxPlanReasonBytes || !utf8.ValidString(reason) {
		return false
	}
	return strings.IndexFunc(reason, func(r rune) bool { return unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) }) < 0
}

func operationClauses(document PlanDocument, clauses map[string]Clause) ([]Clause, []string, error) {
	if len(document.ClauseIDs) == 0 {
		return nil, nil, failure("invalid-operation", "operation %s names no clause", document.ID)
	}
	clauseIDs := append([]string{}, document.ClauseIDs...)
	sort.Strings(clauseIDs)
	selected := make([]Clause, 0, len(clauseIDs))
	for position, id := range clauseIDs {
		if position > 0 && clauseIDs[position-1] == id {
			return nil, nil, failure("invalid-operation", "operation %s repeats clause %s", document.ID, id)
		}
		clause, known := clauses[id]
		if !known {
			return nil, nil, failure("unknown-clause", "operation %s names an unadmitted clause %s", document.ID, id)
		}
		selected = append(selected, clause)
	}
	return selected, clauseIDs, nil
}

// targetPrecondition returns the pinned bytes of a present target, or proves
// absence from the tracked tree. A tracked but unpinned path never qualifies.
func targetPrecondition(index *contextindex.Index, target string) ([]byte, bool, error) {
	if source, pinned := index.Sources[target]; pinned {
		original, verified := verifiedSource(source)
		if !verified || (source.Mode != "100644" && source.Mode != "100755") {
			return nil, false, failure("target-unverified", "target %s is not a verified regular text blob", target)
		}
		if len(original) > maxPlanTargetBytes {
			return nil, false, failure("plan-limit-exceeded", "target %s exceeds %d bytes", target, maxPlanTargetBytes)
		}
		return original, true, nil
	}
	if index.Tracked == nil {
		return nil, false, failure("absence-unproven", "index carries no tracked tree to prove %s absent", target)
	}
	if _, tracked := index.Tracked[target]; tracked {
		return nil, false, failure("unpinned-target", "target %s is tracked but its bytes are not pinned", target)
	}
	if _, skipped := index.Skipped[target]; skipped {
		return nil, false, failure("unpinned-target", "target %s is a non-blob tree entry", target)
	}
	return nil, false, absentTargetConflict(index, target)
}

// absentTargetConflict refuses an untracked target whose creation would need
// a tracked file as a directory, would replace a tracked directory, or would
// alias a tracked path on a case-insensitive filesystem.
func absentTargetConflict(index *contextindex.Index, target string) error {
	for parent := target; strings.Contains(parent, "/"); {
		parent = parent[:strings.LastIndex(parent, "/")]
		if trackedPath(index, parent) {
			return failure("target-conflict", "target %s is under the tracked file %s", target, parent)
		}
	}
	for _, entries := range []map[string]struct{}{index.Tracked, index.Skipped} {
		for path := range entries {
			if foldedPathOverlap(path, target) {
				return failure("target-conflict", "target %s is a tracked directory or a case alias of a tracked path", target)
			}
		}
	}
	return nil
}

// foldedPathOverlap reports whether tracked and target share a directory
// spelled differently only by letter case, or one is a component prefix of
// the other up to letter case.
func foldedPathOverlap(tracked, target string) bool {
	for {
		trackedHead, trackedRest, trackedMore := strings.Cut(tracked, "/")
		targetHead, targetRest, targetMore := strings.Cut(target, "/")
		if !strings.EqualFold(trackedHead, targetHead) {
			return false
		}
		if trackedHead != targetHead || !trackedMore || !targetMore {
			return true
		}
		tracked, target = trackedRest, targetRest
	}
}

func trackedPath(index *contextindex.Index, path string) bool {
	_, tracked := index.Tracked[path]
	_, skipped := index.Skipped[path]
	return tracked || skipped
}

// verifiedSource returns the text of a pinned source whose bytes hash to its blob.
func verifiedSource(source contextindex.Source) ([]byte, bool) {
	text, valid, loaded := source.Text()
	if !valid || !loaded || !revisionPattern.MatchString(source.BlobHash) {
		return nil, false
	}
	data := []byte(text)
	return data, gitBlobID(data, len(source.BlobHash)) == source.BlobHash
}

func sourceSize(index *contextindex.Index, path string) int {
	data, _ := verifiedSource(index.Sources[path])
	return len(data)
}

func insertionSeparator(original []byte) []byte {
	if len(original) == 0 {
		return []byte{}
	}
	if bytes.HasSuffix(original, []byte("\n")) {
		return []byte("\n")
	}
	return []byte("\n\n")
}

// consumedSources is the source identity: every verified anchor source and
// present target, ordered by path. Unverifiable anchor sources are exclusions.
func consumedSources(index *contextindex.Index, clauses []Clause, operations []PlanOperation) ([]PlanSource, []PlanExclusion) {
	paths := map[string]bool{}
	for _, clause := range clauses {
		for _, anchor := range clause.Anchors {
			paths[anchor.Path] = true
		}
	}
	for _, operation := range operations {
		paths[operation.Target] = paths[operation.Target] || operation.TargetState == TargetPresent
	}
	sources := []PlanSource{}
	exclusions := []PlanExclusion{{Code: "edit_nav-requires-configuration-authority"}}
	for path, consumed := range paths {
		data, verified := verifiedSource(index.Sources[path])
		if verified {
			source := index.Sources[path]
			sources = append(sources, PlanSource{Path: path, Mode: source.Mode, Blob: source.BlobHash, SHA256: sha256Hex(data)})
			continue
		}
		if consumed {
			exclusions = append(exclusions, PlanExclusion{Code: unverifiedAnchorSrc, Path: path})
		}
	}
	sort.Slice(sources, func(left, right int) bool { return sources[left].Path < sources[right].Path })
	sort.Slice(exclusions, func(left, right int) bool {
		if exclusions[left].Code != exclusions[right].Code {
			return exclusions[left].Code < exclusions[right].Code
		}
		return exclusions[left].Path < exclusions[right].Path
	})
	return sources, exclusions
}

// proposalPatch renders the operations as a git-apply unified diff in plan order.
func proposalPatch(index *contextindex.Index, operations []PlanOperation, replacements map[string][]byte) []byte {
	var patch bytes.Buffer
	for _, operation := range operations {
		replacement := replacements[operation.Target]
		if operation.Kind == OperationCreateFile {
			added := splitLines(replacement)
			fmt.Fprintf(&patch, "diff --git a/%[1]s b/%[1]s\nnew file mode 100644\n--- /dev/null\n+++ b/%[1]s\n@@ -0,0 +%s @@\n", operation.Target, hunkRange(1, len(added)))
			writeLines(&patch, "+", added)
			continue
		}
		original, _ := verifiedSource(index.Sources[operation.Target])
		insertHunk(&patch, operation.Target, original, replacement)
	}
	return patch.Bytes()
}

// insertHunk emits up to three context lines before the zero-width insertion
// point. Git spells an unterminated last line as removed and re-added.
func insertHunk(patch *bytes.Buffer, target string, original, insertion []byte) {
	lines := splitLines(original)
	first := max(0, len(lines)-3)
	oldLines := lines[first:]
	newLines := splitLines(append(bytes.Join(oldLines, nil), insertion...))
	start := first + 1
	if len(oldLines) == 0 {
		start = 0
	}
	fmt.Fprintf(patch, "diff --git a/%[1]s b/%[1]s\n--- a/%[1]s\n+++ b/%[1]s\n@@ -%s +%s @@\n", target, hunkRange(start, len(oldLines)), hunkRange(max(start, 1), len(newLines)))
	if bytes.HasSuffix(original, []byte("\n")) || len(original) == 0 {
		writeLines(patch, " ", oldLines)
		writeLines(patch, "+", newLines[len(oldLines):])
		return
	}
	writeLines(patch, " ", oldLines[:len(oldLines)-1])
	fmt.Fprintf(patch, "-%s\n\\ No newline at end of file\n", oldLines[len(oldLines)-1])
	writeLines(patch, "+", newLines[len(oldLines)-1:])
}

func hunkRange(start, count int) string {
	if count == 1 {
		return fmt.Sprint(start)
	}
	return fmt.Sprintf("%d,%d", start, count)
}

// splitLines returns lines with their LF; a final LF opens no further line.
func splitLines(data []byte) [][]byte {
	lines := bytes.SplitAfter(data, []byte("\n"))
	if len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func writeLines(patch *bytes.Buffer, prefix string, lines [][]byte) {
	for _, line := range lines {
		patch.WriteString(prefix)
		patch.Write(line)
	}
}

// VerifyAdmittedPlan decodes a canonical plan strictly and refuses it unless
// recompiling its clauses and operations at index reproduces the plan and patch bytes.
func VerifyAdmittedPlan(index *contextindex.Index, raw, patch []byte) (AdmittedPlan, error) {
	if err := VerifyCanonicalJSON(raw); err != nil {
		return AdmittedPlan{}, err
	}
	var plan AdmittedPlan
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return AdmittedPlan{}, failure("plan-unknown-field", "plan is not the closed %s shape: %v", PlanProfile, err)
	}
	checks := []func(AdmittedPlan) error{planIdentityCheck, planOperationKindCheck, planUniquenessCheck, planOrderCheck}
	for _, check := range checks {
		if err := check(plan); err != nil {
			return AdmittedPlan{}, err
		}
	}
	if plan.SourceIdentity.Revision != index.CommitRevision {
		return AdmittedPlan{}, failure("stale-source", "plan revision %s is not the index revision", plan.SourceIdentity.Revision)
	}
	request := AdmittedPlanRequest{Clauses: plan.Clauses}
	for _, operation := range plan.Operations {
		request.Documents = append(request.Documents, PlanDocument{ID: operation.ID, Target: operation.Target, ClauseIDs: operation.ClauseIDs, Reason: operation.Reason})
	}
	recompiled, recompiledPatch, err := CompileAdmittedPlan(index, request)
	if err != nil {
		return AdmittedPlan{}, err
	}
	if !bytes.Equal(recompiled, raw) || !bytes.Equal(recompiledPatch, patch) {
		return AdmittedPlan{}, failure("plan-not-reproducible", "plan or proposal patch differs from its admitted recompilation")
	}
	return plan, nil
}

func planIdentityCheck(plan AdmittedPlan) error {
	if plan.Profile != PlanProfile {
		return failure("plan-profile", "plan profile must be %s", PlanProfile)
	}
	if plan.EnvironmentPin.Kind != PlanEnvironmentPlanOnly {
		return failure("plan-environment-unsupported", "only the plan-only environment variant is admitted")
	}
	if plan.ConfigurationSnapshot.Status != PlanSnapshotNotRun {
		return failure("plan-snapshot-unsupported", "a plan-only configuration snapshot is %s", PlanSnapshotNotRun)
	}
	return nil
}

func planOperationKindCheck(plan AdmittedPlan) error {
	for _, operation := range plan.Operations {
		if operation.Kind == OperationEditNav {
			return failure("nav-authority-required", "edit_nav %s needs the HDCV0-030 configuration authority snapshot", operation.ID)
		}
	}
	return nil
}

func planUniquenessCheck(plan AdmittedPlan) error {
	clauseIDs := map[string]bool{}
	for _, clause := range plan.Clauses {
		if clauseIDs[clause.ID] {
			return failure("duplicate-clause", "clause ID is duplicated: %s", clause.ID)
		}
		clauseIDs[clause.ID] = true
	}
	operationIDs := map[string]bool{}
	for _, operation := range plan.Operations {
		if operationIDs[operation.ID] {
			return failure("duplicate-operation", "operation ID is duplicated: %s", operation.ID)
		}
		operationIDs[operation.ID] = true
	}
	return nil
}

// planOrderCheck refuses any ordered set that is not strictly increasing.
func planOrderCheck(plan AdmittedPlan) error {
	sets := [][]string{nil, nil, nil}
	for _, source := range plan.SourceIdentity.Sources {
		sets[0] = append(sets[0], source.Path)
	}
	for _, clause := range plan.Clauses {
		sets[1] = append(sets[1], clause.ID)
	}
	for _, operation := range plan.Operations {
		sets[2] = append(sets[2], operation.Target)
		sets = append(sets, operation.ClauseIDs)
	}
	for _, set := range sets {
		if !strictlyIncreasing(set) {
			return failure("unordered-set", "plan carries an unordered or repeated path or ID set")
		}
	}
	return nil
}

func strictlyIncreasing(values []string) bool {
	for position := 1; position < len(values); position++ {
		if values[position-1] >= values[position] {
			return false
		}
	}
	return true
}

// StaleOperations reports, in plan order, the operations whose precondition no
// longer holds at index (HDCV0-029). The plan is retained and never retargeted.
func StaleOperations(index *contextindex.Index, plan AdmittedPlan) []string {
	stale := []string{}
	for _, operation := range plan.Operations {
		if !operationFresh(index, operation) {
			stale = append(stale, operation.ID)
		}
	}
	return stale
}

func operationFresh(index *contextindex.Index, operation PlanOperation) bool {
	source, pinned := index.Sources[operation.Target]
	if operation.TargetState == TargetAbsent {
		// The same proof compilation needs: absent, not skipped, and no tracked parent or child.
		_, present, err := targetPrecondition(index, operation.Target)
		return err == nil && !present
	}
	data, verified := verifiedSource(source)
	return pinned && verified && source.BlobHash == operation.TargetBlob && sha256Hex(data) == operation.TargetSHA256
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
