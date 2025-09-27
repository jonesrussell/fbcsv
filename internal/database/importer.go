package database

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"csv-search/internal/models"
)

// Importer handles CSV import operations
type Importer struct {
	db     *Database
	config *models.Config
}

// NewImporter creates a new CSV importer
func NewImporter(db *Database, config *models.Config) *Importer {
	return &Importer{
		db:     db,
		config: config,
	}
}

// ImportCSV imports a CSV file into the database
func (i *Importer) ImportCSV(ctx context.Context, progressCallback func(*models.IndexProgress)) error {
	// Get file info
	fileInfo, err := os.Stat(i.config.CSVFilePath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Open CSV file
	file, err := os.Open(i.config.CSVFilePath)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	// Count total rows for progress tracking
	totalRows, err := i.countRows(file)
	if err != nil {
		return fmt.Errorf("failed to count rows: %w", err)
	}

	// Reset file position
	file.Seek(0, 0)

	// Initialize progress
	progress := &models.IndexProgress{
		ProcessedRows: 0,
		TotalRows:     totalRows,
		Progress:      0.0,
		IsComplete:    false,
	}

	if progressCallback != nil {
		progressCallback(progress)
	}

	// Create CSV reader
	reader := csv.NewReader(bufio.NewReader(file))
	reader.FieldsPerRecord = -1 // Allow variable number of fields

	// Read header
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	// Create column info
	columns := make([]models.ColumnInfo, len(header))
	for idx, col := range header {
		columns[idx] = models.ColumnInfo{
			Name:  col,
			Index: idx,
		}
	}

	// Create dynamic tables based on CSV columns
	err = i.db.CreateDynamicTables(header)
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	// Start transaction for better performance
	tx, err := i.db.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Process rows in batches
	batchSize := 1000
	batch := make([]map[string]string, 0, batchSize)
	processedRows := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read record at row %d: %w", processedRows+2, err)
		}

		// Create row data map
		rowData := make(map[string]string)
		for colIdx, value := range record {
			if colIdx < len(header) {
				rowData[header[colIdx]] = value
			}
		}

		batch = append(batch, rowData)
		processedRows++

		// Process batch when it's full or at end of file
		if len(batch) >= batchSize {
			err = i.insertBatch(tx, batch, header)
			if err != nil {
				return fmt.Errorf("failed to insert batch: %w", err)
			}
			batch = batch[:0] // Reset batch

			// Update progress
			progress.ProcessedRows = processedRows
			progress.Progress = float64(processedRows) / float64(totalRows) * 100
			if progressCallback != nil {
				progressCallback(progress)
			}
		}
	}

	// Process remaining rows
	if len(batch) > 0 {
		err = i.insertBatch(tx, batch, header)
		if err != nil {
			return fmt.Errorf("failed to insert final batch: %w", err)
		}
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Save metadata
	err = i.db.SaveMetadata(i.config.CSVFilePath, fileInfo.Size(), processedRows, len(header), columns)
	if err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Mark as complete
	progress.ProcessedRows = processedRows
	progress.Progress = 100.0
	progress.IsComplete = true
	if progressCallback != nil {
		progressCallback(progress)
	}

	return nil
}

// insertBatch inserts a batch of rows using prepared statements
func (i *Importer) insertBatch(tx *sql.Tx, batch []map[string]string, columns []string) error {
	if len(batch) == 0 {
		return nil
	}

	// Prepare insert statement for main table
	var placeholders []string
	for range columns {
		placeholders = append(placeholders, "?")
	}

	insertSQL := fmt.Sprintf(`
		INSERT INTO csv_data (%s) VALUES (%s)
	`, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Prepare FTS insert statement
	ftsPlaceholders := make([]string, len(columns)+1)
	for i := range ftsPlaceholders {
		ftsPlaceholders[i] = "?"
	}

	ftsSQL := fmt.Sprintf(`
		INSERT INTO csv_data_fts (row_id, %s) VALUES (%s)
	`, strings.Join(columns, ", "), strings.Join(ftsPlaceholders, ", "))

	ftsStmt, err := tx.Prepare(ftsSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare FTS statement: %w", err)
	}
	defer ftsStmt.Close()

	// Insert each row
	for _, rowData := range batch {
		// Insert into main table
		var values []interface{}
		for _, col := range columns {
			values = append(values, rowData[col])
		}

		result, err := stmt.Exec(values...)
		if err != nil {
			return fmt.Errorf("failed to insert row: %w", err)
		}

		// Get row ID
		rowID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get row ID: %w", err)
		}

		// Insert into FTS table
		var ftsValues []interface{}
		ftsValues = append(ftsValues, rowID)
		for _, col := range columns {
			ftsValues = append(ftsValues, rowData[col])
		}

		_, err = ftsStmt.Exec(ftsValues...)
		if err != nil {
			return fmt.Errorf("failed to insert into FTS: %w", err)
		}
	}

	return nil
}

// countRows counts the total number of rows in the CSV file
func (i *Importer) countRows(file *os.File) (int, error) {
	file.Seek(0, 0)
	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count - 1, scanner.Err() // Subtract 1 for header
}

// GetProgress returns the current import progress
func (i *Importer) GetProgress() *models.IndexProgress {
	// This would typically be stored in the database or a shared state
	// For now, return a default progress
	return &models.IndexProgress{
		ProcessedRows: 0,
		TotalRows:     0,
		Progress:      0.0,
		IsComplete:    true,
	}
}
