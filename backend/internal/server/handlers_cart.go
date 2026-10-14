package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"registre-admin/internal/auth"
	"registre-admin/internal/repository"
	"registre-admin/internal/types"
)

// cartExportDataResponse contains all data needed for the multi-sheet Excel report.
type cartExportDataResponse struct {
	Participants []types.Participant              `json:"participants" validate:"required"`
	Consents     []repository.ConsentExportRow    `json:"consents" validate:"required"`
	ExternalIDs  []repository.ExternalIDExportRow `json:"external_ids" validate:"required"`
}

// CartExportDataHandler returns all data for the cart participants (for Excel report generation).
//
// @Summary     Get cart export data
// @Description Returns participants, consents, external IDs, and GUIDs for all cart items
// @Tags        cart
// @Produce     json
// @Success     200 {object} cartExportDataResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /cart/export-data [post]
func CartExportDataHandler(
	cartRepo repository.CartDAO,
	participantRepo repository.ParticipantDAO,
	consentRepo repository.ConsentDAO,
	extIDRepo repository.ExternalIDDAO,
	activityRepo repository.ActivityDAO,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		items, err := cartRepo.ListItems(auth.UserSub(c))
		if err != nil {
			handleInternalError(c, err, "Failed to fetch cart")
			return
		}
		if len(items) == 0 {
			c.JSON(http.StatusOK, cartExportDataResponse{
				Participants: []types.Participant{},
				Consents:     []repository.ConsentExportRow{},
				ExternalIDs:  []repository.ExternalIDExportRow{},
			})
			return
		}

		ids := make([]int, len(items))
		for i, item := range items {
			ids[i] = item.ParticipantID
		}

		participants, err := participantRepo.FindByIDs(ids)
		if err != nil {
			handleInternalError(c, err, "Failed to fetch participants")
			return
		}

		consents, err := consentRepo.ListByParticipantIDs(ids)
		if err != nil {
			handleInternalError(c, err, "Failed to fetch consents")
			return
		}

		extIDs, err := extIDRepo.ListByParticipantIDs(ids)
		if err != nil {
			handleInternalError(c, err, "Failed to fetch external IDs")
			return
		}

		details := fmt.Sprintf("cart export: %d participants (ids %s)", len(participants), joinInts(ids))
		if err := activityRepo.Log("data_exported", nil, getAuthor(c), &details); err != nil {
			handleInternalError(c, err, "Failed to record export")
			return
		}

		c.JSON(http.StatusOK, cartExportDataResponse{
			Participants: participants,
			Consents:     consents,
			ExternalIDs:  extIDs,
		})
	}
}

type addCartRequest struct {
	ParticipantIDs []int `json:"participant_ids" binding:"required"`
}

type removeCartRequest struct {
	ParticipantIDs []int `json:"participant_ids" binding:"required"`
}

type cartListResponse struct {
	Items []repository.CartItemResponse `json:"items" validate:"required"`
	Count int                           `json:"count" validate:"required"`
}

type cartCountResponse struct {
	Count int64 `json:"count" validate:"required"`
}

type cartMutationResponse struct {
	Success bool  `json:"success" validate:"required"`
	Count   int64 `json:"count" validate:"required"`
}

// ListCartItemsHandler returns all items in the user's cart with participant data.
//
// @Summary     List cart items
// @Description Returns all items in the user's cart with participant data
// @Tags        cart
// @Produce     json
// @Success     200 {object} cartListResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /cart/items [get]
func ListCartItemsHandler(repo repository.CartDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		items, err := repo.ListItems(auth.UserSub(c))
		if err != nil {
			handleInternalError(c, err, "Failed to fetch cart")
			return
		}
		c.JSON(http.StatusOK, cartListResponse{Items: items, Count: len(items)})
	}
}

// CartCountHandler returns the number of items in the user's cart.
//
// @Summary     Get cart count
// @Description Returns the number of items in the user's cart
// @Tags        cart
// @Produce     json
// @Success     200 {object} cartCountResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /cart/count [get]
func CartCountHandler(repo repository.CartDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		count, err := repo.CountItems(auth.UserSub(c))
		if err != nil {
			handleInternalError(c, err, "Failed to count cart items")
			return
		}
		c.JSON(http.StatusOK, cartCountResponse{Count: count})
	}
}

// AddCartItemsHandler adds participants to the user's cart.
//
// @Summary     Add items to cart
// @Description Adds one or more participants to the user's cart
// @Tags        cart
// @Accept      json
// @Produce     json
// @Param       body body addCartRequest true "Participant IDs to add"
// @Success     200 {object} cartMutationResponse
// @Failure     400 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /cart/items [post]
func AddCartItemsHandler(repo repository.CartDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req addCartRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			handleBadRequest(c, "participant_ids is required")
			return
		}
		if err := repo.AddItems(auth.UserSub(c), req.ParticipantIDs); err != nil {
			handleWriteError(c, err, "Failed to add items")
			return
		}
		count, _ := repo.CountItems(auth.UserSub(c))
		c.JSON(http.StatusOK, cartMutationResponse{Success: true, Count: count})
	}
}

