//go:build !darwin

package jstestprovider

import "testing"

func ptfLivePort(t *testing.T) { t.Helper(); t.Fatal("PTF live tuple requires Darwin") }
