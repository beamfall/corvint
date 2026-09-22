package main

import (
	"context"
	"io"
	"time"

	"github.com/Beamfall/corvint/internal/authorityevent"
	"github.com/Beamfall/corvint/internal/authoritystore"
)

// The default binary cannot accept a root, policy, receipt or resolver from the
// caller. Only the fixed independently accepted operator store can resolve.
func runAuthorityEvent(parent context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	native := len(args) == 3 && args[2] == "--native-output"
	unavailable := func(reason string) int {
		if native {
			if emit(stdout, authorityevent.NativeOutput(authorityevent.Result{})) != nil {
				return 2
			}
			return 0
		}
		return emitAuthorityEventUnavailable(stdout, reason)
	}
	if (len(args) != 2 && !native) || args[0] != "--input" || args[1] != "-" {
		return unavailable("invalid-authority-event-arguments")
	}
	ctx, cancel := context.WithTimeout(parent, 1600*time.Millisecond)
	defer cancel()
	raw, err := readInputBounded(ctx, stdin, authorityevent.MaxInput)
	if err != nil {
		return unavailable("authority-event-input-unavailable")
	}
	event, err := authorityevent.Parse(raw)
	if err != nil {
		return unavailable("invalid-authority-event")
	}
	result := authorityevent.Handle(ctx, event, authoritystore.Resolve)
	if native {
		if emit(stdout, authorityevent.NativeOutput(result)) != nil {
			return 2
		}
		return 0
	}
	if emit(stdout, result) != nil {
		return 2
	}
	return 0
}

func emitAuthorityEventUnavailable(stdout io.Writer, reason string) int {
	result := authorityevent.Result{Profile: authorityevent.Profile, Authority: "NONE", Support: "UNAVAILABLE", State: "UNKNOWN", Decision: "release", Reason: reason}
	if emit(stdout, result) != nil {
		return 2
	}
	return 0
}
