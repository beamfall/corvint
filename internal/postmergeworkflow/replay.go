package postmergeworkflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	connector "github.com/Beamfall/corvint/internal/postmergeconnector"
	"github.com/Beamfall/corvint/internal/procgroup"
)

func readBounded(name string) ([]byte, error) {
	info, err := os.Stat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxBytes {
		return nil, fmt.Errorf("input-size-invalid")
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("input-unavailable")
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxBytes {
		return nil, fmt.Errorf("input-size-invalid")
	}
	b, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil || len(b) > MaxBytes {
		return nil, fmt.Errorf("input-size-invalid")
	}
	return b, nil
}
func readExecutable(name string) ([]byte, error) {
	info, err := os.Stat(name)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 || info.Size() > 128<<20 {
		return nil, fmt.Errorf("executable-invalid")
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("executable-unavailable")
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 || info.Size() > 128<<20 {
		return nil, fmt.Errorf("executable-invalid")
	}
	b, err := io.ReadAll(io.LimitReader(f, (128<<20)+1))
	if err != nil || len(b) > 128<<20 {
		return nil, fmt.Errorf("executable-invalid")
	}
	return b, nil
}

// Replay executes the exact pinned bytes twice, in fresh private working
// directories. Minimal env is not OS isolation; the report retains that limit.
// Input files are never modified and no live connector writer is constructed.
func Replay(ctx context.Context, root, fixtureFile, policyFile, change string) (Report, error) {
	report := Report{Profile: Profile, Status: "BLOCKED", Reasons: []string{}, HumanVerifiedMismatches: []Mismatch{}, GeneratedMismatches: []Mismatch{}, DeferredStages: []string{}, WorkflowQualification: "NOT_OBSERVED",
		Limits: []string{"adapter-semantic-correctness-not-verified", "filesystem-network-isolation-not-verified", "descendant-observation-bounded", "human-registry-pin-is-not-authentication", "actual-whole-workflow-acceptance-not-observed"}}
	block := func(err error) (Report, error) {
		report.Reasons = append(report.Reasons, err.Error())
		return report, err
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return block(fmt.Errorf("product-root-invalid"))
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return block(fmt.Errorf("product-root-invalid"))
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return block(fmt.Errorf("product-root-invalid"))
	}
	fixtureBytes, err := readBounded(fixtureFile)
	if err != nil {
		return block(err)
	}
	policyBytes, err := readBounded(policyFile)
	if err != nil {
		return block(err)
	}
	report.FixtureSHA256 = SHA256(fixtureBytes)
	report.PolicySHA256 = SHA256(policyBytes)
	var fixture Fixture
	var policy Policy
	if err = Decode(fixtureBytes, &fixture); err != nil {
		return block(err)
	}
	if err = Decode(policyBytes, &policy); err != nil {
		return block(err)
	}
	report.Binding = fixture.Connector.Forge.Change.Binding
	if change != report.Binding.Change {
		return block(fmt.Errorf("change-selector-mismatch"))
	}
	var registry Registry
	if policy.Registry != "" {
		b, e := readBounded(policy.Registry)
		if e != nil {
			return block(e)
		}
		if SHA256(b) != policy.RegistrySHA256 {
			return block(fmt.Errorf("registry-pin-mismatch"))
		}
		if e = Decode(b, &registry); e != nil {
			return block(e)
		}
	}
	if err = validateInputs(fixture, policy, registry); err != nil {
		return block(err)
	}
	if _, err = connector.Read(ctx, root, fixture.Connector, policy.Connector, fixture.SourceItem); err != nil {
		return block(err)
	}
	executable, err := readExecutable(policy.Executable)
	if err != nil {
		return block(err)
	}
	if SHA256(executable) != policy.ExecutableSHA256 {
		return block(fmt.Errorf("executable-pin-mismatch"))
	}
	request := Request{Profile: Profile, Binding: report.Binding, FixtureSHA256: report.FixtureSHA256, RuntimeSHA256: policy.RuntimeSHA256}
	protectedInputs := []string{fixtureFile, policyFile, policy.Executable}
	if policy.Registry != "" {
		protectedInputs = append(protectedInputs, policy.Registry)
	}
	for i, name := range protectedInputs {
		protectedInputs[i], err = filepath.EvalSymlinks(name)
		if err != nil {
			return block(fmt.Errorf("input-path-unavailable"))
		}
		protectedInputs[i], err = filepath.Abs(protectedInputs[i])
		if err != nil {
			return block(fmt.Errorf("input-path-unavailable"))
		}
	}
	var firstResult Result
	var firstRecording []byte
	for i := 0; i < 2; i++ {
		result, recording, e := run(ctx, root, fixture, policy, request, executable, protectedInputs)
		if e != nil {
			return block(e)
		}
		if i == 0 {
			firstResult = result
			firstRecording = recording
			continue
		}
		if valueHash(firstResult) != valueHash(result) || !bytes.Equal(firstRecording, recording) {
			return block(fmt.Errorf("replay-nondeterministic"))
		}
	}
	report.RecordingSHA256 = SHA256(firstRecording)
	report.Recording = string(firstRecording)
	followup := false
	plan, err := connector.Build(ctx, root, fixture.Connector, policy.Connector, firstResult.Input)
	if err != nil {
		return block(err)
	}
	for _, r := range plan.Requests {
		if r.Operation == "upsert-followup" {
			followup = true
		}
	}
	observed := map[string]any{"affected_flows": normalized(firstResult.AffectedFlows), "followup": followup, "test_gaps": normalized(firstResult.TestGaps), "defects": sortedFindings(firstResult.Defects)}
	expected := map[string]any{"affected_flows": normalized(fixture.Expected.AffectedFlows), "followup": fixture.Expected.Followup, "test_gaps": normalized(fixture.Expected.TestGaps), "defects": sortedFindings(fixture.Expected.Defects)}
	for _, field := range []string{"affected_flows", "followup", "test_gaps", "defects"} {
		if valueHash(expected[field]) != valueHash(observed[field]) {
			m := Mismatch{field, fixture.Expected.Labels[field].Basis, valueHash(expected[field]), valueHash(observed[field])}
			if m.Basis == "human-verified" {
				report.HumanVerifiedMismatches = append(report.HumanVerifiedMismatches, m)
			} else {
				report.GeneratedMismatches = append(report.GeneratedMismatches, m)
			}
		}
	}
	for _, stage := range firstResult.Stages {
		if stage.Status == "deferred" {
			report.DeferredStages = append(report.DeferredStages, stage.Name)
		}
	}
	if len(report.DeferredStages) > 0 {
		report.Limits = append(report.Limits, "authoring-scope-validation-stages-deferred")
	}
	report.Status = "MATCH"
	if len(report.HumanVerifiedMismatches)+len(report.GeneratedMismatches) > 0 {
		report.Status = "MISMATCH"
	}
	return report, nil
}

func run(ctx context.Context, root string, f Fixture, p Policy, request Request, executable []byte, inputs []string) (Result, []byte, error) {
	scratch, err := os.MkdirTemp("", "corvint-replay-")
	if err != nil {
		return Result{}, nil, fmt.Errorf("scratch-unavailable")
	}
	defer os.RemoveAll(scratch)
	scratch, err = filepath.EvalSymlinks(scratch)
	if err != nil {
		return Result{}, nil, fmt.Errorf("scratch-unavailable")
	}
	author := filepath.Join(scratch, "author")
	home := filepath.Join(scratch, "home")
	tmp := filepath.Join(scratch, "tmp")
	for _, dir := range []string{author, home, tmp} {
		if err = os.Mkdir(dir, 0700); err != nil {
			return Result{}, nil, fmt.Errorf("scratch-unavailable")
		}
	}
	program := filepath.Join(scratch, "adapter")
	if err = os.WriteFile(program, executable, 0700); err != nil {
		return Result{}, nil, fmt.Errorf("scratch-unavailable")
	}
	stdin, _ := json.Marshal(request)
	o := procgroup.Run(ctx, procgroup.Spec{Argv: append([]string{program}, p.Args...), Dir: scratch, Env: []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LANG=C", "LC_ALL=C", "HOME=" + home, "TMPDIR=" + tmp}, Stdin: stdin, Timeout: 30 * time.Second, ShutdownTimeout: 2 * time.Second, InputLimit: MaxBytes, OutputLimit: MaxBytes, StderrLimit: 64 << 10, ObserveDescendants: true})
	if o.Err != nil || o.ExitStatus != 0 || !o.ExitObserved || !o.WaitCompleted || !o.PipesDrained || !o.OwnedProcessGroupCleanup {
		return Result{}, nil, fmt.Errorf("adapter-process-failed")
	}
	var result Result
	if err = Decode(o.Stdout, &result); err != nil {
		return Result{}, nil, err
	}
	if err = validateResult(result, request, f.SourceItem); err != nil {
		return Result{}, nil, err
	}
	result = normalize(result)
	plan, err := connector.Build(ctx, root, f.Connector, p.Connector, result.Input)
	if err != nil {
		return Result{}, nil, err
	}
	if err = connector.ValidatePlan(ctx, root, f.Connector, p.Connector, plan); err != nil {
		return Result{}, nil, err
	}
	ledger := filepath.Join(scratch, "requests.jsonl")
	if err = connector.Record(ctx, root, f.Connector, p.Connector, plan, ledger, author, inputs); err != nil {
		return Result{}, nil, err
	}
	before, err := readBounded(ledger)
	if err != nil {
		return Result{}, nil, err
	}
	if err = connector.Record(ctx, root, f.Connector, p.Connector, plan, ledger, author, inputs); err != nil {
		return Result{}, nil, err
	}
	after, err := readBounded(ledger)
	if err != nil {
		return Result{}, nil, err
	}
	if !bytes.Equal(before, after) {
		return Result{}, nil, fmt.Errorf("recording-not-idempotent")
	}
	return result, after, nil
}
