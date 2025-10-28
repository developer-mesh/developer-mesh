# Harness MCP Tool Optimization - Implementation Plan

## Phase 1: Remove Platform Admin Tools (IMMEDIATE)

### Tools to Remove (88 tools)

```yaml
# User/Team Administration (13 tools)
- mcp__devmesh__harness_users_list
- mcp__devmesh__harness_users_get
- mcp__devmesh__harness_users_create
- mcp__devmesh__harness_users_update
- mcp__devmesh__harness_users_delete
- mcp__devmesh__harness_usergroups_list
- mcp__devmesh__harness_usergroups_get
- mcp__devmesh__harness_usergroups_create
- mcp__devmesh__harness_usergroups_update
- mcp__devmesh__harness_usergroups_delete
- mcp__devmesh__harness_role_assignments_create
- mcp__devmesh__harness_role_assignments_delete

# RBAC & Governance (15 tools)
- mcp__devmesh__harness_roles_list
- mcp__devmesh__harness_roles_get
- mcp__devmesh__harness_roles_create
- mcp__devmesh__harness_roles_update
- mcp__devmesh__harness_roles_delete
- mcp__devmesh__harness_rbac_policies_list
- mcp__devmesh__harness_rbac_policies_get
- mcp__devmesh__harness_rbac_policies_create
- mcp__devmesh__harness_rbac_policies_update
- mcp__devmesh__harness_rbac_policies_delete
- mcp__devmesh__harness_rbac_policies_evaluate
- mcp__devmesh__harness_governance_policies_list
- mcp__devmesh__harness_governance_policies_get
- mcp__devmesh__harness_governance_policies_create
- mcp__devmesh__harness_governance_policies_update
- mcp__devmesh__harness_governance_policies_delete
- mcp__devmesh__harness_governance_policies_evaluate
- mcp__devmesh__harness_permissions_list
- mcp__devmesh__harness_resourcegroups_list
- mcp__devmesh__harness_resourcegroups_get
- mcp__devmesh__harness_resourcegroups_create
- mcp__devmesh__harness_resourcegroups_update
- mcp__devmesh__harness_resourcegroups_delete

# Organization/Platform Config (20 tools)
- mcp__devmesh__harness_account_get
- mcp__devmesh__harness_account_update
- mcp__devmesh__harness_account_usage
- mcp__devmesh__harness_account_preferences_get
- mcp__devmesh__harness_account_preferences_update
- mcp__devmesh__harness_orgs_list
- mcp__devmesh__harness_orgs_get
- mcp__devmesh__harness_projects_list
- mcp__devmesh__harness_projects_get
- mcp__devmesh__harness_projects_create
- mcp__devmesh__harness_projects_update
- mcp__devmesh__harness_projects_delete
- mcp__devmesh__harness_delegates_list
- mcp__devmesh__harness_delegates_get
- mcp__devmesh__harness_delegates_create
- mcp__devmesh__harness_delegates_delete
- mcp__devmesh__harness_delegates_status
- mcp__devmesh__harness_delegates_heartbeat
- mcp__devmesh__harness_delegate_profiles_list
- mcp__devmesh__harness_delegate_profiles_get
- mcp__devmesh__harness_delegate_profiles_create
- mcp__devmesh__harness_delegate_profiles_update
- mcp__devmesh__harness_delegate_profiles_delete
- mcp__devmesh__harness_apikeys_list
- mcp__devmesh__harness_apikeys_get
- mcp__devmesh__harness_apikeys_create
- mcp__devmesh__harness_apikeys_update
- mcp__devmesh__harness_apikeys_delete
- mcp__devmesh__harness_apikeys_rotate

# Cloud Cost Management (20 tools)
- mcp__devmesh__harness_ccm_perspectives_list
- mcp__devmesh__harness_ccm_perspectives_get
- mcp__devmesh__harness_ccm_perspectives_create
- mcp__devmesh__harness_ccm_perspectives_update
- mcp__devmesh__harness_ccm_perspectives_delete
- mcp__devmesh__harness_ccm_budgets_list
- mcp__devmesh__harness_ccm_budgets_create
- mcp__devmesh__harness_ccm_budgets_update
- mcp__devmesh__harness_ccm_budgets_delete
- mcp__devmesh__harness_ccm_forecasts_get
- mcp__devmesh__harness_ccm_autostopping_list
- mcp__devmesh__harness_ccm_autostopping_get
- mcp__devmesh__harness_ccm_autostopping_create
- mcp__devmesh__harness_ccm_autostopping_update
- mcp__devmesh__harness_ccm_autostopping_delete
- mcp__devmesh__harness_ccm_anomalies_list
- mcp__devmesh__harness_ccm_recommendations_list
- mcp__devmesh__harness_ccm_costs_overview
- mcp__devmesh__harness_ccm_categories_list
- mcp__devmesh__harness_ccm_categories_create

# Audit/Compliance (10 tools)
- mcp__devmesh__harness_audit_events_list
- mcp__devmesh__harness_audit_events_get
- mcp__devmesh__harness_licenses_list
- mcp__devmesh__harness_licenses_get
- mcp__devmesh__harness_licenses_usage
- mcp__devmesh__harness_licenses_summary
- mcp__devmesh__harness_idp_entities_list
- mcp__devmesh__harness_idp_entities_get
- mcp__devmesh__harness_idp_entities_create
- mcp__devmesh__harness_idp_entities_update
- mcp__devmesh__harness_idp_entities_delete
- mcp__devmesh__harness_idp_scorecards_list
- mcp__devmesh__harness_idp_scorecards_get
- mcp__devmesh__harness_idp_scorecards_create
- mcp__devmesh__harness_idp_scorecards_update
- mcp__devmesh__harness_idp_scorecards_delete
- mcp__devmesh__harness_idp_catalog_list

# Low-Level Infrastructure (10 tools)
- mcp__devmesh__harness_chaos_hubs_list
- mcp__devmesh__harness_chaos_infrastructure_list
- mcp__devmesh__harness_gitops_agents_list
- mcp__devmesh__harness_database_schema_list
- mcp__devmesh__harness_database_schema_get
- mcp__devmesh__harness_database_migrations_list
- mcp__devmesh__harness_registries_list
- mcp__devmesh__harness_registries_get
- mcp__devmesh__harness_registries_artifacts
- mcp__devmesh__harness_registries_artifacts_versions
- mcp__devmesh__harness_registries_artifacts_files
```

