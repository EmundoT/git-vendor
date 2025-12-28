package core

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchConfig watches for changes to vendor.yml and triggers sync
func (s *VendorSyncer) WatchConfig(callback func() error) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	configPath := s.configStore.Path()

	// Add config file to watcher
	if err := watcher.Add(configPath); err != nil {
		return fmt.Errorf("failed to watch %s: %w", configPath, err)
	}

	// Also watch the directory for when file is deleted/recreated
	configDir := filepath.Dir(configPath)
	if err := watcher.Add(configDir); err != nil {
		return fmt.Errorf("failed to watch directory %s: %w", configDir, err)
	}

	fmt.Printf("👁 Watching for changes to %s...\n", configPath)
	fmt.Println("Press Ctrl+C to stop")

	// Debounce rapid changes
	var debounceTimer *time.Timer
	const debounceDelay = 1 * time.Second

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// Only process write/create events for the config file
			if event.Name != configPath {
				continue
			}

			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				// Debounce: reset timer on each event
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				debounceTimer = time.AfterFunc(debounceDelay, func() {
					fmt.Printf("\n📝 Detected change to %s\n", filepath.Base(configPath))

					// Check if file still exists
					if _, err := os.Stat(configPath); err != nil {
						s.ui.ShowWarning("File Not Found", "Config file was deleted or is inaccessible")
						return
					}

					// Trigger callback
					if err := callback(); err != nil {
						s.ui.ShowError("Sync Failed", err.Error())
					} else {
						s.ui.ShowSuccess("Sync completed")
					}

					fmt.Println("\n👁 Still watching for changes...")
				})
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Printf("Watch error: %v\n", err)
		}
	}
}
