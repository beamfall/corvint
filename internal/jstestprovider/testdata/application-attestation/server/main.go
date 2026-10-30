package main

import (
	"net/http"
	"os"
	"time"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		client := http.Client{Timeout: time.Second}
		response, err := client.Get("http://127.0.0.1:8080/health")
		if err != nil || response.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		_ = response.Body.Close()
		return
	}
	handler := http.NewServeMux()
	handler.HandleFunc("/health", func(response http.ResponseWriter, _ *http.Request) { response.WriteHeader(http.StatusOK) })
	handler.HandleFunc("/", func(response http.ResponseWriter, _ *http.Request) { _, _ = response.Write([]byte("attested fixture")) })
	if err := http.ListenAndServe(":8080", handler); err != nil {
		os.Exit(1)
	}
}
