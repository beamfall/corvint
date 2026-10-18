package defects_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/defects"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const now = wire.Timestamp("2026-10-17T12:00:00Z")

func s(v string) *string { return &v }

// tempDir resolves the platform temp directory: the register reader refuses
// a symlinked path component (macOS /var).
func tempDir(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func add(t *testing.T, path, title, category string) *defects.Defect {
	t.Helper()
	d, err := defects.Add(path, true, "DEF", defects.Fields{Title: s(title), Location: s("e2e/a.spec.ts:12"), Evidence: s("run-42"), Category: s(category)}, now)
	if err != nil {
		t.Fatalf("add %s: %v", title, err)
	}
	return d
}

// TestTDRV0002_AddAssignsIDsAndDefaults: add assigns <prefix>-N after the
// highest id of that prefix, defaults SUSPECTED / PRODUCT / MEDIUM, and
// refuses a missing or multi-line field and an unknown enum.
func TestTDRV0002_AddAssignsIDsAndDefaults(t *testing.T) {
	path := filepath.Join(tempDir(t), "taskman-defects", defects.RegisterFile)
	d := add(t, path, "login times out", defects.CategoryProduct)
	if d.ID != "DEF-1" || d.Status != defects.StatusSuspected || d.Severity != defects.SeverityMedium || d.Revision != 1 {
		t.Fatalf("first: %+v", d)
	}
	if d2 := add(t, path, "proxy drops", defects.CategoryEnvironment); d2.ID != "DEF-2" {
		t.Fatalf("second: %+v", d2)
	}
	for name, f := range map[string]defects.Fields{
		"no evidence":  {Title: s("x"), Location: s("y")},
		"two lines":    {Title: s("a\nb"), Location: s("y"), Evidence: s("z")},
		"bad severity": {Title: s("x"), Location: s("y"), Evidence: s("z"), Severity: s("URGENT")},
		"dup no link":  {Title: s("x"), Location: s("y"), Evidence: s("z"), Status: s(defects.StatusDuplicate)},
		"link no dup":  {Title: s("x"), Location: s("y"), Evidence: s("z"), DuplicateOf: s("DEF-1")},
	} {
		if _, err := defects.Add(path, true, "DEF", f, now); wire.CodeOf(err) != wire.CodeMalformed {
			t.Fatalf("%s: want MALFORMED, got %v", name, err)
		}
	}
	if reg, err := defects.Read(path, true); err != nil || len(reg.Defects()) != 2 || reg.Lines != 2 {
		t.Fatalf("refusals wrote: %v %+v", err, reg)
	}
}

// TestTDRV0004_UpdatesAppendAndLaterLinesOverride: an update appends the new
// full state; the earlier line stays byte-identical and the fold takes the
// last line. DUPLICATE needs a known, acyclic target.
func TestTDRV0004_UpdatesAppendAndLaterLinesOverride(t *testing.T) {
	path := filepath.Join(tempDir(t), defects.RegisterFile)
	add(t, path, "one", defects.CategoryProduct)
	add(t, path, "two", defects.CategoryProduct)
	before, _ := os.ReadFile(path)
	u, err := defects.Update(path, true, "DEF-1", defects.Fields{Status: s(defects.StatusConfirmed), Severity: s(defects.SeverityHigh)}, now)
	if err != nil || u.Status != defects.StatusConfirmed || u.Revision != 2 || u.Title != "one" {
		t.Fatalf("update: %v %+v", err, u)
	}
	after, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(after), string(before)) || strings.Count(string(after), "\n") != 3 {
		t.Fatalf("update rewrote the register:\n%s", after)
	}
	if _, err := defects.Update(path, true, "DEF-2", defects.Fields{Status: s(defects.StatusDuplicate), DuplicateOf: s("DEF-1")}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := defects.Update(path, true, "DEF-1", defects.Fields{Status: s(defects.StatusDuplicate), DuplicateOf: s("DEF-2")}, now); wire.CodeOf(err) != wire.CodeMalformed {
		t.Fatalf("cycle: %v", err)
	}
	if _, err := defects.Update(path, true, "DEF-9", defects.Fields{Status: s(defects.StatusConfirmed)}, now); wire.CodeOf(err) != wire.CodeMissingEvidence {
		t.Fatalf("unknown: %v", err)
	}
	reg, err := defects.Read(path, true)
	if err != nil || reg.Get("DEF-1").Status != defects.StatusConfirmed || reg.Resolve("DEF-2").ID != "DEF-1" {
		t.Fatalf("fold: %v", err)
	}
	// Leaving DUPLICATE drops the link.
	if u, err := defects.Update(path, true, "DEF-2", defects.Fields{Status: s(defects.StatusSuspected)}, now); err != nil || u.DuplicateOf != nil {
		t.Fatalf("un-duplicate: %v %+v", err, u)
	}
}

// TestTDRV0005_ConcurrentAddsSerializeAndReadsIgnoreAPartialTail: parallel
// adds each get a distinct id with no lost or torn line; a read ignores an
// unterminated tail and the next write seals it on its own line.
func TestTDRV0005_ConcurrentAddsSerializeAndReadsIgnoreAPartialTail(t *testing.T) {
	path := filepath.Join(tempDir(t), defects.RegisterFile)
	var wg sync.WaitGroup
	const workers, each = 8, 5
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				if _, err := defects.Add(path, true, "DEF", defects.Fields{Title: s("w" + strconv.Itoa(w)), Location: s("l"), Evidence: s("e")}, now); err != nil {
					t.Error(err)
				}
			}
		}(w)
	}
	wg.Wait()
	reg, err := defects.Read(path, true)
	if err != nil {
		t.Fatalf("after concurrent adds: %v", err)
	}
	if len(reg.Defects()) != workers*each {
		t.Fatalf("after concurrent adds: %d defects", len(reg.Defects()))
	}
	for i := 1; i <= workers*each; i++ {
		if reg.Get("DEF-"+strconv.Itoa(i)) == nil {
			t.Fatalf("DEF-%d missing", i)
		}
	}
	f, _ := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	f.WriteString(`{"profile":"taskman-defect/0","op":"AD`)
	f.Close()
	if reg, err = defects.Read(path, true); err != nil || len(reg.Defects()) != workers*each {
		t.Fatalf("partial tail: %v", err)
	}
	if d := add(t, path, "after tail", defects.CategoryProduct); d.ID != "DEF-41" {
		t.Fatalf("after tail: %+v", d)
	}
	if _, err := defects.Read(path, true); wire.CodeOf(err) != wire.CodeMalformed {
		t.Fatalf("a sealed torn line is reported, not skipped: %v", err)
	}
}

