// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ASD-V0-089 (V1-3248): a local-outcome aggregate-schema-invalid fix names the
// field and recovery the recorder wrote to local-outcome.stderr, and still
// names a recovery when an older recorder wrote only the bare code.
func TestASDV0089LocalOutcomeSchemaHintNamesField(t *testing.T) {
	dir := t.TempDir()
	c := &change{evidence: dir, base: "0123456789abcdef0123456789abcdef01234567"}
	stderr := filepath.Join(dir, "local-outcome.stderr")
	if err := os.WriteFile(stderr, []byte(`{"code":"aggregate-schema-invalid","error":"aggregate-schema-invalid field: verification \"go test ./x \"; fix: rerun corvint dogfood change BASE","ok":false}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hint := c.fixHint(step{name: "local-outcome", status: "FAIL", reason: "aggregate-schema-invalid"})
	if !strings.HasPrefix(hint, `field: verification "go test ./x "; fix: `) || !strings.Contains(hint, "dogfood change "+c.base) {
		t.Fatalf("hint %q", hint)
	}
	if err := os.WriteFile(stderr, []byte(`{"code":"aggregate-schema-invalid","error":"aggregate-schema-invalid","ok":false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if hint := c.fixHint(step{name: "local-outcome", status: "FAIL", reason: "aggregate-schema-invalid"}); !strings.Contains(hint, "fix: read "+stderr) {
		t.Fatalf("bare-code hint %q", hint)
	}
}

// ASD-V0-092 (V1-3254): the ocm-prepare invalid-requirements-section fix names
// the BASE fix for an intent BASE holds and the bootstrap-intent route for a
// spec the change adds, instead of saying every intent must exist at BASE.
func TestASDV0092RequirementsSectionHintNamesBootstrapRoute(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "old.md"), []byte("# Old\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "docs")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	c := &change{intents: []string{"docs/old.md", "docs/new.md"}}
	c.ctx, c.root, c.base, c.prefix, c.stderr = context.Background(), root, base, "dogfood-change", io.Discard
	present := c.fixHint(step{name: "ocm-prepare-1", status: "FAIL", reason: "invalid-requirements-section"})
	if want := `the intent on line N of DOGFOOD_INTENTS_FILE, ocm-prepare-1 being ocm-prepare-N, must be a spec Markdown file with exactly one "## Requirements" heading outside code fences; give it one, or name instead the spec that holds this change's requirements, then rerun corvint dogfood change ` + base; present != want {
		t.Fatalf("BASE hint:\n%s\nwant:\n%s", present, want)
	}
	bootstrap := c.fixHint(step{name: "ocm-prepare-2", status: "FAIL", reason: "invalid-requirements-section"})
	if want := `"docs/new.md" on line 2 of DOGFOOD_INTENTS_FILE is absent at BASE, a bootstrap intent this change adds (such as a new companion spec, DCW-V0-449); it is read at the change's commit and must hold exactly one "## Requirements" heading outside code fences: add the heading to that new spec and commit it, keep each of its hunks as no row or an unknown row in DOGFOOD_CITATIONS, then rerun corvint dogfood change ` + base; bootstrap != want {
		t.Fatalf("bootstrap hint:\n%s\nwant:\n%s", bootstrap, want)
	}
	if strings.Contains(present+bootstrap, "must be a spec that exists at BASE") {
		t.Fatal("hint still requires every intent at BASE")
	}
}

// TestDCWV0817RecordIntentHintNamesAcceptedForms (V1-3621): a build-log entry
// named as the intent of a change that is not a record change gets a fix line
// naming the record-change rule and the accepted forms, and the
// missing-intent-scope fix names the same rule.
func TestDCWV0817RecordIntentHintNamesAcceptedForms(t *testing.T) {
	c := &change{intents: []string{"docs/build-log/2026-11-01-x.md", "docs/specs/x-v0.md"}}
	c.ctx, c.root, c.base, c.prefix, c.stderr = context.Background(), t.TempDir(), "feedface", "dogfood-change", io.Discard
	hint := c.fixHint(step{name: "ocm-prepare-001", status: "FAIL", reason: "invalid-requirements-section"})
	for _, want := range []string{
		`"docs/build-log/2026-11-01-x.md" on line 001 of DOGFOOD_INTENTS_FILE is a build-log entry or decision`,
		"intent only of a record change",
		`spec Markdown file with a "## Requirements" heading`,
		"#no-intent-declared",
		"rerun corvint dogfood change feedface (DCW-V0-817)",
	} {
		if !strings.Contains(hint, want) {
			t.Fatalf("record intent hint %q lacks %q", hint, want)
		}
	}
	if !strings.Contains(intentScopeHint, "docs/build-log/*.md entry or docs/decisions/*.md file is an intent only of a record change") {
		t.Fatalf("missing-intent-scope fix does not name the record rule: %q", intentScopeHint)
	}
}
