package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// PrintSummary prints repository summaries in a readable format
func PrintSummary(summaries []RepositorySummary) {
	if len(summaries) == 0 {
		fmt.Println("No repositories found.")
		return
	}

	fmt.Printf("Found %d repositories:\n\n", len(summaries))
	for i, summary := range summaries {
		fmt.Printf("%d. %s (%s)\n", i+1, summary.FullName, summary.Source)
		fmt.Printf("   URL: %s\n", summary.URL)
		fmt.Printf("   Description: %s\n", summary.Description)
		fmt.Printf("   Language: %s | Stars: %d | Forks: %d\n",
			summary.Language, summary.Stars, summary.Forks)
		fmt.Printf("   Created: %s | Updated: %s\n", summary.CreatedAt, summary.UpdatedAt)
		if len(summary.Topics) > 0 {
			fmt.Printf("   Topics: %s\n", strings.Join(summary.Topics, ", "))
		}
		fmt.Println(strings.Repeat("-", 50))
	}
}

// searcherTemplate is the interface our main function will program against.
// This allows us to use any concrete implementation from the other files.
type searcherTemplate interface {
	Search(ctx context.Context, query string, maxPages int) (*SearchResult, error)
}

func main() {
	// --- Command Line Flag Parsing ---
	service := flag.String("service", "github", "The search service to use (github, gitlab, bitbucket, gitcode, gitee)")
	pages := flag.Int("pages", 5, "Maximum number of pages to fetch")
	timeout := flag.Duration("timeout", 2*time.Minute, "Search timeout (e.g., 30s, 1m, 2m30s)")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		log.Fatal("Usage: go run . -service=<github|gitlab|bitbucket|gitcode|gitee> [options] <search_query>")
	}
	query := args[0]

	// --- Service Initialization ---
	var searcher searcherTemplate
	var token string
	var client = &http.Client{Timeout: 30 * time.Second}

	switch strings.ToLower(*service) {
	case "github":
		token = os.Getenv("GITHUB_TOKEN") // Optional, but higher rate limits
		if token == "" {
			log.Println("Warning: GITHUB_TOKEN not set. Using unauthenticated requests (low rate limit).")
		}
		searcher = NewGitHubSearcher(token, client)
	case "gitlab":
		token = os.Getenv("GITLAB_TOKEN")
		if token == "" {
			log.Fatal("Error: GITLAB_TOKEN environment variable not set.")
		}
		searcher = NewGitLabSearcher(token, client)
	case "bitbucket":
		token = os.Getenv("BITBUCKET_TOKEN")
		if token == "" {
			log.Fatal("Error: BITBUCKET_TOKEN environment variable not set. Expected format is 'username:app_password'.")
		}
		searcher = NewBitbucketSearcher(token, client)
	case "gitcode":
		token = os.Getenv("GITCODE_TOKEN")
		if token == "" {
			log.Fatal("Error: GITCODE_TOKEN environment variable not set.")
		}
		searcher = NewGitCodeSearcher(token, client)
	case "gitee":
		token = os.Getenv("GITEE_TOKEN")
		if token == "" {
			log.Fatal("Error: GITEE_TOKEN environment variable not set.")
		}
		searcher = NewGiteeSearcher(token, client)
	default:
		log.Fatalf("Unknown service: %s. Must be one of github, gitlab, bitbucket, gitcode, or gitee.", *service)
	}

	// --- Execution ---
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	log.Printf("Starting search on %s for query %q (max %d pages)...", *service, query, *pages)

	result, err := searcher.Search(ctx, query, *pages)
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}

	// --- Results ---
	fmt.Fprintln(os.Stderr, "\n=== KEY REPOSITORY INFORMATION ===")
	PrintSummary(result.Items)

	fmt.Fprintf(os.Stderr, "\nSearch completed:\n")
	fmt.Fprintf(os.Stderr, "- Service: %s\n", *service)
	fmt.Fprintf(os.Stderr, "- Query: %q\n", query)
	if result.TotalCount == -1 {
		fmt.Fprintf(os.Stderr, "- Total repositories available: Unknown\n")
	} else {
		fmt.Fprintf(os.Stderr, "- Total repositories available: %d\n", result.TotalCount)
	}
	fmt.Fprintf(os.Stderr, "- Repositories retrieved: %d\n", len(result.Items))
}
