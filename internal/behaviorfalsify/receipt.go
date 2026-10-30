package behaviorfalsify

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

const ReceiptSchema = "corvint-browser-behavior-evidence/1"
const ApprovalSchema = "corvint-browser-behavior-approval/1"

// ToolIdentity identifies the executable that observed execution, not an authenticated signer.
type ToolIdentity struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Revision    string `json:"revision"`
	Executable  string `json:"executable_sha256"`
	SourceDirty bool   `json:"source_dirty"`
}

type Approval struct {
	Schema     string `json:"schema"`
	PlanDigest string `json:"plan_digest"`
	Method     string `json:"method"`
	Digest     string `json:"digest"`
}

// RawAttempt retains exact bytes, encoded as base64 in JSON; no reserialization is hashed.
type RawAttempt struct {
	ControlID    string   `json:"control_id"`
	Ordinal      int      `json:"ordinal"`
	Attempt      int      `json:"attempt"`
	HookBytes    []byte   `json:"hook_bytes"`
	HookSHA256   string   `json:"hook_sha256"`
	NativeBytes  []byte   `json:"native_bytes"`
	NativeSHA256 string   `json:"native_sha256"`
	Omissions    []string `json:"omissions"`
	// IsolationResetBytes and IsolationVerifyBytes are the shared-server profile's phase stdout;
	// each recorded identity must re-derive from them (BBF-V0-022). Absent for other profiles.
	IsolationResetBytes  []byte `json:"isolation_reset_bytes,omitempty"`
	IsolationVerifyBytes []byte `json:"isolation_verify_bytes,omitempty"`
}

type EvidenceReceipt struct {
	Schema       string       `json:"schema"`
	PlanPreimage []byte       `json:"plan_preimage"`
	Approval     Approval     `json:"approval"`
	Tool         ToolIdentity `json:"tool"`
	RawAttempts  []RawAttempt `json:"raw_attempts"`
	Report       Report       `json:"report"`
	Unsupported  []string     `json:"unsupported"`
	Digest       string       `json:"digest"`
}

var receiptUnsupported = []string{
	"authenticated-operator-identity",
	"authenticated-hook-semantics",
	"independent-process-and-artifact-observation",
}

