package dogfoodflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/dogfoodoperation"
	"github.com/Beamfall/corvint/internal/groupreap"
	"github.com/Beamfall/corvint/internal/trace"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

const AggregateWorkerProfile = "corvint-dogfood-outcome-worker/0"
const aggregateWorkerFrameLimit = tracerecordrepo.AggregateReceiptLimit + 4096

// AggregateWorkerRequest accepts no arbitrary command, output path, or store.
type AggregateWorkerRequest struct {
	Profile                   string                            `json:"profile"`
	Mode                      string                            `json:"mode"`
	Expected                  tracerecordrepo.AggregateExpected `json:"expected"`
	RemainingGitOperations    int                               `json:"remainingGitOperations"`
	RemainingWorkMilliseconds int64                             `json:"remainingWorkMilliseconds"`
	Receipt                   json.RawMessage                   `json:"receipt"`
}

type aggregateLegacyRefusal struct {
	Code  string `json:"code"`
	Error string `json:"error"`
	OK    bool   `json:"ok"`
}

type aggregateProduceResponse struct {
	Profile               string                 `json:"profile"`
	Mode                  string                 `json:"mode"`
	ConsumedGitOperations int                    `json:"consumedGitOperations"`
	Receipt               json.RawMessage        `json:"receipt"`
	LegacyAdmission       aggregateLegacyRefusal `json:"legacyAdmission"`
}
type aggregateVerifyResponse struct {
	Profile                string `json:"profile"`
	Mode                   string `json:"mode"`
	ConsumedGitOperations  int    `json:"consumedGitOperations"`
	VerifiedEnvelopeSHA256 string `json:"verifiedEnvelopeSha256"`
}

func aggregateWorkerJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	parsed, err := wire.Parse(raw)
	if err != nil {
		return nil, err
	}
	raw = wire.CanonicalValue(parsed)
	return append(raw, '\n'), nil
}

// Comparing the fully re-encoded closed struct also requires every field,
// rejects null/default type substitutions, and checks nested expected members.
func aggregateWorkerDecode(raw []byte, value any) error {
	if len(raw) > aggregateWorkerFrameLimit {
		return errors.New("aggregate-worker-invalid")
	}
	if _, err := wire.Parse(bytes.TrimSuffix(raw, []byte{'\n'})); err != nil {
		return errors.New("aggregate-worker-invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return errors.New("aggregate-worker-invalid")
	}
	encoded, err := aggregateWorkerJSON(value)
	if err != nil || !bytes.Equal(raw, encoded) {
		return errors.New("aggregate-worker-invalid")
	}
	return nil
}

func ParseAggregateWorkerRequest(raw []byte) (AggregateWorkerRequest, error) {
	var request AggregateWorkerRequest
	if err := aggregateWorkerDecode(raw, &request); err != nil {
		return request, err
	}
	if request.Profile != AggregateWorkerProfile || (request.Mode != "produce" && request.Mode != "verify") || request.RemainingGitOperations < 1 || request.RemainingGitOperations > 64 || request.RemainingWorkMilliseconds < 1 || request.RemainingWorkMilliseconds > 300000 {
		return request, errors.New("aggregate-worker-invalid")
	}
	if request.Mode == "produce" {
		if !bytes.Equal(request.Receipt, []byte("null")) {
			return request, errors.New("aggregate-worker-invalid")
		}
	} else {
		if _, err := tracerecordrepo.ParseAggregateOutcome(append(bytes.Clone(request.Receipt), '\n')); err != nil {
			return request, err
		}
	}
	return request, nil
}

// ExecuteAggregateWorker is called only after the native entrypoint verified
// group ownership. It has no publication or trace-store path.
func ExecuteAggregateWorker(ctx context.Context, root string, request AggregateWorkerRequest) ([]byte, error) {
	if !gitstatus.OwnedWorker() {
		return nil, errors.New("aggregate-worker-invalid")
	}
	encoded, err := aggregateWorkerJSON(request)
	if err != nil {
		return nil, err
	}
	request, err = ParseAggregateWorkerRequest(encoded)
	if err != nil {
		return nil, err
	}
	budget, err := gitrun.NewOperationBudget(request.RemainingGitOperations, time.Now().Add(time.Duration(request.RemainingWorkMilliseconds)*time.Millisecond), true)
	if err != nil {
		return nil, err
	}
	ctx = gitrun.WithOperationBudget(ctx, budget)
	if request.Mode == "produce" {
		raw, err := tracerecordrepo.ProduceAggregateOutcome(ctx, root, request.Expected)
		if err != nil {
			return nil, err
		}
		return aggregateWorkerJSON(aggregateProduceResponse{AggregateWorkerProfile, "produce", budget.Used(), bytes.TrimSuffix(raw, []byte{'\n'}), aggregateLegacyRefusal{"admitted-path-limit", "changed_paths exceeds 200 paths", false}})
	}
	value, err := tracerecordrepo.VerifyAggregateOutcome(ctx, root, request.Expected, append(bytes.Clone(request.Receipt), '\n'))
	if err != nil {
		return nil, err
	}
	return aggregateWorkerJSON(aggregateVerifyResponse{AggregateWorkerProfile, "verify", budget.Used(), value.EnvelopeSHA256})
}

// aggregateWorkerBuffer prevents pipe writers from growing an unbounded buffer;
// overflow wakes the parent and forces owned retirement before returning.
type aggregateWorkerBuffer struct {
	mu       sync.Mutex
	bytes    []byte
	limit    int
	overflow chan struct{}
	once     sync.Once
}

func (b *aggregateWorkerBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := min(len(p), b.limit-len(b.bytes))
	b.bytes = append(b.bytes, p[:n]...)
	if n < len(p) {
		b.once.Do(func() { close(b.overflow) })
		return n, io.ErrShortWrite
	}
	return n, nil
}
func (b *aggregateWorkerBuffer) data() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return bytes.Clone(b.bytes)
}