// TestTDRV0006_TextIsSecretScreenedWithoutEcho: a credential in any text
// field refuses SECRET_DETECTED naming the field, never the value, and
// writes nothing.
func TestTDRV0006_TextIsSecretScreenedWithoutEcho(t *testing.T) {
	path := filepath.Join(tempDir(t), defects.RegisterFile)
	secret := "AKIA" + "IOSFODNN7EXAMPLE"
	_, err := defects.Add(path, true, "DEF", defects.Fields{Title: s("t"), Location: s("l"), Evidence: s("key " + secret)}, now)
	if wire.CodeOf(err) != wire.CodeSecretDetected || strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), "--evidence") {
		t.Fatalf("screen: %v", err)
	}
	if _, serr := os.Stat(path); !os.IsNotExist(serr) {
		t.Fatalf("a refused add created the register")
	}
}

// TestTDRV0009_PrivateDefaultAndDeclaredPath: the default register lives
// under the Git common directory outside the Tasks state directory; a
// declared path places it in the primary worktree; Read creates nothing.
func TestTDRV0009_PrivateDefaultAndDeclaredPath(t *testing.T) {
	root := tempDir(t)
	repo := &intent.Repository{CommonDir: filepath.Join(root, ".git"), PrimaryWorktree: root, StateDir: filepath.Join(root, ".git", "taskman")}
	path, private := defects.Location(repo, &intent.Policy{})
	if !private || path != filepath.Join(root, ".git", defects.PrivateDir, defects.RegisterFile) || strings.HasPrefix(path, repo.StateDir+string(filepath.Separator)) {
		t.Fatalf("default: %s %v", path, private)
	}
	p := "qa/defects.jsonl"
	if path, private = defects.Location(repo, &intent.Policy{Defects: &intent.DefectPolicy{Path: &p}}); private || path != filepath.Join(root, "qa", "defects.jsonl") {
		t.Fatalf("declared: %s %v", path, private)
	}
	reg, err := defects.Read(filepath.Join(root, ".git", defects.PrivateDir, defects.RegisterFile), true)
	if err != nil || reg.Exists {
		t.Fatalf("missing register: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); !os.IsNotExist(err) {
		t.Fatalf("Read created a directory")
	}
}

// TestTDRV0007_TDRV0008_Findings: register ids cited by obligations and
// expected-fail lines are checked; unknown and NOT_A_BUG ids are findings,
// and a DEFECT or BLOCKED obligation resting only on ENVIRONMENT defects is
// DEFECT_ENVIRONMENT, while a PRODUCT or foreign defect satisfies it.
func TestTDRV0007_TDRV0008_Findings(t *testing.T) {
	path := filepath.Join(tempDir(t), defects.RegisterFile)
	add(t, path, "env", defects.CategoryEnvironment)    // DEF-1
	add(t, path, "product", defects.CategoryProduct)    // DEF-2
	add(t, path, "not a bug", defects.CategoryProduct)  // DEF-3
	add(t, path, "dup of env", defects.CategoryProduct) // DEF-4
	if _, err := defects.Update(path, true, "DEF-3", defects.Fields{Status: s(defects.StatusNotABug)}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := defects.Update(path, true, "DEF-4", defects.Fields{Status: s(defects.StatusDuplicate), DuplicateOf: s("DEF-1")}, now); err != nil {
		t.Fatal(err)
	}
	reg, err := defects.Read(path, true)
	if err != nil {
		t.Fatal(err)
	}
	reason := func(r string) *string { return &r }
	l := &ticket.ObligationLedger{Prefix: "AC", Entries: []ticket.ObligationEntry{
		{ID: "AC-1", Title: "env only", State: ticket.ObligationDefect, Reason: reason("DEF-1")},
		{ID: "AC-2", Title: "env and product", State: ticket.ObligationBlocked, Reason: reason("DEF-1 DEF-2")},
		{ID: "AC-3", Title: "not a bug", State: ticket.ObligationDefect, Reason: reason("DEF-3")},
		{ID: "AC-4", Title: "unknown DEF-99", State: ticket.ObligationOpen},
		{ID: "AC-5", Title: "duplicate of env", State: ticket.ObligationDefect, Reason: reason("DEF-4")},
		{ID: "AC-6", Title: "env plus tracker", State: ticket.ObligationDefect, Reason: reason("DEF-1 GH-12")},
		{ID: "AC-7", Title: "env but open", State: ticket.ObligationOpen, Reason: reason("DEF-1")},
	}}
	files := map[string][]byte{"e2e/a.spec.ts": []byte("test.fail('AC-8 expected-fail DEF-77', () => {})\ntest('AC-9 DEF-55 is not annotated', () => {})\n")}
	var got []string
	for _, f := range defects.Findings(reg, "DEF", l, files) {
		if f.Remedy == "" {
			t.Fatalf("finding without remedy: %+v", f)
		}
		got = append(got, f.Kind+":"+f.ID+":"+f.Path+":"+strconv.Itoa(f.Line))
	}
	want := "DEFECT_ENVIRONMENT:AC-1::0,DEFECT_NOT_A_BUG:DEF-3::0,DEFECT_UNKNOWN:DEF-99::0,DEFECT_ENVIRONMENT:AC-5::0,DEFECT_UNKNOWN:DEF-77:e2e/a.spec.ts:1"
	if strings.Join(got, ",") != want {
		t.Fatalf("findings\n got %s\nwant %s", strings.Join(got, ","), want)
	}
}

// TestTDRV0009_SymlinkedParentRefusesBeforeWriting: a declared register whose
// parent directory is a symlink out of the worktree refuses
// UNSUPPORTED_FILESYSTEM, as its readers do, and creates nothing at the
// symlink's target.
func TestTDRV0009_SymlinkedParentRefusesBeforeWriting(t *testing.T) {
	root, outside := tempDir(t), tempDir(t)
	if err := os.Symlink(outside, filepath.Join(root, "qa")); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"qa/defects.jsonl", "qa/deeper/defects.jsonl"} {
		_, err := defects.Add(filepath.Join(root, filepath.FromSlash(rel)), false, "DEF", defects.Fields{Title: s("t"), Location: s("l"), Evidence: s("e")}, now)
		if wire.CodeOf(err) != wire.CodeUnsupportedFilesystem {
			t.Fatalf("%s: want UNSUPPORTED_FILESYSTEM, got %v", rel, err)
		}
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("wrote outside the worktree: %v %v", entries, err)
	}
}

// TestTDRV0005_AppendNeverCrossesTheReadBound: an append that would grow the
// register past the 8 MiB read bound refuses LIMIT_EXCEEDED, writes nothing,
// and leaves the register readable; the appends below the bound succeed.
func TestTDRV0005_AppendNeverCrossesTheReadBound(t *testing.T) {
	path := filepath.Join(tempDir(t), defects.RegisterFile)
	first := add(t, path, "first", defects.CategoryProduct)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Pad with an unterminated tail, which readers ignore, to just under
	// the bound.
	pad := strings.Repeat("x", defects.MaxRegisterBytes-len(raw)-64)
	if err := os.WriteFile(path, append(raw, pad...), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := defects.Add(path, true, "DEF", defects.Fields{Title: s("second"), Location: s("l"), Evidence: s("e")}, now); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("want LIMIT_EXCEEDED, got %v", err)
	}
	if after, err := os.ReadFile(path); err != nil || string(after) != string(before) {
		t.Fatalf("a refused append changed the register: %v", err)
	}
	reg, err := defects.Read(path, true)
	if err != nil || reg.Get(first.ID) == nil || len(reg.Defects()) != 1 {
		t.Fatalf("register unreadable after the refusal: %v", err)
	}
}

// TestTDRV0004_LineProfileIsChecked: a register line of another profile is
// MALFORMED and one from a later build of this profile is
// UNSUPPORTED_VERSION, never folded as a defect.
func TestTDRV0004_LineProfileIsChecked(t *testing.T) {
	path := filepath.Join(tempDir(t), defects.RegisterFile)
	add(t, path, "first", defects.CategoryProduct)
	good, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for profile, code := range map[string]string{"taskman-other/0": wire.CodeMalformed, "taskman-defect/1": wire.CodeUnsupportedVersion} {
		line := strings.Replace(string(good), defects.Profile, profile, 1)
		if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := defects.Read(path, true); wire.CodeOf(err) != code {
			t.Fatalf("%s: want %s, got %v", profile, code, err)
		}
	}
}
