package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestStrictLFFieldsRejectsNonCanonicalForms(t *testing.T) {
	if fields, err := strictLFFields([]byte("sha1\nhead\ntree\n"), 3); err != nil || !reflect.DeepEqual(fields, []string{"sha1", "head", "tree"}) {
		t.Fatalf("canonical fields = %v, %v", fields, err)
	}
	for _, hostile := range [][]byte{
		[]byte("sha1\nhead\ntree"),
		[]byte("sha1\r\nhead\ntree\n"),
		[]byte("sha1\n\ntree\n"),
		[]byte("sha1\nhead\ntree\nextra\n"),
		append([]byte("sha1\nhead\n"), []byte{'t', 0, 'r', 'e', 'e', '\n'}...),
		[]byte("sha1\nhead\ncaf\xc3\xa9\n"),
	} {
		if _, err := strictLFFields(hostile, 3); err == nil {
			t.Fatalf("accepted malformed fields %q", hostile)
		}
	}
}

func TestDiscoveryRejectsUnicodeControls(t *testing.T) {
	for _, field := range []string{"/tmp/control\u0085path", "/tmp/control\u009fpath"} {
		raw := []byte(field + "\n/tmp/git\n/tmp/common\n")
		if _, err := parseDiscovery(raw); err == nil {
			t.Fatalf("accepted discovery control path %q", field)
		}
	}
}

func TestDirtyPathsRenameDeduplicatesAndUsesFrozenDigest(t *testing.T) {
	raw := []byte("R  destination\x00source\x00 M destination\x00?? zeta\x00")
	paths, err := parseDirtyPaths(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"destination", "source", "zeta"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths = %q, want %q", paths, want)
	}
	digest, err := dirtyDigest(paths)
	if err != nil {
		t.Fatal(err)
	}
	preimage := append([]byte("corvint-dashboard-dirty-paths/0\x00"), []byte(`["destination","source","zeta"]`)...)
	wantHash := sha256.Sum256(preimage)
	if digest != "sha256:"+hex.EncodeToString(wantHash[:]) {
		t.Fatalf("digest = %s", digest)
	}
}

func TestDirtyPathsRejectsMalformedAndHostilePaths(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte("M source\x00"),
		[]byte("MXsource\x00"),
		[]byte("Z  source\x00"),
		[]byte("R  destination\x00"),
		[]byte("?? ../escape\x00"),
		[]byte("?? C:/escape\x00"),
		[]byte("?? line\nbreak\x00"),
		[]byte("?? invalid-\xff\x00"),
		[]byte("?? missing-nul"),
	} {
		if _, err := parseDirtyPaths(raw); err == nil {
			t.Fatalf("accepted hostile status %q", raw)
		}
	}
}

func TestTreeObjectsRequireExactRegularBlobSet(t *testing.T) {
	oidA := strings.Repeat("a", 40)
	oidB := strings.Repeat("b", 40)
	raw := []byte("100755 blob " + oidB + "\tb/path\x00" + "100644 blob " + oidA + "\ta/path\x00")
	objects, regular, err := parseTreeObjects(raw, []string{"a/path", "b/path"}, "sha1")
	if err != nil || !regular || len(objects) != 2 {
		t.Fatalf("objects = %#v, regular=%v, err=%v", objects, regular, err)
	}
	for _, malformed := range [][]byte{
		[]byte("100644 blob " + oidA + "\ta/path"),
		[]byte("100644 blob " + oidA + "\ta/path\x00" + "100644 blob " + oidA + "\ta/path\x00"),
		[]byte("100644 blob " + strings.ToUpper(oidA) + "\ta/path\x00"),
		[]byte("100644 blob " + oidA + "\textra\x00"),
	} {
		if _, _, err := parseTreeObjects(malformed, []string{"a/path"}, "sha1"); err == nil {
			t.Fatalf("accepted malformed ls-tree %q", malformed)
		}
	}
	for _, nonRegular := range [][]byte{
		[]byte("120000 blob " + oidA + "\ta/path\x00"),
		[]byte("040000 tree " + oidA + "\ta/path\x00"),
	} {
		if _, regular, err := parseTreeObjects(nonRegular, []string{"a/path"}, "sha1"); err != nil || regular {
			t.Fatalf("nonregular classification regular=%v err=%v", regular, err)
		}
	}
}

func TestTreeChunkUsesExactPathCountAndByteBounds(t *testing.T) {
	revision := strings.Repeat("a", 40)
	paths := make([]string, maxTreePathsPerChild+1)
	for index := range paths {
		paths[index] = "path-" + strings.Repeat("x", index%7)
	}
	if end := treeChunkEnd(revision, paths, 0); end != maxTreePathsPerChild {
		t.Fatalf("first chunk end = %d", end)
	}
	if end := treeChunkEnd(revision, paths, maxTreePathsPerChild); end != len(paths) {
		t.Fatalf("second chunk end = %d", end)
	}
}

