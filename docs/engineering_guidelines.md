// file name — /docs/engineering_guidelines.md
# Go Engineering Guidelines

This project follows these Go-specific guidelines:

1. Each file has a single, focused responsibility.
2. Everything in the file must belong to the package’s purpose.
3. File names must reflect the file’s purpose.
4. Keep `main` files minimal—only wire dependencies and start the program.
5. All exported symbols must include proper GoDoc comments.
6. Split files when they grow too large or mix concerns.
7. Use consistent error-handling patterns; wrap underlying errors with `%w`.
8. Avoid side effects or heavy work during import; keep `init()` minimal.
9. Import only what the file actually uses.
10. Include tests for files containing meaningful logic.
11. Encapsulate mutable state in structs or interfaces; avoid package-level mutable globals.
12. Tie every goroutine to a context or lifecycle for safe cancellation.
13. Don’t spawn goroutines inside libraries unless explicitly required by the caller.
14. Protect shared data with proper synchronisation; avoid race-prone patterns.
15. Prefer channels for signalling, not storage.
16. Inject clocks, randomness, and other time-dependent behaviour for deterministic tests.
17. Use interfaces for dependencies to enable mocking in tests.
18. Test behaviour, not implementation details; prefer table-driven tests.
19. Centralise logging and error formatting for consistency.
20. Avoid logging inside libraries; return errors upward instead.
21. Prefer structured logs (fields instead of string building).
22. Don’t leak internal error details to API clients; sanitise outputs.
23. Keep business logic out of handlers—delegate quickly to services.
24. Separate configuration, transport, and core logic into different packages.
25. Prefer small, composable interfaces over large ones.
26. Avoid circular dependencies; refactor when needed.
27. Document public APIs and internal architecture decisions.
28. Keep build tags organised and documented.
29. Enforce `gofmt`, linting, and `go vet` in CI.
30. Run `go test ./...` in CI before merging.
31. Keep dependencies tidy and pinned (`go mod tidy`, vendor if needed).
32. Use `go generate` responsibly and document generators.
33. Enforce maximum file and function lengths, except with documented exceptions.
34. Prefer contexts as the first parameter in long-running or I/O-heavy functions.
35. Provide a clear lifecycle interface (`Start`/`Stop`) for background services.
