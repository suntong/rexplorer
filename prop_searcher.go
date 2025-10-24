package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// --- Generic Data Structures ---

// RepositorySummary contains the normalized, key information from any repo source.
// This is the common data structure that all searchers must map their results to.
type RepositorySummary struct {
	Name            string   `json:"name"`
	FullName        string   `json:"full_name"`
	Description     string   `json:"description"`
	URL             string   `json:"url"`
	Stars           int      `json:"stars"`
	Forks           int      `json:"forks"`
	Language        string   `json:"language"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	IsPrivate       bool     `json:"is_private"`
	IsFork          bool     `json:"is_fork"`
	IsArchived      bool     `json:"is_archived"`
	Topics          []string `json:"topics"`
	License         string   `json:"license"`
	OpenIssuesCount int      `json:"open_issues_count"`
}

// SearchResult contains all collected repositories and metadata from a search.
type SearchResult struct {
	Source     string              `json:"source"`
	Query      string              `json:"query"`
	TotalCount int                 `json:"total_count"` // Total available, not just retrieved
	Items      []RepositorySummary `json:"items"`
}

// --- Template Method Pattern ---

// RepoSearcher defines the "primitive operations" that concrete implementations
// must provide. This is the interface that the template method will call.
type RepoSearcher interface {
	// buildSearchURL creates the provider-specific URL for a given page.
	buildSearchURL(query string, page, perPage int) (string, error)
	// buildSearchRequest creates the *http.Request and adds provider-specific headers.
	buildSearchRequest(ctx context.Context, url string) (*http.Request, error)
	// parseSearchResponse unmarshals the provider-specific response body
	// and maps it to the generic []RepositorySummary.
	// It must also return the total count of items available and
	// a boolean indicating if more pages are available.
	parseSearchResponse(body io.Reader) (summaries []RepositorySummary, totalCount int, hasMore bool, err error)
}

// BaseRepoSearcher contains the "template method" (Search) and common fields.
// It embeds the RepoSearcher interface to call the primitive operations.
// This embedding is the Go equivalent of an abstract base class.
type BaseRepoSearcher struct {
	// implementation holds the concrete implementation (e.g., GitCodeSearcher).
	// This is the "subclass" we will call.
	implementation RepoSearcher
	HTTPClient     *http.Client
	Token          string
	Source         string
	BaseURL        string
	// MaxRetries is the number of times to retry a request on failure
	MaxRetries int
	// RetryDelay is the initial delay between retries
	RetryDelay time.Duration
}

// NewBaseRepoSearcher creates a new base searcher.
// The `impl` parameter is the concrete implementation (e.g., *GitCodeSearcher)
func NewBaseRepoSearcher(impl RepoSearcher, token string, client *http.Client) *BaseRepoSearcher {
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	return &BaseRepoSearcher{
		implementation: impl,
		HTTPClient:     client,
		Token:          token,
		MaxRetries:     3,
		RetryDelay:     1 * time.Second,
	}
}

// Search is the "Template Method".
// It defines the skeleton of the search algorithm (pagination, error handling)
// and calls the primitive operations on its embedded `implementation`.
func (s *BaseRepoSearcher) Search(ctx context.Context, query string, maxPages int) (*SearchResult, error) {
	if query == "" {
		return nil, errors.New("query cannot be empty")
	}
	if maxPages <= 0 {
		return nil, errors.New("maxPages must be greater than 0")
	}

	var allRepos []RepositorySummary
	var totalCount int
	const perPage = 50 // Common page size

	for page := 1; page <= maxPages; page++ {
		// 1. Build the URL (Primitive Operation)
		url, err := s.implementation.buildSearchURL(query, page, perPage)
		if err != nil {
			return nil, fmt.Errorf("failed to build URL for page %d: %w", page, err)
		}

		log.Printf("Fetching page %d: %s", page, url)

		// 2. Fetch the data with retries
		body, err := s.fetchWithRetries(ctx, url)
		if err != nil {
			if page == 1 {
				return nil, fmt.Errorf("failed to fetch first page: %w", err)
			}
			// For subsequent pages, log the error and return what we have
			log.Printf("Warning: failed to fetch page %d: %v. Returning partial results.", page, err)
			break
		}

		if body == nil {
			continue // Should not happen if err is nil, but good to check
		}

		// 3. Parse the response (Primitive Operation)
		repos, tc, hasMore, err := s.implementation.parseSearchResponse(body)
		if err != nil {
			log.Printf("Warning: failed to parse page %d: %v", page, err)
			body.Close() // Close the body even on parse error
			break
		}
		body.Close() // Close the body on success

		if page == 1 {
			totalCount = tc // Set total count from the first page
		}

		allRepos = append(allRepos, repos...)

		if !hasMore || len(repos) == 0 {
			log.Printf("No more results found. Stopping at page %d.", page)
			break // No more items, we've reached the end
		}

		// Respect rate limiting
		if page < maxPages {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return &SearchResult{
		Source:     s.Source,
		Query:      query,
		TotalCount: totalCount,
		Items:      allRepos,
	}, nil
}

// fetchWithRetries handles the HTTP GET request and retries on failure.
func (s *BaseRepoSearcher) fetchWithRetries(ctx context.Context, url string) (io.ReadCloser, error) {
	var lastErr error
	delay := s.RetryDelay

	for i := 0; i < s.MaxRetries; i++ {
		// 1. Build the Request (Primitive Operation)
		req, err := s.implementation.buildSearchRequest(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			log.Printf("Request attempt %d/%d failed: %v. Retrying in %v...", i+1, s.MaxRetries, err, delay)
			time.Sleep(delay)
			delay *= 2 // Exponential backoff
			continue
		}

		if resp.StatusCode == http.StatusOK {
			return resp.Body, nil // Success!
		}

		// Read body for error message
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		lastErr = fmt.Errorf("api request failed with status %d: %s", resp.StatusCode, string(body))

		// Handle specific non-retryable errors
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
			return nil, lastErr // Don't retry auth or not found errors
		}

		// Retry other server/rate limit errors
		log.Printf("Request attempt %d/%d failed with status %d. Retrying in %v...", i+1, s.MaxRetries, resp.StatusCode, delay)
		time.Sleep(delay)
		delay *= 2
	}

	return nil, fmt.Errorf("failed to fetch URL after %d attempts: %w", s.MaxRetries, lastErr)
}
