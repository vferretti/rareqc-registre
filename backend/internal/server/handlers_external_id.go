package server

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"registre-admin/internal/repository"
	_ "registre-admin/internal/types" // for swagger annotations
)

// ListParticipantExternalIDsHandler returns all external IDs for a participant.
//
// @Summary     List external IDs for a participant
// @Description Returns all external system identifiers for the given participant
// @Tags        external-ids
// @Produce     json
// @Param       id path int true "Participant ID"
// @Success     200 {array}  repository.ExternalIDResponse
// @Failure     400 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /participants/{id}/external-ids [get]
func ListParticipantExternalIDsHandler(extIDRepo repository.ExternalIDDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		var participantID int
		if _, err := fmt.Sscanf(c.Param("id"), "%d", &participantID); err != nil {
			handleBadRequest(c, "Invalid participant ID")
			return
		}

		extIDs, err := extIDRepo.ListByParticipant(participantID)
		if err != nil {
			handleInternalError(c, err, "Failed to fetch external IDs")
			return
		}

		c.JSON(http.StatusOK, extIDs)
	}
}
