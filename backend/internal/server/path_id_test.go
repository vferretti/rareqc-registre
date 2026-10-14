package server

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"registre-admin/internal/repository"
	"registre-admin/internal/types"
)

// Regression tests for the path-parameter injection: ids used to reach the
// DAO as raw strings, and GORM turns a non-numeric inline condition
// (First(&p, "1 OR 1=1")) into raw SQL. Every handler taking an id must now
// reject non-numeric or non-positive values with a 400 before touching the DAO.

// mockParticipantDAO records the ids it receives; nothing hits a database.
type mockParticipantDAO struct {
	findCalls   []int
	deleteCalls []int
	findErr     error
}

func (m *mockParticipantDAO) FindByID(id int) (types.Participant, error) {
	m.findCalls = append(m.findCalls, id)
	return types.Participant{ID: id}, m.findErr
}
func (m *mockParticipantDAO) Delete(id int) error {
	m.deleteCalls = append(m.deleteCalls, id)
	return nil
}
func (m *mockParticipantDAO) FindByIDs([]int) ([]types.Participant, error) { return nil, nil }
func (m *mockParticipantDAO) List(types.PaginationParams) ([]repository.ParticipantListItem, int, int, error) {
	return nil, 0, 0, nil
}
func (m *mockParticipantDAO) Create(*gorm.DB, *types.Participant) error { return nil }
func (m *mockParticipantDAO) Save(*gorm.DB, *types.Participant) error   { return nil }
func (m *mockParticipantDAO) Exists(int) (bool, error)                  { return true, nil }
func (m *mockParticipantDAO) Transaction(func(tx *gorm.DB) error) error { return nil }
func (m *mockParticipantDAO) Reload(*types.Participant) error           { return nil }

// mockContactDAO records FindByID calls; the other methods are never reached.
type mockContactDAO struct {
	findCalls []int
}

func (m *mockContactDAO) FindByID(id int) (types.Contact, error) {
	m.findCalls = append(m.findCalls, id)
	return types.Contact{ID: id, RelationshipCode: "self"}, nil
}
func (m *mockContactDAO) Transaction(func(tx *gorm.DB) error) error        { return nil }
func (m *mockContactDAO) Create(*gorm.DB, *types.Contact) error            { return nil }
func (m *mockContactDAO) Save(*gorm.DB, *types.Contact) error              { return nil }
func (m *mockContactDAO) Delete(*gorm.DB, *types.Contact) error            { return nil }
func (m *mockContactDAO) SetSelfPrimary(*gorm.DB, int, bool) error         { return nil }
func (m *mockContactDAO) ClearAllPrimary(*gorm.DB, int) error              { return nil }
func (m *mockContactDAO) IsReferencedByConsent(int) (bool, error)          { return false, nil }
func (m *mockContactDAO) IsReferencedByCommunication(int) (bool, error)    { return false, nil }
func (m *mockContactDAO) CountNonSelfPrimary(*gorm.DB, int) (int64, error) { return 0, nil }

// injectionIDs are path values that previously reached GORM as raw SQL,
// plus plain invalid values. All must be rejected before any DAO call.
var injectionIDs = []string{
	"true",
	"1 OR 1=1",
	"id=1 OR 1=1",
	"(select max(id) from participant)",
	"1; DROP TABLE participant",
	"abc",
	"",
	"0",
	"-1",
	"1.5",
}

func TestParticipantHandlersRejectNonNumericIDs(t *testing.T) {
	for _, id := range injectionIDs {
		t.Run("GET "+id, func(t *testing.T) {
			repo := &mockParticipantDAO{}
			w := performRequest(GetParticipantHandler(repo), http.MethodGet, "/participants/x", "",
				gin.Param{Key: "id", Value: id})
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
			if len(repo.findCalls) != 0 {
				t.Errorf("DAO called with %v, want no call", repo.findCalls)
			}
		})
		t.Run("DELETE "+id, func(t *testing.T) {
			repo := &mockParticipantDAO{}
			w := performRequest(DeleteParticipantHandler(repo), http.MethodDelete, "/participants/x", "",
				gin.Param{Key: "id", Value: id})
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
			if len(repo.findCalls)+len(repo.deleteCalls) != 0 {
				t.Errorf("DAO called (find=%v delete=%v), want no call", repo.findCalls, repo.deleteCalls)
			}
		})
		t.Run("PUT "+id, func(t *testing.T) {
			repo := &mockParticipantDAO{}
			w := performRequest(UpdateParticipantHandler(repo, &mockContactDAO{}, nil), http.MethodPut, "/participants/x", "{}",
				gin.Param{Key: "id", Value: id})
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
			if len(repo.findCalls) != 0 {
				t.Errorf("DAO called with %v, want no call", repo.findCalls)
			}
		})
	}
}

func TestParticipantHandlersPassNumericIDAsInt(t *testing.T) {
	repo := &mockParticipantDAO{}
	w := performRequest(GetParticipantHandler(repo), http.MethodGet, "/participants/42", "",
		gin.Param{Key: "id", Value: "42"})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if len(repo.findCalls) != 1 || repo.findCalls[0] != 42 {
		t.Errorf("FindByID calls = %v, want [42]", repo.findCalls)
	}

	repo = &mockParticipantDAO{}
	w = performRequest(DeleteParticipantHandler(repo), http.MethodDelete, "/participants/42", "",
		gin.Param{Key: "id", Value: "42"})
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
	if len(repo.deleteCalls) != 1 || repo.deleteCalls[0] != 42 {
		t.Errorf("Delete calls = %v, want [42]", repo.deleteCalls)
	}
}

func TestContactHandlersRejectNonNumericIDs(t *testing.T) {
	for _, id := range injectionIDs {
		t.Run("POST participant "+id, func(t *testing.T) {
			participants := &mockParticipantDAO{}
			w := performRequest(AddContactHandler(participants, &mockContactDAO{}, nil), http.MethodPost, "/participants/x/contacts", "{}",
				gin.Param{Key: "id", Value: id})
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
			if len(participants.findCalls) != 0 {
				t.Errorf("DAO called with %v, want no call", participants.findCalls)
			}
		})
		for _, method := range []string{http.MethodPut, http.MethodDelete} {
			t.Run(method+" contact "+id, func(t *testing.T) {
				contacts := &mockContactDAO{}
				var handler gin.HandlerFunc
				if method == http.MethodPut {
					handler = UpdateContactHandler(contacts, nil)
				} else {
					handler = DeleteContactHandler(contacts, nil)
				}
				w := performRequest(handler, method, "/contacts/x", "{}",
					gin.Param{Key: "contactId", Value: id})
				if w.Code != http.StatusBadRequest {
					t.Errorf("status = %d, want 400", w.Code)
				}
				if len(contacts.findCalls) != 0 {
					t.Errorf("DAO called with %v, want no call", contacts.findCalls)
				}
			})
		}
	}
}
