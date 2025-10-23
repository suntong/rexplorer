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

// --- GitHub Specific Data Structures ---

// gitHubSearchResponse is the top-level struct for GitHub's search results.
type gitHubSearchResponse struct {
	TotalCount        int                `json:"total_count"`
	IncompleteResults bool               `json:"incomplete_results"`
	Items             []gitHubRepository `json:"items"`
}

// gitHubRepository represents the raw JSON structure for a GitHub repo
type gitHubRepository struct {
	ID              int64          `json:"id"`
	Name            string         `json:"name"`
	FullName        string         `json:"full_name"`
	Description     string         `json:"description"`
	Private         bool           `json:"private"`
	Fork            bool           `json:"fork"`
	HTMLURL         string         `json:"html_url"`
	CreatedAt       string         `json:"created_at"`
	UpdatedAt       string         `json:"updated_at"`
	StargazersCount int            `json:"stargazers_count"`
	ForksCount      int            `json:"forks_count"`
	Language        *string        `json:"language"`
	Archived        bool           `json:"archived"`
	OpenIssuesCount int            `json:"open_issues_count"`
	License         *gitHubLicense `json:"license"`
	Topics          []string       `json:"topics"`
}

type gitHubLicense struct {
	Name string `json:"name"`
}

// GitHubSearcher is the concrete implementation for searching GitHub.
type GitHubSearcher struct {
	*BaseRepoSearcher
	BaseURL string
}

// NewGitHubSearcher creates a new searcher for GitHub.
func NewGitHubSearcher(token string, client *http.Client) *GitHubSearcher {
	searcher := &GitHubSearcher{
		BaseURL: "https://api.github.com",
	}
	base := NewBaseRepoSearcher(searcher, token, client)
	searcher.BaseRepoSearcher = base
	return searcher
}

// buildSearchURL implements the RepoSearcher interface for GitHub.
func (g *GitHubSearcher) buildSearchURL(query string, page, perPage int) (string, error) {
	u, err := url.Parse(g.BaseURL + "/search/repositories")
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL: %w", err)
	}
	q := u.Query()
	q.Set("q", query)
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("per_page", fmt.Sprintf("%d", perPage))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// buildSearchRequest implements the RepoSearcher interface for GitHub.
func (g *GitHubSearcher) buildSearchRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "go-repo-searcher/1.0")
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	return req, nil
}

// parseSearchResponse implements the RepoSearcher interface for GitHub.
func (g *GitHubSearcher) parseSearchResponse(body io.Reader) (summaries []RepositorySummary, totalCount int, hasMore bool, err error) {
	var resp gitHubSearchResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, 0, false, fmt.Errorf("failed to unmarshal GitHub response: %w", err)
	}

	summaries = make([]RepositorySummary, len(resp.Items))
	for i, repo := range resp.Items {
		summaries[i] = g.mapRepoToSummary(repo)
	}

	// GitHub provides the total count
	totalCount = resp.TotalCount
	// We can determine `hasMore` by checking if we have more items to fetch
	hasMore = (len(summaries) > 0) && !resp.IncompleteResults
	// A more robust check: hasMore = (page * perPage) < totalCount
	// But since we don't have page/perPage here, we'll assume `hasMore` if items were returned.
	// The main `Search` loop will stop if len(repos) == 0 anyway.
	hasMore = len(summaries) > 0

	return summaries, totalCount, hasMore, nil
}

// mapRepoToSummary converts a GitHub-specific repo to the generic summary.
func (g *GitHubSearcher) mapRepoToSummary(repo gitHubRepository) RepositorySummary {
	language := "Unknown"
	if repo.Language != nil && *repo.Language != "" {
		language = *repo.Language
	}

	license := "None"
	if repo.License != nil && repo.License.Name != "" {
		license = repo.License.Name
	}

	return RepositorySummary{
		Source:          "GitHub",
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
