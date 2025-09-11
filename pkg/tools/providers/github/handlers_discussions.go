package github

import (
	"context"

	"github.com/shurcooL/githubv4"
)

// Discussion represents a GitHub discussion
type Discussion struct {
	ID            string
	Number        int
	Title         string
	Body          string
	Author        string
	Category      string
	CreatedAt     string
	UpdatedAt     string
	AnswerChosenAt *string
	IsAnswered    bool
	Locked        bool
	Comments      int
	URL           string
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
					ID       string
					Number   int
					Title    string
					Body     string
					Author   struct {
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
			"id":         disc.ID,
			"number":     disc.Number,
			"title":      disc.Title,
			"body":       disc.Body,
			"author":     disc.Author.Login,
			"category":   disc.Category.Name,
			"created_at": disc.CreatedAt.Time,
			"updated_at": disc.UpdatedAt.Time,
			"is_answered": disc.IsAnswered,
			"locked":     disc.Locked,
			"comments":   disc.Comments.TotalCount,
			"url":        disc.URL,
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
		Description: "List discussions in a repository",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"category_id": map[string]interface{}{
					"type":        "string",
					"description": "Filter by discussion category ID",
				},
				"after": map[string]interface{}{
					"type":        "string",
					"description": "Cursor for pagination",
				},
			},
			"required": []string{"owner", "repo"},
		},
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
						ID        string
						Body      string
						Author    struct {
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
				URL           string
				ResourcePath  string
				Upvotes       int `graphql:"upvoteCount"`
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
		"id":       disc.ID,
		"number":   disc.Number,
		"title":    disc.Title,
		"body":     disc.Body,
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
		"created_at":    disc.CreatedAt.Time,
		"updated_at":    disc.UpdatedAt.Time,
		"is_answered":   disc.IsAnswered,
		"locked":        disc.Locked,
		"comments_count": disc.Comments.TotalCount,
		"comments":      comments,
		"labels":        labels,
		"url":           disc.URL,
		"resource_path": disc.ResourcePath,
		"upvotes":       disc.Upvotes,
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
		Description: "Get a specific discussion by number",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"discussion_number": map[string]interface{}{
					"type":        "integer",
					"description": "Discussion number",
				},
			},
			"required": []string{"owner", "repo", "discussion_number"},
		},
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
				ID    string
				Title string
				Comments struct {
					Nodes []struct {
						ID        string
						Body      string
						BodyHTML  string
						Author    struct {
							Login     string
							AvatarUrl string
						}
						CreatedAt  githubv4.DateTime
						UpdatedAt  githubv4.DateTime
						IsAnswer   bool
						Upvotes    int `graphql:"upvoteCount"`
						Replies    struct {
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
		Description: "Get comments for a discussion",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"discussion_number": map[string]interface{}{
					"type":        "integer",
					"description": "Discussion number",
				},
				"after": map[string]interface{}{
					"type":        "string",
					"description": "Cursor for pagination",
				},
			},
			"required": []string{"owner", "repo", "discussion_number"},
		},
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
					ID          string
					Name        string
					Description string
					Emoji       string
					CreatedAt   githubv4.DateTime
					UpdatedAt   githubv4.DateTime
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
			"id":           cat.ID,
			"name":         cat.Name,
			"description":  cat.Description,
			"emoji":        cat.Emoji,
			"created_at":   cat.CreatedAt.Time,
			"updated_at":   cat.UpdatedAt.Time,
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
		Description: "List discussion categories in a repository",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
			},
			"required": []string{"owner", "repo"},
		},
	}
}