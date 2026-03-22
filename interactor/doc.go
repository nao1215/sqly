// Package interactor provides the implementation of use cases for handling
// database operations, file export, and command history in sqly.
//
// The interactor layer acts as an intermediary between the domain layer
// and the infrastructure layer, orchestrating business logic and data flow.
// It implements the use case interfaces defined in the usecase package.
//
// Key components:
//   - SQLite3 interactor: Manages database operations and file import via filesql
//   - Export interactor: Handles table export to CSV, TSV, LTSV, Excel, and Markdown
//   - History interactor: Tracks command history
//
// All file reading operations are delegated to the filesql library, which provides
// automatic type detection, compressed file support, and in-memory SQLite processing.
package interactor
