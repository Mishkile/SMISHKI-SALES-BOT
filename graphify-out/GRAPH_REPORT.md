# Graph Report - .  (2026-07-29)

## Corpus Check
- Corpus is ~23,494 words - fits in a single context window. You may not need a graph.

## Summary
- 329 nodes · 676 edges · 17 communities (15 shown, 2 thin omitted)
- Extraction: 98% EXTRACTED · 2% INFERRED · 0% AMBIGUOUS · INFERRED: 16 edges (avg confidence: 0.62)
- Token cost: 124,977 input · 0 output

## Community Hubs (Navigation)
- Core Bot Services
- Post Lifecycle Handlers
- Localization & Architecture Docs
- Post Creation Input & Tests
- Admin, Config & RBAC
- Package Metadata & CI
- Post Data Layer
- TypeScript Config
- Bot Bootstrap & Routing
- Broadcast Users Feature
- Dev Tooling Dependencies
- ESLint Config
- Dependabot & Runtime Dependencies
- User Data Layer
- FAQ Service & Release Docs
- Issue Template Config
- Feature Request Template

## God Nodes (most connected - your core abstractions)
1. `BotController` - 25 edges
2. `Post` - 23 edges
3. `BotConfig` - 22 edges
4. `User` - 22 edges
5. `PostService` - 21 edges
6. `Agents.md Guidance Document` - 18 edges
7. `PostRepository` - 16 edges
8. `AdminService` - 14 edges
9. `InputService` - 14 edges
10. `MyPostsService` - 14 edges

## Surprising Connections (you probably didn't know these)
- `development-tooling Dependency Group` --references--> `@types/node`  [INFERRED]
  .github/dependabot.yml → package.json
- `Moderation Flow Enforcement Rule` --references--> `ModerationService`  [EXTRACTED]
  Agents.md → src/services/moderationService.ts
- `config.json Runtime Configuration` --references--> `BotConfig`  [EXTRACTED]
  Agents.md → src/types/index.ts
- `Version 1.0.0 Release (Out of Alpha)` --references--> `node-telegram-bot-api`  [EXTRACTED]
  docs/CHANGELOG.md → package.json
- `development-tooling Dependency Group` --references--> `@eslint/js`  [EXTRACTED]
  .github/dependabot.yml → package.json

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Dependabot CI Automation Pipeline** — github_dependabot_config, github_workflows_dependabot_auto_merge_workflow, github_workflows_retry_dependabot_ci_workflow, github_workflows_ts_ci_workflow [INFERRED 0.85]
- **FAQ System Implementation** — docs_readme_faq_system, src_services_faqservice_faqservice, src_services_localeservice_localeserviceimpl_getfaqs, src_tests_checklocals [INFERRED 0.85]
- **RBAC Feature Group** — docs_changelog_rbac_system, docs_readme_rbac, src_services_adminservice_adminservice_handlepromote, src_services_adminservice_adminservice_handledemote, src_services_adminservice_adminservice_handleauth, src_services_userservice_userservice_hasauthlevel [INFERRED 0.85]

## Communities (17 total, 2 thin omitted)

### Community 0 - "Core Bot Services"
Cohesion: 0.19
Nodes (17): Agents.md Guidance Document, Service/Repository Layering Rule, config, configPath, userSchema, configPath, WizardStep, config (+9 more)

### Community 1 - "Post Lifecycle Handlers"
Cohesion: 0.10
Nodes (4): ModerationService, MyPostsService, PendingService, PostService

### Community 2 - "Localization & Architecture Docs"
Cohesion: 0.08
Nodes (19): Localization (Agents.md), Moderation Flow Enforcement Rule, Post Lifecycle, Docker Compose Stack, CC BY-NC 4.0 License, JSTS Sale Bot README, Docker Multi-Stage Build Targets, FAQ System (+11 more)

### Community 3 - "Post Creation Input & Tests"
Cohesion: 0.11
Nodes (13): testCases.ts Isolation Rule, InputService, PostData, TEST_CASES, TEST_MEDIA, testCase1_FullPost(), testCase2_NoMedia(), testCase3_OnePhoto() (+5 more)

