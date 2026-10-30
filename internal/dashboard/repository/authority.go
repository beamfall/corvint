package repository

import (
	"bytes"
	"context"
	"path/filepath"
	"sort"

	dashboardauthority "github.com/Beamfall/corvint/internal/dashboard/authority"
)

const maxSemanticResolutions = 100_000

var (
	discoveryTail = []string{"rev-parse", "--path-format=absolute", "--show-toplevel", "--absolute-git-dir", "--git-common-dir"}
	objectsTail   = []string{"rev-parse", "--path-format=absolute", "--git-path", "objects"}
	identityTail  = []string{"rev-parse", "--show-object-format", "HEAD^{commit}", "HEAD^{tree}"}
	statusTail    = []string{"status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none"}
)

func New(ctx context.Context, root string) (*Authority, dashboardauthority.Snapshot, *Failure) {
	budget := NewBudget(ctx)
	authority, snapshot, failure := newAttemptWithStartup(root, budget, processStartup)
	if failure != nil && failure.Code == FailureRepositoryChanged {
		authority, snapshot, failure = newAttemptWithStartup(root, budget, processStartup)
	}
	if failure != nil {
		budget.Close()
		return nil, dashboardauthority.Snapshot{}, failure
	}
	authority.ownBudget = true
	return authority, snapshot, nil
}

// NewAttempt initializes one repository authority attempt against a cumulative
// Budget. Callers may construct at most one retry with the same Budget.
func NewAttempt(root string, budget *Budget) (*Authority, dashboardauthority.Snapshot, *Failure) {
	return newAttemptWithStartup(root, budget, processStartup)
}

func newAttemptWithStartup(root string, budget *Budget, startup startupState) (*Authority, dashboardauthority.Snapshot, *Failure) {
	if !budget.usable() {
		return nil, dashboardauthority.Snapshot{}, unavailable(ReasonUnavailable)
	}
	if !repositoryExecutionSupported || !validRoot(root) || startup.alternate != "" || startup.objectDir != "" {
		reason := ReasonUnavailable
		if startup.alternate != "" || startup.objectDir != "" {
			reason = ReasonUnsupportedAlternates
		}
		return nil, dashboardauthority.Snapshot{}, unavailable(reason)
	}
	life, cancel := context.WithCancel(budget.context)
	authority := &Authority{
		life: life, cancel: cancel, budget: budget, root: root,
		environment:     startup.childEnvironment(),
		containmentOkay: true,
		objectChecks:    make(map[string]objectCheck),
	}
	fail := func(failure *Failure) (*Authority, dashboardauthority.Snapshot, *Failure) {
		if authority.drifted {
			failure = changed()
		}
		authority.closeLocked()
		return nil, dashboardauthority.Snapshot{}, failure
	}
	executable, failure := startup.resolvedExecutable()
	if failure != nil {
		return fail(failure)
	}
	authority.executable = executable
	executableFile, executableBefore, failure := openExecutableStableRead(life, executable)
	if failure != nil {
		return fail(failure)
	}
	authority.executableFile = executableFile
	authority.executableBefore = executableBefore
	qualifiedLayout, failure := authority.qualifyLayout(life, root)
	if failure != nil {
		return fail(failure)
	}
	authority.layout = qualifiedLayout
	initial, failure := authority.probe(life, qualifiedLayout)
	if failure != nil {
		return fail(failure)
	}
	authority.initial = initial
	authority.snapshot = initial.snapshot
	if budget.reserveObject(initial.snapshot.HeadRevision, "commit") != objectReserved {
		return fail(unavailable(ReasonUnavailable))
	}
	if authority.checkObjects(life, []objectExpectation{{
		id: initial.snapshot.HeadRevision, expectedType: "commit",
	}})[initial.snapshot.HeadRevision] != objectCheckQualified {
		return fail(unavailable(ReasonUnavailable))
	}
	return authority, initial.snapshot, nil
}

func (authority *Authority) Snapshot() dashboardauthority.Snapshot {
	if authority == nil {
		return dashboardauthority.Snapshot{}
	}
	authority.mu.Lock()
	defer authority.mu.Unlock()
	if authority.closed || authority.finished {
		return dashboardauthority.Snapshot{}
	}
	return authority.snapshot
}

