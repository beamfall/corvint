//go:build !darwin && !linux

package observations

import (
	"fmt"
	"os"
)

func lockObservationFile(*os.File) error {
	return fmt.Errorf("serialized self-observation writes are unsupported on this platform")
}

func unlockObservationFile(*os.File) {}
