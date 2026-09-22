package untrackedallowance

import (
	"reflect"
	"testing"
)

func TestParseStatusSeparatesUntrackedFromRenameSources(t *testing.T) {
	raw := []byte("?? notes.md\x00R  new.txt\x00old.txt\x00 M a.go\x00?? b/c.txt\x00")
	untracked, tracked, err := ParseStatus(raw)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"b/c.txt", "notes.md"}; !reflect.DeepEqual(untracked, want) {
		t.Fatalf("untracked=%q, want %q", untracked, want)
	}
	if want := []string{"a.go", "new.txt", "old.txt"}; !reflect.DeepEqual(tracked, want) {
		t.Fatalf("tracked=%q, want %q", tracked, want)
	}
	for _, malformed := range []string{"?? unterminated", "R  new.txt\x00", "?\x00"} {
		if _, _, err := ParseStatus([]byte(malformed)); err == nil {
			t.Fatalf("ParseStatus(%q) accepted malformed status", malformed)
		}
	}
}
