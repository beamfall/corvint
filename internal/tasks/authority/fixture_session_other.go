//go:build !darwin && !linux

package authority

import "os"

func fixtureObserveMount(*os.File, fixtureMount) (fixtureMount, error) {
	return fixtureMount{}, fixtureUnsupported
}
func fixtureLink(*os.File, string, *os.File, string, *os.File) error { return fixtureUnsupported }
func fixtureRename(*os.File, string, *os.File, string, *os.File, *os.File) error {
	return fixtureUnsupported
}
func fixtureUnlink(*os.File, string, *os.File) error { return fixtureUnsupported }
func fixtureMkdir(*os.File, string) error            { return fixtureUnsupported }

// observeMount is fixtureObserveMount; only Linux retains a per-descriptor
// observation file (LGL-V0-002, proposed).
func (s *fixtureSession) observeMount(f *os.File, held fixtureMount) (fixtureMount, error) {
	return fixtureObserveMount(f, held)
}
