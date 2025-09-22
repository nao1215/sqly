// Package interactor provides the implementation of use cases for handling
// various file formats and database operations in sqly.
//
// The interactor layer acts as an intermediary between the domain layer
// and the infrastructure layer, orchestrating business logic and data flow.
// It implements the use case interfaces defined in the usecase package.
//
// Key components:
//   - CSV, TSV, LTSV interactors: Handle file-specific operations
//   - Excel interactor: Manages Excel file operations with sheet support
//   - SQLite3 interactor: Manages database operations
//   - History interactor: Tracks command history
//   - Base file interactor: Provides common functionality for file-based interactors
//
// All file-based interactors now leverage the filesql library for improved
// performance, automatic type detection, and native compressed file support.
package interactor
