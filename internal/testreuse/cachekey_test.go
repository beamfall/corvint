// SPDX-License-Identifier: Apache-2.0

package testreuse

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestValidationKeysDoNotCollide covers AFP-V0-2321: the validation cache
// keys each check by its kind and path, so a tracked file whose name spells
// another kind's key (here walk:a, declared as a file scope) cannot stand in
// for the walk of package a's directory. An empty directory below a must
// still make a run; the unchanged checkout is the negative control.
func TestValidationKeysDoNotCollide(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a colon is not valid in a Windows file name")
	}
	root := fixtureRepo(t)
	writeFixture(t, root, "walk:a", "a file named like a walk key\n")
	writeFixture(t, root, ReadScopesPath, `{"profile":"corvint-test-read-scopes/0","packages":{"a":["data/a.txt","walk:a"],"b":[]}}`+"\n")
	commitAll(t, root, "colliding scope")
	record := recordAt(t, root, Source{Kind: SourceLocal}, allPassed)
	wantUnits(t, "unchanged checkout", decideAt(t, root, localPolicy, record), map[string]string{"a": ReasonRecordDigestEqual, "b": ReasonRecordDigestEqual})
	if err := os.MkdirAll(filepath.Join(root, "a", "testdata", "extra"), 0o755); err != nil {
		t.Fatal(err)
	}
	wantUnits(t, "empty directory below a", decideAt(t, root, localPolicy, record), map[string]string{"a": ReasonUntrackedGrantedEntry, "b": ReasonRecordDigestEqual})
}
