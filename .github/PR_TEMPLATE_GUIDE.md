# Pull Request Template Guide

## Overview
This project uses standardized PR templates to ensure consistency and completeness in pull requests.

## Available Templates

### 1. Feature Template
**When to use**: Adding new functionality or capabilities
- Branch: `feature/<section>/<kebab-feature>`
- Commit: `feat (<section>): <message>`
- CHANGELOG: `Added` section

### 2. Bugfix Template
**When to use**: Fixing non-critical bugs
- Branch: `bugfix/<section>/<kebab-feature>`
- Commit: `fix (<section>): <message>`
- CHANGELOG: `Fixed` section

### 3. Hotfix Template
**When to use**: Critical production issues requiring immediate attention
- Branch: `hotfix/<section>/<kebab-feature>`
- Commit: `hotfix (<section>): <message>`
- CHANGELOG: `Fixed` section (marked as critical)

### 4. Design Template
**When to use**: UI/UX changes and visual improvements
- Branch: `design/<section>/<kebab-feature>`
- Commit: `design (<section>): <message>`
- CHANGELOG: `Added`/`Changed`/`Fixed` (depending on nature)

### 5. Refactor Template
**When to use**: Code restructuring without behavior changes
- Branch: `refactor/<section>/<kebab-feature>`
- Commit: `refactor (<section>): <message>`
- CHANGELOG: `Changed` section (only if user-visible)

### 6. Test Template
**When to use**: Adding or improving tests
- Branch: `test/<section>/<kebab-feature>`
- Commit: `test (<section>): <message>`
- CHANGELOG: Only if adding significant test infrastructure

### 7. Documentation Template
**When to use**: Documentation updates and additions
- Branch: `doc/<section>/<kebab-feature>`
- Commit: `doc (<section>): <message>`
- CHANGELOG: `Added`/`Changed` section

## How to Use Templates

### Method 1: URL Parameter (Recommended)
When creating a PR, add `?template=<template-name>.md` to the URL:
```
https://github.com/owner/repo/compare/main...branch?template=feature.md
```

### Method 2: GitHub UI
1. Click "New Pull Request"
2. GitHub will show template options if you have multiple templates
3. Select the appropriate template

### Method 3: gh CLI
```bash
gh pr create --template feature.md
```

## Common Sections

### Sections
Common sections in this project:
- `logging` - Logging system and utilities
- `server` - Server infrastructure and HTTP handling
- `ui` - User interface and frontend
- `admin` - Admin console and management
- `api` - API endpoints and handlers
- `youtube` - YouTube-specific functionality
- `config` - Configuration management
- `docs` - Documentation

### Branch Naming Examples
```
feature/logging/structured-json-output
bugfix/ui/form-validation-error
hotfix/api/memory-leak
design/admin/dashboard-layout
refactor/server/handler-organization
test/youtube/subscription-coverage
doc/api/endpoint-documentation
```

### Commit Message Examples
```
feat (logging): add structured JSON output format
fix (ui): correct form validation error display
hotfix (api): patch memory leak in subscription handler
design (admin): improve dashboard layout spacing
refactor (server): reorganize HTTP handlers
test (youtube): add subscription flow coverage
doc (api): document all REST endpoints
```

## Workflow Rules

### Push Policy
- Push to GitHub after each discrete, logical change
- Don't batch multiple unrelated changes in one commit
- Ensure each commit is self-contained and buildable

### PR Policy
- **One PR per feature/fix** - Don't combine unrelated changes
- **Update existing PRs** if changes are related and PR is still open
- **Create new PR** if:
  - Previous PR is closed
  - Changes are unrelated to existing open PR
  - Existing PR is merged and branch deleted

### CHANGELOG Policy
- **Always update CHANGELOG.md** before pushing
- Use appropriate section (Added/Changed/Fixed)
- Format: `Section: description of change`
- Be specific and user-focused in descriptions

### Lint Policy
- All changes should pass linting: `go test ./...`
- If **unrelated files** fail lint:
  - Document in PR
  - Use `--no-verify` if needed
  - Don't fix unrelated lint issues in this PR

### Local Branch Cleanup
- If remote PR is merged and branch is deleted:
  ```bash
  git checkout main
  git pull
  git branch -d <local-branch-name>
  ```

## Labels
Apply appropriate labels to all PRs:
- **Type labels**: `feature`, `bugfix`, `hotfix`, `design`, `refactor`, `test`, `documentation`
- **Section labels**: `logging`, `server`, `ui`, `admin`, `api`, `youtube`, `config`, `docs`
- **Priority labels**: `critical` (for hotfixes), `high`, `medium`, `low`
- **Status labels**: `in-progress`, `ready-for-review`, `needs-changes`

## Example PR Creation Workflow

1. **Create branch**:
   ```bash
   git checkout -b feature/logging/rotation-policy
   ```

2. **Make changes and commit**:
   ```bash
   # Make your changes
   # Update CHANGELOG.md
   git add .
   git commit -m "feat (logging): add log rotation policy"
   git push -u origin feature/logging/rotation-policy
   ```

3. **Create PR with template**:
   ```bash
   gh pr create --template feature.md
   ```
   Or visit:
   ```
   https://github.com/owner/repo/compare/main...feature/logging/rotation-policy?template=feature.md
   ```

4. **Fill out template** completely

5. **Add labels**: `feature`, `logging`

6. **Request review**

## Tips
- Read the template carefully before filling it out
- Check all checkboxes that apply
- Provide thorough test plans
- Include screenshots for UI changes
- Link related issues if applicable
- Keep PRs focused and scoped
