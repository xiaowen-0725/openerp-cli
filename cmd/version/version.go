// Package version holds the CLI version as a leaf package so any command (root,
// update, the background self-updater) can read it without an import cycle.
// Overridden at build time via -ldflags -X .../cmd/version.Version.
package version

// Version is the CLI version. `make build` defaults to a dev sentinel; releases
// stamp a clean X.Y.Z tag (which is what gates self-update on/off).
var Version = "0.1.0-poc"