---

## Phase 2: Remove CRUD Operations (25 tools)

### Tools to Remove - Keep Only GET/LIST

```yaml
# Pipeline Management
- mcp__devmesh__harness_pipelines_create
- mcp__devmesh__harness_pipelines_update
- mcp__devmesh__harness_pipelines_delete
- mcp__devmesh__harness_pipelines_validate

# Service Management
- mcp__devmesh__harness_services_create
- mcp__devmesh__harness_services_update
- mcp__devmesh__harness_services_delete

# Environment Management
- mcp__devmesh__harness_environments_create
- mcp__devmesh__harness_environments_update
- mcp__devmesh__harness_environments_delete
- mcp__devmesh__harness_environments_move_configs

# Manifest Management
- mcp__devmesh__harness_manifests_create
- mcp__devmesh__harness_manifests_update
- mcp__devmesh__harness_manifests_delete

# Infrastructure Management
- mcp__devmesh__harness_infrastructures_create
- mcp__devmesh__harness_infrastructures_update
- mcp__devmesh__harness_infrastructures_delete
- mcp__devmesh__harness_infrastructures_move_configs

# Connector Management
- mcp__devmesh__harness_connectors_create
- mcp__devmesh__harness_connectors_update
- mcp__devmesh__harness_connectors_delete
- mcp__devmesh__harness_connectors_catalogue

# Secret Management
- mcp__devmesh__harness_secrets_create
- mcp__devmesh__harness_secrets_update
- mcp__devmesh__harness_secrets_delete

# Variable Management
- mcp__devmesh__harness_variables_create
- mcp__devmesh__harness_variables_update
- mcp__devmesh__harness_variables_delete

# Feature Flag Management
- mcp__devmesh__harness_featureflags_create
- mcp__devmesh__harness_featureflags_update
- mcp__devmesh__harness_featureflags_delete
- mcp__devmesh__harness_featureflags_segments_create

# Chaos Management
- mcp__devmesh__harness_chaos_experiments_create
- mcp__devmesh__harness_chaos_experiments_update
- mcp__devmesh__harness_chaos_experiments_delete
- mcp__devmesh__harness_chaos_experiments_stop

# Other
- mcp__devmesh__harness_notifications_create
- mcp__devmesh__harness_notifications_update
- mcp__devmesh__harness_notifications_delete
- mcp__devmesh__harness_notifications_test
- mcp__devmesh__harness_notifications_list
- mcp__devmesh__harness_notifications_get
- mcp__devmesh__harness_webhooks_create
- mcp__devmesh__harness_webhooks_update
- mcp__devmesh__harness_webhooks_delete
- mcp__devmesh__harness_webhooks_trigger
- mcp__devmesh__harness_webhooks_list
- mcp__devmesh__harness_webhooks_get
- mcp__devmesh__harness_freezewindows_list
- mcp__devmesh__harness_freezewindows_get
- mcp__devmesh__harness_freezewindows_create
- mcp__devmesh__harness_freezewindows_update
- mcp__devmesh__harness_freezewindows_delete
- mcp__devmesh__harness_freezewindows_toggle
- mcp__devmesh__harness_inputsets_list
- mcp__devmesh__harness_inputsets_get
- mcp__devmesh__harness_inputsets_create
- mcp__devmesh__harness_inputsets_update
- mcp__devmesh__harness_inputsets_delete
- mcp__devmesh__harness_inputsets_merge
- mcp__devmesh__harness_templates_list
- mcp__devmesh__harness_templates_get
- mcp__devmesh__harness_templates_create
- mcp__devmesh__harness_templates_update
- mcp__devmesh__harness_templates_delete
- mcp__devmesh__harness_triggers_list
- mcp__devmesh__harness_triggers_get
- mcp__devmesh__harness_triggers_create
- mcp__devmesh__harness_triggers_update
- mcp__devmesh__harness_triggers_delete
- mcp__devmesh__harness_triggers_execute
- mcp__devmesh__harness_dashboards_list
- mcp__devmesh__harness_dashboards_get
- mcp__devmesh__harness_dashboards_data
- mcp__devmesh__harness_filestore_create
- mcp__devmesh__harness_filestore_update
- mcp__devmesh__harness_filestore_delete
- mcp__devmesh__harness_gitops_applications_create
- mcp__devmesh__harness_gitops_applications_update
- mcp__devmesh__harness_gitops_applications_delete
- mcp__devmesh__harness_pullrequests_create
- mcp__devmesh__harness_featureflags_targets_create
```

