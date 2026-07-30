package riskengine

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// RuleWatcher watches for file changes and triggers rule reload
type RuleWatcher struct {
	rulesDir     string
	yamlEngine   *YAMLEngine
	watcher      *fsnotify.Watcher
	stopChan     chan struct{}
	debounceTime time.Duration
	lastReload   time.Time
}

// NewRuleWatcher creates a new rule file watcher
func NewRuleWatcher(rulesDir string, yamlEngine *YAMLEngine) (*RuleWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Watch rules directory
	if err := watcher.Add(rulesDir); err != nil {
		watcher.Close()
		return nil, err
	}

	// Watch subdirectories (if any)
	subdirs, err := filepath.Glob(filepath.Join(rulesDir, "*"))
	if err == nil {
		for _, subdir := range subdirs {
			// Check if it's a directory
			if info, err := os.Stat(subdir); err == nil && info.IsDir() {
				if err := watcher.Add(subdir); err != nil {
					log.Printf("[RuleWatcher] WARNING: Failed to watch subdirectory %s: %v", subdir, err)
				}
			}
		}
	}

	return &RuleWatcher{
		rulesDir:     rulesDir,
		yamlEngine:   yamlEngine,
		watcher:      watcher,
		stopChan:     make(chan struct{}),
		debounceTime: 1 * time.Second, // Debounce multiple file changes
		lastReload:   time.Now(),
	}, nil
}

// Start starts watching for file changes
func (w *RuleWatcher) Start() {
	log.Printf("[RuleWatcher] Starting file watcher for %s", w.rulesDir)

	go func() {
		for {
			select {
			case event, ok := <-w.watcher.Events:
				if !ok {
					return
				}

				// Handle file changes
				if w.shouldReload(event) {
					w.reloadRules()
				}

			case err, ok := <-w.watcher.Errors:
				if !ok {
					return
				}
				log.Printf("[RuleWatcher] Error: %v", err)

			case <-w.stopChan:
				log.Printf("[RuleWatcher] Stopping file watcher")
				return
			}
		}
	}()
}

// shouldReload checks if the event should trigger a reload
func (w *RuleWatcher) shouldReload(event fsnotify.Event) bool {
	// Only reload on Write or Create events
	if event.Op&fsnotify.Write == 0 && event.Op&fsnotify.Create == 0 {
		return false
	}

	// Only reload for YAML files
	ext := filepath.Ext(event.Name)
	if ext != ".yaml" && ext != ".yml" {
		return false
	}

	// Debounce: Don't reload too frequently
	if time.Since(w.lastReload) < w.debounceTime {
		log.Printf("[RuleWatcher] Debouncing reload (last reload %v ago)", time.Since(w.lastReload))
		return false
	}

	log.Printf("[RuleWatcher] File changed: %s (op: %v)", event.Name, event.Op)
	return true
}

// reloadRules triggers a rule reload
func (w *RuleWatcher) reloadRules() {
	log.Printf("[RuleWatcher] ======================================")
	log.Printf("[RuleWatcher] Reloading rules...")

	start := time.Now()

	// Clear CEL cache before reload
	if w.yamlEngine.celCompiler != nil {
		w.yamlEngine.celCompiler.ClearCache()
	}

	// Reload rules
	if err := w.yamlEngine.Reload(); err != nil {
		log.Printf("[RuleWatcher] ❌ ERROR: Failed to reload rules: %v", err)
		return
	}

	duration := time.Since(start)
	w.lastReload = time.Now()

	log.Printf("[RuleWatcher] ✅ Rules reloaded successfully in %v", duration)
	log.Printf("[RuleWatcher] Loaded %d rules", len(w.yamlEngine.GetRules()))
	log.Printf("[RuleWatcher] ======================================")
}

// Stop stops the file watcher
func (w *RuleWatcher) Stop() {
	close(w.stopChan)
	if w.watcher != nil {
		w.watcher.Close()
	}
}

