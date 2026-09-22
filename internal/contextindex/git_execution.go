package contextindex

import (
	"context"
	"path/filepath"
	"runtime"
)

type gitExecutionKey struct{}

type gitExecution struct {
	executable  string
	environment []string
}

// BuildWithGitExecution builds the same immutable evidence as Build using a
// caller-qualified absolute Git executable and closed environment. Qualification
// of that executable, environment and repository belongs to the caller. This
// request reads source bytes only from immutable Git objects, never a worktree
// copy or persisted index, and never falls back to ambient Git settings.
func BuildWithGitExecution(ctx context.Context, root, executable string, environment []string) (*Index, error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return nil, &Error{Message: "qualified Git execution is unsupported on this platform"}
	}
	if !filepath.IsAbs(executable) {
		return nil, &Error{Message: "qualified Git execution requires an absolute executable"}
	}
	if environment == nil {
		return nil, &Error{Message: "qualified Git execution requires a closed environment"}
	}
	return Build(withGitExecution(ctx, executable, environment), root)
}

func withGitExecution(ctx context.Context, executable string, environment []string) context.Context {
	owned := make([]string, len(environment))
	copy(owned, environment)
	return context.WithValue(ctx, gitExecutionKey{}, gitExecution{executable, owned})
}

func pinGitObjects(ctx context.Context, root string, entries []treeEntry) ([]pinnedEntry, error) {
	blobs, err := readBlobs(ctx, root, entries)
	if err != nil {
		return nil, err
	}
	pinned := make([]pinnedEntry, len(entries))
	for position, entry := range entries {
		pinned[position] = pinnedFrom(entry, blobs[entry.oid])
	}
	return pinned, nil
}
