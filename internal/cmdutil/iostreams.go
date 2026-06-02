// Package cmdutil holds the Factory (dependency injection) and IOStreams shared
// by all commands, following larksuite/cli's pattern. Commands read/write only
// through f.IOStreams so tests can inject buffers and stdout stays data-only.
package cmdutil

import (
	"io"
	"os"
)

// IOStreams abstracts process I/O for testability.
type IOStreams struct {
	In  io.Reader
	Out io.Writer // data
	Err io.Writer // errors, verbose logs, progress
}

// SystemIOStreams wires the real process streams.
func SystemIOStreams() *IOStreams {
	return &IOStreams{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}
}
