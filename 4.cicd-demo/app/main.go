package main

import (
	"fmt"
	"net/http"
	"os"
)

// Version 由 CI/CD 构建时注入，用于验证是否部署了新版本
var Version = "dev"

func main() {
	version := os.Getenv("APP_VERSION")
	if version == "" {
		version = Version
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "CI/CD Demo - version: %s\n", version)
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "OK")
	})

	fmt.Println("listening on :8080, version:", version)
	http.ListenAndServe(":8080", nil)
}
