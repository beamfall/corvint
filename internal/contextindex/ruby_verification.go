package contextindex

import (
	"context"
	"strings"

	"github.com/Beamfall/corvint/internal/langplugin/decl"
	"github.com/Beamfall/corvint/internal/langplugin/dialect"
)

// rubyManifestBound (analyzer_bounds.go) bounds each committed Gemfile,
// Gemfile.lock or Rakefile read for the Ruby verification command. A larger
// blob is treated as unreadable: it proves nothing, so the command is withheld
// unless the other, readable manifest proves it alone (NGI-V0-052, TSA-V0-461).

// rubyVerificationCommand names the one repository-wide command that runs the
// tests of the constant-reference impact dialect, or "" when the committed tree
// does not prove one. The rules live in the dialect's plugin (LGP-V0-025);
// core supplies only the bounded read-only view of the committed tree. It is
// called only when the caller has a path of that dialect.
func rubyVerificationCommand(index *Index) string {
	for _, name := range decl.Dialects(decl.ImpactDialect) {
		if decl.RuleOf(decl.ImpactDialect, name) != decl.ConstantReferences {
			continue
		}
		if hook, ok := dialect.LookupImpact(name); ok && hook.Verification != nil {
			return hook.Verification(indexTree{index})
		}
	}
	return ""
}

// indexTree is the dialect.Tree view of an index's committed tree.
type indexTree struct{ index *Index }

func (tree indexTree) Tracks(name string) bool { return indexTracks(tree.index, name) }

func (tree indexTree) TracksUnder(prefix string) bool { return indexTracksUnder(tree.index, prefix) }

func (tree indexTree) RootText(name string) (string, bool) {
	return committedRootText(tree.index, name)
}

// committedRootText reads a tracked repository-root file's committed bytes at
// the index revision: an admitted source's pinned bytes, else one read-only
// `git cat-file` of the immutable tree entry bounded by rubyManifestBound.
// It reports false for an untracked, oversized or unreadable file.
func committedRootText(index *Index, name string) (string, bool) {
	if source, ok := index.Sources[name]; ok {
		text, valid, loaded := source.Text()
		return text, valid && loaded && rubyManifestBound.admits(len(text))
	}
	if _, ok := index.Tracked[name]; !ok || index.Root == "" || index.Revision == "" {
		return "", false
	}
	if _, skipped := index.Skipped[name]; skipped {
		return "", false
	}
	raw, err := git(context.Background(), index.Root, rubyManifestBound.limit, nil, "cat-file", "blob", index.Revision+":"+name)
	if err != nil {
		return "", false
	}
	return string(raw), true
}

func indexTracksUnder(index *Index, prefix string) bool {
	for candidate := range index.Tracked {
		if strings.HasPrefix(candidate, prefix) {
			return true
		}
	}
	return hasPathPrefix(index.Sources, prefix)
}

// rangeHasRubyPath reports whether a committed range admitted a Ruby path.
func rangeHasRubyPath(admittedPaths []string) bool {
	for _, changedPath := range admittedPaths {
		if strings.HasSuffix(changedPath, ".rb") {
			return true
		}
	}
	return false
}
