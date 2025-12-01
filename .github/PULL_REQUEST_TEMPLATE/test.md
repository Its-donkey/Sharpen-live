# Test PR

## Branch & Naming Verification
- [ ] Branch follows pattern: `test/<section>/<kebab-feature>`
- [ ] All commits follow style: `test (<section>): <message>`
- [ ] Changes pushed to GitHub after each discrete change
- [ ] CHANGELOG.md updated (only if adding significant test infrastructure)

**Section**: <!-- e.g., logging, server, ui, admin, api, youtube, config -->

## Test Coverage Goal
<!-- What is being tested? -->

## Motivation
<!-- Why are these tests needed? -->
- [ ] Improving coverage for existing code
- [ ] Adding regression tests
- [ ] Testing previously untested paths
- [ ] Adding integration tests
- [ ] Adding end-to-end tests
- [ ] Other: ___________

## Tests Added
<!-- List the tests added -->
-
-
-

## Coverage Impact
<!-- Before/after coverage metrics if available -->
**Before**: <!-- e.g., 65% -->
**After**: <!-- e.g., 78% -->

## CHANGELOG.md Updates
<!-- Only needed if adding significant test infrastructure -->
- [ ] Significant test infrastructure: Entry added to `## Unreleased > ### Added`
- [ ] Standard test additions: CHANGELOG.md not required
- [ ] Entry format (if applicable): `Section: description of test infrastructure`

## Test Execution
- [ ] All tests pass: `go test ./...`
- [ ] Tests run in CI/CD pipeline
- [ ] No flaky tests introduced

## Test Types
<!-- Check all that apply -->
- [ ] Unit tests
- [ ] Integration tests
- [ ] End-to-end tests
- [ ] Performance tests
- [ ] Load tests

## Labels
`test` `quality` `<section-name>`

## Notes
<!-- Any additional context or testing patterns used -->
