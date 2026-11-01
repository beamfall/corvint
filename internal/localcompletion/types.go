// Package localcompletion coordinates a caller-owned local workflow. Its
// receipts are observations, never harness attestation or semantic authority.
package localcompletion

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

const (
	MaxPlanBytes     = 64 << 10
	maxStateBytes    = 256 << 10
	maxArtifactBytes = 4 << 20
	maxLogBytes      = 256 << 10
	maxAttempts      = 64
)

var keyPattern, idPattern = regexp.MustCompile(`^[0-9a-f]{64}$`), regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

type Check struct {
	ID                       string   `json:"id"`
	Argv                     []string `json:"argv"`
	TimeoutSeconds           int      `json:"timeoutSeconds"`
	AllowCemSidecarOnlyReuse bool     `json:"allowCemSidecarOnlyReuse"`
	// AllowSealArchiveReuse declares the check independent of where the CEM
	// is archived, so an observation at the evidence target qualifies at its
	// exact seal (LCP-V0-920). Omitted when false, so older plan digests hold.
	AllowSealArchiveReuse bool `json:"allowSealArchiveReuse,omitempty"`
}

type Plan struct {
	Base    string   `json:"base"`
	Intents []string `json:"intents"`
	Checks  []Check  `json:"checks"`
}

type IntentPointer struct {
	Path     string `json:"path"`
	Revision string `json:"revision"`
	BlobHash string `json:"blob_hash"`
}

type Evaluation struct {
	EvidenceTarget  string             `json:"evidenceTarget,omitempty"`
	Lifecycle       string             `json:"lifecycle"`
	Owner           string             `json:"owner,omitempty"`
	Satisfied       bool               `json:"satisfied"`
	Unmet           []string           `json:"unmet"`
	Base            string             `json:"base,omitempty"`
	Target          string             `json:"target,omitempty"`
	PlanDigest      string             `json:"planDigest,omitempty"`
	Intents         []string           `json:"intents"`
	IntentPointers  []IntentPointer    `json:"intentPointers"`
	ReportSetDigest string             `json:"reportSetDigest,omitempty"`
	Reports         []string           `json:"reports,omitempty"`
	NextActions     [][]string         `json:"nextActions,omitempty"`
	NoActionReason  string             `json:"noActionReason,omitempty"`
	Checks          []CheckObservation `json:"checks,omitempty"`
	Plan            *Plan              `json:"plan,omitempty"`
	Evidence        []EvidenceWorklist `json:"evidence,omitempty"`
}

