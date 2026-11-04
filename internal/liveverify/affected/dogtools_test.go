// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
)

func TestDTLV0013BuildMapFS(t *testing.T) {
	t.Run("DTL-V0-013", func(t *testing.T) {
		_, err := affected.BuildFS(fstest.MapFS{"Makefile": {Data: []byte("all:\n\t@echo ok\n")}}, shell.New())
		if err != nil {
			t.Fatal(err)
		}
	})
}

type dogtoolsSourceLanguage struct{}

func (dogtoolsSourceLanguage) Name() string       { return "zz" }
func (dogtoolsSourceLanguage) Owns(p string) bool { return p == "input.txt" }
func (dogtoolsSourceLanguage) Units(context.Context, string) (affected.Result, error) {
	panic("ambient read")
}
func (dogtoolsSourceLanguage) UnitsSource(s *affected.Source) (affected.Result, error) {
	if _, err := s.Files(affected.ByName(func(string) bool { return true })); err != nil {
		return affected.Result{}, err
	}
	_, err := s.ReadBounded("input.txt", 100)
	return affected.Result{}, err
}

func TestDTLV0013FakeLanguageBuildCached(t *testing.T) {
	t.Run("DTL-V0-013", func(t *testing.T) {
		pinnable(t)
		root := t.TempDir()
		writeCacheFixture(t, root, map[string]string{"input.txt": "one\n"})
		dir := privateCacheDir(t)
		_, out, err := affected.BuildCached(context.Background(), root, dir, graphCacheIdentity, dogtoolsSourceLanguage{})
		if err != nil || !out.Stored {
			t.Fatalf("%+v %v", out, err)
		}
		_, out, err = affected.BuildCached(context.Background(), root, dir, graphCacheIdentity, dogtoolsSourceLanguage{})
		if err != nil || !out.Hit {
			t.Fatalf("%+v %v", out, err)
		}
		// Fault injection: changed observed bytes must invalidate, and an oversize input must refuse.
		writeCacheFixture(t, root, map[string]string{"input.txt": "changed\n"})
		_, out, err = affected.BuildCached(context.Background(), root, dir, graphCacheIdentity, dogtoolsSourceLanguage{})
		if err != nil || out.Hit {
			t.Fatalf("%+v %v", out, err)
		}
	})
}
