//go:build !darwin && !linux

package depsource

import "os"

// cacheFileOpenFlags relies on os.Root containment and the post-open identity
// match where no-follow and non-blocking open flags are unavailable.
const cacheFileOpenFlags = os.O_RDONLY
