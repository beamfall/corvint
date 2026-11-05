// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// AZU-V0-8900: owning a parser control declaration for audit purposes must
// preserve selection uncertainty and every reader. It grants no directory claim.
func TestAuditControlKeepsSelectionUncertainty_AZU_V0_8900(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, DocumentationPathsPath), []byte(`{"profile":"corvint-documentation-paths/0","paths":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	graph, err := Build(context.Background(), root, fake{name: "g", units: []Unit{{ID: "g:reader", Sources: []string{"reader.g"}, Tests: []string{"reader_test.g"}, PathTokens: []string{DocumentationPathsPath}}}})
	if err != nil {
		t.Fatal(err)
	}
	arbitrary := ".corvint/new-unclaimed.dat"
	if got := graph.OwnershipAudit([]string{DocumentationPathsPath, arbitrary}); !slices.Equal(got, []string{arbitrary}) {
		t.Fatalf("audit claims: %v", got)
	}
	plan := Select(graph, []string{DocumentationPathsPath})
	if !slices.ContainsFunc(plan.Unknown, func(u Unknown) bool { return u.Reason == UnknownUnownedDirtyPath && u.Detail == DocumentationPathsPath }) || !slices.ContainsFunc(plan.Selected, func(s Selection) bool { return s.UnitID == "g:reader" }) {
		t.Fatalf("audit changed selection behavior: %+v", plan)
	}
}
