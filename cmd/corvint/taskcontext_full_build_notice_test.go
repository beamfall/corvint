package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// TCP-V0-140 (V1-1640): a snapshot miss over a large tree names the full
// build, its duration and `corvint index` on stderr; a hit or a small tree
// stays silent.
func TestTaskContextFullBuildNoticeNamesBuildAndIndex(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		build taskContextBuild
		want  string
	}{
		{"large miss", taskContextBuild{tracked: fullBuildNoticeTracked, elapsed: 2500 * time.Millisecond},
			"corvint context: no snapshot of this tree; built the index in memory over 1000 tracked paths in 2.5s. Run `corvint index` once to reuse it.\n"},
		{"small miss", taskContextBuild{tracked: fullBuildNoticeTracked - 1, elapsed: time.Second}, ""},
		{"large hit", taskContextBuild{hit: true, tracked: 50_000}, ""},
	}
	for _, c := range cases {
		var stderr bytes.Buffer
		writeFullBuildNotice(&stderr, c.build)
		if stderr.String() != c.want {
			t.Errorf("%s: stderr %q, want %q", c.name, stderr.String(), c.want)
		}
	}
}

// The compile reports what the notice needs: a miss carries the build time
// and tracked count, and the CLI on a small fixture stays silent.
func TestTaskContextBuildReportsMissTimeAndTrackedCount(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	_, build, err := compileTaskContext(context.Background(), taskContextOptions{root: root, task: "does `Split` keep empty keys", limit: 5, lsp: "off", lspSet: true}, loadContextSnapshot)
	if err != nil || build.hit || build.tracked == 0 || build.tracked >= fullBuildNoticeTracked || build.elapsed <= 0 {
		t.Fatalf("build=%+v err=%v", build, err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--root", root, "context", "--task", "does `Split` keep empty keys"}, strings.NewReader(""), &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}
