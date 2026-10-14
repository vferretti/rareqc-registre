package server

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"registre-admin/internal/guid"
	"registre-admin/internal/repository"
	"registre-admin/internal/types"
)

// ResolveIDsHandler resolves a list of external or internal IDs to participant IDs.
//
// @Summary     Resolve IDs to participant IDs
// @Description Resolves a list of IDs (internal or from an external system) to internal participant IDs
// @Tags        participants
// @Accept      json
// @Produce     json
// @Param       body body resolveIDsRequest true "IDs to resolve"
// @Success     200 {object} resolveIDsResponse
// @Failure     400 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /participants/resolve-ids [post]
func ResolveIDsHandler(participantRepo repository.ParticipantDAO, extIDRepo repository.ExternalIDDAO, guidRepo repository.GuidDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req resolveIDsRequest
		if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 || req.Source == "" {
			handleBadRequest(c, "source and ids are required")
			return
		}

		var resolvedIDs []int
		var notFound []string

		switch req.Source {
		case "internal":
			// Parse string IDs to ints, check existence
			seen := make(map[int]bool)
			for _, raw := range req.IDs {
				var id int
				if _, err := fmt.Sscanf(raw, "%d", &id); err != nil {
					notFound = append(notFound, raw)
					continue
				}
				exists, err := participantRepo.Exists(id)
				if err != nil {
					handleInternalError(c, "Failed to resolve IDs")
					return
				}
				if exists && !seen[id] {
					resolvedIDs = append(resolvedIDs, id)
					seen[id] = true
				} else if !exists {
					notFound = append(notFound, raw)
				}
			}
		case "guid":
			resolvedIDs, notFound = guidRepo.ResolveByGuid(req.IDs)
		default:
			resolvedIDs, notFound = extIDRepo.ResolveBySystem(req.Source, req.IDs)
		}

		if resolvedIDs == nil {
			resolvedIDs = []int{}
		}
		if notFound == nil {
			notFound = []string{}
		}

		c.JSON(http.StatusOK, resolveIDsResponse{
			ResolvedIDs: resolvedIDs,
			NotFound:    notFound,
		})
	}
}

type resolveIDsRequest struct {
	Source string   `json:"source"`
	IDs    []string `json:"ids"`
}

type resolveIDsResponse struct {
	ResolvedIDs []int    `json:"resolved_ids" validate:"required"`
	NotFound    []string `json:"not_found" validate:"required"`
}

// ListParticipantsHandler returns a paginated, sortable, searchable list of participants.
//
// @Summary     List participants
// @Description Returns a paginated, sortable, and searchable list of participants
// @Tags        participants
// @Produce     json
// @Param       page_index query int    false "Page index (0-based)"  default(0)
// @Param       page_size  query int    false "Page size (1-200)"     default(25)
// @Param       sort_field query string false "Sort field"             default(last_name)
// @Param       sort_order query string false "Sort order (asc/desc)" default(asc)
// @Param       search     query string false "Search term (name, RAMQ, etc.)"
// @Success     200 {object} types.PaginatedResponse[repository.ParticipantListItem]
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /participants [get]
func ListParticipantsHandler(repo repository.ParticipantDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		params := parsePaginationParams(c, "last_name")

		participants, total, totalPages, err := repo.List(params)
		if err != nil {
			handleInternalError(c, "Failed to fetch participants")
			return
		}

		c.JSON(http.StatusOK, types.PaginatedResponse[repository.ParticipantListItem]{
			Data:       participants,
			Total:      total,
			PageIndex:  params.PageIndex,
			PageSize:   params.PageSize,
			TotalPages: totalPages,
		})
	}
}

// GetParticipantHandler returns a single participant with its contacts.
//
// @Summary     Get a participant
// @Description Returns a single participant by ID, including contacts and GUID
// @Tags        participants
// @Produce     json
// @Param       id path int true "Participant ID"
// @Success     200 {object} types.Participant
// @Failure     400 {object} types.ErrorResponse
// @Failure     404 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /participants/{id} [get]
func GetParticipantHandler(repo repository.ParticipantDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parsePathID(c, "id", "participant")
		if !ok {
			return
		}
		participant, err := repo.FindByID(id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				handleNotFound(c, "Participant")
				return
			}
			handleInternalError(c, "Failed to fetch participant")
			return
		}
		c.JSON(http.StatusOK, participant)
	}
}