// RunAggregateWorker delegates the unused quota while keeping eight operations
// for parent closing checks. Only closed zero-exit output after RELEASED counts.
func RunAggregateWorker(ctx context.Context, root, mode string, expected tracerecordrepo.AggregateExpected, receipt []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	binary, err := os.Executable()
	if err != nil {
		return nil, err
	}
	budget := gitrun.OperationBudgetFrom(ctx)
	if budget == nil {
		return nil, errors.New("aggregate-worker-invalid")
	}
	quota, remaining, err := budget.Delegate(8)
	if err != nil {
		return nil, err
	}
	request := AggregateWorkerRequest{AggregateWorkerProfile, mode, expected, quota, remaining.Milliseconds(), json.RawMessage("null")}
	if mode == "verify" {
		request.Receipt = bytes.TrimSuffix(receipt, []byte{'\n'})
	}
	raw, err := aggregateWorkerJSON(request)
	if err != nil {
		return nil, err
	}
	if _, err = ParseAggregateWorkerRequest(raw); err != nil {
		return nil, err
	}
	command := exec.Command(binary, "dogfood-outcome-worker", root)
	command.Stdin = bytes.NewReader(raw)
	stdoutRaw, stderrRaw, status, err := runAggregateOwnedProcess(ctx, command, aggregateWorkerFrameLimit, 64<<10, budget.Deadline())
	if err != nil {
		return nil, err
	}
	if status != 0 {
		return nil, aggregateWorkerFailure(status, stdoutRaw, stderrRaw)
	}
	if mode == "produce" {
		var response aggregateProduceResponse
		if err := aggregateWorkerDecode(stdoutRaw, &response); err != nil {
			return nil, err
		}
		if response.Profile != AggregateWorkerProfile || response.Mode != mode || response.LegacyAdmission != (aggregateLegacyRefusal{"admitted-path-limit", "changed_paths exceeds 200 paths", false}) {
			return nil, errors.New("aggregate-worker-invalid")
		}
		encoded := append(bytes.Clone(response.Receipt), '\n')
		value, err := tracerecordrepo.ParseAggregateOutcome(encoded)
		if err != nil {
			return nil, err
		}
		left, _ := aggregateWorkerJSON(value.AggregateExpected)
		right, _ := aggregateWorkerJSON(expected)
		if !bytes.Equal(left, right) {
			return nil, errors.New("aggregate-binding-drift")
		}
		if response.ConsumedGitOperations < 1 || budget.Settle(response.ConsumedGitOperations) != nil {
			return nil, errors.New("aggregate-worker-invalid")
		}
		return encoded, nil
	}
	var response aggregateVerifyResponse
	if err := aggregateWorkerDecode(stdoutRaw, &response); err != nil {
		return nil, err
	}
	value, err := tracerecordrepo.ParseAggregateOutcome(receipt)
	if err != nil {
		return nil, err
	}
	if response.Profile != AggregateWorkerProfile || response.Mode != mode || response.VerifiedEnvelopeSHA256 != value.EnvelopeSHA256 || response.ConsumedGitOperations < 1 || budget.Settle(response.ConsumedGitOperations) != nil {
		return nil, errors.New("aggregate-worker-invalid")
	}
	return bytes.Clone(receipt), nil
}

