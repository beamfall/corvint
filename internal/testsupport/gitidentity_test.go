// SPDX-License-Identifier: Apache-2.0

package testsupport

import (
	"reflect"
	"testing"
)

// CCF-V1-9800: strip all six variables, including duplicates and Windows casing,
// while preserving unrelated configuration and the caller's slice.
func TestWithoutGitIdentity_CCFV19800(t *testing.T) {
	environment := []string{"PATH=bin", "GIT_AUTHOR_NAME=foreign", "GIT_AUTHOR_EMAIL=f@x", "GIT_AUTHOR_DATE=tomorrow",
		"GIT_COMMITTER_NAME=foreign", "GIT_COMMITTER_EMAIL=f@x", "GIT_COMMITTER_DATE=tomorrow",
		"git_author_name=duplicate", "GIT_CONFIG_GLOBAL=fixture", "OTHER=a=b"}
	before := append([]string(nil), environment...)
	want := []string{"PATH=bin", "GIT_CONFIG_GLOBAL=fixture", "OTHER=a=b"}
	if got := WithoutGitIdentity(environment); !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered environment = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(environment, before) {
		t.Fatal("filter changed the caller's environment slice")
	}
}
