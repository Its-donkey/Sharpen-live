# Refactor PR

## Branch & Naming Verification
- [ ] Branch follows pattern: `refactor/<section>/<kebab-feature>`
- [ ] All commits follow style: `refactor (<section>): <message>`
- [ ] Changes pushed to GitHub after each discrete change
- [ ] CHANGELOG.md updated in **Changed** section (if user-visible)

**Section**: <!-- e.g., logging, server, ui, admin, api, youtube, config, docs -->

## Refactoring Goal
<!-- What is being refactored and why? -->

## Motivation
<!-- Why is this refactoring needed? -->
- [ ] Code maintainability
- [ ] Performance improvement
- [ ] Reduce complexity
- [ ] Remove duplication
- [ ] Improve testability
- [ ] Other: ___________

## Changes Made
<!-- List the refactoring changes -->
-
-
-

## CHANGELOG.md Updates
<!-- Only needed if user-visible changes -->
- [ ] User-visible changes: Entry added to `## Unreleased > ### Changed`
- [ ] No user-visible changes: CHANGELOG.md not required
- [ ] Entry format (if applicable): `Section: description of change`

## Test Coverage
- [ ] All existing tests pass: `go test ./...`
- [ ] No behavior changes (pure refactor)
- [ ] Test coverage maintained or improved
- [ ] New tests added for refactored code (if applicable)

## Performance Impact
<!-- Any performance implications? -->

## Breaking Changes
- [ ] No breaking changes
- [ ] Breaking changes (list below):

## Migration Path
<!-- If breaking changes, how should users migrate? -->

## Labels
`refactor` `<section-name>`

## Notes
<!-- Any additional context or technical decisions -->
