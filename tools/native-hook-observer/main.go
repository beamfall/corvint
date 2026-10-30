package main

import (
	"context"
	"os"
)

func main() {
	status := 1
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "observe":
			if len(os.Args) == 2 {
				status = runObserver(context.Background(), os.Stdin, os.Stdout, maxConnection)
			}
		case "client":
			status = runNativeClient(context.Background(), os.Args[2:], os.Stdin, os.Stdout)
		}
	}
	os.Exit(status)
}
