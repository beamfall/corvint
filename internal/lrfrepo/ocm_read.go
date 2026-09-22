package lrfrepo

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

const DefaultOCMReportPath = ".git/corvint/ocm-review.md"

// OCMReadOptions are the authority and policy inputs shared by status, verify,
// and report.
type OCMReadOptions struct {
	OCMPath           string
	CEMPath           string
	ExpectedBase      string
	Target            string
	ExpectedBaseGiven bool
	TargetGiven       bool
	MaxUnknown        *int
}

// OCMReadResult is the transport-neutral read verdict. Command-specific
// envelopes add only their tool name and path fields.
type OCMReadResult struct {
	State         string
	Counts        map[string]any
	Worklist      []any
	PolicyIssues  []any
	Verification  map[string]any
	TestExecution map[string]any
	OCMAbsolute   string
	CEMAbsolute   string
	// OCMRaw is the exact OCM map bytes this verdict was computed from. A
	// caller that needs to bind a digest or re-parse the map to these bytes
	// (rather than re-reading the path, which can observe a different file
	// if it changes between reads) MUST use this field.
	OCMRaw []byte `json:"-"`
	// CEMRaw is the exact CEM map bytes used with OCMRaw for this verdict.
	CEMRaw []byte `json:"-"`
	// VerificationCause retains the failed check before standalone wire redaction.
	VerificationCause error `json:"-"`
}

// ReadOCM verifies one OCM through the same parser and repository authority
// used by LRF. Semantic failures are returned as verdicts; only unsupported
// profiles and operational failures return an error.
func ReadOCM(ctx context.Context, root string, options OCMReadOptions) (*OCMReadResult, error) {
	inputRoot, err := publish.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	ocmRaw, err := readRootInput(inputRoot, options.OCMPath, maxOCMBytes, cemcode.MapUnavailable)
	if err != nil {
		return nil, ocmInputFailure(inputRoot.Path(), options.OCMPath, err, "OCM map", maxOCMBytes)
	}
	cemRaw, err := readRootInput(inputRoot, options.CEMPath, wire.MaxMapBytes, cemcode.MapUnavailable)
	if err != nil {
		return nil, ocmInputFailure(inputRoot.Path(), options.CEMPath, err, "CEM map", wire.MaxMapBytes)
	}
	return readOCMBytes(ctx, inputRoot, ocmRaw, cemRaw, options)
}

func readOCMBytes(ctx context.Context, inputRoot *publish.Root, ocmRaw, cemRaw []byte, options OCMReadOptions) (*OCMReadResult, error) {
	result := &OCMReadResult{
		Counts:        emptyOCMCounts(),
		Worklist:      []any{},
		PolicyIssues:  []any{},
		TestExecution: map[string]any{"state": "NOT_RUN"},
		OCMAbsolute:   absoluteInput(inputRoot.Path(), options.OCMPath),
		CEMAbsolute:   absoluteInput(inputRoot.Path(), options.CEMPath),
		OCMRaw:        ocmRaw,
		CEMRaw:        cemRaw,
	}
	rootValue, err := wire.Parse(ocmRaw)
	if err != nil || rootValue.Kind != wire.KindObject {
		return nil, fail("unsupported-ocm-json-profile", "native Go OCM does not approximate this JSON failure profile")
	}
	result.Counts, result.Worklist = ocmCountsAndWorklist(rootValue)
	if !bytes.Equal(ocmRaw, append(canonicalValue(rootValue), '\n')) {
		return finishOCMFailure(result, "noncanonical-map", "OCM bytes are not canonical"), nil
	}
	cem, err := wire.ParseMap(cemRaw)
	if err != nil {
		return finishOCMFailure(result, "invalid-cem", "CEM is invalid"), nil
	}
	if cem.Spec == wire.Spec02 {
		if !options.ExpectedBaseGiven {
			return finishOCMFailure(result, cemcode.ExpectedBaseRequired, "cem/0.2 requires expected base"), nil
		}
		if !options.TargetGiven {
			return finishOCMFailure(result, cemcode.TargetRequired, "cem/0.2 requires caller target"), nil
		}
	}
	document, err := parseOCMHeader(rootValue)
	if err != nil {
		return finishOCMError(result, err), nil
	}
	if sha256Hex(cemRaw) != document.cemMap {
		return finishOCMFailure(result, "cem-map-digest-mismatch", "CEM map digest does not match"), nil
	}
	if cem.Spec == wire.Spec02 && options.Target == "" {
		return finishOCMFailure(result, "invalid-target-revision", "target must be a full commit OID"), nil
	}
	repository, err := gitauth.Open(inputRoot.Path(), gitrun.NewDefaultBudget())
	if err != nil {
		result.VerificationCause = err
		return finishOCMError(result, mapOCMRepositoryError(err)), nil
	}
	defer repository.BeginObjectSession()()
	if cem.Spec == wire.Spec02 && options.ExpectedBase == "" {
		return finishOCMFailure(result, "invalid-cem", "CEM verification failed"), nil
	}
	verified, err := verifyReadCEM(ctx, repository, cem, cemRaw, document, options)
	if err != nil {
		result.VerificationCause = err
		return finishOCMError(result, mapOCMCEMError(err)), nil
	}
	verificationOptions := Options{
		ExpectedBase: options.ExpectedBase,
		Target:       options.Target,
	}
	if err := verifyOCMBinding(ctx, repository, cem, cemRaw, verified, document, verificationOptions); err != nil {
		return finishOCMError(result, err), nil
	}
	if err := parseOCMIntent(rootValue, document); err != nil {
		return finishOCMError(result, err), nil
	}
	intentContext, err := verifyOCMIntent(ctx, repository, cem, document)
	if err != nil {
		return finishOCMError(result, err), nil
	}
	if err := parseOCMClosure(rootValue, document); err != nil {
		return nil, fail("unsupported-ocm-closure-profile", "native Go OCM does not approximate this closure validation profile")
	}
	if _, err := verifyOCMClosure(cem, document, ocmRaw, intentContext, true); err != nil {
		if CodeOf(err) == "unsupported-ocm-python-claims" {
			return nil, err
		}
		return finishOCMError(result, err), nil
	}
	return finishOCMSuccess(result, document, options.MaxUnknown), nil
}

