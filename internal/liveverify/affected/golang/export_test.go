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
	manifests, err := root.Files(func(name string) bool { return path.Base(name) == "go.mod" })
	if err != nil {
		t.Fatalf("manifests: %v", err)
	}
	_, evidence, err := observeModules(root, manifests, map[string]bool{})
	if err != nil {
		t.Fatalf("observe modules: %v", err)
	}
	return evidence
}
