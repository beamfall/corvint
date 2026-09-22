//go:build beamfall_appstore

package main

import "github.com/beamfall/core/internal/app"

func firstPartyHooks() app.Hooks {
	return firstPartyHooksForBundles(firstPartyBundles())
}
