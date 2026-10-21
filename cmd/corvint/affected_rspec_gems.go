package main

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// The RSpec gem-declaration check of the advisory RSpec command (AFP-V0-066).
const (
	adviceNoRSpecGem = "NO_ADVISORY_RSPEC_COMMAND: the plan selected RSpec units but the root Gemfile and Gemfile.lock declare no rspec gem"
	// adviceMaxGemManifestBytes bounds each root Gemfile or Gemfile.lock read,
	// as NGI-V0-052 bounds the committed manifests.
	adviceMaxGemManifestBytes = 256 << 10
)

var (
	// The NGI-V0-052 gem-declaration patterns: a Gemfile `gem` line, and a
	// two-space DEPENDENCIES or four-space specs entry of a lockfile.
	adviceGemfileGem     = regexp.MustCompile(`(?m)^[ \t]*gem[ \t(]+['"]([A-Za-z0-9_][A-Za-z0-9_.-]*)['"]`)
	adviceGemfileLockGem = regexp.MustCompile(`(?m)^(?:  |    )([A-Za-z0-9_][A-Za-z0-9_.-]*)(?: \([^)\n]*\))?!?\r?$`)
)

// adviceDeclaresRSpecGem reports whether the working-tree root Gemfile or
// Gemfile.lock declares rspec, rspec-core or rspec-rails, by the NGI-V0-052
// check (AFP-V0-066). A manifest that is not a regular file, is larger than
// the bound, or cannot be read declares nothing, which can only withhold the
// command.
func adviceDeclaresRSpecGem(root string) bool {
	for _, manifest := range []struct {
		name    string
		pattern *regexp.Regexp
	}{{"Gemfile", adviceGemfileGem}, {"Gemfile.lock", adviceGemfileLockGem}} {
		text, ok := adviceReadGemManifest(filepath.Join(root, manifest.name))
		if !ok {
			continue
		}
		for _, match := range manifest.pattern.FindAllStringSubmatch(text, -1) {
			if match[1] == "rspec" || match[1] == "rspec-core" || match[1] == "rspec-rails" {
				return true
			}
		}
	}
	return false
}

func adviceReadGemManifest(path string) (string, bool) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > adviceMaxGemManifestBytes {
		return "", false
	}
	file, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, adviceMaxGemManifestBytes+1))
	if err != nil || len(body) > adviceMaxGemManifestBytes {
		return "", false
	}
	return string(body), true
}
