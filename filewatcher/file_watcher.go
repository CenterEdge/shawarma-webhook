/*
Copyright 2016 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package filewatcher

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// FileWatcher is an interface we use to watch changes in files
type FileWatcher interface {
	Close() error
}

// OSFileWatcher defines a watch over a file
type OSFileWatcher struct {
	file    string
	watcher *fsnotify.Watcher
	logger  *zap.Logger
	// onEvent callback to be invoked after the file being watched changes
	onEvent func()
}

// NewFileWatcher creates a new FileWatcher
func NewFileWatcher(file string, onEvent func(), logger *zap.Logger) (FileWatcher, error) {
	fw := OSFileWatcher{
		file:    file,
		onEvent: onEvent,
		logger:  logger,
	}
	err := fw.watch()
	return fw, err
}

// Close ends the watch
func (f OSFileWatcher) Close() error {
	return f.watcher.Close()
}

// watch creates a fsnotify watcher for a file and create of write events
func (f *OSFileWatcher) watch() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	f.watcher = watcher

	realFile, err := filepath.EvalSymlinks(f.file)
	if err != nil {
		return err
	}

	dir, file := path.Split(f.file)
	go func(file string) {
		for {
			select {
			case event := <-watcher.Events:
				if event.Op&fsnotify.Create == fsnotify.Create ||
					event.Op&fsnotify.Write == fsnotify.Write {
					if finfo, err := os.Lstat(event.Name); err != nil {
						f.logger.Debug("can not lstat file",
							zap.String("filename", event.Name),
							zap.Error(err))
					} else if finfo.Mode()&os.ModeSymlink != 0 {
						if currentRealFile, err := filepath.EvalSymlinks(f.file); err == nil &&
							currentRealFile != realFile {
							f.onEvent()
							realFile = currentRealFile
						}
						continue
					}
					if strings.HasSuffix(event.Name, file) {
						f.onEvent()
					}
				}
			case err := <-watcher.Errors:
				if err != nil {
					f.logger.Error("error watching file",
						zap.String("filename", f.file),
						zap.Error(err))
				}
			}
		}
	}(file)
	return watcher.Add(dir)
}