type EvidenceWorklist struct {
	Tool     string `json:"tool"`
	Map      string `json:"map"`
	Code     string `json:"code,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Worklist []any  `json:"worklist"`
	Omitted  int    `json:"omitted"`
}

type CheckObservation struct {
	ID             string   `json:"id"`
	Qualified      bool     `json:"qualified"`
	Argv           []string `json:"argv"`
	TestedCommit   string   `json:"testedCommit,omitempty"`
	CurrentTarget  string   `json:"currentTarget"`
	Exit           int      `json:"exit"`
	TimedOut       bool     `json:"timedOut"`
	Cancelled      bool     `json:"cancelled"`
	SecretScreened bool     `json:"secretScreened"`
	Stdout         string   `json:"stdout,omitempty"`
	Stderr         string   `json:"stderr,omitempty"`
	ReuseRefused   string   `json:"reuseRefused,omitempty"` // closed sidecar-only reuse refusal (CEM-V1-207)
	FailedTests    []string `json:"failedTests,omitempty"`  // verify result only: bounded `--- FAIL:` names of the run just observed (LCP-V0-031)
}

type artifact struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type observation struct {
	CheckID        string   `json:"checkId"`
	CheckDigest    string   `json:"checkDigest"`
	Target         string   `json:"testedCommit"`
	Tree           string   `json:"tree"`
	ContentDigest  string   `json:"contentExcludingCem"`
	Clean          bool     `json:"clean"`
	Exit           string   `json:"exit"`
	Passed         bool     `json:"passed"`
	TimedOut       bool     `json:"timedOut"`
	Cancelled      bool     `json:"cancelled"`
	Overflow       bool     `json:"overflow"`
	SecretScreened bool     `json:"secretScreened"`
	Stdout         artifact `json:"stdout"`
	Stderr         artifact `json:"stderr"`
}

type reportSet struct {
	PlanDigest   string        `json:"planDigest"`
	Target       string        `json:"target"`
	Bindings     []artifact    `json:"bindings"`
	Reports      []artifact    `json:"reports"`
	Observations []observation `json:"observations"`
	Digest       string        `json:"digest"`
}

type terminal struct {
	ReportSet string     `json:"reportSet"`
	CheckExit int        `json:"checkExit"`
	Artifacts []artifact `json:"artifacts"`
}

type state struct {
	Sealed           *sealedContinuation `json:"sealedContinuation,omitempty"`
	AggregateOutcome *aggregateState     `json:"aggregateOutcome,omitempty"`
	Session          string              `json:"session"`
	Lifecycle        string              `json:"lifecycle"`
	Plan             Plan                `json:"plan"`
	PlanDigest       string              `json:"planDigest"`
	IntentPointers   []IntentPointer     `json:"intentPointers"`
	Observations     []observation       `json:"observations"`
	ReportSet        *reportSet          `json:"reportSet"`
	Review           string              `json:"review"`
	Terminal         *terminal           `json:"terminal"`
	Coordination     []artifact          `json:"coordination"`
	Executables      []string            `json:"executables"`
	Generation       string              `json:"generation"`
}

func HashSession(raw string) string {
	return digest([]byte("corvint-local-completion-session/0\x00" + raw))
}

func SessionKey(explicit string) (string, error) {
	if explicit != "" {
		if !keyPattern.MatchString(explicit) {
			return "", errors.New("invalid-session-key")
		}
		return explicit, nil
	}
	for _, name := range []string{"CODEX_THREAD_ID", "CODEX_SESSION_ID"} {
		if raw := os.Getenv(name); raw != "" {
			if len(raw) > 4096 {
				return "", errors.New("invalid-session-identity")
			}
			return HashSession(raw), nil
		}
	}
	return "", errors.New("session-identity-required")
}

func digest(raw []byte) string     { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func valueDigest(value any) string { raw, _ := json.Marshal(value); return digest(raw) }

// Aggregate state is a closed additive discriminator. Omitting it preserves
// legacy state bytes; every aggregate phase is visible to strict old readers.
type aggregateState struct {
	Profile              string                       `json:"profile"`
	BindingSHA256        string                       `json:"bindingSha256"`
	ReceiptSHA256        string                       `json:"receiptSha256"`
	LegacyFailureSHA256  string                       `json:"legacyFailureSha256"`
	AttemptCount         int                          `json:"attemptCount"`
	Phase                string                       `json:"phase"`
	ActiveSnapshotSHA256 string                       `json:"activeSnapshotSha256"`
	PreservationState    string                       `json:"preservationState"`
	ExpectedOld          aggregateExpectedOld         `json:"expectedOld"`
	Issued               []aggregateIssued            `json:"issued"`
	Snapshots            []aggregateSnapshotReference `json:"snapshots"`
}

type aggregateOld struct {
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
	Kind   string `json:"kind"`
}
type aggregateExpectedOld struct {
	Outcome *aggregateOld `json:"outcome"`
	Report  *aggregateOld `json:"report"`
}
type aggregateIssued struct {
	Role   string `json:"role"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}
type aggregateSnapshotReference struct {
	Ordinal        int    `json:"ordinal"`
	SnapshotSHA256 string `json:"snapshotSha256"`
	ReservedBytes  int64  `json:"reservedBytes"`
}
type aggregateSnapshotEntry struct {
	Role    string  `json:"role"`
	Present bool    `json:"present"`
	SHA256  *string `json:"sha256"`
	Bytes   int64   `json:"bytes"`
}
type aggregateSnapshotManifest struct {
	Profile           string                   `json:"profile"`
	BindingSHA256     string                   `json:"bindingSha256"`
	SourceStateSHA256 string                   `json:"sourceStateSha256"`
	Entries           []aggregateSnapshotEntry `json:"entries"`
}
type aggregateReservation struct {
	Ordinal           int                      `json:"ordinal"`
	BindingSHA256     string                   `json:"bindingSha256"`
	SnapshotSHA256    string                   `json:"snapshotSha256"`
	SourceStateSHA256 string                   `json:"sourceStateSha256"`
	ExpectedOld       aggregateExpectedOld     `json:"expectedOld"`
	Entries           []aggregateSnapshotEntry `json:"entries"`
	MaximumBytes      int64                    `json:"maximumBytes"`
}

type aggregateLegacyFailure struct {
	Base            string   `json:"base"`
	Target          string   `json:"target"`
	Tree            string   `json:"tree"`
	Session         string   `json:"session"`
	Generation      string   `json:"generation"`
	PlanDigest      string   `json:"planDigest"`
	ReportSetDigest string   `json:"reportSetDigest"`
	CEMSHA256       string   `json:"cemSha256"`
	ReportSHA256    string   `json:"reportSha256"`
	ExitStatus      int      `json:"exitStatus"`
	StdoutSHA256    string   `json:"stdoutSha256"`
	StderrSHA256    string   `json:"stderrSha256"`
	CaptureOrdinal  int      `json:"captureOrdinal"`
	Task            string   `json:"task"`
	Verification    []string `json:"verification"`
	Outcome         string   `json:"outcome"`
}

