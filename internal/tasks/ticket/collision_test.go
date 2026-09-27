package ticket

import "testing"

// TestCALV0023_CollisionNormalization pins the §4.2 path rules and the
// WHOLE_REPOSITORY rule.
func TestCALV0023_CollisionNormalization(t *testing.T) {
	p := func(k string) Resource { return Resource{Class: "PATH", Key: k} }
	whole := Resource{Class: "WHOLE_REPOSITORY", Key: "repo"}
	for _, c := range []struct {
		a, b Resource
		want bool
	}{
		{p("a/b.go"), p("a/b.go"), true},
		{p("a/"), p("a/b.go"), true},
		{p("a/b.go"), p("a/"), true},
		{p("a/"), p("a/c/"), true},
		{p("a/"), p("ab/c.go"), false},
		{p("a/b.go"), p("a/c.go"), false},
		{p("a"), p("a/b.go"), false},
		{whole, p("x"), true},
		{p("x"), whole, true},
		{Resource{Class: "PORT", Key: "8080"}, Resource{Class: "PORT", Key: "8080"}, true},
		{Resource{Class: "PORT", Key: "8080"}, Resource{Class: "PORT", Key: "8081"}, false},
		{Resource{Class: "PORT", Key: "8080"}, p("8080"), false},
	} {
		if got := Collide([]Resource{c.a}, []Resource{c.b}); got != c.want {
			t.Errorf("Collide(%v, %v) = %v", c.a, c.b, got)
		}
	}
	if Collide(nil, []Resource{whole}) {
		t.Error("an empty set collided")
	}
}
