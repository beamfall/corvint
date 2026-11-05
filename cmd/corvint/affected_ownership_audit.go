package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// affectedOwnershipAuditProfile names the `affected --ownership-audit` report.
const affectedOwnershipAuditProfile = "affected-ownership-audit/0"

// affectedOwnershipAuditReport lists the tracked paths at a commit that no
// plugin, the driver claimant or a project declaration claims (AZU-V0-002).
type affectedOwnershipAuditReport struct {
	Profile  string   `json:"profile"`
	Revision string   `json:"revision"`
	Tracked  int      `json:"tracked"`
	Unowned  []string `json:"unowned"`
	Mutates  bool     `json:"mutates"`
	OK       bool     `json:"ok"`
}

// parseAffectedOwnershipAuditOptions accepts `--ownership-audit` alone. The
// audit builds the graph without the cache, so it reads and writes nothing
// outside the repository's own files (AGENTS invariant 4).
func parseAffectedOwnershipAuditOptions(rest []string) (affectedInvocation, error) {
	if len(rest) != 0 {
		return affectedInvocation{}, argumentError("affected --ownership-audit takes no other argument: " + rest[0])
	}
	return affectedInvocation{OwnershipAudit: true, NoCache: true}, nil
}

// runAffectedOwnershipAudit writes the report and exits 0 when every tracked
// path is claimed, 1 when some path is unowned, and 2 on an error.
func runAffectedOwnershipAudit(ctx context.Context, invocation affectedInvocation, stdout, stderr io.Writer) int {
	report, err := compileAffectedOwnershipAudit(ctx, invocation)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	report.OK = len(report.Unowned) == 0
	if err := gokernel.WriteCanonicalJSON(stdout, report); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write affected ownership audit"})
		return 2
	}
	if !report.OK {
		return 1
	}
	return 0
}

// compileAffectedOwnershipAudit audits HEAD of a clean worktree: the graph is
// built from the worktree, which then holds exactly the commit's tracked files.
func compileAffectedOwnershipAudit(ctx context.Context, invocation affectedInvocation) (affectedOwnershipAuditReport, error) {
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return affectedOwnershipAuditReport{}, affectedGitExecutableRefusal()
	}
	observation, err := affectedObserve(ctx, gitExecutable, invocation.Root)
	if err != nil {
		return affectedOwnershipAuditReport{}, err
	}
	if len(observation.dirty) != 0 {
		return affectedOwnershipAuditReport{}, &gokernel.Error{Code: "unsupported-affected-mixed-worktree", Message: fmt.Sprintf("MIXED worktree: --ownership-audit audits a commit and refuses a dirty worktree; HEAD is commit %s with %d dirty worktree paths; %s", observation.revision, len(observation.dirty), affectedDirtyDetail(observation.dirty))}
	}
	deadline, cancel := context.WithTimeout(ctx, affectedRevisionDeadline)
	listing, err := hermeticGitCommand(deadline, gitExecutable, invocation.Root, "ls-tree", "-r", "-z", "--name-only", "--full-tree", observation.revision).Output()
	cancel()
	if err != nil {
		return affectedOwnershipAuditReport{}, &gokernel.Error{Code: "unsupported-affected-status", Message: "cannot list the tracked paths of " + observation.revision + ": " + err.Error()}
	}
	tracked := []string{}
	for _, name := range strings.Split(string(listing), "\x00") {
		if name != "" {
			tracked = append(tracked, name)
		}
	}
	graph, err := affectedBuildGraph(affectedGraphContext(ctx), invocation)
	if err != nil {
		return affectedOwnershipAuditReport{}, err
	}
	return affectedOwnershipAuditReport{
		Profile:  affectedOwnershipAuditProfile,
		Revision: observation.revision,
		Tracked:  len(tracked),
		Unowned:  graph.OwnershipAudit(tracked),
	}, nil
}
