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

// --- Gitee Specific Data Structures ---

// giteeRepository represents the raw JSON structure for a Gitee repo
// Gitee's search returns an array, similar to GitCode.
type giteeRepository struct {
	ID              int64    `json:"id"`
	Name            string   `json:"name"`
	FullName        string   `json:"full_name"`
	Description     string   `json:"description"`
	Private         bool     `json:"private"`
	Fork            bool     `json:"fork"`
	HTMLURL         string   `json:"html_url"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	StargazersCount int      `json:"stargazers_count"`
	ForksCount      int      `json:"forks_count"`
	Language        *string  `json:"language"`
	Archived        bool     `json:"archived"`
	OpenIssuesCount int      `json:"open_issues_count"`
	License         *string  `json:"license"` // Gitee license is just a string
	Topics          []string `json:"topics"`
}

// GiteeSearcher is the concrete implementation for searching Gitee.
type GiteeSearcher struct {
	*BaseRepoSearcher
	BaseURL string
}

// NewGiteeSearcher creates a new searcher for Gitee.
// Note: Gitee recommends using an access_token for auth.
func NewGiteeSearcher(token string, client *http.Client) *GiteeSearcher {
	searcher := &GiteeSearcher{
		BaseURL: "https://gitee.com/api/v5",
	}
	base := NewBaseRepoSearcher(searcher, token, client)
	searcher.BaseRepoSearcher = base
	return searcher
}

// buildSearchURL implements the RepoSearcher interface for Gitee.
func (g *GiteeSearcher) buildSearchURL(query string, page, perPage int) (string, error) {
	u, err := url.Parse(g.BaseURL + "/search/repositories")
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL: %w", err)
	}
	q := u.Query()
	q.Set("q", query)
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("per_page", fmt.Sprintf("%d", perPage))
	if g.Token != "" {
		q.Set("access_token", g.Token)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// buildSearchRequest implements the RepoSearcher interface for Gitee.
func (g *GiteeSearcher) buildSearchRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "go-repo-searcher/1.0")
	// Token is already in the query param for Gitee, but adding as a header
	// is also an option if preferred (and supported).
	// Per docs, query param `access_token` is standard.
	return req, nil
}

// parseSearchResponse implements the RepoSearcher interface for Gitee.
func (g *GiteeSearcher) parseSearchResponse(body io.Reader) (summaries []RepositorySummary, totalCount int, hasMore bool, err error) {
	var repos []giteeRepository
	if err := json.NewDecoder(body).Decode(&repos); err != nil {
		return nil, 0, false, fmt.Errorf("failed to unmarshal Gitee response: %w", err)
	}

	summaries = make([]RepositorySummary, len(repos))
	for i, repo := range repos {
		summaries[i] = g.mapRepoToSummary(repo)
	}

	// Gitee also doesn't return total count in the response body.
	// We also don't know if there's more. We assume `hasMore` if we got a full page.
	// Note: Gitee *does* provide a `Total-Count` header. A more robust
	// implementation would read this from the `http.Response` object,
	// but our `parseSearchResponse` only gets an `io.Reader`.
	// This is a tradeoff for this simple template method.
	totalCount = -1 // -1 signifies unknown
	hasMore = len(repos) > 0
	return summaries, totalCount, hasMore, nil
}

// mapRepoToSummary converts a Gitee-specific repo to the generic summary.
func (g *GiteeSearcher) mapRepoToSummary(repo giteeRepository) RepositorySummary {
	language := "Unknown"
	if repo.Language != nil && *repo.Language != "" {
		language = *repo.Language
	}

	license := "None"
	if repo.License != nil && *repo.License != "" {
		license = *repo.License
	}

	return RepositorySummary{
		Source:          "Gitee",
		Name:            repo.Name,
		FullName:        repo.FullName,
		Description:     strings.TrimSpace(repo.Description),
		URL:             repo.HTMLURL,
		Stars:           repo.StargazersCount,
		Forks:           repo.ForksCount,
		Language:        language,
		CreatedAt:       repo.CreatedAt,
		UpdatedAt:       repo.UpdatedAt,
		IsPrivate:       repo.Private,
		IsFork:          repo.Fork,
		IsArchived:      repo.Archived,
		Topics:          repo.Topics,
		License:         license,
		OpenIssuesCount: repo.OpenIssuesCount,
	}
}
