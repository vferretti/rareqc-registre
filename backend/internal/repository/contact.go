package repository

import (
	"gorm.io/gorm"
	"registre-admin/internal/types"
)

// ContactDAO defines the interface for contact data access.
type ContactDAO interface {
	Transaction(fn func(tx *gorm.DB) error) error
	FindByID(id int) (types.Contact, error)
	Create(tx *gorm.DB, c *types.Contact) error
	Save(tx *gorm.DB, c *types.Contact) error
	Delete(tx *gorm.DB, c *types.Contact) error
	SetSelfPrimary(tx *gorm.DB, participantID int, primary bool) error
	ClearAllPrimary(tx *gorm.DB, participantID int) error
	IsReferencedByConsent(contactID int) (bool, error)
	IsReferencedByCommunication(contactID int) (bool, error)
	CountNonSelfPrimary(tx *gorm.DB, participantID int) (int64, error)
}

// ContactRepository handles database operations for contacts.
type ContactRepository struct {
	db *gorm.DB
}

// NewContactRepository creates a new ContactRepository.
func NewContactRepository(db *gorm.DB) *ContactRepository {
	return &ContactRepository{db: db}
}

// Transaction executes fn within a database transaction.
func (r *ContactRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// FindByID returns a contact by ID, or an error if not found.
func (r *ContactRepository) FindByID(id int) (types.Contact, error) {
	var c types.Contact
	err := r.db.First(&c, "id = ?", id).Error
	return c, err
}

// Create inserts a new contact within the given transaction.
func (r *ContactRepository) Create(tx *gorm.DB, c *types.Contact) error {
	return tx.Create(c).Error
}

// Save updates an existing contact within the given transaction.
func (r *ContactRepository) Save(tx *gorm.DB, c *types.Contact) error {
	return tx.Save(c).Error
}

// Delete removes a contact within the given transaction.
func (r *ContactRepository) Delete(tx *gorm.DB, c *types.Contact) error {
	return tx.Delete(c).Error
}

// SetSelfPrimary sets the self contact's is_primary flag for a participant.
func (r *ContactRepository) SetSelfPrimary(tx *gorm.DB, participantID int, primary bool) error {
	return tx.Model(&types.Contact{}).
		Where("participant_id = ? AND relationship_code = ?", participantID, "self").
		Update("is_primary", primary).Error
}

// ClearAllPrimary sets is_primary=false on all contacts for a participant.
func (r *ContactRepository) ClearAllPrimary(tx *gorm.DB, participantID int) error {
	return tx.Model(&types.Contact{}).
		Where("participant_id = ?", participantID).
		Update("is_primary", false).Error
}

// IsReferencedByConsent returns true if the contact is referenced as a signer in any consent.
func (r *ContactRepository) IsReferencedByConsent(contactID int) (bool, error) {
	var count int64
	err := r.db.Table("consent").Where("signed_by_id = ?", contactID).Count(&count).Error
	return count > 0, err
}

// IsReferencedByCommunication returns true if the contact is linked to any communication.
func (r *ContactRepository) IsReferencedByCommunication(contactID int) (bool, error) {
	var count int64
	err := r.db.Table("communication").Where("contact_id = ?", contactID).Count(&count).Error
	return count > 0, err
}

// CountNonSelfPrimary returns the count of non-self primary contacts for a participant.
func (r *ContactRepository) CountNonSelfPrimary(tx *gorm.DB, participantID int) (int64, error) {
	var count int64
	err := tx.Model(&types.Contact{}).
		Where("participant_id = ? AND relationship_code != ? AND is_primary = ?", participantID, "self", true).
		Count(&count).Error
	return count, err
}
