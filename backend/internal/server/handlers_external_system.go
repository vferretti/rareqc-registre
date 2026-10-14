package server

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"registre-admin/internal/repository"
	"registre-admin/internal/types"
)

// ListExternalSystemsHandler returns all external systems with their reference status.
//
// @Summary     List external systems
// @Description Returns all external systems with a flag indicating if they are referenced
// @Tags        admin
// @Produce     json
// @Success     200 {array} repository.ExternalSystemResponse
// @Security BearerAuth
// @Router      /external-systems [get]
func ListExternalSystemsHandler(repo repository.ExternalSystemDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		systems, err := repo.List()
		if err != nil {
			handleInternalError(c, err, "Failed to list external systems")
			return
		}
		c.JSON(http.StatusOK, systems)
	}
}

// ExternalSystemRequest represents the request body for creating or updating an external system.
type ExternalSystemRequest struct {
	Name    string `json:"name" binding:"required"`
	TitleFr string `json:"title_fr" binding:"required"`
	TitleEn string `json:"title_en" binding:"required"`
}

// CreateExternalSystemHandler creates a new external system.
//
// @Summary     Create an external system
// @Description Adds a new external system
// @Tags        admin
// @Accept      json
// @Produce     json
// @Param       body body ExternalSystemRequest true "External system"
// @Success     201 {object} types.ExternalSystem
// @Failure     400 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /external-systems [post]
func CreateExternalSystemHandler(repo repository.ExternalSystemDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ExternalSystemRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			handleBadRequest(c, "Invalid request body")
			return
		}
		if req.Name == "" || req.TitleFr == "" || req.TitleEn == "" {
			handleBadRequest(c, "name, title_fr, and title_en are required")
			return
		}

		system := types.ExternalSystem{Name: req.Name, TitleFr: req.TitleFr, TitleEn: req.TitleEn}
		if err := repo.Create(&system); err != nil {
			handleConflict(c, err.Error())
			return
		}
		c.JSON(http.StatusCreated, system)
	}
}

// UpdateExternalSystemHandler updates an existing external system.
//
// @Summary     Update an external system
// @Description Updates name, title_fr, and title_en of an external system.
// @Tags        admin
// @Accept      json
// @Produce     json
// @Param       id   path int true "External system ID"
// @Param       body body ExternalSystemRequest true "Updated external system"
// @Success     200 {object} object{status=string}
// @Failure     400 {object} types.ErrorResponse
// @Failure     409 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /external-systems/{id} [put]
func UpdateExternalSystemHandler(repo repository.ExternalSystemDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscanf(c.Param("id"), "%d", &id); err != nil {
			handleBadRequest(c, "Invalid ID")
			return
		}

		var req ExternalSystemRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			handleBadRequest(c, "Invalid request body")
			return
		}
		if req.Name == "" || req.TitleFr == "" || req.TitleEn == "" {
			handleBadRequest(c, "name, title_fr, and title_en are required")
			return
		}

		if err := repo.Update(id, req.Name, req.TitleFr, req.TitleEn); err != nil {
			handleConflict(c, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "updated"})
	}
}

// DeleteExternalSystemHandler deletes an external system if it's not referenced.
//
// @Summary     Delete an external system
// @Description Removes an external system. Fails if it has associated external IDs.
// @Tags        admin
// @Produce     json
// @Param       id path int true "External system ID"
// @Success     204
// @Failure     409 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /external-systems/{id} [delete]
func DeleteExternalSystemHandler(repo repository.ExternalSystemDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		var id int
		if _, err := fmt.Sscanf(c.Param("id"), "%d", &id); err != nil {
			handleBadRequest(c, "Invalid ID")
			return
		}

		if err := repo.Delete(id); err != nil {
			handleConflict(c, err.Error())
			return
		}
		c.Status(http.StatusNoContent)
	}
}
