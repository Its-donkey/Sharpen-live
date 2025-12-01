# Pull Request

## Template Selection
Please use the appropriate PR template for your change type:

- [Feature PR Template](?expand=1&template=feature.md) - For new features and additions
- [Bugfix PR Template](?expand=1&template=bugfix.md) - For bug fixes
- [Hotfix PR Template](?expand=1&template=hotfix.md) - For critical production issues
- [Design PR Template](?expand=1&template=design.md) - For UI/UX changes
- [Refactor PR Template](?expand=1&template=refactor.md) - For code refactoring
- [Test PR Template](?expand=1&template=test.md) - For test additions/improvements
- [Documentation PR Template](?expand=1&template=doc.md) - For documentation changes

---

## Quick Checklist
If you're not using a specific template, ensure you've covered:

### Branch & Commits
- [ ] Branch follows pattern: `<type>/<section>/<kebab-feature>`
  - Types: `feature`, `bugfix`, `hotfix`, `design`, `refactor`, `test`, `doc`
  - Sections: `logging`, `server`, `ui`, `admin`, `api`, `youtube`, `config`, `docs`, etc.
- [ ] Commits follow style: `<type-short> (<section>): <message>`
  - Type-short examples: `feat`, `fix`, `hotfix`, `design`, `refactor`, `test`, `doc`
- [ ] Pushed after each discrete change

### CHANGELOG.md
- [ ] CHANGELOG.md updated in appropriate section:
  - **Added** - for new features
  - **Changed** - for changes in existing functionality
  - **Fixed** - for bug fixes
  - Entry format: `Section: description`

### Code Quality
- [ ] All tests pass: `go test ./...`
- [ ] No unrelated changes included
- [ ] Lint passes (or `--no-verify` justified for unrelated failures)

### PR Management
- [ ] One PR per feature/fix
- [ ] If related to existing open PR: updated that PR instead
- [ ] If previous PR was closed: created new PR
- [ ] If remote PR merged & branch deleted: cleaned up local branch

## Summary
<!-- Brief description of changes -->

## Changes
<!-- List of changes made -->

## Test Plan
<!-- How was this tested? -->

## Labels
<!-- Add appropriate labels: feature, bugfix, hotfix, design, refactor, test, doc, <section-name> -->
