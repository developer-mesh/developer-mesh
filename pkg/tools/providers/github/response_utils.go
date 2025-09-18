package github

import (
	"github.com/google/go-github/v74/github"
)

const (
	// MaxBodyLength is the maximum length for body/description fields in simplified responses
	MaxBodyLength = 500
)

// truncateString truncates a string to the specified length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// extractActorLogin safely extracts the login from a User object
func extractActorLogin(user *github.User) string {
	if user == nil {
		return ""
	}
	return user.GetLogin()
}

// extractLabelNames extracts label names from a slice of Label objects
func extractLabelNames(labels []*github.Label) []string {
	names := make([]string, 0, len(labels))
	for _, label := range labels {
		if label != nil {
			names = append(names, label.GetName())
		}
	}
	return names
}

// extractAssigneeLogins extracts login names from a slice of User objects
func extractAssigneeLogins(users []*github.User) []string {
	logins := make([]string, 0, len(users))
	for _, user := range users {
		if user != nil && user.Login != nil {
			logins = append(logins, *user.Login)
		}
	}
	return logins
}

// simplifyWorkflowRun creates a simplified version of a workflow run
func simplifyWorkflowRun(run *github.WorkflowRun) map[string]interface{} {
	if run == nil {
		return nil
	}

	simplified := map[string]interface{}{
		"id":          run.GetID(),
		"name":        run.GetName(),
		"status":      run.GetStatus(),
		"conclusion":  run.GetConclusion(),
		"workflow_id": run.GetWorkflowID(),
		"run_number":  run.GetRunNumber(),
		"run_attempt": run.GetRunAttempt(),
		"event":       run.GetEvent(),
		"head_branch": run.GetHeadBranch(),
		"head_sha":    run.GetHeadSHA(),
		"html_url":    run.GetHTMLURL(),
		"created_at":  run.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
		"updated_at":  run.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
	}

	// Add actor if available
	if run.Actor != nil {
		simplified["actor"] = extractActorLogin(run.Actor)
	}

	// Add head commit message if available
	if run.HeadCommit != nil {
		simplified["head_commit_message"] = truncateString(run.HeadCommit.GetMessage(), 200)
	}

	// Add PR numbers if this run is associated with pull requests
	if len(run.PullRequests) > 0 {
		prNumbers := make([]int, 0, len(run.PullRequests))
		for _, pr := range run.PullRequests {
			if pr != nil && pr.Number != nil {
				prNumbers = append(prNumbers, *pr.Number)
			}
		}
		simplified["pull_requests"] = prNumbers
	}

	return simplified
}

// simplifyPullRequest creates a simplified version of a pull request
func simplifyPullRequest(pr *github.PullRequest) map[string]interface{} {
	if pr == nil {
		return nil
	}

	simplified := map[string]interface{}{
		"number":        pr.GetNumber(),
		"id":            pr.GetID(),
		"state":         pr.GetState(),
		"title":         pr.GetTitle(),
		"body":          truncateString(pr.GetBody(), MaxBodyLength),
		"html_url":      pr.GetHTMLURL(),
		"created_at":    pr.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
		"updated_at":    pr.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
		"merged":        pr.GetMerged(),
		"mergeable":     pr.GetMergeable(),
		"draft":         pr.GetDraft(),
		"comments":      pr.GetComments(),
		"commits":       pr.GetCommits(),
		"additions":     pr.GetAdditions(),
		"deletions":     pr.GetDeletions(),
		"changed_files": pr.GetChangedFiles(),
	}

	// Add user login
	if pr.User != nil {
		simplified["user"] = extractActorLogin(pr.User)
	}

	// Add head and base info
	if pr.Head != nil {
		simplified["head"] = map[string]interface{}{
			"ref": pr.Head.GetRef(),
			"sha": pr.Head.GetSHA(),
		}
	}
	if pr.Base != nil {
		simplified["base"] = map[string]interface{}{
			"ref": pr.Base.GetRef(),
			"sha": pr.Base.GetSHA(),
		}
	}

	// Add assignees
	if len(pr.Assignees) > 0 {
		simplified["assignees"] = extractAssigneeLogins(pr.Assignees)
	}

	// Add labels
	if len(pr.Labels) > 0 {
		simplified["labels"] = extractLabelNames(pr.Labels)
	}

	// Add merged_at if merged
	if pr.MergedAt != nil {
		simplified["merged_at"] = pr.MergedAt.Format("2006-01-02T15:04:05Z")
	}

	return simplified
}

// simplifyIssue creates a simplified version of an issue
func simplifyIssue(issue *github.Issue) map[string]interface{} {
	if issue == nil {
		return nil
	}

	simplified := map[string]interface{}{
		"number":     issue.GetNumber(),
		"id":         issue.GetID(),
		"state":      issue.GetState(),
		"title":      issue.GetTitle(),
		"body":       truncateString(issue.GetBody(), MaxBodyLength),
		"html_url":   issue.GetHTMLURL(),
		"created_at": issue.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
		"updated_at": issue.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
		"comments":   issue.GetComments(),
		"locked":     issue.GetLocked(),
	}

	// Add user login
	if issue.User != nil {
		simplified["user"] = extractActorLogin(issue.User)
	}

	// Add assignees
	if len(issue.Assignees) > 0 {
		simplified["assignees"] = extractAssigneeLogins(issue.Assignees)
	}

	// Add labels
	if len(issue.Labels) > 0 {
		simplified["labels"] = extractLabelNames(issue.Labels)
	}

	// Add milestone title if available
	if issue.Milestone != nil {
		simplified["milestone"] = issue.Milestone.GetTitle()
	}

	// Add closed_at if closed
	if issue.ClosedAt != nil {
		simplified["closed_at"] = issue.ClosedAt.Format("2006-01-02T15:04:05Z")
	}

	// Indicate if this is a pull request (GitHub treats PRs as issues too)
	if issue.PullRequestLinks != nil {
		simplified["is_pull_request"] = true
	}

	return simplified
}

