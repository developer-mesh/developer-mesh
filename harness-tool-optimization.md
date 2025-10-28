# Harness MCP Tool Optimization Analysis

## Executive Summary
- **Current Tool Count**: 150+ Harness tools
- **Recommended KEEP**: 45 tools (70% reduction)
- **Recommended REMOVE**: 105+ tools
- **Context Savings**: ~60-70% reduction in Harness-related context

---

## KEEP - Developer Workflow Tools (45 tools)

### 🚀 Pipelines & Execution (8 tools)
| Tool | Optimized Description | Use When |
|------|----------------------|----------|
| `harness_pipelines_execute` | Execute pipeline with parameters. Use when: deploying code, running CI/CD, triggering automated workflows | Developer initiates deployment |
| `harness_pipelines_get` | Get pipeline config (stages, steps, triggers). Use when: reviewing pipeline structure, debugging workflows, analyzing CI/CD setup | Need pipeline details |
| `harness_pipelines_list` | List pipelines (name, status, last run). Use when: finding pipelines, checking deployment history, browsing available workflows | Discovery/exploration |
| `harness_executions_status` | Get execution status (running/success/failed, duration). Use when: monitoring deployments, checking pipeline progress, debugging failures | Track deployment |
| `harness_executions_get` | Get execution details (logs, steps, failures). Use when: debugging failed deployments, analyzing execution flow, reviewing deployment history | Need execution details |
| `harness_executions_abort` | Cancel running execution. Use when: stopping failed deployments, canceling incorrect runs, emergency rollback | Stop deployment |
| `harness_executions_rollback` | Rollback to previous version. Use when: reverting failed deployments, emergency recovery, undoing changes | Undo deployment |
| `harness_logs_stream` | Stream real-time pipeline logs. Use when: monitoring active deployments, debugging live issues, watching execution progress | Live monitoring |

### 📁 Code & Repository (7 tools)
| Tool | Optimized Description | Use When |
|------|----------------------|----------|
| `harness_repositories_list` | List repos (name, URL, last commit). Use when: finding repositories, browsing code sources, discovering projects | Repo discovery |
| `harness_repositories_get` | Get repo details (branches, webhooks, config). Use when: reviewing repo setup, checking integrations, analyzing repository state | Need repo info |
| `harness_repositories_branches` | List branches (name, commit, status). Use when: checking available branches, reviewing branch structure, finding feature branches | Branch exploration |
| `harness_repositories_commits` | Get commit history (message, author, changes). Use when: reviewing code changes, tracking commits, analyzing history | Code review |
| `harness_pullrequests_list` | List PRs (title, status, author, reviewers). Use when: finding PRs, checking review queue, tracking code reviews | PR discovery |
| `harness_pullrequests_get` | Get PR details (changes, comments, checks). Use when: reviewing code, checking PR status, analyzing changes | PR review |
| `harness_pullrequests_merge` | Merge approved PR. Use when: completing code review, deploying approved changes, integrating features | Merge code |

### 🔒 Security & Testing (6 tools)
| Tool | Optimized Description | Use When |
|------|----------------------|----------|
| `harness_sto_scans_list` | List security scans (findings, severity). Use when: checking security status, reviewing vulnerabilities, analyzing scan results | Security review |
| `harness_sto_vulnerabilities_list` | List vulnerabilities (CVE, severity, fix). Use when: prioritizing security fixes, analyzing threats, reviewing exposure | Vuln analysis |
| `harness_ssca_sbom_generate` | Generate software bill of materials. Use when: auditing dependencies, compliance checks, security analysis | Compliance/audit |
| `harness_ssca_vulnerabilities_list` | List supply chain vulnerabilities. Use when: checking dependency risks, analyzing supply chain security, prioritizing updates | Supply chain security |
| `harness_chaos_experiments_list` | List chaos tests (name, status, results). Use when: reviewing resilience tests, analyzing failure scenarios, checking system robustness | Chaos engineering |
| `harness_chaos_experiments_run` | Run chaos experiment. Use when: testing failure scenarios, validating resilience, conducting reliability tests | Test resilience |