// runAggregateOwnedProcess uses one retirement allowance for all cleanup stages.
// Output is accepted only after observed RELEASED, including normal exit.
func runAggregateOwnedProcess(ctx context.Context, command *exec.Cmd, stdoutLimit, stderrLimit int, workDeadline time.Time) ([]byte, []byte, int, error) {
	if err := dogfoodoperation.Check(ctx); err != nil {
		return nil, nil, -1, err
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, -1, err
	}
	callerDeadline, hasCaller := ctx.Deadline()
	if hasCaller {
		lastWork := callerDeadline.Add(-20 * time.Second)
		if !time.Now().Before(lastWork) {
			return nil, nil, -1, errors.New("aggregate-operation-deadline")
		}
		if lastWork.Before(workDeadline) {
			workDeadline = lastWork
		}
	}
	if !time.Now().Before(workDeadline) {
		return nil, nil, -1, errors.New("aggregate-operation-deadline")
	}
	command.WaitDelay = 2 * time.Second
	stdout := &aggregateWorkerBuffer{limit: stdoutLimit, overflow: make(chan struct{})}
	stderr := &aggregateWorkerBuffer{limit: stderrLimit, overflow: make(chan struct{})}
	command.Stdout, command.Stderr = stdout, stderr
	owner, err := groupreap.Start(command)
	if err != nil {
		return nil, nil, -1, err
	}
	timer := time.NewTimer(time.Until(workDeadline))
	defer timer.Stop()
	stopped := false
	select {
	case <-owner.Exited():
	case <-ctx.Done():
		stopped = true
	case <-timer.C:
		stopped = true
	case <-stdout.overflow:
		stopped = true
	case <-stderr.overflow:
		stopped = true
	}
	retirementDeadline := time.Now().Add(20 * time.Second)
	if hasCaller && callerDeadline.Before(retirementDeadline) {
		retirementDeadline = callerDeadline
	}
	retirement, cancel := context.WithDeadline(context.Background(), retirementDeadline)
	defer cancel()
	if stopped {
		owner.Stop()
	}
	bound := groupreap.RetirementBound{Done: retirement.Done(), Expired: func() bool { return !time.Now().Before(retirementDeadline) }}
	result := finishAggregateOwner(ctx, owner, bound)
	if result.State != groupreap.Released {
		// These are bounded prefix observations, not closed successful streams.
		out, errOut := stdout.data(), stderr.data()
		diagnostic, _ := json.Marshal(struct {
			State        string   `json:"state"`
			Leader       int      `json:"leader"`
			StdoutSHA256 [32]byte `json:"stdoutPrefixSha256"`
			StderrSHA256 [32]byte `json:"stderrPrefixSha256"`
			StdoutBytes  int      `json:"stdoutPrefixBytes"`
			StderrBytes  int      `json:"stderrPrefixBytes"`
		}{"HOLD", command.Process.Pid, sha256.Sum256(out), sha256.Sum256(errOut), len(out), len(errOut)})
		retained := struct {
			Owner          *groupreap.Owner
			Stdout, Stderr *aggregateWorkerBuffer
		}{owner, stdout, stderr}
		return nil, nil, -1, dogfoodoperation.Hold(ctx, diagnostic, retained)
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, -1, err
	}
	if !time.Now().Before(workDeadline) {
		return nil, nil, -1, errors.New("aggregate-operation-deadline")
	}
	select {
	case <-stdout.overflow:
		stopped = true
	default:
	}
	select {
	case <-stderr.overflow:
		stopped = true
	default:
	}
	if stopped {
		return nil, nil, -1, errors.New("aggregate-worker-invalid")
	}
	return stdout.data(), stderr.data(), exitStatus(result.WaitErr), nil
}

// The private caller seam exercises HOLD propagation without altering the
// pinned Owner or signalling an uncertain/reaped numeric process identity.
type aggregateOwnerFinishKey struct{}