---

## Phase 3: KEEP Tools with Optimized Descriptions (45 tools)

### Implementation Format

```go
// File: pkg/adapters/harness/tool_definitions.go

var HarnessToolDefinitions = []ToolDefinition{
    // ========== PIPELINES & EXECUTION ==========
    {
        Name: "mcp__devmesh__harness_pipelines_execute",
        Description: "Execute pipeline with parameters. Use when: deploying code, running CI/CD, triggering automated workflows",
        Category: "pipelines",
    },
    {
        Name: "mcp__devmesh__harness_pipelines_get",
        Description: "Get pipeline config (stages, steps, triggers). Use when: reviewing pipeline structure, debugging workflows, analyzing CI/CD setup",
        Category: "pipelines",
    },
    {
        Name: "mcp__devmesh__harness_pipelines_list",
        Description: "List pipelines (name, status, last run). Use when: finding pipelines, checking deployment history, browsing available workflows",
        Category: "pipelines",
    },
    {
        Name: "mcp__devmesh__harness_executions_status",
        Description: "Get execution status (running/success/failed, duration). Use when: monitoring deployments, checking pipeline progress, debugging failures",
        Category: "executions",
    },
    {
        Name: "mcp__devmesh__harness_executions_get",
        Description: "Get execution details (logs, steps, failures). Use when: debugging failed deployments, analyzing execution flow, reviewing deployment history",
        Category: "executions",
    },
    {
        Name: "mcp__devmesh__harness_executions_list",
        Description: "List recent executions (status, duration, pipeline). Use when: reviewing deployment history, finding past runs, analyzing trends",
        Category: "executions",
    },
    {
        Name: "mcp__devmesh__harness_executions_abort",
        Description: "Cancel running execution. Use when: stopping failed deployments, canceling incorrect runs, emergency rollback",
        Category: "executions",
    },
    {
        Name: "mcp__devmesh__harness_executions_rollback",
        Description: "Rollback to previous version. Use when: reverting failed deployments, emergency recovery, undoing changes",
        Category: "executions",
    },
    {
        Name: "mcp__devmesh__harness_logs_stream",
        Description: "Stream real-time pipeline logs. Use when: monitoring active deployments, debugging live issues, watching execution progress",
        Category: "logs",
    },
    {
        Name: "mcp__devmesh__harness_logs_download",
        Description: "Download complete execution logs. Use when: analyzing past failures, archiving logs, detailed debugging",
        Category: "logs",
    },

    // ========== CODE & REPOSITORY ==========
    {
        Name: "mcp__devmesh__harness_repositories_list",
        Description: "List repos (name, URL, last commit). Use when: finding repositories, browsing code sources, discovering projects",
        Category: "repositories",
    },
    {
        Name: "mcp__devmesh__harness_repositories_get",
        Description: "Get repo details (branches, webhooks, config). Use when: reviewing repo setup, checking integrations, analyzing repository state",
        Category: "repositories",
    },
    {
        Name: "mcp__devmesh__harness_repositories_branches",
        Description: "List branches (name, commit, status). Use when: checking available branches, reviewing branch structure, finding feature branches",
        Category: "repositories",
    },
    {
        Name: "mcp__devmesh__harness_repositories_commits",
        Description: "Get commit history (message, author, changes). Use when: reviewing code changes, tracking commits, analyzing history",
        Category: "repositories",
    },
    {
        Name: "mcp__devmesh__harness_pullrequests_list",
        Description: "List PRs (title, status, author, reviewers). Use when: finding PRs, checking review queue, tracking code reviews",
        Category: "pullrequests",
    },
    {
        Name: "mcp__devmesh__harness_pullrequests_get",
        Description: "Get PR details (changes, comments, checks). Use when: reviewing code, checking PR status, analyzing changes",
        Category: "pullrequests",
    },
    {
        Name: "mcp__devmesh__harness_pullrequests_merge",
        Description: "Merge approved PR. Use when: completing code review, deploying approved changes, integrating features",
        Category: "pullrequests",
    },
    {
        Name: "mcp__devmesh__harness_pullrequests_review",
        Description: "Submit PR review (approve/request changes/comment). Use when: reviewing code, providing feedback, approving changes",
        Category: "pullrequests",
    },
    {
        Name: "mcp__devmesh__harness_pullrequests_activities",
        Description: "Get PR activity timeline (comments, commits, reviews). Use when: tracking PR progress, analyzing review history, understanding changes",
        Category: "pullrequests",
    },
    {
        Name: "mcp__devmesh__harness_pullrequests_checks",
        Description: "Get PR check results (CI status, tests, quality gates). Use when: checking build status, validating tests, reviewing quality metrics",
        Category: "pullrequests",
    },

    // ========== SECURITY & TESTING ==========
    {
        Name: "mcp__devmesh__harness_sto_scans_list",
        Description: "List security scans (findings, severity). Use when: checking security status, reviewing vulnerabilities, analyzing scan results",
        Category: "security",
    },
    {
        Name: "mcp__devmesh__harness_sto_vulnerabilities_list",
        Description: "List vulnerabilities (CVE, severity, fix). Use when: prioritizing security fixes, analyzing threats, reviewing exposure",
        Category: "security",
    },
    {
        Name: "mcp__devmesh__harness_sto_exemptions_create",
        Description: "Exempt vulnerability (with justification). Use when: accepting known risks, documenting exceptions, managing false positives",
        Category: "security",
    },
    {
        Name: "mcp__devmesh__harness_ssca_sbom_generate",
        Description: "Generate software bill of materials. Use when: auditing dependencies, compliance checks, security analysis",
        Category: "security",
    },
    {
        Name: "mcp__devmesh__harness_ssca_sbom_get",
        Description: "Get existing SBOM. Use when: reviewing dependencies, checking component versions, analyzing supply chain",
        Category: "security",
    },
    {
        Name: "mcp__devmesh__harness_ssca_vulnerabilities_list",
        Description: "List supply chain vulnerabilities. Use when: checking dependency risks, analyzing supply chain security, prioritizing updates",
        Category: "security",
    },
    {
        Name: "mcp__devmesh__harness_ssca_artifacts_scan",
        Description: "Scan artifact for vulnerabilities. Use when: validating build artifacts, checking container images, analyzing security posture",
        Category: "security",
    },
    {
        Name: "mcp__devmesh__harness_chaos_experiments_list",
        Description: "List chaos tests (name, status, results). Use when: reviewing resilience tests, analyzing failure scenarios, checking system robustness",
        Category: "testing",
    },
    {
        Name: "mcp__devmesh__harness_chaos_experiments_get",
        Description: "Get chaos experiment details (config, history). Use when: reviewing experiment setup, analyzing past runs, understanding failure patterns",
        Category: "testing",
    },
    {
        Name: "mcp__devmesh__harness_chaos_experiments_run",
        Description: "Run chaos experiment. Use when: testing failure scenarios, validating resilience, conducting reliability tests",
        Category: "testing",
    },
    {
        Name: "mcp__devmesh__harness_chaos_experiments_results",
        Description: "Get experiment results (impact, metrics). Use when: analyzing chaos test outcomes, measuring resilience, reviewing system behavior",
        Category: "testing",
    },

    // ========== INFRASTRUCTURE & DEPLOYMENT ==========
    {
        Name: "mcp__devmesh__harness_services_list",
        Description: "List services (name, type, health). Use when: finding services, checking deployment status, browsing applications",
        Category: "services",
    },
    {
        Name: "mcp__devmesh__harness_services_get",
        Description: "Get service config (manifests, artifacts, vars). Use when: reviewing service setup, debugging deployments, analyzing configuration",
        Category: "services",
    },
    {
        Name: "mcp__devmesh__harness_environments_list",
        Description: "List environments (dev/staging/prod, status). Use when: checking available envs, selecting deployment target, reviewing infrastructure",
        Category: "environments",
    },
    {
        Name: "mcp__devmesh__harness_environments_get",
        Description: "Get environment config (infra, overrides). Use when: reviewing env setup, debugging env issues, analyzing configuration",
        Category: "environments",
    },
    {
        Name: "mcp__devmesh__harness_infrastructures_list",
        Description: "List infrastructure (clusters, VMs, config). Use when: finding infra resources, checking deployment targets, reviewing infrastructure",
        Category: "infrastructure",
    },
    {
        Name: "mcp__devmesh__harness_infrastructures_get",
        Description: "Get infrastructure details (specs, status). Use when: reviewing infra setup, debugging connectivity, analyzing capacity",
        Category: "infrastructure",
    },
    {
        Name: "mcp__devmesh__harness_manifests_list",
        Description: "List manifests (K8s/Helm, version). Use when: finding manifests, checking deployed configs, reviewing definitions",
        Category: "manifests",
    },
    {
        Name: "mcp__devmesh__harness_manifests_get",
        Description: "Get manifest content (YAML, values). Use when: reviewing deployment specs, debugging config issues, analyzing manifests",
        Category: "manifests",
    },
    {
        Name: "mcp__devmesh__harness_gitops_applications_list",
        Description: "List GitOps apps (sync status, health). Use when: checking GitOps deployments, reviewing app state, monitoring sync status",
        Category: "gitops",
    },
    {
        Name: "mcp__devmesh__harness_gitops_applications_get",
        Description: "Get GitOps app details (resources, history). Use when: reviewing app config, debugging sync issues, analyzing deployment state",
        Category: "gitops",
    },
    {
        Name: "mcp__devmesh__harness_gitops_applications_sync",
        Description: "Sync GitOps application. Use when: deploying via GitOps, forcing sync, updating to latest commit",
        Category: "gitops",
    },
    {
        Name: "mcp__devmesh__harness_gitops_applications_rollback",
        Description: "Rollback GitOps app to previous version. Use when: reverting bad deployment, emergency recovery, undoing GitOps changes",
        Category: "gitops",
    },
    {
        Name: "mcp__devmesh__harness_gitops_applications_resources",
        Description: "Get GitOps app resources (pods, services). Use when: checking deployment resources, debugging resource issues, analyzing app state",
        Category: "gitops",
    },
    {
        Name: "mcp__devmesh__harness_iacm_workspaces_list",
        Description: "List IaC workspaces (Terraform, status). Use when: managing infrastructure code, checking IaC state, reviewing workspaces",
        Category: "iac",
    },
    {
        Name: "mcp__devmesh__harness_iacm_workspaces_get",
        Description: "Get IaC workspace details (state, variables). Use when: reviewing workspace config, debugging IaC issues, analyzing terraform state",
        Category: "iac",
    },
    {
        Name: "mcp__devmesh__harness_iacm_workspaces_create",
        Description: "Create IaC workspace. Use when: setting up new infrastructure, initializing terraform workspace, creating IaC environment",
        Category: "iac",
    },
    {
        Name: "mcp__devmesh__harness_iacm_stacks_list",
        Description: "List IaC stacks (resources, status). Use when: reviewing infrastructure stacks, checking resource groups, analyzing IaC deployments",
        Category: "iac",
    },
    {
        Name: "mcp__devmesh__harness_iacm_cost_estimation",
        Description: "Estimate infrastructure costs. Use when: planning deployments, budgeting infrastructure, analyzing cost impact",
        Category: "iac",
    },

    // ========== CONFIGURATION & SECRETS ==========
    {
        Name: "mcp__devmesh__harness_variables_list",
        Description: "List variables (name, value, scope). Use when: finding variables, reviewing configuration, debugging missing vars",
        Category: "config",
    },
    {
        Name: "mcp__devmesh__harness_variables_get",
        Description: "Get variable details (type, scope, usage). Use when: checking variable config, debugging values, analyzing usage",
        Category: "config",
    },
    {
        Name: "mcp__devmesh__harness_secrets_list",
        Description: "List secrets (name, type, last rotated). Use when: finding secrets, reviewing credentials, checking access",
        Category: "config",
    },
    {
        Name: "mcp__devmesh__harness_secrets_get",
        Description: "Get secret metadata (type, scope, rotation). Use when: checking secret config, reviewing access, analyzing usage",
        Category: "config",
    },
    {
        Name: "mcp__devmesh__harness_connectors_list",
        Description: "List connectors (type, status, validation). Use when: finding integrations, checking connectivity, reviewing connectors",
        Category: "config",
    },
    {
        Name: "mcp__devmesh__harness_connectors_get",
        Description: "Get connector details (config, credentials). Use when: reviewing connector setup, debugging connection issues, analyzing configuration",
        Category: "config",
    },
    {
        Name: "mcp__devmesh__harness_connectors_validate",
        Description: "Validate connector credentials. Use when: testing connectivity, debugging connection issues, verifying credentials",
        Category: "config",
    },
    {
        Name: "mcp__devmesh__harness_filestore_list",
        Description: "List files (name, path, size). Use when: finding files, browsing storage, checking uploaded artifacts",
        Category: "config",
    },
    {
        Name: "mcp__devmesh__harness_filestore_get",
        Description: "Get file metadata (size, upload date, hash). Use when: checking file details, verifying uploads, analyzing storage",
        Category: "config",
    },
    {
        Name: "mcp__devmesh__harness_filestore_download",
        Description: "Download file content. Use when: retrieving artifacts, accessing stored files, downloading configs",
        Category: "config",
    },

    // ========== FEATURE MANAGEMENT ==========
    {
        Name: "mcp__devmesh__harness_featureflags_list",
        Description: "List feature flags (name, status, rules). Use when: finding flags, reviewing features, checking rollout status",
        Category: "features",
    },
    {
        Name: "mcp__devmesh__harness_featureflags_get",
        Description: "Get flag details (targeting, variations, metrics). Use when: reviewing flag config, debugging targeting, analyzing usage",
        Category: "features",
    },
    {
        Name: "mcp__devmesh__harness_featureflags_toggle",
        Description: "Enable/disable feature flag. Use when: rolling out features, emergency disable, testing variations",
        Category: "features",
    },
    {
        Name: "mcp__devmesh__harness_featureflags_evaluations",
        Description: "Get flag evaluation results. Use when: debugging targeting, checking user assignments, validating rules",
        Category: "features",
    },
    {
        Name: "mcp__devmesh__harness_featureflags_metrics",
        Description: "Get flag usage metrics. Use when: analyzing feature adoption, reviewing performance impact, measuring rollout",
        Category: "features",
    },
    {
        Name: "mcp__devmesh__harness_featureflags_targets_list",
        Description: "List flag targets (users, segments). Use when: reviewing targeting rules, checking user assignments, analyzing audience",
        Category: "features",
    },
    {
        Name: "mcp__devmesh__harness_featureflags_segments_list",
        Description: "List flag segments (criteria, users). Use when: reviewing audience segments, checking targeting groups, analyzing rollout strategy",
        Category: "features",
    },

    // ========== APPROVALS ==========
    {
        Name: "mcp__devmesh__harness_approvals_list",
        Description: "List pending approvals (pipeline, requester). Use when: checking approval queue, finding pending requests, reviewing workflows",
        Category: "approvals",
    },
    {
        Name: "mcp__devmesh__harness_approvals_get",
        Description: "Get approval details (criteria, approvers). Use when: reviewing approval config, checking requirements, analyzing approval state",
        Category: "approvals",
    },
    {
        Name: "mcp__devmesh__harness_approvals_approve",
        Description: "Approve pending request. Use when: approving deployment, completing approval workflow, greenlighting pipeline",
        Category: "approvals",
    },
    {
        Name: "mcp__devmesh__harness_approvals_reject",
        Description: "Reject pending request. Use when: blocking deployment, failing approval workflow, stopping pipeline",
        Category: "approvals",
    },
    {
        Name: "mcp__devmesh__harness_approvals_history",
        Description: "Get approval history (decisions, timestamps). Use when: reviewing past approvals, auditing decisions, analyzing approval patterns",
        Category: "approvals",
    },
}
```

