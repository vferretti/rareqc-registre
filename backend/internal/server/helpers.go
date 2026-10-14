package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"registre-admin/internal/types"
)

// parseUploadForm parses a multipart upload up front so a body cut off by
// the bodyLimit middleware is reported as "too large" instead of surfacing
// later as missing form fields. Returns false after writing the response.
func parseUploadForm(c *gin.Context) bool {
	err := c.Request.ParseMultipartForm(maxUploadBytes)
	if err == nil {
		return true
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		handleError(c, http.StatusRequestEntityTooLarge, "File size exceeds 10 MB limit")
		return false
	}
	handleBadRequest(c, "Invalid multipart form")
	return false
}

// parsePathID reads a numeric path parameter and rejects anything that is not
// a strictly positive integer with a 400. Path parameters must never reach
// the DAO layer as raw strings: GORM treats a non-numeric string passed as an
// inline condition (e.g. First(&p, id)) as raw SQL.
func parsePathID(c *gin.Context, param, label string) (int, bool) {
	id, err := strconv.Atoi(c.Param(param))
	if err != nil || id <= 0 {
		handleBadRequest(c, "Invalid "+label+" ID")
		return 0, false
	}
	return id, true
}

// parseDate parses a "YYYY-MM-DD" string into a *time.Time. Returns nil for empty strings.
func parseDate(dateStr string) (*time.Time, error) {
	if dateStr == "" {
		return nil, nil
	}
	d, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// toStringPtr returns a pointer to s, or nil if s is empty.
func toStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// findSelfContact returns a pointer to the "self" contact in the slice, or nil if none exists.
func findSelfContact(contacts []types.Contact) *types.Contact {
	for i := range contacts {
		if contacts[i].RelationshipCode == "self" {
			return &contacts[i]
		}
	}
	return nil
}

// selfContactSnapshot holds the coordinate fields of the "self" contact for before/after comparison.
type selfContactSnapshot struct {
	Email, Phone, ApartmentNumber, StreetAddress, City, Province, CodePostal, PreferredLanguage string
}

// snapshotSelfContact captures the current coordinate fields of the "self" contact.
func snapshotSelfContact(contacts []types.Contact) selfContactSnapshot {
	if sc := findSelfContact(contacts); sc != nil {
		return selfContactSnapshot{
			Email: sc.Email, Phone: sc.Phone,
			ApartmentNumber: sc.ApartmentNumber, StreetAddress: sc.StreetAddress,
			City: sc.City, Province: sc.Province, CodePostal: sc.CodePostal,
			PreferredLanguage: sc.PreferredLanguage,
		}
	}
	return selfContactSnapshot{}
}

// participantFieldsChanged compares the participant's current fields against the incoming request.
func participantFieldsChanged(p types.Participant, req types.UpdateParticipantRequest, dob time.Time, ramq *string, dateOfDeath *time.Time) bool {
	if p.FirstName != req.FirstName || p.LastName != req.LastName || p.SexAtBirthCode != req.SexAtBirthCode || p.VitalStatusCode != req.VitalStatusCode {
		return true
	}
	if !p.DateOfBirth.Equal(dob) {
		return true
	}
	cityOfBirth := toStringPtr(req.CityOfBirth)
	if (p.CityOfBirth == nil) != (cityOfBirth == nil) || (p.CityOfBirth != nil && cityOfBirth != nil && *p.CityOfBirth != *cityOfBirth) {
		return true
	}
	if (p.RAMQ == nil) != (ramq == nil) || (p.RAMQ != nil && ramq != nil && *p.RAMQ != *ramq) {
		return true
	}
	if (p.DateOfDeath == nil) != (dateOfDeath == nil) || (p.DateOfDeath != nil && dateOfDeath != nil && !p.DateOfDeath.Equal(*dateOfDeath)) {
		return true
	}
	return false
}

// selfContactChanged compares the old self-contact snapshot against the incoming coordinates.
func selfContactChanged(old selfContactSnapshot, req types.UpdateParticipantRequest) bool {
	return old.Email != req.Email || old.Phone != req.Phone ||
		old.ApartmentNumber != req.ApartmentNumber ||
		old.StreetAddress != req.StreetAddress || old.City != req.City ||
		old.Province != req.Province || old.CodePostal != req.CodePostal ||
		old.PreferredLanguage != req.PreferredLanguage
}

// normalizeRAMQ strips spaces and uppercases a RAMQ number for consistent storage.
func normalizeRAMQ(ramq string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(ramq), " ", ""))
}

// guidFieldsChanged returns true if any GUID-relevant field differs between old participant and incoming request.
func guidFieldsChanged(p types.Participant, req types.UpdateParticipantRequest, dob time.Time, ramq *string) bool {
	if p.FirstName != req.FirstName || p.LastName != req.LastName {
		return true
	}
	if !p.DateOfBirth.Equal(dob) {
		return true
	}
	if (p.RAMQ == nil) != (ramq == nil) || (p.RAMQ != nil && ramq != nil && *p.RAMQ != *ramq) {
		return true
	}
	cityOfBirth := toStringPtr(req.CityOfBirth)
	if (p.CityOfBirth == nil) != (cityOfBirth == nil) || (p.CityOfBirth != nil && cityOfBirth != nil && *p.CityOfBirth != *cityOfBirth) {
		return true
	}
	return false
}

// validateDates checks that date of birth is not in the future and date of death is after date of birth.
func validateDates(dob time.Time, dateOfDeath *time.Time) error {
	if dob.After(time.Now()) {
		return fmt.Errorf("date of birth cannot be in the future")
	}
	if dateOfDeath != nil && dateOfDeath.Before(dob) {
		return fmt.Errorf("date of death cannot be before date of birth")
	}
	return nil
}
