//go:build !windows

package mcpconnect

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func redirectStderr(logFile *os.File) error {
	if err := unix.Dup2(int(logFile.Fd()), int(os.Stderr.Fd())); err != nil {
		return fmt.Errorf("failed to redirect stderr: %w", err)
	}
	os.Stderr = logFile
	return nil
}
