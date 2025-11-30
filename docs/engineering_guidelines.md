// file name — /docs/engineering_guidelines.md
# Go Engineering Guidelines

This project follows these Go-specific guidelines:

1.## File Organization

1. Each file has a single, focused responsibility.
2. Everything in the file must belong to the package's purpose.
3. File names must reflect the file's purpose.
4. Keep `main` files minimal—only wire dependencies and start the program.
5. All exported symbols must include proper GoDoc comments.
6. Split files when they grow too large or mix concerns.

## Code Quality

7. Use consistent error-handling patterns.
8. Avoid side effects or heavy work during import; keep `init()` minimal.
9. Import only what the file actually uses.
10. Include tests for files containing meaningful logic.
11. Use meaningful zero values—design types so their zero value is safe and usable when possible.
12. Prefer explicit over implicit—don't rely on type inference when it harms readability; make important types explicit.

## Concurrency

13. Encapsulate mutable state in structs or interfaces; avoid package-level mutable globals.
14. Tie every goroutine to a context or lifecycle for safe cancellation.
15. Don't spawn goroutines inside libraries unless explicitly required by the caller. If a library must spawn goroutines, provide `Start()` and `Stop()` methods and document the lifecycle clearly.
16. Protect shared data with proper synchronization; avoid race-prone patterns.
17. Prefer channels for signaling and ownership transfer, not storage. For simple state protection, use mutexes. Don't overuse channels.
18. Never store context in structs; always pass context as the first parameter to functions.
19. Don't pass `nil` context; use `context.TODO()` if you're unsure which context to use.

## Testing

20. Inject clocks, randomness, and other time-dependent behavior for deterministic tests.
21. Use interfaces for dependencies to enable mocking in tests.
22. Test behavior, not implementation details.
23. Prefer table-driven tests.
24. Separate test utilities—extract test helpers to `testutil/` packages or `*_test.go` files; don't export test-only functions from main packages.

## Observability

25. Centralize logging and error formatting for consistency.
26. Wrap errors with context using `%w` for error chains.
27. Avoid logging inside libraries; return errors upward instead.
28. Prefer structured logs (fields instead of string building).
29. Don't leak internal error details to API clients; sanitize outputs.
30. Define sentinel errors for common conditions: `var ErrNotFound = errors.New("not found")`.

## Architecture

31. Keep business logic out of handlers—delegate quickly to services.
32. Separate configuration, transport, and core logic into different packages.
33. Prefer small, composable interfaces over large ones. Remember: "The bigger the interface, the weaker the abstraction."
34. Avoid circular dependencies; refactor when needed.
35. Design for graceful degradation—services should degrade gracefully when optional dependencies fail; don't crash on non-critical errors.
36. Validate inputs at boundaries—validate once at API/handler layer; internal functions can assume valid inputs (document this assumption).

## Documentation

37. Document public APIs and internal architecture decisions.
38. Keep build tags organized and documented if using platform-specific files.
39. Document performance characteristics—note O(n) operations in comments; document when functions allocate heavily.

## Package Design

40. Use singular nouns for package names (`user`, not `users`).
41. Avoid package name stuttering (`http.HTTPServer` → `http.Server`).
42. No underscores or mixed caps in package names.
43. Package names should be short, clear, and lowercase.

## Tooling & CI

44. Enforce `gofmt`, linting, and `go vet` in CI.
45. Run `go test ./...` in CI before merging.
46. Keep dependencies tidy and pinned (`go mod tidy`; vendor if needed).
47. Use `go generate` responsibly and document generators.

## Examples

### Good Handler Pattern (#31)

```go
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, 400, "invalid request")
        return
    }

    user, err := h.service.CreateUser(r.Context(), req)
    if err != nil {
        respondError(w, 500, "failed to create user")
        return
    }

    respondJSON(w, 201, user)
}
```

### Good Zero Value Design (#11)

```go
// Good: zero value is usable
type Config struct {
    Timeout time.Duration // 0 = no timeout (reasonable default)
    MaxRetries int        // 0 = no retries (reasonable default)
}

// Bad: zero value is invalid
type Config struct {
    Timeout time.Duration // 0 would cause issues, must be set
}
```

### Good Error Sentinel (#30)

```go
var (
    ErrNotFound = errors.New("not found")
    ErrInvalid  = errors.New("invalid input")
    ErrConflict = errors.New("resource conflict")
)

// Usage
if errors.Is(err, ErrNotFound) {
    return nil, ErrNotFound
}
```

### Good Context Usage (#18, #19)

```go
// Good: context as first parameter
func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
    // ...
}

// Bad: context in struct
type Service struct {
    ctx context.Context // Never do this
}

// Bad: nil context
result, err := service.CreateUser(nil, req) // Use context.TODO() instead
```

### Good Table-Driven Test (#23)

```go
func TestValidateEmail(t *testing.T) {
    tests := []struct {
        name    string
        email   string
        wantErr bool
    }{
        {"valid email", "user@example.com", false},
        {"missing @", "userexample.com", true},
        {"empty", "", true},
        {"spaces", "user @example.com", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateEmail(tt.email)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateEmail() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Good Interface Size (#33)

```go
// Good: small, focused interfaces
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}

// Compose when needed
type ReadWriter interface {
    Reader
    Writer
}

// Bad: large interface
type DataStore interface {
    Read() error
    Write() error
    Delete() error
    Update() error
    List() error
    Search() error
    // ... 10 more methods
}
```

### Good Channel Usage (#17)

```go
// Good: channel for signaling
done := make(chan struct{})
go func() {
    defer close(done)
    // work...
}()
<-done

// Good: mutex for simple state
type Counter struct {
    mu    sync.Mutex
    value int
}

func (c *Counter) Inc() {
    c.mu.Lock()
    c.value++
    c.mu.Unlock()
}

// Bad: overusing channels for simple state
type Counter struct {
    ch chan int
}
```

### Good Library Lifecycle (#15)

```go
// Good: explicit lifecycle management
type Worker struct {
    done chan struct{}
    wg   sync.WaitGroup
}

func (w *Worker) Start(ctx context.Context) {
    w.done = make(chan struct{})
    w.wg.Add(1)
    go w.run(ctx)
}

func (w *Worker) Stop() {
    close(w.done)
    w.wg.Wait()
}

func (w *Worker) run(ctx context.Context) {
    defer w.wg.Done()
    for {
        select {
        case <-ctx.Done():
            return
        case <-w.done:
            return
        // ... work
        }
    }
}
```

### Good Input Validation (#36)

```go
// Validate at boundary
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, 400, "invalid request")
        return
    }

    // Validate here
    if err := req.Validate(); err != nil {
        respondError(w, 422, err.Error())
        return
    }

    // Internal functions can assume valid input
    user, err := h.service.createUserInternal(r.Context(), req)
    // ...
}

// Internal function assumes valid input (document this)
// createUserInternal creates a user. Assumes req is already validated.
func (s *Service) createUserInternal(ctx context.Context, req CreateUserRequest) (*User, error) {
    // No validation here - trust the input
    // ...
}
```