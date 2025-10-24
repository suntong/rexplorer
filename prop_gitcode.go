package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// --- GitCode Specific Data Structures ---

// gitCodeRepository represents the raw JSON structure for a GitCode repo
type gitCodeRepository struct {
	ID              int64           `json:"id"`
	Name            string          `json:"name"`
	FullName        string          `json:"full_name"`
	Description     string          `json:"description"`
	Private         bool            `json:"private"`
	Fork            bool            `json:"fork"`
	HTMLURL         string          `json:"html_url"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
	StargazersCount int             `json:"stargazers_count"`
	ForksCount      int             `json:"forks_count"`
	Language        *string         `json:"language"`
	Archived        bool            `json:"archived"`
	OpenIssuesCount int             `json:"open_issues_count"`
	License         *gitCodeLicense `json:"license"`
	Topics          []string        `json:"topics"`
}

type gitCodeLicense struct {
	Name string `json:"name"`
}

// GitCodeSearcher is the concrete implementation for searching GitCode.
type GitCodeSearcher struct {
	*BaseRepoSearcher // Embed the base searcher
}

// NewGitCodeSearcher creates a new searcher for GitCode.
func NewGitCodeSearcher(token string, client *http.Client) *GitCodeSearcher {
	searcher := &GitCodeSearcher{}
	// This is the key: we create the base searcher and pass *itself*
	// as the implementation for the interface to call.
	base := NewBaseRepoSearcher(searcher, token, client)
	base.Source = "GitCode"
	base.BaseURL = "https://api.gitcode.com/api/v5"
	searcher.BaseRepoSearcher = base
	return searcher
}

// buildSearchURL implements the RepoSearcher interface for GitCode.
func (g *GitCodeSearcher) buildSearchURL(query string, page, perPage int) (string, error) {
	u, err := url.Parse(g.BaseURL + "/search/repositories")
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL: %w", err)
	}
	q := u.Query()
	q.Set("q", query)
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("per_page", fmt.Sprintf("%d", perPage))
	if lang := os.Getenv("GITCODE_LANG"); lang != "" {
		q.Set("language", lang)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// buildSearchRequest implements the RepoSearcher interface for GitCode.
func (g *GitCodeSearcher) buildSearchRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "go-repo-searcher/1.0")
	req.Header.Set("Authorization", "Bearer "+g.Token)
	return req, nil
}

// parseSearchResponse implements the RepoSearcher interface for GitCode.
func (g *GitCodeSearcher) parseSearchResponse(body io.Reader) (summaries []RepositorySummary, totalCount int, hasMore bool, err error) {
	// GitCode's response is just an array of repositories.
	var repos []gitCodeRepository
	if err := json.NewDecoder(body).Decode(&repos); err != nil {
		return nil, 0, false, fmt.Errorf("failed to unmarshal GitCode response: %w", err)
	}

	summaries = make([]RepositorySummary, len(repos))
	for i, repo := range repos {
		summaries[i] = g.mapRepoToSummary(repo)
	}

	// GitCode doesn't return total count in the body or headers.
	// We also don't know if there's more. We assume `hasMore` if we got a full page.
	totalCount = -1 // -1 signifies unknown
	hasMore = len(repos) > 0
	return summaries, totalCount, hasMore, nil
}

// mapRepoToSummary converts a GitCode-specific repo to the generic summary.
func (g *GitCodeSearcher) mapRepoToSummary(repo gitCodeRepository) RepositorySummary {
	language := "Unknown"
	if repo.Language != nil && *repo.Language != "" {
		language = *repo.Language
	}

	license := "None"
	if repo.License != nil && repo.License.Name != "" {
		license = repo.License.Name
	}

	return RepositorySummary{
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
