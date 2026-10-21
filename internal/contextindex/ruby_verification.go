package contextindex

import (
	"context"
	"regexp"
	"strings"
)

// rubyManifestBound (analyzer_bounds.go) bounds each committed Gemfile,
// Gemfile.lock or Rakefile read for the Ruby verification command. A larger
// blob is treated as unreadable: it proves nothing, so the command is withheld
// unless the other, readable manifest proves it alone (NGI-V0-052, TSA-V0-461).

var (
	gemfileGemPattern = regexp.MustCompile(`(?m)^[ \t]*gem[ \t(]+['"]([A-Za-z0-9_][A-Za-z0-9_.-]*)['"]`)
	// A lockfile names a gem as a two-space DEPENDENCIES entry or a four-space
	// specs entry; deeper lines are a spec's own requirements.
	gemfileLockGemPattern = regexp.MustCompile(`(?m)^(?:  |    )([A-Za-z0-9_][A-Za-z0-9_.-]*)(?: \([^)\n]*\))?!?\r?$`)
)

// rubyVerificationCommand names the one repository-wide command that runs a
// Ruby repository's tests, or "" when the committed tree does not prove one.
// Every rule needs a tracked root Gemfile, so `bundle exec` never runs where
// Bundler has nothing to resolve (NGI-V0-027). RSpec additionally needs an
// rspec gem declared in the root Gemfile or Gemfile.lock (NGI-V0-052); a
// Minitest repository gets `bin/rails test` or `bundle exec rake test` only
// when that runner is itself proved (NGI-V0-053). It reads at most three
// bounded committed blobs and only when the caller has a Ruby path.
func rubyVerificationCommand(index *Index) string {
	if !indexTracks(index, "Gemfile") {
		return ""
	}
	gems := declaredGems(index)
	switch {
	case (gems["rspec"] || gems["rspec-core"] || gems["rspec-rails"]) &&
		(indexTracks(index, ".rspec") || indexTracksUnder(index, "spec/")):
		return "bundle exec rspec"
	case (gems["rails"] || gems["railties"]) && indexTracks(index, "bin/rails") && indexTracksUnder(index, "test/"):
		return "bin/rails test"
	case gems["minitest"] && indexTracksUnder(index, "test/") && rakefileDeclaresTestTask(index):
		return "bundle exec rake test"
	}
	return ""
}

// declaredGems is the union of gem names the root Gemfile and Gemfile.lock
// declare. A manifest that is absent, oversized or unreadable contributes
// nothing, which can only withhold a command, never invent one.
func declaredGems(index *Index) map[string]bool {
	gems := map[string]bool{}
	if text, ok := committedRootText(index, "Gemfile"); ok {
		for _, match := range gemfileGemPattern.FindAllStringSubmatch(text, -1) {
			gems[match[1]] = true
		}
	}
	if text, ok := committedRootText(index, "Gemfile.lock"); ok {
		for _, match := range gemfileLockGemPattern.FindAllStringSubmatch(text, -1) {
			gems[match[1]] = true
		}
	}
	return gems
}

func rakefileDeclaresTestTask(index *Index) bool {
	text, ok := committedRootText(index, "Rakefile")
	return ok && (strings.Contains(text, "Rake::TestTask") || strings.Contains(text, "Minitest::TestTask"))
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
