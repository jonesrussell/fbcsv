package search

import (
	"context"
	"fmt"
	"strings"
	"time"

	"csv-search/internal/database"
	"csv-search/internal/models"
)

// DatabaseSearcher handles search operations using SQLite
type DatabaseSearcher struct {
	db     *database.Database
	config *models.Config
}

// NewDatabaseSearcher creates a new database searcher
func NewDatabaseSearcher(db *database.Database, config *models.Config) *DatabaseSearcher {
	return &DatabaseSearcher{
		db:     db,
		config: config,
	}
}

// Search performs a full-text search using SQLite FTS5
func (s *DatabaseSearcher) Search(ctx context.Context, req *models.SearchRequest) (*models.SearchResponse, error) {
	start := time.Now()

	// Prepare search query for FTS5
	query := strings.TrimSpace(req.Query)
	if query == "" {
		// Return empty results for empty query
		return &models.SearchResponse{
			Results:    []models.SearchResult{},
			Total:      0,
			Page:       req.Page,
			Limit:      req.Limit,
			TotalPages: 0,
			Query:      query,
			Duration:   time.Since(start),
		}, nil
	}

	// Transform query for FTS5 syntax
	ftsQuery := s.transformQueryForFTS(query)

	// Get columns from database metadata
	stats, err := s.db.GetFileStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get file stats: %w", err)
	}
	if stats == nil {
		return nil, fmt.Errorf("no data available")
	}

	// Extract column names
	columns := make([]string, len(stats.Columns))
	for i, col := range stats.Columns {
		columns[i] = col.Name
	}

	// Perform search
	results, total, err := s.db.Search(ftsQuery, req.Page, req.Limit, columns)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Calculate pagination
	totalPages := (total + req.Limit - 1) / req.Limit
	if totalPages == 0 {
		totalPages = 1
	}

	// Add highlights to results
	for i := range results {
		results[i].Highlights = s.generateHighlights(results[i].Data, query)
	}

	response := &models.SearchResponse{
		Results:    results,
		Total:      total,
		Page:       req.Page,
		Limit:      req.Limit,
		TotalPages: totalPages,
		Query:      query,
		Duration:   time.Since(start),
	}

	return response, nil
}

// GetSuggestions returns search suggestions
func (s *DatabaseSearcher) GetSuggestions(ctx context.Context, query string, limit int) ([]string, error) {
	return s.db.GetSuggestions(query, limit)
}

// transformQueryForFTS transforms a search query for SQLite FTS5 syntax
func (s *DatabaseSearcher) transformQueryForFTS(query string) string {
	// FTS5 supports various query syntaxes:
	// - Simple terms: "hello world"
	// - Phrase search: "hello world"
	// - Prefix search: "hello*"
	// - Boolean operators: "hello AND world", "hello OR world"
	// - Exclude terms: "hello -world"

	query = strings.TrimSpace(query)
	if query == "" {
		return query
	}

	// If query contains quotes, treat as phrase search
	if strings.Contains(query, `"`) {
		return query
	}

	// If query contains boolean operators, use as-is
	if strings.Contains(strings.ToUpper(query), " AND ") ||
		strings.Contains(strings.ToUpper(query), " OR ") ||
		strings.Contains(query, "-") {
		return query
	}

	// For simple queries, add prefix matching to each term
	terms := strings.Fields(query)
	var ftsTerms []string

	for _, term := range terms {
		// Remove special characters that might interfere with FTS
		cleanTerm := s.cleanSearchTerm(term)
		if cleanTerm != "" {
			// Add prefix matching
			ftsTerms = append(ftsTerms, cleanTerm+"*")
		}
	}

	if len(ftsTerms) == 0 {
		return query
	}

	return strings.Join(ftsTerms, " AND ")
}

// cleanSearchTerm removes or escapes special characters in search terms
func (s *DatabaseSearcher) cleanSearchTerm(term string) string {
	// Remove FTS5 special characters that could cause syntax errors
	specialChars := []string{`"`, `'`, `(`, `)`, `[`, `]`, `{`, `}`, `^`, `$`, `\`, `/`, `:`, `;`, `,`, `.`, `!`, `?`}

	cleaned := term
	for _, char := range specialChars {
		cleaned = strings.ReplaceAll(cleaned, char, "")
	}

	return strings.TrimSpace(cleaned)
}

// generateHighlights creates highlight information for search results
func (s *DatabaseSearcher) generateHighlights(data map[string]string, query string) map[string][]string {
	highlights := make(map[string][]string)
	queryLower := strings.ToLower(query)
	queryTerms := strings.Fields(queryLower)

	for column, value := range data {
		valueLower := strings.ToLower(value)
		var columnHighlights []string

		for _, term := range queryTerms {
			term = strings.TrimSpace(term)
			if term == "" {
				continue
			}

			// Find all occurrences of the term
			start := 0
			for {
				pos := strings.Index(valueLower[start:], term)
				if pos == -1 {
					break
				}

				actualPos := start + pos
				// Extract context around the match (e.g., 20 characters before and after)
				contextStart := actualPos - 20
				if contextStart < 0 {
					contextStart = 0
				}
				contextEnd := actualPos + len(term) + 20
				if contextEnd > len(value) {
					contextEnd = len(value)
				}

				context := value[contextStart:contextEnd]
				if !contains(columnHighlights, context) {
					columnHighlights = append(columnHighlights, context)
				}

				start = actualPos + 1
			}
		}

		if len(columnHighlights) > 0 {
			highlights[column] = columnHighlights
		}
	}

	return highlights
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
