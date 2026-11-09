// SPDX-License-Identifier: Apache-2.0
package shell

import (
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-3333: matched filenames can feed different read paths. Keep the
// conservative directory relation until that data flow can be proven bounded.
func TestShellDerivedGlobReaders_AFPV03333(t *testing.T) {
	for _, tc := range []struct {
		name, command string
	}{
		{"review_example", `for f in "$ROOT"/script/*_test.sh; do cat "${f%_test.sh}.tsv"; done`},
		{"replace_suffix", `for f in "$ROOT"/script/*_test.sh; do cat "${f/_test.sh/.tsv}"; done`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "script/foo_test.sh", "true\n")
			write(t, root, "script/foo.tsv", "input\n")
			write(t, root, "reader_test.sh", tc.command+"\n")
			plan := affected.Select(build(t, root), []string{"script/foo.tsv"})
			if !slices.Contains(plan.SelectedTests(), "reader_test.sh") {
				t.Fatalf("derived read lost: selected=%v, command=%s", plan.SelectedTests(), tc.command)
			}
		})
	}
}

// AFP-V0-3332: a basename does not establish standard utility semantics.
// A wrapper may read any operand, including the apparent destination.
func TestShellQualifiedUtilityReaders_AFPV03332(t *testing.T) {
	for _, tc := range []struct {
		name, command string
	}{
		{"review_example", `"$READER_BIN/cp" first.txt script/input.tsv`},
		{"computed_mv", `"$READER_BIN/mv" first.txt script/input.tsv`},
		{"computed_mkdir", `"$READER_BIN/mkdir" script/input.tsv`},
		{"project_cp", `tools/cp first.txt script/input.tsv`},
		{"project_mv", `tools/mv first.txt script/input.tsv`},
		{"project_mkdir", `tools/mkdir script/input.tsv`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "first.txt", "first\n")
			write(t, root, "script/input.tsv", "input\n")
			write(t, root, "reader_test.sh", tc.command+"\n")
			plan := affected.Select(build(t, root), []string{"script/input.tsv"})
			if !slices.Contains(plan.SelectedTests(), "reader_test.sh") {
				t.Fatalf("wrapper input lost: selected=%v, command=%s", plan.SelectedTests(), tc.command)
			}
		})
	}
}

// A glob rooted at a private mktemp directory names paths under that
// directory, not repository paths, so it must not select a reader for a
// repository file it cannot match. A parent climb keeps the conservative glob.
func TestShellTempRootedGlobNotRepositoryGlob_AFPV03333(t *testing.T) {
	for _, tc := range []struct {
		name, command string
		want          bool
	}{
		{"temp_root", "tmp=$(mktemp -d)\ncat \"$tmp/script/*.tsv\"", false},
		{"temp_root_braced", "tmp=$(mktemp -d)\ncat \"${tmp}/script/*.tsv\"", false},
		{"parent_climb", "tmp=$(mktemp -d)\ncat \"$tmp/../script/*.tsv\"", true},
		{"unknown_root", "cat \"$ROOT/script/*.tsv\"", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "script/data.tsv", "input\n")
			write(t, root, "reader_test.sh", tc.command+"\n")
			plan := affected.Select(build(t, root), []string{"script/data.tsv"})
			if got := slices.Contains(plan.SelectedTests(), "reader_test.sh"); got != tc.want {
				t.Fatalf("selected=%v want reader=%v, command=%s", plan.SelectedTests(), tc.want, tc.command)
			}
		})
	}
}
