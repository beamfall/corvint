package main

import (
	"runtime/debug"
	"strings"
	"testing"
)

func build(path, revision, modified string) *debug.BuildInfo {
	info := &debug.BuildInfo{Path: path, Main: debug.Module{Path: corvintModule}}
	if path == cem01Main {
		info.Main.Path = cem01Main
	}
	if revision != "" {
		info.Settings = append(info.Settings, debug.BuildSetting{Key: "vcs", Value: "git"}, debug.BuildSetting{Key: "vcs.revision", Value: revision})
	}
	if modified != "" {
		info.Settings = append(info.Settings, debug.BuildSetting{Key: "vcs.modified", Value: modified})
	}
	return info
}

// TestCEMV1572ReleaseBuildsMustMatch pins the provenance refusal: the two
// release binaries must be corvint and cem01-go built from one clean commit.
func TestCEMV1572ReleaseBuildsMustMatch(t *testing.T) {
	const a, b = "1111111111111111111111111111111111111111", "2222222222222222222222222222222222222222"
	cases := []struct {
		name           string
		corvint, cem01 *debug.BuildInfo
		refusal        string
	}{
		{"same clean commit", build(corvintMain, a, "false"), build(cem01Main, a, "false"), ""},
		{"different commits", build(corvintMain, a, "false"), build(cem01Main, b, "false"), "builds differ"},
		{"modified corvint", build(corvintMain, a, "true"), build(cem01Main, a, "false"), "modified worktree"},
		{"modified unknown", build(corvintMain, a, "false"), build(cem01Main, a, ""), "modified worktree"},
		{"no vcs stamp", build(corvintMain, "", ""), build(cem01Main, a, "false"), "no Git vcs.revision"},
		{"wrong corvint main", build(corvintModule+"/cmd/corvint-tasks", a, "false"), build(cem01Main, a, "false"), "not " + corvintMain},
		{"binaries swapped", build(cem01Main, a, "false"), build(corvintMain, a, "false"), "not " + corvintMain},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			revision, err := sameBuild(c.corvint, c.cem01)
			if c.refusal == "" {
				if err != nil || revision != a {
					t.Fatalf("sameBuild = %q, %v; want %s", revision, err, a)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.refusal) {
				t.Fatalf("sameBuild error %v, want %q", err, c.refusal)
			}
		})
	}
}

func TestWithoutRuntimeKeepsMemberOrder(t *testing.T) {
	line := []byte(`{"axes":{"a":"B"},"runtime":{"goVersion":"go1.27.2","structuralQualification":"NOT_REQUIRED"},"repositoryEnvelope":"x","verdictSha256":"d"}`)
	if got := string(withoutRuntime(line)); got != `{"axes":{"a":"B"},"repositoryEnvelope":"x","verdictSha256":"d"}` {
		t.Fatalf("withoutRuntime = %s", got)
	}
}
