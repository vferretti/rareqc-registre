package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"registre-admin/internal/repository"
	"registre-admin/internal/types"
)

// AddContactHandler creates a new contact for a participant.
//
// @Summary     Add a contact to a participant
// @Description Creates a new contact for the given participant. Cannot create a "self" contact.
// @Tags        contacts
// @Accept      json
// @Produce     json
// @Param       id   path int                         true "Participant ID"
// @Param       body body types.CreateContactRequest   true "Contact data"
// @Success     201 {object} types.Participant
// @Failure     400 {object} types.ErrorResponse
// @Failure     404 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /participants/{id}/contacts [post]
func AddContactHandler(participantRepo repository.ParticipantDAO, contactRepo repository.ContactDAO, activityRepo repository.ActivityDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		participantID, ok := parsePathID(c, "id", "participant")
		if !ok {
			return
		}
		participant, err := participantRepo.FindByID(participantID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				handleNotFound(c, "Participant")
				return
			}
			handleInternalError(c, err, "Failed to fetch participant")
			return
		}

		var req types.CreateContactRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			handleBadRequest(c, "Invalid request body")
			return
		}

		if req.RelationshipCode == "self" {
			handleBadRequest(c, "Cannot create self contact")
			return
		}

		author := getAuthor(c)

		err = contactRepo.Transaction(func(tx *gorm.DB) error {
			// If new contact is primary, clear all others
			if req.IsPrimary {
				if err := contactRepo.ClearAllPrimary(tx, participant.ID); err != nil {
					return err
				}
			}

			contact := types.Contact{
				ParticipantID:     participant.ID,
				FirstName:         req.FirstName,
				LastName:          req.LastName,
				RelationshipCode:  req.RelationshipCode,
				IsPrimary:         req.IsPrimary,
				PreferredLanguage: req.PreferredLanguage,
				Email:             req.Email,
				Phone:             req.Phone,
				ApartmentNumber:   req.ApartmentNumber,
				StreetAddress:     req.StreetAddress,
				City:              req.City,
				Province:          req.Province,
				CodePostal:        req.CodePostal,
			}
			if req.SameCoordinates {
				if sc := findSelfContact(participant.Contacts); sc != nil {
					contact.Email = sc.Email
					contact.Phone = sc.Phone
					contact.ApartmentNumber = sc.ApartmentNumber
					contact.StreetAddress = sc.StreetAddress
					contact.City = sc.City
					contact.Province = sc.Province
					contact.CodePostal = sc.CodePostal
				}
			}
			if err := contactRepo.Create(tx, &contact); err != nil {
				return err
			}

			// If no non-self contact is primary and new one isn't either, self stays primary
			// If new one is primary, self was already cleared above
			if !req.IsPrimary {
				count, err := contactRepo.CountNonSelfPrimary(tx, participant.ID)
				if err != nil {
					return err
				}
				if count == 0 {
					if err := contactRepo.SetSelfPrimary(tx, participant.ID, true); err != nil {
						return err
					}
				}
			}

			details := fmt.Sprintf("%s %s (%s)", req.FirstName, req.LastName, req.RelationshipCode)
			return activityRepo.Record(tx, types.ActionContactCreated, &participant.ID, author, &details)
		})

		if err != nil {
			handleWriteError(c, err, "Failed to create contact")
			return
		}

		if err := participantRepo.Reload(&participant); err != nil {
			handleInternalError(c, err, "Failed to reload participant")
			return
		}
		c.JSON(http.StatusCreated, participant)
	}
}

