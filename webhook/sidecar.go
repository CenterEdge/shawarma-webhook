package webhook

import (
	"os"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

/*sideCars is an array of named SideCar instances*/
type SideCars struct {
	Sidecars []NamedSideCar `json:"sidecars,omitempty"`
}

/*namedSideCar is a named sidecar to be injected*/
type NamedSideCar struct {
	Name    string  `json:"name"`
	Sidecar SideCar `json:"sidecar"`
}

/*SideCar is the template of the sidecar to be implemented*/
type SideCar struct {
	Containers       []corev1.Container            `json:"containers,omitempty"`
	Volumes          []corev1.Volume               `json:"volumes,omitempty"`
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

func LoadSideCars(sideCarConfigFile string, logger *zap.Logger) (map[string]*SideCar, error) {
	data, err := os.ReadFile(sideCarConfigFile)
	if err != nil {
		return nil, err
	}
	logger.Info("New sideCar configuration", 
		zap.ByteString("data", data))

	var cfg SideCars
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	mapOfSideCar := make(map[string]*SideCar, len(cfg.Sidecars))
	for _, configuration := range cfg.Sidecars {
		mapOfSideCar[configuration.Name] = &configuration.Sidecar
	}

	return mapOfSideCar, nil
}

func (in *SideCar) DeepCopy() *SideCar {
	if in == nil {
		return nil
	}

	out := new(SideCar)
	*out = *in

	if in.Containers != nil {
		in, out := &in.Containers, &out.Containers
		*out = make([]corev1.Container, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}

	if in.Volumes != nil {
		in, out := &in.Volumes, &out.Volumes
		*out = make([]corev1.Volume, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}

	if in.ImagePullSecrets != nil {
		in, out := &in.ImagePullSecrets, &out.ImagePullSecrets
		*out = make([]corev1.LocalObjectReference, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}

	return out
}
