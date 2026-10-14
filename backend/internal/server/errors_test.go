package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// pgViolation mimics what the GORM postgres driver produces with
// TranslateError: the gorm sentinel wrapping the original *pgconn.PgError.
func pgViolation(sentinel error, code, constraint string) error {
	return fmt.Errorf("%w: %w", sentinel, &pgconn.PgError{Code: code, ConstraintName: constraint, Message: "secret table detail"})
}

func TestHandleWriteErrorMapsDatabaseErrors(t *testing.T) {
	cases := []struct {
		name       string
		method     string
		err        error
		wantStatus int
		wantBody   string
	}{
		{"not found", http.MethodPut, gorm.ErrRecordNotFound, http.StatusNotFound, "Record not found"},
		{"duplicate ramq", http.MethodPost, pgViolation(gorm.ErrDuplicatedKey, "23505", "participant_ramq_key"), http.StatusConflict, "A participant with this RAMQ already exists"},
		{"duplicate consent", http.MethodPost, pgViolation(gorm.ErrDuplicatedKey, "23505", "consent_clause_id_participant_id_key"), http.StatusConflict, "A consent of this type already exists"},
		{"duplicate unknown", http.MethodPost, pgViolation(gorm.ErrDuplicatedKey, "23505", "some_other_key"), http.StatusConflict, "same unique value already exists"},
		{"duplicate without pg detail", http.MethodPost, gorm.ErrDuplicatedKey, http.StatusConflict, "same unique value already exists"},
		{"fk on insert", http.MethodPost, pgViolation(gorm.ErrForeignKeyViolated, "23503", "participant_sex_at_birth_code_fkey"), http.StatusBadRequest, "A referenced record does not exist"},
		{"fk on delete", http.MethodDelete, pgViolation(gorm.ErrForeignKeyViolated, "23503", "consent_clause_id_fkey"), http.StatusConflict, "Cannot delete"},
		{"check", http.MethodPut, pgViolation(gorm.ErrCheckConstraintViolated, "23514", "chk"), http.StatusBadRequest, "violates a database constraint"},
		{"anything else", http.MethodPost, errors.New("connection reset"), http.StatusInternalServerError, "Failed to save"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := performRequest(func(c *gin.Context) { handleWriteError(c, tc.err, "Failed to save") }, tc.method, "/x", "")
			if w.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantBody) {
				t.Errorf("body = %s, want it to contain %q", w.Body.String(), tc.wantBody)
			}
			if strings.Contains(w.Body.String(), "secret table detail") || strings.Contains(w.Body.String(), "_key") {
				t.Errorf("body leaks database details: %s", w.Body.String())
			}
		})
	}
}
