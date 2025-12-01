# Hotfix PR

## Branch & Naming Verification
- [ ] Branch follows pattern: `hotfix/<section>/<kebab-feature>`
- [ ] All commits follow style: `hotfix (<section>): <message>`
- [ ] Changes pushed to GitHub after each discrete change
- [ ] CHANGELOG.md updated in **Fixed** section

**Section**: <!-- e.g., logging, server, ui, admin, api, youtube, config, docs -->

## Critical Issue
<!-- What critical issue required a hotfix? -->

## Severity
<!-- Why was this a hotfix vs regular bugfix? -->
- [ ] Production breaking
- [ ] Security vulnerability
- [ ] Data loss risk
- [ ] Other: ___________

## Solution
<!-- How was this fixed? -->

## CHANGELOG.md Updates
<!-- Verify entry exists in CHANGELOG.md under Fixed section -->
- [ ] Entry added to `## Unreleased > ### Fixed`
- [ ] Entry clearly marked as hotfix/critical
- [ ] Entry format: `Section: description of the critical fix`

## Test Plan
<!-- How was this tested? Keep it focused given urgency -->
- [ ] Issue reproduction confirmed before fix
- [ ] Issue no longer occurs after fix
- [ ] Critical path testing completed
- [ ] Existing tests pass: `go test ./...`

## Rollback Plan
<!-- If this fails, how do we rollback? -->

## Labels
`hotfix` `critical` `<section-name>`

## Notes
<!-- Any additional context or post-deployment monitoring needed -->
