package gitlab

import (
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
)

// getExtendedOperationMappings returns all the extended GitLab API operations
// This supplements the basic operations in gitlab_provider.go
func getExtendedOperationMappings() map[string]providers.OperationMapping {
	mappings := make(map[string]providers.OperationMapping)

	// === PROJECT OPERATIONS ===

	// Update project
	mappings["projects/update"] = providers.OperationMapping{
		OperationID:    "updateProject",
		Method:         "PUT",
		PathTemplate:   "/projects/{id}",
		RequiredParams: []string{"id"},
		OptionalParams: []string{"name", "path", "description", "visibility", "default_branch",
			"issues_enabled", "merge_requests_enabled", "wiki_enabled", "snippets_enabled",
			"container_registry_enabled", "shared_runners_enabled", "public_builds"},
	}

	// Delete project
	mappings["projects/delete"] = providers.OperationMapping{
		OperationID:    "deleteProject",
		Method:         "DELETE",
		PathTemplate:   "/projects/{id}",
		RequiredParams: []string{"id"},
		OptionalParams: []string{},
	}

	// Fork project
	mappings["projects/fork"] = providers.OperationMapping{
		OperationID:    "forkProject",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/fork",
		RequiredParams: []string{"id"},
		OptionalParams: []string{"namespace", "path", "name", "description", "visibility"},
	}

	// Star project
	mappings["projects/star"] = providers.OperationMapping{
		OperationID:    "starProject",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/star",
		RequiredParams: []string{"id"},
		OptionalParams: []string{},
	}

	// Unstar project
	mappings["projects/unstar"] = providers.OperationMapping{
		OperationID:    "unstarProject",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/unstar",
		RequiredParams: []string{"id"},
		OptionalParams: []string{},
	}

	// Archive project
	mappings["projects/archive"] = providers.OperationMapping{
		OperationID:    "archiveProject",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/archive",
		RequiredParams: []string{"id"},
		OptionalParams: []string{},
	}

	// Unarchive project
	mappings["projects/unarchive"] = providers.OperationMapping{
		OperationID:    "unarchiveProject",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/unarchive",
		RequiredParams: []string{"id"},
		OptionalParams: []string{},
	}

	// === ISSUE OPERATIONS ===

	// Update issue
	mappings["issues/update"] = providers.OperationMapping{
		OperationID:    "updateIssue",
		Method:         "PUT",
		PathTemplate:   "/projects/{id}/issues/{issue_iid}",
		RequiredParams: []string{"id", "issue_iid"},
		OptionalParams: []string{"title", "description", "state_event", "assignee_ids",
			"milestone_id", "labels", "due_date", "weight", "confidential"},
	}

	// Delete issue
	mappings["issues/delete"] = providers.OperationMapping{
		OperationID:    "deleteIssue",
		Method:         "DELETE",
		PathTemplate:   "/projects/{id}/issues/{issue_iid}",
		RequiredParams: []string{"id", "issue_iid"},
		OptionalParams: []string{},
	}

	// Close issue (convenience operation)
	mappings["issues/close"] = providers.OperationMapping{
		OperationID:    "closeIssue",
		Method:         "PUT",
		PathTemplate:   "/projects/{id}/issues/{issue_iid}",
		RequiredParams: []string{"id", "issue_iid"},
		OptionalParams: []string{},
		// Note: Implementation should add {"state_event": "close"} to body
	}

	// Reopen issue (convenience operation)
	mappings["issues/reopen"] = providers.OperationMapping{
		OperationID:    "reopenIssue",
		Method:         "PUT",
		PathTemplate:   "/projects/{id}/issues/{issue_iid}",
		RequiredParams: []string{"id", "issue_iid"},
		OptionalParams: []string{},
		// Note: Implementation should add {"state_event": "reopen"} to body
	}

	// === MERGE REQUEST OPERATIONS ===

	// Update merge request
	mappings["merge_requests/update"] = providers.OperationMapping{
		OperationID:    "updateMergeRequest",
		Method:         "PUT",
		PathTemplate:   "/projects/{id}/merge_requests/{merge_request_iid}",
		RequiredParams: []string{"id", "merge_request_iid"},
		OptionalParams: []string{"title", "description", "state_event", "assignee_ids",
			"milestone_id", "labels", "target_branch", "squash", "remove_source_branch"},
	}

	// Approve merge request
	mappings["merge_requests/approve"] = providers.OperationMapping{
		OperationID:    "approveMergeRequest",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/merge_requests/{merge_request_iid}/approve",
		RequiredParams: []string{"id", "merge_request_iid"},
		OptionalParams: []string{"approval_password"},
	}

	// Unapprove merge request
	mappings["merge_requests/unapprove"] = providers.OperationMapping{
		OperationID:    "unapproveMergeRequest",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/merge_requests/{merge_request_iid}/unapprove",
		RequiredParams: []string{"id", "merge_request_iid"},
		OptionalParams: []string{},
	}

	// Merge merge request
	mappings["merge_requests/merge"] = providers.OperationMapping{
		OperationID:    "mergeMergeRequest",
		Method:         "PUT",
		PathTemplate:   "/projects/{id}/merge_requests/{merge_request_iid}/merge",
		RequiredParams: []string{"id", "merge_request_iid"},
		OptionalParams: []string{"merge_commit_message", "squash_commit_message",
			"squash", "should_remove_source_branch", "merge_when_pipeline_succeeds", "sha"},
	}

	// Close merge request
	mappings["merge_requests/close"] = providers.OperationMapping{
		OperationID:    "closeMergeRequest",
		Method:         "PUT",
		PathTemplate:   "/projects/{id}/merge_requests/{merge_request_iid}",
		RequiredParams: []string{"id", "merge_request_iid"},
		OptionalParams: []string{},
		// Note: Implementation should add {"state_event": "close"} to body
	}

	// Rebase merge request
	mappings["merge_requests/rebase"] = providers.OperationMapping{
		OperationID:    "rebaseMergeRequest",
		Method:         "PUT",
		PathTemplate:   "/projects/{id}/merge_requests/{merge_request_iid}/rebase",
		RequiredParams: []string{"id", "merge_request_iid"},
		OptionalParams: []string{"skip_ci"},
	}

	// Delete merge request
	mappings["merge_requests/delete"] = providers.OperationMapping{
		OperationID:    "deleteMergeRequest",
		Method:         "DELETE",
		PathTemplate:   "/projects/{id}/merge_requests/{merge_request_iid}",
		RequiredParams: []string{"id", "merge_request_iid"},
		OptionalParams: []string{},
	}

	// === PIPELINE OPERATIONS ===

	// Cancel pipeline
	mappings["pipelines/cancel"] = providers.OperationMapping{
		OperationID:    "cancelPipeline",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/pipelines/{pipeline_id}/cancel",
		RequiredParams: []string{"id", "pipeline_id"},
		OptionalParams: []string{},
	}

	// Retry pipeline
	mappings["pipelines/retry"] = providers.OperationMapping{
		OperationID:    "retryPipeline",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/pipelines/{pipeline_id}/retry",
		RequiredParams: []string{"id", "pipeline_id"},
		OptionalParams: []string{},
	}

	// Delete pipeline
	mappings["pipelines/delete"] = providers.OperationMapping{
		OperationID:    "deletePipeline",
		Method:         "DELETE",
		PathTemplate:   "/projects/{id}/pipelines/{pipeline_id}",
		RequiredParams: []string{"id", "pipeline_id"},
		OptionalParams: []string{},
	}

	// === JOB OPERATIONS ===

	// Get job
	mappings["jobs/get"] = providers.OperationMapping{
		OperationID:    "getJob",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/jobs/{job_id}",
		RequiredParams: []string{"id", "job_id"},
		OptionalParams: []string{},
	}

	// Cancel job
	mappings["jobs/cancel"] = providers.OperationMapping{
		OperationID:    "cancelJob",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/jobs/{job_id}/cancel",
		RequiredParams: []string{"id", "job_id"},
		OptionalParams: []string{},
	}

	// Retry job
	mappings["jobs/retry"] = providers.OperationMapping{
		OperationID:    "retryJob",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/jobs/{job_id}/retry",
		RequiredParams: []string{"id", "job_id"},
		OptionalParams: []string{},
	}

	// Play job (for manual jobs)
	mappings["jobs/play"] = providers.OperationMapping{
		OperationID:    "playJob",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/jobs/{job_id}/play",
		RequiredParams: []string{"id", "job_id"},
		OptionalParams: []string{"job_variables_attributes"},
	}

	// Get job artifacts
	mappings["jobs/artifacts"] = providers.OperationMapping{
		OperationID:    "getJobArtifacts",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/jobs/{job_id}/artifacts",
		RequiredParams: []string{"id", "job_id"},
		OptionalParams: []string{},
	}

	// Erase job
	mappings["jobs/erase"] = providers.OperationMapping{
		OperationID:    "eraseJob",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/jobs/{job_id}/erase",
		RequiredParams: []string{"id", "job_id"},
		OptionalParams: []string{},
	}

	// === REPOSITORY FILE OPERATIONS ===

	// Get file
	mappings["files/get"] = providers.OperationMapping{
		OperationID:    "getFile",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/repository/files/{file_path}",
		RequiredParams: []string{"id", "file_path", "ref"},
		OptionalParams: []string{},
	}

	// Get raw file content
	mappings["files/raw"] = providers.OperationMapping{
		OperationID:    "getRawFile",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/repository/files/{file_path}/raw",
		RequiredParams: []string{"id", "file_path", "ref"},
		OptionalParams: []string{"lfs"},
	}

	// Create file
	mappings["files/create"] = providers.OperationMapping{
		OperationID:    "createFile",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/repository/files/{file_path}",
		RequiredParams: []string{"id", "file_path", "branch", "content", "commit_message"},
		OptionalParams: []string{"start_branch", "author_email", "author_name", "encoding"},
	}

	// Update file
	mappings["files/update"] = providers.OperationMapping{
		OperationID:    "updateFile",
		Method:         "PUT",
		PathTemplate:   "/projects/{id}/repository/files/{file_path}",
		RequiredParams: []string{"id", "file_path", "branch", "content", "commit_message"},
		OptionalParams: []string{"start_branch", "author_email", "author_name",
			"last_commit_id", "encoding"},
	}

	// Delete file
	mappings["files/delete"] = providers.OperationMapping{
		OperationID:    "deleteFile",
		Method:         "DELETE",
		PathTemplate:   "/projects/{id}/repository/files/{file_path}",
		RequiredParams: []string{"id", "file_path", "branch", "commit_message"},
		OptionalParams: []string{"start_branch", "author_email", "author_name"},
	}

	// === BRANCH OPERATIONS ===

	// Get branch
	mappings["branches/get"] = providers.OperationMapping{
		OperationID:    "getBranch",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/repository/branches/{branch}",
		RequiredParams: []string{"id", "branch"},
		OptionalParams: []string{},
	}

	// Create branch
	mappings["branches/create"] = providers.OperationMapping{
		OperationID:    "createBranch",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/repository/branches",
		RequiredParams: []string{"id", "branch", "ref"},
		OptionalParams: []string{},
	}

	// Delete branch
	mappings["branches/delete"] = providers.OperationMapping{
		OperationID:    "deleteBranch",
		Method:         "DELETE",
		PathTemplate:   "/projects/{id}/repository/branches/{branch}",
		RequiredParams: []string{"id", "branch"},
		OptionalParams: []string{},
	}

	// Protect branch
	mappings["branches/protect"] = providers.OperationMapping{
		OperationID:    "protectBranch",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/protected_branches",
		RequiredParams: []string{"id", "name"},
		OptionalParams: []string{"push_access_level", "merge_access_level",
			"unprotect_access_level", "allow_force_push", "code_owner_approval_required"},
	}

	// Unprotect branch
	mappings["branches/unprotect"] = providers.OperationMapping{
		OperationID:    "unprotectBranch",
		Method:         "DELETE",
		PathTemplate:   "/projects/{id}/protected_branches/{name}",
		RequiredParams: []string{"id", "name"},
		OptionalParams: []string{},
	}

	// === TAG OPERATIONS ===

	// Get tag
	mappings["tags/get"] = providers.OperationMapping{
		OperationID:    "getTag",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/repository/tags/{tag_name}",
		RequiredParams: []string{"id", "tag_name"},
		OptionalParams: []string{},
	}

	// Create tag
	mappings["tags/create"] = providers.OperationMapping{
		OperationID:    "createTag",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/repository/tags",
		RequiredParams: []string{"id", "tag_name", "ref"},
		OptionalParams: []string{"message", "release_description"},
	}

	// Delete tag
	mappings["tags/delete"] = providers.OperationMapping{
		OperationID:    "deleteTag",
		Method:         "DELETE",
		PathTemplate:   "/projects/{id}/repository/tags/{tag_name}",
		RequiredParams: []string{"id", "tag_name"},
		OptionalParams: []string{},
	}

	// === COMMIT OPERATIONS ===

	// Get commit
	mappings["commits/get"] = providers.OperationMapping{
		OperationID:    "getCommit",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/repository/commits/{sha}",
		RequiredParams: []string{"id", "sha"},
		OptionalParams: []string{"stats"},
	}

	// Create commit
	mappings["commits/create"] = providers.OperationMapping{
		OperationID:    "createCommit",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/repository/commits",
		RequiredParams: []string{"id", "branch", "commit_message", "actions"},
		OptionalParams: []string{"start_branch", "author_email", "author_name",
			"stats", "force"},
	}

	// Get commit diff
	mappings["commits/diff"] = providers.OperationMapping{
		OperationID:    "getCommitDiff",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/repository/commits/{sha}/diff",
		RequiredParams: []string{"id", "sha"},
		OptionalParams: []string{},
	}

	// Get commit comments
	mappings["commits/comments"] = providers.OperationMapping{
		OperationID:    "getCommitComments",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/repository/commits/{sha}/comments",
		RequiredParams: []string{"id", "sha"},
		OptionalParams: []string{},
	}

	// Create commit comment
	mappings["commits/comment"] = providers.OperationMapping{
		OperationID:    "createCommitComment",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/repository/commits/{sha}/comments",
		RequiredParams: []string{"id", "sha", "note"},
		OptionalParams: []string{"path", "line", "line_type"},
	}

	// === GROUP OPERATIONS ===

	// Create group
	mappings["groups/create"] = providers.OperationMapping{
		OperationID:    "createGroup",
		Method:         "POST",
		PathTemplate:   "/groups",
		RequiredParams: []string{"name", "path"},
		OptionalParams: []string{"description", "visibility", "parent_id",
			"auto_devops_enabled", "emails_disabled"},
	}

	// Update group
	mappings["groups/update"] = providers.OperationMapping{
		OperationID:    "updateGroup",
		Method:         "PUT",
		PathTemplate:   "/groups/{id}",
		RequiredParams: []string{"id"},
		OptionalParams: []string{"name", "path", "description", "visibility",
			"auto_devops_enabled", "emails_disabled"},
	}

	// Delete group
	mappings["groups/delete"] = providers.OperationMapping{
		OperationID:    "deleteGroup",
		Method:         "DELETE",
		PathTemplate:   "/groups/{id}",
		RequiredParams: []string{"id"},
		OptionalParams: []string{},
	}

	// === WIKI OPERATIONS ===

	// List wiki pages
	mappings["wikis/list"] = providers.OperationMapping{
		OperationID:    "listWikiPages",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/wikis",
		RequiredParams: []string{"id"},
		OptionalParams: []string{"with_content"},
	}

	// Get wiki page
	mappings["wikis/get"] = providers.OperationMapping{
		OperationID:    "getWikiPage",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/wikis/{slug}",
		RequiredParams: []string{"id", "slug"},
		OptionalParams: []string{"render_html", "version"},
	}

	// Create wiki page
	mappings["wikis/create"] = providers.OperationMapping{
		OperationID:    "createWikiPage",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/wikis",
		RequiredParams: []string{"id", "title", "content"},
		OptionalParams: []string{"format", "slug"},
	}

	// Update wiki page
	mappings["wikis/update"] = providers.OperationMapping{
		OperationID:    "updateWikiPage",
		Method:         "PUT",
		PathTemplate:   "/projects/{id}/wikis/{slug}",
		RequiredParams: []string{"id", "slug"},
		OptionalParams: []string{"title", "content", "format"},
	}

	// Delete wiki page
	mappings["wikis/delete"] = providers.OperationMapping{
		OperationID:    "deleteWikiPage",
		Method:         "DELETE",
		PathTemplate:   "/projects/{id}/wikis/{slug}",
		RequiredParams: []string{"id", "slug"},
		OptionalParams: []string{},
	}

	// === SNIPPET OPERATIONS ===

	// List project snippets
	mappings["snippets/list"] = providers.OperationMapping{
		OperationID:    "listSnippets",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/snippets",
		RequiredParams: []string{"id"},
		OptionalParams: []string{},
	}

	// Get snippet
	mappings["snippets/get"] = providers.OperationMapping{
		OperationID:    "getSnippet",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/snippets/{snippet_id}",
		RequiredParams: []string{"id", "snippet_id"},
		OptionalParams: []string{},
	}

	// Create snippet
	mappings["snippets/create"] = providers.OperationMapping{
		OperationID:    "createSnippet",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/snippets",
		RequiredParams: []string{"id", "title", "content", "visibility"},
		OptionalParams: []string{"description", "file_name"},
	}

	// Update snippet
	mappings["snippets/update"] = providers.OperationMapping{
		OperationID:    "updateSnippet",
		Method:         "PUT",
		PathTemplate:   "/projects/{id}/snippets/{snippet_id}",
		RequiredParams: []string{"id", "snippet_id"},
		OptionalParams: []string{"title", "content", "visibility", "description", "file_name"},
	}

	// Delete snippet
	mappings["snippets/delete"] = providers.OperationMapping{
		OperationID:    "deleteSnippet",
		Method:         "DELETE",
		PathTemplate:   "/projects/{id}/snippets/{snippet_id}",
		RequiredParams: []string{"id", "snippet_id"},
		OptionalParams: []string{},
	}

	// === DEPLOYMENT OPERATIONS ===

	// List deployments
	mappings["deployments/list"] = providers.OperationMapping{
		OperationID:    "listDeployments",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/deployments",
		RequiredParams: []string{"id"},
		OptionalParams: []string{"environment", "status", "updated_after", "updated_before"},
	}

	// Get deployment
	mappings["deployments/get"] = providers.OperationMapping{
		OperationID:    "getDeployment",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/deployments/{deployment_id}",
		RequiredParams: []string{"id", "deployment_id"},
		OptionalParams: []string{},
	}

	// Create deployment
	mappings["deployments/create"] = providers.OperationMapping{
		OperationID:    "createDeployment",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/deployments",
		RequiredParams: []string{"id", "environment", "sha", "ref"},
		OptionalParams: []string{"tag", "status"},
	}

	// Update deployment
	mappings["deployments/update"] = providers.OperationMapping{
		OperationID:    "updateDeployment",
		Method:         "PUT",
		PathTemplate:   "/projects/{id}/deployments/{deployment_id}",
		RequiredParams: []string{"id", "deployment_id"},
		OptionalParams: []string{"status"},
	}

	// === MEMBER OPERATIONS ===

	// List project members
	mappings["members/list"] = providers.OperationMapping{
		OperationID:    "listProjectMembers",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/members",
		RequiredParams: []string{"id"},
		OptionalParams: []string{"query", "user_ids"},
	}

	// Get project member
	mappings["members/get"] = providers.OperationMapping{
		OperationID:    "getProjectMember",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/members/{user_id}",
		RequiredParams: []string{"id", "user_id"},
		OptionalParams: []string{},
	}

	// Add project member
	mappings["members/add"] = providers.OperationMapping{
		OperationID:    "addProjectMember",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/members",
		RequiredParams: []string{"id", "user_id", "access_level"},
		OptionalParams: []string{"expires_at"},
	}

	// Update project member
	mappings["members/update"] = providers.OperationMapping{
		OperationID:    "updateProjectMember",
		Method:         "PUT",
		PathTemplate:   "/projects/{id}/members/{user_id}",
		RequiredParams: []string{"id", "user_id", "access_level"},
		OptionalParams: []string{"expires_at"},
	}

	// Remove project member
	mappings["members/remove"] = providers.OperationMapping{
		OperationID:    "removeProjectMember",
		Method:         "DELETE",
		PathTemplate:   "/projects/{id}/members/{user_id}",
		RequiredParams: []string{"id", "user_id"},
		OptionalParams: []string{},
	}

	return mappings
}

// mergeOperationMappings combines basic and extended operations
func mergeOperationMappings(basic, extended map[string]providers.OperationMapping) map[string]providers.OperationMapping {
	merged := make(map[string]providers.OperationMapping)

	// Copy basic operations
	for k, v := range basic {
		merged[k] = v
	}

	// Add/override with extended operations
	for k, v := range extended {
		merged[k] = v
	}

	return merged
}
