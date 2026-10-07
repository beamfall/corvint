package tasks

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// knowHowReaders is every production file that may name the know-how ledger.
// None of them ranks, plans, cites evidence or decides authority: the record
// codec, the Core reader that admits the optional key, the two write verbs,
// the import and adoption guards, and the read/delivery projection.
var knowHowReaders = []string{
	"internal/taskman/decode.go",
	"internal/taskman/know_how.go",
	"internal/tasks/cli/cli.go",
	"internal/tasks/cli/know_how.go",
	"internal/tasks/cli/lease.go",
	"internal/tasks/intent/policy.go",
	"internal/tasks/mutation/adopt.go",
	"internal/tasks/mutation/apply.go",
	"internal/tasks/mutation/know_how.go",
	"internal/tasks/mutation/mutation.go",
	"internal/tasks/store/claim_delivery.go",
	"internal/tasks/store/know_how.go",
	"internal/tasks/store/lease.go",
	"internal/tasks/ticket/know_how.go",
	"internal/tasks/ticket/record.go",
	"internal/tasks/ticket/view.go",
	"internal/tasks/transaction/model.go",
	"internal/tasks/wire/ticketkeys.go",
}

// TestKHNV0007_KnowHowNeverReachesRankingEvidenceOrAuthority: the set of
// production Go files that name the ledger is exactly knowHowReaders, so no
// ranking, planning, evidence, review or Core context path can read a note
// without this test and the spec changing together.
func TestKHNV0007_KnowHowNeverReachesRankingEvidenceOrAuthority(t *testing.T) {
	root := filepath.Join("..", "..")
	name := regexp.MustCompile(`(?i)know_?how`)
	var got []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && skipDir(path, d, root) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if name.Match(body) || bytes.Contains(body, []byte("KNOWHOW_")) {
			rel, _ := filepath.Rel(root, path)
			got = append(got, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(knowHowReaders, "\n") {
		t.Fatalf("files naming the know-how ledger changed:\ngot  %v\nwant %v", got, knowHowReaders)
	}
}