// ExecuteReceipt runs the same approved executor and retains an offline-verifiable observation.
// A native report must be declared as a hook artifact before cleanup removes it.
func ExecuteReceipt(ctx context.Context, plan Plan, approved string, tool ToolIdentity) (EvidenceReceipt, error) {
	if !validToolIdentity(tool) {
		return EvidenceReceipt{}, errors.New("receipt-tool-identity-invalid")
	}
	if plan.Request.Provider != nil {
		// The runner-neutral profile retains no native report to bind yet (BBF-V0-028).
		return EvidenceReceipt{}, errors.New("receipt-profile-unsupported")
	}
	preimage := plan
	preimage.Digest = ""
	planBytes, err := marshalJSON(preimage)
	if err != nil || !safeReceiptBytes(planBytes) {
		return EvidenceReceipt{}, errors.New("receipt-plan-unsafe")
	}
	r := EvidenceReceipt{Schema: ReceiptSchema, PlanPreimage: planBytes, Tool: tool, RawAttempts: []RawAttempt{}, Unsupported: slices.Clone(receiptUnsupported)}
	r.Approval = Approval{Schema: ApprovalSchema, PlanDigest: approved, Method: "explicit-approve-plan"}
	r.Approval.Digest = approvalDigest(r.Approval)
	retainedBytes := len(planBytes)
	unsafe := false
	r.Report, err = execute(ctx, plan, approved, func(control PlannedControl, attempt AttemptResult, raw []byte) {
		if attempt.HookProcess.StdoutSHA256 == "" {
			// No hook stdout was observed, so there is nothing to retain (BBF-V0-018).
			return
		}
		entry := RawAttempt{ControlID: control.ID, Ordinal: control.Ordinal, Attempt: attempt.Attempt, HookSHA256: digestBytes(raw), Omissions: []string{}}
		if !safeReceiptBytes(raw) {
			unsafe = true
		} else if retainedBytes+len(raw) > maxDocumentBytes/2 {
			entry.Omissions = append(entry.Omissions, "retained-byte-bound")
		} else {
			entry.HookBytes = bytes.Clone(raw)
			retainedBytes += len(raw)
		}
		// Only an admitted hook receipt declares a native report; an attempt without one retains
		// its stdout alone and keeps its executor classification (BBF-V0-018).
		if hook := attempt.Receipt; hook != nil {
			for _, artifact := range hook.Artifacts {
				if artifact.SHA256 != hook.NativeReceiptSHA256 {
					continue
				}
				data, readErr := readNativeArtifact(plan.Request.DisposableRoot, artifact)
				if readErr != nil {
					continue
				}
				if !safeReceiptBytes(data) {
					unsafe = true
					break
				}
				if retainedBytes+len(data) <= maxDocumentBytes/2 {
					entry.NativeBytes = data
					entry.NativeSHA256 = digestBytes(data)
					retainedBytes += len(data)
				}
				break
			}
		}
		if attempt.Receipt != nil && entry.NativeBytes == nil {
			entry.Omissions = append(entry.Omissions, "native-report-unavailable")
		}
		r.RawAttempts = append(r.RawAttempts, entry)
	})
	if err != nil {
		return EvidenceReceipt{}, err
	}
	retainIsolationBytes(&r, &retainedBytes, &unsafe)
	if unsafe {
		return EvidenceReceipt{}, errors.New("receipt-secret-shaped-data")
	}
	// A valid /0 kill cannot become an evidence-profile kill without its retained reports.
	for i := range r.Report.Results {
		control := &r.Report.Results[i]
		for j := range control.Attempts {
			a := &control.Attempts[j]
			for _, raw := range r.RawAttempts {
				if raw.Ordinal == control.Ordinal && raw.Attempt == a.Attempt && len(raw.Omissions) != 0 {
					a.Reasons = uniqueReasons(append(a.Reasons, raw.Omissions...))
					a.Status = StatusInvalidControl
					a.Application = ""
				}
			}
		}
		if len(control.Attempts) != 0 {
			control.Status, control.Reasons = aggregateAttempts(control.Attempts, plan.Request.Attempts)
			control.Application = aggregateApplication(plan.Controls[i], control.Attempts, control.Status)
		}
	}
	recountReport(&r.Report, plan)
	r.Report.Digest = digestJSON(r.Report)
	r.Digest = evidenceDigest(r)
	if _, err = EncodeEvidence(r); err != nil {
		return EvidenceReceipt{}, err
	}
	if err := refuseAuthenticationMaterial(plan, r); err != nil {
		return EvidenceReceipt{}, err
	}
	return r, nil
}

// retainIsolationBytes moves each attempt's in-memory isolation phase stdout onto its retained
// raw attempt, so an offline verifier can re-derive the recorded identities (BBF-V0-022).
func retainIsolationBytes(r *EvidenceReceipt, retainedBytes *int, unsafe *bool) {
	for i := range r.Report.Results {
		control := &r.Report.Results[i]
		for j := range control.Attempts {
			a := &control.Attempts[j]
			if a.Isolation == nil {
				continue
			}
			observed := *a.Isolation
			a.Isolation = &IsolationObservation{ResetProcess: observed.ResetProcess, Before: observed.Before, VerifyProcess: observed.VerifyProcess, After: observed.After}
			var entry *RawAttempt
			for k := range r.RawAttempts {
				if r.RawAttempts[k].Ordinal == control.Ordinal && r.RawAttempts[k].Attempt == a.Attempt {
					entry = &r.RawAttempts[k]
				}
			}
			if entry == nil {
				// No hook stdout was retained, so the attempt cannot score (BBF-V0-018).
				continue
			}
			for _, phase := range []struct {
				retained *[]byte
				data     []byte
			}{{&entry.IsolationResetBytes, observed.resetRaw}, {&entry.IsolationVerifyBytes, observed.verifyRaw}} {
				switch {
				case len(phase.data) == 0:
				case !safeReceiptBytes(phase.data):
					*unsafe = true
				case *retainedBytes+len(phase.data) > maxDocumentBytes/2:
					if !slices.Contains(entry.Omissions, "retained-byte-bound") {
						entry.Omissions = append(entry.Omissions, "retained-byte-bound")
					}
				default:
					*phase.retained = bytes.Clone(phase.data)
					*retainedBytes += len(phase.data)
				}
			}
		}
	}
}

