package webhook

import (
	"fmt"

	"github.com/CenterEdge/shawarma-webhook/filewatcher"
	"go.uber.org/zap"
)

type SideCarMonitor struct {
	filePath string
	output chan<- map[string]*SideCar
	watcher filewatcher.FileWatcher
	logger  *zap.Logger
}

func NewSideCarMonitor(filePath string, output chan<- map[string]*SideCar, logger *zap.Logger) (*SideCarMonitor, error) {
	if filePath == "" {
		return nil, fmt.Errorf("filePath is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	monitor := &SideCarMonitor{
		filePath: filePath,
		output:  output,
		logger:  logger,
	}

	watcher, err := filewatcher.NewFileWatcher(filePath, func() {
		logger.Debug("File changed",
			zap.String("file", filePath))

		monitor.processFile()
	}, logger)
	if err != nil {
		return nil, err
	}

	monitor.watcher = watcher

	// Perform initial load
	monitor.processFile()

	return monitor, nil
}

func (monitor *SideCarMonitor) Shutdown() {
	if monitor.watcher != nil {
		monitor.watcher.Close()
		monitor.watcher = nil
	}
}

func (monitor *SideCarMonitor) processFile() {
	data, err := LoadSideCars(monitor.filePath, monitor.logger)
	if err != nil {
		monitor.logger.Error("Invalid side car configuration file",
			zap.Error(err))

		monitor.output <- make(map[string]*SideCar)
	} else {
		monitor.output <- data
	}
}
