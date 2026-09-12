package framework

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

var (
	testLogger   *slog.Logger
	testLoggerMu sync.RWMutex
	testLogFile  *os.File
)

// SetupTestLogger initializes the file-backed JSON logger for this Ginkgo process.
// outDir is the per-process directory (e.g. /tmp/ngf-test-logs/proc-1); empty means no-op.
func SetupTestLogger(outDir string) error {
	if outDir == "" {
		setLogger(slog.New(noopHandler{}))
		return nil
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(filepath.Join(outDir, "test.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}

	testLogFile = f
	setLogger(slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo})))
	return nil
}

// CloseTestLogger flushes and closes the log file; must be called from SynchronizedAfterSuite phase 1.
func CloseTestLogger() error {
	// Prevent any late log writes from targeting a closed file.
	setLogger(slog.New(noopHandler{}))
	if testLogFile == nil {
		return nil
	}
	err := testLogFile.Close()
	testLogFile = nil
	return err
}

func LogTestStart(name string) {
	getLogger().Info("starting test", "test", name)
}

func LogTestEnd(name, status string) {
	getLogger().Info("finished test", "test", name, "status", status)
}

func setLogger(l *slog.Logger) {
	testLoggerMu.Lock()
	defer testLoggerMu.Unlock()
	testLogger = l
}

func getLogger() *slog.Logger {
	testLoggerMu.RLock()
	defer testLoggerMu.RUnlock()
	if testLogger == nil {
		return slog.New(noopHandler{})
	}
	return testLogger
}

// noopHandler discards all log records.
type noopHandler struct{}

func (noopHandler) Enabled(_ context.Context, _ slog.Level) bool  { return false }
func (noopHandler) Handle(_ context.Context, _ slog.Record) error { return nil }
func (noopHandler) WithAttrs(_ []slog.Attr) slog.Handler          { return noopHandler{} }
func (noopHandler) WithGroup(_ string) slog.Handler               { return noopHandler{} }

var _ slog.Handler = noopHandler{}
