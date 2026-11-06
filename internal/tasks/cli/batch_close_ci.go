package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CI-derived gate evidence of a gated batch close (TCD-V0-770..773): a
// required gate the policy admits in ciEvidence is satisfied by the green
// run of its required workflow in the --runs export, recorded at the batch
// merge commit, and is never run locally. Only a push run qualifies: a
// merge-queue or pull-request run may be narrowed, while the push run of a
// main commit runs in full or reuses a full run of the same tree (ci.yml,
// AFP-V0-024).
const ciGateEvent = "push"

// ciGate is one ciEvidence gate's planned record for the batch merge: the
// derived canonical taskman-ci-run/0 bytes and run id, or the reason none
// can be derived.
type ciGate struct {
	raw                    []byte
	runID                  string
	path                   string
	code, detail, recovery string
}

// ciGateFor plans the CI evidence of one ciEvidence gate once per batch.
func (b *gatedBatch) ciGateFor(policy *intent.Policy, id string, entry *intent.CIEvidenceGate) *ciGate {
	if c, ok := b.ci[id]; ok {
		return c
	}
	c := &ciGate{}
	b.ci[id] = c
	rec, code, detail := deriveCIRun(b.runs, b.merge, b.repo, entry)
	if code == "" {
		code, detail = transaction.CIRunRefusal(policy, id, rec)
	}
	if code == "" {
		raw, err := rec.Encode()
		if err != nil {
			code, detail = wire.CodeOf(err), prose(err.Error())
		} else {
			c.raw, c.runID = raw, string(rec.RunID)
		}
	}
	if code != "" {
		c.code = code
		c.detail = "required gate " + id + " needs CI evidence at batch merge " + b.merge + ": " + detail
		c.recovery = "export `gh run list --commit " + b.merge + " --json databaseId,headSha,conclusion,url,workflowName,event,updatedAt` as --runs once the push run of " +
			prose(strings.Join(entry.RequiredChecks, ", ")) + " is green, or pass --ci-run FILE; the gate is never run locally"
	}
	return c
}

// deriveCIRun builds the taskman-ci-run/0 record of a ciEvidence gate from
// the --runs export: the highest-databaseId push run at the merge commit of
// the gate's one required workflow, which must have concluded success. It
// returns the refusal code and detail when no record can be derived.
func deriveCIRun(runs []ghRun, merge, repo string, entry *intent.CIEvidenceGate) (*snapshot.CIRun, string, string) {
	if len(entry.RequiredChecks) != 1 {
		return nil, wire.CodePrerequisiteUnsatisfied, "its required checks " + prose(strings.Join(entry.RequiredChecks, ", ")) + " span more than one workflow run; record them with --ci-run FILE"
	}
	workflow := entry.RequiredChecks[0]
	var best *ghRun
	described := false
	for i := range runs {
		r := &runs[i]
		if r.HeadSha != merge {
			continue
		}
		described = described || (r.Event != "" && r.WorkflowName != "")
		if r.Event != ciGateEvent || r.WorkflowName != workflow {
			continue
		}
		if best == nil || r.DatabaseID > best.DatabaseID {
			best = r
		}
	}
	if best == nil {
		if !described {
			return nil, wire.CodePrerequisiteUnsatisfied, "the --runs export names no run at the merge with its event and workflowName"
		}
		return nil, wire.CodePrerequisiteUnsatisfied, "the --runs export holds no " + ciGateEvent + " run of workflow " + prose(workflow) + " at the merge (merge-queue and pull-request runs may be narrowed and never qualify)"
	}
	id := strconv.FormatInt(best.DatabaseID, 10)
	switch best.Conclusion {
	case "success":
	case "":
		return nil, wire.CodePrerequisiteUnsatisfied, "run " + id + " has not completed"
	default:
		return nil, wire.CodeGateFailed, "run " + id + " concluded " + prose(best.Conclusion)
	}
	at, err := wire.ParseTimestamp("updatedAt", best.UpdatedAt)
	if err != nil {
		return nil, wire.CodePrerequisiteUnsatisfied, "run " + id + " has no YYYY-MM-DDTHH:MM:SSZ updatedAt in the export"
	}
	name := repo
	if m := ghRunRepo.FindStringSubmatch(best.URL); m != nil {
		name = strings.TrimPrefix(m[1], "https://github.com/")
	}
	if name == "" {
		return nil, wire.CodePrerequisiteUnsatisfied, "run " + id + " has no https url in the export; export `url` or pass --repo OWNER/NAME"
	}
	return &snapshot.CIRun{Repository: name, RunID: wire.Size(id), HeadSha: merge, Conclusion: "success", PassedChecks: []string{workflow}, CompletedAt: at}, "", ""
}

// ciRunFile is the --ci-run file of a planned CI gate: the --ci-run flag,
// else the derived record written once into the batch's private directory.
func (b *gatedBatch) ciRunFile(id string) (string, error) {
	c := b.ci[id]
	if c == nil {
		return b.ciRun, nil
	}
	if c.path != "" {
		return c.path, nil
	}
	if b.ciDir == "" {
		dir, err := os.MkdirTemp("", "corvint-batch-close-ci-")
		if err != nil {
			return "", wire.Errorf(wire.CodeUnsupportedFilesystem, "--runs", "cannot create the CI run record directory: %s", prose(err.Error()))
		}
		// The --ci-run reader refuses symlinked ancestry, and the default
		// macOS temporary directory lives under the /var -> /private/var link.
		resolved, err := filepath.EvalSymlinks(dir)
		if err != nil {
			_ = os.RemoveAll(dir)
			return "", wire.Errorf(wire.CodeUnsupportedFilesystem, "--runs", "cannot resolve the CI run record directory: %s", prose(err.Error()))
		}
		b.ciDir = resolved
	}
	path := filepath.Join(b.ciDir, "ci-run-"+strconv.Itoa(len(b.ciFiles))+".json")
	if err := os.WriteFile(path, c.raw, 0o600); err != nil {
		return "", wire.Errorf(wire.CodeUnsupportedFilesystem, "--runs", "cannot write the CI run record: %s", prose(err.Error()))
	}
	b.ciFiles = append(b.ciFiles, path)
	c.path = path
	return path, nil
}

// cleanup removes the derived CI run records; the store keeps each one it
// accepted as evidence/<sha256>.
func (b *gatedBatch) cleanup() {
	if b != nil && b.ciDir != "" {
		_ = os.RemoveAll(b.ciDir)
		b.ciDir = ""
	}
}
