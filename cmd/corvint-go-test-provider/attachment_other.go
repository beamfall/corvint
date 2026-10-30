//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package main

import (
	"context"
	"os"
)

func safeAttachmentFile(os.FileInfo) bool { return false }

func supportedProviderHost() bool { return false }

func providerContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

func readParentCapability() ([]byte, bool) { return nil, false }