func (authority *Authority) Finish(ctx context.Context) dashboardauthority.FinishResult {
	if authority == nil {
		return dashboardauthority.FinishResult{Code: dashboardauthority.FinishUnavailable}
	}
	authority.mu.Lock()
	defer authority.mu.Unlock()
	if authority.finished || authority.closed {
		return dashboardauthority.FinishResult{Code: dashboardauthority.FinishUnavailable}
	}
	authority.finished = true
	defer authority.closeLocked()
	if ctx != nil && ctx.Err() != nil || authority.life.Err() != nil {
		return dashboardauthority.FinishResult{Code: dashboardauthority.FinishUnavailable}
	}
	finishContext := authority.life
	stopCaller := func() bool { return true }
	var cancelFinish context.CancelFunc
	if ctx != nil {
		finishContext, cancelFinish = context.WithCancel(authority.life)
		stopCaller = context.AfterFunc(ctx, cancelFinish)
		defer func() {
			stopCaller()
			cancelFinish()
		}()
	}
	if authority.drifted {
		return dashboardauthority.FinishResult{Code: dashboardauthority.FinishChanged}
	}
	if authority.objectFailed || !authority.containmentOkay {
		return dashboardauthority.FinishResult{Code: dashboardauthority.FinishUnavailable}
	}
	if !retainedLayoutStable(authority.layout) || !retainedLayoutFilesStable(ctx, authority.layout) {
		return dashboardauthority.FinishResult{Code: dashboardauthority.FinishChanged}
	}
	finalLayout, failure := authority.qualifyLayout(ctx, authority.root)
	if failure != nil {
		if failure.Code == FailureRepositoryChanged || authority.drifted {
			return dashboardauthority.FinishResult{Code: dashboardauthority.FinishChanged}
		}
		return dashboardauthority.FinishResult{Code: dashboardauthority.FinishUnavailable}
	}
	defer closeLayout(&finalLayout)
	finalProbe, failure := authority.probe(ctx, finalLayout)
	if failure != nil {
		if failure.Code == FailureRepositoryChanged || authority.drifted {
			return dashboardauthority.FinishResult{Code: dashboardauthority.FinishChanged}
		}
		return dashboardauthority.FinishResult{Code: dashboardauthority.FinishUnavailable}
	}
	executableAfterFile, executableAfter, failure := openExecutableStableRead(finishContext, authority.executable)
	if failure != nil {
		if ctx != nil && ctx.Err() != nil {
			return dashboardauthority.FinishResult{Code: dashboardauthority.FinishUnavailable}
		}
		return dashboardauthority.FinishResult{Code: dashboardauthority.FinishChanged}
	}
	_ = executableAfterFile.Close()
	if executableAfter != authority.executableBefore || finalProbe != authority.initial {
		return dashboardauthority.FinishResult{Code: dashboardauthority.FinishChanged}
	}
	return dashboardauthority.FinishResult{Code: dashboardauthority.FinishStable}
}

func (authority *Authority) Close() {
	if authority == nil {
		return
	}
	authority.mu.Lock()
	defer authority.mu.Unlock()
	authority.finished = true
	authority.closeLocked()
}

func (authority *Authority) closeLocked() {
	if authority.closed {
		return
	}
	authority.closed = true
	closeLayout(&authority.layout)
	if authority.executableFile != nil {
		_ = authority.executableFile.Close()
		authority.executableFile = nil
	}
	if authority.cancel != nil {
		authority.cancel()
	}
	if authority.ownBudget && authority.budget != nil {
		authority.budget.Close()
	}
	authority.executable = ""
	authority.environment = nil
	authority.root = ""
	authority.budget = nil
	authority.objectChecks = nil
}

func (authority *Authority) qualifyLayout(ctx context.Context, root string) (layout, *Failure) {
	bindings, failure := prequalifyLayout(ctx, root)
	if failure != nil {
		return layout{}, failure
	}
	fail := func(reason FailureReason) (layout, *Failure) {
		closeLayout(&bindings)
		return layout{}, unavailable(reason)
	}
	failWith := func(failure *Failure) (layout, *Failure) {
		closeLayout(&bindings)
		return layout{}, failure
	}
	discovery, failure := authority.discovery(ctx, root, nil)
	if failure != nil {
		return fail(ReasonUnavailable)
	}
	matches, matchFailure := discoveryMatches(discovery, bindings)
	if matchFailure != nil {
		return failWith(matchFailure)
	}
	if !matches {
		return fail(ReasonUnsupportedAlternates)
	}
	worktreeDiscovery, failure := authority.discovery(ctx, bindings.worktree.path, nil)
	if failure != nil {
		return fail(ReasonUnsupportedAlternates)
	}
	matches, matchFailure = discoveryMatches(worktreeDiscovery, bindings)
	if matchFailure != nil {
		return failWith(matchFailure)
	}
	if !matches {
		return fail(ReasonUnsupportedAlternates)
	}
	inserted := []string{"--git-dir=.", "--work-tree=" + bindings.worktree.path}
	gitDirectoryDiscovery, failure := authority.discovery(ctx, bindings.gitDir.path, inserted)
	if failure != nil {
		return fail(ReasonUnsupportedAlternates)
	}
	matches, matchFailure = discoveryMatches(gitDirectoryDiscovery, bindings)
	if matchFailure != nil {
		return failWith(matchFailure)
	}
	if !matches {
		return fail(ReasonUnsupportedAlternates)
	}
	objectsResult, failure := authority.run(ctx, bindings.worktree.path, nil, objectsTail)
	if failure != nil || objectsResult.exit != 0 {
		return fail(ReasonUnsupportedAlternates)
	}
	fields, parseErr := strictUTF8LFFields(objectsResult.stdout, 1)
	if parseErr != nil || !validNativeAbsolutePath(fields[0]) {
		return fail(ReasonUnsupportedAlternates)
	}
	objectsBinding, objectFailure := openDirectory(fields[0])
	if objectFailure != nil {
		return failWith(objectFailure)
	}
	if objectsBinding.identity != bindings.objects.identity {
		closeDirectoryBinding(&objectsBinding)
		return failWith(changed())
	}
	closeDirectoryBinding(&objectsBinding)
	return bindings, nil
}

