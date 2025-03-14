package filewatcher

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-logr/logr"
)

const monitoringInterval = 5 * time.Second

var emptyEvent = fsnotify.Event{
	Name: "",
	Op:   0,
}

// FileWatcher watches for changes to files and notifies the channel when a change occurs.
type FileWatcher struct {
	filesChanged *atomic.Bool
	watcher      *fsnotify.Watcher
	notifyCh     chan<- struct{}
	logger       logr.Logger
	filesToWatch []string
	interval     time.Duration
}

// NewFileWatcher creates a new FileWatcher instance.
func NewFileWatcher(logger logr.Logger, files []string, notifyCh chan<- struct{}) (*FileWatcher, error) {
	filesChanged := &atomic.Bool{}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize TLS file watcher: %w", err)
	}

	return &FileWatcher{
		filesChanged: filesChanged,
		watcher:      watcher,
		logger:       logger,
		filesToWatch: files,
		notifyCh:     notifyCh,
		interval:     monitoringInterval,
	}, nil
}

// Watch starts the watch for file changes.
func (w *FileWatcher) Watch(ctx context.Context) {
	w.logger.V(1).Info("Starting file watcher")

	ticker := time.NewTicker(w.interval)
	for _, file := range w.filesToWatch {
		w.addWatcher(file)
	}

	for {
		select {
		case <-ctx.Done():
			if err := w.watcher.Close(); err != nil {
				w.logger.Error(err, "unable to close file watcher")
			}
			return
		case event := <-w.watcher.Events:
			w.handleEvent(event)
		case <-ticker.C:
			w.checkForUpdates()
		case err := <-w.watcher.Errors:
			w.logger.Error(err, "error watching file")
		}
	}
}

func (w *FileWatcher) addWatcher(path string) {
	if err := w.watcher.Add(path); err != nil {
		w.logger.Error(err, "failed to watch file", "file", path)
	}
}

func (w *FileWatcher) handleEvent(event fsnotify.Event) {
	if isEventSkippable(event) {
		return
	}

	if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
		w.addWatcher(event.Name)
	}

	w.filesChanged.Store(true)
}

func (w *FileWatcher) checkForUpdates() {
	if w.filesChanged.Load() {
		w.logger.Info("TLS files changed, sending notification to reset nginx agent connections")
		w.notifyCh <- struct{}{}
		w.filesChanged.Store(false)
	}
}

func isEventSkippable(event fsnotify.Event) bool {
	return event == emptyEvent ||
		event.Name == "" ||
		event.Has(fsnotify.Chmod) ||
		event.Has(fsnotify.Create)
}
