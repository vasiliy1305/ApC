package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	// "strings"
	"math/rand"
	"time"
)

var (
	monolithURL        *url.URL
	moviesServiceURL   *url.URL
	gradualMigration   bool
	migrationPercent   int
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

	gradualMigration = os.Getenv("GRADUAL_MIGRATION") == "true"

	migrationPercent, err = strconv.Atoi(os.Getenv("MOVIES_MIGRATION_PERCENT"))
	if err != nil {
		migrationPercent = 0
	}

	rand.Seed(time.Now().UnixNano())
}

func main() {
	http.HandleFunc("/api/movies", moviesHandler)
	http.HandleFunc("/api/movies/", moviesHandler) // для любых подмаршрутов
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

// Перенаправляет запрос либо на монолит, либо на movies-service
func moviesHandler(w http.ResponseWriter, r *http.Request) {
	target := monolithURL
	if gradualMigration {
		// С вероятностью migrationPercent отправляем на movies
		if rand.Intn(100) < migrationPercent {
			target = moviesServiceURL
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Подменяем хост в запросе, чтобы целевой сервис корректно его принял
	r.Host = target.Host

	log.Printf("Proxying request %s %s to %s", r.Method, r.URL.Path, target)

	proxy.ServeHTTP(w, r)
}
