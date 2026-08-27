package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dlx/downloader"
	"dlx/server"
)

const version = "1.0.0"

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[DLX] ")

	cfg := LoadConfig()

	log.Printf("Starting DLX v%s (Video Downloader)", version)
	log.Printf("Configuration: Port=%s, MaxConcurrent=%d, MaxFileSize=%s, Timeout=%s",
		cfg.Port, cfg.MaxConcurrentDownloads, cfg.MaxFileSize, cfg.DownloadTimeout)
	log.Printf("Paths: TempDir=%s, YtDlp=%s, FFmpeg=%s", cfg.TempDir, cfg.YtDlpPath, cfg.FFmpegPath)

	// Initialize Job Manager
	jobManager := downloader.NewJobManager(
		cfg.YtDlpPath,
		cfg.FFmpegPath,
		cfg.MaxFileSize,
		cfg.TempDir,
		cfg.DownloadTimeout,
		cfg.MaxConcurrentDownloads,
		TranslateError,
		SanitizeFilename,
	)

	// Startup cleanup of temporary files
	jobManager.CleanupStartup()

	// Extractor instance for info fetching
	extractor := &downloader.YtDlpExtractor{
		YtDlpPath:  cfg.YtDlpPath,
		FFmpegPath: cfg.FFmpegPath,
	}

	// Setup HTTP Handlers
	handlers := &server.Handlers{
		JobManager:  jobManager,
		Extractor:   extractor,
		ValidateURL: ValidateURL,
		InfoTimeout: 15 * time.Second,
	}

	httpHandler := server.Router(handlers)

	listenAddr := fmt.Sprintf("0.0.0.0:%s", cfg.Port)
	httpServer := &http.Server{
		Addr:              listenAddr,
		Handler:           httpHandler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Channel to listen for errors coming from the listener
	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("Server is ready and listening on http://%s", listenAddr)
		serverErrors <- httpServer.ListenAndServe()
	}()

	// Graceful shutdown channel
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Fatal server error: %v", err)
		}
	case sig := <-shutdown:
		log.Printf("Received signal %v. Initiating graceful shutdown...", sig)

		// Context for shutdown timeout
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		// 1. Stop HTTP listener
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down HTTP server: %v", err)
			_ = httpServer.Close()
		}

		// 2. Shutdown active jobs and clean temporary resources
		jobManager.Shutdown(ctx)

		log.Println("DLX server shutdown complete. Goodbye!")
	}
}
