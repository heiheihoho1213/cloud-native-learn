package main

import (
	"fmt"
	"net/http"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello, Docker + Go Web!")
}

func main() {
	http.HandleFunc("/", helloHandler)
	fmt.Println("Server start at :8080")
	// 监听 0.0.0.0:8080，容器内必须监听 0.0.0.0 才能外部访问
	http.ListenAndServe("0.0.0.0:8080", nil)
}
