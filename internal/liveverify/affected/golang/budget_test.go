package golang_test

import (
	"fmt"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// budgetFS is an immutable source that, like a RevisionFS over an exhausted
// Git budget, fails every operation after its first limit operations, and
// counts the operations attempted after the first failure.
type budgetFS struct {
	files        fstest.MapFS
	limit, ops   int
	afterFailure int
}

func (b *budgetFS) charge() error {
	b.ops++
	if b.ops > b.limit+1 {
		b.afterFailure++
	}
	if b.ops > b.limit {
		return cemcode.New(cemcode.GitBudgetExceeded, "Git operation budget exhausted")
	}
	return nil
}

func (b *budgetFS) Open(name string) (fs.File, error) {
	if err := b.charge(); err != nil {
		return nil, err
	}
	return b.files.Open(name)
}

func (b *budgetFS) OpenBounded(name string, _ int) (fs.File, error) { return b.Open(name) }

func (b *budgetFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if err := b.charge(); err != nil {
		return nil, err
	}
	return b.files.ReadDir(name)
}

func (b *budgetFS) Stat(name string) (fs.FileInfo, error) {
	if err := b.charge(); err != nil {
		return nil, err
	}
	return b.files.Stat(name)
}

// V1-1416 (DLT-V0-002): once the immutable source reports git-budget-exceeded
// during file reads, the Go observer attempts at most one further operation
// and returns that fatal failure instead of a partial unit set.
func TestUnitsSourceStopsAtGitBudgetExhaustion_V11416(t *testing.T) {
	const packages = 40
	files := budgetFixture(packages)
	unlimited := &budgetFS{files: files, limit: 1 << 30}
	if _, err := golang.New().UnitsSource(affected.FSSource(unlimited)); err != nil {
		t.Fatalf("unbounded observation: %v", err)
	}
	// The walk lists the root and every package directory; the budget runs
	// out a few file reads later, well before the observation ends.
	limit := packages + 1 + 5
	if unlimited.ops <= limit+packages {
		t.Fatalf("fixture too small: an unbounded observation used %d operations", unlimited.ops)
	}
	exhausted := &budgetFS{files: files, limit: limit}
	source := affected.FSSource(exhausted)
	result, err := golang.New().UnitsSource(source)
	if cemcode.CodeOf(err) != cemcode.GitBudgetExceeded || len(result.Units) != 0 {
		t.Fatalf("UnitsSource = %d units, %v; want the git-budget-exceeded failure", len(result.Units), err)
	}
	if cemcode.CodeOf(source.Err()) != cemcode.GitBudgetExceeded {
		t.Fatalf("retained failure %v", source.Err())
	}
	if exhausted.afterFailure > 1 {
		t.Fatalf("%d operations after git-budget-exceeded; want at most one", exhausted.afterFailure)
	}
}

func budgetFixture(packages int) fstest.MapFS {
	files := fstest.MapFS{"go.mod": {Data: []byte("module example.test/m\n")}}
	for index := range packages {
		directory := fmt.Sprintf("p%02d", index)
		files[directory+"/a.go"] = &fstest.MapFile{Data: []byte("package " + directory + "\n")}
		files[directory+"/a_test.go"] = &fstest.MapFile{Data: []byte("package " + directory + "\n")}
	}
	return files
}

// V1-1416: a read inside a walk that exhausts the budget ends the walk at the
// next entry, and every later read, stat and walk of the source returns the
// retained failure without reaching the backend.
func TestSourceShortCircuitsAfterFatalRead_V11416(t *testing.T) {
	exhausted := &budgetFS{files: budgetFixture(40), limit: 5}
	source := affected.FSSource(exhausted)
	err := source.Walk(".", func(name string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			_, _ = source.Read(name) // a provider that tolerates a failed read
		}
		return nil
	})
	if cemcode.CodeOf(err) != cemcode.GitBudgetExceeded || exhausted.afterFailure > 1 {
		t.Fatalf("Walk = %v after %d operations past the failure; want git-budget-exceeded and at most one", err, exhausted.afterFailure)
	}
	spent := exhausted.ops
	_, statErr := source.Stat("go.mod")
	_, lstatErr := source.Lstat("go.mod")
	_, readErr := source.ReadBounded("go.mod", 64)
	_, filesErr := source.Files(func(_, _ string) bool { return true })
	walkErr := source.Walk(".", func(string, fs.DirEntry, error) error { return nil })
	for _, err := range []error{statErr, lstatErr, readErr, filesErr, walkErr} {
		if cemcode.CodeOf(err) != cemcode.GitBudgetExceeded {
			t.Fatalf("later access returned %v; want the retained git-budget-exceeded", err)
		}
	}
	if exhausted.ops != spent {
		t.Fatalf("later accesses reached the backend %d times", exhausted.ops-spent)
	}
}
