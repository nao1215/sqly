# CHANGELOG

## [v0.12.2](https://github.com/nao1215/sqly/compare/v0.12.1...v0.12.2) (2025-09-17)

### Bug Fixes
* **Table Names**: Fix SQL syntax errors caused by special characters in filenames ([#153](https://github.com/nao1215/sqly/pull/153))
  - Automatically sanitize table names by replacing problematic characters (hyphens, dots, special chars) with underscores
  - Example: `bug-syntax-error.csv` now creates table `bug_syntax_error` instead of failing with syntax error
  - Added comprehensive test coverage for filename sanitization edge cases

### Documentation
* **README**: Update all localized README files with table name sanitization information
  - Added explanations in English, Japanese, Korean, Russian, Chinese, Spanish, and French
  - Clarified that special characters in filenames are automatically replaced with underscores
  - Provided clear examples of filename → table name conversion

### Technical Improvements
* **Testing**: Enhanced test suite for filename edge cases
  - Added tests for files with hyphens, dots, and special characters
  - Verified cross-platform compatibility of table name generation
  - Ensured deterministic table naming behavior

## [v0.12.1](https://github.com/nao1215/sqly/compare/v0.12.0...v0.12.1) (2025-09-06)

### Bug Fixes
* **Completion**: Fix shell completion functionality that was preventing file discovery ([066ea6a](https://github.com/nao1215/sqly/commit/066ea6a))
  - Fixed hidden directory skipping issue in file path completion
  - Completion now properly discovers all importable files recursively
  - Improved completion performance with efficient directory traversal
* **Windows**: Fix Windows compatibility issues in tests ([cc11ab6](https://github.com/nao1215/sqly/commit/cc11ab6))
  - Fixed directory cleanup issues in Windows test environments
  - Added proper directory restoration patterns for cross-platform compatibility
* **Testing**: Add ORDER BY clauses to SQL queries for deterministic test results ([e0fe515](https://github.com/nao1215/sqly/commit/e0fe515))
  - Ensures consistent test results across different platforms and SQLite versions

### Enhancements
* **Shell**: Add Windows path separator support in completion system ([066ea6a](https://github.com/nao1215/sqly/commit/066ea6a))
  - Support for backslash (`\`) path separators on Windows
  - Enhanced path pattern recognition for Windows-style paths (`.\`, `..\`, `C:\`)
* **Code Quality**: Improve error handling and remove unused parameters ([066ea6a](https://github.com/nao1215/sqly/commit/066ea6a))
  - All lint issues resolved
  - Better error propagation in file system operations

### Technical Improvements
* **Completion System**: Optimize file completion algorithm
  - Recursive directory walking with proper hidden file handling  
  - Cross-platform path normalization with `filepath.ToSlash()`
  - Efficient filtering of importable file types
* **Test Coverage**: Maintain high test coverage (36.2% for shell package)
  - All existing tests pass on both Unix and Windows platforms
  - Enhanced test stability with deterministic SQL query ordering

## [v0.12.0](https://github.com/nao1215/sqly/compare/v0.9.0...v0.12.0) (2025-01-09)

### Major Changes
* **BREAKING**: Remove JSON file format support in favor of filesql integration ([d5649f9](https://github.com/nao1215/sqly/commit/d5649f9))
* **Integration**: Migrate to filesql library for enhanced performance and compressed file support ([d5649f9](https://github.com/nao1215/sqly/commit/d5649f9))
* **Performance**: Implement bulk insert operations with transaction batching for faster file processing
* **Compression**: Add native support for compressed files (.gz, .bz2, .xz, .zst) ([d5649f9](https://github.com/nao1215/sqly/commit/d5649f9))
* **Dependencies**: Remove mattn/go-sqlite3 (CGO) in favor of pure Go modernc.org/sqlite ([d5649f9](https://github.com/nao1215/sqly/commit/d5649f9))

### New Features
* **Shell Commands**: Add .cd helper command for directory navigation ([d49e5a7](https://github.com/nao1215/sqly/commit/d49e5a7))
* **Shell Commands**: Add .ls helper command to list directory contents ([d49e5a7](https://github.com/nao1215/sqly/commit/d49e5a7))
* **Shell Commands**: Add .pwd helper command to show current working directory ([8812122](https://github.com/nao1215/sqly/commit/8812122))
* **Interactive**: Display current output mode in shell prompt ([a0f7047](https://github.com/nao1215/sqly/commit/a0f7047))
* **Type Detection**: Automatic column data type detection ensures proper numeric sorting
* **Go Version**: Add support for Go 1.24 ([a4c7512](https://github.com/nao1215/sqly/commit/a4c7512))

### Architecture Improvements
* **Clean Architecture**: Refactor codebase to follow Clean Architecture principles more strictly ([5a4bb96](https://github.com/nao1215/sqly/commit/5a4bb96))
* **Architecture Linting**: Add go-arch-lint for architectural boundary enforcement ([35c7e8f](https://github.com/nao1215/sqly/commit/35c7e8f))
* **Domain Model**: Convert parts of domain model to Value Objects for better encapsulation ([5c8ec2d](https://github.com/nao1215/sqly/commit/5c8ec2d))
* **Dependency Injection**: Improve usecase interfaces and add mock code for testing ([ee92763](https://github.com/nao1215/sqly/commit/ee92763))
* **Package Structure**: Refactor shell package for better organization ([101163f](https://github.com/nao1215/sqly/commit/101163f))

### Documentation & Developer Experience
* **LLM Integration**: Add Claude Code, Cursor, and GitHub Copilot configuration files ([2ceefa0](https://github.com/nao1215/sqly/commit/2ceefa0))
* **Documentation**: Create comprehensive developer documentation ([c368778](https://github.com/nao1215/sqly/commit/c368778))
* **GitHub Pages**: Set up documentation site at https://nao1215.github.io/sqly/ ([a061c49](https://github.com/nao1215/sqly/commit/a061c49))
* **Internationalization**: Add README translations for multiple languages ([b676409](https://github.com/nao1215/sqly/commit/b676409)):
  - Spanish (es)
  - French (fr) 
  - Japanese (ja)
  - Korean (ko)
  - Russian (ru)
  - Chinese Simplified (zh-cn)

### GitHub Actions & Automation
* **AI Assistance**: Add Claude Code Review workflow ([0a86dd2](https://github.com/nao1215/sqly/commit/0a86dd2))
* **AI Assistance**: Add Claude PR Assistant workflow ([5b8be74](https://github.com/nao1215/sqly/commit/5b8be74))

### Dependencies
* Bump github.com/sergi/go-diff from 1.3.1 to 1.4.0 ([dd44965](https://github.com/nao1215/sqly/commit/dd44965))
* Bump github.com/spf13/pflag from 1.0.6 to 1.0.10 ([0763386](https://github.com/nao1215/sqly/commit/0763386))
* Bump github.com/stretchr/testify from 1.10.0 to 1.11.1 ([f9fe0e5](https://github.com/nao1215/sqly/commit/f9fe0e5))
* Bump github.com/xuri/excelize/v2 from 2.9.0 to 2.9.1 ([9cbb0ff](https://github.com/nao1215/sqly/commit/9cbb0ff))
* Bump go.uber.org/mock from 0.5.1 to 0.5.2 ([c50a81f](https://github.com/nao1215/sqly/commit/c50a81f))
* Bump golang.org/x/net from 0.33.0 to 0.36.0 ([3ff5306](https://github.com/nao1215/sqly/commit/3ff5306))
* Bump modernc.org/sqlite from 1.34.5 to 1.36.1 ([b03c0d2](https://github.com/nao1215/sqly/commit/b03c0d2))
* Bump github.com/google/go-cmp from 0.6.0 to 0.7.0 ([38d711c](https://github.com/nao1215/sqly/commit/38d711c))

### Breaking Changes
- **JSON Support Removed**: JSON files (`.json`) are no longer supported as input format
- **CLI Flag Removed**: The `--json` output flag has been removed
- **Output Format**: Numeric formatting may differ slightly due to improved type detection
- **Dependencies**: Removed CGO dependency (mattn/go-sqlite3) in favor of pure Go implementation

### Migration Guide
- **For JSON users**: Export JSON data to CSV format before processing with sqly
- **For developers**: Update any code that relied on JSON-specific functionality
- **Benefits**: Enjoy improved performance, compressed file support, and better type handling

## [v0.9.0](https://github.com/nao1215/sqly/compare/v0.8.1...v0.9.0) (2025-02-03)

* Add architecture linter [#87](https://github.com/nao1215/sqly/pull/87) ([nao1215](https://github.com/nao1215))
* Reduce dependency and add unit tests for interactor [#86](https://github.com/nao1215/sqly/pull/86) ([nao1215](https://github.com/nao1215))
* Add usecase interface and mock code [#85](https://github.com/nao1215/sqly/pull/85) ([nao1215](https://github.com/nao1215))
* Bump github.com/spf13/pflag from 1.0.5 to 1.0.6 [#84](https://github.com/nao1215/sqly/pull/84) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump golang.org/x/net from 0.30.0 to 0.33.0 [#83](https://github.com/nao1215/sqly/pull/83) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.34.4 to 1.34.5 [#82](https://github.com/nao1215/sqly/pull/82) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/mattn/go-colorable from 0.1.13 to 0.1.14 [#81](https://github.com/nao1215/sqly/pull/81) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.34.3 to 1.34.4 [#80](https://github.com/nao1215/sqly/pull/80) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump golang.org/x/crypto from 0.28.0 to 0.31.0 [#79](https://github.com/nao1215/sqly/pull/79) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.34.1 to 1.34.3 [#78](https://github.com/nao1215/sqly/pull/78) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.33.1 to 1.34.1 [#76](https://github.com/nao1215/sqly/pull/76) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/fatih/color from 1.17.0 to 1.18.0 [#75](https://github.com/nao1215/sqly/pull/75) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/mattn/go-sqlite3 from 1.14.23 to 1.14.24 [#73](https://github.com/nao1215/sqly/pull/73) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/xuri/excelize/v2 from 2.8.1 to 2.9.0 [#74](https://github.com/nao1215/sqly/pull/74) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/mattn/go-sqlite3 from 1.14.22 to 1.14.23 [#70](https://github.com/nao1215/sqly/pull/70) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.33.0 to 1.33.1 [#72](https://github.com/nao1215/sqly/pull/72) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.32.0 to 1.33.0 [#71](https://github.com/nao1215/sqly/pull/71) ([dependabot[bot]](https://github.com/apps/dependabot))
* Add go 1.23 in unit test coverage [#69](https://github.com/nao1215/sqly/pull/69) ([nao1215](https://github.com/nao1215))
* Bump modernc.org/sqlite from 1.31.1 to 1.32.0 [#68](https://github.com/nao1215/sqly/pull/68) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.30.2 to 1.31.1 [#67](https://github.com/nao1215/sqly/pull/67) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.30.1 to 1.30.2 [#66](https://github.com/nao1215/sqly/pull/66) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.30.0 to 1.30.1 [#65](https://github.com/nao1215/sqly/pull/65) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump goreleaser/goreleaser-action from 5 to 6 [#64](https://github.com/nao1215/sqly/pull/64) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.29.10 to 1.30.0 [#63](https://github.com/nao1215/sqly/pull/63) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.29.9 to 1.29.10 [#62](https://github.com/nao1215/sqly/pull/62) ([dependabot[bot]](https://github.com/apps/dependabot))
* Update project config [#61](https://github.com/nao1215/sqly/pull/61) ([nao1215](https://github.com/nao1215))
* Bump github.com/fatih/color from 1.16.0 to 1.17.0 [#60](https://github.com/nao1215/sqly/pull/60) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.29.8 to 1.29.9 [#59](https://github.com/nao1215/sqly/pull/59) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.8.1](https://github.com/nao1215/sqly/compare/v0.8.0...v0.8.1) (2024-05-01)

* Introduce homebrew [#58](https://github.com/nao1215/sqly/pull/58) ([nao1215](https://github.com/nao1215))

## [v0.8.0](https://github.com/nao1215/sqly/compare/v0.7.0...v0.8.0) (2024-05-01)

* Change SQLite3 driver from mattn/go-sqlite3 to modernc.org/sqlite [#57](https://github.com/nao1215/sqly/pull/57) ([nao1215](https://github.com/nao1215))
* Add benchmark [#56](https://github.com/nao1215/sqly/pull/56) ([nao1215](https://github.com/nao1215))
* Add unit test for excel [#55](https://github.com/nao1215/sqly/pull/55) ([nao1215](https://github.com/nao1215))

## [v0.7.0](https://github.com/nao1215/sqly/compare/v0.6.5...v0.7.0) (2024-04-30)

* Bump golang.org/x/net from 0.21.0 to 0.23.0 [#54](https://github.com/nao1215/sqly/pull/54) ([dependabot[bot]](https://github.com/apps/dependabot))
* Support Microsoft Excel™ (XLAM / XLSM / XLSX / XLTM / XLTX) [#53](https://github.com/nao1215/sqly/pull/53) ([nao1215](https://github.com/nao1215))

## [v0.6.5](https://github.com/nao1215/sqly/compare/v0.6.4...v0.6.5) (2024-04-29)

## [v0.6.4](https://github.com/nao1215/sqly/compare/v0.5.2...v0.6.4) (2024-04-29)

* Bump goreleaser/goreleaser-action from 2 to 5 [#50](https://github.com/nao1215/sqly/pull/50) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump actions/checkout from 3 to 4 [#52](https://github.com/nao1215/sqly/pull/52) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump actions/setup-go from 3 to 5 [#51](https://github.com/nao1215/sqly/pull/51) ([dependabot[bot]](https://github.com/apps/dependabot))
* Maintain dependencies for GitHub Actions [#49](https://github.com/nao1215/sqly/pull/49) ([nao1215](https://github.com/nao1215))
* Introduce numerical sorting [#48](https://github.com/nao1215/sqly/pull/48) ([nao1215](https://github.com/nao1215))
* Fix issue 43: Panic when importing json table with numeric field. [#47](https://github.com/nao1215/sqly/pull/47) ([nao1215](https://github.com/nao1215))
* Fix issue 42 (bug): Panic when json field is null [#46](https://github.com/nao1215/sqly/pull/46) ([nao1215](https://github.com/nao1215))
* Update project config [#45](https://github.com/nao1215/sqly/pull/45) ([nao1215](https://github.com/nao1215))
* Introduce octocov [#44](https://github.com/nao1215/sqly/pull/44) ([nao1215](https://github.com/nao1215))
* Bump github.com/google/wire from 0.5.0 to 0.6.0 [#41](https://github.com/nao1215/sqly/pull/41) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/mattn/go-sqlite3 from 1.14.19 to 1.14.22 [#40](https://github.com/nao1215/sqly/pull/40) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/mattn/go-sqlite3 from 1.14.18 to 1.14.19 [#37](https://github.com/nao1215/sqly/pull/37) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/fatih/color from 1.15.0 to 1.16.0 [#36](https://github.com/nao1215/sqly/pull/36) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/mattn/go-sqlite3 from 1.14.17 to 1.14.18 [#35](https://github.com/nao1215/sqly/pull/35) ([dependabot[bot]](https://github.com/apps/dependabot))
* (auto merged) Bump github.com/google/go-cmp from 0.5.9 to 0.6.0 [#34](https://github.com/nao1215/sqly/pull/34) ([dependabot[bot]](https://github.com/apps/dependabot))
* Add automerged workflows [#33](https://github.com/nao1215/sqly/pull/33) ([nao1215](https://github.com/nao1215))
* Bump github.com/mattn/go-sqlite3 from 1.14.16 to 1.14.17 [#32](https://github.com/nao1215/sqly/pull/32) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/nao1215/gorky from 0.2.0 to 0.2.1 [#31](https://github.com/nao1215/sqly/pull/31) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/fatih/color from 1.14.1 to 1.15.0 [#30](https://github.com/nao1215/sqly/pull/30) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/fatih/color from 1.13.0 to 1.14.1 [#29](https://github.com/nao1215/sqly/pull/29) ([dependabot[bot]](https://github.com/apps/dependabot))
* Change golden package import path [#28](https://github.com/nao1215/sqly/pull/28) ([nao1215](https://github.com/nao1215))

## [v0.5.2](https://github.com/nao1215/sqly/compare/v0.5.1...v0.5.2) (2022-11-27)

* add unit test for infra package [#27](https://github.com/nao1215/sqly/pull/27) ([nao1215](https://github.com/nao1215))
* Add basic unit test for shell [#26](https://github.com/nao1215/sqly/pull/26) ([nao1215](https://github.com/nao1215))
* Add unit test for model package [#24](https://github.com/nao1215/sqly/pull/24) ([nao1215](https://github.com/nao1215))
* Bump github.com/google/go-cmp from 0.2.0 to 0.5.9 [#25](https://github.com/nao1215/sqly/pull/25) ([dependabot[bot]](https://github.com/apps/dependabot))
* Change golden test package from goldie to golden and more [#23](https://github.com/nao1215/sqly/pull/23) ([nao1215](https://github.com/nao1215))
* Add unit test for argument paser [#21](https://github.com/nao1215/sqly/pull/21) ([nao1215](https://github.com/nao1215))

## [v0.5.1](https://github.com/nao1215/sqly/compare/v0.5.0...v0.5.1) (2022-11-19)

* Add sqlite3 syntax completion [#22](https://github.com/nao1215/sqly/pull/22) ([nao1215](https://github.com/nao1215))

## [v0.5.0](https://github.com/nao1215/sqly/compare/v0.4.0...v0.5.0) (2022-11-13)

* Feat dump tsv ltsv json [#20](https://github.com/nao1215/sqly/pull/20) ([nao1215](https://github.com/nao1215))
* Add featuer thar print date by markdown table format [#19](https://github.com/nao1215/sqly/pull/19) ([nao1215](https://github.com/nao1215))
* Feat import ltsv [#18](https://github.com/nao1215/sqly/pull/18) ([nao1215](https://github.com/nao1215))

## [v0.4.0](https://github.com/nao1215/sqly/compare/v0.3.1...v0.4.0) (2022-11-13)

* Feat import tsv [#17](https://github.com/nao1215/sqly/pull/17) ([nao1215](https://github.com/nao1215))

## [v0.3.1](https://github.com/nao1215/sqly/compare/v0.3.0...v0.3.1) (2022-11-11)

* Fix panic bug when import file that is without extension [#15](https://github.com/nao1215/sqly/pull/15) ([nao1215](https://github.com/nao1215))

## [v0.3.0](https://github.com/nao1215/sqly/compare/v0.2.1...v0.3.0) (2022-11-10)

* Feat import json [#14](https://github.com/nao1215/sqly/pull/14) ([nao1215](https://github.com/nao1215))
* Fix input delays when increasing records [#13](https://github.com/nao1215/sqly/pull/13) ([nao1215](https://github.com/nao1215))

## [v0.2.1](https://github.com/nao1215/sqly/compare/v0.2.0...v0.2.1) (2022-11-09)

* Add header command [#12](https://github.com/nao1215/sqly/pull/12) ([nao1215](https://github.com/nao1215))

## [v0.2.0](https://github.com/nao1215/sqly/compare/v0.1.1...v0.2.0) (2022-11-09)

* Fixed a display collapse problem when multiple lines are entered [#11](https://github.com/nao1215/sqly/pull/11) ([nao1215](https://github.com/nao1215))

## [v0.1.1](https://github.com/nao1215/sqly/compare/v0.1.0...v0.1.1) (2022-11-07)

* Fixed a bug that caused SQL to fail if there was a trailing semicolon [#10](https://github.com/nao1215/sqly/pull/10) ([nao1215](https://github.com/nao1215))

## [v0.1.0](https://github.com/nao1215/sqly/compare/v0.0.11...v0.1.0) (2022-11-07)

* Add move cursor function in intaractive shell [#9](https://github.com/nao1215/sqly/pull/9) ([nao1215](https://github.com/nao1215))

## [v0.0.11](https://github.com/nao1215/sqly/compare/v0.0.10...v0.0.11) (2022-11-06)

* Fixed a bug in which the wrong arguments were used [#8](https://github.com/nao1215/sqly/pull/8) ([nao1215](https://github.com/nao1215))

## [v0.0.10](https://github.com/nao1215/sqly/compare/v0.0.9...v0.0.10) (2022-11-06)

* Added CSV output mode [#7](https://github.com/nao1215/sqly/pull/7) ([nao1215](https://github.com/nao1215))

## [v0.0.9](https://github.com/nao1215/sqly/compare/v0.0.7...v0.0.9) (2022-11-06)

## [v0.0.7](https://github.com/nao1215/sqly/compare/v0.0.6...v0.0.7) (2022-11-06)

* Improve execute query [#6](https://github.com/nao1215/sqly/pull/6) ([nao1215](https://github.com/nao1215))

## [v0.0.6](https://github.com/nao1215/sqly/compare/v0.0.5...v0.0.6) (2022-11-05)

## [v0.0.5](https://github.com/nao1215/sqly/compare/v0.0.4...v0.0.5) (2022-11-05)

* Add history usecase, repository, infra. sqly manage history by sqlite3 [#5](https://github.com/nao1215/sqly/pull/5) ([nao1215](https://github.com/nao1215))
* Add function that execute select query [#4](https://github.com/nao1215/sqly/pull/4) ([nao1215](https://github.com/nao1215))

## [v0.0.4](https://github.com/nao1215/sqly/compare/v0.0.3...v0.0.4) (2022-11-05)

## [v0.0.3](https://github.com/nao1215/sqly/compare/v0.0.2...v0.0.3) (2022-11-05)

* Add import command [#3](https://github.com/nao1215/sqly/pull/3) ([nao1215](https://github.com/nao1215))

## [v0.0.2](https://github.com/nao1215/sqly/compare/v0.0.1...v0.0.2) (2022-11-05)

* Add .tables command [#2](https://github.com/nao1215/sqly/pull/2) ([nao1215](https://github.com/nao1215))
* Add .exit/.help command and history manager [#1](https://github.com/nao1215/sqly/pull/1) ([nao1215](https://github.com/nao1215))

## [v0.0.1](https://github.com/nao1215/sqly/compare/dbf99896449e...v0.0.1) (2022-11-03)