// rawIsolationVerifies checks that a retained attempt's isolation identities re-derive from its
// retained phase stdout and that the stdout matches the recorded process digests (BBF-V0-022).
func rawIsolationVerifies(plan Plan, control PlannedControl, a AttemptResult, raw RawAttempt) bool {
	observed := a.Isolation
	if plan.Request.Isolation == nil || observed == nil {
		return plan.Request.Isolation == nil && observed == nil && len(raw.IsolationResetBytes) == 0 && len(raw.IsolationVerifyBytes) == 0
	}
	if !isolationBytesVerify(plan, control.ID, a.Attempt, isolationPhaseReset, observed.ResetProcess, raw.IsolationResetBytes, observed.Before) {
		return false
	}
	if observed.VerifyProcess == nil {
		return len(raw.IsolationVerifyBytes) == 0 && observed.After == nil
	}
	return isolationBytesVerify(plan, control.ID, a.Attempt, isolationPhaseVerify, *observed.VerifyProcess, raw.IsolationVerifyBytes, observed.After)
}

func readNativeArtifact(root string, a Artifact) ([]byte, error) {
	if len(a.Path) > 4096 || filepath.IsAbs(a.Path) || filepath.Clean(a.Path) != a.Path || a.Path == ".." || strings.HasPrefix(a.Path, ".."+string(filepath.Separator)) {
		return nil, errors.New("receipt-native-path-invalid")
	}
	path, err := containedRegularFile(root, a.Path)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxDocumentBytes/2+1))
	if err != nil || len(data) > maxDocumentBytes/2 || digestBytes(data) != a.SHA256 {
		return nil, errors.New("receipt-native-report-invalid")
	}
	return data, nil
}

func validToolIdentity(t ToolIdentity) bool {
	return t.Name == "corvint-behavior-falsify" && t.Version != "" && len(t.Version) <= 128 &&
		gitOIDPattern.MatchString(t.Revision) && digestPattern.MatchString(t.Executable)
}

func approvalDigest(a Approval) string        { a.Digest = ""; return digestJSON(a) }
func evidenceDigest(r EvidenceReceipt) string { r.Digest = ""; return digestJSON(r) }

func safeReceiptBytes(raw []byte) bool {
	if secretscreen.MatchString(string(raw)) {
		return false
	}
	var value any
	if json.Unmarshal(raw, &value) == nil {
		var safe func(any) bool
		safe = func(v any) bool {
			switch x := v.(type) {
			case string:
				return !secretscreen.MatchString(x)
			case []any:
				for _, item := range x {
					if !safe(item) {
						return false
					}
				}
			case map[string]any:
				for key, item := range x {
					if !safe(item) {
						return false
					}
					if text, ok := item.(string); ok && secretscreen.MatchString(key+"="+text) {
						return false
					}
				}
			}
			return true
		}
		return safe(value)
	}
	return true
}

func EncodeEvidence(r EvidenceReceipt) ([]byte, error) {
	// Encoded byte fields alone form a lower bound on the final document. Keep
	// arithmetic below the document cap, then let Encode count JSON overhead.
	remaining := maxDocumentBytes
	fits := func(data []byte) bool {
		if len(data) > remaining/4*3 {
			return false
		}
		remaining -= base64.StdEncoding.EncodedLen(len(data))
		return true
	}
	if !fits(r.PlanPreimage) {
		return nil, errors.New("document exceeds bound")
	}
	for _, a := range r.RawAttempts {
		if !fits(a.HookBytes) || !fits(a.NativeBytes) || !fits(a.IsolationResetBytes) || !fits(a.IsolationVerifyBytes) {
			return nil, errors.New("document exceeds bound")
		}
	}
	raw, err := Encode(r)
	if err != nil {
		return nil, err
	}
	if !safeReceiptBytes(r.PlanPreimage) {
		return nil, errors.New("receipt-secret-shaped-data")
	}
	for _, a := range r.RawAttempts {
		if !safeReceiptBytes(a.HookBytes) || !safeReceiptBytes(a.NativeBytes) || !safeReceiptBytes(a.IsolationResetBytes) || !safeReceiptBytes(a.IsolationVerifyBytes) {
			return nil, errors.New("receipt-secret-shaped-data")
		}
	}
	if !safeReceiptBytes(raw) {
		return nil, errors.New("receipt-secret-shaped-data")
	}
	return raw, err
}

