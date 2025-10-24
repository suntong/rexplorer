import json
import math
from collections import defaultdict

# 1. Load Data
def load_data(file_path):
    """Loads and parses the JSON data from the file."""
    print("Loading data...")
    with open(file_path, 'r') as f:
        data = json.load(f)
    print(f"Loaded {len(data)} repositories.")
    return data

# 2. Categorization Logic
def infer_category(repo):
    """Infers the technology stack category based on language, topics, and description."""
    lang = repo.get('language', '').lower()
    topics = [t.lower() for t in repo.get('topics', [])]
    description = repo.get('description', '').lower()

    # Keywords for categorization
    go_keywords = ['go', 'golang', 'gin', 'gorm', 'beego', 'echo', 'backend', 'server']
    vue_keywords = ['vue', 'vuejs', 'element-ui', 'nuxt', 'frontend', 'ui', 'admin', 'spa']
    react_keywords = ['react', 'nextjs', 'reactjs']
    kubernetes_keywords = ['kubernetes', 'k8s', 'cluster', 'operator', 'helm']
    devops_keywords = ['devops', 'cicd', 'deploy', 'terminal', 'sftp', 'monitor', 'nginx']

    # Check for specific combinations
    is_go = lang == 'go' or any(k in topics for k in go_keywords) or any(k in description for k in go_keywords)
    is_vue = lang in ['vue', 'javascript', 'typescript'] or any(k in topics for k in vue_keywords) or any(k in description for k in vue_keywords)
    is_react = lang in ['javascript', 'typescript'] and (any(k in topics for k in react_keywords) or any(k in description for k in react_keywords))
    is_k8s = any(k in topics for k in kubernetes_keywords) or any(k in description for k in kubernetes_keywords)
    is_devops = any(k in topics for k in devops_keywords) or any(k in description for k in devops_keywords)

    # Determine Category
    if is_k8s:
        return "Kubernetes/Cloud-Native Tools"
    elif is_devops:
        return "DevOps/Deployment Tools"
    elif is_go and is_vue:
        return "Go & Vue Full-stack Applications"
    elif is_go:
        return "Go Backend/CLI Tools"
    elif is_vue:
        # Check for other frontend frameworks if primary is Vue/JS/TS
        if is_react:
            return "React & Go/Vue Mixed Frontend" # Less likely but possible, keep it for robustness
        return "Vue Frontend Applications"
    elif is_react:
        return "React Frontend Applications"
    else:
        # Fallback for projects with 'Unknown' language or non-obvious stacks
        if 'blog' in description or 'blog' in topics:
            return "Blogging/CMS"
        return "Miscellaneous/Uncategorized"

# 3. Ranking and Sorting Logic
def calculate_score(repo):
    """
    Calculates the custom ranking score:
    log(stars+3,4)*log(forks+3,4)/log(open_issues_count+3,4)
    """
    stars = repo.get('stars', 0)
    forks = repo.get('forks', 0)
    issues = repo.get('open_issues_count', 0)

    # Helper function for log base 4
    def log4(x):
        return math.log(x, 4)

    # To avoid log(0) which is undefined, we use the formula provided: log(x+3, 4)
    # The formula is robust against zero values for stars, forks, and issues.
    try:
        numerator = log4(stars + 3) * log4(forks + 3)
        denominator = log4(issues + 3)
        # Note: A low open_issues_count (e.g., 0) will result in a smaller denominator (log4(3) ~ 0.79),
        # which increases the score, favoring well-maintained projects with few open issues.
        # This aligns with the spirit of the formula where `open_issues_count` is in the denominator.
        return numerator / denominator
    except ValueError:
        # This should not happen with the +3 offset, but as a safeguard
        return 0.0
    except ZeroDivisionError:
        # This should only happen if log4(issues + 3) is 0, which means issues+3 = 1, issues = -2. Not possible.
        return 0.0

def process_data(data):
    """Categorizes, calculates scores, and groups repositories."""
    categorized_repos = defaultdict(list)

    for repo in data:
        category = infer_category(repo)
        score = calculate_score(repo)
        repo['score'] = score
        repo['category'] = category
        categorized_repos[category].append(repo)

    # Sort repositories within each category
    for category, repos in categorized_repos.items():
        repos.sort(key=lambda x: x['score'], reverse=True)

    # Calculate category popularity (total repos per category)
    category_popularity = {
        category: len(repos) for category, repos in categorized_repos.items()
    }

    # Rank categories by popularity (descending)
    ranked_categories = sorted(
        category_popularity.items(),
        key=lambda item: item[1],
        reverse=True
    )

    return categorized_repos, ranked_categories

# 4. Presentation
def format_results(categorized_repos, ranked_categories):
    """Formats the results into a markdown string for presentation."""
    markdown_output = "# GitHub Repository Analysis: Technology Stack and Popularity Ranking\n\n"
    markdown_output += "The repositories were analyzed, categorized by inferred technology stack, and ranked by popularity (total number of repositories in the category). Repositories within each category are sorted by a custom score: `log(stars+3,4)*log(forks+3,4)/log(open_issues_count+3,4)`.\n\n"

    for category, count in ranked_categories:
        repos = categorized_repos[category]
        markdown_output += f"## {category} (Total Repositories: {count})\n\n"

        # Create a table for the top 10 repositories in the category
        table_data = []
        for i, repo in enumerate(repos[:10]):
            # Truncate description for table
            description = repo.get('description', 'No description provided')
            if len(description) > 50:
                description = description[:47] + '...'

            table_data.append([
                i + 1,
                f"[{repo['name']}]({repo['url']})",
                f"{repo['score']:.3f}",
                f"{repo['stars']:,}",
                f"{repo['forks']:,}",
                f"{repo['open_issues_count']:,}",
                repo.get('language', 'N/A'),
                description
            ])

        # Format table
        markdown_output += "| Rank | Repository | Score | Stars | Forks | Issues | Lang | Description |\n"
        markdown_output += "| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n"
        for row in table_data:
            markdown_output += "| " + " | ".join(map(str, row)) + " |\n"
        markdown_output += "\n"

        # Add a note if there are more than 10 repos
        if len(repos) > 10:
            markdown_output += f"*(Showing top 10 of {count} repositories in this category.)*\n\n"

    return markdown_output

# Main execution
if __name__ == "__main__":
    try:
        repo_data = load_data('../Out-GitHub.json')
        categorized_repos, ranked_categories = process_data(repo_data)
        final_report = format_results(categorized_repos, ranked_categories)

        # Save the final report to a markdown file
        with open('analysis_report.md', 'w') as f:
            f.write(final_report)

        print("Analysis complete. Report saved to analysis_report.md")

    except Exception as e:
        print(f"An error occurred: {e}")
