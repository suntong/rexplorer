package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// --- Bitbucket Specific Data Structures ---

// bitbucketSearchResponse is the top-level struct for Bitbucket's repository list.
type bitbucketSearchResponse struct {
	Size   int                   `json:"size"` // Total count of repositories
	Page   int                   `json:"page"`
	Values []bitbucketRepository `json:"values"`
	Next   string                `json:"next"`
}

// bitbucketRepository represents the raw JSON structure for a Bitbucket repo.
type bitbucketRepository struct {
	Name        string          `json:"name"`
	FullName    string          `json:"full_name"`
	Description string          `json:"description"`
	Language    string          `json:"language"`
	CreatedOn   string          `json:"created_on"`
	UpdatedOn   string          `json:"updated_on"`
	IsPrivate   bool            `json:"is_private"`
	Parent      *map[string]any `json:"parent"` // If not nil, it's a fork
	Links       bitbucketLinks  `json:"links"`
}

type bitbucketLinks struct {
	HTML bitbucketLink `json:"html"`
}

type bitbucketLink struct {
	Href string `json:"href"`
}

// BitbucketSearcher is the concrete implementation for searching Bitbucket.
type BitbucketSearcher struct {
	*BaseRepoSearcher
}

// NewBitbucketSearcher creates a new searcher for Bitbucket.
// The token should be in the format "username:app_password".
func NewBitbucketSearcher(token string, client *http.Client) *BitbucketSearcher {
	searcher := &BitbucketSearcher{}
	base := NewBaseRepoSearcher(searcher, token, client)
	base.Source = "Bitbucket"
	base.BaseURL = "https://api.bitbucket.org/2.0"
	searcher.BaseRepoSearcher = base
	return searcher
}

// buildSearchURL implements the RepoSearcher interface for Bitbucket.
func (b *BitbucketSearcher) buildSearchURL(query string, page, perPage int) (string, error) {
	u, err := url.Parse(b.BaseURL + "/repositories")
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL: %w", err)
	}
	q := u.Query()
	// Bitbucket's 'q' param allows for more complex queries. We'll use a simple name search.
	// Example: name~"query"
	q.Set("q", fmt.Sprintf(`name~"%s"`, query))
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("pagelen", fmt.Sprintf("%d", perPage))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// buildSearchRequest implements the RepoSearcher interface for Bitbucket.
func (b *BitbucketSearcher) buildSearchRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "go-repo-searcher/1.0")

	// Bitbucket Cloud API uses Basic Auth with username and an app password.
	// We expect the token to be in the "username:app_password" format.
	if b.Token != "" {
		parts := strings.SplitN(b.Token, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid Bitbucket token format; expected 'username:app_password'")
		}
		req.SetBasicAuth(parts[0], parts[1])
	}
	return req, nil
}

// parseSearchResponse implements the RepoSearcher interface for Bitbucket.
func (b *BitbucketSearcher) parseSearchResponse(body io.Reader) (summaries []RepositorySummary, totalCount int, hasMore bool, err error) {
	var resp bitbucketSearchResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, 0, false, fmt.Errorf("failed to unmarshal Bitbucket response: %w", err)
	}

	summaries = make([]RepositorySummary, len(resp.Values))
	for i, repo := range resp.Values {
		summaries[i] = b.mapRepoToSummary(repo)
	}

	totalCount = resp.Size
	hasMore = resp.Next != ""
	return summaries, totalCount, hasMore, nil
}

// mapRepoToSummary converts a Bitbucket-specific repo to the generic summary.
func (b *BitbucketSearcher) mapRepoToSummary(repo bitbucketRepository) RepositorySummary {
	language := "Unknown"
	if repo.Language != "" {
		language = repo.Language
	}

	// The /repositories endpoint doesn't provide stars, forks, issues, etc.
	return RepositorySummary{
		Name:            repo.Name,
		FullName:        repo.FullName,
		Description:     strings.TrimSpace(repo.Description),
		URL:             repo.Links.HTML.Href,
		Stars:           -1, // Not available in this endpoint
		Forks:           -1, // Not available in this endpoint
		Language:        language,
		CreatedAt:       repo.CreatedOn,
		UpdatedAt:       repo.UpdatedOn,
		IsPrivate:       repo.IsPrivate,
		IsFork:          repo.Parent != nil,
		IsArchived:      false,      // Not available in this endpoint
		Topics:          []string{}, // Not available
		License:         "Unknown",  // Not available
		OpenIssuesCount: -1,         // Not available
	}
}