---

## Implementation Steps

### Step 1: Update MCP Server Configuration
**File**: `apps/mcp-server/internal/config/harness_tools.yaml`

```yaml
# Remove these tool prefixes entirely
excluded_tool_patterns:
  - "harness_users_*"
  - "harness_usergroups_*"
  - "harness_role_assignments_*"
  - "harness_roles_*"
  - "harness_rbac_policies_*"
  - "harness_governance_policies_*"
  - "harness_permissions_*"
  - "harness_resourcegroups_*"
  - "harness_account_*"
  - "harness_orgs_*"
  - "harness_projects_*"
  - "harness_delegates_*"
  - "harness_delegate_profiles_*"
  - "harness_apikeys_*"
  - "harness_ccm_*"
  - "harness_audit_*"
  - "harness_licenses_*"
  - "harness_idp_*"
  - "harness_chaos_hubs_*"
  - "harness_chaos_infrastructure_*"
  - "harness_gitops_agents_*"
  - "harness_database_*"
  - "harness_registries_*"
  - "harness_notifications_*"
  - "harness_webhooks_*"
  - "harness_freezewindows_*"
  - "harness_inputsets_*"
  - "harness_templates_*"
  - "harness_triggers_*"
  - "harness_dashboards_*"

# Remove CRUD operations for these resources
excluded_operations:
  - "*_create"
  - "*_update"
  - "*_delete"
  exceptions:
    # Keep these CRUD operations (necessary for workflow)
    - "pullrequests_merge"
    - "pullrequests_review"
    - "approvals_approve"
    - "approvals_reject"
    - "executions_abort"
    - "executions_rollback"
    - "gitops_applications_sync"
    - "gitops_applications_rollback"
    - "chaos_experiments_run"
    - "featureflags_toggle"
    - "sto_exemptions_create"
    - "ssca_sbom_generate"
    - "ssca_artifacts_scan"
    - "iacm_workspaces_create"
```

