package sessiontest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gateselect"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-982 (V1-3749): the deciding driver (gateselect.Select, CIN-V0-860) and
// the shadow (`corvint test`, AFP-V0-028) narrow under exactly the same
// plan.unknown entries. Each entry here is one unknown of a plan over a dirty
// source of an existing package: the driver's verdict must be a run exactly
// when the shadow's allowlist admits the entry, so a case added to either side
// alone fails here. The one named exception is a well-formed unobserved-module
// frontier, which both sides read through gateselect.UnobservedModule: the
// driver admits it, and the shadow runs the module's own suite outside the ci
// profile. The table holds every reason the planner emits and every Go frontier
// detail it reports, beside non-Go frontiers.
func TestDriverAndShadowShareOneUnknownAllowlist_AFPV0982(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":            "module example.test/m\n\ngo 1.27\n",
		"core/core.go":      "package core\n",
		"core/core_test.go": "package core\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	details := []string{
		"go:build-constraint-variants", "go:nested-module-frontier", "go:unobserved-directory:x",
		"go:unobserved-module:nested:nested/n.go", "go:path-token-bound:go:example.test/m/core",
		"go:workspace-module-outside-root", "go:workspace-unparsed", "go:cgo-frontier", "go:unparsed-source",
		"go:module-path-unresolved", "go:included-directory-walk-bounded", "go:test-read-scopes-invalid",
		"go:test-binary-execs-invalid", "go:gated-root-locators-invalid", "go:gated-root-locators-changed",
		"typescript:dynamic-import:a.ts:1", "shell:foreign-invocation-unmodelled", "cfamily:jni-boundary", "",
	}
	type entry struct{ reason, detail string }
	var entries []entry
	for _, reason := range []string{affected.UnknownLanguageFrontier, affected.UnknownUnownedDirtyPath, affected.UnknownNoSelectableTest, affected.UnknownUnindexedSourcePath, "PLAN_TRUNCATED"} {
		for _, detail := range append(details, "core/other.go", "Makefile") {
			entries = append(entries, entry{reason, detail})
		}
	}
	for _, e := range entries {
		var plan gateselect.Receipt
		plan.Provider.Go.State = "RUNNABLE"
		plan.Provider.Go.Packages = []string{"example.test/m/core"}
		plan.Plan.Scope = "UNKNOWN"
		plan.Plan.Dirty = []string{"core/core.go"}
		plan.Plan.Unknown = append(plan.Plan.Unknown, struct {
			Reason string `json:"reason"`
			Detail string `json:"detail"`
		}{e.reason, e.detail})
		lines := gateselect.Select(plan, "example.test/m", root)
		driver := strings.HasPrefix(lines[len(lines)-1], "run ")
		shadow := allowedUnknown(Unknown{Reason: e.reason, Detail: e.detail})
		if _, module := moduleDirectory(Unknown{Reason: e.reason, Detail: e.detail}); module {
			if !driver || shadow {
				t.Errorf("%s %q: an unobserved-module frontier: driver narrows=%v, shadow allowlist=%v, want true and false", e.reason, e.detail, driver, shadow)
			}
			continue
		}
		if driver != shadow {
			t.Errorf("%s %q: driver narrows=%v (%s), shadow admits=%v", e.reason, e.detail, driver, lines[len(lines)-1], shadow)
		}
	}
}
