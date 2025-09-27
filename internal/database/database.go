package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"csv-search/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

// Database represents the SQLite database connection and operations
type Database struct {
	db     *sql.DB
	config *models.Config
}

// NewDatabase creates a new database instance
func NewDatabase(config *models.Config) (*Database, error) {
	dbPath := filepath.Join(config.IndexPath, "csv_search.db")

	// Ensure directory exists
	if err := ensureDir(config.IndexPath); err != nil {
		return nil, fmt.Errorf("failed to create index directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=10000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	database := &Database{
		db:     db,
		config: config,
	}

	// Initialize database schema
	if err := database.initializeSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return database, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// initializeSchema creates the necessary tables and indexes
func (d *Database) initializeSchema() error {
	// Check if FTS5 is available
	var fts5Available bool
	err := d.db.QueryRow("SELECT 1 FROM pragma_compile_options WHERE compile_options LIKE '%FTS5%'").Scan(&fts5Available)
	if err != nil {
		// FTS5 not available, we'll use a simpler approach
		log.Println("FTS5 not available, using basic search")
	}

	// Create metadata table
	_, err = d.db.Exec(`
		CREATE TABLE IF NOT EXISTS csv_metadata (
			id INTEGER PRIMARY KEY,
			file_path TEXT UNIQUE NOT NULL,
			file_size INTEGER NOT NULL,
			row_count INTEGER NOT NULL,
			column_count INTEGER NOT NULL,
			indexed_at DATETIME NOT NULL,
			columns TEXT NOT NULL -- JSON array of column names
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create metadata table: %w", err)
	}

	// Create FTS5 virtual table for full-text search if available
	if fts5Available {
		_, err = d.db.Exec(`
			CREATE VIRTUAL TABLE IF NOT EXISTS csv_data_fts USING fts5(
				row_id UNINDEXED,
				content,
				content='csv_data',
				content_rowid='row_id'
			)
		`)
		if err != nil {
			log.Printf("Warning: Failed to create FTS5 table: %v", err)
			fts5Available = false
		}
	}

	// Create main data table
	_, err = d.db.Exec(`
		CREATE TABLE IF NOT EXISTS csv_data (
			row_id INTEGER PRIMARY KEY AUTOINCREMENT,
			raw_data TEXT NOT NULL -- JSON object of row data
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create data table: %w", err)
	}

	return nil
}

// CreateDynamicTables creates tables based on CSV columns
func (d *Database) CreateDynamicTables(columns []string) error {
	// Create the main data table with dynamic columns
	var columnDefs []string
	for _, col := range columns {
		// Sanitize column names for SQL
		sanitized := sanitizeColumnName(col)
		columnDefs = append(columnDefs, fmt.Sprintf("`%s` TEXT", sanitized))
	}

	// Drop existing table if it exists
	_, err := d.db.Exec("DROP TABLE IF EXISTS csv_data")
	if err != nil {
		return fmt.Errorf("failed to drop existing table: %w", err)
	}

	// Create new table with CSV columns
	createSQL := fmt.Sprintf(`
		CREATE TABLE csv_data (
			row_id INTEGER PRIMARY KEY AUTOINCREMENT,
			%s
		)
	`, strings.Join(columnDefs, ", "))

	_, err = d.db.Exec(createSQL)
	if err != nil {
		return fmt.Errorf("failed to create data table: %w", err)
	}

	// Recreate FTS table
	_, err = d.db.Exec("DROP TABLE IF EXISTS csv_data_fts")
	if err != nil {
		return fmt.Errorf("failed to drop FTS table: %w", err)
	}

	// Create FTS table with dynamic columns
	var ftsColumns []string
	ftsColumns = append(ftsColumns, "row_id UNINDEXED")
	for _, col := range columns {
		sanitized := sanitizeColumnName(col)
		ftsColumns = append(ftsColumns, fmt.Sprintf("`%s`", sanitized))
	}

	ftsSQL := fmt.Sprintf(`
		CREATE VIRTUAL TABLE csv_data_fts USING fts5(
			%s,
			content='csv_data',
			content_rowid='row_id'
		)
	`, strings.Join(ftsColumns, ", "))

	_, err = d.db.Exec(ftsSQL)
	if err != nil {
		return fmt.Errorf("failed to create FTS table: %w", err)
	}

	return nil
}

// InsertRow inserts a single row into the database
func (d *Database) InsertRow(rowData map[string]string, columns []string) error {
	// Prepare insert statement
	var placeholders []string
	var values []interface{}
	var ftsValues []interface{}

	for _, col := range columns {
		placeholders = append(placeholders, "?")
		value := rowData[col]
		values = append(values, value)
		ftsValues = append(ftsValues, value)
	}

	// Insert into main table
	insertSQL := fmt.Sprintf(`
		INSERT INTO csv_data (%s) VALUES (%s)
	`, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	result, err := d.db.Exec(insertSQL, values...)
	if err != nil {
		return fmt.Errorf("failed to insert row: %w", err)
	}

	// Get the row ID
	rowID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get row ID: %w", err)
	}

	// Insert into FTS table
	ftsPlaceholders := make([]string, len(columns)+1)
	for i := range ftsPlaceholders {
		ftsPlaceholders[i] = "?"
	}

	ftsSQL := fmt.Sprintf(`
		INSERT INTO csv_data_fts (row_id, %s) VALUES (%s)
	`, strings.Join(columns, ", "), strings.Join(ftsPlaceholders, ", "))

	ftsValues = append([]interface{}{rowID}, ftsValues...)
	_, err = d.db.Exec(ftsSQL, ftsValues...)
	if err != nil {
		return fmt.Errorf("failed to insert into FTS: %w", err)
	}

	return nil
}

// Search performs a full-text search
func (d *Database) Search(query string, page, limit int, columns []string) ([]models.SearchResult, int, error) {
	offset := (page - 1) * limit

	// Build search query for FTS5
	// FTS5 uses a different syntax than regular SQL
	ftsQuery := "csv_data_fts MATCH ? ORDER BY rank"

	// Get total count
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM csv_data_fts WHERE %s
	`, ftsQuery)

	var total int
	err := d.db.QueryRow(countQuery, query).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get total count: %w", err)
	}

	// Get search results with pagination
	searchSQL := fmt.Sprintf(`
		SELECT csv_data.row_id, csv_data_fts.rank, %s
		FROM csv_data_fts
		JOIN csv_data ON csv_data_fts.row_id = csv_data.row_id
		WHERE %s
		ORDER BY csv_data_fts.rank
		LIMIT ? OFFSET ?
	`, strings.Join(columns, ", "), ftsQuery)

	rows, err := d.db.Query(searchSQL, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute search: %w", err)
	}
	defer rows.Close()

	var results []models.SearchResult
	for rows.Next() {
		var rowID int
		var rank float64

		// Create a slice to scan values into
		scanValues := make([]interface{}, len(columns)+2)
		scanValues[0] = &rowID
		scanValues[1] = &rank

		// Create string pointers for column values
		columnValues := make([]string, len(columns))
		for i := range columnValues {
			scanValues[i+2] = &columnValues[i]
		}

		err := rows.Scan(scanValues...)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan row: %w", err)
		}

		// Build result data map
		data := make(map[string]string)
		for i, col := range columns {
			data[col] = columnValues[i]
		}

		result := models.SearchResult{
			Row:   rowID,
			Data:  data,
			Score: rank,
		}

		results = append(results, result)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, total, nil
}

// GetFileStats returns file statistics
func (d *Database) GetFileStats() (*models.FileStats, error) {
	var stats models.FileStats

	query := `
		SELECT file_size, row_count, column_count, indexed_at, columns
		FROM csv_metadata
		ORDER BY indexed_at DESC
		LIMIT 1
	`

	var fileSize int64
	var rowCount, columnCount int
	var indexedAtStr, columnsJSON string

	err := d.db.QueryRow(query).Scan(&fileSize, &rowCount, &columnCount, &indexedAtStr, &columnsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No data indexed yet
		}
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	// Parse indexed_at timestamp
	indexedAt, err := parseTime(indexedAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	// Parse columns JSON
	columns, err := parseColumnsJSON(columnsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse columns: %w", err)
	}

	stats = models.FileStats{
		RowCount:    rowCount,
		ColumnCount: columnCount,
		FileSize:    fileSize,
		IndexedAt:   indexedAt,
		Columns:     columns,
	}

	return &stats, nil
}

// SaveMetadata saves file metadata
func (d *Database) SaveMetadata(filePath string, fileSize int64, rowCount, columnCount int, columns []models.ColumnInfo) error {
	columnsJSON, err := marshalColumnsJSON(columns)
	if err != nil {
		return fmt.Errorf("failed to marshal columns: %w", err)
	}

	_, err = d.db.Exec(`
		INSERT OR REPLACE INTO csv_metadata 
		(file_path, file_size, row_count, column_count, indexed_at, columns)
		VALUES (?, ?, ?, ?, datetime('now'), ?)
	`, filePath, fileSize, rowCount, columnCount, columnsJSON)

	if err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}

// GetSuggestions returns search suggestions
func (d *Database) GetSuggestions(query string, limit int) ([]string, error) {
	if len(query) < 2 {
		return []string{}, nil
	}

	// Use FTS5 snippet function to get suggestions
	suggestionSQL := `
		SELECT DISTINCT snippet(csv_data_fts, -1, '<mark>', '</mark>', '...', 64) as snippet
		FROM csv_data_fts
		WHERE csv_data_fts MATCH ?
		LIMIT ?
	`

	rows, err := d.db.Query(suggestionSQL, query+"*", limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get suggestions: %w", err)
	}
	defer rows.Close()

	var suggestions []string
	for rows.Next() {
		var snippet string
		if err := rows.Scan(&snippet); err != nil {
			return nil, fmt.Errorf("failed to scan suggestion: %w", err)
		}

		// Clean up the snippet
		cleanSuggestion := cleanSnippet(snippet)
		if len(cleanSuggestion) > 0 {
			suggestions = append(suggestions, cleanSuggestion)
		}
	}

	return suggestions, nil
}

// Helper functions

func ensureDir(_ string) error {
	return nil // For now, assume directory exists or will be created by the system
}

func sanitizeColumnName(name string) string {
	// Replace problematic characters with underscores
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")

	// Ensure it starts with a letter or underscore
	if len(name) > 0 && !isValidSQLIdentifierStart(name[0]) {
		name = "_" + name
	}

	return name
}

func isValidSQLIdentifierStart(char byte) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || char == '_'
}

func parseTime(timeStr string) (time.Time, error) {
	// Parse the timestamp
	return time.Parse("2006-01-02 15:04:05", timeStr)
}

func parseColumnsJSON(jsonStr string) ([]models.ColumnInfo, error) {
	var columns []models.ColumnInfo
	err := json.Unmarshal([]byte(jsonStr), &columns)
	return columns, err
}

func marshalColumnsJSON(columns []models.ColumnInfo) (string, error) {
	data, err := json.Marshal(columns)
	return string(data), err
}

func cleanSnippet(snippet string) string {
	// Remove HTML tags and clean up
	snippet = strings.ReplaceAll(snippet, "<mark>", "")
	snippet = strings.ReplaceAll(snippet, "</mark>", "")
	snippet = strings.ReplaceAll(snippet, "...", "")
	snippet = strings.TrimSpace(snippet)
	return snippet
}
