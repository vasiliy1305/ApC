package main

import (
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"time"
)

var (
	monolithURL      *url.URL
	moviesServiceURL *url.URL
	migrationPercent int
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

	migrationPercentStr := os.Getenv("MOVIES_MIGRATION_PERCENT")
	migrationPercent, err = strconv.Atoi(migrationPercentStr)
	if err != nil || migrationPercent < 0 || migrationPercent > 100 {
		log.Printf("Invalid MOVIES_MIGRATION_PERCENT: %v, defaulting to 0", err)
		migrationPercent = 0
	}

	rand.Seed(time.Now().UnixNano())
}

func main() {
	http.HandleFunc("/api/movies", moviesHandler)
	http.HandleFunc("/api/movies/", moviesHandler)
	http.HandleFunc("/api/", monolithHandler) // <-- Добавляем
	http.HandleFunc("/health", healthHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("Proxy service started on port %s with %d%% migration to Movies Service", port, migrationPercent)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func monolithHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Routing to MONOLITH: %s", monolithURL.String())

	proxy := httputil.NewSingleHostReverseProxy(monolithURL)
	r.Host = monolithURL.Host
	proxy.ServeHTTP(w, r)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"status": true}`))
}

func moviesHandler(w http.ResponseWriter, r *http.Request) {
	var target *url.URL

	if rand.Intn(100) < migrationPercent {
		target = moviesServiceURL
		log.Printf("Routing to MOVIES_SERVICE: %s", target.String())
	} else {
		target = monolithURL
		log.Printf("Routing to MONOLITH: %s", target.String())
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	r.Host = target.Host
	proxy.ServeHTTP(w, r)
}
