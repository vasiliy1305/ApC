package main

import (
    "fmt"
    "log"
    "net/http"
)

func main() {
    http.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
        log.Println("Received event request")
        fmt.Fprintf(w, "Event received")
    })

    log.Println("Starting events service on port 8082")
    log.Fatal(http.ListenAndServe(":8082", nil))
}
