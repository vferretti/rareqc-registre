package server

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"registre-admin/internal/repository"
)

// activityEntry is one call captured by mockActivityDAO.
type activityEntry struct {
	action        string
	participantID *int
	details       string
	inTx          bool
}

// mockActivityDAO captures every Record/Log call.
type mockActivityDAO struct {
	entries []activityEntry
	err     error
}

func (m *mockActivityDAO) Record(_ *gorm.DB, action string, participantID *int, _ string, details *string) error {
	return m.capture(action, participantID, details, true)
}
func (m *mockActivityDAO) Log(action string, participantID *int, _ string, details *string) error {
	return m.capture(action, participantID, details, false)
}
func (m *mockActivityDAO) List(repository.ListParams) ([]repository.ActivityLogResponse, int, int, error) {
	return nil, 0, 0, nil
}
func (m *mockActivityDAO) capture(action string, participantID *int, details *string, inTx bool) error {
	e := activityEntry{action: action, participantID: participantID, inTx: inTx}
	if details != nil {
		e.details = *details
	}
	m.entries = append(m.entries, e)
	return m.err
}

func TestDeleteParticipantIsAuditedBeforeTheDelete(t *testing.T) {
	participants := &mockParticipantDAO{}
	activity := &mockActivityDAO{}
	w := performRequest(DeleteParticipantHandler(participants, activity), http.MethodDelete, "/participants/42", "",
		gin.Param{Key: "id", Value: "42"})
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", w.Code, w.Body.String())
	}
	if len(activity.entries) != 1 {
		t.Fatalf("activity entries = %+v, want exactly one", activity.entries)
	}
	e := activity.entries[0]
	if e.action != "participant_deleted" || !e.inTx || e.participantID != nil {
		t.Errorf("entry = %+v, want participant_deleted, transactional, no participant reference", e)
	}
	if e.details != "  (#42)" {
		t.Errorf("details = %q, want name and id", e.details)
	}
	if len(participants.deleteCalls) != 1 || participants.deleteCalls[0] != 42 {
		t.Errorf("Delete calls = %v, want [42]", participants.deleteCalls)
	}
}

func TestDeleteParticipantFailsWhenTheAuditCannotBeWritten(t *testing.T) {
	participants := &mockParticipantDAO{}
	activity := &mockActivityDAO{err: gorm.ErrInvalidDB}
	w := performRequest(DeleteParticipantHandler(participants, activity), http.MethodDelete, "/participants/42", "",
		gin.Param{Key: "id", Value: "42"})
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if len(participants.deleteCalls) != 0 {
		t.Errorf("Delete calls = %v, want none when the audit entry fails", participants.deleteCalls)
	}
}

func TestListParticipantsLogsExportsOnly(t *testing.T) {
	activity := &mockActivityDAO{}
	w := performRequest(ListParticipantsHandler(&mockParticipantDAO{}, activity), http.MethodGet,
		"/participants?page_size=50&search=tremblay", "")
	if w.Code != http.StatusOK || len(activity.entries) != 0 {
		t.Fatalf("plain listing: status = %d, entries = %+v; want 200 and no audit entry", w.Code, activity.entries)
	}

	w = performRequest(ListParticipantsHandler(&mockParticipantDAO{}, activity), http.MethodGet,
		"/participants?export=true&page_size=200&page_index=0&search=tremblay", "")
	if w.Code != http.StatusOK {
		t.Fatalf("export listing: status = %d, want 200", w.Code)
	}
	if len(activity.entries) != 1 || activity.entries[0].action != "data_exported" || activity.entries[0].inTx {
		t.Fatalf("entries = %+v, want one non-transactional data_exported entry", activity.entries)
	}
	if got, want := activity.entries[0].details, "participant list export: 0 rows (search=tremblay)"; got != want {
		t.Errorf("details = %q, want %q", got, want)
	}
}

func TestExportDetailsDropsPaginationNoise(t *testing.T) {
	q := url.Values{"export": {"true"}, "page_index": {"0"}, "page_size": {"200"}}
	if got := exportDetails("participant list", 12, q); got != "participant list export: 12 rows" {
		t.Errorf("got %q", got)
	}
	q.Set("consent_registry", "valid")
	if got := exportDetails("participant list", 12, q); got != "participant list export: 12 rows (consent_registry=valid)" {
		t.Errorf("got %q", got)
	}
}

func TestJoinInts(t *testing.T) {
	if got := joinInts([]int{3, 1, 2}); got != "3,1,2" {
		t.Errorf("got %q", got)
	}
	if got := joinInts(nil); got != "" {
		t.Errorf("got %q", got)
	}
}
