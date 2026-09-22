//go:build !darwin

package main

import (
	"errors"
	"net"
)

func platformPeerIdentity(_ net.Conn, _, _ string) (peerIdentity, error) {
	return peerIdentity{}, errors.New("unsupported-platform")
}
