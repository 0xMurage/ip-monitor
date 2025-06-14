package worker

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
	"github.com/showwin/speedtest-go/speedtest"
)


	
// Record holds the data for a single network check.
// Ensure fields are exported (start with uppercase) for template access.
type Record struct {
	ID           int64
	Timestamp    time.Time
	IPAddress    string
	Latency      time.Duration
	DownloadMbps float64
	UploadMbps   float64
	Error        string
}

// Start initializes the database and begins the monitoring loop.
func Start(dbFilePath string, checkInterval time.Duration) {

	log.Println("Worker: Initializing and starting network monitoring...")

	// Ensure the directory for the database file exists
	dbDir := filepath.Dir(dbFilePath)
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		if dbDir != "." && dbDir != "" { // Avoid trying to create "." or an empty dir
			if err := os.MkdirAll(dbDir, 0755); err != nil {
				log.Fatalf("Worker: Failed to create database directory %s: %v", dbDir, err)
			}
		}
	}

	db, err := initDB(dbFilePath)
	if err != nil {
		log.Fatalf("Worker: Failed to initialize database: %v", err)
	}
	defer db.Close()


	log.Printf("Worker: Database initialized at %s. Will check network status every %v.", dbFilePath, checkInterval)

	// Run the first check immediately
	performCheck(db)

	// Start the ticker for periodic checks
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		performCheck(db)
	}
}

func initDB(filePath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS network_records (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"timestamp" DATETIME NOT NULL,
		"ip_address" TEXT,
		"latency_ms" REAL,
		"download_mbps" REAL,
		"upload_mbps" REAL,
		"error" TEXT
	);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create table: %w", err)
	}
	return db, nil
}

func performCheck(db *sql.DB) {
	log.Println("Worker: Performing network check...")
	var record Record
	record.Timestamp = time.Now().UTC() // Store in UTC

	ip, err := getPublicIP()
	if err != nil {
		record.Error = fmt.Sprintf("IP check failed: %v", err)
		log.Printf("Worker: %s", record.Error)
	} else {
		record.IPAddress = ip
		log.Printf("Worker: Public IP: %s", record.IPAddress)
	}

	latency, dl, ul, speedErr := getNetworkMetrics()
	if speedErr != nil {
		if record.Error != "" {
			record.Error += "; "
		}
		errorMsg := fmt.Sprintf("Speed test failed: %v", speedErr)
		record.Error += errorMsg
		log.Printf("Worker: %s", errorMsg)
	} else {
		record.Latency = latency
		record.DownloadMbps = dl
		record.UploadMbps = ul
		log.Printf("Worker: Latency: %v, Download: %.2f Mbps, Upload: %.2f Mbps", latency, dl, ul)
	}

	if err := insertRecord(db, record); err != nil {
		log.Printf("Worker: ERROR - Failed to save record to database: %v", err)
	} else {
		log.Println("Worker: Successfully saved record to database.")
	}
}

func getPublicIP() (string, error) {
	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func getNetworkMetrics() (latency time.Duration, downloadMbps, uploadMbps float64, err error) {
	client := speedtest.New()
	serverList, err := client.FetchServers()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("could not fetch server list: %w", err)
	}
	targets, err := serverList.FindServer([]int{})
	if err != nil || len(targets) == 0 {
		return 0, 0, 0, fmt.Errorf("could not find a suitable server: %w", err)
	}
	s := targets[0]
	if err = s.PingTest(nil); err != nil {
		return 0, 0, 0, fmt.Errorf("ping test failed: %w", err)
	}
	if err = s.DownloadTest(); err != nil {
		return 0, 0, 0, fmt.Errorf("download test failed: %w", err)
	}
	if err = s.UploadTest(); err != nil {
		return 0, 0, 0, fmt.Errorf("upload test failed: %w", err)
	}
	return s.Latency, s.DLSpeed.Mbps(), s.ULSpeed.Mbps(), nil
}

func insertRecord(db *sql.DB, record Record) error {
	insertSQL := `
	INSERT INTO network_records(timestamp, ip_address, latency_ms, download_mbps, upload_mbps, error)
	VALUES (?, ?, ?, ?, ?, ?)`
	stmt, err := db.Prepare(insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()
	_, err = stmt.Exec(
		record.Timestamp,
		record.IPAddress,
		float64(record.Latency.Milliseconds()),
		record.DownloadMbps,
		record.UploadMbps,
		record.Error,
	)
	if err != nil {
		return fmt.Errorf("failed to execute insert statement: %w", err)
	}
	return nil
}
