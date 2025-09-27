package search

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"csv-search/internal/models"
)

// Searcher handles search operations
type Searcher struct {
	indexer *Indexer
	config  *models.Config
}

// NewSearcher creates a new searcher instance
func NewSearcher(indexer *Indexer, config *models.Config) *Searcher {
	return &Searcher{
		indexer: indexer,
		config:  config,
	}
}

// SearchResult represents a search result with scoring
type SearchResult struct {
	RowIndex   int
	Score      float64
	Highlights map[string][]string
}

// Search performs a search operation
func (s *Searcher) Search(ctx context.Context, req *models.SearchRequest) (*models.SearchResponse, error) {
	start := time.Now()

	// Validate request
	if err := s.validateRequest(req); err != nil {
		return nil, err
	}

	// Get index
	index := s.indexer.GetIndex()
	if index == nil {
		return nil, fmt.Errorf("index not available")
	}

	// Perform search
	results, err := s.performSearch(ctx, req, index)
	if err != nil {
		return nil, err
	}

	// Apply pagination
	total := len(results)
	startIdx, endIdx := s.calculatePagination(req.Page, req.Limit, total)
	paginatedResults := results[startIdx:endIdx]

	// Convert to response format
	response := &models.SearchResponse{
		Results:    make([]models.SearchResult, len(paginatedResults)),
		Total:      total,
		Page:       req.Page,
		Limit:      req.Limit,
		TotalPages: (total + req.Limit - 1) / req.Limit,
		Query:      req.Query,
		Duration:   time.Since(start),
	}

	// Convert search results to response format
	for i, result := range paginatedResults {
		response.Results[i] = models.SearchResult{
			Row:        result.RowIndex + 1, // Convert to 1-based indexing
			Data:       index.rows[result.RowIndex],
			Score:      result.Score,
			Highlights: result.Highlights,
		}
	}

	return response, nil
}