type aggregateCheckCapture struct {
	Session            string  `json:"session"`
	Generation         string  `json:"generation"`
	PlanDigest         string  `json:"planDigest"`
	BindingSHA256      string  `json:"bindingSha256"`
	SnapshotSHA256     string  `json:"snapshotSha256"`
	ReportSetDigest    string  `json:"reportSetDigest"`
	Base               string  `json:"base"`
	Target             string  `json:"target"`
	Tree               string  `json:"tree"`
	ReportBeforeSHA256 string  `json:"reportBeforeSha256"`
	ReportAfterSHA256  *string `json:"reportAfterSha256"`
	ExitStatus         int     `json:"exitStatus"`
	Failed             bool    `json:"failed"`
	StdoutSHA256       string  `json:"stdoutSha256"`
	StderrSHA256       string  `json:"stderrSha256"`
	StdoutBytes        int64   `json:"stdoutBytes"`
	StderrBytes        int64   `json:"stderrBytes"`
}

// Per-check plan bounds (LCP-V0-032).
const (
	maxCheckArgv           = 64
	maxCheckTimeoutSeconds = 3600
)

// CancelReasonError is the cancel-reason-required refusal (LCP-V0-038). Its
// Error is the stable code; Detail names what the cancel would close.
type CancelReasonError struct {
	Checks       int
	Observations int
}

func (e *CancelReasonError) Error() string { return "cancel-reason-required" }

// Detail names the selected checks and saved observations the cancel closes.
func (e *CancelReasonError) Detail() string {
	return fmt.Sprintf("cancelling this active enrollment closes %d selected checks and %d saved observations as non-success", e.Checks, e.Observations)
}

// SealedDriftError is the sealed-evidence-drift refusal (LCP-V0-042). Its
// Error is the stable code; Detail names the drifted artifact by a fixed label
// and a fixed reason, with the pinned and current digests when the drift is a
// byte change. Labels and reasons are constants, so Detail carries no path.
type SealedDriftError struct {
	Artifact string
	Reason   string
	Pinned   string
	Current  string
}

func (e *SealedDriftError) Error() string { return "sealed-evidence-drift" }

// Detail names the artifact, the reason and, for a byte change, both digests.
func (e *SealedDriftError) Detail() string {
	detail := "artifact=" + e.Artifact + " reason=" + e.Reason
	if e.Pinned != "" {
		detail += " pinned=" + e.Pinned + " current=" + e.Current
	}
	return detail
}

// FinalCheckError is a final strict check that did not pass (LCP-V0-043). When
// the caller's context ended during the check (a supervised signal), it is the
// finish-interrupted result naming the step and the rerun command; otherwise it
// is final-check-failed naming the retained logs. Step and Logs are fixed names
// under the Git directory and Session is the hashed key, so Detail carries no
// command output.
type FinalCheckError struct {
	Interrupted bool
	Step        string
	Session     string
	Logs        []string
}

func (e *FinalCheckError) Error() string {
	if e.Interrupted {
		return "finish-interrupted"
	}
	return "final-check-failed"
}

// Detail names the step, its retained logs and, after an interruption, the rerun.
func (e *FinalCheckError) Detail() string {
	detail := "step=" + e.Step
	if len(e.Logs) > 0 {
		detail += " logs=" + strings.Join(e.Logs, ",")
	}
	if e.Interrupted {
		detail += " rerun=corvint dogfood finish --session-key " + e.Session
	}
	return detail
}

// CheckBoundError is the invalid-check-bound refusal. Its Error is the stable
// code; Detail names the bounds and the requested values (LCP-V0-032, V1-1011).
// CheckID has already matched the check id pattern, so Detail carries no path
// or command text; an id that matches the secret screen is shown as
// secretscreen.Placeholder.
type CheckBoundError struct {
	CheckID        string
	Argv           int
	TimeoutSeconds int
}

func (e *CheckBoundError) Error() string { return "invalid-check-bound" }

// Detail names the per-check bounds and the values the plan requested.
func (e *CheckBoundError) Detail() string {
	id := e.CheckID
	if secretscreen.MatchString(id) {
		id = secretscreen.Placeholder
	}
	return fmt.Sprintf("check %s requested %d argv elements and timeoutSeconds %d (bounds 1-%d and 1-%d)", id, e.Argv, e.TimeoutSeconds, maxCheckArgv, maxCheckTimeoutSeconds)
}
