package infra

import "time"

// Config stores configuration.
type Config struct {
	// EnvName is the name of created environment
	EnvName string

	// Profiles defines the list of application profiles to run
	Profiles []string

	// TimeoutCommit allows to define custom timeout commit for all used chains.
	TimeoutCommit time.Duration

	// CoredVersion defines the version of the cored to be used on start
	CoredVersion string

	// HomeDir is the path where all the files are kept
	HomeDir string

	// RootDir is the path where all the repositories are kept
	RootDir string

	// AppDir is the path where app data are stored
	AppDir string

	// WrapperDir is the path where wrappers are stored
	WrapperDir string

	// TestFilter is a regular expressions used to filter tests to run
	TestFilter string

	// TestGroups limits running integration tests on selected repository test group, empty means no filter
	TestGroups []string

	// VerboseLogging turns on verbose logging
	VerboseLogging bool

	// LogFormat is the format used to encode logs
	LogFormat string

	// CoverageOutputFile is the output path for coverage data in text format
	CoverageOutputFile string
}