// UpdateContactHandler updates an existing contact.
//
// @Summary     Update a contact
// @Description Updates an existing contact. Cannot edit a "self" contact directly.
// @Tags        contacts
// @Accept      json
// @Produce     json
// @Param       contactId path int                         true "Contact ID"
// @Param       body      body types.CreateContactRequest   true "Updated contact data"
// @Success     200 {object} types.Contact
// @Failure     400 {object} types.ErrorResponse
// @Failure     404 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /contacts/{contactId} [put]
func UpdateContactHandler(contactRepo repository.ContactDAO, activityRepo repository.ActivityDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		contactID, ok := parsePathID(c, "contactId", "contact")
		if !ok {
			return
		}
		contact, err := contactRepo.FindByID(contactID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				handleNotFound(c, "Contact")
				return
			}
			handleInternalError(c, err, "Failed to fetch contact")
			return
		}

		if contact.RelationshipCode == "self" {
			handleBadRequest(c, "Cannot edit self contact directly")
			return
		}

		var req types.CreateContactRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			handleBadRequest(c, "Invalid request body")
			return
		}

		author := getAuthor(c)

		err = contactRepo.Transaction(func(tx *gorm.DB) error {
			// If becoming primary, clear all others first
			if req.IsPrimary {
				if err := contactRepo.ClearAllPrimary(tx, contact.ParticipantID); err != nil {
					return err
				}
			}

			contact.FirstName = req.FirstName
			contact.LastName = req.LastName
			contact.RelationshipCode = req.RelationshipCode
			contact.IsPrimary = req.IsPrimary
			contact.PreferredLanguage = req.PreferredLanguage
			contact.Email = req.Email
			contact.Phone = req.Phone
			contact.ApartmentNumber = req.ApartmentNumber
			contact.StreetAddress = req.StreetAddress
			contact.City = req.City
			contact.Province = req.Province
			contact.CodePostal = req.CodePostal

			if err := contactRepo.Save(tx, &contact); err != nil {
				return err
			}

			// Ensure at least one contact is primary
			if !req.IsPrimary {
				count, err := contactRepo.CountNonSelfPrimary(tx, contact.ParticipantID)
				if err != nil {
					return err
				}
				if count == 0 {
					if err := contactRepo.SetSelfPrimary(tx, contact.ParticipantID, true); err != nil {
						return err
					}
				}
			}

			details := fmt.Sprintf("%s %s (%s)", req.FirstName, req.LastName, req.RelationshipCode)
			return activityRepo.Record(tx, types.ActionContactEdited, &contact.ParticipantID, author, &details)
		})

		if err != nil {
			handleWriteError(c, err, "Failed to update contact")
			return
		}

		c.JSON(http.StatusOK, contact)
	}
}

// DeleteContactHandler removes a contact by ID and records the activity.
//
// @Summary     Delete a contact
// @Description Deletes a contact by ID. Cannot delete a "self" contact or one referenced by a consent.
// @Tags        contacts
// @Produce     json
// @Param       contactId path int true "Contact ID"
// @Success     200 {object} object{message=string}
// @Failure     400 {object} types.ErrorResponse
// @Failure     404 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /contacts/{contactId} [delete]
func DeleteContactHandler(contactRepo repository.ContactDAO, activityRepo repository.ActivityDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		contactID, ok := parsePathID(c, "contactId", "contact")
		if !ok {
			return
		}
		contact, err := contactRepo.FindByID(contactID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				handleNotFound(c, "Contact")
				return
			}
			handleInternalError(c, err, "Failed to fetch contact")
			return
		}

		if contact.RelationshipCode == "self" {
			handleBadRequest(c, "Cannot delete self contact")
			return
		}

		referenced, err := contactRepo.IsReferencedByConsent(contact.ID)
		if err != nil {
			handleInternalError(c, err, "Failed to check consent references")
			return
		}
		if referenced {
			handleBadRequest(c, "Cannot delete contact: referenced as consent signer")
			return
		}

		commReferenced, err := contactRepo.IsReferencedByCommunication(contact.ID)
		if err != nil {
			handleInternalError(c, err, "Failed to check communication references")
			return
		}
		if commReferenced {
			handleBadRequest(c, "Cannot delete contact: referenced in a communication")
			return
		}

		author := getAuthor(c)

		err = contactRepo.Transaction(func(tx *gorm.DB) error {
			wasPrimary := contact.IsPrimary
			if err := contactRepo.Delete(tx, &contact); err != nil {
				return err
			}
			if wasPrimary {
				count, err := contactRepo.CountNonSelfPrimary(tx, contact.ParticipantID)
				if err != nil {
					return err
				}
				if count == 0 {
					if err := contactRepo.SetSelfPrimary(tx, contact.ParticipantID, true); err != nil {
						return err
					}
				}
			}
			details := fmt.Sprintf("%s %s (%s)", contact.FirstName, contact.LastName, contact.RelationshipCode)
			return activityRepo.Record(tx, types.ActionContactDeleted, &contact.ParticipantID, author, &details)
		})

		if err != nil {
			handleWriteError(c, err, "Failed to delete contact")
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Contact deleted"})
	}
}