### 🏗️ Infrastructure & Deployment (12 tools)
| Tool | Optimized Description | Use When |
|------|----------------------|----------|
| `harness_services_list` | List services (name, type, health). Use when: finding services, checking deployment status, browsing applications | Service discovery |
| `harness_services_get` | Get service config (manifests, artifacts, vars). Use when: reviewing service setup, debugging deployments, analyzing configuration | Service details |
| `harness_environments_list` | List environments (dev/staging/prod, status). Use when: checking available envs, selecting deployment target, reviewing infrastructure | Env discovery |
| `harness_environments_get` | Get environment config (infra, overrides). Use when: reviewing env setup, debugging env issues, analyzing configuration | Env details |
| `harness_infrastructures_list` | List infrastructure (clusters, VMs, config). Use when: finding infra resources, checking deployment targets, reviewing infrastructure | Infra discovery |
| `harness_infrastructures_get` | Get infrastructure details (specs, status). Use when: reviewing infra setup, debugging connectivity, analyzing capacity | Infra details |
| `harness_manifests_list` | List manifests (K8s/Helm, version). Use when: finding manifests, checking deployed configs, reviewing definitions | Manifest discovery |
| `harness_manifests_get` | Get manifest content (YAML, values). Use when: reviewing deployment specs, debugging config issues, analyzing manifests | Manifest details |
| `harness_gitops_applications_list` | List GitOps apps (sync status, health). Use when: checking GitOps deployments, reviewing app state, monitoring sync status | GitOps monitoring |
| `harness_gitops_applications_sync` | Sync GitOps application. Use when: deploying via GitOps, forcing sync, updating to latest commit | Deploy via GitOps |
| `harness_iacm_workspaces_list` | List IaC workspaces (Terraform, status). Use when: managing infrastructure code, checking IaC state, reviewing workspaces | IaC management |
| `harness_iacm_cost_estimation` | Estimate infrastructure costs. Use when: planning deployments, budgeting infrastructure, analyzing cost impact | Cost planning |

### ⚙️ Configuration & Secrets (7 tools)
| Tool | Optimized Description | Use When |
|------|----------------------|----------|
| `harness_variables_list` | List variables (name, value, scope). Use when: finding variables, reviewing configuration, debugging missing vars | Variable discovery |
| `harness_variables_get` | Get variable details (type, scope, usage). Use when: checking variable config, debugging values, analyzing usage | Variable details |
| `harness_secrets_list` | List secrets (name, type, last rotated). Use when: finding secrets, reviewing credentials, checking access | Secret discovery |
| `harness_secrets_get` | Get secret metadata (type, scope, rotation). Use when: checking secret config, reviewing access, analyzing usage | Secret details |
| `harness_connectors_list` | List connectors (type, status, validation). Use when: finding integrations, checking connectivity, reviewing connectors | Connector discovery |
| `harness_connectors_validate` | Validate connector credentials. Use when: testing connectivity, debugging connection issues, verifying credentials | Test connection |
| `harness_filestore_list` | List files (name, path, size). Use when: finding files, browsing storage, checking uploaded artifacts | File discovery |

### 🎯 Feature Management (5 tools)
| Tool | Optimized Description | Use When |
|------|----------------------|----------|
| `harness_featureflags_list` | List feature flags (name, status, rules). Use when: finding flags, reviewing features, checking rollout status | Flag discovery |
| `harness_featureflags_get` | Get flag details (targeting, variations, metrics). Use when: reviewing flag config, debugging targeting, analyzing usage | Flag details |
| `harness_featureflags_toggle` | Enable/disable feature flag. Use when: rolling out features, emergency disable, testing variations | Control rollout |
| `harness_featureflags_evaluations` | Get flag evaluation results. Use when: debugging targeting, checking user assignments, validating rules | Debug targeting |
| `harness_featureflags_metrics` | Get flag usage metrics. Use when: analyzing feature adoption, reviewing performance impact, measuring rollout | Analyze adoption |

