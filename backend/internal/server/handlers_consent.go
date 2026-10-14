package server

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"registre-admin/internal/repository"
	"registre-admin/internal/types"
)

// ConsentTemplateResponse is a consent template document with a flag indicating if it has signed consents.
type ConsentTemplateResponse struct {
	types.Document
	HasConsents bool `json:"has_consents" validate:"required"`
}

// ConsentTemplateUpdateResponse is returned after updating a consent template.
type ConsentTemplateUpdateResponse struct {
	ID   int    `json:"id" validate:"required"`
	Name string `json:"name" validate:"required"`
}

// ListParticipantConsentsHandler returns all consents for a participant.
//
// @Summary     List consents for a participant
// @Description Returns all consent records for the given participant, including clause and signer details
// @Tags        consents
// @Produce     json
// @Param       id path int true "Participant ID"
// @Success     200 {array}  repository.ConsentResponse
// @Failure     400 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /participants/{id}/consents [get]
func ListParticipantConsentsHandler(consentRepo repository.ConsentDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		participantID, ok := parsePathID(c, "id", "participant")
		if !ok {
			return
		}

		consents, err := consentRepo.ListByParticipant(participantID)
		if err != nil {
			handleInternalError(c, err, "Failed to fetch consents")
			return
		}

		c.JSON(http.StatusOK, consents)
	}
}

// ListConsentClausesHandler returns all available consent clauses.
//
// @Summary     List consent clauses
// @Description Returns all consent clauses, optionally filtered by template document ID
// @Tags        consents
// @Produce     json
// @Param       template_document_id query int false "Filter by template document ID"
// @Success     200 {array}  types.ConsentClause
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /consent-clauses [get]
func ListConsentClausesHandler(consentRepo repository.ConsentDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		var templateID *int
		if tid := c.Query("template_document_id"); tid != "" {
			if id, err := strconv.Atoi(tid); err == nil && id > 0 {
				templateID = &id
			}
		}
		clauses, err := consentRepo.ListClauses(templateID)
		if err != nil {
			handleInternalError(c, err, "Failed to fetch clauses")
			return
		}
		c.JSON(http.StatusOK, clauses)
	}
}

// ListConsentTemplatesHandler returns all consent template documents.
//
// @Summary     List consent templates
// @Description Returns all consent template documents
// @Tags        consents
// @Produce     json
// @Success     200 {array}  ConsentTemplateResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /consent-templates [get]
func ListConsentTemplatesHandler(consentRepo repository.ConsentDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		templates, err := consentRepo.ListConsentTemplates()
		if err != nil {
			handleInternalError(c, err, "Failed to fetch templates")
			return
		}
		// Batch load which templates have consents (one query instead of N)
		withConsents, err := consentRepo.TemplateIDsWithConsents()
		if err != nil {
			handleInternalError(c, err, "Failed to fetch consent status")
			return
		}
		results := make([]ConsentTemplateResponse, len(templates))
		for i, tpl := range templates {
			results[i] = ConsentTemplateResponse{Document: tpl, HasConsents: withConsents[tpl.ID]}
		}
		c.JSON(http.StatusOK, results)
	}
}

// CreateConsentRequest represents the payload for creating a consent.
// Fields: ClauseID (required), StatusCode (required), Date in YYYY-MM-DD format (required),
// optional SignedByID (contact who signed), and optional DocumentID (linked template document).
type CreateConsentRequest struct {
	ClauseID   int    `json:"clause_id" binding:"required"`
	StatusCode string `json:"status_code" binding:"required"`
	Date       string `json:"date" binding:"required"`
	SignedByID *int   `json:"signed_by_id"`
	DocumentID *int   `json:"document_id"`
}

