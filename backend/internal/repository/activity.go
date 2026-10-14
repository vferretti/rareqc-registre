package repository

import (
	"math"
	"strings"

	"gorm.io/gorm"
	"registre-admin/internal/types"
)

// ActivityDAO defines the interface for activity log data access.
type ActivityDAO interface {
	Record(tx *gorm.DB, actionTypeCode string, participantID *int, author string, details *string) error
	Log(actionTypeCode string, participantID *int, author string, details *string) error
	List(p ListParams) ([]ActivityLogResponse, int, int, error)
}

// ActivityRepository handles database operations for activity logs.
type ActivityRepository struct {
	db *gorm.DB
}

// NewActivityRepository creates a new ActivityRepository.
func NewActivityRepository(db *gorm.DB) *ActivityRepository {
	return &ActivityRepository{db: db}
}

// Record inserts an activity log entry within the given transaction.
func (r *ActivityRepository) Record(tx *gorm.DB, actionTypeCode string, participantID *int, author string, details *string) error {
	log := types.ActivityLog{
		ActionTypeCode: actionTypeCode,
		ParticipantID:  participantID,
		Author:         author,
		Details:        details,
	}
	return tx.Create(&log).Error
}

// Log inserts an activity log entry outside any caller transaction, for
// read-only operations that must still leave a trace (exports).
func (r *ActivityRepository) Log(actionTypeCode string, participantID *int, author string, details *string) error {
	return r.Record(r.db, actionTypeCode, participantID, author, details)
}

// ActivityLogResponse extends ActivityLog with the participant's full name for display.
type ActivityLogResponse struct {
	types.ActivityLog
	ParticipantName *string `json:"participant_name,omitempty"` // optional: only present when participant exists
}

// ListParams holds query parameters for listing activity logs.
type ListParams struct {
	types.PaginationParams
	ParticipantID *int
	ActionType    string
	Search        string
	DateFrom      string
	DateTo        string
	IncludeName   bool
}

// activitySortFields maps allowed query sort fields to their qualified column names.
var activitySortFields = map[string]string{
	"id":               "activity_log.id",
	"created_at":       "activity_log.created_at",
	"action_type_code": "activity_log.action_type_code",
	"author":           "activity_log.author",
}

// List returns a paginated list of activity logs with optional filtering and participant names.
func (r *ActivityRepository) List(p ListParams) ([]ActivityLogResponse, int, int, error) {
	query := r.db.Model(&types.ActivityLog{})

	if p.ParticipantID != nil {
		query = query.Where("participant_id = ?", *p.ParticipantID)
	}
	if p.ActionType != "" {
		types := strings.Split(p.ActionType, ",")
		if len(types) == 1 {
			query = query.Where("action_type_code = ?", types[0])
		} else {
			query = query.Where("action_type_code IN ?", types)
		}
	}
	if p.DateFrom != "" {
		query = query.Where("activity_log.created_at >= ?", p.DateFrom+"T00:00:00Z")
	}
	if p.DateTo != "" {
		query = query.Where("activity_log.created_at < ?", p.DateTo+"T23:59:59Z")
	}
	if p.Search != "" {
		like := "%" + p.Search + "%"
		query = query.
			Joins("LEFT JOIN participant ON participant.id = activity_log.participant_id").
			Where(
				"activity_log.author ILIKE ? OR activity_log.details ILIKE ? OR CONCAT(participant.first_name, ' ', participant.last_name) ILIKE ?",
				like, like, like,
			)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, 0, err
	}

	// Sort
	sortCol, ok := activitySortFields[p.SortField]
	if !ok {
		sortCol = "activity_log.created_at"
	}
	order := sortCol
	if p.SortOrder == "desc" {
		order += " DESC"
	} else {
		order += " ASC"
	}

	if p.IncludeName {
		query = query.Preload("Participant")
	}

	var logs []types.ActivityLog
	err := query.Order(order).
		Offset(p.PageIndex * p.PageSize).
		Limit(p.PageSize).
		Find(&logs).Error
	if err != nil {
		return nil, 0, 0, err
	}

	// Convert to responses
	responses := make([]ActivityLogResponse, len(logs))
	for i, log := range logs {
		responses[i] = ActivityLogResponse{ActivityLog: log}
		if p.IncludeName && log.Participant != nil {
			name := log.Participant.FirstName + " " + log.Participant.LastName
			responses[i].ParticipantName = &name
		}
	}

	totalPages := int(math.Ceil(float64(total) / float64(p.PageSize)))
	return responses, int(total), totalPages, nil
}
