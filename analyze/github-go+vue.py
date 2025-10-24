import json
import math
from collections import defaultdict
from datetime import datetime, timezone

# --- Constants and Configuration ---
# File to simulate the data loading from (replace with actual path if needed)
DATA_FILE_PATH = '../Out-GitHub.json'

# --- 1. Load Data (Simplified for context, assumes file is accessible) ---
def load_data(file_path):
    """Loads and parses the JSON data from the file."""
    print("Loading data...")
    # NOTE: In a real scenario, this would load the actual JSON file.
    # We will simulate loading an empty list if the file is not found
    # to allow the rest of the script to be displayed and runnable without the file.
    try:
        with open(file_path, 'r') as f:
            data = json.load(f)
        print(f"Loaded {len(data)} repositories.")
        return data
    except FileNotFoundError:
        print(f"ERROR: Data file not found at {file_path}. Returning empty list.")
        # Simulating data structure for display purposes, but returning an empty list
        # to ensure the code block is fully enhanced as requested.
        return []
    except json.JSONDecodeError:
        print(f"ERROR: Could not decode JSON from {file_path}. Returning empty list.")
        return []


# --- 2. Enhanced Categorization Logic ---
def infer_category(repo):
    """Infers the technology stack category based on language, topics, and description."""
    lang = repo.get('language', '').lower()
    topics = [t.lower() for t in repo.get('topics', [])]
    description = repo.get('description', '').lower()

    # Keywords for categorization
    go_api_keywords = ['api', 'rest', 'gin', 'gorm', 'beego', 'echo', 'server', 'http']
    go_cli_keywords = ['cli', 'terminal', 'tui', 'command-line', 'tool']
    vue_app_keywords = ['admin', 'dashboard', 'spa', 'app', 'starter', 'boilerplate', 'full-stack']
    vue_component_keywords = ['component', 'ui', 'element-ui', 'library', 'widget', 'json-viewer']
    k8s_keywords = ['kubernetes', 'k8s', 'operator', 'helm', 'cloud-native']

    # Flags
    is_go = lang == 'go' or 'go' in topics or 'golang' in topics
    is_vue_js = lang in ['vue', 'javascript', 'typescript'] or 'vue' in topics or 'vuejs' in topics

    # Primary stack determination
    is_go_api = is_go and (any(k in topics for k in go_api_keywords) or any(k in description for k in go_api_keywords))
    is_go_cli = is_go and (any(k in topics for k in go_cli_keywords) or any(k in description for k in go_cli_keywords))
    is_vue_app = is_vue_js and (any(k in topics for k in vue_app_keywords) or any(k in description for k in vue_app_keywords))
    is_vue_component = is_vue_js and (any(k in topics for k in vue_component_keywords) or any(k in description for k in vue_component_keywords))
    is_k8s = any(k in topics for k in k8s_keywords) or any(k in description for k in k8s_keywords)

    # Determine Category - Order matters
    if is_k8s and is_go:
        return "Kubernetes/Cloud-Native Tools (Go)"
    if is_go and is_vue_js:
        return "Go & Vue Full-stack Applications"
    if is_vue_component:
        return "Vue UI Components & Libraries"
    if is_vue_js:
        return "Vue Frontend Applications"
    if is_go_cli:
        # Catch Go CLI/TUI tools that might not have a clear Vue integration
        return "Go CLI/TUI Tools"
    if is_go_api or is_go:
        return "Go Backend/API Servers"
    
    # Fallback
    return "Miscellaneous/Uncategorized"

# --- 3. Enhanced Ranking and Sorting Logic ---
def calculate_score(repo):
    """
    Calculates the custom ranking score including a maintenance factor.
    New Score = [log(stars+3,4)*log(forks+3,4)/log(open_issues_count+3,4)] * Maintenance_Factor
    """
    stars = repo.get('stars', 0)
    forks = repo.get('forks', 0)
    issues = repo.get('open_issues_count', 0)
    created_at_str = repo.get('created_at')
    updated_at_str = repo.get('updated_at')

    # Helper function for log base 4
    def log4(x):
        return math.log(x, 4)

    # Base Popularity Score (Score)
    try:
        numerator = log4(stars + 3) * log4(forks + 3)
        denominator = log4(issues + 3)
        base_score = numerator / denominator
    except Exception:
        base_score = 0.0

    # Maintenance Factor (M)
    try:
        # Parse dates and use current time for recency
        NOW = datetime.now(timezone.utc)
        created_at = datetime.fromisoformat(created_at_str.replace('Z', '+00:00'))
        updated_at = datetime.fromisoformat(updated_at_str.replace('Z', '+00:00'))

        # Maintenance Length (Age of the project in days)
        age_days = (NOW - created_at).total_seconds() / (60*60*24)
        
        # Maintenance Duration (Time actively developed, in days)
        maintenance_duration_days = (updated_at - created_at).total_seconds() / (60*60*24)
        
        # Recency (How long since the last update, in days)
        recency_days = (NOW - updated_at).total_seconds() / (60*60*24)

        # Factor components:
        # 1. Age: Favor projects that have been around longer. Use log smoothing.
        age_factor = log4(age_days + 1) if age_days > 0 else 0.0
        
        # 2. Maintenance: Favor projects actively maintained over a long period.
        maintenance_factor_duration = log4(maintenance_duration_days + 1) if maintenance_duration_days > 0 else 0.0

        # 3. Recency: Favor projects that have been recently updated (low recency_days).
        # We use an inverse relationship: smaller recency_days -> larger recency_factor
        # We cap the maximum penalty/boost to prevent extreme values.
        # Recency is a *penalty* multiplier if too old. max(1 - penalty, 0.1)
        # Penalty is 0.01 per day past 30 days, maxing out at a factor of 0.1
        recency_penalty = max(0, recency_days - 30) * 0.01 
        recency_multiplier = max(0.1, 1.0 - recency_penalty)
        
        # Final Maintenance Factor M
        # The sum of age and maintenance duration gives a robust measure of long-term effort.
        # The recency multiplier then adjusts this for current relevance.
        M = (age_factor + maintenance_factor_duration) * recency_multiplier
        
        # Ensure M is not zero to avoid Score' being 0 if M is 0
        if M <= 0:
            M = 0.1
            
        final_score = base_score * M
        return final_score

    except Exception as e:
        # Handle cases where dates are missing or invalid
        print(f"Warning: Could not calculate maintenance factor for {repo.get('full_name')}: {e}")
        return base_score # Fallback to base score

