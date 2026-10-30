package runtimeenv

import "testing"

func TestResolveNamespace(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values map[string]string
		want   string
	}{
		{"absent", nil, ""},
		{"current", map[string]string{"CORVINT_X": "value"}, "value"},
		{"empty", map[string]string{"CORVINT_X": ""}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lookup := func(k string) (string, bool) { v, ok := tc.values[k]; return v, ok }
			if got := Resolve(lookup, "X"); got != tc.want {
				t.Fatalf("got %q", got)
			}
		})
	}
}
