package contextindex

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/diagnostic"
)

// TestClassifyMissingObjectsNamesOnlyAnObjectTheReadNamed pins the V1-0284 refusal rule: with
// several promisor blobs missing, the refusal names the one the failed read named, never merely the
// first one missing, and a failure that names none of them keeps its original error.
func TestClassifyMissingObjectsNamesOnlyAnObjectTheReadNamed(t *testing.T) {
	t.Parallel()
	source := testRepository(t)
	writeTestFile(t, source, "a.txt", "a\n")
	writeTestFile(t, source, "b.txt", "b\n")
	testGit(t, source, "add", ".")
	testGit(t, source, "commit", "-qm", "two more blobs")
	testGit(t, source, "config", "uploadpack.allowFilter", "true")
	blobs := strings.Fields(testGit(t, source, "rev-parse", "HEAD:a.txt", "HEAD:b.txt"))
	root := filepath.Join(t.TempDir(), "clone")
	testGit(t, t.TempDir(), "clone", "-q", "--filter=blob:none", "--no-checkout", "file://"+source, root)
	tree := testGit(t, root, "rev-parse", "HEAD^{tree}")
	original := errors.New("read failed")
	for _, object := range blobs {
		err := classifyMissingObjects(context.Background(), root, original, "fatal: unable to read "+object, tree)
		var refusal *diagnostic.Error
		if !errors.As(err, &refusal) || fmt.Sprint(refusal.Refusal.Evidence) != fmt.Sprint([]diagnostic.Evidence{{Name: "object", Value: object}}) {
			t.Errorf("cause naming %s: got %v, want a refusal naming it", object, err)
		}
	}
	if err := classifyMissingObjects(context.Background(), root, original, "fatal: unrelated failure", tree); !errors.Is(err, original) {
		t.Errorf("cause naming no missing object: got %v, want the original error", err)
	}
}