func finishAggregateOwner(ctx context.Context, owner *groupreap.Owner, bound groupreap.RetirementBound) groupreap.Result {
	if finish, ok := ctx.Value(aggregateOwnerFinishKey{}).(func(*groupreap.Owner, groupreap.RetirementBound) groupreap.Result); ok {
		return finish(owner, bound)
	}
	return owner.FinishBounded(bound)
}

// ValidAggregateVerifierArguments is deliberately closed to the actual strict
// checker requests. A verifier worker cannot run arbitrary public commands.
func ValidAggregateVerifierArguments(args []string) bool {
	if len(args) == 5 && args[0] == "query" && args[1] == "--task" && args[3] == "--limit" && args[4] == "1" {
		return !strings.ContainsRune(args[2], 0) && contextindex.ValidateQueryAuthorityStart(args[2], 1) == nil
	}
	if len(args) == 7 && args[0] == "impact" && args[1] == "--base" && wire.IsGitOid(args[2]) && args[3] == "--range-profile" && args[4] == "expanded-256" && args[5] == "--limit" && args[6] == "20" {
		return true
	}
	if len(args) == 6 && args[0] == "dogfood-ocm" && args[1] == "status" && args[2] == "--expected-base" && wire.IsGitOid(args[3]) && args[4] == "--target" && wire.IsGitOid(args[5]) {
		return true
	}
	if len(args) == 12 && args[0] == "cem" && args[1] == "status" && args[2] == "--map" && args[3] == ".corvint/change.cem.json" && args[4] == "--expected-base" && wire.IsGitOid(args[5]) && args[6] == "--target" && wire.IsGitOid(args[7]) && args[8] == "--max-unknown" && args[10] == "--max-mechanical" && args[11] == "0" {
		count, err := strconv.Atoi(args[9])
		return err == nil && count >= 0 && count <= 200000 && strconv.Itoa(count) == args[9]
	}
	return false
}

func runAggregateVerifier(ctx context.Context, runner Runner, root string, args []string) (verification, error) {
	if !filepath.IsAbs(runner.Path) || !ValidAggregateVerifierArguments(args) {
		return verification{}, errors.New("aggregate-worker-invalid")
	}
	command := exec.Command(runner.Path, append([]string{"dogfood-verifier-worker", root}, args...)...)
	command.Env = append(os.Environ(), "LC_ALL=C")
	stdout, stderr, status, err := runAggregateOwnedProcess(ctx, command, 4<<20, 64<<10, time.Now().Add(300*time.Second))
	return verification{status, stdout, stderr}, err
}

// AggregateWorkerFailureCode preserves the profile's typed resource/refusal
// reasons across the owned subprocess boundary. Other failures remain generic.
func AggregateWorkerFailureCode(err error) string {
	if err == nil {
		return "aggregate-worker-invalid"
	}
	code := trace.AdmissionFailureReason(err)
	if code == "" {
		code = err.Error()
	}
	switch code {
	case "aggregate-profile-unsupported", "aggregate-object-format-unsupported", "aggregate-enrollment-required", "aggregate-legacy-failure-unverified", "aggregate-not-required", "aggregate-schema-invalid", "aggregate-binding-drift", "aggregate-source-set-drift", "aggregate-candidate-limit", "aggregate-admitted-limit", "aggregate-receipt-byte-limit", "aggregate-git-operation-limit", "aggregate-operation-deadline", "aggregate-cleanup-hold", "aggregate-history-bound-exceeded", "aggregate-prior-evidence-drift", "aggregate-publication-failed", "aggregate-worker-invalid":
		return code
	default:
		return "aggregate-worker-invalid"
	}
}

func aggregateWorkerFailure(status int, stdout, stderr []byte) error {
	bad := errors.New("aggregate-worker-invalid")
	if status != 2 || len(stdout) != 0 || len(stderr) > 64<<10 {
		return bad
	}
	parsed, err := wire.Parse(stderr)
	if err != nil {
		return bad
	}
	var value aggregateLegacyRefusal
	decoder := json.NewDecoder(bytes.NewReader(stderr))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || value.OK || value.Error == "" {
		return bad
	}
	canonical, err := aggregateWorkerJSON(value)
	if err != nil || !bytes.Equal(wire.CanonicalValue(parsed), bytes.TrimSuffix(canonical, []byte{'\n'})) {
		return bad
	}
	code := AggregateWorkerFailureCode(errors.New(value.Code))
	if code != value.Code {
		return bad
	}
	return errors.New(code)
}
