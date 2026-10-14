package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"registre-admin/internal/repository"
	"registre-admin/internal/types"
)

// ListActivityLogsHandler returns a paginated list of all activity logs with participant names.
//
// @Summary     List activity logs
// @Description Returns a paginated list of all activity logs, with optional filters
// @Tags        activity
// @Produce     json
// @Param       page_index     query int    false "Page index (0-based)"  default(0)
// @Param       page_size      query int    false "Page size (1-200)"     default(25)
// @Param       sort_field     query string false "Sort field"             default(created_at)
// @Param       sort_order     query string false "Sort order (asc/desc)" default(desc)
// @Param       participant_id query int    false "Filter by participant ID"
// @Param       action_type    query string false "Filter by action type code"
// @Param       search         query string false "Search term"
// @Success     200 {object} types.PaginatedResponse[repository.ActivityLogResponse]
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /activity-logs [get]
func ListActivityLogsHandler(repo repository.ActivityDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		params := defaultDescParams(c)

		p := repository.ListParams{
			PaginationParams: params,
			ActionType:       c.Query("action_type"),
			Search:           c.Query("search"),
			DateFrom:         c.Query("date_from"),
			DateTo:           c.Query("date_to"),
			IncludeName:      true,
		}
		if pid := c.Query("participant_id"); pid != "" {
			id, _ := strconv.Atoi(pid)
			p.ParticipantID = &id
		}

		responses, total, totalPages, err := repo.List(p)
		if err != nil {
			handleInternalError(c, err, "Failed to fetch activity logs")
			return
		}

		c.JSON(http.StatusOK, types.PaginatedResponse[repository.ActivityLogResponse]{
			Data:       responses,
			Total:      total,
			PageIndex:  params.PageIndex,
			PageSize:   params.PageSize,
			TotalPages: totalPages,
		})
	}
}

// ListParticipantActivityLogsHandler returns a paginated list of activity logs for a specific participant.
//
// @Summary     List activity logs for a participant
// @Description Returns a paginated list of activity logs for the given participant
// @Tags        activity
// @Produce     json
// @Param       id         path  int    true  "Participant ID"
// @Param       page_index query int    false "Page index (0-based)"  default(0)
// @Param       page_size  query int    false "Page size (1-200)"     default(25)
// @Param       sort_field query string false "Sort field"             default(created_at)
// @Param       sort_order query string false "Sort order (asc/desc)" default(desc)
// @Success     200 {object} types.PaginatedResponse[repository.ActivityLogResponse]
// @Failure     400 {object} types.ErrorResponse
// @Failure     404 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /participants/{id}/activity-logs [get]
func ListParticipantActivityLogsHandler(participantRepo repository.ParticipantDAO, activityRepo repository.ActivityDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		participantID, ok := parsePathID(c, "id", "participant")
		if !ok {
			return
		}

		exists, err := participantRepo.Exists(participantID)
		if err != nil {
			handleInternalError(c, err, "Failed to check participant")
			return
		}
		if !exists {
			handleNotFound(c, "Participant")
			return
		}

		params := defaultDescParams(c)
		p := repository.ListParams{
			PaginationParams: params,
			ParticipantID:    &participantID,
			IncludeName:      false,
		}

		responses, total, totalPages, err := activityRepo.List(p)
		if err != nil {
			handleInternalError(c, err, "Failed to fetch activity logs")
			return
		}

		c.JSON(http.StatusOK, types.PaginatedResponse[repository.ActivityLogResponse]{
			Data:       responses,
			Total:      total,
			PageIndex:  params.PageIndex,
			PageSize:   params.PageSize,
			TotalPages: totalPages,
		})
	}
}

// defaultDescParams returns pagination params with default sort order set to "desc" (most recent first).
func defaultDescParams(c *gin.Context) types.PaginationParams {
	params := parsePaginationParams(c, "created_at")
	if params.SortOrder == "asc" && c.Query("sort_order") == "" {
		params.SortOrder = "desc"
	}
	return params
}
