package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

// version is bumped on each demo commit to make rolling updates visible.
const version = "v7"

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		hostname, _ := os.Hostname()
		fmt.Fprintf(w, "Hello from git-deploy-helloworld %s (pod %s)\n", version, hostname)
	})
	log.Println("listening on :8080, version", version)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