// UpdateParticipantHandler updates a participant's identity and self-contact coordinates.
//
// @Summary     Update a participant
// @Description Updates a participant's identity fields and self-contact coordinates
// @Tags        participants
// @Accept      json
// @Produce     json
// @Param       id   path int                              true "Participant ID"
// @Param       body body types.UpdateParticipantRequest    true "Updated participant data"
// @Success     200 {object} types.Participant
// @Failure     400 {object} types.ErrorResponse
// @Failure     404 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /participants/{id} [put]
func UpdateParticipantHandler(participantRepo repository.ParticipantDAO, contactRepo repository.ContactDAO, activityRepo repository.ActivityDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parsePathID(c, "id", "participant")
		if !ok {
			return
		}
		participant, err := participantRepo.FindByID(id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				handleNotFound(c, "Participant")
				return
			}
			handleInternalError(c, "Failed to fetch participant")
			return
		}

		var req types.UpdateParticipantRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			handleBadRequest(c, "Invalid request body")
			return
		}

		dob, err := time.Parse("2006-01-02", req.DateOfBirth)
		if err != nil {
			handleBadRequest(c, "Invalid date_of_birth format")
			return
		}

		dateOfDeath, err := parseDate(req.DateOfDeath)
		if err != nil {
			handleBadRequest(c, "Invalid date_of_death format")
			return
		}

		if err := validateDates(dob, dateOfDeath); err != nil {
			handleBadRequest(c, err.Error())
			return
		}

		ramq := toStringPtr(normalizeRAMQ(req.RAMQ))
		author := getAuthor(c)

		// Snapshot old state for activity comparison
		oldChanged := participantFieldsChanged(participant, req, dob, ramq, dateOfDeath)
		oldSelfSnapshot := snapshotSelfContact(participant.Contacts)
		oldSelfChanged := selfContactChanged(oldSelfSnapshot, req)
		needGuidRecompute := guidFieldsChanged(participant, req, dob, ramq)

		err = participantRepo.Transaction(func(tx *gorm.DB) error {
			// Update participant fields
			participant.FirstName = req.FirstName
			participant.LastName = req.LastName
			participant.DateOfBirth = dob
			participant.CityOfBirth = toStringPtr(req.CityOfBirth)
			participant.RAMQ = ramq
			participant.SexAtBirthCode = req.SexAtBirthCode
			participant.VitalStatusCode = req.VitalStatusCode
			participant.DateOfDeath = dateOfDeath
			if err := participantRepo.Save(tx, &participant); err != nil {
				return err
			}

			// Recompute GUIDs only if identity fields changed
			if needGuidRecompute {
				g := guid.Compute(&participant)
				if err := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "participant_id"}},
					DoUpdates: clause.AssignmentColumns([]string{"guid_basic", "guid_ramq", "guid_birthplace"}),
				}).Create(g).Error; err != nil {
					return err
				}
			}

			// Update self contact coordinates
			if sc := findSelfContact(participant.Contacts); sc != nil {
				sc.FirstName = req.FirstName
				sc.LastName = req.LastName
				sc.Email = req.Email
				sc.Phone = req.Phone
				sc.ApartmentNumber = req.ApartmentNumber
				sc.StreetAddress = req.StreetAddress
				sc.City = req.City
				sc.Province = req.Province
				sc.CodePostal = req.CodePostal
				sc.PreferredLanguage = req.PreferredLanguage
				if err := contactRepo.Save(tx, sc); err != nil {
					return err
				}
			}

			// Log only if something actually changed
			if oldChanged || oldSelfChanged {
				details := fmt.Sprintf("%s %s", req.FirstName, req.LastName)
				if err := activityRepo.Record(tx, "participant_edited", &participant.ID, author, &details); err != nil {
					return err
				}
			}

			return nil
		})

		if err != nil {
			handleInternalError(c, "Failed to update participant")
			return
		}

		if err := participantRepo.Reload(&participant); err != nil {
			handleInternalError(c, "Failed to reload participant")
			return
		}
		c.JSON(http.StatusOK, participant)
	}
}