func TestObjectCheckParserRequiresExactOrderedTranscript(t *testing.T) {
	commit := strings.Repeat("a", 40)
	blob := strings.Repeat("b", 40)
	expected := []objectExpectation{
		{id: commit, expectedType: "commit"},
		{id: blob, expectedType: "blob"},
	}
	raw := []byte(commit + " commit 123\n" + blob + " blob 16777216\n")
	checks, ok := parseObjectChecks(raw, expected)
	if !ok || checks[commit] != objectCheckQualified || checks[blob] != objectCheckQualified {
		t.Fatalf("checks = %#v, ok=%v", checks, ok)
	}

	semantic := []struct {
		raw  string
		want objectCheck
	}{
		{commit + " blob 1\n", objectCheckInvalid},
		{commit + " commit 16777217\n", objectCheckOversize},
	}
	for _, test := range semantic {
		checks, ok := parseObjectChecks([]byte(test.raw), expected[:1])
		if !ok || checks[commit] != test.want {
			t.Fatalf("parseObjectChecks(%q) = %#v, %v", test.raw, checks, ok)
		}
	}

	for _, hostile := range []string{
		"",
		commit + " commit 1",
		commit + " commit 01\n",
		commit + " missing\n",
		commit + " ambiguous\n",
		commit + " commit -1\n",
		commit + " commit 1 extra\n",
		commit + "  commit 1\n",
		strings.ToUpper(commit) + " commit 1\n",
		blob + " blob 1\n" + commit + " commit 1\n",
		commit + " commit 1\r\n",
	} {
		if _, ok := parseObjectChecks([]byte(hostile), expected[:1]); ok {
			t.Fatalf("accepted hostile cat-file transcript %q", hostile)
		}
	}
}

func TestCatChunkAndInputUseFrozenBounds(t *testing.T) {
	values := make([]objectExpectation, maxCatIDsPerChild+1)
	for index := range values {
		values[index] = objectExpectation{id: strings.Repeat("a", 40), expectedType: "commit"}
	}
	if end := catChunkEnd(values, 0); end != maxCatIDsPerChild {
		t.Fatalf("first chunk end = %d", end)
	}
	if end := catChunkEnd(values, maxCatIDsPerChild); end != len(values) {
		t.Fatalf("second chunk end = %d", end)
	}
	input := catInput(values[:2])
	if want := strings.Repeat("a", 40) + "\n" + strings.Repeat("a", 40) + "\n"; string(input) != want {
		t.Fatalf("input = %q, want %q", input, want)
	}
}

func TestChildEnvironmentOmitsObjectDirectoryAndStartupPath(t *testing.T) {
	state := startupState{bootstrap: []string{"TMPDIR=/private/tmp"}}
	environment := state.childEnvironment()
	want := []string{
		"TMPDIR=/private/tmp", "LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull,
		"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1", "GIT_GRAFT_FILE=" + os.DevNull, "GIT_ALTERNATE_OBJECT_DIRECTORIES=",
		"GIT_PROTOCOL_FROM_USER=0", "GCM_INTERACTIVE=never", "GIT_ASKPASS=", "SSH_ASKPASS=",
	}
	if !reflect.DeepEqual(environment, want) {
		t.Fatalf("environment = %q, want %q", environment, want)
	}
	for _, row := range environment {
		if strings.HasPrefix(row, "PATH=") || strings.HasPrefix(row, "GIT_OBJECT_DIRECTORY=") {
			t.Fatalf("forbidden environment row: %q", row)
		}
	}
}

func TestRepositoryConfigGrammarRejectsIncludesAndAmbiguity(t *testing.T) {
	for _, accepted := range []string{
		"",
		"# comment\n; another\n[core]\n\tbare = false\n",
		"[remote \"origin\"]\nurl=https://example.invalid/repo\n",
		"[worktree]\nuseRelativePaths\n",
	} {
		if !validRepositoryConfig([]byte(accepted)) {
			t.Fatalf("rejected valid config %q", accepted)
		}
	}
	for _, rejected := range []string{
		"[include]\npath=/tmp/external\n",
		"[includeIf \"gitdir:/tmp\"]\npath=/tmp/external\n",
		"[include.extra]\npath=/tmp/external\n",
		"[core]\nvalue = escaped\\nline\n",
		"[core] trailing\nvalue=true\n",
		"[core]\nvalue=true # inline\n",
		"[core]\nvalue=fragment#inline\n",
		"[core]\nvalue=semi;inline\n",
		"[core]\r\nvalue=true\r\n",
		"# comment\u0085hidden\n",
		"; comment\u007fhidden\n",
		"value=outside-section\n",
	} {
		if validRepositoryConfig([]byte(rejected)) {
			t.Fatalf("accepted hostile config %q", rejected)
		}
	}
}

func TestBoundedReadersConsumeExactlyOverflowByte(t *testing.T) {
	for name, read := range map[string]func(context.Context, *os.File, uint64) bool{
		"layout": func(ctx context.Context, file *os.File, limit uint64) bool {
			_, ok := readBoundedFile(ctx, file, limit)
			return ok
		},
		"executable": func(ctx context.Context, file *os.File, limit uint64) bool {
			_, _, ok := readDigestPass(ctx, file, limit)
			return ok
		},
	} {
		t.Run(name, func(t *testing.T) {
			file, err := os.CreateTemp(t.TempDir(), "bounded")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if _, err := file.Write([]byte("12345678")); err != nil {
				t.Fatal(err)
			}
			if _, err := file.Seek(0, 0); err != nil {
				t.Fatal(err)
			}
			if read(context.Background(), file, 3) {
				t.Fatal("overflowing input accepted")
			}
			offset, err := file.Seek(0, 1)
			if err != nil {
				t.Fatal(err)
			}
			if offset != 4 {
				t.Fatalf("physical bytes read = %d, want 4", offset)
			}
		})
	}
}
