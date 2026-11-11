package main

import (
	"sync"
)

var workBoundBuild struct {
	once sync.Once
	path string
	err  error
}
