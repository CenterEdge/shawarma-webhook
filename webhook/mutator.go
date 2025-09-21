package webhook

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	v1 "k8s.io/api/admission/v1"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	runtimeScheme    = runtime.NewScheme()
	codecs           = serializer.NewCodecFactory(runtimeScheme)
	deserializer     = codecs.UniversalDeserializer()
	systemNameSpaces = []string{
		metav1.NamespaceSystem,
		metav1.NamespacePublic,
	}
)

const (
	sideCarNameSpace                 = "shawarma.centeredge.io/"
	injectAnnotation                 = "service-name"
	labelInjectAnnotation            = "service-labels"
	imageAnnotation                  = "image"
	statusAnnotation                 = "status"
	sideCarInjectionAnnotation       = sideCarNameSpace + injectAnnotation
	sideCarLabelInjectionAnnotation  = sideCarNameSpace + labelInjectAnnotation
	sideCarInjectionStatusAnnotation = sideCarNameSpace + statusAnnotation
	sideCarInjectionImageAnnotation  = sideCarNameSpace + imageAnnotation
	injectedValue                    = "injected"
	sideCarName                      = "shawarma"
	sideCarWithTokenName             = "shawarma-withtoken"
)

// unversionedAdmissionReview is used to decode both v1 and v1beta1 AdmissionReview types.
type unversionedAdmissionReview struct {
	v1.AdmissionReview
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value any         `json:"value,omitempty"`
}

type MutatorConfig struct {
	SideCarConfigFile       string
	ShawarmaImage           string
	NativeSidecars      	bool
	ShawarmaServiceAcctName string
	ShawarmaSecretTokenName string
	Logger					*zap.Logger
}

/*Mutator is the interface for mutating webhook*/
type Mutator struct {
	sideCars                atomic.Value
	sideCarMonitor          *SideCarMonitor

	shawarmaImage           string
	nativeSidecars          bool
	shawarmaServiceAcctName string
	shawarmaSecretTokenName string
	serviceAcctMonitors     *ServiceAcctMonitorSet
	Logger                  *zap.Logger
}

func Init() {
	utilruntime.Must(v1.AddToScheme(runtimeScheme))
	utilruntime.Must(v1beta1.AddToScheme(runtimeScheme))
}

func NewMutator(config *MutatorConfig) (*Mutator, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	if config.SideCarConfigFile == "" {
		return nil, fmt.Errorf("config.SideCarConfigFile is required")
	}
	if config.ShawarmaImage == "" {
		return nil, fmt.Errorf("config.ShawarmaImage is required")
	}
	if config.Logger == nil {
		return nil, fmt.Errorf("config.Logger is required")
	}

	monitor, err := NewSideCarMonitor(config.SideCarConfigFile, config.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create side car monitor: %w", err)
	}

	mutator := &Mutator{
		sideCars:                atomic.Value{},
		sideCarMonitor:          monitor,
		shawarmaImage:           config.ShawarmaImage,
		nativeSidecars:          config.NativeSidecars,
		shawarmaServiceAcctName: config.ShawarmaServiceAcctName,
		shawarmaSecretTokenName: config.ShawarmaSecretTokenName,
		serviceAcctMonitors:     NewServiceAcctMonitorSet(config.Logger),
		Logger:                  config.Logger,
	}

	go func() {
		for sideCarConfig := range monitor.GetOutput() {
			mutator.sideCars.Store(sideCarConfig)

			mutator.Logger.Info("Sidecar config loaded")
		}
	}()

	monitor.Start()
	
	return mutator, nil
}

// Shutdown the mutator
func (mutator *Mutator) Shutdown() {
	if mutator.serviceAcctMonitors != nil {
		mutator.serviceAcctMonitors.StopAll()
		mutator.serviceAcctMonitors = nil
	}

	if mutator.sideCarMonitor != nil {
		mutator.sideCarMonitor.Shutdown()
		mutator.sideCarMonitor = nil
	}
}

func (mutator *Mutator) GetSideCars() map[string]*SideCar {
	val := mutator.sideCars.Load()
	if val == nil {
		return make(map[string]*SideCar)
	}
	sideCars, ok := val.(map[string]*SideCar)
	if !ok {
		return make(map[string]*SideCar)
	}
	return sideCars
}

