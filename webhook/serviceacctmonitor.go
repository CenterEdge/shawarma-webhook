package webhook

import (
	"time"

	"go.uber.org/zap"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// ServiceAcctMonitor observes a service account and keeps SecretName up to date
type ServiceAcctMonitor struct {
	Namespace          string
	ServiceAccountName string
	SecretName         string
	hasFirstUpdate     bool
	firstUpdate        chan struct{}
	stop               chan struct{}
	logger             *zap.Logger
}

var (
	k8sClient *kubernetes.Clientset
)

// NewServiceAcctMonitor Create a new service account monitor
func NewServiceAcctMonitor(namespace string, serviceAccountName string, logger *zap.Logger) (*ServiceAcctMonitor, error) {
	monitor := ServiceAcctMonitor{
		Namespace:          namespace,
		ServiceAccountName: serviceAccountName,
		firstUpdate:        make(chan struct{}, 1),
		stop:               make(chan struct{}),
		logger:             logger.With(
			zap.String("namespace", namespace),
			zap.String("serviceAccountName", serviceAccountName)),
	}

	return &monitor, nil
}

func extractSecretName(serviceAccount *v1.ServiceAccount) string {
	if serviceAccount == nil || serviceAccount.Secrets == nil {
		return ""
	}

	for _, secret := range serviceAccount.Secrets {
		if secret.Name != "" {
			return secret.Name
		}
	}

	return ""
}

// Start the service account monitor
func (monitor *ServiceAcctMonitor) Start() error {
	go func() {
		watchList := cache.NewListWatchFromClient(
			k8sClient.CoreV1().RESTClient(),
			"serviceaccounts",
			monitor.Namespace,
			fields.SelectorFromSet(fields.Set{
				"metadata.name": monitor.ServiceAccountName,
			}),
		)

		_, controller := cache.NewInformer(
			watchList,
			&v1.ServiceAccount{},
			time.Second*0,
			cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					serviceAccount := obj.(*v1.ServiceAccount)

					monitor.logger.Debug("service account added")

					monitor.updateSecretName(extractSecretName(serviceAccount))
				},
				DeleteFunc: func(obj interface{}) {
					monitor.logger.Debug("service account deleted")

					monitor.updateSecretName("")
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					serviceAccount := newObj.(*v1.ServiceAccount)

					monitor.logger.Debug("service account changed")

					monitor.updateSecretName(extractSecretName(serviceAccount))
				},
			},
		)

		controller.Run(monitor.stop)

		close(monitor.firstUpdate)
		close(monitor.stop)
	}()

	return nil
}

// Stop the monitor
func (monitor *ServiceAcctMonitor) Stop() {
	monitor.stop <- struct{}{}
}

// WaitForFirstUpdate returns true if the first update was received, false if timed out
func (monitor *ServiceAcctMonitor) WaitForFirstUpdate(timeout time.Duration) bool {
	if monitor.hasFirstUpdate {
		return true
	}

	monitor.logger.Info("Waiting for first update for service account")

	select {
	case <-monitor.firstUpdate:
		monitor.logger.Debug("Got first update for service account")
		return true
	case <-time.After(timeout):
		monitor.logger.Warn("Timeout waiting for first update for service account")
		return false
	}
}

func (monitor *ServiceAcctMonitor) updateSecretName(secretName string) {
	monitor.SecretName = secretName

	if !monitor.hasFirstUpdate {
		monitor.hasFirstUpdate = true

		select {
		case monitor.firstUpdate <- struct{}{}:
		default:
		}
	}
}

// InitializeServiceAcctMonitor with Kubernetes configuration
func InitializeServiceAcctMonitor() error {
	if k8sClient != nil {
		return nil
	}

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	k8sClient = clientset

	return nil
}
