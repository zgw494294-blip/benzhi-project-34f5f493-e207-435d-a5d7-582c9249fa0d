package auditreaderreuse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"powerpermit/internal/application"
	"powerpermit/internal/audit"
	"powerpermit/internal/domain"
	"powerpermit/internal/storage"
	"powerpermit/internal/web"
)

type auditResponse struct {
	Items  []audit.Event        `json:"items"`
	Digest audit.TimelineDigest `json:"digest"`
}

func TestRepeatedAuditQueriesRemainComplete(t *testing.T) {
	repo, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.New(repo, audit.New())
	now := time.Now().UTC()
	created, err := service.CreateCase(application.CreateCaseCommand{
		Meta:         application.Meta{Actor: "申请人", IdempotencyKey: "create-audit-case"},
		ActivityName: "展会",
		Venue:        "展馆",
		StartAt:      now.Add(time.Hour),
		EndAt:        now.Add(2 * time.Hour),
		Contact:      domain.Contact{Name: "李工", Phone: "13800000000"},
		RiskLevel:    domain.RiskLow,
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := web.New(service).Handler()
	path := "/api/cases/" + created.Case.Case.ID + "/audit"
	first := getAudit(t, handler, path)
	second := getAudit(t, handler, path)
	if len(first.Items) == 0 {
		t.Fatal("first audit query unexpectedly returned no events")
	}
	if len(second.Items) != len(first.Items) || second.Digest.EventCount != first.Digest.EventCount {
		t.Fatalf("repeated audit query lost events: first=%d second=%d", len(first.Items), len(second.Items))
	}
}

func getAudit(t *testing.T, handler http.Handler, path string) auditResponse {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("audit request returned status %d", response.Code)
	}
	var payload auditResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}
