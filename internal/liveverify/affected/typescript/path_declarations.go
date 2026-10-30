package typescript

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// PlaywrightUnreachedPathsPath is the repository's declaration of the source paths its Playwright
// suite cannot reach, not even through the application under test, so a dirty path it lists that
// the Playwright closure provably leaves out is a bounded exclusion instead of a selection
// unknown (TJAA-V0-106).
const PlaywrightUnreachedPathsPath = ".corvint/playwright-unreached-paths.json"

// PlaywrightUnreachedPathsProfile is that declaration's closed profile name.
const PlaywrightUnreachedPathsProfile = "corvint-playwright-unreached-paths/0"

// NonTestAssetPathsPath is the repository's declaration of the YAML or other data paths no test
// runner reads as a test asset, so they raise no cross-language test-asset frontier
// (TJAA-V0-111).
const NonTestAssetPathsPath = ".corvint/non-test-asset-paths.json"

// NonTestAssetPathsProfile is that declaration's closed profile name.
const NonTestAssetPathsProfile = "corvint-non-test-asset-paths/0"

const (
	maxPathDeclarationBytes   = 64 << 10
	maxPathDeclarationEntries = 256
)

type pathDeclarationFile struct {
	Profile string   `json:"profile"`
	Paths   []string `json:"paths"`
}

// readPathDeclaration reads one project-owned path declaration by the AFP-V0-160 (1) rules: at
// most 64 KiB, a closed {profile, paths} object, at most 256 strictly ascending entries, each a
// repository-relative file or a `dir/`, none at or under `.corvint/`. It returns the SHA-256 of
// the bytes and the entries, or an empty digest and nil entries when the file is absent, and an
// error for any other defect, so a malformed declaration is never silently ignored.
func readPathDeclaration(source *affected.Source, name, profile string) (string, []string, error) {
	if source == nil {
		return "", nil, nil
	}
	body, err := source.ReadBounded(name, maxPathDeclarationBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", name, err)
	}
	var declared pathDeclarationFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return "", nil, fmt.Errorf("%s: %w", name, err)
	}
	if declared.Profile != profile || declared.Paths == nil || len(declared.Paths) > maxPathDeclarationEntries {
		return "", nil, fmt.Errorf("%s: profile must be %s with at most %d paths", name, profile, maxPathDeclarationEntries)
	}
	previous := ""
	for _, entry := range declared.Paths {
		trimmed := strings.TrimSuffix(entry, "/")
		if !affected.ValidReadScopeEntry(entry) || entry <= previous || trimmed == ".corvint" || strings.HasPrefix(trimmed, ".corvint/") {
			return "", nil, fmt.Errorf("%s: path %q", name, entry)
		}
		previous = entry
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), declared.Paths, nil
}
