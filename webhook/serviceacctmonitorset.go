package webhook

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// ServiceAcctMonitorSet contains a set of ServiceAcctMonitor
type ServiceAcctMonitorSet struct {
	Monitors []*ServiceAcctMonitor
	mutex    sync.Mutex
	logger   *zap.Logger
}

func NewServiceAcctMonitorSet(logger *zap.Logger) *ServiceAcctMonitorSet {
	return &ServiceAcctMonitorSet{
		Monitors: []*ServiceAcctMonitor{},
		logger:   logger,
	}
}

// StopAll service account monitors
func (set *ServiceAcctMonitorSet) StopAll() {
	for _, monitor := range set.Monitors {
		monitor.Stop()
	}

	set.Monitors = []*ServiceAcctMonitor{}
}

// Get a service account monitor, or create if missing
func (set *ServiceAcctMonitorSet) Get(namespace string, serviceAccountName string, timeout time.Duration) (*ServiceAcctMonitor, error) {
	set.mutex.Lock()
	defer set.mutex.Unlock()

	if set.Monitors != nil {
		for _, monitor := range set.Monitors {
			if monitor.Namespace == namespace && monitor.ServiceAccountName == serviceAccountName {
				return monitor, nil
			}
		}
	}

	// Monitor isn't found, so let's create
	monitor, err := NewServiceAcctMonitor(namespace, serviceAccountName, set.logger)
	if err != nil {
		return nil, err
	}

	err = monitor.Start()
	if err != nil {
		return nil, err
	}

	monitor.WaitForFirstUpdate(timeout)

	set.Monitors = append(set.Monitors, monitor)
	return monitor, nil
}
