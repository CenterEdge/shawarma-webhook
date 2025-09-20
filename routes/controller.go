package routes

import (
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

func writeError(writer http.ResponseWriter, logger *zap.Logger, message string, err error, status int) {
	logger.Error(message, zap.Error(err))
	http.Error(writer, fmt.Sprintf("%s: %v", message, err), status)
}