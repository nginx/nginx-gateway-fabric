package filewatcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	fileHashes   map[string]string
	logger       logr.Logger
	filesToWatch []string
	pathsToWatch []string
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
		pathsToWatch: buildPathsToWatch(files),
		fileHashes:   make(map[string]string, len(files)),
		notifyCh:     notifyCh,
		interval:     monitoringInterval,
	}, nil
}

// Watch starts the watch for file changes.
func (w *FileWatcher) Watch(ctx context.Context) {
	w.logger.V(1).Info("Starting file watcher")

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for _, watchPath := range w.pathsToWatch {
		w.addWatcherWithRetry(ctx, watchPath)
	}

	w.snapshotFileHashes()

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
		w.logger.Error(err, "failed to watch path", "path", path)
	}
}

func (w *FileWatcher) addWatcherWithRetry(ctx context.Context, path string) {
	backoff := 100 * time.Millisecond
	for attempt := 1; ; attempt++ {
		if err := w.watcher.Add(path); err != nil {
			if attempt >= 5 {
				w.logger.Error(err, "failed to watch path after retries", "path", path, "attempts", attempt)
				return
			}

			w.logger.Error(err, "failed to watch path, retrying", "path", path, "attempt", attempt)

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}

			backoff *= 2
			continue
		}

		return
	}
}

func (w *FileWatcher) handleEvent(event fsnotify.Event) {
	if isEventSkippable(event) {
		return
	}

	if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
		w.addWatcher(filepath.Dir(event.Name))
	}

	w.filesChanged.Store(true)
}

func (w *FileWatcher) checkForUpdates() {
	if w.didFileHashesChange() {
		w.filesChanged.Store(true)
	}

	if w.filesChanged.Load() {
		w.logger.Info("TLS files changed, sending notification to reset nginx agent connections")
		w.notifyCh <- struct{}{}
		w.filesChanged.Store(false)
	}
}

func (w *FileWatcher) snapshotFileHashes() {
	for _, file := range w.filesToWatch {
		w.fileHashes[file] = hashFileContents(file)
	}
}

func (w *FileWatcher) didFileHashesChange() bool {
	changed := false
	for _, file := range w.filesToWatch {
		currentHash := hashFileContents(file)
		if prevHash, ok := w.fileHashes[file]; !ok || prevHash != currentHash {
			w.fileHashes[file] = currentHash
			changed = true
		}
	}

	return changed
}

func buildPathsToWatch(files []string) []string {
	set := make(map[string]struct{}, len(files))
	for _, file := range files {
		dir := filepath.Dir(file)
		if dir == "." {
			dir = file
		}
		set[dir] = struct{}{}
	}

	paths := make([]string, 0, len(set))
	for path := range set {
		paths = append(paths, path)
	}

	sort.Strings(paths)
	return paths
}

func hashFileContents(path string) string {
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("read-error:%v", err)
	}

	hash := sha256.Sum256(contents)
	return hex.EncodeToString(hash[:])
}

func isEventSkippable(event fsnotify.Event) bool {
	return event == emptyEvent ||
		event.Name == "" ||
		event.Has(fsnotify.Chmod) ||
		event.Has(fsnotify.Create)
}
