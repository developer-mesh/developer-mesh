package github

import (
	"context"

	"github.com/shurcooL/githubv4"
)

// Discussion represents a GitHub discussion
type Discussion struct {
	ID             string
	Number         int
	Title          string
	Body           string
	Author         string
	Category       string
	CreatedAt      string
	UpdatedAt      string
	AnswerChosenAt *string
	IsAnswered     bool
	Locked         bool
	Comments       int
	URL            string
}

// ListDiscussionsHandler lists discussions in a repository
type ListDiscussionsHandler struct {
	provider *GitHubProvider
}

// NewListDiscussionsHandler creates a new handler
func NewListDiscussionsHandler(p *GitHubProvider) *ListDiscussionsHandler {
	return &ListDiscussionsHandler{provider: p}
}

// Execute lists discussions using GraphQL (REST API doesn't support discussions)
func (h *ListDiscussionsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	owner := extractString(params, "owner")
	repo := extractString(params, "repo")

	if owner == "" || repo == "" {
		return ErrorResult("owner and repo are required"), nil
	}

	client, err := h.provider.getGraphQLClient(ctx)
	if err != nil {
		return ErrorResult("Failed to get GraphQL client: %v", err), nil
	}

	// GraphQL query for discussions
	var query struct {
		Repository struct {
			Discussions struct {
				Nodes []struct {
					ID     string
					Number int
					Title  string
					Body   string
					Author struct {
						Login string
					}
					Category struct {
						Name string
					}
					CreatedAt      githubv4.DateTime
					UpdatedAt      githubv4.DateTime
					AnswerChosenAt *githubv4.DateTime
					IsAnswered     bool
					Locked         bool
					Comments       struct {
						TotalCount int
					}
					URL string
				}
				PageInfo struct {
					HasNextPage bool
					EndCursor   githubv4.String
				}
				TotalCount int
			} `graphql:"discussions(first: $first, after: $after, categoryId: $categoryId, orderBy: $orderBy)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	// Build variables
	variables := map[string]interface{}{
		"owner": githubv4.String(owner),
		"repo":  githubv4.String(repo),
		"first": githubv4.Int(30),
	}

	// Handle pagination
	if after := extractString(params, "after"); after != "" {
		variables["after"] = githubv4.String(after)
	} else {
		variables["after"] = (*githubv4.String)(nil)
	}

	// Handle category filter
	if categoryId := extractString(params, "category_id"); categoryId != "" {
		variables["categoryId"] = githubv4.ID(categoryId)
	} else {
		variables["categoryId"] = (*githubv4.ID)(nil)
	}

	// Handle ordering
	variables["orderBy"] = githubv4.DiscussionOrder{
		Field:     githubv4.DiscussionOrderFieldUpdatedAt,
		Direction: githubv4.OrderDirectionDesc,
	}

	// Execute query
	err = client.Query(ctx, &query, variables)
	if err != nil {
		return ErrorResult("Failed to list discussions: %v", err), nil
	}

	// Convert to response format
	discussions := make([]map[string]interface{}, 0, len(query.Repository.Discussions.Nodes))
	for _, disc := range query.Repository.Discussions.Nodes {
		discussion := map[string]interface{}{
			"id":          disc.ID,
			"number":      disc.Number,
			"title":       disc.Title,
			"body":        disc.Body,
			"author":      disc.Author.Login,
			"category":    disc.Category.Name,
			"created_at":  disc.CreatedAt.Time,
			"updated_at":  disc.UpdatedAt.Time,
			"is_answered": disc.IsAnswered,
			"locked":      disc.Locked,
			"comments":    disc.Comments.TotalCount,
			"url":         disc.URL,
		}

		if disc.AnswerChosenAt != nil {
			discussion["answer_chosen_at"] = disc.AnswerChosenAt.Time
		}

		discussions = append(discussions, discussion)
	}

	result := map[string]interface{}{
		"discussions": discussions,
		"pageInfo": map[string]interface{}{
			"hasNextPage": query.Repository.Discussions.PageInfo.HasNextPage,
			"endCursor":   string(query.Repository.Discussions.PageInfo.EndCursor),
		},
		"totalCount": query.Repository.Discussions.TotalCount,
	}

	return SuccessResult(result), nil
}

// GetDefinition returns the tool definition
func (h *ListDiscussionsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "discussions_list",
		Description: GetOperationDescription("discussions_list"),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner (username or organization)",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
					"example":     "Hello-World",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"category_id": map[string]interface{}{
					"type":        "string",
					"description": "Filter by discussion category ID (GraphQL node ID)",
					"example":     "DIC_kwDOABCD5M4CABCD",
					"pattern":     "^DIC_[a-zA-Z0-9]+$",
				},
				"after": map[string]interface{}{
					"type":        "string",
					"description": "Cursor for pagination (from previous response's endCursor)",
					"example":     "Y3Vyc29yOnYyOpHOAAAAAA==",
				},
			},
			"required": []string{"owner", "repo"},
		},
		Metadata: map[string]interface{}{
			"oauth_scopes": []string{"repo", "read:discussion"},
			"rate_limit":   "graphql",
			"api_version":  "GraphQL",
			"points_cost":  1,
			"graphql_only": true, // Discussions API is GraphQL-only
		},
		ResponseExample: map[string]interface{}{
			"discussions": []map[string]interface{}{
				{
					"id":               "D_kwDOABCD5M4AABCD",
					"number":           42,
					"title":            "How to contribute?",
					"body":             "I'd like to contribute to this project...",
					"author":           "newcontributor",
					"category":         "Q&A",
					"created_at":       "2024-01-15T10:00:00Z",
					"updated_at":       "2024-01-16T14:30:00Z",
					"answer_chosen_at": "2024-01-16T15:00:00Z",
					"is_answered":      true,
					"locked":           false,
					"comments":         5,
					"url":              "https://github.com/octocat/Hello-World/discussions/42",
				},
			},
			"pageInfo": map[string]interface{}{
				"hasNextPage": true,
				"endCursor":   "Y3Vyc29yOnYyOpHOAAAAAA==",
			},
			"totalCount": 123,
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "Discussions not enabled",
				"cause":    "Repository has discussions disabled",
				"solution": "Enable discussions in repository settings",
			},
			{
				"error":    "Category not found",
				"cause":    "Invalid category ID provided",
				"solution": "Use discussion_categories_list to get valid category IDs",
			},
			{
				"error":    "GraphQL required",
				"cause":    "Discussions API only available via GraphQL",
				"solution": "Ensure GraphQL client is configured",
			},
		},
		ExtendedHelp: `The discussions_list operation retrieves repository discussions using GitHub's GraphQL API.

Discussions are:
- Community conversations for Q&A, ideas, and general discussion
- Different from issues (not for tracking work)
- Organized by categories
- Can be marked as answered (for Q&A categories)

Category types:
- 💬 General: Open-ended conversations
- 🙏 Q&A: Questions with accepted answers
- 💡 Ideas: Feature requests and suggestions
- 🎆 Show and tell: Share creations
- 📣 Announcements: Updates and news

Filtering:
- By category: Use category_id from discussion_categories_list
- Pagination: Use cursor-based pagination with 'after'
- Default ordering: Most recently updated first

Examples:

# List all discussions
{
  "owner": "facebook",
  "repo": "react"
}

# Filter by category
{
  "owner": "facebook",
  "repo": "react",
  "category_id": "DIC_kwDOABCD5M4CABCD"
}

# Paginate through results
{
  "owner": "facebook",
  "repo": "react",
  "after": "Y3Vyc29yOnYyOpHOAAAAAA=="
}

Use cases:
- Community support forums
- Feature request tracking
- Project announcements
- Knowledge base Q&A`,
	}
}

// GetDiscussionHandler gets a specific discussion
type GetDiscussionHandler struct {
	provider *GitHubProvider
}

// NewGetDiscussionHandler creates a new handler
func NewGetDiscussionHandler(p *GitHubProvider) *GetDiscussionHandler {
	return &GetDiscussionHandler{provider: p}
}

// Execute gets a discussion by number
func (h *GetDiscussionHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	discussionNumber := extractInt(params, "discussion_number")

	if owner == "" || repo == "" || discussionNumber == 0 {
		return ErrorResult("owner, repo, and discussion_number are required"), nil
	}

	client, err := h.provider.getGraphQLClient(ctx)
	if err != nil {
		return ErrorResult("Failed to get GraphQL client: %v", err), nil
	}

	// GraphQL query for a specific discussion
	var query struct {
		Repository struct {
			Discussion struct {
				ID       string
				Number   int
				Title    string
				Body     string
				BodyHTML string
				Author   struct {
					Login     string
					AvatarUrl string
				}
				Category struct {
					ID          string
					Name        string
					Description string
					Emoji       string
				}
				CreatedAt      githubv4.DateTime
				UpdatedAt      githubv4.DateTime
				AnswerChosenAt *githubv4.DateTime
				IsAnswered     bool
				Locked         bool
				LockedAt       *githubv4.DateTime
				Comments       struct {
					TotalCount int
					Nodes      []struct {
						ID     string
						Body   string
						Author struct {
							Login string
						}
						CreatedAt githubv4.DateTime
						UpdatedAt githubv4.DateTime
					}
				} `graphql:"comments(first: 10)"`
				Labels struct {
					Nodes []struct {
						Name  string
						Color string
					}
				} `graphql:"labels(first: 10)"`
				URL          string
				ResourcePath string
				Upvotes      int `graphql:"upvoteCount"`
			} `graphql:"discussion(number: $number)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	variables := map[string]interface{}{
		"owner":  githubv4.String(owner),
		"repo":   githubv4.String(repo),
		"number": githubv4.Int(discussionNumber),
	}

	err = client.Query(ctx, &query, variables)
	if err != nil {
		return ErrorResult("Failed to get discussion: %v", err), nil
	}

	disc := query.Repository.Discussion

	// Format comments
	comments := make([]map[string]interface{}, 0, len(disc.Comments.Nodes))
	for _, comment := range disc.Comments.Nodes {
		comments = append(comments, map[string]interface{}{
			"id":         comment.ID,
			"body":       comment.Body,
			"author":     comment.Author.Login,
			"created_at": comment.CreatedAt.Time,
			"updated_at": comment.UpdatedAt.Time,
		})
	}

	// Format labels
	labels := make([]map[string]interface{}, 0, len(disc.Labels.Nodes))
	for _, label := range disc.Labels.Nodes {
		labels = append(labels, map[string]interface{}{
			"name":  label.Name,
			"color": label.Color,
		})
	}

	result := map[string]interface{}{
		"id":        disc.ID,
		"number":    disc.Number,
		"title":     disc.Title,
		"body":      disc.Body,
		"body_html": disc.BodyHTML,
		"author": map[string]interface{}{
			"login":      disc.Author.Login,
			"avatar_url": disc.Author.AvatarUrl,
		},
		"category": map[string]interface{}{
			"id":          disc.Category.ID,
			"name":        disc.Category.Name,
			"description": disc.Category.Description,
			"emoji":       disc.Category.Emoji,
		},
		"created_at":     disc.CreatedAt.Time,
		"updated_at":     disc.UpdatedAt.Time,
		"is_answered":    disc.IsAnswered,
		"locked":         disc.Locked,
		"comments_count": disc.Comments.TotalCount,
		"comments":       comments,
		"labels":         labels,
		"url":            disc.URL,
		"resource_path":  disc.ResourcePath,
		"upvotes":        disc.Upvotes,
	}

	if disc.AnswerChosenAt != nil {
		result["answer_chosen_at"] = disc.AnswerChosenAt.Time
	}

	if disc.LockedAt != nil {
		result["locked_at"] = disc.LockedAt.Time
	}

	return SuccessResult(result), nil
}

// GetDefinition returns the tool definition
func (h *GetDiscussionHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "discussion_get",
		Description: GetOperationDescription("discussion_get"),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner (username or organization)",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
					"example":     "Hello-World",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"discussion_number": map[string]interface{}{
					"type":        "integer",
					"description": "Discussion number (sequential ID in repository)",
					"example":     42,
					"minimum":     1,
				},
			},
			"required": []string{"owner", "repo", "discussion_number"},
		},
		Metadata: map[string]interface{}{
			"oauth_scopes": []string{"repo", "read:discussion"},
			"rate_limit":   "graphql",
			"api_version":  "GraphQL",
			"points_cost":  1,
			"graphql_only": true,
		},
		ResponseExample: map[string]interface{}{
			"id":               "D_kwDOABCD5M4AABCD",
			"number":           42,
			"title":            "How to contribute?",
			"body":             "I'd like to contribute to this project. What's the best way to get started?",
			"author":           "newcontributor",
			"category":         "Q&A",
			"created_at":       "2024-01-15T10:00:00Z",
			"updated_at":       "2024-01-16T14:30:00Z",
			"answer_chosen_at": "2024-01-16T15:00:00Z",
			"answer_chosen_by": "maintainer1",
			"is_answered":      true,
			"locked":           false,
			"locked_at":        nil,
			"comments":         5,
			"reactions": map[string]interface{}{
				"thumbs_up":   10,
				"thumbs_down": 0,
				"heart":       3,
			},
			"url": "https://github.com/octocat/Hello-World/discussions/42",
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "Discussion not found",
				"cause":    "Discussion doesn't exist or was deleted",
				"solution": "Verify discussion number exists",
			},
			{
				"error":    "Discussions not enabled",
				"cause":    "Repository has discussions disabled",
				"solution": "Enable discussions in repository settings",
			},
		},
		ExtendedHelp: `The discussion_get operation retrieves detailed information about a specific discussion.

Returned information includes:
- Full discussion content
- Author information
- Category and tags
- Answer status (for Q&A)
- Lock status
- Reaction counts
- Comment count
- Timestamps

Discussion states:
- Open: Accepting new comments
- Answered: Has accepted answer (Q&A only)
- Locked: No new comments allowed
- Unanswered: Q&A without accepted answer

Example:
{
  "owner": "facebook",
  "repo": "react",
  "discussion_number": 42
}

Use this to:
- Display full discussion thread
- Check if question is answered
- Get discussion metadata
- Monitor discussion activity`,
	}
}