### Step 2: Update Tool Descriptions
**File**: `pkg/adapters/harness/tool_registry.go`

```go
func (r *HarnessToolRegistry) GetOptimizedDescription(toolName string) string {
    descriptions := map[string]string{
        // Copy descriptions from Phase 3 above
        "harness_pipelines_execute": "Execute pipeline with parameters. Use when: deploying code, running CI/CD, triggering automated workflows",
        "harness_pipelines_get": "Get pipeline config (stages, steps, triggers). Use when: reviewing pipeline structure, debugging workflows, analyzing CI/CD setup",
        // ... etc
    }

    if desc, ok := descriptions[toolName]; ok {
        return desc
    }
    return ""
}
```

### Step 3: Update MCP Tool Handler
**File**: `apps/mcp-server/internal/handlers/tools_handler.go`

```go
func (h *ToolsHandler) ListTools(ctx context.Context) ([]Tool, error) {
    allTools, err := h.toolRegistry.ListAll(ctx)
    if err != nil {
        return nil, err
    }

    // Filter out excluded tools
    filtered := make([]Tool, 0, len(allTools))
    for _, tool := range allTools {
        if h.shouldIncludeTool(tool.Name) {
            // Update description with optimized version
            if optimized := h.getOptimizedDescription(tool.Name); optimized != "" {
                tool.Description = optimized
            }
            filtered = append(filtered, tool)
        }
    }

    return filtered, nil
}

func (h *ToolsHandler) shouldIncludeTool(toolName string) bool {
    // Check against excluded patterns
    excludedPrefixes := []string{
        "harness_users_",
        "harness_ccm_",
        // ... etc
    }

    for _, prefix := range excludedPrefixes {
        if strings.HasPrefix(toolName, prefix) {
            return false
        }
    }

    // Check excluded operations (with exceptions)
    excludedSuffixes := []string{"_create", "_update", "_delete"}
    exceptions := []string{
        "pullrequests_merge",
        "approvals_approve",
        // ... etc
    }

    for _, suffix := range excludedSuffixes {
        if strings.HasSuffix(toolName, suffix) {
            for _, exception := range exceptions {
                if strings.Contains(toolName, exception) {
                    return true
                }
            }
            return false
        }
    }

    return true
}
```

