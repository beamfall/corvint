package ticket

import "strings"

// Collide reports whether two resource sets collide under TCP-00 §4.2 and
// CAL-V0-023: WHOLE_REPOSITORY collides with every resource, PATH keys collide
// when equal or when one is a directory key (trailing "/") prefixing the
// other, and every other class collides on an equal class and key.
func Collide(a, b []Resource) bool {
	for _, x := range a {
		if collidesAny(x, b) {
			return true
		}
	}
	return false
}

func collidesAny(x Resource, b []Resource) bool {
	for _, y := range b {
		if collides(x, y) {
			return true
		}
	}
	return false
}

func collides(x, y Resource) bool {
	if x.Class == "WHOLE_REPOSITORY" || y.Class == "WHOLE_REPOSITORY" {
		return true
	}
	if x.Class != y.Class {
		return false
	}
	if x.Class != "PATH" {
		return x.Key == y.Key
	}
	return PathCovers(x.Key, y.Key) || PathCovers(y.Key, x.Key)
}

// PathCovers reports whether PATH key scope covers path: equal bytes, or
// scope is a directory key ending in "/" that prefixes path.
func PathCovers(scope, path string) bool {
	if scope == path {
		return true
	}
	return strings.HasSuffix(scope, "/") && strings.HasPrefix(path, scope)
}
