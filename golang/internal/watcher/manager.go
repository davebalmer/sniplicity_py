package watcher

import (
	"log"
	"sync"
)

// Manager handles starting, stopping, and switching file watchers
type Manager struct {
	mu      sync.Mutex
	watcher *Watcher
	callback func()
}

// NewManager creates a new watcher manager
func NewManager(callback func()) *Manager {
	return &Manager{
		callback: callback,
	}
}

// Start starts watching the given directory
func (m *Manager) Start(watchDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Stop existing watcher if any
	if m.watcher != nil {
		m.watcher.Close()
		m.watcher = nil
	}
	
	// Don't start if no directory provided
	if watchDir == "" {
		return nil
	}
	
	// Create new watcher
	w, err := New(watchDir, m.callback)
	if err != nil {
		return err
	}
	
	m.watcher = w
	log.Printf("File watcher started for: %s", watchDir)
	return nil
}

// Switch switches to watching a new directory
func (m *Manager) Switch(newDir string) error {
	return m.Start(newDir) // Start() already handles stopping the old one
}

// Stop stops the current watcher
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.watcher != nil {
		m.watcher.Close()
		m.watcher = nil
		log.Printf("File watcher stopped")
	}
}

// IsRunning returns true if a watcher is currently active
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.watcher != nil
}