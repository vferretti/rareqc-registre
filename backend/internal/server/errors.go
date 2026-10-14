package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"registre-admin/internal/types"
)

func handleError(c *gin.Context, status int, msg string) {
	c.JSON(status, types.ErrorResponse{Error: msg})
}

func handleNotFound(c *gin.Context, entity string) {
	handleError(c, http.StatusNotFound, entity+" not found")
}

func handleBadRequest(c *gin.Context, msg string) {
	handleError(c, http.StatusBadRequest, msg)
}

// handleInternalError answers a generic 500 and logs the underlying error
// with the request id, so the cause is diagnosable without ever reaching
// the client.
func handleInternalError(c *gin.Context, err error, msg string) {
	slog.Error(msg,
		"error", err,
		"request_id", RequestID(c),
		"method", c.Request.Method,
		"path", c.FullPath())
	handleError(c, http.StatusInternalServerError, msg)
}

func handleConflict(c *gin.Context, msg string) {
	handleError(c, http.StatusConflict, msg)
}

// uniqueConstraintMessages gives a business-level wording to the unique
// constraints a client can realistically trip over. Anything else falls back
// to a generic conflict message.
var uniqueConstraintMessages = map[string]string{
	"participant_ramq_key":                              "A participant with this RAMQ already exists",
	"consent_clause_id_participant_id_key":              "A consent of this type already exists for this participant",
	"external_id_external_system_id_participant_id_key": "This participant already has an ID for this external system",
}

// handleWriteError maps the database errors a mutation can legitimately
// produce to client-facing statuses, and treats everything else as a 500:
//
//   - unique violation          → 409 (with a business message when known)
//   - foreign-key violation     → 400 on insert/update (unknown reference),
//     409 on delete (dependent rows exist)
//   - check-constraint violation → 400
//   - record not found          → 404
//
// Constraint and table names never reach the client; the raw error is logged
// with the request id for diagnosis.
func handleWriteError(c *gin.Context, err error, msg string) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		handleError(c, http.StatusNotFound, "Record not found")
	case errors.Is(err, gorm.ErrDuplicatedKey):
		logWriteRejection(c, err, "unique violation")
		handleConflict(c, uniqueViolationMessage(err))
	case errors.Is(err, gorm.ErrForeignKeyViolated):
		logWriteRejection(c, err, "foreign key violation")
		if c.Request.Method == http.MethodDelete {
			handleConflict(c, "Cannot delete: related records still reference this one")
			return
		}
		handleBadRequest(c, "A referenced record does not exist")
	case errors.Is(err, gorm.ErrCheckConstraintViolated):
		logWriteRejection(c, err, "check constraint violation")
		handleBadRequest(c, "A value violates a database constraint")
	default:
		handleInternalError(c, err, msg)
	}
}

// uniqueViolationMessage picks the wording for a 409 from the violated
// constraint, when the driver exposes it.
func uniqueViolationMessage(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if msg, ok := uniqueConstraintMessages[pgErr.ConstraintName]; ok {
			return msg
		}
	}
	return "A record with the same unique value already exists"
}

// logWriteRejection keeps a trace of rejected writes at warning level, since
// they are client mistakes rather than server failures.
func logWriteRejection(c *gin.Context, err error, kind string) {
	slog.Warn("write rejected: "+kind,
		"error", err,
		"request_id", RequestID(c),
		"method", c.Request.Method,
		"path", c.FullPath())
}
