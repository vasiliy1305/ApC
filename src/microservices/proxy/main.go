package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

var (
	monolithURL      *url.URL
	moviesServiceURL *url.URL
)

func init() {
	var err error
	monolithURL, err = url.Parse(os.Getenv("MONOLITH_URL"))
	if err != nil {
		log.Fatalf("Invalid MONOLITH_URL: %v", err)
	}

	moviesServiceURL, err = url.Parse(os.Getenv("MOVIES_SERVICE_URL"))
	if err != nil {
		log.Fatalf("Invalid MOVIES_SERVICE_URL: %v", err)
	}
}

func main() {
	http.HandleFunc("/api/movies", moviesHandler)
	http.HandleFunc("/api/movies/", moviesHandler)

	http.HandleFunc("/api/users", monolithHandler)
	http.HandleFunc("/api/users/", monolithHandler)

	http.HandleFunc("/api/payments", monolithHandler)
	http.HandleFunc("/api/payments/", monolithHandler)

	http.HandleFunc("/api/subscriptions", monolithHandler)
	http.HandleFunc("/api/subscriptions/", monolithHandler)

	http.HandleFunc("/health", healthHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("Proxy service started on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"status": true}`))
}

func moviesHandler(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, moviesServiceURL)
}

func monolithHandler(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, monolithURL)
}

func proxyRequest(w http.ResponseWriter, r *http.Request, target *url.URL) {
	proxy := httputil.NewSingleHostReverseProxy(target)
	r.Host = target.Host
	log.Printf("Proxying request %s %s to %s", r.Method, r.URL.Path, target)
	proxy.ServeHTTP(w, r)
}
