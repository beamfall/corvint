package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Beamfall/corvint/internal/opencodequalification"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	code := make(chan int, 1)
	done := make(chan struct{})
	go func() {
		select {
		case s := <-signals:
			n := 143
			if s == os.Interrupt {
				n = 130
			}
			code <- n
			cancel()
		case <-done:
		}
	}()
	err := opencodequalification.Run(ctx, os.Args[1:])
	signal.Stop(signals)
	close(done)
	cancel()
	select {
	case n := <-code:
		os.Exit(n)
	default:
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
