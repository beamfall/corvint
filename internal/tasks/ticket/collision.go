package ticket

import "github.com/Beamfall/corvint/internal/tasks/wire"

// Collide reports whether two resource sets collide under TCP-00 §4.2 and
// CAL-V0-023: WHOLE_REPOSITORY collides with every resource, PATH keys collide
// when equal or when one is a directory key (trailing "/") prefixing the
// other, and every other class collides on an equal class and key.
func Collide(a, b []Resource) bool {
	return CollideShared(a, b, nil)
}

// CollideShared is Collide except that two overlapping PATH keys do not
// collide when shared covers both of them (NTP-V0-026): the policy's
// reservations.sharedPaths name derived files that every lane regenerates,
// so listing one never excludes another lane that lists it too. A PATH key
// that shared does not cover keeps exclusive semantics.
func CollideShared(a, b []Resource, shared []string) bool {
	for _, x := range a {
		for _, y := range b {
			if collides(x, y, shared) {
				return true
			}
		}
	}
	return false
}

func collides(x, y Resource, shared []string) bool {
	if x.Class == "WHOLE_REPOSITORY" || y.Class == "WHOLE_REPOSITORY" {
		return true
	}
	if x.Class != y.Class {
		return false
	}
	if x.Class != "PATH" {
		return x.Key == y.Key
	}
	if !PathCovers(x.Key, y.Key) && !PathCovers(y.Key, x.Key) {
		return false
	}
	return !sharedCovers(shared, x.Key) || !sharedCovers(shared, y.Key)
}

func sharedCovers(shared []string, key string) bool {
	for _, s := range shared {
		if PathCovers(s, key) {
			return true
		}
	}
	return false
}

// PathCovers reports whether PATH key scope covers path: equal bytes, or
// scope is a directory key ending in "/" that prefixes path.
func PathCovers(scope, path string) bool {
	return wire.PathKeyCovers(scope, path)
}
