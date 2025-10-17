// Package buildinfo provides a centralized interface for accessing and displaying
// metadata about the current build of the application.
//
// This package defines three variables — Version, Date, and Commit — which are
// automatically populated at build time using Go linker flags (-ldflags).
//
// Typical usage:
//
//	go build -ldflags "\
//	 -X github.com/Heidric/metrics.git/internal/buildinfo.Version=1.2.3 \
//	 -X github.com/Heidric/metrics.git/internal/buildinfo.Date=2025-10-17T12:34:56Z \
//	 -X github.com/Heidric/metrics.git/internal/buildinfo.Commit=$(git rev-parse --short HEAD)" \
//	 -o ./bin/server ./cmd/server
//
// The buildinfo package can then be imported by runtime components (such as cmd/server
// or cmd/agent) to print or log build metadata for observability and debugging purposes.
//
// Example:
//
//	import "github.com/Heidric/metrics.git/internal/buildinfo"
//
//	func main() {
//		buildinfo.PrintStdout()
//		// Rest of the logic...
//	}
//
// # Variables
//
// The following variables are set via -ldflags during build:
//
//   - Version: the semantic version or tag of the build.
//   - Date: the date of the build in the format of a UTC RFC 3339 timestamp (e.g., 2025-10-17T12:34:56Z).
//   - Commit: the short Git commit hash.
//
// If any variable is not populated at link time (as part of go build), it defaults to N/A.
package buildinfo
