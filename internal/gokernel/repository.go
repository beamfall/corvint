package gokernel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/projectprofile"
)

const (
	gitDeadline        = 10 * time.Second
	maxGitStatusBytes  = 8 * 1024 * 1024
	maxGitErrorBytes   = 64 * 1024
	maxGitIdentityByte = 4 * 1024
	maxDirtyPaths      = 100_000
)

type Repository struct {
	CommitRevision string
	DirtyPathCount int
	DirtyPathsSHA  string
	ObjectFormat   string
	ProfileID      string
	TreeRevision   string
	WorktreeState  string
}

type gitRunner func(context.Context, string, int, ...string) ([]byte, error)

func (repository Repository) wire() map[string]any {
	return map[string]any{
		"commitRevision":   repository.CommitRevision,
		"dirtyPathCount":   repository.DirtyPathCount,
		"dirtyPathsSha256": repository.DirtyPathsSHA,
		"objectFormat":     repository.ObjectFormat,
		"treeRevision":     repository.TreeRevision,
		"worktreeState":    repository.WorktreeState,
	}
}

type boundedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (buffer *boundedBuffer) Write(value []byte) (int, error) {
	remaining := buffer.limit - buffer.Len()
	if remaining <= 0 {
		buffer.exceeded = true
		return len(value), nil
	}
	if len(value) > remaining {
		_, _ = buffer.Buffer.Write(value[:remaining])
		buffer.exceeded = true
		return len(value), nil
	}
	return buffer.Buffer.Write(value)
}

func SanitizedGitEnvironment() []string {
	environment := make([]string, 0, 16)
	for _, name := range []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"} {
		if value, exists := os.LookupEnv(name); exists {
			environment = append(environment, name+"="+value)
		}
	}
	return append(environment,
		"LANG=C", "LC_ALL=C",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1",
		"GCM_INTERACTIVE=never", "GIT_ASKPASS=",
	)
}

func git(ctx context.Context, root string, outputLimit int, arguments ...string) ([]byte, error) {
	if len(arguments) > 0 && arguments[0] == "status" {
		result, err := gitstatus.Status(ctx, root, outputLimit, gitRaw, arguments...)
		if err != nil {
			if ctx.Err() != nil {
				return nil, probeContextError(ctx)
			}
			var failure *Error
			if errors.As(err, &failure) {
				return nil, err
			}
			refused := newError("repository-probe-failed", gitstatus.RefusalMessage(err))
			refused.ReasonClass = gitstatus.RefusalClass(err)
			return nil, refused
		}
		return result, nil
	}
	return gitRaw(ctx, root, outputLimit, arguments...)
}

func gitRaw(ctx context.Context, root string, outputLimit int, arguments ...string) ([]byte, error) {
	commandArguments := []string{
		"--no-optional-locks",
		"-c", "core.fsmonitor=false",
		"-c", "core.untrackedCache=false",
		"-c", "core.excludesFile=",
		"-c", "credential.helper=",
		"-c", "submodule.recurse=false",
		"-C", root,
	}
	commandArguments = append(commandArguments, arguments...)
	command := exec.CommandContext(ctx, gitstatus.Executable(), commandArguments...)
	command.Env = SanitizedGitEnvironment()
	if gitstatus.Isolated(ctx) {
		command.Env = append(command.Env, "GIT_CEILING_DIRECTORIES="+filepath.Dir(root))
	}
	configureProcess(command)
	stdout := &boundedBuffer{limit: outputLimit}
	stderr := &boundedBuffer{limit: maxGitErrorBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		if ctx.Err() != nil {
			return nil, probeContextError(ctx)
		}
		return nil, newError("repository-probe-failed", "cannot start Git")
	}
	// waitGroupLeader signals the process group before it reaps the leader, so
	// the group ID cannot name a process that reused the leader's PID.
	err := waitGroupLeader(command)
	if ctx.Err() != nil {
		return nil, probeContextError(ctx)
	}
	if stdout.exceeded {
		return nil, newError("repository-probe-too-large", "Git output exceeds its byte limit")
	}
	if stderr.exceeded {
		return nil, newError("repository-probe-too-large", "Git error exceeds its byte limit")
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = "git command failed"
		}
		return nil, newError("repository-probe-failed", "Git error: "+detail)
	}
	return stdout.Bytes(), nil
}

