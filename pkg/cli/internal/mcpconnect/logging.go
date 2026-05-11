package mcpconnect

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
)

const logFileName = "mcp-connect.log"

func redirectLogs(localStorageDir string) (*os.File, error) {
	logFile, err := os.OpenFile(filepath.Join(localStorageDir, logFileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	if err := redirectStderr(logFile); err != nil {
		_ = logFile.Close()
		return nil, err
	}

	log.SetOutput(logFile)
	slog.SetDefault(slog.New(slog.NewTextHandler(logFile, nil)))
	return logFile, nil
}