/*Mutate function performs the actual mutation of pod spec*/
func (mutator *Mutator) Mutate(req []byte) ([]byte, error) {
	admissionReviewResp := v1.AdmissionReview{}
	admissionReviewReq := v1.AdmissionRequest{}
	var admissionResponse *v1.AdmissionResponse

	// Both v1 and v1beta1 AdmissionReview types are exactly the same, so the v1beta1 type can
	// be decoded into the v1 type. However the runtime codec's decoder guesses which type to
	// decode into by type name if an Object's TypeMeta isn't set. By setting TypeMeta of an
	// unregistered type to the v1 GVK, the decoder will coerce a v1beta1 AdmissionReview to v1.
	// The actual AdmissionReview GVK will be used to write a typed response in case the
	// webhook config permits multiple versions, otherwise this response will fail.
	ar := unversionedAdmissionReview{}
	// avoid an extra copy
	ar.Request = &admissionReviewReq
	ar.SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("AdmissionReview"))
	_, actualGVK, err := deserializer.Decode(req, nil, &ar)

	if err == nil && ar.Request != nil {
		admissionResponse = mutate(&admissionReviewReq, mutator)
	} else {
		message := "Failed to decode request"

		if err != nil {
			message = fmt.Sprintf("message: %s err: %v", message, err)
		}

		admissionResponse = &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: message,
			},
		}
	}

	admissionReviewResp.Response = admissionResponse

	// Default to a v1 AdmissionReview, otherwise the API server may not recognize the request
	// if multiple AdmissionReview versions are permitted by the webhook config.
	if actualGVK == nil || *actualGVK == (schema.GroupVersionKind{}) {
		admissionReviewResp.SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("AdmissionReview"))
	} else {
		admissionReviewResp.SetGroupVersionKind(*actualGVK)
	}

	return json.Marshal(admissionReviewResp)
}

func mutate(req *v1.AdmissionRequest, mutator *Mutator) *v1.AdmissionResponse {
	mutator.Logger.Info("AdmissionReview",
		zap.Any("kind", req.Kind),
		zap.String("namespace", req.Namespace),
		zap.String("name", req.Name),
		zap.String("uid", string(req.UID)),
		zap.String("operation", string(req.Operation)),
		zap.Any("userInfo", req.UserInfo))

	pod, err := unMarshall(req)
	if err != nil {
		return mutator.errorResponse(req.UID, err)
	}

	if sideCarNames, ok := shouldMutate(systemNameSpaces, &pod.ObjectMeta, req.Namespace, mutator); ok {
		annotations := map[string]string{sideCarInjectionStatusAnnotation: injectedValue}
		patchBytes, err := createPatch(&pod, req.Namespace, sideCarNames, mutator, annotations)
		if err != nil {
			return mutator.errorResponse(req.UID, err)
		}

		mutator.Logger.Info("AdmissionResponse: Patch",
			zap.ByteString("patch", patchBytes))
		pt := v1.PatchTypeJSONPatch
		return &v1.AdmissionResponse{
			UID:       req.UID,
			Allowed:   true,
			Patch:     patchBytes,
			PatchType: &pt,
		}
	}

	return &v1.AdmissionResponse{
		Allowed: true,
	}
}

func (mutator *Mutator) errorResponse(uid types.UID, err error) *v1.AdmissionResponse {
	mutator.Logger.Error("AdmissionReview failed",
		zap.String("uid", string(uid)), 
		zap.Error(err))

	return &v1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}

func unMarshall(req *v1.AdmissionRequest) (corev1.Pod, error) {
	var pod corev1.Pod
	err := json.Unmarshal(req.Object.Raw, &pod)
	return pod, err
}

