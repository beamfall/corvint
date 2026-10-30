package cemcandidate

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// RunnerAdmissionProfile is the one value of a plain request's optional
// runnerAdmission member (CEM-V1-280). When it is present the producer decodes
// the runner plan and receipt as corvint-test-runner documents and refuses a
// receipt that is tampered, stale, failed or wholly skipped before it writes
// anything; when it is absent the runner files stay opaque (CEM-V1-240).
const RunnerAdmissionProfile = "corvint-test-runner/0"

const (
	runnerPlanProfile    = "corvint-test-runner-plan/0"
	runnerReceiptProfile = "corvint-test-runner-receipt/0"
	// maxRunnerSourceBytes bounds the target blobs one admission reads.
	maxRunnerSourceBytes = 64 << 20
)

// admitRunnerReceipt applies the CEM-V1-281..285 checks, in that order, and
// returns a registered error naming the first one that fails. An admitted
// receipt returns its plan's runner profile, which the runners form uses to
// refuse two receipts of one profile for one criterion (CEM-V1-576).
func admitRunnerReceipt(ctx context.Context, repository, target string, plan, receipt []byte) (string, error) {
	var p tr.PlanDocument
	if e := tr.DecodeDocument(plan, &p); e != nil || p.Profile != runnerPlanProfile {
		return "", cemcode.New("runner-document-invalid", "runner plan is not a closed %s document", runnerPlanProfile)
	}
	var r tr.ReceiptDocument
	if e := tr.DecodeDocument(receipt, &r); e != nil || r.Profile != runnerReceiptProfile {
		return "", cemcode.New("runner-document-invalid", "runner receipt is not a closed %s document", runnerReceiptProfile)
	}
	if r.PlanSha256 != tr.Identity(p) || r.Execution.InputSha256 != tr.Identity(p.Request.InputFiles) || r.Execution.Runner != p.Request.Runner {
		return "", cemcode.New("runner-receipt-tampered", "runner receipt does not bind the pinned plan")
	}
	if e := runnerInputsAtTarget(ctx, repository, target, p.Request.InputFiles); e != nil {
		return "", e
	}
	passed := false
	for _, t := range r.Observation.Tests {
		switch t.State {
		case tr.Passed:
			passed = true
		case tr.Skipped:
		default:
			return "", cemcode.New("runner-receipt-failed", "runner receipt observes a %s test", t.State)
		}
	}
	if r.Error != "" || !r.Observation.Complete {
		return "", cemcode.New("runner-receipt-failed", "runner receipt observation is incomplete or carries an execution error")
	}
	if !passed {
		return "", cemcode.New("runner-receipt-skipped", "runner receipt observes no passed test")
	}
	return p.Request.Runner, nil
}

// runnerInputsAtTarget requires every plan input file to be a blob of the
// target tree whose SHA-256 equals the plan's pin (CEM-V1-283): a receipt
// over other source bytes is stale for this target.
func runnerInputsAtTarget(ctx context.Context, repository, target string, inputs map[string]string) error {
	if len(inputs) == 0 || len(inputs) > 4096 {
		return cemcode.New("runner-receipt-stale", "runner plan pins no bounded input set")
	}
	names := make([]string, 0, len(inputs))
	var stdin bytes.Buffer
	for name := range inputs {
		if !literal(name) || strings.ContainsAny(name, "\r\n") {
			return cemcode.New("runner-document-invalid", "runner plan input path is not a literal repository path")
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		stdin.WriteString(target + ":" + name + "\n")
	}
	env := []string{"PATH=" + os.Getenv("PATH"), "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_TERMINAL_PROMPT=0"}
	out, e := gitrun.Run(ctx, gitrun.NewBudget(1, time.Minute), gitrun.Options{Dir: repository, Env: env, Stdin: stdin.Bytes(), StdoutLimit: maxRunnerSourceBytes}, "--no-optional-locks", "-C", repository, "cat-file", "--batch")
	if e != nil {
		return fmt.Errorf("read runner inputs at target: %w", e)
	}
	reader := bufio.NewReader(bytes.NewReader(out))
	for _, name := range names {
		header, e := reader.ReadString('\n')
		if e != nil {
			return fmt.Errorf("read runner inputs at target: truncated object stream")
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || fields[1] != "blob" {
			return cemcode.New("runner-receipt-stale", "runner input %s is not a file of the target", name)
		}
		size, e := strconv.Atoi(fields[2])
		if e != nil || size < 0 || size > maxRunnerSourceBytes {
			return fmt.Errorf("read runner inputs at target: invalid object size")
		}
		blob := make([]byte, size+1)
		if _, e := io.ReadFull(reader, blob); e != nil || blob[size] != '\n' {
			return fmt.Errorf("read runner inputs at target: truncated object")
		}
		if tr.Digest(blob[:size]) != inputs[name] {
			return cemcode.New("runner-receipt-stale", "runner input %s differs from the target", name)
		}
	}
	return nil
}