func (authority *Authority) discovery(ctx context.Context, cwd string, inserted []string) ([3]string, *Failure) {
	var empty [3]string
	result, failure := authority.run(ctx, cwd, inserted, discoveryTail)
	if failure != nil || result.exit != 0 {
		return empty, unavailable(ReasonUnavailable)
	}
	parsed, err := parseDiscovery(result.stdout)
	if err != nil {
		return empty, unavailable(ReasonUnavailable)
	}
	return parsed, nil
}

func openLayoutDirectories(paths [3]string) (layout, *Failure) {
	worktree, failure := openDirectory(paths[0])
	if failure != nil {
		return layout{}, failure
	}
	value := layout{worktree: worktree}
	fail := func(failure *Failure) (layout, *Failure) {
		closeLayout(&value)
		return layout{}, failure
	}
	gitDirectory, failure := openDirectory(paths[1])
	if failure != nil {
		return fail(failure)
	}
	value.gitDir = gitDirectory
	common, failure := openDirectory(paths[2])
	if failure != nil {
		return fail(failure)
	}
	value.common = common
	objects, failure := openDirectory(filepath.Join(paths[2], "objects"))
	if failure != nil {
		return fail(failure)
	}
	value.objects = objects
	return value, nil
}

func discoveryMatches(paths [3]string, expected layout) (bool, *Failure) {
	if paths != [3]string{expected.worktree.path, expected.gitDir.path, expected.common.path} {
		return false, nil
	}
	actual, failure := openLayoutDirectories(paths)
	if failure != nil {
		return false, failure
	}
	defer closeLayout(&actual)
	if layoutIdentities(actual) != layoutIdentities(expected) {
		return false, changed()
	}
	return true, nil
}

func layoutIdentities(value layout) [4]fileIdentity {
	return [4]fileIdentity{value.worktree.identity, value.gitDir.identity, value.common.identity, value.objects.identity}
}

func layoutProofOf(value layout) layoutProof {
	return layoutProof{
		worktree: value.worktree.identity, gitEntry: value.gitEntry.identity,
		gitDir: value.gitDir.identity, common: value.common.identity, objects: value.objects.identity,
		pointer: value.pointer, commonFile: value.commonFile, reciprocal: value.reciprocal,
		configCount: value.configCount, configs: value.configs,
		commonWorktreeConfig: value.commonWorktreeConfig, gitWorktreeConfig: value.gitWorktreeConfig,
	}
}

func (authority *Authority) probe(ctx context.Context, qualifiedLayout layout) (probeState, *Failure) {
	identityResult, failure := authority.run(ctx, qualifiedLayout.worktree.path, nil, identityTail)
	if failure != nil || identityResult.exit != 0 {
		return probeState{}, unavailable(ReasonUnavailable)
	}
	format, head, tree, parseErr := parseIdentity(identityResult.stdout)
	if parseErr != nil {
		return probeState{}, unavailable(ReasonUnavailable)
	}
	statusResult, failure := authority.run(ctx, qualifiedLayout.worktree.path, nil, statusTail)
	if failure != nil || statusResult.exit != 0 {
		return probeState{}, unavailable(ReasonUnavailable)
	}
	dirty, parseErr := parseDirtyPaths(statusResult.stdout)
	if parseErr != nil {
		return probeState{}, unavailable(ReasonUnavailable)
	}
	digest, digestErr := dirtyDigest(dirty)
	if digestErr != nil {
		return probeState{}, unavailable(ReasonUnavailable)
	}
	state := dashboardauthority.WorktreeClean
	if len(dirty) != 0 {
		state = dashboardauthority.WorktreeMixed
	}
	snapshot := dashboardauthority.Snapshot{
		DirtyPathCount: uint64(len(dirty)), DirtyPathsSHA256: digest,
		HeadRevision: head, ObjectFormat: format, TreeRevision: tree, WorktreeState: state,
	}
	return probeState{snapshot: snapshot, layout: layoutProofOf(qualifiedLayout)}, nil
}

func sortedUniquePaths(paths []string) ([]string, bool) {
	values := append([]string(nil), paths...)
	for _, value := range values {
		if !validTracePath(value) {
			return nil, false
		}
	}
	sort.Slice(values, func(left, right int) bool {
		return bytes.Compare([]byte(values[left]), []byte(values[right])) < 0
	})
	result := values[:0]
	for index, value := range values {
		if index == 0 || value != values[index-1] {
			result = append(result, value)
		}
	}
	return result, true
}
