package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func main() {

	if len(os.Args) < 2 {
		log.Fatal("Please provide atleast one directory to watch")
	}
	watcher := NewFileWatcher(os.Args[1:])
	fmt.Printf("Starting file watcher for directories: %v\n", watcher.directories)

	if err := watcher.Start(); err != nil {
		log.Fatal(err)
	}
}

// NewFileWatcher creates a new file watcher instance
func NewFileWatcher(dirs []string) *FileWatcher {
	return &FileWatcher{
		directories: dirs,
		fileStates:  make(map[string]FileInfo),
		interval:    time.Second, // Check every second
	}
}

// FileWatcher manages the watching of directories
type FileWatcher struct {
	directories []string
	fileStates  map[string]FileInfo
	mutex       sync.RWMutex
	interval    time.Duration
}

// FileInfo stores information about a file
type FileInfo struct {
	Size    int64
	ModTime time.Time
	Mode    os.FileMode
	IsDir   bool
}

// getFileInfo retrieves file information
func getFileInfo(path string) (FileInfo, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{
		Size:    stat.Size(),
		ModTime: stat.ModTime(),
		Mode:    stat.Mode(),
		IsDir:   stat.IsDir(),
	}, nil
}

// initialize builds the initial state
func (fw *FileWatcher) initialize() error {
	for _, dir := range fw.directories {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			fileInfo, err := getFileInfo(path)
			if err != nil {
				return err
			}
			fw.mutex.Lock()
			fw.fileStates[path] = fileInfo
			fw.mutex.Unlock()
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Start begins watching the directories
func (fw *FileWatcher) Start() error {
	if err := fw.initialize(); err != nil {
		return err
	}

	ticker := time.NewTicker(fw.interval)
	defer ticker.Stop()

	for range ticker.C {
		fw.checkChanges()
	}
	return nil
}

// checkChanges looks for file system changes
func (fw *FileWatcher) checkChanges() {
	currentFiles := make(map[string]struct{})

	// Check all directories for changes
	for _, dir := range fw.directories {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				// File might have been deleted
				return nil
			}

			currentFiles[path] = struct{}{}
			newInfo, err := getFileInfo(path)
			if err != nil {
				return nil
			}

			fw.mutex.RLock()
			oldInfo, exists := fw.fileStates[path]
			fw.mutex.RUnlock()

			if !exists {
				fmt.Printf("File created: %s\n", path)
			} else {
				// Check for modifications
				if oldInfo.Size != newInfo.Size {
					fmt.Printf("File content modified (size changed): %s\n", path)
				}
				if oldInfo.ModTime != newInfo.ModTime {
					fmt.Printf("File modified (time changed): %s\n", path)
				}
				if oldInfo.Mode != newInfo.Mode {
					fmt.Printf("File attributes modified: %s\n", path)
				}
			}

			fw.mutex.Lock()
			fw.fileStates[path] = newInfo
			fw.mutex.Unlock()

			return nil
		})
	}

	// Check for deleted files
	fw.mutex.Lock()
	for path := range fw.fileStates {
		if _, exists := currentFiles[path]; !exists {
			fmt.Printf("File deleted: %s\n", path)
			delete(fw.fileStates, path)
		}
	}
	fw.mutex.Unlock()
}