---

## REMOVE - Platform Management Tools (105+ tools)

### ❌ User/Team Administration (13 tools)
- `harness_users_*` (list, get, create, update, delete)
- `harness_usergroups_*` (list, get, create, update, delete)
- `harness_role_assignments_*` (create, delete)

**Rationale**: User management is admin/platform work, not developer workflow. Developers don't create users or manage teams during coding/deployment.

### ❌ Access Control (15 tools)
- `harness_roles_*` (list, get, create, update, delete)
- `harness_rbac_policies_*` (list, get, create, update, delete, evaluate)
- `harness_governance_policies_*` (list, get, create, update, delete, evaluate)
- `harness_permissions_list`
- `harness_resourcegroups_*` (list, get, create, update, delete)

**Rationale**: RBAC and governance policy management is security admin work, not developer workflow.

### ❌ Organization/Platform Config (20 tools)
- `harness_account_*` (get, update, usage, preferences)
- `harness_orgs_*` (list, get)
- `harness_projects_*` (list, get, create, update, delete)
- `harness_delegates_*` (list, get, create, delete, status, heartbeat)
- `harness_delegate_profiles_*` (list, get, create, update, delete)
- `harness_apikeys_*` (list, get, create, update, delete, rotate)

**Rationale**: Platform configuration and delegate management is infrastructure admin work.

### ❌ Cost Management (20+ tools)
- `harness_ccm_perspectives_*` (list, get, create, update, delete)
- `harness_ccm_budgets_*` (list, get, create, update, delete)
- `harness_ccm_forecasts_get`
- `harness_ccm_autostopping_*` (list, get, create, update, delete)
- `harness_ccm_anomalies_list`
- `harness_ccm_recommendations_list`
- `harness_ccm_costs_overview`
- `harness_ccm_categories_*` (list, create)

**Rationale**: Cost management is FinOps/admin work. Developers need cost estimation (kept in iacm_cost_estimation), not budget/perspective management.

### ❌ Audit/Compliance (10 tools)
- `harness_audit_events_*` (list, get)
- `harness_licenses_*` (list, get, usage, summary)
- `harness_idp_entities_*` (list, get, create, update, delete)
- `harness_idp_scorecards_*` (list, get, create, update, delete)
- `harness_idp_catalog_list`

**Rationale**: Audit logs and license management are compliance/admin functions. IDP (Internal Developer Platform) entity management is platform admin, not developer workflow.

### ❌ Low-Level Infrastructure (10 tools)
- `harness_chaos_hubs_list`
- `harness_chaos_infrastructure_list`
- `harness_gitops_agents_list`
- `harness_database_schema_*` (list, get)
- `harness_database_migrations_list`
- `harness_registries_*` (all operations)

**Rationale**: Infrastructure discovery and database schema are platform admin tasks. Developers use higher-level abstractions (services, environments).

### ❌ Redundant/Overlapping (17 tools to consolidate)
- `harness_pipelines_create/update/delete` - Use UI/IaC for pipeline management
- `harness_services_create/update/delete` - Use UI/IaC for service management
- `harness_environments_create/update/delete` - Use UI/IaC for environment management
- `harness_manifests_create/update/delete` - Use UI/IaC for manifest management
- `harness_infrastructures_create/update/delete` - Use UI/IaC for infra management
- `harness_connectors_create/update/delete` - Use UI for connector setup
- `harness_secrets_create/update/delete` - Use UI for secret management
- `harness_variables_create/update/delete` - Use UI/IaC for variable management
- `harness_featureflags_create/update/delete` - Use UI for flag management
- `harness_chaos_experiments_create/update/delete/stop` - Use UI for experiment management
- `harness_notifications_*` (all operations) - Platform admin
- `harness_webhooks_create/update/delete` - Platform admin
- `harness_freezewindows_*` (all operations) - Platform admin
- `harness_inputsets_*` (all operations) - Advanced feature, rarely used
- `harness_templates_*` (all operations) - Use UI/IaC for template management
- `harness_triggers_*` (all operations) - Use UI for trigger setup
- `harness_dashboards_*` (all operations) - Platform admin

