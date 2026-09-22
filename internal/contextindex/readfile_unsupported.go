//go:build !darwin && !linux

package contextindex

type cleanFileOpener struct{}

func newCleanFileOpener(_ string) (*cleanFileOpener, bool) { return nil, false }

func (opener *cleanFileOpener) close() {}

func readCleanFile(_, _ string) ([]byte, bool) { return nil, false }

func readCleanFileUsing(_ *cleanFileOpener, _ string) ([]byte, bool) { return nil, false }

func verifyCleanFile(_, _, _, _ string) (bool, bool) { return false, false }

func verifyCleanFileWithScratch(_, _, _, _ string, _ *cleanFileVerificationScratch) (bool, bool) {
	return false, false
}

func verifyCleanFileUsing(_ *cleanFileOpener, _, _, _ string, _ *cleanFileVerificationScratch) (bool, bool) {
	return false, false
}
