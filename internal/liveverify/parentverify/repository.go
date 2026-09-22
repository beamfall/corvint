package parentverify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

var gitObjectID = regexp.MustCompile(`^[0-9a-f]+$`)

// observeRepository runs one repository observation and returns both the
// hashed commitments and the dirty path list decoded from the very same
// `git status` bytes the StatusSHA256 commitment covers.
//
// The path list is returned rather than stored on Repository on purpose.
// Repository is marshalled into the workspace-source body that yields the
// source observation identity, and it is compared with == to detect drift
// across the two observations that bracket a snapshot. A new field would
// change canonical bytes and a slice field would not compile.
func observeRepository(ctx context.Context, config Config, run CommandRunner) (Repository, []string, error) {
	gitDigest, _, err := stableExecutableFile(config.GitExecutable)
	if err != nil || gitDigest != config.GitExecutableSHA256 {
		return Repository{}, nil, errors.Join(fmt.Errorf("Git executable: %w", ErrDrift), err)
	}
	environment := []string{
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1", "GCM_INTERACTIVE=never", "GIT_ASKPASS=",
		"LANG=C", "LC_ALL=C", "TMPDIR=" + config.TemporaryParent,
	}
	base := []string{"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "-c", "core.excludesFile=", "-c", "credential.helper=", "-c", "submodule.recurse=false"}
	runRaw := func(ctx context.Context, root string, limit int, tail ...string) ([]byte, error) {
		argv := append(append([]string(nil), base...), "-C", root)
		argv = append(argv, tail...)
		childEnvironment := environment
		if gitstatus.Isolated(ctx) {
			childEnvironment = append(append([]string(nil), environment...), "GIT_CEILING_DIRECTORIES="+filepath.Dir(root))
		}
		result, runErr := run(ctx, config.GitExecutable, argv, childEnvironment, root, config.Timeout)
		if runErr != nil || !successfulCommand(result) || len(result.Stderr) != 0 || len(result.Stdout) > min(limit, int(config.OutputLimitBytes)) {
			return nil, errors.Join(fmt.Errorf("git %s: %w", tail[0], ErrCommand), runErr)
		}
		return result.Stdout, nil
	}
	runGit := func(tail ...string) ([]byte, error) {
		if tail[0] == "status" {
			raw, err := gitstatus.StatusIn(ctx, config.RepositoryRoot, config.TemporaryParent, int(config.OutputLimitBytes), runRaw, tail...)
			if err != nil {
				return nil, errors.Join(fmt.Errorf("git status: %w", ErrCommand), err)
			}
			return raw, nil
		}
		return runRaw(ctx, config.RepositoryRoot, int(config.OutputLimitBytes), tail...)
	}
	identityRaw, err := runGit("rev-parse", "--show-object-format", "HEAD", "HEAD^{tree}")
	if err != nil {
		return Repository{}, nil, err
	}
	identity := strings.Split(strings.TrimSuffix(string(identityRaw), "\n"), "\n")
	if len(identity) != 3 || identity[0] != "sha1" && identity[0] != "sha256" {
		return Repository{}, nil, ErrUnavailable
	}
	oidLength := 40
	if identity[0] == "sha256" {
		oidLength = 64
	}
	if len(identity[1]) != oidLength || len(identity[2]) != oidLength || !gitObjectID.MatchString(identity[1]) || !gitObjectID.MatchString(identity[2]) {
		return Repository{}, nil, ErrUnavailable
	}
	layoutRaw, err := runGit("rev-parse", "--path-format=absolute", "--show-toplevel", "--git-dir", "--git-common-dir")
	if err != nil {
		return Repository{}, nil, err
	}
	layout := strings.Split(strings.TrimSuffix(string(layoutRaw), "\n"), "\n")
	if len(layout) != 3 || filepath.Clean(layout[0]) != config.RepositoryRoot || !cleanAbsolute(layout[1]) || !cleanAbsolute(layout[2]) {
		return Repository{}, nil, ErrUnavailable
	}
	indexRaw, err := runGit("ls-files", "--stage", "-z")
	if err != nil {
		return Repository{}, nil, err
	}
	statusRaw, err := runGit("status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=no")
	if err != nil {
		return Repository{}, nil, err
	}
	configRaw, err := runGit("config", "--local", "--null", "--list")
	if err != nil {
		return Repository{}, nil, err
	}
	versionRaw, err := runGit("version")
	if err != nil || !bytes.HasPrefix(versionRaw, []byte("git version ")) || !bytes.HasSuffix(versionRaw, []byte("\n")) {
		return Repository{}, nil, ErrUnavailable
	}
	dirtyPaths, err := affected.DecodeStatus(statusRaw)
	if err != nil {
		return Repository{}, nil, errors.Join(fmt.Errorf("git status: %w", ErrUnavailable), err)
	}
	return Repository{
		ConfigSHA256: hashRaw(configRaw), GitExeSHA256: gitDigest, GitVersionSHA256: hashRaw(versionRaw),
		HeadRevision: identity[1], IndexSHA256: hashRaw(indexRaw), LayoutSHA256: hashRaw(layoutRaw),
		ObjectFormat: identity[0], StatusSHA256: hashRaw(statusRaw), TreeRevision: identity[2],
	}, dirtyPaths, nil
}

func hashRaw(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
