package buildinfo

import (
	"fmt"
	"io"
	"os"
)

// Version is the application build version. It is injected at build time via -ldflags.
var Version string

// Date is the date of the build in the format of a UTC RFC 3339 timestamp (e.g., 2025-10-17T12:34:56Z).
// It is injected at build time via -ldflags.
var Date string

// Commit is the short Git commit hash for the build. It is injected at build time via -ldflags.
var Commit string

func valueOrNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

// Print prints into a passed writer.
func Print(w io.Writer) {
	fmt.Fprintln(w, "Build version:", valueOrNA(Version))
	fmt.Fprintln(w, "Build date:", valueOrNA(Date))
	fmt.Fprintln(w, "Build commit:", valueOrNA(Commit))
}

// PrintStdout prints into the stdout.
func PrintStdout() { Print(os.Stdout) }
