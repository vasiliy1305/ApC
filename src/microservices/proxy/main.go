package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	monolithURL, moviesServiceURL, eventsServiceURL *url.URL
	migrationPercent                                 int
	gradualMigration                                 bool
	httpClient                                       *http.Client
)

func mustParse(name string) *url.URL {
	v := strings.TrimSpace(os.Getenv(name))
	u, err := url.Parse(v)
	if err != nil || u == nil || u.Scheme == "" || u.Host == "" {
		log.Fatalf("Invalid %s: %q (%v)", name, v, err)
	}
	return u
}

func init() {
	monolithURL = mustParse("MONOLITH_URL")
	moviesServiceURL = mustParse("MOVIES_SERVICE_URL")

	if ev := strings.TrimSpace(os.Getenv("EVENTS_SERVICE_URL")); ev != "" {
		eventsServiceURL = mustParse("EVENTS_SERVICE_URL")
	}

	if p, err := strconv.Atoi(strings.TrimSpace(os.Getenv("MOVIES_MIGRATION_PERCENT"))); err == nil && p >= 0 && p <= 100 {
		migrationPercent = p
	} else {
		log.Printf("Invalid MOVIES_MIGRATION_PERCENT, defaulting to 0")
		migrationPercent = 0
	}

	gradualMigration = strings.EqualFold(strings.TrimSpace(os.Getenv("GRADUAL_MIGRATION")), "true")

	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   20,
	}
	httpClient = &http.Client{Transport: tr, Timeout: 30 * time.Second}

	rand.Seed(time.Now().UnixNano())
}

func newReverseProxy(target *url.URL) *httputil.ReverseProxy {
	rp := httputil.NewSingleHostReverseProxy(target)
	rp.Transport = httpClient.Transport
	origDirector := rp.Director
	rp.Director = func(r *http.Request) {
		origDirector(r)
		r.Host = target.Host
		if r.Header.Get("X-Request-Id") == "" {
			r.Header.Set("X-Request-Id", fmt.Sprintf("%d", time.Now().UnixNano()))
		}
	}
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("proxy error to %s: %v", target, err)
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}
	return rp
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true}`))
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ready":true}`))
	})

	mux.HandleFunc("/api/movies", moviesHandler)
	mux.HandleFunc("/api/movies/", moviesHandler)

	if eventsServiceURL != nil {
		mux.HandleFunc("/api/events", eventsHandler)
		mux.HandleFunc("/api/events/", eventsHandler)
	}

	mux.HandleFunc("/api/", monolithHandler)

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8000"
	}
	addr := ":" + port

	srv := &http.Server{
		Addr:         addr,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Proxy listening on %s | gradual=%v | percent=%d%%", addr, gradualMigration, migrationPercent)
	log.Printf("Targets: monolith=%s movies=%s events=%v",
		monolithURL, moviesServiceURL, eventsServiceURL != nil)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen error: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutting down proxy...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Println("Proxy stopped")
}

func moviesHandler(w http.ResponseWriter, r *http.Request) {
	target := chooseMoviesTarget()
	log.Printf("-> %s %s %s", targetLabel(target), r.Method, r.URL.Path)
	newReverseProxy(target).ServeHTTP(w, r)
}

func eventsHandler(w http.ResponseWriter, r *http.Request) {
	if eventsServiceURL == nil {
		http.Error(w, "events service not configured", http.StatusNotImplemented)
		return
	}
	log.Printf("-> EVENTS %s %s", r.Method, r.URL.Path)
	newReverseProxy(eventsServiceURL).ServeHTTP(w, r)
}

func monolithHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("-> MONOLITH %s %s", r.Method, r.URL.Path)
	newReverseProxy(monolithURL).ServeHTTP(w, r)
}

func chooseMoviesTarget() *url.URL {
	if !gradualMigration {
		if migrationPercent >= 100 {
			return moviesServiceURL
		}
		return monolithURL
	}
	if rand.Intn(100) < migrationPercent {
		return moviesServiceURL
	}
	return monolithURL
}

func targetLabel(u *url.URL) string {
	if u == nil {
		return "UNKNOWN"
	}
	if u.String() == moviesServiceURL.String() {
		return "MOVIES"
	}
	if u.String() == monolithURL.String() {
		return "MONOLITH"
	}
	if eventsServiceURL != nil && u.String() == eventsServiceURL.String() {
		return "EVENTS"
	}
	return "OTHER"
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = fmt.Sprintf("%d", time.Now().UnixNano())
			r.Header.Set("X-Request-Id", id)
		}
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[%s] %s %s %s", id, r.Method, r.URL.Path, time.Since(start))
	})
}
