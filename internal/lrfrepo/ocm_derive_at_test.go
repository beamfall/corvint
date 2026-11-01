package lrfrepo

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestDeriveIntentAtMatchesPrepareOCM_DCW_V0_945 (V1-3633): for every intent
// PrepareOCM refuses to derive, DeriveIntentAt, the pre-flight's derivation,
// returns the same refusal code, and both admit a valid intent.
func TestDeriveIntentAtMatchesPrepareOCM_DCW_V0_945(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitOCMPrepare(t, root, "init", "-q", "-b", "main")
	writeOCMPrepareFile(t, root, "docs/specs/intent.md", "# Intent\n\n## Requirements\n\n- `OCM-TEST-001`: preserve the review.\n\n## Non-goals\n")
	writeOCMPrepareFile(t, root, "internal/example.txt", "before\n")
	gitOCMPrepare(t, root, "add", ".")
	gitOCMPrepare(t, root, "commit", "-qm", "base")
	base := gitOCMPrepare(t, root, "rev-parse", "HEAD")
	writeOCMPrepareFile(t, root, "internal/example.txt", "after\n")
	writeOCMPrepareFile(t, root, "docs/specs/latin1.md", "# Intent\n\n## Requirements\n\n- `OCM-TEST-002`: caf\xe9.\n")
	over := new(strings.Builder)
	over.WriteString("# Intent\n\n## Requirements\n\n")
	for index := 1; index <= MaxOCMObligations+1; index++ {
		fmt.Fprintf(over, "- `OVR-V0-%03d`: keep the bound.\n", index)
	}
	writeOCMPrepareFile(t, root, "docs/specs/over.md", over.String())
	gitOCMPrepare(t, root, "add", ".")
	gitOCMPrepare(t, root, "commit", "-qm", "target")
	target := gitOCMPrepare(t, root, "rev-parse", "HEAD")
	prepareCEM(t, root, base, target, false)
	code := func(err error) string {
		var refusal *Error
		if errors.As(err, &refusal) {
			return refusal.Code
		}
		if err != nil {
			return "uncoded: " + err.Error()
		}
		return ""
	}
	for _, intent := range []string{"docs/specs/intent.md", "docs/specs/latin1.md", "docs/specs/over.md", "docs/specs/absent.md", "docs/specs"} {
		_, prepareErr := PrepareOCM(context.Background(), root, PrepareOptions{
			MapPath: ".corvint/derive-" + strings.ReplaceAll(intent, "/", "-") + ".ocm.json", CEMPath: wire.ExcludedCEMPath,
			IntentPath: intent, ExpectedBase: base, Target: target,
		})
		_, deriveErr := DeriveIntentAt(context.Background(), root, target, intent)
		if code(prepareErr) != code(deriveErr) {
			t.Errorf("%s: PrepareOCM %q, DeriveIntentAt %q", intent, code(prepareErr), code(deriveErr))
		}
		if intent != "docs/specs/intent.md" && code(deriveErr) == "" {
			t.Errorf("%s: derived a bad intent", intent)
		}
	}
}