**Rationale**: Developers primarily READ configuration (get/list) during coding/debugging. CREATE/UPDATE/DELETE operations are typically done via UI, IaC, or during initial setup, not during active development workflow.

---

## Consolidation Opportunities

### 1. Merge PR Operations
**Current**: `harness_pullrequests_review`, `harness_pullrequests_activities`, `harness_pullrequests_checks`
**Consolidated**: `harness_pullrequests_get` (include review status, activities, checks in response)

### 2. Simplify Execution Monitoring
**Current**: `harness_executions_status`, `harness_executions_get`, `harness_executions_list`
**Consolidated**: `harness_executions_get` (include status in response), keep `list` for discovery

### 3. Merge Service Operations
**Current**: `harness_services_get`, `harness_services_list`
**Keep**: Both (list for discovery, get for details)

### 4. Combine GitOps Operations
**Current**: `harness_gitops_applications_get`, `harness_gitops_applications_resources`
**Consolidated**: `harness_gitops_applications_get` (include resources in response)

---

## Implementation Recommendations

### Phase 1: Remove Platform Admin Tools (Immediate - 60% reduction)
Remove these tool categories entirely:
- All CCM tools (20+)
- All user/team admin tools (13)
- All RBAC/governance tools (15)
- All account/org/project CRUD tools (20)
- All audit/license tools (10)
- All delegate/infrastructure discovery tools (10)

**Expected Savings**: ~88 tools removed, ~40% context reduction

### Phase 2: Remove CRUD Operations (Quick wins - 20% reduction)
Keep only GET/LIST operations for:
- Pipelines, Services, Environments, Manifests, Infrastructures
- Connectors, Secrets, Variables
- Feature Flags, Templates, Triggers

**Expected Savings**: ~25 tools removed, ~15% context reduction

### Phase 3: Consolidate Overlapping Tools (Optimization - 10% reduction)
Merge tools with overlapping functionality:
- PR operations → single `pullrequests_get` with rich response
- Execution monitoring → simplified status checking
- GitOps apps → combined resource view

**Expected Savings**: ~10 tools removed, ~7% context reduction

---

## Final Tool Count Summary

| Category | Current | Recommended | Reduction |
|----------|---------|-------------|-----------|
| Pipelines & Execution | 15 | 8 | -47% |
| Code & Repository | 12 | 7 | -42% |
| Security & Testing | 10 | 6 | -40% |
| Infrastructure | 25 | 12 | -52% |
| Configuration | 15 | 7 | -53% |
| Feature Management | 8 | 5 | -38% |
| **Developer Workflow** | **85** | **45** | **-47%** |
| Platform Admin | 88 | 0 | -100% |
| **Total** | **173** | **45** | **-74%** |

---

## Context Savings Calculation

Assuming each tool consumes:
- Tool name: ~30 tokens
- Description: ~50 tokens
- Parameters schema: ~100 tokens
- **Total per tool**: ~180 tokens

**Current Usage**: 173 tools × 180 tokens = **31,140 tokens**
**Optimized Usage**: 45 tools × 180 tokens = **8,100 tokens**
**Savings**: **23,040 tokens (74% reduction)**

With optimized descriptions (50 → 30 tokens):
**Further Optimized**: 45 tools × 160 tokens = **7,200 tokens**
**Total Savings**: **23,940 tokens (77% reduction)**

---

## Next Steps

1. **Validate with stakeholders**: Confirm developer workflow priorities
2. **Update MCP tool registry**: Remove platform admin tools
3. **Optimize descriptions**: Apply optimized descriptions to KEEP tools
4. **Test with Claude Code**: Verify improved tool selection accuracy
5. **Monitor usage**: Track which tools are actually used by developers
6. **Iterate**: Remove unused tools, add missing workflow tools
