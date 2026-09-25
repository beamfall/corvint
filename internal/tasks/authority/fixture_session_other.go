//go:build !darwin && !linux

package authority

import "os"

func fixtureObserveMount(*os.File) (fixtureMount, error)             { return fixtureMount{}, fixtureUnsupported }
func fixtureLink(*os.File, string, *os.File, string, *os.File) error { return fixtureUnsupported }
func fixtureRename(*os.File, string, *os.File, string, *os.File, *os.File) error {
	return fixtureUnsupported
}
func fixtureUnlink(*os.File, string, *os.File) error { return fixtureUnsupported }
func fixtureMkdir(*os.File, string) error            { return fixtureUnsupported }