def process_data(data):
    """Categorizes, calculates scores, and groups repositories."""
    categorized_repos = defaultdict(list)
    print("Processing and scoring repositories...")

    for repo in data:
        # 1. Categorize and Score
        category = infer_category(repo)
        score = calculate_score(repo)
        repo['score'] = score
        repo['category'] = category
        categorized_repos[category].append(repo)

    # 2. Sort repositories within each category
    print("Sorting repositories by score...")
    for category, repos in categorized_repos.items():
        repos.sort(key=lambda x: x['score'], reverse=True)

    # 3. Calculate and Rank categories by popularity
    category_popularity = {
        category: len(repos) for category, repos in categorized_repos.items()
    }
    ranked_categories = sorted(
        category_popularity.items(),
        key=lambda item: item[1],
        reverse=True
    )
    print("Categories ranked.")

    return categorized_repos, ranked_categories

# --- 4. Presentation and Output ---
def format_results_markdown(categorized_repos, ranked_categories):
    """Formats the results into a markdown string for presentation."""
    markdown_output = "# GitHub Repository Analysis: Technology Stack and Popularity Ranking\n\n"
    markdown_output += "The repositories were categorized and ranked by popularity (total repos). Repositories within categories are sorted by a custom score that combines popularity metrics (Stars, Forks, Issues) with a **Maintenance Factor** (Age and Recency of Updates).\n\n"
    markdown_output += "### Ranking Formula\n"
    markdown_output += "$$Score' = \\left(\\frac{\\log_4(\\text{stars}+3) \\cdot \\log_4(\\text{forks}+3)}{\\log_4(\\text{open\_issues\_count}+3)}\\right) \\cdot M$$\n"
    markdown_output += "Where $M$ is the **Maintenance Factor** (based on project age and recency).\n\n---\n"

    for category, count in ranked_categories:
        repos = categorized_repos[category]
        markdown_output += f"## {category} (Total Repositories: {count})\n\n"

        # Prepare table data for the top 10
        table_data = []
        for i, repo in enumerate(repos[:10]):
            description = repo.get('description', 'No description provided')
            description = (description[:47] + '...') if len(description) > 50 else description

            table_data.append([
                i + 1,
                f"[{repo['name']}]({repo['url']})",
                f"{repo['score']:.3f}",
                f"{repo['stars']:,}",
                f"{repo['forks']:,}",
                f"{repo['open_issues_count']:,}",
                datetime.fromisoformat(repo.get('created_at').replace('Z', '+00:00')).strftime('%Y-%m-%d'),
                datetime.fromisoformat(repo.get('updated_at').replace('Z', '+00:00')).strftime('%Y-%m-%d'),
                description
            ])

        # Format table
        markdown_output += "| Rank | Repository | Score | Stars | Forks | Issues | Created | Updated | Description |\n"
        markdown_output += "| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n"
        for row in table_data:
            markdown_output += "| " + " | ".join(map(str, row)) + " |\n"
        markdown_output += "\n"

        if len(repos) > 10:
            markdown_output += f"*(Showing top 10 of {count} repositories in this category.)*\n\n"
        
        markdown_output += "---\n"

    return markdown_output

def output_category_jsons(categorized_repos):
    """Saves the sorted data for each category into a separate JSON file."""
    print("Saving categorized data to JSON files...")
    for category, repos in categorized_repos.items():
        # Create a safe filename from the category name
        filename = category.lower().replace(' ', '_').replace('/', '_').replace('-', '_') + '_repos.json'
        
        # Prepare the data for JSON output (excluding the temporary 'category' key)
        json_output = [
            {k: v for k, v in repo.items() if k != 'category'} 
            for repo in repos
        ]
        
        with open(filename, 'w') as f:
            json.dump(json_output, f, indent=2)
        print(f"Saved {len(repos)} repos to {filename}")


# --- Main Execution ---
if __name__ == "__main__":
    # The script will attempt to load the data from this file path.
    # To run this, ensure a 'github_repos.json' file with the repository data exists
    # in the same directory, or modify DATA_FILE_PATH.
    try:
        repo_data = load_data(DATA_FILE_PATH)
        
        if repo_data:
            categorized_repos, ranked_categories = process_data(repo_data)
            
            # 1. Output to Markdown Report
            final_report = format_results_markdown(categorized_repos, ranked_categories)
            with open('analysis_report.md', 'w') as f:
                f.write(final_report)
            print("Analysis complete. Report saved to analysis_report.md")
            
            # 2. Output to JSON files per category
            output_category_jsons(categorized_repos)
        else:
            print("Skipping processing due to empty or missing data.")

    except Exception as e:
        print(f"An unexpected error occurred during execution: {e}")