// CreateParticipantConsentHandler creates a consent for a participant.
//
// @Summary     Create a consent for a participant
// @Description Creates a new consent record. Fails with 409 if a consent for the same clause already exists.
// @Tags        consents
// @Accept      json
// @Produce     json
// @Param       id   path int                  true "Participant ID"
// @Param       body body CreateConsentRequest  true "Consent data"
// @Success     201 {object} types.Consent
// @Failure     400 {object} types.ErrorResponse
// @Failure     409 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /participants/{id}/consents [post]
func CreateParticipantConsentHandler(consentRepo repository.ConsentDAO, contactRepo repository.ContactDAO, activityRepo repository.ActivityDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		participantID, ok := parsePathID(c, "id", "participant")
		if !ok {
			return
		}

		var req CreateConsentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			handleBadRequest(c, "Invalid request body")
			return
		}

		// Validate signer belongs to this participant
		if req.SignedByID != nil {
			signer, err := contactRepo.FindByID(*req.SignedByID)
			if err != nil {
				handleBadRequest(c, "Invalid signed_by_id: contact not found")
				return
			}
			if signer.ParticipantID != participantID {
				handleBadRequest(c, "Signer must be a contact of this participant")
				return
			}
		}

		// Check for duplicate: same participant + same clause type (across templates)
		clauseType, err := consentRepo.ClauseTypeForClause(req.ClauseID)
		if err != nil {
			handleBadRequest(c, "Invalid clause ID")
			return
		}
		typeExists, err := consentRepo.ExistsByClauseType(participantID, clauseType)
		if err != nil {
			handleInternalError(c, err, "Failed to check existing consents")
			return
		}
		if typeExists {
			handleConflict(c, "A consent of this type already exists for this participant")
			return
		}

		date, err := time.Parse("2006-01-02", req.Date)
		if err != nil {
			handleBadRequest(c, "Invalid date format")
			return
		}

		consent := types.Consent{
			ClauseID:      req.ClauseID,
			ParticipantID: participantID,
			Date:          date,
			StatusCode:    req.StatusCode,
			SignedByID:    req.SignedByID,
			DocumentID:    req.DocumentID,
		}

		author := getAuthor(c)
		details := fmt.Sprintf("%s — %s", req.StatusCode, req.Date)

		err = consentRepo.DB().Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&consent).Error; err != nil {
				return err
			}
			return activityRepo.Record(tx, types.ActionConsentAdded, &participantID, author, &details)
		})
		if err != nil {
			handleWriteError(c, err, "Failed to create consent")
			return
		}

		c.JSON(http.StatusCreated, consent)
	}
}

// UpdateConsentRequest represents the payload for updating an existing consent.
// Fields: StatusCode (required), Date in YYYY-MM-DD format (required),
// optional SignedByID (contact who signed), and optional DocumentID (linked template document).
type UpdateConsentRequest struct {
	StatusCode string `json:"status_code" binding:"required"`
	Date       string `json:"date" binding:"required"`
	SignedByID *int   `json:"signed_by_id"`
	DocumentID *int   `json:"document_id"`
}

// UpdateConsentHandler updates an existing consent (status, date, signer, document).
//
// @Summary     Update a consent
// @Description Updates an existing consent's status, date, signer, and/or linked document
// @Tags        consents
// @Accept      json
// @Produce     json
// @Param       consentId path int                  true "Consent ID"
// @Param       body      body UpdateConsentRequest  true "Updated consent data"
// @Success     200 {object} types.Consent
// @Failure     400 {object} types.ErrorResponse
// @Failure     404 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /consents/{consentId} [put]
func UpdateConsentHandler(consentRepo repository.ConsentDAO, contactRepo repository.ContactDAO, activityRepo repository.ActivityDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		consentID, ok := parsePathID(c, "consentId", "consent")
		if !ok {
			return
		}

		consent, err := consentRepo.FindByID(consentID)
		if err != nil {
			handleNotFound(c, "Consent")
			return
		}

		var req UpdateConsentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			handleBadRequest(c, "Invalid request body")
			return
		}

		// Validate signer belongs to this participant
		if req.SignedByID != nil {
			signer, err := contactRepo.FindByID(*req.SignedByID)
			if err != nil {
				handleBadRequest(c, "Invalid signed_by_id: contact not found")
				return
			}
			if signer.ParticipantID != consent.ParticipantID {
				handleBadRequest(c, "Signer must be a contact of this participant")
				return
			}
		}

		date, err := time.Parse("2006-01-02", req.Date)
		if err != nil {
			handleBadRequest(c, "Invalid date format")
			return
		}

		oldStatus := consent.StatusCode
		consent.StatusCode = req.StatusCode
		consent.Date = date
		consent.SignedByID = req.SignedByID
		consent.DocumentID = req.DocumentID

		err = consentRepo.DB().Transaction(func(tx *gorm.DB) error {
			if err := tx.Save(&consent).Error; err != nil {
				return err
			}

			// Record activity if status changed
			if oldStatus != req.StatusCode {
				author := getAuthor(c)
				details := fmt.Sprintf("%s → %s", oldStatus, req.StatusCode)
				if err := activityRepo.Record(tx, types.ActionConsentEdited, &consent.ParticipantID, author, &details); err != nil {
					return err
				}

				// Business rule: if registry consent is withdrawn or expired, cascade to all other clauses
				clauseType, err := consentRepo.ClauseTypeForClause(consent.ClauseID)
				if err != nil {
					return err
				}
				if clauseType == "registry" && (req.StatusCode == "withdrawn" || req.StatusCode == "expired") {
					// Cascade: find all non-registry consents with row-level lock
					var consents []types.Consent
					if err := tx.
						Clauses(clause.Locking{Strength: "UPDATE"}).
						Preload("Clause").
						Joins("JOIN consent_clause ON consent.clause_id = consent_clause.id").
						Where("consent.participant_id = ? AND consent_clause.clause_type_code != ? AND consent.status_code != ?",
							consent.ParticipantID, "registry", req.StatusCode).
						Find(&consents).Error; err != nil {
						return err
					}
					if len(consents) > 0 {
						// Batch update all cascaded consents in one query
						ids := make([]int, len(consents))
						for i, cs := range consents {
							ids[i] = cs.ID
						}
						if err := tx.Model(&types.Consent{}).Where("id IN ?", ids).
							Updates(map[string]interface{}{"status_code": req.StatusCode, "date": date}).Error; err != nil {
							return err
						}
						// Record activity for each cascaded consent
						for _, cs := range consents {
							d := fmt.Sprintf("%s — %s → %s (registre %s)", cs.Clause.ClauseTypeCode, oldStatus, req.StatusCode, req.StatusCode)
							if err := activityRepo.Record(tx, types.ActionConsentEdited, &consent.ParticipantID, author, &d); err != nil {
								return err
							}
						}
					}
				}
			}
			return nil
		})
		if err != nil {
			handleWriteError(c, err, "Failed to update consent")
			return
		}

		c.JSON(http.StatusOK, consent)
	}
}

