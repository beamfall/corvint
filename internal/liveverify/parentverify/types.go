// Package parentverify implements the local parent authority required by the
// experimental Go live-test producer. It is deliberately separate from the
// producer: every operation recomputes the source, dependency, and toolchain
// commitments and a lease is only an equality witness over those commitments.
package parentverify

import (
	"context"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/godiscovery"
	"github.com/Beamfall/corvint/internal/liveverify/provider"
)

const (
	ProviderVersion = "0.1.0-experimental"
	MaxTreeEntries  = 1_000_000
	MaxTreeBytes    = uint64(8 << 30)
)

type Config struct {
	VerifierExecutable       string
	VerifierExecutableSHA256 string
	GoExecutable             string
	GitExecutable            string
	GitExecutableSHA256      string
	GoWorkPath               string
	GOARCH                   string
	GOOS                     string
	GOROOT                   string
	ModuleCacheDirectory     string
	ModuleMode               provider.ModuleMode
	OutputLimitBytes         int64
	Packages                 []string
	RepositoryRoot           string
	TemporaryParent          string
	Timeout                  time.Duration
}

type CommandResult struct {
	Stdout, Stderr      []byte
	ExitCode            int
	Started, Exited     bool
	Cancelled, TimedOut bool
	OutputLimitExceeded bool
	ProcessCleanupDone  bool
	PipesDrained        bool
	WaitCompleted       bool
}

type CommandRunner func(context.Context, string, []string, []string, string, time.Duration) (CommandResult, error)

type Snapshot struct {
	Binding               provider.AuthorityBinding
	SourceMaterialization string
	SourceObservation     string
	ToolchainObservation  string
	LeaseID               string
	Toolchain             Toolchain
	Repository            Repository
	// DirtyPaths is the repository-relative path set decoded from the exact
	// `git status` bytes Repository.StatusSHA256 commits to, in the second of
	// the two bracketing observations — the one whose Repository is marshalled
	// into the source observation. The drift refusal above it proves the first
	// observation saw the same bytes, so this list is not a third look at a
	// mutable worktree.
	//
	// Snapshot is never marshalled and never compared with ==, so publishing
	// the list here leaves every canonical form and receipt digest unchanged.
	DirtyPaths []string
}

type Repository struct {
	ConfigSHA256     string `json:"configSha256"`
	GitExeSHA256     string `json:"gitExeSha256"`
	GitVersionSHA256 string `json:"gitVersionSha256"`
	HeadRevision     string `json:"headRevision"`
	IndexSHA256      string `json:"indexSha256"`
	LayoutSHA256     string `json:"layoutSha256"`
	ObjectFormat     string `json:"objectFormat"`
	StatusSHA256     string `json:"statusSha256"`
	TreeRevision     string `json:"treeRevision"`
}

type Toolchain struct {
	CGOEnabled         string `json:"cgoEnabled"`
	GOARCH             string `json:"goarch"`
	GoEnvSHA256        string `json:"goenvSha256"`
	GoExeSHA256        string `json:"goexeSha256"`
	GOOS               string `json:"goos"`
	GOROOTSHA256       string `json:"gorootSha256"`
	GoVersion          string `json:"goversion"`
	ID                 string `json:"id"`
	InvokedToolsSHA256 string `json:"invokedToolsSha256"`
	PathSHA256         string `json:"pathSha256"`
	ToolDirSHA256      string `json:"toolDirSha256"`
}

func Acquire(ctx context.Context, config Config, request provider.AuthorityRequest, run CommandRunner) (Snapshot, error) {
	if err := validateAuthorityRequest(config, request); err != nil {
		return Snapshot{}, err
	}
	return observe(ctx, config, run)
}

func Revalidate(ctx context.Context, config Config, request provider.AuthorityRequest, leaseID string, run CommandRunner) (Snapshot, error) {
	if err := validateAuthorityRequest(config, request); err != nil {
		return Snapshot{}, err
	}
	snapshot, err := observe(ctx, config, run)
	if err != nil {
		return Snapshot{}, err
	}
	if snapshot.LeaseID != leaseID {
		return Snapshot{}, ErrDrift
	}
	return snapshot, nil
}

type Discovery struct {
	Raw         []byte
	Commitments godiscovery.Commitments
	ID          string
}