### Step 4: Testing
**File**: `test/e2e/harness_tools_optimization_test.go`

```go
func TestHarnessToolOptimization(t *testing.T) {
    tests := []struct {
        name          string
        expectedCount int
        excludedTools []string
        includedTools []string
    }{
        {
            name:          "Platform admin tools excluded",
            expectedCount: 45,
            excludedTools: []string{
                "harness_users_list",
                "harness_ccm_perspectives_list",
                "harness_audit_events_list",
            },
            includedTools: []string{
                "harness_pipelines_execute",
                "harness_executions_status",
                "harness_pullrequests_list",
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tools := getHarnessTools()

            assert.Equal(t, tt.expectedCount, len(tools))

            for _, excluded := range tt.excludedTools {
                assert.NotContains(t, tools, excluded)
            }

            for _, included := range tt.includedTools {
                assert.Contains(t, tools, included)
                // Verify optimized description
                tool := findTool(tools, included)
                assert.Contains(t, tool.Description, "Use when:")
            }
        })
    }
}
```

---

## Rollout Plan

### Phase 1: Immediate (Week 1)
- [ ] Remove platform admin tools (88 tools)
- [ ] Update configuration files
- [ ] Deploy to staging
- [ ] Validate with integration tests
- [ ] Monitor Claude Code usage patterns

