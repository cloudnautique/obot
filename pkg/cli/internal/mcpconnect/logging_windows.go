//go:build windows

package mcpconnect

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func redirectStderr(logFile *os.File) error {
	if err := windows.SetStdHandle(windows.STD_ERROR_HANDLE, windows.Handle(logFile.Fd())); err != nil {
		return fmt.Errorf("failed to redirect stderr: %w", err)
	}
	os.Stderr = logFile
	return nil
}