func shouldMutate(ignoredList []string, metadata *metav1.ObjectMeta, namespace string, mutator *Mutator) ([]string, bool) {
	podName := metadata.Name
	if podName == "" {
		podName = metadata.GenerateName
	}

	logger := mutator.Logger.With(
		zap.String("podName", podName), 
		zap.String("namespace", namespace))

	for _, namespace := range ignoredList {
		if metadata.Namespace == namespace {
			logger.Info("Skipping mutation for pod in special namespace")

			return nil, false
		}
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	if status, ok := annotations[sideCarInjectionStatusAnnotation]; ok && strings.ToLower(status) == injectedValue {
		logger.Info("Skipping mutation for pod. Has been mutated already")

		return nil, false
	}

	selectedSideCarName := sideCarName
	if mutator.shawarmaSecretTokenName != "" || mutator.shawarmaServiceAcctName != "" {
		// We need to attach a token, use the alternate side car format
		selectedSideCarName = sideCarWithTokenName
	}

	if serviceName, ok := annotations[sideCarInjectionAnnotation]; ok {
		if len(serviceName) > 0 {
			logger.Info("shawarma injection for pod",
				zap.String("serviceName", serviceName),
				zap.String("sidecar", selectedSideCarName))

			return []string{selectedSideCarName}, true
		}
	}

	if serviceLabels, ok := annotations[sideCarLabelInjectionAnnotation]; ok {
		if len(serviceLabels) > 0 {
			logger.Info("shawarma injection for pod",
				zap.String("serviceLabels", serviceLabels),
				zap.String("sidecar", selectedSideCarName))
			return []string{selectedSideCarName}, true
		}
	}

	logger.Info("Skipping mutation for pod. No action required")
	return nil, false
}

func createPatch(pod *corev1.Pod, namespace string, sideCarNames []string, mutator *Mutator, annotations map[string]string) ([]byte, error) {

	var patch []patchOperation
	var containers []corev1.Container
	var volumes []corev1.Volume
	var imagePullSecrets []corev1.LocalObjectReference

	// Check for image override in annotations
	shawarmaImage := mutator.shawarmaImage
	existingAnnotations := pod.ObjectMeta.GetAnnotations()
	if existingAnnotations != nil {
		if image, ok := existingAnnotations[sideCarInjectionImageAnnotation]; ok {
			mutator.Logger.Info("Overriding Shawarma image", 
				zap.String("image", image))

			shawarmaImage = image
		}
	}

	// Handle the secret name
	secretName := mutator.shawarmaSecretTokenName
	if secretName == "" && mutator.shawarmaServiceAcctName != "" {
		// Get the secret name from the service account
		monitor, err := mutator.serviceAcctMonitors.Get(namespace, mutator.shawarmaServiceAcctName, time.Second*1)
		if err != nil {
			return nil, err
		}

		secretName = monitor.SecretName
		if secretName == "" {
			return nil, fmt.Errorf("cannot find secret for service account %s/%s", namespace, mutator.shawarmaServiceAcctName)
		} else {
			mutator.Logger.Debug("Using service token for service account",
				zap.String("secretName", secretName),
				zap.String("namespace", namespace),
				zap.String("serviceAccountName", mutator.shawarmaServiceAcctName))
		}
	}

	// Atomic get of the current side cars to prevent errors if they mutate while we're processing
	sideCars := mutator.GetSideCars()
	for _, name := range sideCarNames {
		if sideCarSrc, ok := sideCars[name]; ok {
			sideCar := sideCarSrc.DeepCopy()

			for i, container := range sideCar.Containers {
				sideCar.Containers[i].Image = strings.ReplaceAll(container.Image, "|SHAWARMA_IMAGE|", shawarmaImage)

				if mutator.nativeSidecars {
					// Set restart policy to Always so it's a sidecar and not a normal init container
					restartPolicy := corev1.ContainerRestartPolicyAlways
					sideCar.Containers[i].RestartPolicy = &restartPolicy
				}
			}

			// Apply the configured volumes
			for i, volume := range sideCar.Volumes {
				if volume.Secret != nil {
					sideCar.Volumes[i].Secret.SecretName = strings.ReplaceAll(volume.Secret.SecretName, "|SHAWARMA_TOKEN_NAME|", secretName)
				}
			}

			containers = append(containers, sideCar.Containers...)
			volumes = append(volumes, sideCar.Volumes...)
			imagePullSecrets = append(imagePullSecrets, sideCar.ImagePullSecrets...)
		} else {
			return nil, fmt.Errorf("did not find one or more sidecars to inject %v", sideCarNames)
		}
	}

	if mutator.nativeSidecars {
		patch = append(patch, addContainer(pod.Spec.InitContainers, containers, "/spec/initContainers")...)
	} else {
		patch = append(patch, addContainer(pod.Spec.Containers, containers, "/spec/containers")...)
	}

	patch = append(patch, addVolume(pod.Spec.Volumes, volumes, "/spec/volumes")...)
	patch = append(patch, addImagePullSecrets(pod.Spec.ImagePullSecrets, imagePullSecrets, "/spec/imagePullSecrets")...)
	patch = append(patch, updateAnnotation(pod.Annotations, annotations)...)

	return json.Marshal(patch)
}

func addContainer(target, added []corev1.Container, basePath string) []patchOperation {
	var patch []patchOperation
	first := len(target) == 0
	var value interface{}
	for _, add := range added {
		value = add
		path := basePath
		if first {
			first = false
			value = []corev1.Container{add}
		} else {
			path = path + "/-"
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}

func addVolume(target, added []corev1.Volume, basePath string) []patchOperation {
	var patch []patchOperation
	first := len(target) == 0
	var value interface{}
	for _, add := range added {
		value = add
		path := basePath
		if first {
			first = false
			value = []corev1.Volume{add}
		} else {
			path = path + "/-"
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}

func addImagePullSecrets(target, added []corev1.LocalObjectReference, basePath string) []patchOperation {
	var patch []patchOperation
	first := len(target) == 0
	var value interface{}
	for _, add := range added {
		value = add
		path := basePath
		if first {
			first = false
			value = []corev1.LocalObjectReference{add}
		} else {
			path = path + "/-"
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}

func updateAnnotation(target map[string]string, added map[string]string) []patchOperation {
	var patch []patchOperation
	if target == nil {
		target = map[string]string{}
	}
	for key, value := range added {
		keyEscaped := strings.Replace(key, "/", "~1", -1)

		_, ok := target[key]
		if ok {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  "/metadata/annotations/" + keyEscaped,
				Value: value,
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "add",
				Path:  "/metadata/annotations/" + keyEscaped,
				Value: value,
			})
		}
	}
	return patch
}
