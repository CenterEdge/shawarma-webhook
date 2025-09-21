package webhook

import (
	"fmt"

	"github.com/CenterEdge/shawarma-webhook/filewatcher"
	"go.uber.org/zap"
)

type SideCarMonitor struct {
	filePath string
	output   chan map[string]*SideCar
	logger   *zap.Logger
	watcher  filewatcher.FileWatcher
}

func NewSideCarMonitor(filePath string, logger *zap.Logger) (*SideCarMonitor, error) {
	if filePath == "" {
		return nil, fmt.Errorf("filePath is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	monitor := &SideCarMonitor{
		filePath: filePath,
		output:   make(chan map[string]*SideCar),
		logger:   logger,
	}

	return monitor, nil
}

func (monitor *SideCarMonitor) Start() error {
	watcher, err := filewatcher.NewFileWatcher(monitor.filePath, func() {
		monitor.logger.Debug("File changed",
			zap.String("file", monitor.filePath))

		monitor.processFile()
	}, monitor.logger)
	if err != nil {
		return err
	}

	monitor.watcher = watcher

	// Perform initial load
	monitor.processFile()

	return nil
}

func (monitor *SideCarMonitor) GetOutput() <-chan map[string]*SideCar {
	return monitor.output
}

func (monitor *SideCarMonitor) Shutdown() {
	if monitor.watcher != nil {
		monitor.watcher.Close()
		monitor.watcher = nil

		close(monitor.output)
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
