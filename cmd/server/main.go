package main

import (
	"net/http"
	"os"

	"subkit/internal/logger"
	"subkit/internal/scheduler"
	"subkit/internal/server"

	"github.com/joho/godotenv"
)

func main() {
	logger.Println("========================================")
	logger.Println("   Subkit - Proxy Converter Service")
	logger.Println("========================================")

	logger.Info("Loading environment variables...")
	if err := godotenv.Load(); err != nil {
		logger.Warn("No .env file found, using system environment")
	}

	logger.Init()

	logger.Info("Initializing server...")
	srv, err := server.NewServer()
	if err != nil {
		logger.Error("Create server failed: %v", err)
		os.Exit(1)
	}
	logger.Info("Server initialized successfully")

	logger.Info("Starting rule list updater (2-day interval)...")
	updater := scheduler.NewUpdater(2)
	updater.Start()

	logger.Info("Initializing rate limiter...")
	rateLimiter := server.NewRateLimiter()

	logger.Info("Registering HTTP routes...")
	http.Handle("/", http.FileServer(http.Dir("web/static")))
	http.HandleFunc("/api/convert", rateLimiter.Middleware(srv.HandleConvert))
	http.HandleFunc("/api/convert-stream", rateLimiter.Middleware(srv.HandleConvertStream))
	http.HandleFunc("/api/extract-nodes", rateLimiter.Middleware(srv.HandleExtractNodes))
	http.HandleFunc("/subscribe/", srv.HandleSubscribe)
	http.HandleFunc("/api/node-to-uri", srv.HandleNodeToURI)
	http.HandleFunc("/api/uri-to-node", srv.HandleURIToNode)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Println("========================================")
	logger.Info("🚀 Server started on http://0.0.0.0:%s", port)
	logger.Println("========================================")
	logger.Info("Available endpoints:")
	logger.Info("  - GET  /                      Web interface")
	logger.Info("  - POST /api/convert           Convert subscription/URIs with LLM")
	logger.Info("  - POST /api/convert-stream    Convert with progress (SSE)")
	logger.Info("  - POST /api/extract-nodes     Extract nodes from subscription (no LLM)")
	logger.Info("  - GET  /subscribe/{id}        Download config")
	logger.Info("  - POST /api/node-to-uri       Convert node to URI")
	logger.Info("  - POST /api/uri-to-node       Convert URI to node")
	logger.Println("========================================")

	if err := http.ListenAndServe("0.0.0.0:"+port, nil); err != nil {
		logger.Error("Server failed: %v", err)
		os.Exit(1)
	}
}
