package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/patriciabonaldy/queryservice/internal/api"
	"github.com/patriciabonaldy/queryservice/internal/config"
)

func main() {
	// Load configuration
	cfg := config.Load()

	log.Printf("Starting Query Service...")
	log.Printf("Database: %s", cfg.DBPath)
	log.Printf("LLM: %s (model: %s)", cfg.LLMBaseURL, cfg.LLMModel)

	// Open database connection
	db, err := sql.Open("sqlite3", cfg.DBPath+"?mode=ro")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Printf("Database connection established")

	// Create API handler
	handler := api.NewHandler(db, cfg.LLMBaseURL, cfg.LLMModel)

	// Create HTTP server
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Add CORS middleware
	corsHandler := corsMiddleware(mux)

	// Add logging middleware
	loggedHandler := loggingMiddleware(corsHandler)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      loggedHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Query Service listening on http://%s", addr)
		log.Printf("Endpoints:")
		log.Printf("  POST /api/query  - Execute natural language queries")
		log.Printf("  GET  /api/health - Health check")
		log.Printf("  GET  /api/schema - View available schema")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

// corsMiddleware adds CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs incoming requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create response wrapper to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		log.Printf("%s %s %d %v", r.Method, r.URL.Path, rw.statusCode, time.Since(start))
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