// validateRequest validates the search request
func (s *Searcher) validateRequest(req *models.SearchRequest) error {
	// Allow empty query for wildcard search
	if req.Query == "" {
		req.Query = "*"
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.Limit < 1 {
		req.Limit = s.config.PageSize
	}
	if req.Limit > s.config.MaxResults {
		req.Limit = s.config.MaxResults
	}
	return nil
}

// performSearch performs the actual search operation
func (s *Searcher) performSearch(ctx context.Context, req *models.SearchRequest, index *Index) ([]SearchResult, error) {
	// Handle wildcard query to return all results
	if req.Query == "*" || req.Query == "" {
		return s.getAllResults(req.Page, req.Limit, index), nil
	}

	// Tokenize query
	queryTokens := s.tokenizeQuery(req.Query)
	if len(queryTokens) == 0 {
		return []SearchResult{}, nil
	}

	// Find matching rows
	rowScores := make(map[int]float64)
	rowHighlights := make(map[int]map[string][]string)

	for _, token := range queryTokens {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Find rows containing this token
		if rowIndices, exists := index.invertedIndex[token]; exists {
			for _, rowIndex := range rowIndices {
				// Calculate score based on token frequency and position
				score := s.calculateTokenScore(token, rowIndex, index, queryTokens)
				rowScores[rowIndex] += score

				// Build highlights
				if rowHighlights[rowIndex] == nil {
					rowHighlights[rowIndex] = make(map[string][]string)
				}
				s.addHighlights(rowHighlights[rowIndex], token, rowIndex, index)
			}
		}
	}

	// Convert to results and sort by score
	results := make([]SearchResult, 0, len(rowScores))
	for rowIndex, score := range rowScores {
		results = append(results, SearchResult{
			RowIndex:   rowIndex,
			Score:      score,
			Highlights: rowHighlights[rowIndex],
		})
	}

	// Sort by score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

// getAllResults returns all results with pagination
func (s *Searcher) getAllResults(page, limit int, index *Index) []SearchResult {
	start := (page - 1) * limit
	end := start + limit

	if start >= len(index.rows) {
		return []SearchResult{}
	}

	if end > len(index.rows) {
		end = len(index.rows)
	}

	results := make([]SearchResult, 0, end-start)
	for i := start; i < end; i++ {
		results = append(results, SearchResult{
			RowIndex:   i,
			Score:      1.0, // Default score for all results
			Highlights: make(map[string][]string),
		})
	}

	return results
}

// tokenizeQuery tokenizes the search query
func (s *Searcher) tokenizeQuery(query string) []string {
	// Convert to lowercase and split
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return []string{}
	}

	// Split on whitespace
	words := strings.Fields(query)
	tokens := make([]string, 0, len(words))
	for _, word := range words {
		if len(word) > 0 {
			tokens = append(tokens, word)
		}
	}
	return tokens
}

// calculateTokenScore calculates the score for a token in a row
func (s *Searcher) calculateTokenScore(token string, rowIndex int, index *Index, queryTokens []string) float64 {
	row := index.rows[rowIndex]
	score := 0.0

	// Base score for token match
	baseScore := 1.0

	// Boost score for exact matches
	for _, value := range row {
		lowerValue := strings.ToLower(value)
		if strings.Contains(lowerValue, token) {
			// Exact match gets higher score
			if lowerValue == token {
				score += baseScore * 2.0
			} else if strings.HasPrefix(lowerValue, token) {
				score += baseScore * 1.5
			} else {
				score += baseScore
			}
		}
	}

	// Boost score for multiple query tokens matching
	if len(queryTokens) > 1 {
		matches := 0
		for _, queryToken := range queryTokens {
			for _, value := range row {
				if strings.Contains(strings.ToLower(value), queryToken) {
					matches++
					break
				}
			}
		}
		if matches > 1 {
			score *= float64(matches) / float64(len(queryTokens))
		}
	}

	return score
}

// addHighlights adds highlighting information for a token
func (s *Searcher) addHighlights(highlights map[string][]string, token string, rowIndex int, index *Index) {
	row := index.rows[rowIndex]

	for colName, value := range row {
		lowerValue := strings.ToLower(value)
		if strings.Contains(lowerValue, token) {
			// Find all occurrences of the token
			start := 0
			for {
				pos := strings.Index(lowerValue[start:], token)
				if pos == -1 {
					break
				}
				actualPos := start + pos
				highlight := s.createHighlight(value, actualPos, len(token))
				if highlight != "" {
					highlights[colName] = append(highlights[colName], highlight)
				}
				start = actualPos + len(token)
			}
		}
	}
}

// createHighlight creates a highlighted snippet around a token
func (s *Searcher) createHighlight(text string, start, length int) string {
	// Create a snippet with context around the match
	contextSize := 20
	snippetStart := start - contextSize
	if snippetStart < 0 {
		snippetStart = 0
	}
	snippetEnd := start + length + contextSize
	if snippetEnd > len(text) {
		snippetEnd = len(text)
	}

	snippet := text[snippetStart:snippetEnd]

	// Add ellipsis if needed
	if snippetStart > 0 {
		snippet = "..." + snippet
	}
	if snippetEnd < len(text) {
		snippet = snippet + "..."
	}

	return snippet
}

// calculatePagination calculates pagination boundaries
func (s *Searcher) calculatePagination(page, limit, total int) (start, end int) {
	start = (page - 1) * limit
	end = start + limit
	if end > total {
		end = total
	}
	if start > total {
		start = total
	}
	return start, end
}

// GetSuggestions returns search suggestions based on partial query
func (s *Searcher) GetSuggestions(ctx context.Context, query string, limit int) ([]string, error) {
	if query == "" {
		return []string{}, nil
	}

	index := s.indexer.GetIndex()
	if index == nil {
		return []string{}, nil
	}

	query = strings.ToLower(strings.TrimSpace(query))
	suggestions := make(map[string]bool)

	// Find tokens that start with the query
	for token := range index.invertedIndex {
		if strings.HasPrefix(token, query) {
			suggestions[token] = true
		}
	}

	// Convert to slice and sort
	result := make([]string, 0, len(suggestions))
	for suggestion := range suggestions {
		result = append(result, suggestion)
	}
	sort.Strings(result)

	// Limit results
	if len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}