func ocmInputFailure(root, requested string, err error, label string, limit int) error {
	message := ocmMessage(err)
	switch {
	case inputIsSymlink(root, requested):
		message = "cannot read " + label
	case strings.Contains(message, "not a regular file"):
		message = label + " is not a regular file"
	case strings.Contains(message, "exceeds"):
		message = label + " exceeds the " + strconv.Itoa(limit) + "-byte limit"
	case strings.Contains(message, "changed") || strings.Contains(message, "read completely"):
		message = label + " changed while being read"
	default:
		message = "cannot read " + label
	}
	return fail("ocm-cli-error", "%s", message)
}

func inputIsSymlink(root, requested string) bool {
	info, err := os.Lstat(absoluteInput(root, requested))
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

func verifyReadCEM(ctx context.Context, repository *gitauth.Repository, cem *wire.Map, cemRaw []byte, document *ocmDocument, options OCMReadOptions) (*verifiedCEM, error) {
	if cem.Spec == wire.Spec02 {
		outcome, _, err := verify.Canonical(ctx, repository, cem, verify.CanonicalOptions{
			ExpectedBase: options.ExpectedBase,
			Target:       options.Target,
			RawMapBytes:  cemRaw,
		})
		if err != nil {
			return nil, err
		}
		patchBytes, err := repository.CanonicalDiff(ctx, outcome.BaseRevision, outcome.TargetRevision)
		if err != nil {
			return nil, err
		}
		excluded := wire.ExcludedCEMPath
		return &verifiedCEM{outcome.BaseRevision, outcome.TargetRevision, patchBytes, "canonical-derived", &excluded}, nil
	}
	if options.Target != "" {
		resolved, err := repository.Resolve(ctx, options.Target)
		if err != nil {
			return nil, err
		}
		if resolved != document.target {
			return nil, fail("target-mismatch", "caller target differs from OCM target")
		}
	}
	patchBytes, err := repository.CanonicalDiff(ctx, cem.BaseRevision, document.target)
	if err != nil {
		return nil, err
	}
	outcome, err := verify.Exact(ctx, repository, cem, patchBytes, verify.ExactOptions{
		ExpectedBase: options.ExpectedBase,
		Target:       document.target,
	})
	if err != nil {
		return nil, err
	}
	return &verifiedCEM{outcome.BaseRevision, document.target, patchBytes, "canonical-derived", nil}, nil
}

func finishOCMSuccess(result *OCMReadResult, document *ocmDocument, maxUnknown *int) *OCMReadResult {
	total := len(document.obligations)
	linked := result.Counts["linked"].(int)
	unknown := result.Counts["unknown"].(int)
	result.Verification = map[string]any{
		"valid": true, "spec": ocmSpec, "targetRevision": document.target,
		"structuralCoverage": map[string]any{"total": total, "linked": linked, "unknown": unknown},
		"testExecution":      map[string]any{"state": "NOT_RUN"},
		"issues":             []any{},
		"policyIssues":       []any{},
	}
	result.State = "ready-for-review"
	if maxUnknown != nil && unknown > *maxUnknown {
		issue := map[string]any{
			"code":    "max-unknown-exceeded",
			"message": "unknown obligation count exceeds policy maximum",
		}
		result.PolicyIssues = []any{issue}
		result.Verification["valid"] = false
		result.Verification["policyIssues"] = result.PolicyIssues
		result.State = "incomplete"
	}
	return result
}

func finishOCMError(result *OCMReadResult, err error) *OCMReadResult {
	return finishOCMFailure(result, CodeOf(err), ocmMessage(err))
}

func finishOCMFailure(result *OCMReadResult, code, message string) *OCMReadResult {
	if code == "" {
		code = "invalid-map"
	}
	if result.VerificationCause == nil {
		result.VerificationCause = fail(code, "%s", message)
	}
	issue := map[string]any{"code": code}
	if message != "" {
		issue["message"] = message
	}
	if code == "invalid-path" || code == cemcode.RepositoryObjectUnavailable {
		result.Verification = map[string]any{"valid": false, "issues": []any{map[string]any{"code": code}}}
		result.TestExecution = map[string]any{"observed": false, "state": "NOT_RUN"}
	} else {
		result.Verification = map[string]any{
			"valid": false, "spec": ocmSpec, "targetRevision": "",
			"structuralCoverage": map[string]any{"total": 0, "linked": 0, "unknown": 0},
			"testExecution":      map[string]any{"state": "NOT_RUN"},
			"issues":             []any{issue},
			"policyIssues":       []any{},
		}
	}
	result.State = "invalid"
	result.PolicyIssues = []any{}
	return result
}

func mapOCMCEMError(err error) error {
	code := CodeOf(err)
	switch code {
	case cemcode.BaseRevisionMismatch, cemcode.ExcludedArtifactMismatch,
		cemcode.ExcludedPathNotFile, cemcode.ExpectedBaseRequired,
		cemcode.TargetRequired, cemcode.UnsupportedObjectAlternates,
		"target-mismatch":
		return err
	case cemcode.RepositoryObjectUnavailable, cemcode.GitCancelled,
		cemcode.GitTimeout, cemcode.GitOutputExceeded, cemcode.GitBudgetExceeded,
		cemcode.GitStartFailed, cemcode.GitExitFailure, cemcode.GitDiffFailed,
		cemcode.GitDiffTimeout:
		return fail(cemcode.RepositoryObjectUnavailable, "repository object unavailable")
	default:
		return fail("invalid-cem", "CEM verification failed")
	}
}

func mapOCMRepositoryError(err error) error {
	if CodeOf(err) == cemcode.UnsupportedObjectAlternates {
		return err
	}
	return fail(cemcode.RepositoryObjectUnavailable, "repository object unavailable")
}

func ocmMessage(err error) string {
	var adapter *Error
	if errors.As(err, &adapter) {
		return adapter.Message
	}
	var registered *cemcode.Error
	if errors.As(err, &registered) {
		return registered.Message
	}
	return err.Error()
}

func emptyOCMCounts() map[string]any {
	return map[string]any{"total": 0, "linked": 0, "unknown": 0}
}

func ocmCountsAndWorklist(root wire.Value) (map[string]any, []any) {
	counts := emptyOCMCounts()
	work := []any{}
	obligations, found := root.Obj.Get("obligations")
	if !found || obligations.Kind != wire.KindArray || len(obligations.Arr) > maxObligations {
		return counts, work
	}
	for index, item := range obligations.Arr {
		if item.Kind != wire.KindObject {
			return emptyOCMCounts(), []any{}
		}
		fields := []string{"id", "disposition", "reason", "hunkIds", "claimIds"}
		row := map[string]any{"selector": index + 1}
		for _, field := range fields {
			value, present := item.Obj.Get(field)
			if !present {
				return emptyOCMCounts(), []any{}
			}
			converted, ok := ocmJSONValue(value)
			if !ok {
				return emptyOCMCounts(), []any{}
			}
			row[field] = converted
		}
		disposition, _ := row["disposition"].(string)
		row["next"] = "link-or-review-unknown"
		if disposition == "linked" {
			row["next"] = "done"
		}
		counts["total"] = counts["total"].(int) + 1
		if current, ok := counts[disposition].(int); ok {
			counts[disposition] = current + 1
		}
		work = append(work, row)
	}
	return counts, work
}

func ocmJSONValue(value wire.Value) (any, bool) {
	switch value.Kind {
	case wire.KindString:
		return value.Str, true
	case wire.KindArray:
		result := make([]any, 0, len(value.Arr))
		for _, item := range value.Arr {
			converted, ok := ocmJSONValue(item)
			if !ok {
				return nil, false
			}
			result = append(result, converted)
		}
		return result, true
	default:
		return nil, false
	}
}

func absoluteInput(root, requested string) string {
	if filepath.IsAbs(requested) {
		return filepath.Clean(requested)
	}
	return filepath.Join(root, filepath.FromSlash(requested))
}

// PublishOCMReport resolves and optionally publishes one bounded report. A
// false publish flag performs every path-safety check without writing.
func PublishOCMReport(ctx context.Context, root, requested string, data []byte, publishReport bool) (string, error) {
	if requested == "" || requested == "." {
		return "", fail("ocm-cli-error", "output path must be normalized")
	}
	if filepath.IsAbs(requested) {
		return "", fail("unsupported-ocm-output-path", "native Go OCM report output must remain inside the repository root")
	}
	if err := wire.ValidatePath(requested); err != nil {
		return "", fail("unsupported-ocm-output-path", "native Go OCM report output must be normalized and repository-relative")
	}
	if requested == DefaultOCMReportPath {
		repository, err := gitauth.Open(root, gitrun.NewDefaultBudget())
		if err != nil {
			return "", err
		}
		outputRoot, err := publish.OpenRoot(repository.GitDir)
		if err != nil {
			return "", err
		}
		return publishOCMPath(ctx, outputRoot, "corvint/ocm-review.md", data, publishReport, true)
	}
	for _, segment := range strings.Split(requested, "/") {
		if strings.EqualFold(segment, ".git") {
			return "", fail("unsupported-ocm-output-path", "native Go OCM report output must not name Git metadata")
		}
	}
	outputRoot, err := publish.OpenRoot(root)
	if err != nil {
		return "", err
	}
	return publishOCMPath(ctx, outputRoot, requested, data, publishReport, false)
}

func publishOCMPath(ctx context.Context, root *publish.Root, relative string, data []byte, publishReport, createParent bool) (string, error) {
	full := filepath.Join(root.Path(), filepath.FromSlash(relative))
	segments := strings.Split(relative, "/")
	current := root.Path()
	for _, segment := range segments[:len(segments)-1] {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) && createParent {
			continue
		}
		if err != nil {
			return "", fail("unsafe-output", "output parent does not exist")
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", fail("unsafe-output", "output parent is not a real directory")
		}
	}
	if info, err := os.Lstat(full); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return "", fail("unsafe-output", "report output is not a regular file")
	}
	if publishReport {
		if err := publish.PublishSecure(ctx, publish.Output{Root: root, Relative: relative, Data: data}, createParent); err != nil {
			return "", err
		}
	}
	return full, nil
}

