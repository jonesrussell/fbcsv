package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"csv-search/internal/api"
	"csv-search/internal/models"
	"csv-search/internal/search"
)

func main() {
	// Parse command line flags
	var (
		port       = flag.String("port", "8080", "Server port")
		csvFile    = flag.String("csv", "data.csv", "CSV file path")
		indexPath  = flag.String("index", "index", "Index storage path")
		maxResults = flag.Int("max-results", 1000, "Maximum search results")
		pageSize   = flag.Int("page-size", 50, "Default page size")
		cacheSize  = flag.Int("cache-size", 1000, "Cache size")
		enableCORS = flag.Bool("cors", true, "Enable CORS")
		logLevel   = flag.String("log-level", "info", "Log level")
	)
	flag.Parse()

	// Create configuration
	config := &models.Config{
		Port:        *port,
		CSVFilePath: *csvFile,
		IndexPath:   *indexPath,
		MaxResults:  *maxResults,
		PageSize:    *pageSize,
		CacheSize:   *cacheSize,
		EnableCORS:  *enableCORS,
		LogLevel:    *logLevel,
	}

	// Validate CSV file exists
	if _, err := os.Stat(config.CSVFilePath); os.IsNotExist(err) {
		log.Fatalf("CSV file not found: %s", config.CSVFilePath)
	}

	// Create indexer and searcher
	indexer := search.NewIndexer(config)
	searcher := search.NewSearcher(indexer, config)

	// Create handler
	handler := api.NewHandler(indexer, searcher, config)

	// Setup routes
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/search", handler.SearchHandler)
	mux.HandleFunc("/api/columns", handler.ColumnsHandler)
	mux.HandleFunc("/api/stats", handler.StatsHandler)
	mux.HandleFunc("/api/progress", handler.ProgressHandler)
	mux.HandleFunc("/api/suggestions", handler.SuggestionsHandler)
	mux.HandleFunc("/api/health", handler.HealthHandler)

	// Serve static files
	webDir := filepath.Join(".", "web")
	if _, err := os.Stat(webDir); err == nil {
		mux.Handle("/", http.FileServer(http.Dir(webDir)))
	} else {
		// Fallback for development
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				http.Redirect(w, r, "/index.html", http.StatusTemporaryRedirect)
				return
			}
			http.NotFound(w, r)
		})
	}

	// Apply middleware
	var server http.Handler = mux
	server = handler.CORSHandler(server)
	server = handler.LoggingHandler(server)
	server = handler.RecoveryHandler(server)

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      server,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start indexing in background
	indexCtx, indexCancel := context.WithCancel(context.Background())
	go func() {
		log.Println("Starting CSV indexing...")
		if err := indexer.BuildIndex(indexCtx); err != nil {
			log.Printf("Indexing failed: %v", err)
		} else {
			log.Println("CSV indexing completed successfully")
		}
	}()

	// Start server in background
	go func() {
		log.Printf("Starting server on port %s", config.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Cancel indexing
	indexCancel()

	// Shutdown server gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

// init function to set up logging
func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