### Community 4 - "Admin, Config & RBAC"
Cohesion: 0.14
Nodes (11): config.json Runtime Configuration, RBAC System (0.1.6), Role-Based Access Control (RBAC) Feature, AdminService, getForwardSenderId(), CONFIG_SCHEMA, ConfigFieldSpec, parseConfigValue() (+3 more)

### Community 5 - "Package Metadata & CI"
Cohesion: 0.09
Nodes (22): Retry Dependabot CI Workflow, TypeScript CI Workflow, auditLevel, author, bugs, url, description, homepage (+14 more)

### Community 6 - "Post Data Layer"
Cohesion: 0.16
Nodes (6): IMediaItem, IPost, mediaItemSchema, postSchema, PostRepository, Post

### Community 7 - "TypeScript Config"
Cohesion: 0.10
Nodes (20): dist, ES2022, node, node_modules, src/**/*, compilerOptions, declaration, esModuleInterop (+12 more)

### Community 8 - "Bot Bootstrap & Routing"
Cohesion: 0.19
Nodes (6): Adding a New Command Procedure, Session Management, main(), connectDB(), BotController, UserSession

### Community 9 - "Broadcast Users Feature"
Cohesion: 0.14
Nodes (12): formatPostText (removed dead code), MediaService (removed dead code), Version 1.0.0 Release (Out of Alpha), BroadcastFailure, BroadcastReport, BroadcastUsersService, mapSendError(), truncateFailures() (+4 more)

### Community 10 - "Dev Tooling Dependencies"
Cohesion: 0.16
Nodes (18): eslint, @eslint/js, development-tooling Dependency Group, globals, nodemon, devDependencies, eslint, @eslint/js (+10 more)

### Community 11 - "ESLint Config"
Cohesion: 0.13
Nodes (13): env, es2022, node, extends, parser, plugins, rules, @typescript-eslint/no-explicit-any (+5 more)

### Community 12 - "Dependabot & Runtime Dependencies"
Cohesion: 0.20
Nodes (12): dotenv, Dependabot Configuration, database Dependency Group, environment Dependency Group, telegram-stack Dependency Group, Dependabot Auto-Merge Workflow, mongoose, node-telegram-bot-api (+4 more)

### Community 13 - "User Data Layer"
Cohesion: 0.36
Nodes (3): IUser, UserRepository, User

### Community 14 - "FAQ Service & Release Docs"
Cohesion: 0.24
Nodes (7): CHANGELOG, FAQ Module (0.1.1), VERSION.md Versioning Document, Semantic Versioning 2.0.0, Create a Version Badge Workflow, version, FaqService

## Knowledge Gaps
- **69 isolated node(s):** `parser`, `eslint:recommended`, `plugin:@typescript-eslint/recommended`, `node`, `es2022` (+64 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **2 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `BroadcastUsersService` connect `Broadcast Users Feature` to `Core Bot Services`, `Bot Bootstrap & Routing`, `Localization & Architecture Docs`, `Post Creation Input & Tests`?**
  _High betweenness centrality (0.153) - this node is a cross-community bridge._
- **Why does `JSTS Sale Bot README` connect `Localization & Architecture Docs` to `Broadcast Users Feature`, `Admin, Config & RBAC`, `FAQ Service & Release Docs`, `Post Lifecycle Handlers`?**
  _High betweenness centrality (0.148) - this node is a cross-community bridge._
- **Why does `VERSION.md Versioning Document` connect `FAQ Service & Release Docs` to `Localization & Architecture Docs`?**
  _High betweenness centrality (0.135) - this node is a cross-community bridge._
- **What connects `parser`, `eslint:recommended`, `plugin:@typescript-eslint/recommended` to the rest of the system?**
  _69 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Post Lifecycle Handlers` be split into smaller, more focused modules?**
  _Cohesion score 0.10037878787878787 - nodes in this community are weakly interconnected._
- **Should `Localization & Architecture Docs` be split into smaller, more focused modules?**
  _Cohesion score 0.07862903225806452 - nodes in this community are weakly interconnected._
- **Should `Post Creation Input & Tests` be split into smaller, more focused modules?**
  _Cohesion score 0.11264367816091954 - nodes in this community are weakly interconnected._