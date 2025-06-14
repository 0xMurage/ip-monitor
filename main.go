package main

import (
	"embed"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"ip-monitor/internal/web"
	"ip-monitor/internal/worker"
	"time"
)

//go:embed web/views/*.html
var htmlTemplateFs embed.FS

//go:embed web/static/*
var staticFS embed.FS

const (
	databaseFolder      = ".data"
	dbFileName        = "monitor.db"
	checkInterval     = 30 * time.Second // How often to run the check
)

// Record holds the data for a single network check.
type Record struct {
	Timestamp    time.Time
	IPAddress    string
	Latency      time.Duration
	DownloadMbps float64
	UploadMbps   float64
	Error        string
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile) // Add file/line number to logs
	log.Println("Starting IP Monitor application...")


	if err := os.MkdirAll(databaseFolder, 0755); err != nil {
		log.Fatalf("Failed to create data directory %s: %v", databaseFolder, err)
	}

	dbFilePath := filepath.Join(databaseFolder, dbFileName)

	log.Printf("Database will be stored at: %s", dbFilePath)

	var listenAddr = ":3187" //default listening address + port

	// Get listen port from environment variable or use default
	listenPort := os.Getenv("PORT")
	if listenPort != "" {
		listenAddr = ":"+listenPort
	}

	// Start the network monitoring worker in a separate goroutine
	go worker.Start(dbFilePath, checkInterval)

	staticSubFS, _ := fs.Sub(staticFS, "web/static")

	options := &web.ServerOptions{
		ListenAddr:        listenAddr,
		DatabaseFilepath:  dbFilePath,
		StaticFiles:       staticSubFS,
		HtmlTemplateFiles: htmlTemplateFs,
	}

	// Start the web server (this will block the main goroutine)
	web.StartServer(options)
}



