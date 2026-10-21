package taskprojection

import "testing"

func TestOnlyProjection(t *testing.T) {
	for _, tc := range []struct {
		paths []string
		want  bool
	}{
		{nil, false},
		{[]string{".taskman/queue.json"}, true},
		{[]string{".taskman/policy.json", ".taskman/tickets/V1-0001.json", ".taskman/releases/r.json"}, true},
		{[]string{".taskman/queue.json", "main.go"}, false},
		{[]string{".taskman/journal/x.json"}, false},
		{[]string{".taskman/tickets/.json"}, false},
		{[]string{".taskman/tickets/a/b.json"}, false},
	} {
		if got := OnlyProjection(tc.paths); got != tc.want {
			t.Errorf("OnlyProjection(%q) = %v, want %v", tc.paths, got, tc.want)
		}
	}
}
