package server

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"registre-admin/internal/types"
)

func handleError(c *gin.Context, status int, msg string) {
	c.JSON(status, types.ErrorResponse{Error: msg})
}

func handleNotFound(c *gin.Context, entity string) {
	handleError(c, http.StatusNotFound, entity+" not found")
}

func handleBadRequest(c *gin.Context, msg string) {
	handleError(c, http.StatusBadRequest, msg)
}

// handleInternalError answers a generic 500 and logs the underlying error
// with the request id, so the cause is diagnosable without ever reaching
// the client.
func handleInternalError(c *gin.Context, err error, msg string) {
	slog.Error(msg,
		"error", err,
		"request_id", RequestID(c),
		"method", c.Request.Method,
		"path", c.FullPath())
	handleError(c, http.StatusInternalServerError, msg)
}

func handleConflict(c *gin.Context, msg string) {
	handleError(c, http.StatusConflict, msg)
}