func probeContextError(ctx context.Context) *Error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return newError("repository-probe-timeout", "Git repository probe exceeded its 10-second deadline")
	}
	return newError("repository-probe-cancelled", "Git repository probe was cancelled")
}

func repositoryIdentity(ctx context.Context, run gitRunner, root string) ([3]string, error) {
	var identity [3]string
	raw, err := run(ctx, root, maxGitIdentityByte, "rev-parse", "--show-object-format", "HEAD", "HEAD^{tree}")
	if err != nil {
		return identity, err
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) != 3 {
		return identity, newError("repository-identity-malformed", "Git object identity is malformed")
	}
	copy(identity[:], lines)
	length := 0
	switch identity[0] {
	case "sha1":
		length = 40
	case "sha256":
		length = 64
	default:
		format := identity[0]
		if format == "" {
			format = "<empty>"
		}
		return identity, newError("unsupported-git-object-format", "unsupported Git object format: "+format)
	}
	oid := regexp.MustCompile(fmt.Sprintf(`^[0-9a-f]{%d}$`, length))
	if !oid.MatchString(identity[1]) || !oid.MatchString(identity[2]) {
		return identity, newError("repository-identity-malformed", "Git returned an invalid object identity")
	}
	return identity, nil
}

func dirtyPaths(ctx context.Context, run gitRunner, root string) ([]string, []byte, error) {
	raw, err := run(ctx, root, maxGitStatusBytes, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, nil, err
	}
	paths, err := parseDirtyPaths(raw)
	return paths, raw, err
}

func parseDirtyPaths(raw []byte) ([]string, error) {
	paths := make(map[string]struct{})
	for offset := 0; offset < len(raw); {
		field, next, ok := nextNULTerminated(raw, offset)
		if !ok {
			return nil, newError("repository-status-malformed", "Git status output is malformed")
		}
		offset = next
		if len(field) <= 3 || field[2] != ' ' {
			return nil, newError("repository-status-malformed", "Git status output is malformed")
		}
		if !utf8.Valid(field[3:]) {
			return nil, newError("repository-status-malformed", "Git status path is not valid UTF-8")
		}
		paths[string(field[3:])] = struct{}{}
		code := field[:2]
		if bytes.ContainsRune(code, 'R') || bytes.ContainsRune(code, 'C') {
			source, nextSource, ok := nextNULTerminated(raw, offset)
			if !ok || len(source) == 0 {
				return nil, newError("repository-status-malformed", "Git status output contains an empty path")
			}
			offset = nextSource
			if !utf8.Valid(source) {
				return nil, newError("repository-status-malformed", "Git status path is not valid UTF-8")
			}
			paths[string(source)] = struct{}{}
		}
		if len(paths) > maxDirtyPaths {
			return nil, newError("repository-status-too-large", fmt.Sprintf("Git status exceeds the %d-path limit", maxDirtyPaths))
		}
	}
	result := make([]string, 0, len(paths))
	for path := range paths {
		result = append(result, path)
	}
	sort.Strings(result)
	return result, nil
}

func nextNULTerminated(raw []byte, offset int) ([]byte, int, bool) {
	if offset >= len(raw) {
		return nil, offset, false
	}
	relative := bytes.IndexByte(raw[offset:], 0)
	if relative < 0 {
		return nil, offset, false
	}
	end := offset + relative
	return raw[offset:end], end + 1, true
}

func repositoryProfile(ctx context.Context, run gitRunner, root, treeRevision string) (string, error) {
	arguments := append([]string{"ls-tree", "-z", "--name-only", treeRevision, "--"}, projectprofile.Signals()...)
	raw, err := run(ctx, root, maxGitIdentityByte, arguments...)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(raw) {
		return "", newError("repository-profile-malformed", "Git profile path is not valid UTF-8")
	}
	paths := make(map[string]struct{})
	for _, rawPath := range bytes.Split(raw, []byte{0}) {
		if len(rawPath) != 0 {
			paths[string(rawPath)] = struct{}{}
		}
	}
	return projectprofile.Detect(func(path string) bool {
		_, present := paths[path]
		return present
	}).ID, nil
}

