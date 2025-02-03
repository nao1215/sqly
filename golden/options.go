package golden

import "os"

// WithFixtureDir sets the fixture directory.
//
// Defaults to `testdata`
func (g *Golden) WithFixtureDir(dir string) error {
	g.fixtureDir = dir
	return nil
}

// WithNameSuffix sets the file suffix to be used for the golden file.
//
// Defaults to `.golden`
func (g *Golden) WithNameSuffix(suffix string) error {
	g.fileNameSuffix = suffix
	return nil
}

// WithFilePerms sets the file permissions on the golden files that are
// created.
//
// Defaults to 0644.
func (g *Golden) WithFilePerms(mode os.FileMode) error {
	g.filePerms = mode
	return nil
}

// WithDirPerms sets the directory permissions for the directories in which the
// golden files are created.
//
// Defaults to 0755.
func (g *Golden) WithDirPerms(mode os.FileMode) error {
	g.dirPerms = mode
	return nil
}

// WithDiffEngine sets the `diff` engine that will be used to generate the
// `diff` text.
func (g *Golden) WithDiffEngine(engine DiffEngine) error {
	g.diffEngine = engine
	return nil
}

// WithDiffFn sets the `diff` engine to be a function that implements the
// DiffFn signature. This allows for any customized diff logic you would like
// to create.
func (g *Golden) WithDiffFn(fn DiffFn) error {
	g.diffFn = fn
	return nil
}

// WithIgnoreTemplateErrors allows template processing to ignore any variables
// in the template that do not have corresponding data values passed in.
//
// Default value is false.
func (g *Golden) WithIgnoreTemplateErrors(ignoreErrors bool) error {
	g.ignoreTemplateErrors = ignoreErrors
	return nil
}

// WithTestNameForDir will create a directory with the test's name in the
// fixture directory to store all the golden files.
//
// Default value is false.
func (g *Golden) WithTestNameForDir(use bool) error {
	g.useTestNameForDir = use
	return nil
}

// WithSubTestNameForDir will create a directory with the sub test's name to
// store all the golden files. If WithTestNameForDir is enabled, it will be in
// the test name's directory. Otherwise, it will be in the fixture directory.
//
// Default value is false.
func (g *Golden) WithSubTestNameForDir(use bool) error {
	g.useSubTestNameForDir = use
	return nil
}
