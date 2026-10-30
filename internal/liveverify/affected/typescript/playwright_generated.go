package typescript

import (
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// PlaywrightGenerated records the repository's generated-path declaration a Playwright plan
// applied: its bytes' identity and every entry the source walk excluded (AFP-V0-444).
type PlaywrightGenerated struct {
	Path   string   `json:"path"`
	Paths  []string `json:"paths"`
	SHA256 string   `json:"sha256"`
}

// playwrightBoundSourceFiles lists the files the Playwright source binding covers: every
// observed name the walk lists, minus the declared generated or vendored entries, so every
// digest producer and verifier agrees whether or not a caller holds the exclusion (AFP-V0-444).
func playwrightBoundSourceFiles(root string) ([]string, *PlaywrightGenerated, error) {
	sum, entries, err := affected.ReadGeneratedDeclaration(root)
	if err != nil {
		return nil, nil, err
	}
	files, err := affected.SourceFiles(root, playwrightObservedName)
	if err != nil {
		return nil, nil, err
	}
	if entries == nil {
		return files, nil, nil
	}
	kept := files[:0:0]
	for _, file := range files {
		if !affected.IgnoredMatch(entries, file) {
			kept = append(kept, file)
		}
	}
	return kept, &PlaywrightGenerated{Path: affected.GeneratedPathsPath, Paths: entries, SHA256: sum}, nil
}