// VerifyReceipt checks retained bytes against caller-pinned identities without accessing a
// workspace or running hooks. It validates observations; it does not authenticate their author.
func VerifyReceipt(r EvidenceReceipt, expectedPlan, expectedTool string) error {
	invalid := func() error { return errors.New("receipt-invalid-control") }
	if r.Schema != ReceiptSchema || !digestPattern.MatchString(expectedPlan) ||
		!validToolIdentity(r.Tool) || r.Tool.Executable != expectedTool ||
		!slices.Equal(r.Unsupported, receiptUnsupported) || r.Digest != evidenceDigest(r) {
		return invalid()
	}
	if _, err := EncodeEvidence(r); err != nil {
		return invalid()
	}
	var plan Plan
	if Decode(r.PlanPreimage, &plan) != nil || plan.Digest != "" || plan.Schema != PlanSchema || digestBytes(r.PlanPreimage) != expectedPlan {
		return invalid()
	}
	canonical, err := marshalJSON(plan)
	if err != nil || !bytes.Equal(canonical, r.PlanPreimage) {
		return invalid()
	}
	plan.Digest = expectedPlan
	if len(plan.Controls) == 0 || len(plan.Controls) > 64 || plan.Request.Attempts < 1 || plan.Request.Attempts > 16 ||
		r.Approval.Schema != ApprovalSchema || r.Approval.Method != "explicit-approve-plan" || r.Approval.PlanDigest != expectedPlan || r.Approval.Digest != approvalDigest(r.Approval) ||
		r.Report.Schema != ReportSchema || r.Report.PlanDigest != expectedPlan || r.Report.Digest != digestJSON(r.Report) || len(r.Report.Results) != len(plan.Controls) {
		return invalid()
	}
	// The report must record exactly the plan's declared isolation (BBF-V0-022); the
	// runner-neutral profile has no evidence receipt (BBF-V0-028).
	if plan.Request.Provider != nil || r.Report.Provider != "" || !reflect.DeepEqual(r.Report.Isolation, plan.Request.Isolation) || !reflect.DeepEqual(r.Report.Authentication, plan.Request.Authentication) {
		return invalid()
	}
	rawIndex := 0
	for i, control := range plan.Controls {
		result := r.Report.Results[i]
		if control.Ordinal != i+1 || result.Ordinal != control.Ordinal || result.ID != control.ID || result.Kind != control.Kind || len(result.Attempts) > plan.Request.Attempts {
			return invalid()
		}
		for j, a := range result.Attempts {
			if a.Attempt != j+1 {
				return invalid()
			}
			if a.HookProcess.StdoutSHA256 == "" {
				if !unobservedAttemptVerifies(a) {
					return invalid()
				}
				continue
			}
			if rawIndex >= len(r.RawAttempts) {
				return invalid()
			}
			raw := r.RawAttempts[rawIndex]
			rawIndex++
			if raw.Ordinal != control.Ordinal || raw.ControlID != control.ID || raw.Attempt != a.Attempt || len(raw.Omissions) != 0 ||
				raw.HookSHA256 != digestBytes(raw.HookBytes) || raw.HookSHA256 != a.HookProcess.StdoutSHA256 {
				return invalid()
			}
			if !rawIsolationVerifies(plan, control, a, raw) {
				return invalid()
			}
			if a.Receipt == nil {
				if !unadmittedAttemptVerifies(plan, control, a, raw) {
					return invalid()
				}
				continue
			}
			if raw.NativeSHA256 != digestBytes(raw.NativeBytes) || raw.NativeBytes == nil {
				return invalid()
			}
			var hook HookReceipt
			if Decode(raw.HookBytes, &hook) != nil || !reflect.DeepEqual(a.Receipt, &hook) || hook.NativeReceiptSHA256 != raw.NativeSHA256 {
				return invalid()
			}
			nativeDeclared := false
			if !reflect.DeepEqual(hook.Artifacts, a.RetainedArtifacts) {
				return invalid()
			}
			for _, artifact := range a.RetainedArtifacts {
				if artifact.SHA256 == raw.NativeSHA256 {
					nativeDeclared = true
				}
			}
			if !nativeDeclared || a.WorkspaceBefore != plan.WorkspaceSHA256 {
				return invalid()
			}
			candidate := a
			candidate.Reasons = nil
			status, reasons, application := classifyAttempt(plan, control, candidate)
			if status != a.Status || !slices.Equal(reasons, a.Reasons) || application != a.Application {
				return invalid()
			}
		}
		switch control.Disposition {
		case "not_supported":
			if len(result.Attempts) != 0 || result.Status != StatusNotSupported {
				return invalid()
			}
		case "not_run":
			if len(result.Attempts) != 0 || result.Status != StatusNotRun {
				return invalid()
			}
		case "run":
			if len(result.Attempts) == 0 {
				if result.Status != StatusNotRun {
					return invalid()
				}
			} else {
				status, reasons := aggregateAttempts(result.Attempts, plan.Request.Attempts)
				if status != result.Status || !slices.Equal(reasons, result.Reasons) {
					return invalid()
				}
			}
		default:
			return invalid()
		}
		if result.Application != aggregateApplication(control, result.Attempts, result.Status) {
			return invalid()
		}
	}
	if rawIndex != len(r.RawAttempts) {
		return invalid()
	}
	want := r.Report
	recountReport(&want, plan)
	want.CompleteVocabulary = completeVocabulary(plan.Controls)
	if !reflect.DeepEqual(want.Counts, r.Report.Counts) || !slices.Equal(want.CoverageGaps, r.Report.CoverageGaps) || !slices.Equal(want.InertControls, r.Report.InertControls) || want.MutationScore != r.Report.MutationScore || want.Requested != r.Report.Requested || want.Executed != r.Report.Executed || want.Supported != r.Report.Supported || want.CompleteVocabulary != r.Report.CompleteVocabulary || r.Report.Fallback != "full-relevant-suite" {
		return invalid()
	}
	return nil
}

