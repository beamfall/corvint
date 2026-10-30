package main

import cemcli "github.com/Beamfall/corvint/internal/cem/cli"

// proveCEMUnknownArgument refuses an argument the `prove --cem` parsers do not
// know. A paired-change flag gets the CEM-V1-648 reason `cem` gives, since
// `prove --cem` reads the same single-repository cem/1.0 maps; anything else
// is the argparse unrecognized-arguments line.
func proveCEMUnknownArgument(token string) error {
	if message, paired := cemcli.PairedChangeRefusal([]string{token}); paired {
		return argumentError(message)
	}
	return argumentError("unrecognized arguments: " + token)
}
