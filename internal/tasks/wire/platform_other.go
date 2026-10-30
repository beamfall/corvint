//go:build !darwin && !linux

package wire

// platformRefusal is the up-front Tasks refusal on a platform without the
// qualified safe opening boundary and POSIX PathText (CTS-V0-025).
var platformRefusal = Errorf(CodeUnsupported, "platform", "Corvint Tasks is unsupported on this platform")

// PlatformRefusal returns the UNSUPPORTED refusal every Tasks command makes on
// this platform before it reads or writes any state (CTS-V0-025).
func PlatformRefusal() error { return platformRefusal }
