//go:build !darwin

package authoritystore

import "context"

func openProtectedFiles() (protectedFiles, error)       { return nil, errUnavailable }
func verifyRuntime(context.Context, RootDocument) error { return errUnavailable }

func verifyRuntimePins(context.Context, RootDocument, runtimePins) error { return errUnavailable }

func verifyDirectRuntime(context.Context, RootDocument, DirectRuntime) error { return errUnavailable }
func verifyPiRuntime(context.Context, RootDocument, PiRuntime) error         { return errUnavailable }
