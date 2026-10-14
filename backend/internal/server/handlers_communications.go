package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"registre-admin/internal/repository"
	"registre-admin/internal/types"
)

// CreateCommunicationRequest represents the payload for creating a communication.
type CreateCommunicationRequest struct {
	ContactID         int     `json:"contact_id" binding:"required"`
	ContactValue      *string `json:"contact_value"`
	MethodCode        string  `json:"method_code" binding:"required"`
	SubjectCode       string  `json:"subject_code" binding:"required"`
	OutcomeCode       *string `json:"outcome_code"`
	CommunicationDate string  `json:"communication_date" binding:"required"`
	Comment           *string `json:"comment"`
}

// UpdateCommunicationRequest represents the payload for updating a communication.
type UpdateCommunicationRequest struct {
	ContactID         int     `json:"contact_id" binding:"required"`
	ContactValue      *string `json:"contact_value"`
	MethodCode        string  `json:"method_code" binding:"required"`
	SubjectCode       string  `json:"subject_code" binding:"required"`
	OutcomeCode       *string `json:"outcome_code"`
	CommunicationDate string  `json:"communication_date" binding:"required"`
	Comment           *string `json:"comment"`
}

// ListParticipantCommunicationsHandler returns all communications for a participant.
//
// @Summary     List communications for a participant
// @Description Returns all communication records for the given participant
// @Tags        communications
// @Produce     json
// @Param       id path int true "Participant ID"
// @Success     200 {array}  repository.CommunicationResponse
// @Failure     400 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /participants/{id}/communications [get]
func ListParticipantCommunicationsHandler(commRepo repository.CommunicationDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		participantID, ok := parsePathID(c, "id", "participant")
		if !ok {
			return
		}

		comms, err := commRepo.ListByParticipant(participantID)
		if err != nil {
			handleInternalError(c, err, "Failed to fetch communications")
			return
		}

		c.JSON(http.StatusOK, comms)
	}
}

// CreateParticipantCommunicationHandler creates a communication for a participant.
//
// @Summary     Record a communication for a participant
// @Description Creates a new communication record (phone call or email attempt)
// @Tags        communications
// @Accept      json
// @Produce     json
// @Param       id   path int                         true "Participant ID"
// @Param       body body CreateCommunicationRequest   true "Communication data"
// @Success     201 {object} types.Communication
// @Failure     400 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /participants/{id}/communications [post]
func CreateParticipantCommunicationHandler(commRepo repository.CommunicationDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		participantID, ok := parsePathID(c, "id", "participant")
		if !ok {
			return
		}

		var req CreateCommunicationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			handleBadRequest(c, "Invalid request body")
			return
		}

		commDate, err := time.Parse("2006-01-02", req.CommunicationDate)
		if err != nil {
			handleBadRequest(c, "Invalid date format (expected YYYY-MM-DD)")
			return
		}

		comm := types.Communication{
			ParticipantID:     participantID,
			ContactID:         req.ContactID,
			ContactValue:      req.ContactValue,
			MethodCode:        req.MethodCode,
			SubjectCode:       req.SubjectCode,
			OutcomeCode:       req.OutcomeCode,
			CommunicationDate: commDate,
			Author:            getAuthor(c),
			Comment:           req.Comment,
		}

		if err := commRepo.Create(&comm); err != nil {
			handleInternalError(c, err, "Failed to create communication")
			return
		}

		c.JSON(http.StatusCreated, comm)
	}
}

// UpdateCommunicationHandler updates an existing communication.
//
// @Summary     Update a communication
// @Description Updates an existing communication record
// @Tags        communications
// @Accept      json
// @Produce     json
// @Param       communicationId path int                         true "Communication ID"
// @Param       body            body UpdateCommunicationRequest   true "Updated communication data"
// @Success     200 {object} types.Communication
// @Failure     400 {object} types.ErrorResponse
// @Failure     404 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /communications/{communicationId} [put]
func UpdateCommunicationHandler(commRepo repository.CommunicationDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		communicationID, ok := parsePathID(c, "communicationId", "communication")
		if !ok {
			return
		}

		comm, err := commRepo.FindByID(communicationID)
		if err != nil {
			handleNotFound(c, "Communication")
			return
		}

		var req UpdateCommunicationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			handleBadRequest(c, "Invalid request body")
			return
		}

		commDate, err := time.Parse("2006-01-02", req.CommunicationDate)
		if err != nil {
			handleBadRequest(c, "Invalid date format (expected YYYY-MM-DD)")
			return
		}

		comm.ContactID = req.ContactID
		comm.ContactValue = req.ContactValue
		comm.MethodCode = req.MethodCode
		comm.SubjectCode = req.SubjectCode
		comm.OutcomeCode = req.OutcomeCode
		comm.CommunicationDate = commDate
		comm.Comment = req.Comment

		if err := commRepo.Update(&comm); err != nil {
			handleInternalError(c, err, "Failed to update communication")
			return
		}

		c.JSON(http.StatusOK, comm)
	}
}

// DeleteCommunicationHandler deletes a communication.
//
// @Summary     Delete a communication
// @Description Deletes an existing communication record
// @Tags        communications
// @Param       communicationId path int true "Communication ID"
// @Success     204
// @Failure     400 {object} types.ErrorResponse
// @Failure     404 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /communications/{communicationId} [delete]
func DeleteCommunicationHandler(commRepo repository.CommunicationDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		communicationID, ok := parsePathID(c, "communicationId", "communication")
		if !ok {
			return
		}

		if _, err := commRepo.FindByID(communicationID); err != nil {
			handleNotFound(c, "Communication")
			return
		}

		if err := commRepo.Delete(communicationID); err != nil {
			handleInternalError(c, err, "Failed to delete communication")
			return
		}

		c.Status(http.StatusNoContent)
	}
}