// CreateConsentTemplateClause represents a clause in the create-template request.
type CreateConsentTemplateClause struct {
	ClauseFr       string `json:"clause_fr" binding:"required"`
	ClauseEn       string `json:"clause_en" binding:"required"`
	ClauseTypeCode string `json:"clause_type_code" binding:"required"`
}

// CreateConsentTemplateHandler creates a consent template document with its clauses.
//
// @Summary     Create a consent template
// @Description Creates a consent template document (PDF) with associated clauses
// @Tags        consents
// @Accept      multipart/form-data
// @Produce     json
// @Param       name    formData string true  "Template display name"
// @Param       clauses formData string true  "JSON array of clauses"
// @Param       file    formData file   true  "PDF file"
// @Success     201 {object} types.Document
// @Failure     400 {object} types.ErrorResponse
// @Failure     413 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /consent-templates [post]
func CreateConsentTemplateHandler(consentRepo repository.ConsentDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !parseUploadForm(c) {
			return
		}
		name := c.PostForm("name")
		clausesJSON := c.PostForm("clauses")

		if name == "" || clausesJSON == "" {
			handleBadRequest(c, "name and clauses are required")
			return
		}

		var clauseReqs []CreateConsentTemplateClause
		if err := json.Unmarshal([]byte(clausesJSON), &clauseReqs); err != nil {
			handleBadRequest(c, "Invalid clauses JSON")
			return
		}

		if len(clauseReqs) == 0 {
			handleBadRequest(c, "At least one clause is required")
			return
		}

		fileHeader, err := c.FormFile("file")
		if err != nil {
			handleBadRequest(c, "file is required")
			return
		}

		const maxUploadSize = 10 << 20 // 10 MB
		if fileHeader.Size > maxUploadSize {
			handleBadRequest(c, "File size exceeds 10 MB limit")
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			handleInternalError(c, err, "Failed to read file")
			return
		}
		defer file.Close()

		fileBytes, err := io.ReadAll(file)
		if err != nil {
			handleInternalError(c, err, "Failed to read file")
			return
		}

		mimeType := fileHeader.Header.Get("Content-Type")
		if mimeType == "" || mimeType == "application/octet-stream" {
			mimeType = mime.TypeByExtension(filepath.Ext(fileHeader.Filename))
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
		}

		doc := types.Document{
			Name:     name,
			FileName: filepath.Base(fileHeader.Filename),
			MimeType: mimeType,
		}

		clauses := make([]types.ConsentClause, len(clauseReqs))
		for i, cr := range clauseReqs {
			clauses[i] = types.ConsentClause{
				ClauseFr:       cr.ClauseFr,
				ClauseEn:       cr.ClauseEn,
				ClauseTypeCode: cr.ClauseTypeCode,
			}
		}

		if err := consentRepo.CreateTemplate(&doc, fileBytes, clauses); err != nil {
			handleWriteError(c, err, "Failed to create template")
			return
		}

		c.JSON(http.StatusCreated, doc)
	}
}

