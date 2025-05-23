package filewatcher

import (
	"context"
	"os"
	"path"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
)

func TestFileWatcher_Watch(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	notifyCh := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	file := path.Join(os.TempDir(), "test-file")
	_, err := os.Create(file)
	g.Expect(err).ToNot(HaveOccurred())
	defer os.Remove(file)

	w, err := NewFileWatcher(logr.Discard(), []string{file}, notifyCh)
	g.Expect(err).ToNot(HaveOccurred())
	w.interval = 300 * time.Millisecond

	go w.Watch(ctx)

	w.watcher.Events <- fsnotify.Event{Name: file, Op: fsnotify.Write}
	g.Eventually(func() bool {
		return w.filesChanged.Load()
	}).Should(BeTrue())

	g.Eventually(notifyCh).Should(Receive())
}

func TestFileWatcher_handleEvent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	w, err := NewFileWatcher(logr.Discard(), []string{"test-file"}, nil)
	g.Expect(err).ToNot(HaveOccurred())

	w.handleEvent(fsnotify.Event{Op: fsnotify.Write})
	g.Expect(w.filesChanged.Load()).To(BeFalse())

	w.handleEvent(fsnotify.Event{Name: "test-chmod", Op: fsnotify.Chmod})
	g.Expect(w.filesChanged.Load()).To(BeFalse())

	w.handleEvent(fsnotify.Event{Name: "test-create", Op: fsnotify.Create})
	g.Expect(w.filesChanged.Load()).To(BeFalse())

	w.handleEvent(fsnotify.Event{Name: "test-write", Op: fsnotify.Write})
	g.Expect(w.filesChanged.Load()).To(BeTrue())
	w.filesChanged.Store(false)

	w.handleEvent(fsnotify.Event{Name: "test-remove", Op: fsnotify.Remove})
	g.Expect(w.filesChanged.Load()).To(BeTrue())
	w.filesChanged.Store(false)

	w.handleEvent(fsnotify.Event{Name: "test-rename", Op: fsnotify.Rename})
	g.Expect(w.filesChanged.Load()).To(BeTrue())
	w.filesChanged.Store(false)
}
