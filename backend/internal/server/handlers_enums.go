package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"registre-admin/internal/repository"
)

// EnumsHandler returns all reference table data.
//
// @Summary     Get all enums
// @Description Returns all reference data
// @Tags        system
// @Produce     json
// @Success     200 {object} repository.EnumsData
// @Security BearerAuth
// @Router      /enums [get]
func EnumsHandler(codeTableRepo repository.CodeTableDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, err := codeTableRepo.LoadAllEnums()
		if err != nil {
			handleInternalError(c, err, "Failed to load enums")
			return
		}
		c.JSON(http.StatusOK, data)
	}
}
