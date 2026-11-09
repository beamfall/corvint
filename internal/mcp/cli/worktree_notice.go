package cli

import (
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
)

// worktreeNoticeSchema is the informational launch-worktree notice (MMR-V0-009).
const worktreeNoticeSchema = "corvint-mcp-worktree-notice/0"

// worktreeNoticeText is the closed, path-free notice text. It is server-authored,
// so it travels outside the repository-data envelope.
var worktreeNoticeText = func() string {
	raw, err := gokernel.CanonicalJSON(map[string]any{
		"schema": worktreeNoticeSchema, "code": "launch-worktree-differs", "mutates": false,
		"message": "This server was launched from a different linked worktree of the same repository than the --root it is bound to. " +
			"Results describe the bound --root only; the launch worktree is not read and selects nothing.",
	})
	if err != nil {
		panic(err)
	}
	return string(raw)
}()

// launchWorktreeNotices observes the process's launch directory once, at
// startup, and returns the notice for each binding whose root is a different
// linked worktree of the repository the launch directory belongs to. The launch
// directory is local process state, never request data, and it changes no
// routing or authority (MCPV0-015). Anything unobserved yields no notice.
func launchWorktreeNotices(launch string, bindings map[*bridge.Registry]string) map[*bridge.Registry]string {
	top, ok := enclosingWorktree(launch)
	if !ok {
		return nil
	}
	notices := map[*bridge.Registry]string{}
	for registry, root := range bindings {
		if differentLinkedWorktree(top, root) {
			notices[registry] = worktreeNoticeText
		}
	}
	return notices
}

// enclosingWorktree returns the nearest ancestor of directory, itself included,
// holding a .git directory or gitfile; it never runs Git.
func enclosingWorktree(directory string) (string, bool) {
	if directory == "" {
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil || !filepath.IsAbs(resolved) {
		return "", false
	}
	for current := resolved; ; {
		if info, err := os.Lstat(filepath.Join(current, ".git")); err == nil && (info.IsDir() || info.Mode().IsRegular()) {
			return current, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

// differentLinkedWorktree reports whether top and root are distinct worktree
// directories sharing one Git common directory.
func differentLinkedWorktree(top, root string) bool {
	topInfo, topErr := os.Stat(top)
	rootInfo, rootErr := os.Stat(root)
	if topErr != nil || rootErr != nil || os.SameFile(topInfo, rootInfo) {
		return false
	}
	_, topCommon, topErr := gitauth.WorktreeDirectories(top)
	_, rootCommon, rootErr := gitauth.WorktreeDirectories(root)
	if topErr != nil || rootErr != nil {
		return false
	}
	topCommonInfo, topErr := os.Stat(topCommon)
	rootCommonInfo, rootErr := os.Stat(rootCommon)
	return topErr == nil && rootErr == nil && os.SameFile(topCommonInfo, rootCommonInfo)
}

// withWorktreeNotice returns a wrapper that appends the binding's notice as a
// second text content block to a tool result or tool error; structuredContent
// and the first block are unchanged, and protocol errors pass through.
func (handler *toolHandler) withWorktreeNotice(registry *bridge.Registry) func(map[string]any, *protocol.RPCError) (map[string]any, *protocol.RPCError) {
	return func(response map[string]any, failure *protocol.RPCError) (map[string]any, *protocol.RPCError) {
		notice := handler.notices[registry]
		if notice == "" || response == nil {
			return response, failure
		}
		content, _ := response["content"].([]any)
		response["content"] = append(content, map[string]any{"type": "text", "text": notice})
		return response, failure
	}
}
