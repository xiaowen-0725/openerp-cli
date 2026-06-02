//go:build !windows

package selfupdate

import (
	"os"
	"syscall"
)

// replaceExecutable atomically swaps the running binary. On Unix a rename over
// the in-use file succeeds: the live process keeps the old inode, and the next
// invocation execs the new one.
func replaceExecutable(target, staged string) error {
	return os.Rename(staged, target)
}

// detachSysProcAttr puts the updater in its own session so it survives the
// parent CLI exiting (and any terminal SIGHUP).
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
