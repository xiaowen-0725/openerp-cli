//go:build windows

package selfupdate

import (
	"os"
	"syscall"
)

// replaceExecutable swaps the running binary on Windows, where the live .exe
// cannot be overwritten directly: move it aside, move the new one in, then
// best-effort delete the old (it may linger until the process exits).
func replaceExecutable(target, staged string) error {
	old := target + ".old"
	_ = os.Remove(old)
	if err := os.Rename(target, old); err != nil {
		return err
	}
	if err := os.Rename(staged, target); err != nil {
		_ = os.Rename(old, target) // roll back
		return err
	}
	_ = os.Remove(old)
	return nil
}

// detachSysProcAttr starts the updater in a new process group so it is not
// terminated with the parent CLI.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: 0x00000200} // CREATE_NEW_PROCESS_GROUP
}
