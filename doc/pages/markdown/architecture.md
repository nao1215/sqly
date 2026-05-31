### The sqly architecture

The sqly project adopts the [Clean Architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html). We are verifying whether the implementation follows the architecture using [fe3dback/go-arch-lint](https://github.com/fe3dback/go-arch-lint).

The sqly shell calls the `usecase` interface, and the `interactor` implements the `usecase`. The `interactor` uses the `domain` (business logic) to perform data operations. Specifically, it uses the `infrastructure` that implements the `domain/repository` interface.

The sqly reads data from each file, converts it into a table format, and stores the converted table data in an in-memory SQLite3 database. sqly does not have its own SQL parser and relies on SQLite3 for parsing.

Here is a high-level overview of the Clean Architecture for the sqly project:

```text
+------------------+     +------------------+     +------------------+
|      cmd        | --> |      shell       | --> |     usecase      | interface
+------------------+     +------------------+     +------------------+
                                                          |
                                                          v
                                                 +------------------+
                                                 |    interactor    | implement
                                                 +------------------+
                                                          |
                                                          v
                      +------------------+     +------------------+
                      | domain/model     | --> | domain/repository | interface
                      +------------------+     +------------------+
                                                          |
                                                          v
                                                 +------------------+
                                                 |  infrastructure  | implement
                                                 +------------------+
```

### filesql session integration

sqly reads files through the [filesql](https://github.com/nao1215/filesql) library and runs SQL on a single shared in-memory SQLite database that lives for the whole session.

On import, sqly streams the files directly into the shared database with `filesql.LoadInto`, which loads each file as a table (with filesql's automatic column-type detection) and replaces a same-named table so re-import is last-wins. Because the data lands in the session database in one pass, there is no separate filesql database and no table-by-table row copy. The pool is pinned to one connection, since SQLite `:memory:` is private per connection.

Earlier sqly opened a temporary filesql database and copied every table into the shared one. That preserved schema fidelity but inserted every row twice and held the data in memory twice; for a 100k-row CSV that made imports about 2.5x slower and roughly doubled peak memory. `LoadInto` (filesql v0.13.0) removes that boundary: the shared database still backs one long-lived session, so command history, repeated imports, cross-file JOINs, last-wins overwrite, `.schema`/`.describe` (real `sqlite_master` and `PRAGMA table_info`), `--inspect`, and export keep working, now without the copy overhead.

```text
files --> filesql.LoadInto --> shared in-memory SQLite DB --> SQL, .schema, .describe, --inspect, .dump
```

ACH and Fedwire imports have a deterministic cleanup path. Loading registers ACH/Fedwire table sets in global registries used for round-trip dump. sqly holds the data in the shared database and does not retain those registries, so after each import it unregisters them, scoped to the base names of the `.ach`/`.fed` files, using `defer` so cleanup runs even on partial failure. This keeps long-running shells leak-free and makes repeated imports produce identical tables.

### Directory structure

```shell
├── config  # When the sqly command is executed, the configuration is read from the config directory.  
├── di      # Dependency injection
├── doc     # Documentation
├── domain  # Business logic. This directory contains the model and repository interfaces.
├── golden  # Test framework. This package is forked from https://github.com/sebdah/goldie
├── infrastructure # Implementation of the repository interface
├── interactor    # Implementation of the usecase interface. This package uses the domain and infrastructure packages.
├── shell        # sqly shell
├── testdata     # Test data
└── usecase      # Use case interface. The shell calls this interface.
```
