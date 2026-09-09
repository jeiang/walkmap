package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// server holds the dependencies shared by the API handlers.
type server struct {
	db       *pgxpool.Pool
	valhalla *valhallaClient
}

func cmdServe(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	listen := fs.String("listen", getenv("WALKMAP_LISTEN", "127.0.0.1:8867"), "listen address (loopback/mesh only, never expose directly)")
	valhallaURL := fs.String("valhalla-url", getenv("VALHALLA_URL", "http://127.0.0.1:8002"), "valhalla service base URL")
	staticDir := fs.String("static-dir", os.Getenv("WALKMAP_STATIC_DIR"), "directory of built SPA assets to serve at / (optional; piece 3)")
	basemap := fs.String("basemap", os.Getenv("WALKMAP_BASEMAP"), "path to basemap.pmtiles, served at /basemap.pmtiles (optional; piece 3)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL is not set")
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	s := &server{
		db:       pool,
		valhalla: newValhallaClient(*valhallaURL),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /api/categories", s.handleCategories)
	mux.HandleFunc("GET /api/nearby", s.handleNearby)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("GET /api/route", s.handleRoute)
	if *basemap != "" {
		mux.HandleFunc("GET /basemap.pmtiles", handleBasemap(*basemap))
	}
	if *staticDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(*staticDir)))
	} else {
		mux.HandleFunc("/", handlePlaceholder)
	}

	// Identity comes from the edge's X-Remote-User header; this service
	// does no authentication of its own, so the listener above must never
	// be reachable except from loopback and the NetBird mesh.
	log.Printf("walkmap: listening on %s (valhalla=%s)", *listen, *valhallaURL)
	return http.ListenAndServe(*listen, logRequests(mux))
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// handleBasemap serves basemap.pmtiles from disk. It lives outside
// staticDir (data, rebuilt nightly, not part of the SPA build) but the
// PMTiles JS client needs Range support, which http.ServeFile provides.
func handleBasemap(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path)
	}
}

func handlePlaceholder(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "walkmap: no static SPA configured (set WALKMAP_STATIC_DIR); API is under /api")
}

// logRequests logs method, path, status, duration, and the edge-supplied
// identity header (if present) to stderr.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		user := r.Header.Get("X-Remote-User")
		if user == "" {
			user = "-"
		}
		log.Printf("%s %s %s %d %s user=%s", r.RemoteAddr, r.Method, r.URL.RequestURI(), sw.status, time.Since(start), user)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
