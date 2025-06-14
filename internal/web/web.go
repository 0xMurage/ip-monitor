package web

import (
	"database/sql"
	"fmt"
	"html/template"
	"io/fs"
	"ip-monitor/internal/worker"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

type ServerOptions struct {
	DatabaseFilepath  string
	ListenAddr        string
	StaticFiles       fs.FS
	HtmlTemplateFiles fs.FS
}

// TemplateData holds all the data needed for rendering the index.html template.
type TemplateData struct {
	Records     []worker.Record
	CurrentPage int
	TotalPages  int
	NextPage    int
	PrevPage    int
	HasNextPage bool
	HasPrevPage bool
	Pages       []int // For displaying page number links
	Error       string
}

// StartServer initializes and starts the web server.
func StartServer(options *ServerOptions) {
	
	// Serve static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(options.StaticFiles))))


	// Handle requests to the root path
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleIndex(w, r, options.HtmlTemplateFiles, options.DatabaseFilepath)
	})

	log.Printf("Web server listening on %s", options.ListenAddr)

	if err := http.ListenAndServe(options.ListenAddr, nil); err != nil {
		log.Fatalf("Web server failed: %v", err)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request, HtmlTemplateFiles fs.FS, DatabaseFilepath string) {
	const pageSize = 20 // Records per page

	pageStr := r.URL.Query().Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	//template

	var funcMap = template.FuncMap{
		"formatTime": func(t time.Time) string {
			// Display in local time for readability
			return t.Local().Format("2006-01-02 15:04:05")
		},
		"formatDuration": func(d time.Duration) string {
			return d.String()
		},
		"sub": func(a, b int) int {
			return a - b
		},
	}

	// Pre-parse the template. Funcs allows calling functions from template.
	tmpl := template.Must(template.New("index.html").Funcs(funcMap).ParseFS(HtmlTemplateFiles, "web/views/index.html"))

	data := TemplateData{CurrentPage: page}

	db, err := sql.Open("sqlite3", DatabaseFilepath+"?mode=ro") // Open in read-only mode
	if err != nil {
		log.Printf("Web: Error opening database: %v", err)
		data.Error = "Could not connect to the database."
		executeTemplate(w, tmpl, data)
		return
	}
	defer db.Close()

	totalRecords, err := fetchTotalRecordsCount(db)
	if err != nil {
		log.Printf("Web: Error fetching total records count: %v", err)
		data.Error = "Could not retrieve record count."
		executeTemplate(w, tmpl, data)
		return
	}

	data.TotalPages = int(math.Ceil(float64(totalRecords) / float64(pageSize)))
	if data.TotalPages == 0 {
		data.TotalPages = 1 // Ensure at least one page even if no records
	}
	if page > data.TotalPages { // If requested page is out of bounds
		page = data.TotalPages
		data.CurrentPage = page
	}

	records, err := fetchRecords(db, page, pageSize)
	if err != nil {
		log.Printf("Web: Error fetching records: %v", err)
		data.Error = "Could not retrieve records."
	} else {
		data.Records = records
	}

	// Pagination logic
	if page > 1 {
		data.HasPrevPage = true
		data.PrevPage = page - 1
	}
	if page < data.TotalPages {
		data.HasNextPage = true
		data.NextPage = page + 1
	}

	// Generate page numbers for display (e.g., 1, 2, ..., 5, 6, 7, ..., 10, 11)
	// This is a simple version, can be made more sophisticated
	startPage := int(math.Max(1, float64(page-2)))
	endPage := int(math.Min(float64(data.TotalPages), float64(page+2)))
	if page <= 3 {
		endPage = int(math.Min(float64(data.TotalPages), 5))
	}
	if page >= data.TotalPages-2 {
		startPage = int(math.Max(1, float64(data.TotalPages-4)))
	}

	for i := startPage; i <= endPage; i++ {
		data.Pages = append(data.Pages, i)
	}

	executeTemplate(w, tmpl, data)
}

func executeTemplate(w http.ResponseWriter, tmpl *template.Template, data TemplateData) {
	err := tmpl.Execute(w, data)
	if err != nil {
		log.Printf("Web: Error executing template: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
	}
}

func fetchRecords(db *sql.DB, page, pageSize int) ([]worker.Record, error) {
	offset := (page - 1) * pageSize
	query := `
	SELECT id, timestamp, ip_address, latency_ms, download_mbps, upload_mbps, error
	FROM network_records
	ORDER BY timestamp DESC
	LIMIT ? OFFSET ?`

	rows, err := db.Query(query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var records []worker.Record
	for rows.Next() {
		var r worker.Record
		var latencyMs sql.NullFloat64 // Handle NULL latency
		err := rows.Scan(
			&r.ID,
			&r.Timestamp,
			&r.IPAddress,
			&latencyMs,
			&r.DownloadMbps,
			&r.UploadMbps,
			&r.Error,
		)
		if err != nil {
			return nil, fmt.Errorf("row scan failed: %w", err)
		}
		if latencyMs.Valid {
			r.Latency = time.Duration(latencyMs.Float64) * time.Millisecond
		}
		records = append(records, r)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return records, nil
}

func fetchTotalRecordsCount(db *sql.DB) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM network_records`
	err := db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count records: %w", err)
	}
	return count, nil
}
