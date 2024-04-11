package main

import (
	"fmt"
	"net/http"
  "log"
	"os"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("REQUEST RECEIVED: Handling HTTP request.")
    hostname, err := os.Hostname()
		if err != nil {
			http.Error(w, "Could not get hostname", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "Hostname: %s\n", hostname)
	})

	fmt.Println("Server started on port 8080")
	http.ListenAndServe(":8080", nil)
}

