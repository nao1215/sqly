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