// unobservedAttemptVerifies checks an attempt with no observed hook stdout (BBF-V0-018). It
// carries no receipt, artifacts, application or started hook, and its status is one of the
// executor's non-scoring outcomes, so no status value, including one outside the vocabulary, can
// aggregate into a kill or a survivor.
func unobservedAttemptVerifies(a AttemptResult) bool {
	if a.Receipt != nil || len(a.RetainedArtifacts) != 0 || a.HookProcess.Started || a.Application != "" {
		return false
	}
	return a.Status == StatusInfrastructureFailed || a.Status == StatusInvalidControl || a.Status == StatusNotRun
}

// unadmittedAttemptVerifies checks an attempt whose retained stdout yielded no admitted hook
// receipt (BBF-V0-018). Its raw stdout is already digest-bound; it retains no native report or
// artifacts, and its bytes do not decode as a hook receipt whatever its recorded process
// outcome, so an admitted receipt cannot be hidden behind a forged process failure.
func unadmittedAttemptVerifies(plan Plan, control PlannedControl, a AttemptResult, raw RawAttempt) bool {
	var hook HookReceipt
	if raw.NativeBytes != nil || raw.NativeSHA256 != "" || len(a.RetainedArtifacts) != 0 || a.WorkspaceBefore != plan.WorkspaceSHA256 ||
		Decode(raw.HookBytes, &hook) == nil {
		return false
	}
	return unadmittedStatusVerifies(plan, control, a)
}

// unadmittedStatusVerifies recomputes the executor's classification of an attempt without an
// admitted receipt from its process and cleanup observations; it is never a kill or a survivor.
func unadmittedStatusVerifies(plan Plan, control PlannedControl, a AttemptResult) bool {
	candidate := a
	candidate.Reasons = []string{"hook-output-invalid"}
	if !processSucceeded(a.HookProcess) {
		candidate.Reasons = []string{"hook-process-failed"}
	}
	for _, reason := range a.Reasons {
		if reason == "cleanup-budget-overrun" || reason == "workspace-cleanup-mismatch" {
			candidate.Reasons = append(candidate.Reasons, reason)
		}
	}
	status, reasons, application := classifyAttempt(plan, control, candidate)
	return status != StatusKilled && status != StatusSurvived && status == a.Status && slices.Equal(reasons, a.Reasons) && application == a.Application
}

func recountReport(r *Report, plan Plan) {
	r.Counts = allStatusCounts()
	r.CoverageGaps = nil
	r.InertControls = nil
	r.Requested = len(r.Results)
	r.Executed, r.Supported = 0, 0
	for _, c := range r.Results {
		r.Counts[c.Status]++
		if c.Status != StatusKilled {
			r.CoverageGaps = append(r.CoverageGaps, c.ID)
		}
		if c.Application == ApplicationNotApplied {
			r.InertControls = append(r.InertControls, c.ID)
		}
		if controlExecuted(c) {
			r.Executed++
		}
	}
	for _, c := range plan.Controls {
		if c.Disposition == "run" {
			r.Supported++
		}
	}
	if hookReportedIsolation(plan, r.Results) {
		r.CoverageGaps = append(r.CoverageGaps, IsolationHookReportedGap)
	}
	r.MutationScore = mutationScore(r.Counts)
}