func ProbeRepository(root string) (Repository, error) {
	return ProbeRepositoryContext(context.Background(), root)
}

func ProbeRepositoryContext(parent context.Context, root string) (Repository, error) {
	return probeRepositoryContext(parent, root, git)
}

// repositoryProbe is one ProbeRepositoryContext still in flight.
type repositoryProbe struct {
	done       chan struct{}
	repository Repository
	err        error
}

// probeRepositoryConcurrently starts the probe and returns before it finishes.
// An index-backed event's context block reads the repository through its own
// GPK-V0-007 bracket and consumes nothing the probe produces, so the two
// brackets are independent observations of one read-only operation and may
// overlap; sequentially, a user-prompt paid the probe's four Git processes and
// its profile read before the index build could start its own. Both brackets
// still span every read either of them issues, so each still witnesses a change
// the other could have caused. Nothing about the result changes: Event reports
// the probe's failure before the context block's, exactly as the sequential
// form did, and an event whose block needs the probe's own fields blocks on it.
func probeRepositoryConcurrently(ctx context.Context, root string) *repositoryProbe {
	probe := &repositoryProbe{done: make(chan struct{})}
	go func() {
		defer close(probe.done)
		probe.repository, probe.err = ProbeRepositoryContext(ctx, root)
	}()
	return probe
}

// result blocks until the probe finishes and reports what it observed.
func (probe *repositoryProbe) result() (Repository, error) {
	<-probe.done
	return probe.repository, probe.err
}

// repositoryObservation is one side of the GPK-V0-007 nonmutation bracket: the
// repository identity and the dirty path set, sampled as a single instant.
type repositoryObservation struct {
	identity  [3]string
	dirty     []string
	statusRaw []byte
}

// observeRepository takes one side of the bracket. The identity read and the
// status read are independent observations of the same instant, so they are
// issued concurrently; neither depends on the other's bytes. Their errors are
// still reported in the oracle's order — identity first — so a failing probe
// reports the same typed error it always did.
func observeRepository(ctx context.Context, run gitRunner, root string) (repositoryObservation, error) {
	var dirty []string
	var statusRaw []byte
	var dirtyErr error
	var pending sync.WaitGroup
	pending.Add(1)
	go func() {
		defer pending.Done()
		dirty, statusRaw, dirtyErr = dirtyPaths(ctx, run, root)
	}()
	identity, identityErr := repositoryIdentity(ctx, run, root)
	pending.Wait()
	if identityErr != nil {
		return repositoryObservation{}, identityErr
	}
	if dirtyErr != nil {
		return repositoryObservation{}, dirtyErr
	}
	return repositoryObservation{identity: identity, dirty: dirty, statusRaw: statusRaw}, nil
}

func probeRepositoryContext(parent context.Context, root string, run gitRunner) (Repository, error) {
	ctx, cancel := context.WithTimeout(parent, gitDeadline)
	defer cancel()
	// The three stages below stay strictly sequential: the profile read is
	// issued only after a complete observation, and the second complete
	// observation is taken only after it returns. That ordering is a
	// receipt-stability check, not the GPK-V0-007 nonmutation proof itself —
	// the comparison is identity plus the dirty-path set, so a status-code
	// or content change to an already-dirty path would not show up in it.
	// Byte-for-byte nonmutation is proven separately by the cli-parity
	// snapshot (conformance/cli-parity-v0/snapshot.go:35). Only the two
	// reads *within* one observation overlap.
	for attempt := 0; attempt < 3; attempt++ {
		before, err := observeRepository(ctx, run, root)
		if err != nil {
			return Repository{}, err
		}
		profileID, err := repositoryProfile(ctx, run, root, before.identity[2])
		if err != nil {
			return Repository{}, err
		}
		after, err := observeRepository(ctx, run, root)
		if err != nil {
			return Repository{}, err
		}
		if before.identity != after.identity || !slices.Equal(before.dirty, after.dirty) {
			continue
		}
		return repositoryFromObservation(after, profileID)
	}
	return Repository{}, newError("repository-state-unstable", "repository HEAD changed while probing repository state")
}

