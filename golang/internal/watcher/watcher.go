package watcher

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher handles file system watching
type Watcher struct {
	watcher  *fsnotify.Watcher
	callback func()
	debounce time.Duration
	timer    *time.Timer
}

// New creates a new file watcher
func New(watchDir string, callback func()) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("cannot create file watcher: %w", err)
	}

	w := &Watcher{
		watcher:  fsWatcher,
		callback: callback,
		debounce: 500 * time.Millisecond, // Debounce multiple events
	}

	// Add the directory to watch
	err = filepath.Walk(watchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return fsWatcher.Add(path)
		}
		return nil
	})
	if err != nil {
		fsWatcher.Close()
		return nil, fmt.Errorf("cannot add watch directory: %w", err)
	}

	// Start the event loop
	go w.watchLoop()

	return w, nil
}

// Close stops the watcher
func (w *Watcher) Close() error {
	if w.timer != nil {
		w.timer.Stop()
	}
	return w.watcher.Close()
}

func (w *Watcher) watchLoop() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			
			// Only trigger on write and create events
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				// Debounce: reset timer on each event
				if w.timer != nil {
					w.timer.Stop()
				}
				w.timer = time.AfterFunc(w.debounce, w.callback)
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watch error: %v", err)
		}
	}
}