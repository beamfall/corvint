package main

import (
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

func TestCLIInvocationFramingAndRefusal(t *testing.T) {
	testsupport.CheckAnalyzerBridgeCLI(t, "c-jni")
}
