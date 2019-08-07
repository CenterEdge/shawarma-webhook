package routes

import (
	"fmt"
	"net/http"
)

/*HealthController is an interface that implements mutation method*/
type HealthController interface {
	Health(http.ResponseWriter, *http.Request)
}

/*NewHealthController is a factory method to create an instance of HealthController*/
func NewHealthController() (HealthController, error) {
	return healthController{}, nil
}

type healthController struct {
}

func (controller healthController) Health(writer http.ResponseWriter, request *http.Request) {
	if request.Body != nil {
		defer request.Body.Close()
	}

	if _, err := writer.Write([]byte("Healthy")); err != nil {
		writeError(writer, fmt.Sprintf("Failed to write response: %v", err), http.StatusInternalServerError)
	}
}
