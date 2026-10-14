package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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
func (m *mockParticipantDAO) Delete(_ *gorm.DB, id int) error {
	m.deleteCalls = append(m.deleteCalls, id)
	return nil
}
func (m *mockParticipantDAO) FindByIDs([]int) ([]types.Participant, error) { return nil, nil }
func (m *mockParticipantDAO) List(types.PaginationParams) ([]repository.ParticipantListItem, int, int, error) {
	return nil, 0, 0, nil
}
func (m *mockParticipantDAO) Create(*gorm.DB, *types.Participant) error    { return nil }
func (m *mockParticipantDAO) Save(*gorm.DB, *types.Participant) error      { return nil }
func (m *mockParticipantDAO) Exists(int) (bool, error)                     { return true, nil }
func (m *mockParticipantDAO) Transaction(fn func(tx *gorm.DB) error) error { return fn(nil) }
func (m *mockParticipantDAO) Reload(*types.Participant) error              { return nil }

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
			w := performRequest(DeleteParticipantHandler(repo, &mockActivityDAO{}), http.MethodDelete, "/participants/x", "",
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
	w = performRequest(DeleteParticipantHandler(repo, &mockActivityDAO{}), http.MethodDelete, "/participants/42", "",
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

// TestParsePathID pins the accepted shape: strictly positive base-10
// integers only. "12abc" used to pass through fmt.Sscanf as 12.
func TestParsePathID(t *testing.T) {
	cases := map[string]bool{
		"1": true, "42": true, "007": true,
		"0": false, "-1": false, "abc": false, "12abc": false, "": false,
		"1.5": false, " 3": false, "1e3": false, "true": false, "1 OR 1=1": false,
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			w := performRequest(func(c *gin.Context) {
				if id, ok := parsePathID(c, "id", "thing"); ok {
					c.JSON(http.StatusOK, gin.H{"id": id})
				}
			}, http.MethodGet, "/x", "", gin.Param{Key: "id", Value: in})
			if got := w.Code == http.StatusOK; got != want {
				t.Errorf("parsePathID(%q) accepted = %v, want %v (status %d)", in, got, want, w.Code)
			}
		})
	}
}

// performMultipart sends a well-formed (empty) multipart form so handlers
// that parse the upload before the id get past parseUploadForm.
func performMultipart(handler gin.HandlerFunc, method, path string, params ...gin.Param) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("name", "x")
	_ = mw.Close()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, &buf)
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())
	c.Params = params
	handler(c)
	c.Writer.WriteHeaderNow()
	return w
}

// TestEveryIDHandlerRejectsInvalidIDs runs each handler that takes a path
// id with nil DAOs: a malformed id must produce a 400 before any DAO call
// (a nil interface method call would panic and fail the test).
func TestEveryIDHandlerRejectsInvalidIDs(t *testing.T) {
	handlers := []struct {
		name      string
		param     string
		method    string
		handler   gin.HandlerFunc
		multipart bool // handlers parsing an upload form before the id
	}{
		{"GetParticipant", "id", http.MethodGet, GetParticipantHandler(nil), false},
		{"UpdateParticipant", "id", http.MethodPut, UpdateParticipantHandler(nil, nil, nil), false},
		{"DeleteParticipant", "id", http.MethodDelete, DeleteParticipantHandler(nil, nil), false},
		{"AddContact", "id", http.MethodPost, AddContactHandler(nil, nil, nil), false},
		{"UpdateContact", "contactId", http.MethodPut, UpdateContactHandler(nil, nil), false},
		{"DeleteContact", "contactId", http.MethodDelete, DeleteContactHandler(nil, nil), false},
		{"ListParticipantActivityLogs", "id", http.MethodGet, ListParticipantActivityLogsHandler(nil, nil), false},
		{"ListParticipantExternalIDs", "id", http.MethodGet, ListParticipantExternalIDsHandler(nil), false},
		{"DownloadDocument", "id", http.MethodGet, DownloadDocumentHandler(nil), false},
		{"UpdateExternalSystem", "id", http.MethodPut, UpdateExternalSystemHandler(nil), false},
		{"DeleteExternalSystem", "id", http.MethodDelete, DeleteExternalSystemHandler(nil), false},
		{"ListParticipantConsents", "id", http.MethodGet, ListParticipantConsentsHandler(nil), false},
		{"CreateParticipantConsent", "id", http.MethodPost, CreateParticipantConsentHandler(nil, nil, nil), false},
		{"UpdateConsent", "consentId", http.MethodPut, UpdateConsentHandler(nil, nil, nil), false},
		{"DeleteConsentTemplate", "id", http.MethodDelete, DeleteConsentTemplateHandler(nil), false},
		{"UpdateConsentTemplate", "id", http.MethodPut, UpdateConsentTemplateHandler(nil), true},
		{"ListParticipantCommunications", "id", http.MethodGet, ListParticipantCommunicationsHandler(nil), false},
		{"CreateParticipantCommunication", "id", http.MethodPost, CreateParticipantCommunicationHandler(nil), false},
		{"UpdateCommunication", "communicationId", http.MethodPut, UpdateCommunicationHandler(nil), false},
		{"DeleteCommunication", "communicationId", http.MethodDelete, DeleteCommunicationHandler(nil), false},
	}
	for _, h := range handlers {
		for _, id := range []string{"abc", "12abc", "0", "-1", "1 OR 1=1"} {
			t.Run(h.name+"/"+id, func(t *testing.T) {
				var w *httptest.ResponseRecorder
				if h.multipart {
					w = performMultipart(h.handler, h.method, "/x", gin.Param{Key: h.param, Value: id})
				} else {
					w = performRequest(h.handler, h.method, "/x", "{}", gin.Param{Key: h.param, Value: id})
				}
				if w.Code != http.StatusBadRequest {
					t.Errorf("status = %d, want 400 (body %s)", w.Code, w.Body.String())
				}
			})
		}
	}
}