// finishConcurrently starts the read stage's continuation, if any, and
// returns the channel that carries its result once. The bracket always
// receives from it before comparing, so a retry never overlaps two of them.
func finishConcurrently(finish func() error) <-chan error {
	finished := make(chan error, 1)
	if finish == nil {
		finished <- nil
		return finished
	}
	go func() { finished <- finish() }()
	return finished
}

func repositoryFromObservation(after repositoryObservation, profileID string) (Repository, error) {
	dirtyAfter := after.dirty
	dirtyJSON, err := CanonicalJSON(dirtyAfter)
	if err != nil {
		return Repository{}, wrapError("canonical-json-failed", "cannot encode dirty paths", err)
	}
	state := "clean"
	if len(dirtyAfter) != 0 {
		state = "mixed"
	}
	return Repository{
		CommitRevision: after.identity[1], DirtyPathCount: len(dirtyAfter),
		DirtyPathsSHA: sha256Hex(dirtyJSON), ObjectFormat: after.identity[0],
		ProfileID: profileID, TreeRevision: after.identity[2], WorktreeState: state,
	}, nil
}

// Observation is the bracket's opening observation as handed to a shared
// reader: the identity that names an index snapshot and the dirty set a
// snapshot hit applies (IDX-SNAP-V0-010). StatusSHA256 digests the raw status
// bytes exactly as the index loader's own status read would.
type Observation struct {
	ObjectFormat, CommitRevision, TreeRevision string
	DirtyPaths                                 []string
	StatusSHA256                               string
}

func (observation repositoryObservation) exported() Observation {
	return Observation{
		ObjectFormat: observation.identity[0], CommitRevision: observation.identity[1],
		TreeRevision: observation.identity[2], DirtyPaths: slices.Clone(observation.dirty),
		StatusSHA256: sha256Hex(observation.statusRaw),
	}
}

// sharedRead is the read stage of a shared bracket: the event's own reads,
// issued against the opening observation. It returns the computation that
// finishes the event from what was read, or nil. That computation touches no
// repository state -- it is arithmetic over memory -- so it is not part of the
// read the bracket proves and runs concurrently with the closing observation.
type sharedRead func(context.Context, Observation) (func() error, error)

// probeRepositorySharing is probeRepositoryContext with the event's own read
// placed in the bracket's read stage (CORVINT_HARNESS_SHARED_OBSERVATION=1,
// proposed GPK-V0-058). The proof shape is unchanged: a complete observation,
// the read, a complete observation, equal or retry. What changes is that the
// read consumes the opening observation instead of re-reading identity and
// status itself, and the `ls-tree` profile read is issued only when the
// caller emits the profile (wantProfile): Repository.wire omits it, so for
// every other event the spawn produced a value no output carried. A retry
// re-runs the read against the new opening observation. The read's error is
// held until the closing observation so the bracket's own failures keep their
// precedence over the context block's, as the concurrent default path reports.
func probeRepositorySharing(parent context.Context, root string, run gitRunner, read sharedRead, wantProfile bool) (Repository, error) {
	ctx, cancel := context.WithTimeout(parent, gitDeadline)
	defer cancel()
	for attempt := 0; attempt < 3; attempt++ {
		before, err := observeRepository(ctx, run, root)
		if err != nil {
			return Repository{}, err
		}
		finish, readErr := read(ctx, before.exported())
		profileID := ""
		if wantProfile {
			profileID, err = repositoryProfile(ctx, run, root, before.identity[2])
			if err != nil {
				return Repository{}, err
			}
		}
		finished := finishConcurrently(finish)
		after, err := observeRepository(ctx, run, root)
		finishErr := <-finished
		if err != nil {
			return Repository{}, err
		}
		if before.identity != after.identity || !slices.Equal(before.dirty, after.dirty) {
			continue
		}
		if readErr != nil {
			return Repository{}, readErr
		}
		if finishErr != nil {
			return Repository{}, finishErr
		}
		return repositoryFromObservation(after, profileID)
	}
	return Repository{}, newError("repository-state-unstable", "repository HEAD changed while probing repository state")
}
