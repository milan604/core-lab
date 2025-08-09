package version

import (
	"runtime"
)

// These variables are intended to be set at build time via -ldflags.
// Defaults are useful for local development builds.
var (
	// Version is the semantic version of the build, e.g. v0.1.0. Defaults to "dev".
	Version = "dev"
	// Commit is the short git commit hash. Defaults to ""
	Commit = ""
	// Date is the build timestamp in RFC3339. Defaults to ""
	Date = ""
	// Go is the Go toolchain version used for the build.
	Go = runtime.Version()
)

// Info returns a map of version/build metadata suitable for logging or JSON responses.
func Info() map[string]string {
	return map[string]string{
		"version": Version,
		"commit":  Commit,
		"date":    Date,
		"go":      Go,
	}
}