**Expected Impact**: 50% context reduction

### Phase 2: Quick Wins (Week 2)
- [ ] Remove CRUD operations (25 tools)
- [ ] Update tool descriptions (45 tools)
- [ ] Deploy to staging
- [ ] A/B test with Claude Code
- [ ] Gather feedback on tool selection accuracy

**Expected Impact**: Additional 25% context reduction

### Phase 3: Refinement (Week 3)
- [ ] Consolidate overlapping tools
- [ ] Optimize remaining descriptions
- [ ] Production deployment
- [ ] Monitor tool usage analytics
- [ ] Iterate based on actual usage

**Expected Impact**: Final 2-5% optimization

---

## Success Metrics

### Context Usage
- **Before**: ~31,140 tokens (173 tools × 180 tokens)
- **After**: ~7,200 tokens (45 tools × 160 tokens)
- **Reduction**: 77% (23,940 tokens saved)

### Tool Selection Accuracy
- Measure: % of times Claude Code selects correct tool on first try
- Target: Increase from ~70% to >90%

### Developer Satisfaction
- Measure: Feedback on tool relevance and usefulness
- Target: >80% positive feedback

### Usage Analytics
- Track which tools are actually used
- Remove unused tools after 30 days
- Add missing workflow tools based on feedback

---

## Rollback Plan

If optimization causes issues:
1. Revert to previous tool list configuration
2. Re-enable specific tool categories as needed
3. Adjust descriptions based on feedback
4. Gradual rollout of changes

---

## Next Actions

1. **Review with stakeholders** (Product, Engineering)
2. **Implement Phase 1** configuration changes
3. **Deploy to staging** environment
4. **Test with Claude Code** in real workflows
5. **Monitor metrics** and gather feedback
6. **Iterate** based on results