// RemoveCartItemsHandler removes specific participants from the user's cart.
//
// @Summary     Remove items from cart
// @Description Removes one or more participants from the user's cart
// @Tags        cart
// @Accept      json
// @Produce     json
// @Param       body body removeCartRequest true "Participant IDs to remove"
// @Success     200 {object} cartMutationResponse
// @Failure     400 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /cart/items [delete]
func RemoveCartItemsHandler(repo repository.CartDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req removeCartRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			handleBadRequest(c, "participant_ids is required")
			return
		}
		if err := repo.RemoveItems(auth.UserSub(c), req.ParticipantIDs); err != nil {
			handleInternalError(c, err, "Failed to remove items")
			return
		}
		count, _ := repo.CountItems(auth.UserSub(c))
		c.JSON(http.StatusOK, cartMutationResponse{Success: true, Count: count})
	}
}

// BulkCommunicationRequest represents the payload for creating a communication on every cart participant.
type BulkCommunicationRequest struct {
	MethodCode        string  `json:"method_code" binding:"required"`
	SubjectCode       string  `json:"subject_code" binding:"required"`
	OutcomeCode       *string `json:"outcome_code"`
	CommunicationDate string  `json:"communication_date" binding:"required"`
	Comment           *string `json:"comment"`
}

// bulkCommunicationResponse summarizes the result of a bulk-create operation.
type bulkCommunicationResponse struct {
	Created int   `json:"created" validate:"required"`
	Skipped []int `json:"skipped" validate:"required"`
}

// CreateCartCommunicationsHandler creates one communication record per participant in the user's cart.
//
// For each participant, the handler resolves the contact to use as follows:
//  1. The non-self contact flagged as primary (if any), then
//  2. The "self" contact (always present for any participant).
//
// Participants without any contact are reported in the response's "skipped" array.
//
// @Summary     Create a communication for every cart participant
// @Description Records a communication (phone or email attempt) for each participant currently in the user's cart, using their primary contact
// @Tags        cart
// @Accept      json
// @Produce     json
// @Param       body body BulkCommunicationRequest true "Communication data"
// @Success     201 {object} bulkCommunicationResponse
// @Failure     400 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /cart/communications [post]
func CreateCartCommunicationsHandler(
	cartRepo repository.CartDAO,
	commRepo repository.CommunicationDAO,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req BulkCommunicationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			handleBadRequest(c, "Invalid request body")
			return
		}

		commDate, err := time.Parse("2006-01-02", req.CommunicationDate)
		if err != nil {
			handleBadRequest(c, "Invalid date format (expected YYYY-MM-DD)")
			return
		}

		items, err := cartRepo.ListItems(auth.UserSub(c))
		if err != nil {
			handleInternalError(c, err, "Failed to fetch cart")
			return
		}
		if len(items) == 0 {
			handleBadRequest(c, "Cart is empty")
			return
		}

		participantIDs := make([]int, len(items))
		for i, item := range items {
			participantIDs[i] = item.ParticipantID
		}

		// Pre-load every contact of every cart participant in a single query, then index
		// them by participant ID so we can pick the right one without an extra round trip.
		var contacts []types.Contact
		if err := commRepo.DB().
			Where("participant_id IN ?", participantIDs).
			Find(&contacts).Error; err != nil {
			handleInternalError(c, err, "Failed to fetch contacts")
			return
		}
		byParticipant := make(map[int][]types.Contact, len(participantIDs))
		for _, ct := range contacts {
			byParticipant[ct.ParticipantID] = append(byParticipant[ct.ParticipantID], ct)
		}

		author := getAuthor(c)
		created := 0
		skipped := []int{}

		err = commRepo.DB().Transaction(func(tx *gorm.DB) error {
			for _, pid := range participantIDs {
				contact := pickPrimaryContact(byParticipant[pid])
				if contact == nil {
					skipped = append(skipped, pid)
					continue
				}
				var contactValue *string
				if req.MethodCode == "email" && contact.Email != "" {
					v := contact.Email
					contactValue = &v
				} else if req.MethodCode == "phone" && contact.Phone != "" {
					v := contact.Phone
					contactValue = &v
				}
				comm := types.Communication{
					ParticipantID:     pid,
					ContactID:         contact.ID,
					ContactValue:      contactValue,
					MethodCode:        req.MethodCode,
					SubjectCode:       req.SubjectCode,
					OutcomeCode:       req.OutcomeCode,
					CommunicationDate: commDate,
					Author:            author,
					Comment:           req.Comment,
				}
				if err := tx.Create(&comm).Error; err != nil {
					return err
				}
				created++
			}
			return nil
		})
		if err != nil {
			handleWriteError(c, err, "Failed to create communications")
			return
		}

		c.JSON(http.StatusCreated, bulkCommunicationResponse{Created: created, Skipped: skipped})
	}
}

// pickPrimaryContact returns the contact best suited to receive a bulk communication:
// the non-self primary contact if one exists, otherwise the "self" contact.
func pickPrimaryContact(contacts []types.Contact) *types.Contact {
	var self *types.Contact
	for i := range contacts {
		c := &contacts[i]
		if c.RelationshipCode != "self" && c.IsPrimary {
			return c
		}
		if c.RelationshipCode == "self" {
			self = c
		}
	}
	return self
}

// ClearCartHandler empties the user's cart.
//
// @Summary     Clear cart
// @Description Removes all items from the user's cart
// @Tags        cart
// @Produce     json
// @Success     200 {object} cartMutationResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /cart [delete]
func ClearCartHandler(repo repository.CartDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := repo.ClearCart(auth.UserSub(c)); err != nil {
			handleInternalError(c, err, "Failed to clear cart")
			return
		}
		c.JSON(http.StatusOK, cartMutationResponse{Success: true, Count: 0})
	}
}
