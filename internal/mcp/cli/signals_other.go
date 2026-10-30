//go:build !unix

package cli

import "os"

func terminationSignals() []os.Signal { return []os.Signal{os.Interrupt} }
