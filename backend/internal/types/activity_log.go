package types

import "time"

// Action type codes, mirroring the action_type_code reference table.
const (
	ActionParticipantCreated = "participant_created"
	ActionParticipantEdited  = "participant_edited"
	ActionParticipantDeleted = "participant_deleted"
	ActionContactCreated     = "contact_created"
	ActionContactEdited      = "contact_edited"
	ActionContactDeleted     = "contact_deleted"
	ActionConsentAdded       = "consent_added"
	ActionConsentEdited      = "consent_edited"
	ActionDataExported       = "data_exported"
)

type ActivityLog struct {
	ID             int          `json:"id" gorm:"primaryKey;autoIncrement" validate:"required"`
	ActionTypeCode string       `json:"action_type_code" gorm:"not null;type:text;index" validate:"required"`
	ActionType     ActionType   `json:"-" gorm:"foreignKey:ActionTypeCode;references:Code"`
	ParticipantID  *int         `json:"participant_id,omitempty" gorm:"index"`
	Participant    *Participant `json:"-" gorm:"foreignKey:ParticipantID;constraint:OnDelete:SET NULL"`
	Author         string       `json:"author" gorm:"not null;type:text" validate:"required"`
	Details        *string      `json:"details,omitempty" gorm:"type:text"`
	CreatedAt      time.Time    `json:"created_at" gorm:"autoCreateTime;index" validate:"required"`
}

func (ActivityLog) TableName() string { return "activity_log" }
