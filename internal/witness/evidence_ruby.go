package witness

import (
	"bytes"
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/rubytestprovider"
)

// maxRunBlobCheckBytes bounds the git cat-file --batch-check output one
// provider document's blob comparison reads: one short header per file.
const maxRunBlobCheckBytes = 1 << 20

// runIdentity is what a repository-observing provider document (RSpec,
// AGW-V0-010; go test -json, AGW-V0-012) records about its run.
type runIdentity struct {
	kind           string
	infrastructure string
	before, after  *jstestprovider.ApplicationRepositoryIdentity
	objectFormat   string
	// blobs are (worktree-relative path, blob ID) pairs, in document order.
	blobs [][2]string
}

// bindRuby admits a corvint-ruby-test-provider/0 document under bindRun's
// conditions, comparing every spec and helper blob ID it records (AGW-V0-010).
func bindRuby(ctx context.Context, budget *gitrun.Budget, index *contextindex.Index, document *rubytestprovider.Document) (string, string) {
	run := runIdentity{kind: "RSpec", before: document.RepositoryBefore, after: document.RepositoryAfter, objectFormat: document.ObjectFormat}
	if document.Infrastructure != nil {
		run.infrastructure = document.Infrastructure.Reason
	}
	for _, helper := range document.Helpers {
		run.blobs = append(run.blobs, [2]string{helper.Path, helper.Blob})
	}
	for _, example := range document.Examples {
		if example.SpecBlob != "" {
			run.blobs = append(run.blobs, [2]string{example.File, example.SpecBlob})
		}
	}
	return bindRun(ctx, budget, index, run)
}

// bindRun admits a provider document only when the run completed, recorded the
// same clean repository identity before and after, that identity is the
// report's head commit and head tree in the head's object format, and every
// blob ID it records is the head tree's blob at that path. Every other case
// names why it is UNBOUND.
func bindRun(ctx context.Context, budget *gitrun.Budget, index *contextindex.Index, run runIdentity) (string, string) {
	before, after := run.before, run.after
	headFormat := "sha1"
	if len(index.CommitRevision) == 64 {
		headFormat = "sha256"
	}
	switch {
	case run.infrastructure != "":
		return "UNBOUND", "the " + run.kind + " run recorded an infrastructure failure (" + run.infrastructure + "), so it is not a complete observation"
	case before == nil || after == nil:
		return "UNBOUND", "the " + run.kind + " document does not record the repository identity both before and after the run"
	case *before != *after:
		return "UNBOUND", "the repository identity changed between the start and the end of the " + run.kind + " run"
	case before.DirtyState != "clean":
		return "UNBOUND", "the repository was not clean during the " + run.kind + " run (dirtyState=" + before.DirtyState + ")"
	case run.objectFormat != headFormat:
		return "UNBOUND", "the " + run.kind + " document names " + run.objectFormat + " blob IDs, not the head repository's " + headFormat
	case before.Revision != index.CommitRevision:
		return "UNBOUND", "the " + run.kind + " run was at revision " + before.Revision + ", not the report head " + index.CommitRevision
	case before.Tree != index.Revision:
		return "UNBOUND", "the " + run.kind + " run was at tree " + before.Tree + ", not the report head tree " + index.Revision
	}
	if detail := compareRunBlobs(ctx, budget, index, run); detail != "" {
		return "UNBOUND", detail
	}
	return "BOUND", "caller-reported " + run.kind + " run at the head tree; CALLER_REPORTED never closes an obligation (CF-V0-014)"
}

// compareRunBlobs checks every blob ID the document records against the head
// tree's blob at that worktree-relative path with one git cat-file
// --batch-check. It returns "" when all match, and otherwise names the first
// path that is contradictory, absent from the head tree or different.
func compareRunBlobs(ctx context.Context, budget *gitrun.Budget, index *contextindex.Index, run runIdentity) string {
	expected := map[string]string{}
	for _, pair := range run.blobs {
		file, blob := pair[0], pair[1]
		if prior, seen := expected[file]; seen && prior != blob {
			return "the " + run.kind + " document records two different blob IDs for " + file
		}
		if repositoryPath(file) != file || strings.ContainsAny(file, "\n\r") {
			return "the " + run.kind + " document names " + file + ", which is not a clean worktree-relative path"
		}
		expected[file] = blob
	}
	if len(expected) == 0 {
		return ""
	}
	ordered := make([]string, 0, len(expected))
	for file := range expected {
		ordered = append(ordered, file)
	}
	sort.Strings(ordered)
	var request bytes.Buffer
	for _, file := range ordered {
		request.WriteString(index.Revision + ":" + file + "\n")
	}
	output, err := gitrun.Run(ctx, budget, gitrun.Options{Dir: index.Root, Env: gitEnvironment(), Stdin: request.Bytes(), StdoutLimit: maxRunBlobCheckBytes},
		"-c", "advice.graftFileDeprecated=false", "cat-file", "--batch-check")
	if err != nil {
		return "the head blobs the " + run.kind + " document names could not be read for comparison: " + err.Error()
	}
	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if len(lines) != len(ordered) {
		return "git cat-file --batch-check returned " + strconv.Itoa(len(lines)) + " lines for " + strconv.Itoa(len(ordered)) + " paths"
	}
	for position, file := range ordered {
		fields := strings.Fields(lines[position])
		if len(fields) != 3 || fields[1] != "blob" {
			return "the " + run.kind + " document names " + file + ", which is not a blob in the head tree"
		}
		if fields[0] != expected[file] {
			return "the " + run.kind + " document's blob ID for " + file + " differs from the head blob, so the run did not read the head tree's bytes"
		}
	}
	return ""
}

// rubyTests maps each bound RSpec example onto the head-tree file it is
// declared in. An example whose spec file the provider could not bind to a
// blob attaches nothing and is counted unattached.
func rubyTests(document *rubytestprovider.Document) []evidenceTest {
	tests := make([]evidenceTest, 0, len(document.Examples))
	for _, example := range document.Examples {
		name := example.Description
		if name == "" {
			name = example.ID
		}
		test := evidenceTest{name: name, state: example.Status}
		if file := repositoryPath(example.File); file != "" && example.SpecBlob != "" {
			test.anchor, test.files = file, []string{file}
		}
		tests = append(tests, test)
	}
	return tests
}