// DeleteConsentTemplateHandler deletes a consent template if no consents exist for it.
//
// @Summary     Delete a consent template
// @Description Deletes a consent template and its clauses. Fails with 409 if any consents exist.
// @Tags        consents
// @Param       id path int true "Template document ID"
// @Success     204
// @Failure     400 {object} types.ErrorResponse
// @Failure     409 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /consent-templates/{id} [delete]
func DeleteConsentTemplateHandler(consentRepo repository.ConsentDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parsePathID(c, "id", "template")
		if !ok {
			return
		}

		hasConsents, err := consentRepo.HasConsentsForTemplate(id)
		if err != nil {
			handleInternalError(c, err, "Failed to check consents")
			return
		}
		if hasConsents {
			handleConflict(c, "Cannot delete: participants have signed this template")
			return
		}

		if err := consentRepo.DeleteTemplate(id); err != nil {
			handleWriteError(c, err, "Failed to delete template")
			return
		}

		c.Status(http.StatusNoContent)
	}
}

// UpdateConsentTemplateHandler updates a consent template if no consents exist for it.
//
// @Summary     Update a consent template
// @Description Updates a consent template's name, clauses, and optionally the PDF. Fails with 409 if any consents exist.
// @Tags        consents
// @Accept      multipart/form-data
// @Produce     json
// @Param       id      path     int    true  "Template document ID"
// @Param       name    formData string true  "Template display name"
// @Param       clauses formData string true  "JSON array of clauses"
// @Param       file    formData file   false "PDF file (optional)"
// @Success     200 {object} ConsentTemplateUpdateResponse
// @Failure     400 {object} types.ErrorResponse
// @Failure     413 {object} types.ErrorResponse
// @Failure     409 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /consent-templates/{id} [put]
func UpdateConsentTemplateHandler(consentRepo repository.ConsentDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !parseUploadForm(c) {
			return
		}
		id, ok := parsePathID(c, "id", "template")
		if !ok {
			return
		}

		hasConsents, err := consentRepo.HasConsentsForTemplate(id)
		if err != nil {
			handleInternalError(c, err, "Failed to check consents")
			return
		}
		if hasConsents {
			handleConflict(c, "Cannot edit: participants have signed this template")
			return
		}

		name := c.PostForm("name")
		clausesJSON := c.PostForm("clauses")
		if name == "" || clausesJSON == "" {
			handleBadRequest(c, "name and clauses are required")
			return
		}

		var clauseReqs []CreateConsentTemplateClause
		if err := json.Unmarshal([]byte(clausesJSON), &clauseReqs); err != nil {
			handleBadRequest(c, "Invalid clauses JSON")
			return
		}

		if len(clauseReqs) == 0 {
			handleBadRequest(c, "At least one clause is required")
			return
		}

		// Optional file replacement
		var fileBytes []byte
		if fileHeader, err := c.FormFile("file"); err == nil {
			const maxUploadSize = 10 << 20 // 10 MB
			if fileHeader.Size > maxUploadSize {
				handleBadRequest(c, "File size exceeds 10 MB limit")
				return
			}
			file, err := fileHeader.Open()
			if err != nil {
				handleInternalError(c, err, "Failed to read file")
				return
			}
			defer file.Close()
			fileBytes, err = io.ReadAll(file)
			if err != nil {
				handleInternalError(c, err, "Failed to read file")
				return
			}
		}

		clauses := make([]types.ConsentClause, len(clauseReqs))
		for i, cr := range clauseReqs {
			clauses[i] = types.ConsentClause{
				ClauseFr:       cr.ClauseFr,
				ClauseEn:       cr.ClauseEn,
				ClauseTypeCode: cr.ClauseTypeCode,
			}
		}

		if err := consentRepo.UpdateTemplate(id, name, fileBytes, clauses); err != nil {
			handleWriteError(c, err, "Failed to update template")
			return
		}

		c.JSON(http.StatusOK, ConsentTemplateUpdateResponse{ID: id, Name: name})
	}
}
