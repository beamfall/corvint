package golang

import (
	"path"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// SetMaxPathTokens lowers the per-package path token bound for one test.
func SetMaxPathTokens(t testing.TB, bound int) {
	previous := maxPathTokens
	maxPathTokens = bound
	t.Cleanup(func() { maxPathTokens = previous })
}

// SetMaxNestedManifestBytes lowers the nested go.mod read bound for one test.
func SetMaxNestedManifestBytes(t testing.TB, bound int64) {
	previous := maxNestedManifestBytes
	maxNestedManifestBytes = bound
	t.Cleanup(func() { maxNestedManifestBytes = previous })
}

// NestedModule is one nested manifest read and why it keeps the frontier open.
type NestedModule = nestedModule

// NestedModules is the per-module nested-frontier evidence for one source.
func NestedModules(t testing.TB, root *affected.Source) []NestedModule {
	t.Helper()
	manifests, err := root.Files(func(_, name string) bool { return path.Base(name) == "go.mod" })
	if err != nil {
		t.Fatalf("manifests: %v", err)
	}
	_, evidence, err := observeModules(root, manifests, map[string]bool{})
	if err != nil {
		t.Fatalf("observe modules: %v", err)
	}
	return evidence
}

// SetReadScopeOperations lowers the committed declaration's Git operation
// base and per-component allowance for one test.
func SetReadScopeOperations(t testing.TB, base, component int) {
	previousBase, previousComponent := readScopeBaseOperations, readScopeComponentOperations
	readScopeBaseOperations, readScopeComponentOperations = base, component
	t.Cleanup(func() { readScopeBaseOperations, readScopeComponentOperations = previousBase, previousComponent })
}

// HasPlatformSuffix exposes the GOOS/GOARCH file-name check (AFP-V0-580).
func HasPlatformSuffix(relative string) bool { return hasPlatformSuffix(relative) }

// EmbedPatterns and EmbedTargets expose the //go:embed resolution (AFP-V0-600).
var (
	EmbedPatterns = embedPatterns
	EmbedTargets  = embedTargets
)
