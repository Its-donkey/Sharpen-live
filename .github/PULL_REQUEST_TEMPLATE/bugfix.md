# Bugfix PR

## Branch & Naming Verification
- [ ] Branch follows pattern: `bugfix/<section>/<kebab-feature>`
- [ ] All commits follow style: `fix (<section>): <message>`
- [ ] Changes pushed to GitHub after each discrete change
- [ ] CHANGELOG.md updated in **Fixed** section

**Section**: <!-- e.g., logging, server, ui, admin, api, youtube, config, docs -->

## Bug Description
<!-- What was the bug? -->

## Root Cause
<!-- What caused this bug? -->

## Solution
<!-- How was this fixed? -->

## CHANGELOG.md Updates
<!-- Verify entry exists in CHANGELOG.md under Fixed section -->
- [ ] Entry added to `## Unreleased > ### Fixed`
- [ ] Entry format: `Section: description of the fix`

## Test Plan
<!-- How was this tested? -->
- [ ] Bug reproduction confirmed before fix
- [ ] Bug no longer reproduces after fix
- [ ] Existing tests pass: `go test ./...`
- [ ] Regression test added (if applicable)

## Impact
<!-- What areas are affected by this fix? -->

## Labels
`bugfix` `<section-name>`

## Notes
<!-- Any additional context or considerations -->
