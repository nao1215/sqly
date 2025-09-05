# GitHub Copilot Instructions for sqly

## Project Overview
sqly is a command-line SQL query tool for file formats (CSV, TSV, LTSV, Excel) using SQLite3 in-memory database. Built with Clean Architecture and filesql integration.

## Architecture Guidelines

### Clean Architecture Layers
- `domain/`: Business models and repository interfaces
- `interactor/`: Use cases and application logic  
- `infrastructure/`: External dependencies (filesql, SQLite3, file system)
- `shell/`: Interactive shell interface
- `config/`: Configuration and argument parsing
- `di/`: Dependency injection with Google Wire

### Key Principles
- Dependencies flow inward toward domain layer
- Repository interfaces in domain, implementations in infrastructure
- Use dependency injection (Google Wire) instead of global variables
- Enforce architectural boundaries with go-arch-lint

## Technology Stack

### Core Libraries
- **filesql**: File processing library (github.com/nao1215/filesql)
- **SQLite**: Pure Go implementation (modernc.org/sqlite)
- **Wire**: Dependency injection (github.com/google/wire)
- **go-prompt**: Interactive shell (github.com/c-bata/go-prompt)
- **tablewriter**: Table formatting (github.com/olekukonko/tablewriter@v0.0.5)

### File Format Support
- CSV, TSV, LTSV, Excel (.xlsx)
- Compressed files: .gz, .bz2, .xz, .zst
- Automatic type detection and format recognition

## Development Standards

### Code Quality
- Follow Effective Go guidelines
- Write documentation comments for all public APIs
- Use `errors.Is` and `errors.As` for error handling
- Never ignore errors
- Remove duplicate code

### Testing
- Write tests using `t.Run()` with clear input/output descriptions
- Use `t.Parallel()` when possible
- Maintain >80% test coverage
- Store test data in `testdata/` directories
- Use golden file testing for output validation

### Commands
```bash
make test     # Run tests with coverage
make lint     # Code quality checks
make generate # Generate Wire DI code
make build    # Build binary
```

## Common Patterns

### Error Handling
```go
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}
```

### Dependency Injection
```go
// Add to wire.go, then run `make generate`
wire.Build(
    NewComponent,
    // ... other providers
)
```

### Test Structure
```go
func TestFunction(t *testing.T) {
    t.Parallel()
    
    tests := []struct {
        name string
        input string
        want string
    }{
        {name: "valid input returns expected output", input: "test", want: "result"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            // test implementation
        })
    }
}
```

### Repository Pattern
```go
// Interface in domain/repository
type UserRepository interface {
    Create(ctx context.Context, user *model.User) error
}

// Implementation in infrastructure
type userRepository struct {
    db *sql.DB
}
```

## File Processing Guidelines

### Using filesql
```go
// Load files into SQLite3 database
db, err := filesql.Open("data.csv", "users.xlsx")
if err != nil {
    return fmt.Errorf("failed to open files: %w", err)
}
defer db.Close()

// Execute SQL queries
rows, err := db.Query("SELECT * FROM data WHERE column > ?", value)
```

### Shell Commands
- Commands start with dots: `.help`, `.tables`, `.import`
- Use go-prompt for completion and history
- Format output with tablewriter

## Important Notes

### Recent Changes
- Migrated to filesql for better performance
- Removed JSON support (focus on structured data)
- Added compressed file support
- Switched to pure Go SQLite (no CGO)

### Architecture Enforcement
- go-arch-lint enforces architectural boundaries
- Check `.go-arch-lint.yml` for component dependencies
- Respect layer separation and dependency direction

### Breaking Changes Awareness
- JSON files no longer supported
- Output formatting may differ due to type detection
- Dependencies changed from CGO to pure Go

## Suggestions for Copilot

When suggesting code:
1. Respect Clean Architecture boundaries
2. Use filesql for file operations
3. Follow established error handling patterns
4. Include appropriate tests with suggestions
5. Consider cross-platform compatibility
6. Use dependency injection instead of globals
7. Write documentation comments for public APIs
8. Check architectural compliance with existing patterns