// GetDiscussionCommentsHandler gets comments for a discussion
type GetDiscussionCommentsHandler struct {
	provider *GitHubProvider
}

// NewGetDiscussionCommentsHandler creates a new handler
func NewGetDiscussionCommentsHandler(p *GitHubProvider) *GetDiscussionCommentsHandler {
	return &GetDiscussionCommentsHandler{provider: p}
}

// Execute gets discussion comments
func (h *GetDiscussionCommentsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	discussionNumber := extractInt(params, "discussion_number")

	if owner == "" || repo == "" || discussionNumber == 0 {
		return ErrorResult("owner, repo, and discussion_number are required"), nil
	}

	client, err := h.provider.getGraphQLClient(ctx)
	if err != nil {
		return ErrorResult("Failed to get GraphQL client: %v", err), nil
	}

	// GraphQL query for discussion comments
	var query struct {
		Repository struct {
			Discussion struct {
				ID       string
				Title    string
				Comments struct {
					Nodes []struct {
						ID       string
						Body     string
						BodyHTML string
						Author   struct {
							Login     string
							AvatarUrl string
						}
						CreatedAt githubv4.DateTime
						UpdatedAt githubv4.DateTime
						IsAnswer  bool
						Upvotes   int `graphql:"upvoteCount"`
						Replies   struct {
							TotalCount int
							Nodes      []struct {
								ID     string
								Body   string
								Author struct {
									Login string
								}
								CreatedAt githubv4.DateTime
							}
						} `graphql:"replies(first: 5)"`
					}
					PageInfo struct {
						HasNextPage bool
						EndCursor   githubv4.String
					}
					TotalCount int
				} `graphql:"comments(first: $first, after: $after)"`
			} `graphql:"discussion(number: $number)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	variables := map[string]interface{}{
		"owner":  githubv4.String(owner),
		"repo":   githubv4.String(repo),
		"number": githubv4.Int(discussionNumber),
		"first":  githubv4.Int(50),
	}

	// Handle pagination
	if after := extractString(params, "after"); after != "" {
		variables["after"] = githubv4.String(after)
	} else {
		variables["after"] = (*githubv4.String)(nil)
	}

	err = client.Query(ctx, &query, variables)
	if err != nil {
		return ErrorResult("Failed to get discussion comments: %v", err), nil
	}

	// Format comments
	comments := make([]map[string]interface{}, 0, len(query.Repository.Discussion.Comments.Nodes))
	for _, comment := range query.Repository.Discussion.Comments.Nodes {
		// Format replies
		replies := make([]map[string]interface{}, 0, len(comment.Replies.Nodes))
		for _, reply := range comment.Replies.Nodes {
			replies = append(replies, map[string]interface{}{
				"id":         reply.ID,
				"body":       reply.Body,
				"author":     reply.Author.Login,
				"created_at": reply.CreatedAt.Time,
			})
		}

		comments = append(comments, map[string]interface{}{
			"id":        comment.ID,
			"body":      comment.Body,
			"body_html": comment.BodyHTML,
			"author": map[string]interface{}{
				"login":      comment.Author.Login,
				"avatar_url": comment.Author.AvatarUrl,
			},
			"created_at":    comment.CreatedAt.Time,
			"updated_at":    comment.UpdatedAt.Time,
			"is_answer":     comment.IsAnswer,
			"upvotes":       comment.Upvotes,
			"replies_count": comment.Replies.TotalCount,
			"replies":       replies,
		})
	}

	result := map[string]interface{}{
		"discussion_id":    query.Repository.Discussion.ID,
		"discussion_title": query.Repository.Discussion.Title,
		"comments":         comments,
		"pageInfo": map[string]interface{}{
			"hasNextPage": query.Repository.Discussion.Comments.PageInfo.HasNextPage,
			"endCursor":   string(query.Repository.Discussion.Comments.PageInfo.EndCursor),
		},
		"totalCount": query.Repository.Discussion.Comments.TotalCount,
	}

	return SuccessResult(result), nil
}

// GetDefinition returns the tool definition
func (h *GetDiscussionCommentsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "discussion_comments_get",
		Description: GetOperationDescription("discussion_comments_get"),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner (username or organization)",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
					"example":     "Hello-World",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"discussion_number": map[string]interface{}{
					"type":        "integer",
					"description": "Discussion number to get comments for",
					"example":     42,
					"minimum":     1,
				},
				"after": map[string]interface{}{
					"type":        "string",
					"description": "Cursor for pagination (from previous response's endCursor)",
					"example":     "Y3Vyc29yOnYyOpHOAAAAAA==",
				},
			},
			"required": []string{"owner", "repo", "discussion_number"},
		},
		Metadata: map[string]interface{}{
			"oauth_scopes": []string{"repo", "read:discussion"},
			"rate_limit":   "graphql",
			"api_version":  "GraphQL",
			"points_cost":  1,
			"graphql_only": true,
		},
		ResponseExample: map[string]interface{}{
			"comments": []map[string]interface{}{
				{
					"id":           "DC_kwDOABCD5M4AABCD",
					"body":         "Great question! Here's how you can get started...",
					"author":       "maintainer1",
					"created_at":   "2024-01-15T11:00:00Z",
					"updated_at":   "2024-01-15T11:30:00Z",
					"is_answer":    true,
					"is_minimized": false,
					"reactions": map[string]interface{}{
						"thumbs_up": 5,
						"heart":     2,
					},
					"replies_count": 3,
				},
				{
					"id":            "DC_kwDOABCD5M4AABCE",
					"body":          "Thanks for the help!",
					"author":        "newcontributor",
					"created_at":    "2024-01-15T12:00:00Z",
					"is_answer":     false,
					"replies_count": 0,
				},
			},
			"pageInfo": map[string]interface{}{
				"hasNextPage": false,
				"endCursor":   "Y3Vyc29yOnYyOpHOAAAAAA==",
			},
			"totalCount": 5,
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "Discussion not found",
				"cause":    "Discussion doesn't exist",
				"solution": "Verify discussion number",
			},
			{
				"error":    "Discussions not enabled",
				"cause":    "Repository has discussions disabled",
				"solution": "Enable discussions in repository settings",
			},
		},
		ExtendedHelp: `The discussion_comments_get operation retrieves all comments for a specific discussion.

Comment types:
- Regular comments: Standard replies
- Answers: Marked solutions (Q&A categories only)
- Minimized: Hidden due to abuse/spam
- Replies: Nested responses to comments

Comment features:
- Rich text with Markdown
- Reactions (emoji responses)
- Threading (replies to comments)
- Answer marking (Q&A only)

Pagination:
- Returns 30 comments per page
- Use 'after' cursor for next page
- Comments ordered chronologically

Examples:

# Get first page of comments
{
  "owner": "facebook",
  "repo": "react",
  "discussion_number": 42
}

# Get next page
{
  "owner": "facebook",
  "repo": "react",
  "discussion_number": 42,
  "after": "Y3Vyc29yOnYyOpHOAAAAAA=="
}

Use for:
- Building discussion threads
- Finding accepted answers
- Analyzing community engagement
- Moderating discussions`,
	}
}

// ListDiscussionCategoriesHandler lists discussion categories
type ListDiscussionCategoriesHandler struct {
	provider *GitHubProvider
}

// NewListDiscussionCategoriesHandler creates a new handler
func NewListDiscussionCategoriesHandler(p *GitHubProvider) *ListDiscussionCategoriesHandler {
	return &ListDiscussionCategoriesHandler{provider: p}
}

// Execute lists discussion categories
func (h *ListDiscussionCategoriesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	owner := extractString(params, "owner")
	repo := extractString(params, "repo")

	if owner == "" || repo == "" {
		return ErrorResult("owner and repo are required"), nil
	}

	client, err := h.provider.getGraphQLClient(ctx)
	if err != nil {
		return ErrorResult("Failed to get GraphQL client: %v", err), nil
	}

	// GraphQL query for discussion categories
	var query struct {
		Repository struct {
			DiscussionCategories struct {
				Nodes []struct {
					ID           string
					Name         string
					Description  string
					Emoji        string
					CreatedAt    githubv4.DateTime
					UpdatedAt    githubv4.DateTime
					IsAnswerable bool
				}
				TotalCount int
			} `graphql:"discussionCategories(first: 100)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	variables := map[string]interface{}{
		"owner": githubv4.String(owner),
		"repo":  githubv4.String(repo),
	}

	err = client.Query(ctx, &query, variables)
	if err != nil {
		return ErrorResult("Failed to list discussion categories: %v", err), nil
	}

	// Format categories
	categories := make([]map[string]interface{}, 0, len(query.Repository.DiscussionCategories.Nodes))
	for _, cat := range query.Repository.DiscussionCategories.Nodes {
		categories = append(categories, map[string]interface{}{
			"id":            cat.ID,
			"name":          cat.Name,
			"description":   cat.Description,
			"emoji":         cat.Emoji,
			"created_at":    cat.CreatedAt.Time,
			"updated_at":    cat.UpdatedAt.Time,
			"is_answerable": cat.IsAnswerable,
		})
	}

	result := map[string]interface{}{
		"categories": categories,
		"totalCount": query.Repository.DiscussionCategories.TotalCount,
	}

	return SuccessResult(result), nil
}

// GetDefinition returns the tool definition
func (h *ListDiscussionCategoriesHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "discussion_categories_list",
		Description: GetOperationDescription("discussion_categories_list"),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner (username or organization)",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
					"example":     "Hello-World",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
			},
			"required": []string{"owner", "repo"},
		},
		Metadata: map[string]interface{}{
			"oauth_scopes": []string{"repo", "read:discussion"},
			"rate_limit":   "graphql",
			"api_version":  "GraphQL",
			"points_cost":  1,
			"graphql_only": true,
		},
		ResponseExample: map[string]interface{}{
			"categories": []map[string]interface{}{
				{
					"id":            "DIC_kwDOABCD5M4CABCD",
					"name":          "Q&A",
					"description":   "Ask the community for help",
					"emoji":         "🙏",
					"created_at":    "2023-01-01T00:00:00Z",
					"updated_at":    "2023-01-01T00:00:00Z",
					"is_answerable": true,
				},
				{
					"id":            "DIC_kwDOABCD5M4CABCE",
					"name":          "General",
					"description":   "General discussions",
					"emoji":         "💬",
					"is_answerable": false,
				},
				{
					"id":            "DIC_kwDOABCD5M4CABCF",
					"name":          "Ideas",
					"description":   "Share ideas for new features",
					"emoji":         "💡",
					"is_answerable": false,
				},
				{
					"id":            "DIC_kwDOABCD5M4CABCG",
					"name":          "Show and tell",
					"description":   "Show off something you've made",
					"emoji":         "🎆",
					"is_answerable": false,
				},
			},
			"totalCount": 5,
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "Discussions not enabled",
				"cause":    "Repository has discussions disabled",
				"solution": "Enable discussions in repository settings",
			},
			{
				"error":    "Repository not found",
				"cause":    "Repository doesn't exist or you lack access",
				"solution": "Verify repository name and permissions",
			},
		},
		ExtendedHelp: `The discussion_categories_list operation retrieves all discussion categories for a repository.

Default categories:
- 💬 General: Open discussions
- 💡 Ideas: Feature requests
- 🙏 Q&A: Questions with answers
- 🎆 Show and tell: Showcase projects
- 📣 Announcements: Updates (usually locked)

Category properties:
- is_answerable: Whether answers can be marked (Q&A)
- emoji: Visual identifier
- description: Purpose of category

Custom categories:
- Can be created by maintainers
- Custom emojis supported
- Can be answerable or discussion-only

Example:
{
  "owner": "facebook",
  "repo": "react"
}

Use categories to:
- Filter discussions by type
- Organize community content
- Set expectations for interaction
- Create custom workflows (e.g., RFCs, Support)`,
	}
}
