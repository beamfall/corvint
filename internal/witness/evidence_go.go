package witness

import (
	"context"
	"encoding/json"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gotestrun"
)

// decodeGoRun recognizes a corvint-go-test-run/0 document by its top-level
// profile member. ok is false for every other document, which the existing
// decoders read; a document that names the profile but fails its closed shape
// is an error.
func decodeGoRun(data []byte) (*gotestrun.Document, bool, error) {
	var head struct {
		Profile string `json:"profile"`
	}
	if json.Unmarshal(data, &head) != nil || head.Profile != gotestrun.Profile {
		return nil, false, nil
	}
	document, err := gotestrun.Decode(data)
	if err != nil {
		return nil, true, err
	}
	return &document, true, nil
}

// bindGo admits a corvint-go-test-run/0 document under exactly the RSpec
// conditions (bindRun): a completed run, the same clean identity before and
// after at the head commit and tree in the head's object format, and every
// recorded test file's blob ID equal to the head blob (AGW-V0-012).
func bindGo(ctx context.Context, budget *gitrun.Budget, index *contextindex.Index, document *gotestrun.Document) (string, string) {
	run := runIdentity{kind: "go test", before: document.RepositoryBefore, after: document.RepositoryAfter, objectFormat: document.ObjectFormat}
	if document.Infrastructure != nil {
		run.infrastructure = document.Infrastructure.Reason
	}
	for _, test := range document.Tests {
		if test.File != "" {
			run.blobs = append(run.blobs, [2]string{test.File, test.Blob})
		}
	}
	return bindRun(ctx, budget, index, run)
}

// goTests maps each bound Go test onto the head-tree file that declares its
// top-level function, named by import path and full test name. A test the
// provider could not anchor (outside the module, undeclared or ambiguous)
// attaches nothing and is counted unattached.
func goTests(document *gotestrun.Document) []evidenceTest {
	tests := make([]evidenceTest, 0, len(document.Tests))
	for _, test := range document.Tests {
		item := evidenceTest{name: test.Package + "." + test.Name, state: test.Action}
		if file := repositoryPath(test.File); file != "" && test.Blob != "" {
			item.anchor, item.files = file, []string{file}
		}
		tests = append(tests, item)
	}
	return tests
}
