package routes

import (
	"net/http"

	"go.uber.org/zap"
)

/*HealthController is an interface that implements mutation method*/
type HealthController interface {
	Health(http.ResponseWriter, *http.Request)
}

/*NewHealthController is a factory method to create an instance of HealthController*/
func NewHealthController(logger *zap.Logger) (HealthController, error) {
	return healthController{logger: logger}, nil
}

type healthController struct {
	logger *zap.Logger
}

func (controller healthController) Health(writer http.ResponseWriter, request *http.Request) {
	if request.Body != nil {
		defer request.Body.Close()
	}

	if _, err := writer.Write([]byte("Healthy")); err != nil {
		writeError(writer, controller.logger, "Failed to write response", err, http.StatusInternalServerError)
	}
}
