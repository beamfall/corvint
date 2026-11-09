// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"context"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/testsupport"
)

// AFP-V0-3323: disk and immutable callers attach content evidence, while an
// absent base keeps the existing conservative result.
func TestSelectCommentOnlyEndpoints_AFP_V0_3323(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git unavailable")
	}
	root := t.TempDir()
	files := map[string]string{
		"go.mod":            "module example.test/g\n",
		"lib/lib.go":        "package lib\nfunc F() {}\n",
		"lib/lib_test.go":   "package lib\n",
		"user/user.go":      "package user\nimport _ \"example.test/g/lib\"\n",
		"user/user_test.go": "package user\n",
	}
	writeFixture(t, root, files)
	base := fstest.MapFS{}
	for name, body := range files {
		base[name] = &fstest.MapFile{Data: []byte(body)}
	}
	testsupport.RunGit(t, git, root, "init", "--quiet")
	testsupport.RunGit(t, git, root, "add", "-A")
	testsupport.RunGit(t, git, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "--quiet", "-m", "fixture")
	commit := strings.TrimSpace(testsupport.RunGit(t, git, root, "rev-parse", "HEAD"))
	writeFixture(t, root, map[string]string{"lib/lib.go": files["lib/lib.go"] + "// context\n"})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	dirty := []string{"lib/lib.go"}
	disk := SelectAt(context.Background(), root, root, commit, graph, dirty)
	immutable := SelectBetween(base, os.DirFS(root), graph, dirty)
	if !reflect.DeepEqual(disk, immutable) || len(disk.Selected) != 1 || disk.Selected[0].UnitID != "go:example.test/g/lib" || len(disk.GoChanges) != 1 {
		t.Fatalf("endpoint plans differ or did not narrow: disk=%+v immutable=%+v", disk, immutable)
	}
	unknown := SelectAt(context.Background(), root, root, "", graph, dirty)
	if !reflect.DeepEqual(unknown, SelectBetween(nil, os.DirFS(root), graph, dirty)) || len(unknown.Selected) != 2 || len(unknown.GoChanges) != 0 {
		t.Fatalf("unknown endpoint did not fail closed: %+v", unknown)
	}
}
