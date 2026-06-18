package main

import (
	"fmt"
	"net/http"
	"os"
)

func handler(w http.ResponseWriter, r *http.Request) {
	version := os.Getenv("VERSION")
	fmt.Fprintf(w, "Hello Go Service | Version: %s\n", version)
}

func main() {
	http.HandleFunc("/", handler)
	fmt.Println("Service start on :8080")
	_ = http.ListenAndServe(":8080", nil)
}
