package golden

// fixtureNotFoundError is thrown when the fixture file could not be found.
type fixtureNotFoundError struct {
	message string
}

// newErrFixtureNotFound returns a new instance of the error.
func newErrFixtureNotFound() *fixtureNotFoundError {
	return &fixtureNotFoundError{
		// TODO: flag name should be based on the variable value
		message: "Golden fixture not found. Try running with -update flag.",
	}
}

// Error returns the error message.
func (e *fixtureNotFoundError) Error() string {
	return e.message
}

// fixtureMismatchError is thrown when the actual and expected data is not
// matching.
type fixtureMismatchError struct {
	message string
}

// newErrFixtureMismatch returns a new instance of the error.
func newErrFixtureMismatch(message string) *fixtureMismatchError {
	return &fixtureMismatchError{
		message: message,
	}
}

func (e *fixtureMismatchError) Error() string {
	return e.message
}

// errFixtureDirecetoryIsFile is thrown when the fixture directory is a file
type fixtureDirectoryIsFileError struct {
	file string
}

// newFixtureDirectoryIsFile returns a new instance of the error.
func newErrFixtureDirectoryIsFile(file string) *fixtureDirectoryIsFileError {
	return &fixtureDirectoryIsFileError{
		file: file,
	}
}

func (e *fixtureDirectoryIsFileError) Error() string {
	return "fixture folder is a file: " + e.file
}

func (e *fixtureDirectoryIsFileError) File() string {
	return e.file
}

// missingKeyError is thrown when a value for a template is missing
type missingKeyError struct {
	message string
}

// newErrMissingKey returns a new instance of the error.
func newErrMissingKey(message string) *missingKeyError {
	return &missingKeyError{
		message: message,
	}
}

func (e *missingKeyError) Error() string {
	return e.message
}
