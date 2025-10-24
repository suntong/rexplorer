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

// --- GitLab Specific Data Structures ---

// gitLabRepository represents the raw JSON structure for a GitLab project/repo.
type gitLabRepository struct {
	ID                int64           `json:"id"`
	Name              string          `json:"name"`
	PathWithNamespace string          `json:"path_with_namespace"`
	Description       string          `json:"description"`
	Visibility        string          `json:"visibility"`
	WebURL            string          `json:"web_url"`
	CreatedAt         string          `json:"created_at"`
	LastActivityAt    string          `json:"last_activity_at"`
	StarCount         int             `json:"star_count"`
	ForksCount        int             `json:"forks_count"`
	Archived          bool            `json:"archived"`
	OpenIssuesCount   int             `json:"open_issues_count"`
	Topics            []string        `json:"topics"`
	License           *gitLabLicense  `json:"license"`
	ForkedFromProject *map[string]any `json:"forked_from_project"` // Presence indicates a fork
}

type gitLabLicense struct {
	Name string `json:"name"`
}

// GitLabSearcher is the concrete implementation for searching GitLab.
type GitLabSearcher struct {
	*BaseRepoSearcher
}

// NewGitLabSearcher creates a new searcher for GitLab.
func NewGitLabSearcher(token string, client *http.Client) *GitLabSearcher {
	searcher := &GitLabSearcher{}
	base := NewBaseRepoSearcher(searcher, token, client)
	base.Source = "GitLab"
	base.BaseURL = "https://gitlab.com/api/v4"
	searcher.BaseRepoSearcher = base
	return searcher
}

// buildSearchURL implements the RepoSearcher interface for GitLab.
func (g *GitLabSearcher) buildSearchURL(query string, page, perPage int) (string, error) {
	u, err := url.Parse(g.BaseURL + "/projects")
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL: %w", err)
	}
	q := u.Query()
	q.Set("search", query)
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("per_page", fmt.Sprintf("%d", perPage))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// buildSearchRequest implements the RepoSearcher interface for GitLab.
func (g *GitLabSearcher) buildSearchRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "go-repo-searcher/1.0")
	// GitLab uses PRIVATE-TOKEN header for authentication, but it's not required for public repos.
	// The token is now optional.
	if g.Token != "" {
		req.Header.Set("PRIVATE-TOKEN", g.Token)
	}
	return req, nil
}

// parseSearchResponse implements the RepoSearcher interface for GitLab.
func (g *GitLabSearcher) parseSearchResponse(body io.Reader) (summaries []RepositorySummary, totalCount int, hasMore bool, err error) {
	// GitLab's response for a project search is a direct array of repositories.
	var repos []gitLabRepository
	if err := json.NewDecoder(body).Decode(&repos); err != nil {
		return nil, 0, false, fmt.Errorf("failed to unmarshal GitLab response: %w", err)
	}

	summaries = make([]RepositorySummary, len(repos))
	for i, repo := range repos {
		summaries[i] = g.mapRepoToSummary(repo)
	}

	// GitLab returns pagination info in headers (X-Total, X-Next-Page), which
	// we can't access here. We'll follow the same pattern as GitCode.
	totalCount = -1 // -1 signifies unknown
	hasMore = len(repos) > 0
	return summaries, totalCount, hasMore, nil
}

// mapRepoToSummary converts a GitLab-specific repo to the generic summary.
func (g *GitLabSearcher) mapRepoToSummary(repo gitLabRepository) RepositorySummary {
	license := "None"
	if repo.License != nil && repo.License.Name != "" {
		license = repo.License.Name
	}

	return RepositorySummary{
		Name:            repo.Name,
		FullName:        repo.PathWithNamespace,
		Description:     strings.TrimSpace(repo.Description),
		URL:             repo.WebURL,
		Stars:           repo.StarCount,
		Forks:           repo.ForksCount,
		Language:        "Unknown", // GitLab API for search doesn't provide language easily
		CreatedAt:       repo.CreatedAt,
		UpdatedAt:       repo.LastActivityAt,
		IsPrivate:       repo.Visibility == "private",
		IsFork:          repo.ForkedFromProject != nil,
		IsArchived:      repo.Archived,
		Topics:          repo.Topics,
		License:         license,
		OpenIssuesCount: repo.OpenIssuesCount,
	}
}
