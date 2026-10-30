package main

import (
	"context"
	"github.com/Beamfall/corvint/internal/runtimeenv"
	"os"
)

func adapterLookup(ctx context.Context) runtimeenv.Lookup {
	if lookup, ok := ctx.Value(adapterEnvKey{}).(runtimeenv.Lookup); ok {
		return lookup
	}
	return os.LookupEnv
}
