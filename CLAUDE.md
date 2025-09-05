# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Building and Testing
- `make build` - Build the sqly binary
- `make test` - Run tests with coverage report (generates cover.out and cover.html)
- `make clean` - Clean build artifacts
- `make bench` - Run benchmarks for import performance
- `make coverage-tree` - Generate test coverage treemap visualization

### Code Quality
- `make lint` - Run golangci-lint and go-arch-lint
- `make generate` - Generate code from templates (runs `go generate ./...`)
- `make tools` - Install all development dependencies

### Running Tests
- `go test ./...` - Run all tests
- `go test ./package_name` - Run tests for specific package
- `go test -v ./package_name` - Run tests with verbose output

## Architecture Overview

sqly follows Clean Architecture principles with dependency injection via Google Wire:

### Core Layers
- **Domain** (`domain/`): Business logic and models for different file formats (CSV, TSV, LTSV, Excel)
- **Interactor** (`interactor/`): Use cases and application logic
- **Infrastructure** (`infrastructure/`): External dependencies (SQLite3, file system)
- **Shell** (`shell/`): Interactive shell interface and commands

### Key Components
- **Dependency Injection**: Uses Google Wire (`di/wire.go`) for dependency management
- **Interactive Shell**: Built with go-prompt library, provides SQL completion and command history
- **File Format Support**: Automatic detection and import of CSV/TSV/LTSV/Excel into SQLite3 in-memory database
- **Multiple Output Formats**: Results can be output as table, CSV, TSV, LTSV

### Package Structure
- `config/` - Configuration and argument parsing
- `domain/model/` - Domain models for each file format
- `domain/repository/` - Repository interfaces
- `infrastructure/persistence/` - File format parsers and persisters
- `infrastructure/memory/` - SQLite3 in-memory database management
- `interactor/` - Business logic interactors for each file format
- `shell/` - Interactive shell commands and state management
- `golden/` - Test utilities for golden file testing

### Development Notes
- Uses SQLite3 in-memory database for SQL execution
- Wire dependency injection requires running `wire` command after changes to DI configuration
- Table names are automatically derived from file names or Excel sheet names
- Shell commands begin with dots (e.g., `.help`, `.tables`, `.import`)

## Development Rules
- **Test-Driven Development**: Follow TDD practices. Always write test code and maintain the test pyramid
- **Working code**: Ensure that `make test` and `make lint` succeed after completing work
- **Comments in English**: Write code comments in English to accept international contributors
- **User-friendly documentation comments**: Write detailed explanations and example code for public functions

## Coding Guidelines
- **No global variables**: Manage state through function arguments and return values
- **Coding rules**: Follow Golang coding rules and [Effective Go](https://go.dev/doc/effective_go) guidelines
- **Package comments are mandatory**: Describe package overview in `doc.go` for each package
- **Comments for public functions**: Always write comments following go doc rules for public APIs
- **Remove duplicate code**: Check for and remove unnecessary duplicate code after completing work
- **Error handling**: Use `errors.Is` and `errors.As` for error interface equality checks. Never omit error handling
- **Documentation comments**: Write documentation comments to help users understand code usage
- **Update README**: When adding features, update all localized README files (ja/ko/ru/zh-cn/es/fr)

## Testing Guidelines
- **Readable Test Code**: Avoid excessive optimization (DRY) and aim for easily understandable tests
- **Clear input/output**: Create tests with `t.Run()` and clarify test case input/output
- **Test descriptions**: The first argument of `t.Run()` should clearly describe input/output relationship
- **Test granularity**: Aim for 80% or higher coverage with unit tests
- **Parallel test execution**: Use `t.Parallel()` whenever possible for faster test runs
- **Cross-platform support**: Tests run on Linux, macOS, and Windows through GitHub Actions
- **Test data storage**: Store sample files in the `testdata` directory

### Major Migration: filesql Integration

#### Overview
sqly has migrated to use the [filesql](https://github.com/nao1215/filesql) library for improved performance and compressed file support.

#### Key Changes
- **Enhanced Performance**: Better bulk insert operations and automatic type detection
- **Compressed File Support**: Native support for .gz, .bz2, .xz, .zst files
- **Removed JSON Support**: JSON file format support has been removed to focus on structured data formats
- **Improved SQLite Integration**: Uses modernc.org/sqlite (pure Go) instead of mattn/go-sqlite3 (CGO)

#### Architecture Changes
- `infrastructure/filesql/` package provides the filesql integration layer
- All file format interactors now use filesql for file processing
- Dependency injection updated to provide FileSQLAdapter instead of traditional implementations
- Tests updated to use filesql-based implementations for consistency

#### Breaking Changes
- JSON file support removed (`.json` files no longer supported)
- `--json` flag removed from CLI
- Output formatting may differ slightly due to improved type detection
- Dependencies reduced by removing CGO-based SQLite driver

### Architecture Enforcement
The project uses go-arch-lint (`.go-arch-lint.yml`) to enforce architectural boundaries and prevent circular dependencies. The architecture follows these principles:

- **Layered Architecture**: Clear separation between domain, usecase, infrastructure, and presentation layers
- **Dependency Direction**: Dependencies flow inward toward the domain layer
- **Clean Interfaces**: Repository interfaces defined in domain, implemented in infrastructure
- **Vendor Isolation**: External dependencies are contained within specific layers

### Supported File Formats
- CSV, TSV, LTSV files (including compressed versions: .gz, .bz2, .xz, .zst)
- Microsoft Excel files (.xlsx)
- Automatic file format detection based on file extension