// sameInputFile reports whether two root-relative or absolute inputs name one
// file: equal after resolution, or, when both exist, the same inode, so an
// absolute or case-folded alias of the CEM path cannot become the OCM output.
func sameInputFile(root, left, right string) bool {
	leftPath := absoluteInput(root, left)
	rightPath := absoluteInput(root, right)
	if leftPath == rightPath {
		return true
	}
	leftInfo, err := os.Stat(leftPath)
	if err != nil {
		return false
	}
	rightInfo, err := os.Stat(rightPath)
	if err != nil {
		return false
	}
	return os.SameFile(leftInfo, rightInfo)
}

// unreadableExistingMap lets prepare treat its OCM map as absent only when no
// file exists there. An existing regular map it cannot read, such as one over
// the size limit, is refused unless replaced; a symlink, non-regular file or
// unsupported path is left for the publisher to refuse.
func unreadableExistingMap(root string, options PrepareOptions, err error) error {
	info, statErr := os.Lstat(absoluteInput(root, options.MapPath))
	if statErr != nil {
		return nil
	}
	if options.Replace {
		return nil
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	if filepath.IsAbs(options.MapPath) {
		return nil
	}
	if wire.ValidatePath(options.MapPath) != nil {
		return nil
	}
	return ocmInputFailure(root, options.MapPath, err, "OCM map", maxOCMBytes)
}
