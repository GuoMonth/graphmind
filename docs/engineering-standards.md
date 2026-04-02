# Engineering Standards

Hard rules for development workflow, code quality, and quality gates. All contributors (human and AI) must follow these.

---

## Development Workflow

### Design → Code → Test

Every feature follows this three-phase workflow. No phase may be skipped.

```
1. Design     Write or update docs in docs/.
              Define interfaces, data models, CLI commands.
              Get design reviewed before implementation.

2. Code       Implement against the design.
              Follow go-and-db-conventions.md.
              Run `make check` continuously during development.

3. Test       Write tests after implementation.
              Every public function must have test coverage.
              Use table-driven tests and subtests.
              Run `make test` to verify.
```

### Commit Discipline

- Each commit is **one logical change**. Don't bundle unrelated changes.
- Commit message format: **imperative mood**, 72-char subject line.
- Always include `Co-authored-by` trailer for AI-assisted commits.
- Run `make check` before committing (enforced by git hook).

---

## The `any` Restriction

**Hard rule: `any` is banned by default.**

`any` (alias for `interface{}`) destroys type safety. Do not use it as a function parameter, return type, or variable type unless explicitly justified.

### Allowed Exceptions

Every use of `any` must include a comment explaining why.

| Exception | Example | Justification |
|---|---|---|
| JSON properties | `map[string]any` | SQLite `properties` column stores unstructured JSON |
| `database/sql` scanning | `sql.Scan(&val)` | Driver requires `any` for dynamic column types |
| Generic constraints | `func Foo[T any](...)` | `any` is the semantically correct constraint |
| Third-party interfaces | Cobra/testing helpers | External API contract requires `any` |

### Examples

```go
// BAD — no justification, loses type safety
func process(data any) error { ... }

// BAD — lazy typing
var result any = fetchSomething()

// GOOD — justified, commented
// properties is stored as unstructured JSON in SQLite
type Node struct {
    Properties map[string]any `json:"properties"`
}

// GOOD — generic constraint, any is semantically correct
func Contains[T comparable](slice []T, item T) bool { ... }
```

Every unjustified use of `any` is a code review failure.

---

## Code Quality Tools

### go fmt (mandatory)

- All Go code must be formatted with `go fmt`. No exceptions.
- The pre-commit hook auto-formats and re-stages changed files.

### go vet (mandatory)

- All Go code must pass `go vet` with zero warnings.
- Run automatically on every commit via git hook.

### go fix (periodic)

- Run `go fix ./...` periodically to modernize code to current Go idioms.
- Especially useful after Go version upgrades.

### golangci-lint v2 (mandatory)

- **Version**: v2.9.0+ (required for Go 1.26 support)
- **Config**: `.golangci.yml` at repository root
- **Run**: `golangci-lint run ./...` or `make lint`
- **Policy**: zero tolerance — all warnings must be resolved before merge

The config enables these linter categories:

| Category | Linters | Purpose |
|---|---|---|
| Bug detection | `errcheck`, `govet`, `staticcheck`, `gosimple`, `ineffassign`, `unused` | Catch real bugs |
| Resource safety | `bodyclose`, `noctx`, `sqlclosecheck`, `rowserrcheck` | Prevent resource leaks |
| Code quality | `revive`, `gocritic`, `unconvert`, `unparam`, `nakedret`, `misspell` | Enforce idiomatic Go |
| Complexity control | `gocyclo`, `cyclop`, `funlen`, `lll` | Prevent code rot |
| Type safety | `asasalint`, `durationcheck` | Restrict `...any` abuse |

---

## Quality Gate Thresholds

### Function Complexity

| Metric | Threshold | Linter |
|---|---|---|
| Cyclomatic complexity | ≤ 15 | `gocyclo`, `cyclop` |
| Function length | ≤ 80 lines | `funlen` |
| Function statements | ≤ 50 | `funlen` |
| Line length | ≤ 140 chars | `lll` |

### File Size

| Metric | Target | Hard limit |
|---|---|---|
| Lines per file (excl. tests) | ≤ 300 | ≤ 500 |
| Files per package | ≤ 6 | — |

If a file exceeds 300 lines, split it. If a package exceeds 6 files, consider extracting a sub-package.

### Test Coverage

| Scope | Target |
|---|---|
| Service layer (`graph/`, `tag/`, `proposal/`, `event/`) | ≥ 80% |
| Overall project | ≥ 60% |
| CLI layer | Integration tests (invoke binary, verify JSON output) |

All public functions in service packages must have tests.

---

## Git Hooks

Two quality gates enforced via git hooks:

### pre-commit (fast feedback)

Runs on every `git commit`. Blocks commit on failure.

1. **`go fmt ./...`** — auto-format, re-stage changed files
2. **`go vet ./...`** — static analysis
3. **`golangci-lint run ./...`** — full linting

### pre-push (thorough validation)

Runs on every `git push`. Blocks push on failure.

1. **`go build ./cmd/gm`** — ensure binary compiles
2. **`go test -race -count=1 ./...`** — full test suite with race detector

### Setup

```bash
make setup-hooks
```

This copies hook scripts from `scripts/` to `.git/hooks/` and makes them executable. Run once after cloning.

---

## Makefile Commands

| Command | Description | When to use |
|---|---|---|
| `make build` | Build `gm` binary to `./bin/` | Before manual testing |
| `make test` | Run all tests with race detector | During development |
| `make test-cover` | Run tests + generate coverage report | Before PR |
| `make lint` | Run golangci-lint | During development |
| `make fmt` | Run go fmt | Auto-run by hook |
| `make vet` | Run go vet | Auto-run by hook |
| `make fix` | Run go fix for modernization | After Go upgrades |
| `make check` | Run fmt + vet + lint (pre-commit gate) | Before committing |
| `make validate` | Run build + test (pre-push gate) | Before pushing |
| `make setup-hooks` | Install git hooks | Once after cloning |
| `make clean` | Remove build artifacts | Housekeeping |

---

## Import Organization

Imports must be grouped in this order, separated by blank lines:

1. Standard library
2. Third-party packages
3. Internal packages

```go
import (
    "context"
    "database/sql"
    "fmt"

    "github.com/google/uuid"
    "github.com/spf13/cobra"

    "github.com/anthropic/graphmind/internal/model"
    "github.com/anthropic/graphmind/internal/db"
)
```

`goimports` (enabled in `.golangci.yml` formatters) enforces this automatically.

---

## File Naming

| Type | Convention | Example |
|---|---|---|
| Go source | `snake_case.go` | `node_store.go` |
| Test files | `*_test.go` (same package) | `node_store_test.go` |
| Migration SQL | `NNN_description.sql` | `001_init.sql` |
| Package names | Single lowercase word | `graph`, `model`, `event` |

---

## Dependency Management

- Run `go mod tidy` after any dependency change.
- Review `go.sum` diff in PRs.
- Audit new dependencies for:
  - License compatibility (MIT, BSD, Apache 2.0 preferred)
  - Maintenance status (active commits in last 6 months)
  - Transitive dependency count (fewer is better)
- **Current approved dependencies** (see `go-and-db-conventions.md`):
  - `modernc.org/sqlite` — SQLite driver (pure Go)
  - `github.com/google/uuid` — UUID v7
  - `github.com/spf13/cobra` — CLI framework
