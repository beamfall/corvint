package p

import "sync"

var publishFault func() error

var keyed sync.Map

var count int

func publish() error {
	if publishFault != nil {
		return publishFault()
	}
	return nil
}