// simplifyCommit creates a simplified version of a repository commit
func simplifyCommit(commit *github.RepositoryCommit) map[string]interface{} {
	if commit == nil {
		return nil
	}

	simplified := map[string]interface{}{
		"sha":      commit.GetSHA(),
		"html_url": commit.GetHTMLURL(),
	}

	// Add commit details
	if commit.Commit != nil {
		simplified["message"] = truncateString(commit.Commit.GetMessage(), 200)

		if commit.Commit.Author != nil {
			simplified["author"] = map[string]interface{}{
				"name":  commit.Commit.Author.GetName(),
				"email": commit.Commit.Author.GetEmail(),
				"date":  commit.Commit.Author.GetDate().Format("2006-01-02T15:04:05Z"),
			}
		}

		if commit.Commit.Committer != nil {
			simplified["committer"] = map[string]interface{}{
				"name":  commit.Commit.Committer.GetName(),
				"email": commit.Commit.Committer.GetEmail(),
				"date":  commit.Commit.Committer.GetDate().Format("2006-01-02T15:04:05Z"),
			}
		}

		// Add tree SHA for reference
		if commit.Commit.Tree != nil {
			simplified["tree_sha"] = commit.Commit.Tree.GetSHA()
		}

		// Add parent SHAs
		if len(commit.Commit.Parents) > 0 {
			parents := make([]string, 0, len(commit.Commit.Parents))
			for _, parent := range commit.Commit.Parents {
				if parent != nil {
					parents = append(parents, parent.GetSHA())
				}
			}
			simplified["parents"] = parents
		}
	}

	// Add author login if available
	if commit.Author != nil {
		simplified["author_login"] = extractActorLogin(commit.Author)
	}

	// Add stats if available
	if commit.Stats != nil {
		simplified["stats"] = map[string]interface{}{
			"additions": commit.Stats.GetAdditions(),
			"deletions": commit.Stats.GetDeletions(),
			"total":     commit.Stats.GetTotal(),
		}
	}

	// Add file count if files are included
	if len(commit.Files) > 0 {
		simplified["files_changed"] = len(commit.Files)
	}

	return simplified
}

// simplifyRepository creates a simplified version of a repository
func simplifyRepository(repo *github.Repository) map[string]interface{} {
	if repo == nil {
		return nil
	}

	simplified := map[string]interface{}{
		"id":          repo.GetID(),
		"name":        repo.GetName(),
		"full_name":   repo.GetFullName(),
		"description": truncateString(repo.GetDescription(), MaxBodyLength),
		"private":     repo.GetPrivate(),
		"fork":        repo.GetFork(),
		"html_url":    repo.GetHTMLURL(),
		"clone_url":   repo.GetCloneURL(),
		"ssh_url":     repo.GetSSHURL(),
		"language":    repo.GetLanguage(),
		"size":        repo.GetSize(),
		"stars":       repo.GetStargazersCount(),
		"watchers":    repo.GetWatchersCount(),
		"forks":       repo.GetForksCount(),
		"open_issues": repo.GetOpenIssuesCount(),
		"created_at":  repo.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
		"updated_at":  repo.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
		"pushed_at":   repo.GetPushedAt().Format("2006-01-02T15:04:05Z"),
		"archived":    repo.GetArchived(),
		"disabled":    repo.GetDisabled(),
	}

	// Add owner login
	if repo.Owner != nil {
		simplified["owner"] = extractActorLogin(repo.Owner)
	}

	// Add default branch
	simplified["default_branch"] = repo.GetDefaultBranch()

	// Add license info if available
	if repo.License != nil {
		simplified["license"] = repo.License.GetKey()
	}

	// Add topics if available
	if len(repo.Topics) > 0 {
		simplified["topics"] = repo.Topics
	}

	// Add visibility
	if repo.Visibility != nil {
		simplified["visibility"] = *repo.Visibility
	}

	return simplified
}

// simplifyCodeResult creates a simplified version of a code search result
func simplifyCodeResult(result *github.CodeResult) map[string]interface{} {
	if result == nil {
		return nil
	}

	simplified := map[string]interface{}{
		"name":     result.GetName(),
		"path":     result.GetPath(),
		"sha":      result.GetSHA(),
		"html_url": result.GetHTMLURL(),
	}

	// Add repository name if available
	if result.Repository != nil {
		simplified["repository"] = result.Repository.GetFullName()
	}

	// Add text matches if available (for context)
	if len(result.TextMatches) > 0 {
		matches := make([]map[string]interface{}, 0, len(result.TextMatches))
		for _, match := range result.TextMatches {
			if match != nil {
				matchMap := map[string]interface{}{
					"fragment": truncateString(match.GetFragment(), 200),
				}
				if match.Property != nil {
					matchMap["property"] = *match.Property
				}
				matches = append(matches, matchMap)
			}
		}
		simplified["text_matches"] = matches
	}

	return simplified
}
