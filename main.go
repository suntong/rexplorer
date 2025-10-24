package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// PrintSummary prints repository summaries in a readable format
func PrintSummary(summaries []RepositorySummary, source string) {
	if len(summaries) == 0 {
		fmt.Println("No repositories found.")
		return
	}

	fmt.Printf("Found %d repositories from %s:\n\n", len(summaries), source)
	for i, summary := range summaries {
		fmt.Printf("%d. %s\n", i+1, summary.FullName)
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
			log.Println("Warning: GITLAB_TOKEN not set. Using unauthenticated requests.")
		}
		searcher = NewGitLabSearcher(token, client)
	case "bitbucket":
		token = os.Getenv("BITBUCKET_TOKEN")
		if token == "" {
			log.Fatal("Error: BITBUCKET_TOKEN environment variable not set. Expected format is 'username:app_password'.")
		}
		// Useless!! The authenticated call will only search repos where you have an explicit role (member, contributor, admin, or owner)!
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
	// --- Results ---
	fmt.Fprintln(os.Stderr, "\n=== KEY REPOSITORY INFORMATION ===")
	PrintSummary(result.Items, result.Source)

	// Write JSON output
	if err := writeJSONOutput(result); err != nil {
		log.Printf("Warning: failed to write JSON output: %v", err)
	}

	fmt.Fprintf(os.Stderr, "\nSearch completed:\n")
	fmt.Fprintf(os.Stderr, "- Service: %s\n", result.Source)
	fmt.Fprintf(os.Stderr, "- Query: %q\n", query)
	if result.TotalCount == -1 {
		fmt.Fprintf(os.Stderr, "- Total repositories available: Unknown\n")
	} else {
		fmt.Fprintf(os.Stderr, "- Total repositories available: %d\n", result.TotalCount)
	}
	fmt.Fprintf(os.Stderr, "- Repositories retrieved: %d\n", len(result.Items))
}

// writeJSONOutput marshals the search result items to a JSON file.
func writeJSONOutput(result *SearchResult) error {
	if len(result.Items) == 0 {
		return nil // Don't write empty files
	}

	// Sanitize the source for the filename
	safeSource := strings.ReplaceAll(result.Source, " ", "")
	filename := fmt.Sprintf("Out-%s.json", safeSource)

	// Marshal the items with pretty printing
	jsonData, err := json.MarshalIndent(result.Items, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results to JSON: %w", err)
	}

	// Write the file
	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON to file %s: %w", filename, err)
	}

	log.Printf("Successfully wrote %d results to %s", len(result.Items), filename)
	return nil
}
