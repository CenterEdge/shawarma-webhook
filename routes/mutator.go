package routes

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/CenterEdge/shawarma-webhook/webhook"
	"go.uber.org/zap"
	"sigs.k8s.io/yaml"
)

/*SideCars is an array of named SideCar instances*/
type SideCars struct {
	Sidecars []SideCar `json:"sidecars"`
}

/*SideCar is a named sidecar to be injected*/
type SideCar struct {
	Name    string          `json:"name"`
	Sidecar webhook.SideCar `json:"sidecar"`
}

func loadConfig(sideCarConfigFile string, logger *zap.Logger) (map[string]webhook.SideCar, error) {
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

	mapOfSideCar := make(map[string]webhook.SideCar)
	for _, configuration := range cfg.Sidecars {
		mapOfSideCar[configuration.Name] = configuration.Sidecar
	}

	return mapOfSideCar, nil
}

/*MutatorController is an interface that implements mutation method*/
type MutatorController interface {
	Shutdown()
	Mutate(http.ResponseWriter, *http.Request)
}

/*NewMutatorController is a factory method to create an instance of MutatorController*/
func NewMutatorController(sideCarConfigFile string, shawarmaImage string, shawarmaServiceAcctName string, shawarmaSecretTokenName string, logger *zap.Logger) (MutatorController, error) {
	mapOfSideCars, err := loadConfig(sideCarConfigFile, logger)
	if mapOfSideCars != nil {
		mutator := webhook.Mutator{
			SideCars:                mapOfSideCars,
			ShawarmaImage:           shawarmaImage,
			ShawarmaServiceAcctName: shawarmaServiceAcctName,
			ShawarmaSecretTokenName: shawarmaSecretTokenName,
			ServiceAcctMonitors:     webhook.NewServiceAcctMonitorSet(logger),
			Logger: logger,
		}

		return mutatorController{mutator: mutator}, nil
	}
	return nil, err
}

type mutatorController struct {
	mutator webhook.Mutator
}

func (controller mutatorController) Shutdown() {
	controller.mutator.Shutdown()
}

func (controller mutatorController) Mutate(writer http.ResponseWriter, request *http.Request) {
	body, err := controller.readRequestBody(request)
	if err != nil {
		writeError(writer, controller.mutator.Logger, "Bad request", err, http.StatusBadRequest)
		return
	}

	resp, err := controller.mutator.Mutate(body)
	if err != nil {
		writeError(writer, controller.mutator.Logger, "Failed to process request", err, http.StatusInternalServerError)
		return
	}

	if _, err := writer.Write(resp); err != nil {
		writeError(writer, controller.mutator.Logger, "Failed to write response", err, http.StatusInternalServerError)
	}
}

func (controller mutatorController) readRequestBody(r *http.Request) ([]byte, error) {
	var body []byte

	if r.Body != nil {
		defer r.Body.Close()
		if data, err := io.ReadAll(r.Body); err != nil {
			io.Copy(io.Discard, r.Body)
		} else {
			body = data
		}
	}

	if len(body) == 0 {
		return nil, errors.New("body of the request is empty")
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		return nil, fmt.Errorf("received Content-Type=%s, Expected Content-Type is 'application/json'", contentType)
	}

	controller.mutator.Logger.Debug("Request received", 
		zap.ByteString("body", body))
	return body, nil
}
