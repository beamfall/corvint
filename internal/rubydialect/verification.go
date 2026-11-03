// SPDX-License-Identifier: Apache-2.0

package rubydialect

import (
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/langplugin/decl"
	"github.com/Beamfall/corvint/internal/langplugin/dialect"
)

var (
	gemfileGemPattern = regexp.MustCompile(`(?m)^[ \t]*gem[ \t(]+['"]([A-Za-z0-9_][A-Za-z0-9_.-]*)['"]`)
	// A lockfile names a gem as a two-space DEPENDENCIES entry or a four-space
	// specs entry; deeper lines are a spec's own requirements.
	gemfileLockGemPattern = regexp.MustCompile(`(?m)^(?:  |    )([A-Za-z0-9_][A-Za-z0-9_.-]*)(?: \([^)\n]*\))?!?\r?$`)
)

// VerificationCommand names the one repository-wide command that runs a Ruby
// repository's tests, or "" when the committed tree does not prove one. Every
// rule needs a tracked root Gemfile, so `bundle exec` never runs where Bundler
// has nothing to resolve (NGI-V0-027). RSpec additionally needs an rspec gem
// declared in the root Gemfile or Gemfile.lock (NGI-V0-052); a Minitest
// repository gets `bin/rails test` or `bundle exec rake test` only when that
// runner is itself proved (NGI-V0-053). It reads at most three bounded
// committed blobs.
func VerificationCommand(tree dialect.Tree) string {
	if !tree.Tracks("Gemfile") {
		return ""
	}
	gems := declaredGems(tree)
	switch {
	case declaredRunner(gems, "rspec") &&
		(tree.Tracks(".rspec") || tree.TracksUnder("spec/")):
		return "bundle exec rspec"
	case (gems["rails"] || gems["railties"]) && tree.Tracks("bin/rails") && tree.TracksUnder("test/"):
		return "bin/rails test"
	case declaredRunner(gems, "minitest") && tree.TracksUnder("test/") && rakefileDeclaresTestTask(tree):
		return "bundle exec rake test"
	}
	return ""
}

// declaredRunner reports whether gems names a package the Ruby plugin maps to
// runner (LGP-V0-017).
func declaredRunner(gems map[string]bool, runner string) bool {
	for gem, bringsRunner := range decl.DependencyRunners(plugin) {
		if bringsRunner == runner && gems[gem] {
			return true
		}
	}
	return false
}

// declaredGems is the union of gem names the root Gemfile and Gemfile.lock
// declare. A manifest that is absent, oversized or unreadable contributes
// nothing, which can only withhold a command, never invent one.
func declaredGems(tree dialect.Tree) map[string]bool {
	gems := map[string]bool{}
	if text, ok := tree.RootText("Gemfile"); ok {
		for _, match := range gemfileGemPattern.FindAllStringSubmatch(text, -1) {
			gems[match[1]] = true
		}
	}
	if text, ok := tree.RootText("Gemfile.lock"); ok {
		for _, match := range gemfileLockGemPattern.FindAllStringSubmatch(text, -1) {
			gems[match[1]] = true
		}
	}
	return gems
}

func rakefileDeclaresTestTask(tree dialect.Tree) bool {
	text, ok := tree.RootText("Rakefile")
	return ok && (strings.Contains(text, "Rake::TestTask") || strings.Contains(text, "Minitest::TestTask"))
}