// CreateParticipantHandler creates a new participant with a "self" contact and optional additional contacts.
//
// @Summary     Create a participant
// @Description Creates a new participant with a self contact and optional additional contacts
// @Tags        participants
// @Accept      json
// @Produce     json
// @Param       body body types.CreateParticipantRequest true "New participant data"
// @Success     201 {object} types.Participant
// @Failure     400 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /participants [post]
func CreateParticipantHandler(participantRepo repository.ParticipantDAO, contactRepo repository.ContactDAO, activityRepo repository.ActivityDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.CreateParticipantRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			handleBadRequest(c, "Invalid request body")
			return
		}
		dob, err := time.Parse("2006-01-02", req.DateOfBirth)
		if err != nil {
			handleBadRequest(c, "Invalid date_of_birth format")
			return
		}

		dateOfDeath, err := parseDate(req.DateOfDeath)
		if err != nil {
			handleBadRequest(c, "Invalid date_of_death format")
			return
		}

		if err := validateDates(dob, dateOfDeath); err != nil {
			handleBadRequest(c, err.Error())
			return
		}

		participant := types.Participant{
			FirstName:       req.FirstName,
			LastName:        req.LastName,
			DateOfBirth:     dob,
			CityOfBirth:     toStringPtr(req.CityOfBirth),
			RAMQ:            toStringPtr(normalizeRAMQ(req.RAMQ)),
			SexAtBirthCode:  req.SexAtBirthCode,
			VitalStatusCode: req.VitalStatusCode,
			DateOfDeath:     dateOfDeath,
		}

		author := getAuthor(c)

		err = participantRepo.Transaction(func(tx *gorm.DB) error {
			if err := participantRepo.Create(tx, &participant); err != nil {
				return err
			}
			if err := activityRepo.Record(tx, "participant_created", &participant.ID, author, nil); err != nil {
				return err
			}

			// Create self contact
			selfContact := types.Contact{
				ParticipantID:     participant.ID,
				FirstName:         req.FirstName,
				LastName:          req.LastName,
				RelationshipCode:  "self",
				IsPrimary:         true,
				Email:             req.Email,
				Phone:             req.Phone,
				ApartmentNumber:   req.ApartmentNumber,
				StreetAddress:     req.StreetAddress,
				City:              req.City,
				Province:          req.Province,
				CodePostal:        req.CodePostal,
				PreferredLanguage: req.PreferredLanguage,
			}
			if err := contactRepo.Create(tx, &selfContact); err != nil {
				return err
			}

			// Create additional contacts
			for _, cr := range req.Contacts {
				contact := types.Contact{
					ParticipantID:     participant.ID,
					FirstName:         cr.FirstName,
					LastName:          cr.LastName,
					RelationshipCode:  cr.RelationshipCode,
					IsPrimary:         cr.IsPrimary,
					PreferredLanguage: cr.PreferredLanguage,
					Email:             cr.Email,
					Phone:             cr.Phone,
					ApartmentNumber:   cr.ApartmentNumber,
					StreetAddress:     cr.StreetAddress,
					City:              cr.City,
					Province:          cr.Province,
					CodePostal:        cr.CodePostal,
				}
				if cr.SameCoordinates {
					contact.Email = req.Email
					contact.Phone = req.Phone
					contact.ApartmentNumber = req.ApartmentNumber
					contact.StreetAddress = req.StreetAddress
					contact.City = req.City
					contact.Province = req.Province
					contact.CodePostal = req.CodePostal
				}
				if err := contactRepo.Create(tx, &contact); err != nil {
					return err
				}
			}

			// Compute and upsert GUIDs
			g := guid.Compute(&participant)
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "participant_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"guid_basic", "guid_ramq", "guid_birthplace"}),
			}).Create(g).Error; err != nil {
				return err
			}

			// If a non-self contact is primary, self loses the flag
			hasPrimary := false
			for _, cr := range req.Contacts {
				if cr.IsPrimary {
					hasPrimary = true
					break
				}
			}
			if hasPrimary {
				return contactRepo.SetSelfPrimary(tx, participant.ID, false)
			}
			return nil
		})

		if err != nil {
			handleInternalError(c, "Failed to create participant")
			return
		}

		if err := participantRepo.Reload(&participant); err != nil {
			handleInternalError(c, "Failed to reload participant")
			return
		}
		c.JSON(http.StatusCreated, participant)
	}
}

// DeleteParticipantHandler permanently deletes a participant and all related data (cascade).
//
// @Summary     Delete a participant
// @Description Permanently deletes a participant and all associated data (contacts, consents, communications, activity logs, GUIDs, external IDs, cart items). This action is irreversible.
// @Tags        participants
// @Param       id path int true "Participant ID"
// @Success     204
// @Failure     400 {object} types.ErrorResponse
// @Failure     404 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /participants/{id} [delete]
func DeleteParticipantHandler(participantRepo repository.ParticipantDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parsePathID(c, "id", "participant")
		if !ok {
			return
		}

		if _, err := participantRepo.FindByID(id); err != nil {
			handleNotFound(c, "Participant")
			return
		}

		if err := participantRepo.Delete(id); err != nil {
			handleInternalError(c, "Failed to delete participant")
			return
		}

		c.Status(http.StatusNoContent)
	}
}
